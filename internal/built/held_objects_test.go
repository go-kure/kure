package built

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// HeldObjects returns every item of a typed List that is no unstructured object,
// Lists included, also those of the Lists it holds, so that the caller can look
// for a wrapper that embeds an unstructured object or an unstructured List.
func TestHeldObjects_ReturnsEveryItemThatIsNoUnstructuredObject(t *testing.T) {
	cm := configMap("a")
	other := &unwrittenType{}
	inner := &objectItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}, Items: []runtime.Object{other}}
	outer := &objectItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}, Items: []runtime.Object{cm, inner}}
	items, others, err := HeldObjects(outer)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0] != cm {
		t.Errorf("items = %v, want the one unstructured item", items)
	}
	if len(others) != 2 || others[0] != runtime.Object(inner) || others[1] != runtime.Object(other) {
		t.Errorf("others = %v, want the inner List, then its item", others)
	}
	if got, err := HeldItems(outer); err != nil || len(got) != 1 {
		t.Errorf("HeldItems = %v, %v; want the unstructured item alone", got, err)
	}
}
