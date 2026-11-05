package fluxcd

import (
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack/layout"
)

// A typed object that is no List may hold an Unstructured in a field, a
// runtime.RawExtension's Object included. A copy of the typed object follows
// it, and apimachinery's copy of an Unstructured panics on a value that is not
// JSON typed, so the held object is normalized as the items of a typed List are
// (go-kure/kure#1252).

// fieldHolder holds an Unstructured by pointer, by value, in a map and in an
// interface.
type fieldHolder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Ptr               *unstructured.Unstructured           `json:"ptr,omitempty"`
	Val               unstructured.Unstructured            `json:"val,omitempty"`
	ByKey             map[string]unstructured.Unstructured `json:"byKey,omitempty"`
	Any               runtime.Object                       `json:"any,omitempty"`
	Iface             any                                  `json:"iface,omitempty"`
}

func (h *fieldHolder) DeepCopyObject() runtime.Object { c := *h; return &c }

func heldCM(n any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]any{"name": "cm", "namespace": "default"},
		"spec":     map[string]any{"n": n},
	}}
}

func revisionHolding(u *unstructured.Unstructured, raw string) *appsv1.ControllerRevision {
	cr := &appsv1.ControllerRevision{Data: runtime.RawExtension{Raw: []byte(raw), Object: u}}
	cr.Name, cr.Namespace = "held", "default"
	return cr
}

func holderOf(f func(h *fieldHolder)) *fieldHolder {
	h := &fieldHolder{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Holder"}}
	h.Name, h.Namespace = "holder", "default"
	f(h)
	return h
}

// specN returns the value of spec.n of u.
func specN(u *unstructured.Unstructured) any {
	spec, _ := u.Object["spec"].(map[string]any)
	return spec["n"]
}

// An unstructured object held in a field of a typed object that needs
// conversion is checked by the walk and then converted, so each of its values
// counts twice against the call's budget of maxJSONNodes (2^22) values: the
// limit for it is about 2^21 values, where the same object as a resource of its
// own, charged once, passes. The rows sit either side of 2^21 = 2,097,152, with
// a margin for the few values around the list.
func TestNormalizeCallerObjects_HeldUnstructuredCountsTwiceAgainstBudget(t *testing.T) {
	t.Parallel()
	withValues := func(n int) *unstructured.Unstructured {
		vals := make([]any, n)
		for i := range vals {
			vals[i] = i // a plain int, so the object needs conversion
		}
		u := heldCM(0)
		u.Object["spec"] = map[string]any{"n": vals}
		return u
	}
	const below, above = 2_000_000, 2_200_000
	held := func(n int) error {
		return normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{revisionHolding(withValues(n), "{}")}})
	}
	if err := held(below); err != nil {
		t.Errorf("a held object of %d values: %v, want it accepted", below, err)
	}
	if err := held(above); err == nil || !strings.Contains(err.Error(), "values") {
		t.Errorf("a held object of %d values: %v, want the budget refusal", above, err)
	}
	if err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{withValues(above)}}); err != nil {
		t.Errorf("the same object as a resource of its own, charged once: %v, want it accepted", err)
	}
}

func TestNormalizeCallerObjects_HeldUnstructuredInField(t *testing.T) {
	t.Parallel()
	t.Run("plain int in a RawExtension is replaced by an int64", func(t *testing.T) {
		t.Parallel()
		u := heldCM(7)
		held := u.Object
		undo, err := normalizeCallerObjects(&layout.ManifestLayout{Name: "l", Resources: []client.Object{revisionHolding(u, "{}")}})
		if err != nil {
			t.Fatal(err)
		}
		if got, ok := specN(u).(int64); !ok || got != 7 {
			t.Fatalf("spec.n = %#v, want int64(7)", specN(u))
		}
		// The caller's own map is not changed: the object holds a copy, and the
		// undo puts the caller's map back.
		if got := specN(&unstructured.Unstructured{Object: held}); got != 7 {
			t.Errorf("the caller's map was changed: spec.n = %#v", got)
		}
		undo()
		if sameMap(u.Object, held) == false {
			t.Error("undo did not put the caller's map back")
		}
	})
	t.Run("the copy of the held object no longer panics", func(t *testing.T) {
		t.Parallel()
		u := heldCM(7)
		cr := revisionHolding(u, "{}")
		if err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{cr}}); err != nil {
			t.Fatal(err)
		}
		if _, err := comparableContent(cr); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("int64 and a clean map are left as they are", func(t *testing.T) {
		t.Parallel()
		u := heldCM(int64(7))
		held := u.Object
		if err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{revisionHolding(u, "{}")}}); err != nil {
			t.Fatal(err)
		}
		if !sameMap(u.Object, held) {
			t.Error("a map that needs no change was replaced")
		}
	})
	t.Run("an object held twice is replaced once", func(t *testing.T) {
		t.Parallel()
		u := heldCM(7)
		h := holderOf(func(h *fieldHolder) { h.Ptr, h.Any = u, u })
		if err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{h}}); err != nil {
			t.Fatal(err)
		}
		if _, ok := specN(u).(int64); !ok {
			t.Errorf("spec.n = %#v, want an int64", specN(u))
		}
	})
	t.Run("a plain typed field and an interface field are normalized", func(t *testing.T) {
		t.Parallel()
		p, a := heldCM(1), heldCM(2)
		h := holderOf(func(h *fieldHolder) { h.Ptr, h.Any = p, a })
		if err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{h}}); err != nil {
			t.Fatal(err)
		}
		for _, u := range []*unstructured.Unstructured{p, a} {
			if _, ok := specN(u).(int64); !ok {
				t.Errorf("spec.n = %#v, want an int64", specN(u))
			}
		}
	})
	t.Run("an object held by value in a field of a pointer is normalized", func(t *testing.T) {
		t.Parallel()
		h := holderOf(func(h *fieldHolder) { h.Val = *heldCM(3) })
		if err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{h}}); err != nil {
			t.Fatal(err)
		}
		if _, ok := specN(&h.Val).(int64); !ok {
			t.Errorf("spec.n = %#v, want an int64", specN(&h.Val))
		}
	})
	t.Run("an object held by value in a map cannot be replaced, so it is refused", func(t *testing.T) {
		t.Parallel()
		h := holderOf(func(h *fieldHolder) { h.ByKey = map[string]unstructured.Unstructured{"a": *heldCM(3)} })
		err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{h}})
		for _, want := range []string{"by value", `layout "`, "the typed object.ByKey[key]"} {
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("got %v, want a refusal that names %q", err, want)
			}
		}
	})
	t.Run("an object held by value in an interface cannot be replaced, so it is refused", func(t *testing.T) {
		t.Parallel()
		h := holderOf(func(h *fieldHolder) { h.Iface = *heldCM(3) })
		err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{h}})
		for _, want := range []string{"by value", `layout "`, "the typed object.Iface"} {
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("got %v, want a refusal that names %q", err, want)
			}
		}
	})
	t.Run("an object held by value in a map that needs no change is accepted", func(t *testing.T) {
		t.Parallel()
		h := holderOf(func(h *fieldHolder) { h.ByKey = map[string]unstructured.Unstructured{"a": *heldCM(int64(3))} })
		if err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{h}}); err != nil {
			t.Error(err)
		}
	})
	t.Run("a later refusal puts the map back", func(t *testing.T) {
		t.Parallel()
		u := heldCM(7)
		bad := heldCM(1)
		bad.Object["spec"] = map[string]any{"c": make(chan int)}
		l := &layout.ManifestLayout{Name: "l", Resources: []client.Object{revisionHolding(u, "{}"), bad}}
		held := u.Object
		undo, err := normalizeCallerObjects(l)
		if err == nil {
			t.Fatal("want a refusal of the channel")
		}
		undo()
		if !sameMap(u.Object, held) {
			t.Error("a refused call did not leave the held object's map as the caller gave it")
		}
	})
	t.Run("a held map that holds itself is still refused", func(t *testing.T) {
		t.Parallel()
		u := heldCM(7)
		spec := map[string]any{}
		spec["self"] = spec
		u.Object["spec"] = spec
		err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{revisionHolding(u, "{}")}})
		if err == nil || !strings.Contains(err.Error(), "holds itself") {
			t.Errorf("got %v, want the refusal of a map that holds itself", err)
		}
	})
}

// heldList is a plain unstructured List with n in its own Object map.
func heldList(n any) *unstructured.UnstructuredList {
	return &unstructured.UnstructuredList{Object: map[string]any{"apiVersion": "v1", "kind": "List", "extra": n}}
}

// A plain unstructured List held in a field has an Object map of its own beside
// its items, and the copy of the List panics on a plain int in it as the copy of
// an Unstructured does (go-kure/kure#1252).
func TestNormalizeCallerObjects_HeldUnstructuredListInField(t *testing.T) {
	t.Parallel()
	revisionHoldingList := func(l *unstructured.UnstructuredList) *appsv1.ControllerRevision {
		cr := &appsv1.ControllerRevision{Data: runtime.RawExtension{Raw: []byte("{}"), Object: l}}
		cr.Name, cr.Namespace = "held", "default"
		return cr
	}
	t.Run("a plain int in its own map is replaced by an int64 and put back by the undo", func(t *testing.T) {
		t.Parallel()
		l := heldList(7)
		held := l.Object
		undo, err := normalizeCallerObjects(&layout.ManifestLayout{Name: "l", Resources: []client.Object{revisionHoldingList(l)}})
		if err != nil {
			t.Fatal(err)
		}
		if got, ok := l.Object["extra"].(int64); !ok || got != 7 {
			t.Fatalf("extra = %#v, want int64(7)", l.Object["extra"])
		}
		undo()
		if !sameMap(l.Object, held) {
			t.Error("undo did not put the caller's map back")
		}
	})
	t.Run("the copy of the held List no longer panics", func(t *testing.T) {
		t.Parallel()
		cr := revisionHoldingList(heldList(7))
		if err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{cr}}); err != nil {
			t.Fatal(err)
		}
		if _, err := comparableContent(cr); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("a clean map is left as it is", func(t *testing.T) {
		t.Parallel()
		l := heldList(int64(7))
		held := l.Object
		if err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{revisionHoldingList(l)}}); err != nil {
			t.Fatal(err)
		}
		if !sameMap(l.Object, held) {
			t.Error("a map that needs no change was replaced")
		}
	})
	t.Run("a List held by value in an interface cannot be replaced, so it is refused", func(t *testing.T) {
		t.Parallel()
		h := holderOf(func(h *fieldHolder) { h.Iface = *heldList(3) })
		err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{h}})
		for _, want := range []string{"by value", `layout "`, "the typed object.Iface"} {
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("got %v, want a refusal that names %q", err, want)
			}
		}
	})
	t.Run("a later refusal puts its map back", func(t *testing.T) {
		t.Parallel()
		l := heldList(7)
		bad := heldCM(1)
		bad.Object["spec"] = map[string]any{"c": make(chan int)}
		held := l.Object
		undo, err := normalizeCallerObjects(&layout.ManifestLayout{Name: "l", Resources: []client.Object{revisionHoldingList(l), bad}})
		if err == nil {
			t.Fatal("want a refusal of the channel")
		}
		undo()
		if !sameMap(l.Object, held) {
			t.Error("a refused call did not leave the held List's map as the caller gave it")
		}
	})
}

// rawItemList is a typed List whose items are raw extensions that hold objects.
type rawItemList struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             []runtime.RawExtension `json:"items"`
}

func (l *rawItemList) DeepCopyObject() runtime.Object { c := *l; return &c }

// hiddenFieldList is a typed List that writes its Items and holds an object in a
// field it does not write.
type hiddenFieldList struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             []*unstructured.Unstructured `json:"items"`
	Hidden            any                          `json:"-"`
}

func (l *hiddenFieldList) DeepCopyObject() runtime.Object { c := *l; return &c }

// valueItem is a runtime.Object by value that holds an Unstructured by value in
// a field it does not write.
type valueItem struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Held              unstructured.Unstructured `json:"-"`
}

func (v valueItem) GetObjectKind() schema.ObjectKind { return &v.TypeMeta }
func (v valueItem) DeepCopyObject() runtime.Object   { return v }

// valueItemList is a typed List whose Items hold the objects themselves.
type valueItemList struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             []valueItem `json:"items"`
}

func (l *valueItemList) DeepCopyObject() runtime.Object { c := *l; return &c }

// A typed List is read from the items it writes. What it holds in another field
// is read by nothing, so it is left as it is and refuses nothing. An item that
// is no List is copied whole as any other typed object is, so what it holds in a
// field is normalized, or refused, as it is for a resource of its own
// (go-kure/kure#1252).
func TestNormalizeCallerObjects_HeldUnstructuredInTypedList(t *testing.T) {
	t.Parallel()
	typedList := func(items ...runtime.Object) *rawItemList {
		l := &rawItemList{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}}
		l.Name, l.Namespace = "holder", "default"
		for _, o := range items {
			l.Items = append(l.Items, runtime.RawExtension{Object: o})
		}
		return l
	}
	t.Run("an item that is no List is normalized", func(t *testing.T) {
		t.Parallel()
		u := heldCM(7)
		cr := revisionHolding(u, "{}")
		l := typedList(cr)
		if err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{l}}); err != nil {
			t.Fatal(err)
		}
		if _, ok := specN(u).(int64); !ok {
			t.Errorf("spec.n = %#v, want an int64", specN(u))
		}
		if _, err := comparableContent(cr); err != nil {
			t.Errorf("the copy of the item: %v", err)
		}
	})
	t.Run("an item of a List held by the List is normalized", func(t *testing.T) {
		t.Parallel()
		u := heldCM(7)
		l := typedList(typedList(revisionHolding(u, "{}")))
		if err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{l}}); err != nil {
			t.Fatal(err)
		}
		if _, ok := specN(u).(int64); !ok {
			t.Errorf("spec.n = %#v, want an int64", specN(u))
		}
	})
	t.Run("an item that holds an object by value that cannot be replaced is refused", func(t *testing.T) {
		t.Parallel()
		l := typedList(holderOf(func(h *fieldHolder) { h.Iface = *heldCM(3) }))
		err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{l}})
		if err == nil || !strings.Contains(err.Error(), "by value") {
			t.Errorf("got %v, want a refusal naming the value", err)
		}
	})
	t.Run("an item held by value is normalized where it lies", func(t *testing.T) {
		t.Parallel()
		l := &valueItemList{
			TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"},
			Items:    []valueItem{{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Item"}, Held: *heldCM(7)}},
		}
		l.Name, l.Namespace = "holder", "default"
		if err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{l}}); err != nil {
			t.Fatal(err)
		}
		if _, ok := specN(&l.Items[0].Held).(int64); !ok {
			t.Errorf("spec.n = %#v, want an int64", specN(&l.Items[0].Held))
		}
	})
	t.Run("an object in a field the List does not write is left as it is", func(t *testing.T) {
		t.Parallel()
		hidden := heldCM(7)
		l := &hiddenFieldList{
			TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"},
			Items:    []*unstructured.Unstructured{heldCM(int64(1))},
			Hidden:   *hidden, // by value, with a plain int: refused if it were copied
		}
		l.Name, l.Namespace = "holder", "default"
		held := hidden.Object
		if err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{l}}); err != nil {
			t.Fatal(err)
		}
		if !sameMap(hidden.Object, held) {
			t.Error("an object the List does not write was replaced")
		}
	})
	t.Run("an object held by pointer in a field the List does not write is not replaced", func(t *testing.T) {
		t.Parallel()
		hidden := heldCM(7)
		l := &hiddenFieldList{
			TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"},
			Items:    []*unstructured.Unstructured{heldCM(int64(1))},
			Hidden:   hidden,
		}
		l.Name, l.Namespace = "holder", "default"
		held := hidden.Object
		if err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{l}}); err != nil {
			t.Fatal(err)
		}
		if !sameMap(hidden.Object, held) {
			t.Error("an object the List does not write was replaced")
		}
	})
}
