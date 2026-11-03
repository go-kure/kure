package built

import (
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type namedItems []runtime.Object

type genericList[T any] struct {
	metav1.TypeMeta `json:",inline"`
	Items           T `json:"items"`
}

func (l *genericList[T]) DeepCopyObject() runtime.Object { c := *l; return &c }

// privateCycleList is a typed List of items no object, with a private map that
// holds itself: formatting it has no end.
type privateCycleList struct {
	metav1.TypeMeta `json:",inline"`
	Items           []int `json:"items"`
	cycle           map[string]any
}

func (l *privateCycleList) DeepCopyObject() runtime.Object { c := *l; return &c }

// goItems reads a well-formed typed List as meta.ExtractList does.
func TestGoItems_MatchesExtractListOnWellFormedLists(t *testing.T) {
	cm := &corev1.ConfigMap{}
	cm.Name = "a"
	objs := []runtime.Object{cm, &corev1.Secret{}}
	ptrs := &objs
	cases := map[string]runtime.Object{
		"[]runtime.Object":         &genericList[[]runtime.Object]{Items: objs},
		"named slice":              &genericList[namedItems]{Items: namedItems(objs)},
		"[]ConfigMap":              &genericList[[]corev1.ConfigMap]{Items: []corev1.ConfigMap{*cm, *cm}},
		"[]*ConfigMap":             &genericList[[]*corev1.ConfigMap]{Items: []*corev1.ConfigMap{cm, nil}},
		"pointer to slice":         &genericList[*[]runtime.Object]{Items: ptrs},
		"pointer in an interface":  &genericList[any]{Items: ptrs},
		"raw extension, object":    &genericList[[]runtime.RawExtension]{Items: []runtime.RawExtension{{Object: cm}, {}}},
		"empty":                    &genericList[[]runtime.Object]{},
		"metav1.List":              &metav1.List{Items: []runtime.RawExtension{{Object: cm}}},
		"nil pointer to slice not": &genericList[*[]runtime.Object]{},
	}
	for name, obj := range cases {
		t.Run(name, func(t *testing.T) {
			want, wantErr := meta.ExtractList(obj)
			got, isList, nilEntry, err := goItems(obj)
			if !isList || nilEntry {
				t.Fatalf("isList = %v, nilEntry = %v, want a List without one", isList, nilEntry)
			}
			if (err != nil) != (wantErr != nil) {
				t.Fatalf("error %v, ExtractList's %v", err, wantErr)
			}
			if len(got) != len(want) || (len(got) > 0 && !reflect.DeepEqual(got, want)) {
				t.Errorf("items %v, ExtractList's %v", got, want)
			}
		})
	}
}

// An item held as raw JSON is read from it, as the object is written.
func TestGoItems_RawJSONBeatsObject(t *testing.T) {
	l := &metav1.List{Items: []runtime.RawExtension{{Raw: []byte(`{"kind":"X"}`), Object: &corev1.ConfigMap{}}}}
	got, _, _, err := goItems(l)
	if err != nil || len(got) != 1 {
		t.Fatalf("got %v, %v", got, err)
	}
	if u, ok := got[0].(*runtime.Unknown); !ok || string(u.Raw) != `{"kind":"X"}` {
		t.Errorf("got %v, want the raw JSON", got[0])
	}
}

// Shapes meta.ExtractList panics on are no List, or a refusal, and never a
// panic; a nil entry is reported.
func TestGoItems_ShapesApimachineryPanicsOn(t *testing.T) {
	var nilPtr *genericList[[]runtime.Object]
	for name, tc := range map[string]struct {
		obj              runtime.Object
		isList, nilEntry bool
		wantErr          string
	}{
		"typed nil":                {obj: nilPtr},
		"nil entry":                {obj: &genericList[[]runtime.Object]{Items: []runtime.Object{nil, &corev1.ConfigMap{}}}, isList: true, nilEntry: true},
		"nil interface items":      {obj: &genericList[any]{}},
		"nil items pointer":        {obj: &genericList[*[]runtime.Object]{}, isList: true, wantErr: "Items pointer is nil"},
		"items that are no object": {obj: &genericList[[]int]{Items: []int{1}}, isList: true, wantErr: "not objects"},
		"items that are no slice":  {obj: &genericList[int]{}},
	} {
		t.Run(name, func(t *testing.T) {
			_, isList, nilEntry, err := goItems(tc.obj)
			if isList != tc.isList || nilEntry != tc.nilEntry {
				t.Errorf("isList = %v, nilEntry = %v, want %v, %v", isList, nilEntry, tc.isList, tc.nilEntry)
			}
			if tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Errorf("error %v, want one containing %q", err, tc.wantErr)
			}
		})
	}
}

// A List of items that are no object, with a private map that holds itself,
// is refused without formatting the List.
func TestGoItems_ItemsOfNoObjectAndAPrivateCycleAreRefusedNotFormatted(t *testing.T) {
	l := &privateCycleList{Items: []int{1}, cycle: map[string]any{}}
	l.cycle["self"] = l.cycle
	_, isList, _, err := goItems(l)
	if !isList || err == nil || !strings.Contains(err.Error(), "privateCycleList") {
		t.Errorf("isList = %v, error %v, want a refusal naming the type", isList, err)
	}
}
