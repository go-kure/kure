package fluxcd

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
// shallowest, and a tie at that depth is refused with both paths named.
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
// wrote the wrapper's own.
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
// as an item of a typed List, which reads its items as copies.
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
// not, and never refused for being unreachable.
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
// is refused when an item needs conversion and kept when none does.
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

// recursiveTyped embeds its own type by pointer, next to a typed object and no
// unstructured one. Its embedding is finite (the pointer is nil), and the type
// holds nothing to check, so it is no concern of the wrapper check.
type recursiveTyped struct {
	*corev1.ConfigMap
	*recursiveTyped
}

func TestEmbeddedUnstructured_RecursiveTypeWithoutAnUnstructuredObjectIsNoWrapper(t *testing.T) {
	r := &recursiveTyped{ConfigMap: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "default"}}, recursiveTyped: nil}
	objs, _, err := embeddedUnstructured(r)
	if err != nil || len(objs) != 0 {
		t.Fatalf("got %v, %v; want no wrapper and no error", objs, err)
	}
}

// shared is reached through two parents at the same depth: one type, two paths,
// which is still an ambiguity.
type (
	shared  struct{ *unstructured.Unstructured }
	parentA struct{ shared }
	parentB struct{ shared }
	viaTwo  struct {
		parentA
		parentB
	}
)

func (v *viaTwo) GetObjectKind() schema.ObjectKind { return schema.EmptyObjectKind }
func (v *viaTwo) DeepCopyObject() runtime.Object   { c := *v; return &c }

func TestEmbeddedUnstructured_OneTypeReachedByTwoPathsAtTheSameDepthIsAmbiguous(t *testing.T) {
	_, _, err := embeddedUnstructured(&viaTwo{parentA{shared{wrapperObj(int64(1))}}, parentB{shared{wrapperObj(int64(1))}}})
	if err == nil || !strings.Contains(err.Error(), "parentA.shared.Unstructured") || !strings.Contains(err.Error(), "parentB.shared.Unstructured") {
		t.Fatalf("got %v, want a refusal naming both paths", err)
	}
}

// deepCopier is an interface with a DeepCopyObject. A struct that embeds it
// takes its DeepCopyObject from it when the embed is shallower than the
// unstructured object it also embeds.
type deepCopier interface{ DeepCopyObject() runtime.Object }

// viaInterface embeds the *Unstructured two levels down and a deepCopier one
// level down: Go promotes DeepCopyObject from the interface, not from the object.
type viaInterface struct {
	nestedEmbed
	deepCopier
}

// withClientObject embeds the whole client.Object interface beside the nested
// object, which has a DeepCopyObject by embedding runtime.Object.
type withClientObject struct {
	nestedEmbed
	client.Object
}

// withStringer embeds an interface with no DeepCopyObject.
type withStringer struct {
	nestedEmbed
	fmt.Stringer
}

// copyByValue and copyByPointer are named non-struct types with a
// DeepCopyObject, declared with a value and a pointer receiver.
type (
	copyByValue   map[string]string
	copyByPointer []string
)

func (copyByValue) DeepCopyObject() runtime.Object    { return nil }
func (*copyByPointer) DeepCopyObject() runtime.Object { return nil }

type (
	viaNamedValue struct {
		nestedEmbed
		copyByValue
	}
	viaNamedPointer struct {
		nestedEmbed
		copyByPointer
	}
)

// holdsCopier is a struct whose DeepCopyObject is promoted from the interface
// it embeds, so the compiler wrote it and the source declares none.
type holdsCopier struct{ deepCopier }

// deeperProvider embeds the *Unstructured directly and the deepCopier one level
// further down, through holdsCopier: Go promotes DeepCopyObject from the object.
type deeperProvider struct {
	*unstructured.Unstructured
	holdsCopier
}

// ownCopyStruct is a struct that declares its own DeepCopyObject.
type ownCopyStruct struct{ n int } //nolint:unused // read by reflection in the embed search

func (ownCopyStruct) DeepCopyObject() runtime.Object { return nil }

// offPathOwn embeds the object two levels down and, beside the path to it, a
// struct that declares its own DeepCopyObject one level down.
type offPathOwn struct {
	nestedEmbed
	ownCopyStruct
}

// offPathPromoted embeds the object three levels down and, beside the path to
// it, a struct one level down whose DeepCopyObject is promoted from an
// interface: Go promotes DeepCopyObject from that interface, two levels down.
type (
	nestedTwice     struct{ nestedEmbed }
	offPathPromoted struct {
		nestedTwice
		holdsCopier
	}
)

// offPathUnrelated embeds the object two levels down and, beside the path to
// it, a struct that declares a method that is none of the object's.
type (
	unrelatedMethod  struct{ n int } //nolint:unused // read by reflection in the embed search
	offPathUnrelated struct {
		nestedEmbed
		unrelatedMethod
	}
)

func (unrelatedMethod) Describe() string { return "unrelated" }

// ownCopyBelow holds ownCopyStruct, so its DeepCopyObject is a promotion; the
// object it sits beside is shallower than the struct that declares it.
type (
	ownCopyBelow struct{ ownCopyStruct }
	deeperStruct struct {
		*unstructured.Unstructured
		ownCopyBelow
	}
)

// A wrapper whose DeepCopyObject Go promotes from an embedded interface, or from
// a named type that is no struct, at or above the object's depth, calls caller
// code on the layout's copy and is refused whatever its map holds. One whose
// object is shallower than every such embed keeps the object's own.
func TestEmbeddedUnstructured_DeepCopyObjectFromAnEmbeddedProvider(t *testing.T) {
	run := func(r client.Object) error {
		return normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{r}})
	}
	// The map holds a clean value, so only the type decides.
	one := int64(1)
	for _, tc := range []struct {
		name     string
		r        client.Object
		typ      string
		provider string
	}{
		{"an embedded interface that is nil", &viaInterface{nestedEmbed{wrapperObj(one)}, nil}, "fluxcd.viaInterface", "deepCopier"},
		{"an embedded interface that holds a copier", &viaInterface{nestedEmbed{wrapperObj(one)}, wrapperObj(one)}, "fluxcd.viaInterface", "deepCopier"},
		{"an embedded client.Object", &withClientObject{nestedEmbed{wrapperObj(one)}, nil}, "fluxcd.withClientObject", "Object"},
		{"a named non-struct type, value receiver", &viaNamedValue{nestedEmbed{wrapperObj(one)}, nil}, "fluxcd.viaNamedValue", "copyByValue"},
		{"a named non-struct type, pointer receiver", &viaNamedPointer{nestedEmbed{wrapperObj(one)}, nil}, "fluxcd.viaNamedPointer", "copyByPointer"},
		{"an off-path struct that declares one", &offPathOwn{nestedEmbed{wrapperObj(one)}, ownCopyStruct{}}, "fluxcd.offPathOwn", "ownCopyStruct"},
		{"an off-path struct that promotes one from an interface", &offPathPromoted{nestedTwice{nestedEmbed{wrapperObj(one)}}, holdsCopier{}}, "fluxcd.offPathPromoted", "holdsCopier.deepCopier"},
	} {
		t.Run("refused: "+tc.name, func(t *testing.T) {
			err := run(tc.r)
			want := tc.provider + ", which it embeds"
			if err == nil || !strings.Contains(err.Error(), tc.typ) || !strings.Contains(err.Error(), want) {
				t.Fatalf("got %v, want a refusal naming %s and %q", err, tc.typ, want)
			}
		})
	}
	// The object is shallower than every other DeepCopyObject, or the embed has none.
	for _, tc := range []struct {
		name string
		r    client.Object
	}{
		{"an embedded interface with no DeepCopyObject", &withStringer{nestedEmbed{wrapperObj(one)}, nil}},
		{"a provider deeper than the object", &deeperProvider{wrapperObj(one), holdsCopier{}}},
		{"a struct that declares an unrelated method", &offPathUnrelated{nestedEmbed{wrapperObj(one)}, unrelatedMethod{}}},
		{"a struct that promotes a DeepCopyObject deeper than the object", &deeperStruct{wrapperObj(one), ownCopyBelow{}}},
	} {
		t.Run("kept: "+tc.name, func(t *testing.T) {
			if err := run(tc.r); err != nil {
				t.Fatalf("refused: %v", err)
			}
		})
	}
}

// sameDepthCopier embeds the object and an interface at one depth: Go promotes
// DeepCopyObject from neither, so the type is no runtime.Object, and the search
// still returns the interface as a provider at the object's depth.
type sameDepthCopier struct {
	*unstructured.Unstructured
	deepCopier //nolint:unused // an embedded field the embed search reads by reflection
}

type label string

// otherHolder and twoHolders reach the one interface type deepCopier through two
// structs at one depth: the search keeps one path to it.
type (
	otherHolder struct{ deepCopier } //nolint:unused // an embedded type the embed search reads by reflection
	twoHolders  struct {
		nestedTwice
		holdsCopier //nolint:unused // an embedded field the embed search reads by reflection
		otherHolder //nolint:unused // an embedded field the embed search reads by reflection
	}
)

type withLabel struct {
	nestedEmbed
	label //nolint:unused // an embedded field the embed search reads by reflection
}

func TestShallowestEmbeds_ProvidersAtOrAboveTheObject(t *testing.T) {
	paths := func(steps []embedStep) []string {
		var out []string
		for _, s := range steps {
			out = append(out, s.path)
		}
		return out
	}
	for _, tc := range []struct {
		name string
		typ  reflect.Type
		want []string
	}{
		{"an interface above the object", reflect.TypeFor[viaInterface](), []string{"deepCopier"}},
		{"an interface at the object's depth", reflect.TypeFor[sameDepthCopier](), []string{"deepCopier"}},
		{"an interface below the object", reflect.TypeFor[deeperProvider](), nil},
		{"a named non-struct type above the object", reflect.TypeFor[viaNamedPointer](), []string{"copyByPointer"}},
		{"an interface held by a struct above the object", reflect.TypeFor[offPathPromoted](), []string{"holdsCopier.deepCopier"}},
		{"a named type with no DeepCopyObject is still listed", reflect.TypeFor[withLabel](), []string{"label"}},
		{"one interface type reached by two paths is listed once", reflect.TypeFor[twoHolders](), []string{"holdsCopier.deepCopier"}},
		{"no provider", reflect.TypeFor[nestedEmbed](), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			found, _, providers := shallowestEmbeds(tc.typ)
			if len(found) != 1 {
				t.Fatalf("found %d unstructured objects, want 1", len(found))
			}
			if got := paths(providers); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("providers %v, want %v", got, tc.want)
			}
		})
	}
}

func TestProviderHasDeepCopyObject(t *testing.T) {
	for _, tc := range []struct {
		name string
		typ  reflect.Type
		want bool
	}{
		{"an interface with it", reflect.TypeFor[deepCopier](), true},
		{"client.Object", reflect.TypeFor[client.Object](), true},
		{"an interface without it", reflect.TypeFor[fmt.Stringer](), false},
		{"a value-receiver method on a named type", reflect.TypeFor[copyByValue](), true},
		{"a pointer-receiver method on a named type", reflect.TypeFor[copyByPointer](), true},
		{"a named type with no methods", reflect.TypeFor[label](), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := providerHasDeepCopyObject(tc.typ); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
