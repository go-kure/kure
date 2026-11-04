package fluxcd

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack/layout"
)

// A plain unstructured List that a typed List holds has its own Object map
// beside its items, and the map is normalized like any object's: apimachinery
// panics in DeepCopy on a plain int in it. The map is put back by the undo.
func TestNormalizeCallerObjects_PlainListOwnMapIsNormalized(t *testing.T) {
	own := func() map[string]any {
		return map[string]any{"apiVersion": "v1", "kind": "List", "metadata": map[string]any{"x": 7}}
	}
	t.Run("held by a typed List", func(t *testing.T) {
		held := own()
		inner := &unstructured.UnstructuredList{Object: held}
		outer := &typedList{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}, Items: []runtime.Object{inner}}
		outer.Name, outer.Namespace = "holder", "default"
		ml := &layout.ManifestLayout{Name: "root", Resources: []client.Object{outer}}
		undo, err := normalizeCallerObjects(ml)
		if err != nil {
			t.Fatalf("got %v, want the List normalized", err)
		}
		if got := inner.Object["metadata"].(map[string]any)["x"]; got != int64(7) {
			t.Errorf("the List's own map holds %T(%v), want an int64 7", got, got)
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("DeepCopy of the List panicked: %v", r)
				}
			}()
			_ = inner.DeepCopy()
		}()
		undo()
		if !sameMap(inner.Object, held) {
			t.Errorf("the undo did not put the caller's map back")
		}
		if got := held["metadata"].(map[string]any)["x"]; got != 7 {
			t.Errorf("the caller's map holds %T(%v), want the int it was given", got, got)
		}
	})
	t.Run("held by a typed List in a typed List", func(t *testing.T) {
		inner := &unstructured.UnstructuredList{Object: own()}
		middle := &typedList{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}, Items: []runtime.Object{inner}}
		outer := &typedList{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}, Items: []runtime.Object{middle}}
		outer.Name, outer.Namespace = "holder", "default"
		ml := &layout.ManifestLayout{Name: "root", Resources: []client.Object{outer}}
		if _, err := normalizeCallerObjects(ml); err != nil {
			t.Fatalf("got %v, want the List normalized", err)
		}
		if got := inner.Object["metadata"].(map[string]any)["x"]; got != int64(7) {
			t.Errorf("the List's own map holds %T(%v), want an int64 7", got, got)
		}
	})
	t.Run("a value that cannot be converted refuses the call", func(t *testing.T) {
		bad := own()
		bad["metadata"].(map[string]any)["x"] = make(chan int)
		inner := &unstructured.UnstructuredList{Object: bad}
		outer := &typedList{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}, Items: []runtime.Object{inner}}
		outer.Name, outer.Namespace = "holder", "default"
		ml := &layout.ManifestLayout{Name: "root", Resources: []client.Object{outer}}
		_, err := normalizeCallerObjects(ml)
		if err == nil || !strings.Contains(err.Error(), "chan int") {
			t.Errorf("got %v, want a refusal that names the chan int", err)
		}
	})
}

// scalarObject is a named scalar that is an object through its pointer.
type scalarObject int8

func (*scalarObject) GetObjectKind() schema.ObjectKind { return schema.EmptyObjectKind }
func (s *scalarObject) DeepCopyObject() runtime.Object { c := *s; return &c }

// scalarItems is a typed List whose items are scalars that are objects.
type scalarItems struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             []scalarObject `json:"items"`
}

func (l *scalarItems) DeepCopyObject() runtime.Object { c := *l; return &c }

// A slice of scalars that are objects is not skipped by the walk: the reader
// of the List makes a slot for each, so each is counted against the bound, and
// a slice over it is refused before the reader runs. An ordinary scalar slice
// stays skipped.
func TestWalkHeld_ScalarItemsThatAreObjectsAreCountedAgainstTheBound(t *testing.T) {
	defer SetMaxJSONNodesForTest(1 << 12)()
	meta := metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}
	over := &scalarItems{TypeMeta: meta, Items: make([]scalarObject, 1<<13)}
	if _, _, err := walkHeld(over, &convBudget{}, false); err == nil || !strings.Contains(err.Error(), "more than") {
		t.Errorf("got %v, want a refusal with the value limit", err)
	}
	under := &scalarItems{TypeMeta: meta, Items: make([]scalarObject, 1<<10)}
	if _, _, err := walkHeld(under, &convBudget{}, false); err != nil {
		t.Errorf("a slice under the bound: got %v, want it accepted", err)
	}
	plain := &plainScalarItems{TypeMeta: meta, Items: make([]int8, 1<<13)}
	if _, _, err := walkHeld(plain, &convBudget{}, false); err != nil {
		t.Errorf("an ordinary scalar slice: got %v, want it skipped", err)
	}
}

// plainScalarItems holds a slice of an ordinary scalar, which is no object.
type plainScalarItems struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             []int8 `json:"items"`
}

func (l *plainScalarItems) DeepCopyObject() runtime.Object { c := *l; return &c }
