package built

import (
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// kindPanics reports its kind with a panic.
type kindPanics struct{ metav1.TypeMeta }

func (k *kindPanics) GetObjectKind() schema.ObjectKind { panic("GetObjectKind") }
func (k *kindPanics) DeepCopyObject() runtime.Object   { c := *k; return &c }

// marshalPanics reports a kind and panics when it is marshalled.
type marshalPanics struct{ metav1.TypeMeta }

func (m *marshalPanics) MarshalJSON() ([]byte, error)   { panic(errors.New("MarshalJSON")) }
func (m *marshalPanics) DeepCopyObject() runtime.Object { c := *m; return &c }

// marshalFails reports a kind and returns an error when it is marshalled.
type marshalFails struct{ metav1.TypeMeta }

func (m *marshalFails) MarshalJSON() ([]byte, error)   { return nil, errors.New("no JSON for you") }
func (m *marshalFails) DeepCopyObject() runtime.Object { c := *m; return &c }

func listHolding(items ...runtime.Object) *objectItems {
	return &objectItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}, Items: items}
}

// A method of a caller's object that panics is a refusal naming the Go type
// of the object, whatever the panic value is, and it is told from an error the
// method returned.
func TestGuard_PanicIsARefusalNamingTheType(t *testing.T) {
	_, err := kindOf(&kindPanics{})
	if err == nil || !isRefusal(err) {
		t.Fatalf("kindOf = %v, want a refusal", err)
	}
	for _, want := range []string{"*built.kindPanics", "string", "panicked"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not mention %q", err, want)
		}
	}

	_, err = marshalHeld(&marshalPanics{TypeMeta: metav1.TypeMeta{Kind: "X"}})
	if err == nil || !isRefusal(err) || !strings.Contains(err.Error(), "*built.marshalPanics") {
		t.Errorf("marshalHeld of a panicking marshaler = %v, want a refusal naming its type", err)
	}

	_, err = marshalHeld(&marshalFails{TypeMeta: metav1.TypeMeta{Kind: "X"}})
	if err == nil || isRefusal(err) {
		t.Errorf("marshalHeld of a marshaler that fails = %v, want its own error, no refusal", err)
	}
}

// WrittenType reads the kind and the apiVersion from the written form.
func TestWrittenType(t *testing.T) {
	cases := map[string]struct {
		obj               runtime.Object
		kind, apiVersion  string
		wantErr, wantRefs string
	}{
		"unstructured":       {obj: configMap("a"), kind: "ConfigMap", apiVersion: "v1"},
		"typed":              {obj: typedConfigMap("a"), kind: "ConfigMap", apiVersion: "v1"},
		"typed, no TypeMeta": {obj: &corev1.ConfigMap{}},
		"type meta not written": {obj: &unwrittenType{
			TypeMeta: metav1.TypeMeta{Kind: "Hidden"}, WrittenKind: "Shown", WrittenAPIVersion: "v9",
		}, kind: "Shown", apiVersion: "v9"},
		"cannot be written":    {obj: &unwritable{Bad: make(chan int)}, wantErr: "marshal"},
		"method that panics":   {obj: &kindPanics{}, wantErr: "panicked", wantRefs: "yes"},
		"marshaler that fails": {obj: &marshalFails{}, wantErr: "no JSON for you"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			kind, apiVersion, err := WrittenType(c.obj)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("error = %v, want one with %q", err, c.wantErr)
				}
				if got := isRefusal(err); got != (c.wantRefs != "") {
					t.Errorf("isRefusal = %v, want %v", got, c.wantRefs != "")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if kind != c.kind || apiVersion != c.apiVersion {
				t.Errorf("WrittenType = %q, %q; want %q, %q", kind, apiVersion, c.kind, c.apiVersion)
			}
		})
	}
}

// RefuseItemCycle fails for a typed List that holds itself, directly or through
// a List it holds, and not for one that holds a List twice or none.
func TestRefuseItemCycle(t *testing.T) {
	self := listHolding()
	self.Items = []runtime.Object{self}

	inner := listHolding()
	outer := listHolding(inner)
	inner.Items = []runtime.Object{outer}

	shared := listHolding(configMap("a"))
	twice := listHolding(shared, shared)

	for name, c := range map[string]struct {
		list    runtime.Object
		wantErr bool
	}{
		"holds itself":                  {self, true},
		"holds itself through a List":   {outer, true},
		"holds a List twice":            {twice, false},
		"holds an unstructured object":  {listHolding(configMap("a")), false},
		"is an unstructured object":     {configMap("a"), false},
		"is no List":                    {typedConfigMap("a"), false},
		"is a List of typed ConfigMaps": {&corev1.List{}, false},
	} {
		t.Run(name, func(t *testing.T) {
			err := RefuseItemCycle(c.list)
			if (err != nil) != c.wantErr {
				t.Fatalf("RefuseItemCycle = %v, want an error: %v", err, c.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "holds itself") {
				t.Errorf("error %q does not say the List holds itself", err)
			}
		})
	}
}

// HeldItems reads the items of a List apimachinery cannot read (one that holds
// a nil entry) as far as it can, and returns the error where the List is read.
func TestHeldItems_ListThatCannotBeRead(t *testing.T) {
	a, b := configMap("a"), configMap("b")
	inner := listHolding(nil, b, &runtime.Unknown{Raw: []byte(`{"kind":"X"}`)}, typedConfigMap("t"))
	outer := listHolding(nil, a, inner)

	got, err := HeldItems(outer)
	if err == nil {
		t.Fatal("HeldItems of a List with a nil entry gave no error")
	}
	if len(got) != 2 || got[0] != a || got[1] != b {
		t.Errorf("HeldItems = %v, want the two unstructured items, nested ones included", got)
	}
	if got, err := HeldItems(configMap("a")); got != nil || err != nil {
		t.Errorf("HeldItems of an unstructured object = %v, %v; want neither", got, err)
	}
	if got, err := HeldItems(typedConfigMap("a")); got != nil || err != nil {
		t.Errorf("HeldItems of a typed object that is no List = %v, %v; want neither", got, err)
	}
}

// HeldObjects of what holds no item, and of a List that holds itself.
func TestHeldObjects_EdgesAndErrors(t *testing.T) {
	for name, r := range map[string]runtime.Object{
		"an unstructured object": configMap("a"),
		"a typed object":         typedConfigMap("a"),
		"an empty List":          listHolding(),
	} {
		if items, others, err := HeldObjects(r); items != nil || others != nil || err != nil {
			t.Errorf("%s: HeldObjects = %v, %v, %v; want nothing", name, items, others, err)
		}
	}

	self := listHolding()
	self.Items = []runtime.Object{self}
	if _, _, err := HeldObjects(self); err == nil || !strings.Contains(err.Error(), "holds itself") {
		t.Errorf("HeldObjects of a List that holds itself = %v, want a refusal", err)
	}

	// An error in a nested List stops the walk and is returned with what was
	// found before it.
	bad := listHolding(nil)
	outer := listHolding(configMap("a"), bad)
	items, others, err := HeldObjects(outer)
	if err == nil {
		t.Fatal("HeldObjects of a List that holds a List with a nil entry gave no error")
	}
	if len(items) != 1 || len(others) != 1 || others[0] != runtime.Object(bad) {
		t.Errorf("HeldObjects = %v, %v; want the first item and the inner List", items, others)
	}
}
