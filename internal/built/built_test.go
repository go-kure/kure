package built

import (
	"runtime/debug"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func configMap(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": name, "namespace": "default"},
	}}
}

func typedConfigMap(name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
	}
}

// listOf is an unstructured object of the given kind with items.
func listOf(kind string, items ...*unstructured.Unstructured) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": kind}}
	held := make([]any, 0, len(items))
	for _, it := range items {
		held = append(held, it.Object)
	}
	u.Object["items"] = held
	return u
}

// coreList is a core v1 List: its items are runtime.RawExtension values and
// it has no object metadata.
func coreList(items ...runtime.RawExtension) *metav1.List {
	return &metav1.List{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "List"}, Items: items}
}

func raw(json string) runtime.RawExtension { return runtime.RawExtension{Raw: []byte(json)} }

const rawConfigMap = `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"raw","namespace":"default"}}`

// optionalItems leaves its items out of what is written when it has none.
type optionalItems struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             []corev1.ConfigMap `json:"items,omitempty"`
}

func (o *optionalItems) DeepCopyObject() runtime.Object { c := *o; return &c }

// hiddenItems writes its items from a field apimachinery does not take for a
// list's items.
type hiddenItems struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Entries           []corev1.ConfigMap `json:"items"`
}

func (h *hiddenItems) DeepCopyObject() runtime.Object { c := *h; return &c }

// unwrittenItems has items apimachinery takes for a list's, and writes
// something else under items.
type unwrittenItems struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             []corev1.ConfigMap `json:"-"`
	Written           any                `json:"items"`
}

func (u *unwrittenItems) DeepCopyObject() runtime.Object { c := *u; return &c }

// unwritable cannot be marshalled.
type unwritable struct {
	metav1.TypeMeta `json:",inline"`
	Items           []corev1.ConfigMap `json:"items"`
	Bad             chan int           `json:"bad"`
}

func (u *unwritable) DeepCopyObject() runtime.Object { c := *u; return &c }

// described is what a test expects of one returned object.
type described struct {
	kind, name string
	read       bool
}

func describe(t *testing.T, objs []Object) []described {
	t.Helper()
	out := make([]described, 0, len(objs))
	for _, o := range objs {
		d := described{kind: o.GetObjectKind().GroupVersionKind().Kind, read: o.Read}
		switch v := o.Object.(type) {
		case *unstructured.Unstructured:
			d.name = v.GetName()
		case metav1.Object:
			d.name = v.GetName()
		}
		out = append(out, d)
	}
	return out
}

func TestObjects(t *testing.T) {
	typedList := func(kind string, items ...corev1.ConfigMap) *optionalItems {
		return &optionalItems{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: kind}, Items: items}
	}
	cases := map[string]struct {
		resource runtime.Object
		want     []described
	}{
		"an object":      {configMap("a"), []described{{"ConfigMap", "a", false}}},
		"a typed object": {typedConfigMap("a"), []described{{"ConfigMap", "a", false}}},

		"an unstructured List": {
			listOf("List", configMap("a"), configMap("b")),
			[]described{{"ConfigMap", "a", false}, {"ConfigMap", "b", false}},
		},
		"an unstructured List of another List kind": {
			listOf("ConfigMapList", configMap("a")),
			[]described{{"ConfigMap", "a", false}},
		},
		"an unstructured List in a List in a List": {
			listOf("List", configMap("a"), listOf("List", listOf("List", configMap("b")), configMap("c"))),
			[]described{{"ConfigMap", "a", false}, {"ConfigMap", "b", false}, {"ConfigMap", "c", false}},
		},
		"an empty unstructured List": {listOf("List"), nil},
		"an unstructured List whose items are null": {
			&unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "List", "items": nil}},
			nil,
		},
		"an unstructured List kind without items": {
			&unstructured.Unstructured{Object: map[string]any{"apiVersion": "example.com/v1", "kind": "ShoppingList", "metadata": map[string]any{"name": "s"}}},
			[]described{{"ShoppingList", "s", false}},
		},
		"an unstructured List whose items are not an array": {
			&unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "List", "metadata": map[string]any{"name": "s"}, "items": "none"}},
			[]described{{"List", "s", false}},
		},
		"an unstructured object of another kind with items": {
			func() runtime.Object {
				u := listOf("Inventory", configMap("a"))
				u.SetName("inv")
				return u
			}(),
			[]described{{"Inventory", "inv", false}},
		},

		"a typed List": {
			typedList("ConfigMapList", *typedConfigMap("a"), *typedConfigMap("b")),
			[]described{{"ConfigMap", "a", false}, {"ConfigMap", "b", false}},
		},
		"a typed List kind that leaves its empty items out": {
			func() runtime.Object {
				l := typedList("ShoppingList")
				l.Name = "s"
				return l
			}(),
			[]described{{"ShoppingList", "s", false}},
		},
		"a typed object of another kind with items": {
			func() runtime.Object {
				l := typedList("Inventory", *typedConfigMap("a"))
				l.Name = "inv"
				return l
			}(),
			[]described{{"Inventory", "inv", false}},
		},
		"a typed List that writes items its Go value does not give": {
			&hiddenItems{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMapList"}, Entries: []corev1.ConfigMap{*typedConfigMap("a")}},
			[]described{{"ConfigMap", "a", true}},
		},

		// The written form decides, as for an unstructured List: what the Go
		// value holds beside it is not built.
		"a typed List whose written items are null": {
			&unwrittenItems{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMapList"}, Items: []corev1.ConfigMap{*typedConfigMap("a")}},
			nil,
		},
		"a typed List whose written items are not an array": {
			&unwrittenItems{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMapList"},
				ObjectMeta: metav1.ObjectMeta{Name: "s"},
				Items:      []corev1.ConfigMap{*typedConfigMap("a")},
				Written:    "none",
			},
			[]described{{"ConfigMapList", "s", false}},
		},
		// An array written from another field than the one the Go value gives
		// its items in is what is built: the objects are read from it.
		"a typed List that writes other items than its Go value gives": {
			&unwrittenItems{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMapList"},
				Items:    []corev1.ConfigMap{*typedConfigMap("a")},
				Written:  []corev1.ConfigMap{*typedConfigMap("b"), *typedConfigMap("c")},
			},
			[]described{{"ConfigMap", "b", true}, {"ConfigMap", "c", true}},
		},
		"a typed List that writes one item other than its Go value gives": {
			&unwrittenItems{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMapList"},
				Items:    []corev1.ConfigMap{*typedConfigMap("a"), *typedConfigMap("b")},
				Written:  []corev1.ConfigMap{*typedConfigMap("a"), *typedConfigMap("c")},
			},
			[]described{{"ConfigMap", "a", true}, {"ConfigMap", "c", true}},
		},
		"a typed List that writes an empty array beside the items its Go value gives": {
			&unwrittenItems{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMapList"},
				Items:    []corev1.ConfigMap{*typedConfigMap("a")},
				Written:  []corev1.ConfigMap{},
			},
			nil,
		},
		// One List, held twice, is opened twice: that is no List holding itself.
		"a core List held twice by a core List": {
			func() runtime.Object {
				inner := coreList(runtime.RawExtension{Object: typedConfigMap("a")})
				return coreList(runtime.RawExtension{Object: inner}, runtime.RawExtension{Object: inner})
			}(),
			[]described{{"ConfigMap", "a", false}, {"ConfigMap", "a", false}},
		},
		// The raw JSON is what is written, so the object beside it is not
		// read, the List itself included.
		"a core List item with raw JSON and the List itself as the object": {
			func() runtime.Object {
				l := coreList()
				l.Items = []runtime.RawExtension{{Raw: []byte(rawConfigMap), Object: l}}
				return l
			}(),
			[]described{{"ConfigMap", "raw", true}},
		},
		"an unstructured List held twice by a List": {
			func() runtime.Object {
				inner := listOf("List", configMap("a"))
				return listOf("List", inner, inner)
			}(),
			[]described{{"ConfigMap", "a", false}, {"ConfigMap", "a", false}},
		},

		// A nil pointer is written as null, like an empty item.
		"a core List with an item that is a nil pointer": {
			coreList(runtime.RawExtension{Object: (*corev1.ConfigMap)(nil)}, runtime.RawExtension{Object: configMap("a")}),
			[]described{{"ConfigMap", "a", false}},
		},
		"a core List in a core List with an item that is a nil pointer": {
			coreList(runtime.RawExtension{Object: coreList(runtime.RawExtension{Object: (*metav1.List)(nil)}, runtime.RawExtension{Object: configMap("a")})}),
			[]described{{"ConfigMap", "a", false}},
		},

		"a core List of objects": {
			coreList(runtime.RawExtension{Object: configMap("a")}, runtime.RawExtension{Object: typedConfigMap("b")}),
			[]described{{"ConfigMap", "a", false}, {"ConfigMap", "b", false}},
		},
		"a core List with a raw JSON item": {
			coreList(runtime.RawExtension{Object: configMap("a")}, raw(rawConfigMap)),
			[]described{{"ConfigMap", "a", false}, {"ConfigMap", "raw", true}},
		},
		// The raw JSON is what is written when an item has both.
		"a core List item with raw JSON and an object": {
			coreList(runtime.RawExtension{Raw: []byte(rawConfigMap), Object: configMap("unwritten")}),
			[]described{{"ConfigMap", "raw", true}},
		},
		"a core List with an empty item and a null item": {
			coreList(runtime.RawExtension{}, raw("null"), runtime.RawExtension{Object: configMap("a")}),
			[]described{{"ConfigMap", "a", false}},
		},
		"a core List in a core List": {
			coreList(runtime.RawExtension{Object: coreList(runtime.RawExtension{Object: configMap("a")}, raw(rawConfigMap))}),
			[]described{{"ConfigMap", "a", false}, {"ConfigMap", "raw", true}},
		},
		"an unstructured List in a core List": {
			coreList(runtime.RawExtension{Object: listOf("List", configMap("a"))}),
			[]described{{"ConfigMap", "a", false}},
		},
		// What a copy holds is a copy.
		"a raw JSON List in a core List": {
			coreList(raw(`{"apiVersion":"v1","kind":"List","items":[` + rawConfigMap + `,{"apiVersion":"v1","kind":"List","items":[` + rawConfigMap + `]}]}`)),
			[]described{{"ConfigMap", "raw", true}, {"ConfigMap", "raw", true}},
		},
		"a raw JSON object of another kind with items": {
			coreList(raw(`{"apiVersion":"example.com/v1","kind":"Inventory","metadata":{"name":"inv"},"items":[` + rawConfigMap + `]}`)),
			[]described{{"Inventory", "inv", true}},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			objs, err := Objects(tc.resource)
			if err != nil {
				t.Fatalf("Objects: %v", err)
			}
			got := describe(t, objs)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("object %d: got %v, want %v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// An object that is not Read is the resource's own: a change to it is a
// change to the resource, wherever in the resource it sits.
func TestObjects_OwnObjectsAreTheResources(t *testing.T) {
	mark := func(t *testing.T, objs []Object) {
		t.Helper()
		for _, o := range objs {
			if o.Read {
				t.Fatalf("%v is read from the written form", o.Object)
			}
			switch v := o.Object.(type) {
			case *unstructured.Unstructured:
				v.SetLabels(map[string]string{"marked": "yes"})
			case metav1.Object:
				v.SetLabels(map[string]string{"marked": "yes"})
			default:
				t.Fatalf("%T has no object metadata", o.Object)
			}
		}
	}
	written := func(t *testing.T, r runtime.Object) string {
		t.Helper()
		m, err := writtenForm(r)
		if err != nil {
			t.Fatal(err)
		}
		u := unstructured.Unstructured{Object: m}
		data, err := u.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	for name, r := range map[string]runtime.Object{
		"an unstructured List in a List": listOf("List", listOf("List", configMap("a")), configMap("b")),
		"a typed List of structs": &optionalItems{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMapList"},
			Items:    []corev1.ConfigMap{*typedConfigMap("a"), *typedConfigMap("b")},
		},
		"a core List in a core List": coreList(
			runtime.RawExtension{Object: coreList(runtime.RawExtension{Object: typedConfigMap("a")})},
			runtime.RawExtension{Object: configMap("b")}),
	} {
		t.Run(name, func(t *testing.T) {
			objs, err := Objects(r)
			if err != nil {
				t.Fatal(err)
			}
			if len(objs) != 2 {
				t.Fatalf("%d objects, want 2", len(objs))
			}
			mark(t, objs)
			if got := strings.Count(written(t, r), `"marked":"yes"`); got != 2 {
				t.Errorf("the resource is written with %d marked objects, want 2", got)
			}
		})
	}
}

// A Read object is a copy: a change to it is not written.
func TestObjects_ReadObjectsAreCopies(t *testing.T) {
	r := coreList(raw(rawConfigMap))
	objs, err := Objects(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 1 || !objs[0].Read {
		t.Fatalf("got %v, want the one raw JSON item, read", describe(t, objs))
	}
	objs[0].Object.(*unstructured.Unstructured).SetLabels(map[string]string{"marked": "yes"})
	if string(r.Items[0].Raw) != rawConfigMap {
		t.Errorf("the raw JSON item changed: %s", r.Items[0].Raw)
	}
}

func TestObjects_Errors(t *testing.T) {
	cases := map[string]struct {
		resource runtime.Object
		want     string
	}{
		// The List cannot be written with it, which is found first.
		"a raw JSON item that is not JSON":      {coreList(raw(`{"kind":`)), "marshal the List"},
		"a raw JSON item that is not an object": {coreList(raw(`[1]`)), "read a raw JSON item"},
		"an unstructured List with an item that is not an object": {
			&unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "List", "items": []any{"none"}}},
			"items member is not an object",
		},
		"a typed List that cannot be written": {
			&unwritable{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMapList"}},
			"marshal the ConfigMapList",
		},
		"a typed List that cannot be written, in a List": {
			coreList(runtime.RawExtension{Object: &unwritable{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMapList"}}}),
			"marshal the",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			objs, err := Objects(tc.resource)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v and error %v, want an error containing %q", describe(t, objs), err, tc.want)
			}
		})
	}
}

// An unstructured List's items are read from its map, so one that holds
// itself, directly or through another, has no end to read: it is refused. So
// is a typed List that holds itself as an item's object, which has no end to
// marshal. The stack is kept small for the test, so that a reader that does
// not stop fails here and not after a gigabyte of it.
func TestObjects_ListThatHoldsItself(t *testing.T) {
	defer debug.SetMaxStack(debug.SetMaxStack(32 << 20))
	itself := listOf("List")
	itself.Object["items"] = []any{itself.Object}
	first, second := listOf("List"), listOf("List")
	first.Object["items"] = []any{second.Object}
	second.Object["items"] = []any{configMap("a").Object, first.Object}
	typed := coreList()
	typed.Items = []runtime.RawExtension{{Object: typed}}
	outer, inner := coreList(), coreList()
	outer.Items = []runtime.RawExtension{{Object: inner}}
	inner.Items = []runtime.RawExtension{{Object: typedConfigMap("a")}, {Object: outer}}
	const holdsItself = "a List holds itself among its items"
	for name, tc := range map[string]struct {
		resource runtime.Object
		want     string
	}{
		"a List among its own items":         {itself, holdsItself},
		"two Lists that hold each other":     {first, holdsItself},
		"such a List inside another, twice":  {listOf("List", itself, itself), holdsItself},
		"such a List behind an object in it": {listOf("List", configMap("a"), second), holdsItself},
		// A typed List is read from its written form, and marshalling it
		// finds the unstructured List inside it first.
		"such a List inside a core List": {coreList(runtime.RawExtension{Object: itself}), "encountered a cycle"},

		"a core List among its own items":     {typed, holdsItself},
		"two core Lists that hold each other": {outer, holdsItself},
		"such a core List inside another":     {coreList(runtime.RawExtension{Object: typed}), holdsItself},
		"such a core List behind an object":   {coreList(runtime.RawExtension{Object: typedConfigMap("a")}, runtime.RawExtension{Object: inner}), holdsItself},
	} {
		t.Run(name, func(t *testing.T) {
			objs, err := Objects(tc.resource)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v and error %v, want an error containing %q", describe(t, objs), err, tc.want)
			}
		})
	}
}

// A typed object without a kind is one object: there is no kind to take it
// for a List by.
func TestObjects_NoKind(t *testing.T) {
	r := &optionalItems{Items: []corev1.ConfigMap{*typedConfigMap("a")}}
	objs, err := Objects(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 1 || objs[0].Object != runtime.Object(r) {
		t.Errorf("got %v, want the object itself", describe(t, objs))
	}
	if gvk := objs[0].GetObjectKind().GroupVersionKind(); gvk != (schema.GroupVersionKind{}) {
		t.Errorf("kind %v, want none", gvk)
	}
}
