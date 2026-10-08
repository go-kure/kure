package io

import (
	"reflect"
	"strings"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/kubernetes"
)

// The list types below have Items fields of shapes apimachinery's list reading
// does not expect (go-kure/kure#1031). None is a client.Object, so a document
// of each reaches that reading, which is given the zero value Scheme.New
// returns. They are registered in kubernetes.Scheme under a group of their
// own, which no other test names.
var guardGV = schema.GroupVersion{Group: "listguard.kure.test", Version: "v1"}

// anyItemsList holds its items in an interface, nil in the zero value:
// apimachinery panics on it.
type anyItemsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           any `json:"items"`
}

func (l *anyItemsList) DeepCopyObject() runtime.Object { c := *l; return &c }

// itemsHolder is an unexported struct whose Items field the lists below
// reach by embedding it.
type itemsHolder struct {
	Items []corev1.ConfigMap `json:"items"`
}

// embeddedItemsList reaches its items through an embedded pointer, nil in the
// zero value: apimachinery panics on it.
type embeddedItemsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	*itemsHolder    `json:",inline"`
}

func (l *embeddedItemsList) DeepCopyObject() runtime.Object { c := *l; return &c }

// unexportedItemsList reaches its items through an unexported embedded
// struct. Reflection reads a field promoted from it as any exported field, so
// apimachinery reads it as a List.
type unexportedItemsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	itemsHolder     `json:",inline"`
}

func (l *unexportedItemsList) DeepCopyObject() runtime.Object { c := *l; return &c }

// pointerItemsList holds its items behind a pointer, nil in the zero value.
// apimachinery reads it as a List and returns that nil pointer.
type pointerItemsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           *[]corev1.ConfigMap `json:"items"`
}

func (l *pointerItemsList) DeepCopyObject() runtime.Object { c := *l; return &c }

var registerGuardTypes = sync.OnceFunc(func() {
	kubernetes.Scheme.AddKnownTypeWithName(guardGV.WithKind("AnyItemsList"), &anyItemsList{})
	kubernetes.Scheme.AddKnownTypeWithName(guardGV.WithKind("EmbeddedItemsList"), &embeddedItemsList{})
	kubernetes.Scheme.AddKnownTypeWithName(guardGV.WithKind("UnexportedItemsList"), &unexportedItemsList{})
	kubernetes.Scheme.AddKnownTypeWithName(guardGV.WithKind("PointerItemsList"), &pointerItemsList{})
})

// guardList is a document of kind with one ConfigMap among its items.
func guardList(kind string) string {
	return `{"apiVersion":"listguard.kure.test/v1","kind":"` + kind +
		`","items":[{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"a"}}]}`
}

// TestTypedListItems: the guarded step reads the items of a typed list, and
// reads a list apimachinery panics on as no List.
func TestTypedListItems(t *testing.T) {
	registerGuardTypes()
	configMap := reflect.TypeOf(corev1.ConfigMap{})

	got, ok := typedListItems(&corev1.ConfigMapList{})
	if !ok || got != configMap {
		t.Errorf("ConfigMapList: got %v, %v; want %v, true", got, ok, configMap)
	}
	got, ok = typedListItems(&metav1.List{})
	if !ok || got != reflect.TypeOf(runtime.RawExtension{}) {
		t.Errorf("List: got %v, %v; want %v, true", got, ok, reflect.TypeOf(runtime.RawExtension{}))
	}
	if got, ok = typedListItems(&corev1.ConfigMap{}); ok {
		t.Errorf("ConfigMap: got %v, true; want no List", got)
	}

	for _, kind := range []string{"AnyItemsList", "EmbeddedItemsList"} {
		list, err := kubernetes.Scheme.New(guardGV.WithKind(kind))
		if err != nil {
			t.Fatal(err)
		}
		// GetItemsPtr, not IsListType: IsListType keeps its answer per type,
		// so a repeated run would not reach the panic.
		mustPanic(t, kind, func() { _, _ = meta.GetItemsPtr(list) })
		if got, ok := typedListItems(list); ok {
			t.Errorf("%s: got %v, true; want no List", kind, got)
		}
	}
	for _, kind := range []string{"UnexportedItemsList", "PointerItemsList"} {
		list, err := kubernetes.Scheme.New(guardGV.WithKind(kind))
		if err != nil {
			t.Fatal(err)
		}
		if got, ok := typedListItems(list); !ok || got != configMap {
			t.Errorf("%s: got %v, %v; want %v, true", kind, got, ok, configMap)
		}
	}
}

// TestParse_ListShapesApimachineryCannotRead: a document of a registered list
// kind returns objects or an error, never a panic, whatever the shape of its
// Items. A kind apimachinery panics on is not read as a list: it is decoded as
// one object of its type, and refused as no client.Object or as a document
// that does not decode. The steps run in order: the fifth has apimachinery
// take anyItemsList for a List first, which it keeps, so the panic comes where
// the items are read.
func TestParse_ListShapesApimachineryCannotRead(t *testing.T) {
	registerGuardTypes()

	// parse fails the test unless ParseYAML on a document of kind returns,
	// without a panic, the objects named in want or an error containing
	// wantErr.
	parse := func(name, kind string, want []string, wantErr string) {
		t.Helper()
		var objs []client.Object
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%s: ParseYAML panicked: %v", name, r)
				}
			}()
			objs, err = ParseYAML([]byte(guardList(kind)))
		}()
		if wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), wantErr) {
				t.Errorf("%s: error %v, want one containing %q", name, err, wantErr)
			}
		} else if err != nil {
			t.Errorf("%s: error %v, want none", name, err)
		}
		var got []string
		for _, o := range objs {
			got = append(got, o.GetObjectKind().GroupVersionKind().Kind+"/"+o.GetName())
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}

	parse("items of interface type", "AnyItemsList",
		nil, "object of type *io.anyItemsList does not implement client.Object")
	parse("items behind a nil embedded pointer", "EmbeddedItemsList",
		nil, "failed to decode object")
	parse("items promoted from an unexported struct", "UnexportedItemsList",
		[]string{"ConfigMap/a"}, "")
	parse("items behind a pointer", "PointerItemsList",
		[]string{"ConfigMap/a"}, "")

	if !meta.IsListType(&anyItemsList{Items: &[]corev1.ConfigMap{}}) {
		t.Fatal("apimachinery does not take anyItemsList holding a pointer to a slice for a List")
	}
	list, err := kubernetes.Scheme.New(guardGV.WithKind("AnyItemsList"))
	if err != nil {
		t.Fatal(err)
	}
	mustPanic(t, "AnyItemsList, once taken for a List", func() { _, _ = meta.GetItemsPtr(list) })
	parse("items of interface type, once taken for a List", "AnyItemsList",
		nil, "object of type *io.anyItemsList does not implement client.Object")
}

// mustPanic fails the test unless f panics: a shape the guard is tested on
// must be one apimachinery panics on, or the test proves nothing.
func mustPanic(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s: apimachinery did not panic; the shape no longer tests the guard", name)
		}
	}()
	f()
}
