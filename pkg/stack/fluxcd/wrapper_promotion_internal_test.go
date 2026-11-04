package fluxcd

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack/layout"
)

// nestedEmbed embeds the *Unstructured one level down.
type nestedEmbed struct{ *unstructured.Unstructured }

// directAndNested embeds the *Unstructured directly and through nestedEmbed.
// Go promotes the methods of the shallower, direct one.
type directAndNested struct {
	nestedEmbed
	*unstructured.Unstructured
}

// sideA and sideB each embed an *Unstructured; ambiguous embeds both at the
// same depth, so Go promotes the methods of neither and the type declares them.
type (
	sideA     struct{ *unstructured.Unstructured }
	sideB     struct{ *unstructured.Unstructured }
	ambiguous struct {
		sideA
		sideB
	}
)

func (a *ambiguous) GetObjectKind() schema.ObjectKind { return schema.EmptyObjectKind }
func (a *ambiguous) DeepCopyObject() runtime.Object   { c := *a; return &c }

// outerOfOwn embeds ownCopy, which declares its own DeepCopyObject; the
// compiler writes the DeepCopyObject of outerOfOwn, a promotion of that one.
type outerOfOwn struct{ ownCopy }

// withUnrelated declares a method that is none of the object's.
type withUnrelated struct{ *unstructured.Unstructured }

func (withUnrelated) Describe() string { return "unrelated" }

type outerOfUnrelated struct{ withUnrelated }

func wrapperObj(n any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]any{"name": "cm", "namespace": "default"},
		"spec":     map[string]any{"n": n},
	}}
}

// listObj is an unstructured List with one ConfigMap item that holds n.
func listObj(n any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "List",
		"items": []any{wrapperObj(n).Object},
	}}
}

func inList(items ...runtime.Object) *wrapperList {
	l := &wrapperList{Items: items}
	l.Kind, l.APIVersion = "List", "example.com/v1"
	l.Name, l.Namespace = "holder", "default"
	return l
}

// The wrapper's embedded object is the one Go promotes the methods of: the
// shallowest, and a tie at that depth is refused with both paths named
// (round 26).
func TestEmbeddedUnstructured_FollowsGoPromotion(t *testing.T) {
	run := func(t *testing.T, r client.Object) error {
		t.Helper()
		return normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{r}})
	}
	t.Run("the direct embed decides: it holds a plain int", func(t *testing.T) {
		w := &directAndNested{nestedEmbed{wrapperObj(int64(1))}, wrapperObj(7)}
		err := run(t, w)
		if err == nil || !strings.Contains(err.Error(), "fluxcd.directAndNested") {
			t.Fatalf("got %v, want a refusal naming fluxcd.directAndNested", err)
		}
	})
	t.Run("the direct embed decides: it is clean, the nested one is not", func(t *testing.T) {
		w := &directAndNested{nestedEmbed{wrapperObj(7)}, wrapperObj(int64(1))}
		if err := run(t, w); err != nil {
			t.Fatalf("got %v, want the wrapper kept", err)
		}
	})
	t.Run("two embeds at the same depth are refused, naming both paths", func(t *testing.T) {
		_, _, err := embeddedUnstructured(&ambiguous{sideA{wrapperObj(int64(1))}, sideB{wrapperObj(int64(1))}})
		if err == nil || !strings.Contains(err.Error(), "sideA.Unstructured") || !strings.Contains(err.Error(), "sideB.Unstructured") {
			t.Fatalf("got %v, want a refusal naming sideA.Unstructured and sideB.Unstructured", err)
		}
		l := &layout.ManifestLayout{Name: "l", Resources: []client.Object{inList(&ambiguous{sideA{wrapperObj(int64(1))}, sideB{wrapperObj(int64(1))}})}}
		if err := normalizeForTest(l); err == nil || !strings.Contains(err.Error(), "sideA.Unstructured") {
			t.Fatalf("as an item of a List: got %v, want the same refusal", err)
		}
	})
}

// An embedded struct on the way to the object that declares a DeepCopyObject of
// its own makes the wrapper's DeepCopyObject caller code, though the compiler
// wrote the wrapper's own (round 26).
func TestEmbeddedUnstructured_IntermediateWithItsOwnMethod(t *testing.T) {
	t.Run("an intermediate DeepCopyObject of its own is refused", func(t *testing.T) {
		w := &outerOfOwn{ownCopy{wrapperObj(int64(1))}}
		err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{w}})
		if err == nil || !strings.Contains(err.Error(), "DeepCopyObject of its own") {
			t.Fatalf("got %v, want a refusal naming the DeepCopyObject of its own", err)
		}
	})
	t.Run("a value-receiver DeepCopyObject of its own is refused when the wrapper is held by pointer", func(t *testing.T) {
		w := &ownCopy{wrapperObj(int64(1))}
		err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{w}})
		if err == nil || !strings.Contains(err.Error(), "DeepCopyObject of its own") {
			t.Fatalf("got %v, want a refusal naming the DeepCopyObject of its own", err)
		}
	})
	t.Run("an intermediate that declares an unrelated method is kept", func(t *testing.T) {
		w := &outerOfUnrelated{withUnrelated{wrapperObj(int64(1))}}
		if err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{w}}); err != nil {
			t.Fatalf("got %v, want the wrapper kept", err)
		}
	})
}

// A wrapper around an unstructured List is a wrapper wherever it is held, also
// as an item of a typed List, which reads its items as copies (round 26).
func TestHeldWrapper_OverAnUnstructuredList(t *testing.T) {
	t.Run("a wrapper over a List in a typed List holds a plain int and is refused", func(t *testing.T) {
		l := &layout.ManifestLayout{Name: "l", Resources: []client.Object{inList(promotesTrusted{listObj(7)})}}
		err := normalizeForTest(l)
		if err == nil || !strings.Contains(err.Error(), "fluxcd.promotesTrusted") {
			t.Fatalf("got %v, want a refusal naming fluxcd.promotesTrusted", err)
		}
	})
	t.Run("a wrapper over a clean List in a typed List is kept", func(t *testing.T) {
		l := &layout.ManifestLayout{Name: "l", Resources: []client.Object{inList(promotesTrusted{listObj(int64(7))})}}
		if err := normalizeForTest(l); err != nil {
			t.Fatalf("got %v, want the wrapper kept", err)
		}
	})
	t.Run("the same as a plain Unstructured List is normalized", func(t *testing.T) {
		u := listObj(7)
		l := &layout.ManifestLayout{Name: "l", Resources: []client.Object{inList(u)}}
		if err := normalizeForTest(l); err != nil {
			t.Fatal(err)
		}
		item := u.Object["items"].([]any)[0].(map[string]any)
		if n := item["spec"].(map[string]any)["n"]; n != int64(7) {
			t.Errorf("the item's spec.n = %#v, want int64(7)", n)
		}
	})
}

// privInner is an unexported struct that embeds the *Unstructured; privOuter
// embeds it by pointer, so the intermediate field cannot be interfaced.
type privInner struct{ *unstructured.Unstructured }
type privOuter struct{ *privInner }

// listWrapper embeds an unstructured.UnstructuredList, whose items are held by
// value in Items.
type listWrapper struct{ *unstructured.UnstructuredList }

// realList is an unstructured.UnstructuredList with one ConfigMap item that
// holds n.
func realList(n any) *unstructured.UnstructuredList {
	return &unstructured.UnstructuredList{
		Object: map[string]any{"apiVersion": "v1", "kind": "List"},
		Items:  []unstructured.Unstructured{*wrapperObj(n)},
	}
}

// A wrapper whose embedded object is reached through an unexported
// intermediate pointer is judged like any other: kept when clean, refused when
// not, and never refused for being unreachable (round 27).
func TestEmbeddedUnstructured_ThroughAnUnexportedIntermediate(t *testing.T) {
	run := func(r client.Object) error {
		return normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{r}})
	}
	t.Run("a clean wrapper is kept", func(t *testing.T) {
		if err := run(&privOuter{&privInner{wrapperObj(int64(1))}}); err != nil {
			t.Fatalf("got %v, want the wrapper kept", err)
		}
	})
	t.Run("a wrapper that holds a plain int is refused, naming its type", func(t *testing.T) {
		err := run(&privOuter{&privInner{wrapperObj(7)}})
		if err == nil || !strings.Contains(err.Error(), "fluxcd.privOuter") || strings.Contains(err.Error(), "cannot be reached") {
			t.Fatalf("got %v, want a refusal naming fluxcd.privOuter for the value", err)
		}
	})
}

// An embedded unstructured.UnstructuredList is a wrapper too: its items are
// held by value, and a read of a typed List that holds it returns copies, so it
// is refused when an item needs conversion and kept when none does (round 27).
func TestHeldWrapper_OverAnUnstructuredListType(t *testing.T) {
	t.Run("an item with a plain int is refused, naming the wrapper", func(t *testing.T) {
		l := &layout.ManifestLayout{Name: "l", Resources: []client.Object{inList(listWrapper{realList(7)})}}
		err := normalizeForTest(l)
		if err == nil || !strings.Contains(err.Error(), "fluxcd.listWrapper") {
			t.Fatalf("got %v, want a refusal naming fluxcd.listWrapper", err)
		}
	})
	t.Run("clean items are kept", func(t *testing.T) {
		l := &layout.ManifestLayout{Name: "l", Resources: []client.Object{inList(listWrapper{realList(int64(7))})}}
		if err := normalizeForTest(l); err != nil {
			t.Fatalf("got %v, want the wrapper kept", err)
		}
	})
	t.Run("a plain UnstructuredList with a plain int is still converted", func(t *testing.T) {
		ul := realList(7)
		l := &layout.ManifestLayout{Name: "l", Resources: []client.Object{inList(ul)}}
		if err := normalizeForTest(l); err != nil {
			t.Fatal(err)
		}
		if n := ul.Items[0].Object["spec"].(map[string]any)["n"]; n != int64(7) {
			t.Errorf("the item's spec.n = %#v, want int64(7)", n)
		}
	})
}
