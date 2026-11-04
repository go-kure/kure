package fluxcd_test

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// A caller may build an unstructured object with a Go value that is not JSON
// typed, a plain int say. apimachinery panics on it in every Nested* read and
// in DeepCopy, so kure reads such an object as JSON would hold it, an int64,
// once, before any read, or refuses it (go-kure/kure#1201). The three
// placements below are the ones the issue names: a Kustomization's
// spec.sourceRef, a List item's spec and a Source's spec.

// integrateOutcome integrates the walked cluster and returns the error text
// ("" for none) and whatever the call panicked with.
func integrateOutcome(t *testing.T, c *stack.Cluster, placement layout.FluxPlacement) (errText string, panicked any) {
	t.Helper()
	defer func() { panicked = recover() }()
	if err := integrateWalked(t, c, placement, nil); err != nil {
		errText = err.Error()
	}
	return errText, nil
}

func TestCallerObject_NonJSONValueIsReadLikeItsJSON(t *testing.T) {
	kustomization := func(n any) client.Object {
		k := unstructuredObj("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux-system", "caller")
		k.Object["spec"] = map[string]any{
			"path":      "./elsewhere",
			"sourceRef": map[string]any{"kind": "GitRepository", "name": "repo", "extra": n},
		}
		return k
	}
	source := func(n any) client.Object {
		s := unstructuredObj("source.toolkit.fluxcd.io/v1", "GitRepository", "flux-system", "repo")
		s.Object["spec"] = map[string]any{"url": "https://example.invalid/repo", "extra": map[string]any{"n": n}}
		return s
	}
	listItem := func(n any) client.Object {
		return &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "List", "items": []any{
			map[string]any{
				"apiVersion": "kustomize.toolkit.fluxcd.io/v1", "kind": "Kustomization",
				"metadata": map[string]any{"name": "in-list", "namespace": "flux-system"},
				"spec": map[string]any{"path": "./elsewhere",
					"sourceRef": map[string]any{"kind": "GitRepository", "name": "repo", "extra": n}},
			},
			map[string]any{
				"apiVersion": "source.toolkit.fluxcd.io/v1", "kind": "GitRepository",
				"metadata": map[string]any{"name": "list-repo", "namespace": "flux-system"},
				"spec":     map[string]any{"extra": n},
			},
		}}}
	}
	placements := map[string]func(n any) client.Object{
		"a Kustomization's spec.sourceRef": kustomization,
		"a List item's spec":               listItem,
		"a Source's spec":                  source,
	}
	for name, obj := range placements {
		for _, placement := range append(slices.Clone(integratedPlacements), layout.FluxSeparate) {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				want, panicked := integrateOutcome(t, baseCluster(obj(int64(53))), placement)
				if panicked != nil {
					t.Fatalf("the int64 baseline panicked: %v", panicked)
				}
				for _, n := range []any{53, int32(53), uint(53), float32(53)} {
					got, panicked := integrateOutcome(t, baseCluster(obj(n)), placement)
					if panicked != nil {
						t.Errorf("%T %v: integration panicked: %v", n, n, panicked)
						continue
					}
					if got != want {
						t.Errorf("%T %v: got error %q, want the int64 outcome %q", n, n, got, want)
					}
				}
			})
		}
	}
}

// The other entry points that take a caller's layout read it as its JSON as
// well: the object held after the call holds an int64.
func TestCallerObject_OtherEntryPointsNormalize(t *testing.T) {
	entries := map[string]func(ml *layout.ManifestLayout, c *stack.Cluster) error{
		"UmbrellaTerminals": func(ml *layout.ManifestLayout, _ *stack.Cluster) error {
			_, err := fluxstack.UmbrellaTerminals(ml)
			return err
		},
		"ResourceGenerator.UmbrellaTerminals": func(ml *layout.ManifestLayout, _ *stack.Cluster) error {
			_, err := fluxstack.NewResourceGenerator().UmbrellaTerminals(ml)
			return err
		},
		"GenerateFromLayout": func(ml *layout.ManifestLayout, c *stack.Cluster) error {
			_, err := fluxstack.NewResourceGenerator().GenerateFromLayout(ml, c)
			return err
		},
		"WorkflowEngine.IntegrateWithLayout": func(ml *layout.ManifestLayout, c *stack.Cluster) error {
			return fluxstack.NewWorkflowEngine().IntegrateWithLayout(ml, c, withPlacement(nodeOnly, layout.FluxSeparate))
		},
	}
	for name, call := range entries {
		t.Run(name, func(t *testing.T) {
			s := unstructuredObj("source.toolkit.fluxcd.io/v1", "GitRepository", "flux-system", "repo")
			s.Object["spec"] = map[string]any{"url": "https://example.invalid/repo", "extra": 53}
			var obj client.Object = s
			app := stack.NewApplication("raw", "default", &fakeAppConfig{objs: []*client.Object{&obj}})
			c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", app)}}
			ml, err := layout.WalkCluster(c, withPlacement(nodeOnly, layout.FluxSeparate))
			if err != nil {
				t.Fatal(err)
			}
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("panicked: %v", r)
					}
				}()
				if err := call(ml, c); err != nil {
					t.Errorf("got %v, want none", err)
				}
			}()
			if v := s.Object["spec"].(map[string]any)["extra"]; v != int64(53) {
				t.Errorf("spec.extra = %#v after the call, want int64(53)", v)
			}
		})
	}
}

// A typed List returns the unstructured items it holds as they are, so those
// are the caller's objects too.
func TestCallerObject_TypedListItemIsReadLikeItsJSON(t *testing.T) {
	item := func(n any) *unstructured.Unstructured {
		k := unstructuredObj("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux-system", "caller")
		k.Object["spec"] = map[string]any{
			"path":      "./elsewhere",
			"sourceRef": map[string]any{"kind": "GitRepository", "name": "repo", "extra": n},
		}
		return k
	}
	for _, placement := range append(slices.Clone(integratedPlacements), layout.FluxSeparate) {
		t.Run(string(placement), func(t *testing.T) {
			want, panicked := integrateOutcome(t, baseCluster(typedListOf("List", item(int64(53)))), placement)
			if panicked != nil {
				t.Fatalf("the int64 baseline panicked: %v", panicked)
			}
			got, panicked := integrateOutcome(t, baseCluster(typedListOf("List", item(53))), placement)
			if panicked != nil {
				t.Fatalf("integration panicked: %v", panicked)
			}
			if got != want {
				t.Errorf("got error %q, want the int64 outcome %q", got, want)
			}
		})
	}
}

// A call that a later check refuses leaves the caller's objects as they were.
func TestCallerObject_RefusedCallPutsTheObjectsBack(t *testing.T) {
	handBuilt := func() (*layout.ManifestLayout, *unstructured.Unstructured, map[string]any) {
		cm := unstructuredObj("v1", "ConfigMap", "default", "cm")
		cm.Object["spec"] = map[string]any{"n": 53}
		return &layout.ManifestLayout{Name: "root", Resources: []client.Object{cm}}, cm, cm.Object
	}
	rules := withPlacement(nodeOnly, layout.FluxSeparate)
	calls := map[string]func(ml *layout.ManifestLayout) error{
		"IntegrateWithLayout": func(ml *layout.ManifestLayout) error {
			return fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, baseCluster(), rules)
		},
		"GenerateFromLayout": func(ml *layout.ManifestLayout) error {
			_, err := fluxstack.NewResourceGenerator().GenerateFromLayout(ml, baseCluster())
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			ml, cm, held := handBuilt()
			if err := call(ml); err == nil {
				t.Fatal("a hand-built tree was not refused")
			}
			if fmt.Sprintf("%p", cm.Object) != fmt.Sprintf("%p", held) {
				t.Error("the refused call left a new Object map on the caller's object")
			}
			if v := cm.Object["spec"].(map[string]any)["n"]; v != 53 {
				t.Errorf("spec.n = %#v after the refused call, want int 53", v)
			}
		})
	}
}

// UmbrellaTerminals reports a value that does not marshal as an error that
// wraps its sentinel.
func TestCallerObject_UmbrellaTerminalsRefusal(t *testing.T) {
	s := unstructuredObj("source.toolkit.fluxcd.io/v1", "GitRepository", "flux-system", "repo")
	s.Object["spec"] = map[string]any{"bad": func() {}}
	ml, err := layout.WalkCluster(baseCluster(s), withPlacement(nodeOnly, layout.FluxSeparate))
	if err != nil {
		t.Fatal(err)
	}
	_, err = fluxstack.UmbrellaTerminals(ml)
	if !errors.Is(err, fluxstack.ErrUmbrellaTerminals) || !strings.Contains(err.Error(), "spec.bad") {
		t.Errorf("got %v, want ErrUmbrellaTerminals naming spec.bad", err)
	}
}

// A value that does not marshal cannot be read as JSON would hold it: the
// call is refused with the object and the field named, and the caller's
// object is left as it was.
func TestCallerObject_UnmarshalableValueIsRefused(t *testing.T) {
	for _, placement := range append(slices.Clone(integratedPlacements), layout.FluxSeparate) {
		t.Run(string(placement), func(t *testing.T) {
			bad := make(chan int)
			s := unstructuredObj("source.toolkit.fluxcd.io/v1", "GitRepository", "flux-system", "repo")
			s.Object["spec"] = map[string]any{"url": "https://example.invalid/repo", "ref": map[string]any{"bad": bad}}
			got, panicked := integrateOutcome(t, baseCluster(s), placement)
			if panicked != nil {
				t.Fatalf("integration panicked: %v", panicked)
			}
			for _, want := range []string{"GitRepository", "flux-system/repo", "spec.ref.bad"} {
				if !strings.Contains(got, want) {
					t.Errorf("error %q does not name %q", got, want)
				}
			}
			if v := s.Object["spec"].(map[string]any)["ref"].(map[string]any)["bad"]; fmt.Sprint(v) != fmt.Sprint(bad) {
				t.Errorf("the caller's object was changed: spec.ref.bad = %v", v)
			}
		})
	}
}

// An unstructured List a typed List holds is the caller's object: it is
// normalized in place, and not through the temporary wrappers of its items.
func TestCallerObject_UnstructuredListInTypedListIsReadLikeItsJSON(t *testing.T) {
	item := func(n any) *unstructured.Unstructured {
		k := unstructuredObj("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "flux-system", "caller")
		k.Object["spec"] = map[string]any{
			"path":      "./elsewhere",
			"sourceRef": map[string]any{"kind": "GitRepository", "name": "repo", "extra": n},
		}
		return k
	}
	for _, placement := range append(slices.Clone(integratedPlacements), layout.FluxSeparate) {
		t.Run(string(placement), func(t *testing.T) {
			want, panicked := integrateOutcome(t, baseCluster(typedListOf("List", wrapInList(item(int64(53))))), placement)
			if panicked != nil {
				t.Fatalf("the int64 baseline panicked: %v", panicked)
			}
			got, panicked := integrateOutcome(t, baseCluster(typedListOf("List", wrapInList(item(53)))), placement)
			if panicked != nil {
				t.Fatalf("integration panicked: %v", panicked)
			}
			if got != want {
				t.Errorf("got error %q, want the int64 outcome %q", got, want)
			}
		})
	}
}

// anyItems is a typed object that apimachinery takes for a List and cannot
// read: its Items field is an interface that holds nothing.
type anyItems struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             any `json:"items"`
}

func (l *anyItems) DeepCopyObject() runtime.Object { c := *l; return &c }

// A typed object whose list items apimachinery cannot read is no reason to
// panic: the helper uses the same protected discovery as the reads.
func TestCallerObject_UnreadableTypedListDoesNotPanic(t *testing.T) {
	l := &anyItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}}
	l.Name, l.Namespace = "holder", "default"
	ml := &layout.ManifestLayout{Name: "root", Resources: []client.Object{l}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("UmbrellaTerminals panicked: %v", r)
		}
	}()
	_, _ = fluxstack.UmbrellaTerminals(ml)
}

// An item a typed List holds that does not marshal refuses the call, with the
// object and the field named, as an object of a layout does.
func TestCallerObject_TypedListItemUnmarshalableValueIsRefused(t *testing.T) {
	cm := unstructuredObj("v1", "ConfigMap", "default", "cm")
	cm.Object["spec"] = map[string]any{"bad": make(chan int)}
	var obj client.Object = typedListOf("List", cm)
	app := stack.NewApplication("raw", "default", &fakeAppConfig{objs: []*client.Object{&obj}})
	c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", app)}}
	ml, err := layout.WalkCluster(c, withPlacement(nodeOnly, layout.FluxSeparate))
	if err != nil {
		t.Fatal(err)
	}
	_, err = fluxstack.NewResourceGenerator().GenerateFromLayout(ml, c)
	if err == nil {
		t.Fatal("GenerateFromLayout accepted an item that does not marshal")
	}
	for _, want := range []string{"ConfigMap", "default/cm", "spec.bad"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// writesOtherItems is a typed List whose Go Items are not what it writes under
// items: kure reads the written ones, so the unwritten ones are no object it
// reads.
type writesOtherItems struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             []*unstructured.Unstructured `json:"-"`
	Written           []map[string]any             `json:"items"`
}

func (l *writesOtherItems) DeepCopyObject() runtime.Object { c := *l; return &c }

// An object a typed List holds but does not write is read by nothing, so a
// value in it that does not marshal refuses nothing.
func TestCallerObject_UnwrittenTypedListItemIsNotRefused(t *testing.T) {
	unwritten := unstructuredObj("v1", "ConfigMap", "default", "unwritten")
	unwritten.Object["spec"] = map[string]any{"bad": make(chan int)}
	l := &writesOtherItems{
		TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"},
		Items:    []*unstructured.Unstructured{unwritten},
		Written:  []map[string]any{{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": "written", "namespace": "default"}}},
	}
	l.Name, l.Namespace = "holder", "default"
	ml := &layout.ManifestLayout{Name: "root", Resources: []client.Object{l}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("UmbrellaTerminals panicked: %v", r)
		}
	}()
	if _, err := fluxstack.UmbrellaTerminals(ml); err != nil && strings.Contains(err.Error(), "spec.bad") {
		t.Errorf("got %v: an object the List does not write was refused", err)
	}
}

// panicMarshaler panics when it is marshalled, and counts the calls.
type panicMarshaler struct{}

var panicMarshalCalls int

func (panicMarshaler) MarshalJSON() ([]byte, error) {
	panicMarshalCalls++
	panic("a caller's marshaler was called")
}

// A typed List is discovered before its items are validated. The discovery
// must not marshal an item that holds a value with a marshaler: the call is
// refused, and the method is never called (go-kure/kure#1201).
func TestCallerObject_TypedListItemMarshalerIsRefusedNotCalled(t *testing.T) {
	for _, placement := range append(slices.Clone(integratedPlacements), layout.FluxSeparate) {
		t.Run(string(placement), func(t *testing.T) {
			panicMarshalCalls = 0
			cm := unstructuredObj("v1", "ConfigMap", "default", "cm")
			cm.Object["spec"] = map[string]any{"m": panicMarshaler{}}
			got, panicked := integrateOutcome(t, baseCluster(typedListOf("List", cm)), placement)
			if panicked != nil {
				t.Fatalf("integration panicked: %v", panicked)
			}
			for _, want := range []string{"ConfigMap", "default/cm", "spec.m", "marshals itself"} {
				if !strings.Contains(got, want) {
					t.Errorf("error %q does not name %q", got, want)
				}
			}
			if panicMarshalCalls != 0 {
				t.Errorf("the marshaler was called %d times", panicMarshalCalls)
			}
		})
	}
}

// The same holds for an item of a typed List held by a typed List, and for
// the entry point that reads the layout without integrating it.
func TestCallerObject_NestedTypedListItemMarshalerIsRefusedNotCalled(t *testing.T) {
	panicMarshalCalls = 0
	cm := unstructuredObj("v1", "ConfigMap", "default", "cm")
	cm.Object["spec"] = map[string]any{"m": panicMarshaler{}}
	// The object is an item of an inner typed List, which is an item of an outer one.
	outer := &objectItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}, Items: []runtime.Object{typedListOf("List", cm)}}
	outer.Name, outer.Namespace = "outer", "default"
	ml := &layout.ManifestLayout{Name: "root", Resources: []client.Object{outer}}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("UmbrellaTerminals panicked: %v", r)
			}
		}()
		_, err := fluxstack.UmbrellaTerminals(ml)
		if err == nil || !strings.Contains(err.Error(), "marshals itself") {
			t.Errorf("got %v, want a refusal naming the marshaler", err)
		}
	}()
	if panicMarshalCalls != 0 {
		t.Errorf("the marshaler was called %d times", panicMarshalCalls)
	}
}

// nilMarshalerWrapper embeds an interface from a trusted package and leaves it
// nil: MarshalJSON is promoted from the interface, and calling it panics.
type nilMarshalerWrapper struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	json.Marshaler
}

func (w *nilMarshalerWrapper) DeepCopyObject() runtime.Object { c := *w; return &c }

// The type is judged trusted for the package of the interface it embeds, and
// the method that runs is the one of the value the field holds, which a nil
// one has none of. The call panics, and the read guard recovers it: the entry
// point refuses the object, naming its type, and never panics.
func TestCallerObject_EmbeddedNilMarshalerInterfaceIsRefused(t *testing.T) {
	w := &nilMarshalerWrapper{
		TypeMeta:   metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Wrapper"},
		ObjectMeta: metav1.ObjectMeta{Name: "w", Namespace: "default"},
	}
	ml := &layout.ManifestLayout{Name: "root", Resources: []client.Object{w}}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("UmbrellaTerminals panicked: %v", r)
			}
		}()
		_, err := fluxstack.UmbrellaTerminals(ml)
		if err == nil || !strings.Contains(err.Error(), "nilMarshalerWrapper") || !strings.Contains(err.Error(), "cannot be read") {
			t.Errorf("got %v, want a refusal naming the wrapper and saying it cannot be read", err)
		}
	}()
}

// objectItems is a typed List whose Items are runtime.Objects, as a list of
// mixed kinds has them.
type objectItems struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             []runtime.Object `json:"items"`
}

func (l *objectItems) DeepCopyObject() runtime.Object { c := *l; return &c }

// apimachinery cannot read the items of a List that holds a nil one, and it
// panics. The held objects after the nil are still found and checked, and not
// marshalled with the List.
func TestCallerObject_ListWithNilItemStillChecksItsObjects(t *testing.T) {
	panicMarshalCalls = 0
	cm := unstructuredObj("v1", "ConfigMap", "default", "cm")
	cm.Object["spec"] = map[string]any{"m": panicMarshaler{}}
	l := &objectItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}, Items: []runtime.Object{nil, cm}}
	l.Name, l.Namespace = "holder", "default"
	ml := &layout.ManifestLayout{Name: "root", Resources: []client.Object{l}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("UmbrellaTerminals panicked: %v", r)
		}
	}()
	_, err := fluxstack.UmbrellaTerminals(ml)
	if err == nil || !strings.Contains(err.Error(), "marshals itself") {
		t.Errorf("got %v, want a refusal naming the marshaler", err)
	}
	if panicMarshalCalls != 0 {
		t.Errorf("the marshaler was called %d times", panicMarshalCalls)
	}
}

// A List that holds itself under a nil item ends the walk that finds its
// objects.
func TestCallerObject_ListWithNilItemHoldingItselfDoesNotRecurse(t *testing.T) {
	l := &objectItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}}
	l.Name, l.Namespace = "holder", "default"
	l.Items = []runtime.Object{nil, l}
	ml := &layout.ManifestLayout{Name: "root", Resources: []client.Object{l}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("UmbrellaTerminals panicked: %v", r)
		}
	}()
	_, _ = fluxstack.UmbrellaTerminals(ml)
}

// ptrItems is a typed List whose Items field is a pointer to a slice.
type ptrItems struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             *[]runtime.Object `json:"items"`
}

func (l *ptrItems) DeepCopyObject() runtime.Object { c := *l; return &c }

func refusedWithoutPanic(t *testing.T, r client.Object, want string) {
	t.Helper()
	ml := &layout.ManifestLayout{Name: "root", Resources: []client.Object{r}}
	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("UmbrellaTerminals panicked: %v", p)
		}
	}()
	_, err := fluxstack.UmbrellaTerminals(ml)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("got %v, want a refusal naming %q", err, want)
	}
}

// The held objects of a List are found wherever the List holds them: through a
// pointer to a slice as well, with a nil item that apimachinery cannot read.
func TestCallerObject_PointerItemsWithNilItemStillChecksItsObjects(t *testing.T) {
	panicMarshalCalls = 0
	cm := unstructuredObj("v1", "ConfigMap", "default", "cm")
	cm.Object["spec"] = map[string]any{"m": panicMarshaler{}}
	items := []runtime.Object{nil, cm}
	l := &ptrItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}, Items: &items}
	l.Name, l.Namespace = "holder", "default"
	refusedWithoutPanic(t, l, "marshals itself")
	if panicMarshalCalls != 0 {
		t.Errorf("the marshaler was called %d times", panicMarshalCalls)
	}
}

// A cycle the item walk cannot see, through a nil item and a RawExtension that
// holds the List, is refused, and not marshalled until the stack ends.
func TestCallerObject_ListCycleThroughRawExtensionIsRefused(t *testing.T) {
	l := &objectItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}}
	l.Name, l.Namespace = "holder", "default"
	inner := &metav1.List{Items: []runtime.RawExtension{{Object: l}}}
	l.Items = []runtime.Object{nil, inner}
	refusedWithoutPanic(t, l, "holds itself")
}

// chainNode is a typed object that holds a chain of its own kind.
type chainNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Next              *chainNode `json:"next,omitempty"`
}

func (c *chainNode) DeepCopyObject() runtime.Object { n := *c; return &n }

// A typed object nested deeper than encoding/json takes (10,000 levels, and
// the walk takes several steps for each) is refused.
func TestCallerObject_VeryDeepTypedObjectIsRefused(t *testing.T) {
	head := &chainNode{}
	head.Name, head.Namespace = "head", "default"
	for tail, i := head, 0; i < 25000; i++ {
		tail.Next = &chainNode{}
		tail = tail.Next
	}
	refusedWithoutPanic(t, head, "nested deeper than")
}

// otherField is a typed object that holds an unstructured object under a field
// that is no Items.
type otherField struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Extra             *unstructured.Unstructured `json:"extra,omitempty"`
}

func (o *otherField) DeepCopyObject() runtime.Object { c := *o; return &c }

// An unstructured object held under a field other than Items is checked too.
func TestCallerObject_UnstructuredUnderAnotherFieldIsChecked(t *testing.T) {
	panicMarshalCalls = 0
	cm := unstructuredObj("v1", "ConfigMap", "default", "cm")
	cm.Object["spec"] = map[string]any{"m": panicMarshaler{}}
	o := &otherField{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Thing"}, Extra: cm}
	o.Name, o.Namespace = "holder", "default"
	refusedWithoutPanic(t, o, "marshals itself")
	if panicMarshalCalls != 0 {
		t.Errorf("the marshaler was called %d times", panicMarshalCalls)
	}
}

// keyedField is a typed object with a map whose key holds itself.
type keyedField struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Keyed             map[*map[string]any]any `json:"keyed,omitempty"`
}

func (o *keyedField) DeepCopyObject() runtime.Object { c := *o; return &c }

// The walk does not format a map key: a key that holds itself has no end when
// formatted.
func TestCallerObject_MapKeyThatHoldsItselfDoesNotOverflow(t *testing.T) {
	m := map[string]any{}
	m["self"] = m
	o := &keyedField{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Thing"}, Keyed: map[*map[string]any]any{&m: nil}}
	o.Name, o.Namespace = "holder", "default"
	ml := &layout.ManifestLayout{Name: "root", Resources: []client.Object{o}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("UmbrellaTerminals panicked: %v", r)
		}
	}()
	_, _ = fluxstack.UmbrellaTerminals(ml)
}

// hidingItem holds an unstructured object in a private field, one struct down,
// and writes it through its own marshaler: marshalling reaches what the walk
// of fields does not.
type hidingItem struct {
	inner struct{ u *unstructured.Unstructured }
}

func (h hidingItem) MarshalJSON() ([]byte, error) { return json.Marshal(h.inner.u) }

type hidingHolder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Hidden            hidingItem `json:"hidden"`
}

func (o *hidingHolder) DeepCopyObject() runtime.Object { c := *o; return &c }

// A type that marshals itself and is not from a trusted package is refused
// wherever the object holds it, with its type named, and is never called.
func TestCallerObject_SelfMarshalingTypeOfAnotherPackageIsRefused(t *testing.T) {
	panicMarshalCalls = 0
	cm := unstructuredObj("v1", "ConfigMap", "default", "cm")
	cm.Object["spec"] = map[string]any{"m": panicMarshaler{}}
	o := &hidingHolder{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Thing"}}
	o.Hidden.inner.u = cm
	o.Name, o.Namespace = "holder", "default"
	refusedWithoutPanic(t, o, "hidingItem")
	if panicMarshalCalls != 0 {
		t.Errorf("the marshaler was called %d times", panicMarshalCalls)
	}
}

// key writes itself as text, and holds an unstructured object.
type key struct{ U *unstructured.Unstructured }

func (k key) MarshalText() ([]byte, error) { return json.Marshal(k.U) }

type keyedMarshaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Keyed             map[key]any `json:"keyed,omitempty"`
}

func (o *keyedMarshaler) DeepCopyObject() runtime.Object { c := *o; return &c }

func TestCallerObject_MapKeyThatMarshalsItselfIsRefused(t *testing.T) {
	panicMarshalCalls = 0
	cm := unstructuredObj("v1", "ConfigMap", "default", "cm")
	cm.Object["spec"] = map[string]any{"m": panicMarshaler{}}
	o := &keyedMarshaler{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Thing"}, Keyed: map[key]any{{U: cm}: nil}}
	o.Name, o.Namespace = "holder", "default"
	refusedWithoutPanic(t, o, "fluxcd_test.key")
	if panicMarshalCalls != 0 {
		t.Errorf("the marshaler was called %d times", panicMarshalCalls)
	}
}

// ifaceKeyed holds a map whose key type is an interface and whose values are
// scalars, so nothing but its keys can hold a marshaler.
type ifaceKeyed struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Keyed             map[encoding.TextMarshaler]string `json:"keyed,omitempty"`
}

func (o *ifaceKeyed) DeepCopyObject() runtime.Object { c := *o; return &c }

func TestCallerObject_InterfaceMapKeyThatMarshalsItselfIsRefused(t *testing.T) {
	panicMarshalCalls = 0
	cm := unstructuredObj("v1", "ConfigMap", "default", "cm")
	cm.Object["spec"] = map[string]any{"m": panicMarshaler{}}
	o := &ifaceKeyed{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Thing"}, Keyed: map[encoding.TextMarshaler]string{key{U: cm}: "v"}}
	o.Name, o.Namespace = "holder", "default"
	refusedWithoutPanic(t, o, "fluxcd_test.key")
	if panicMarshalCalls != 0 {
		t.Errorf("the marshaler was called %d times", panicMarshalCalls)
	}
}

// trustedHolder holds types of trusted packages that marshal themselves.
type trustedHolder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	When              metav1.Time        `json:"when"`
	Size              resource.Quantity  `json:"size"`
	Port              intstr.IntOrString `json:"port"`
	Raw               json.RawMessage    `json:"raw,omitempty"`
}

func (o *trustedHolder) DeepCopyObject() runtime.Object { c := *o; return &c }

func TestCallerObject_SelfMarshalingTypesOfTrustedPackagesAreAccepted(t *testing.T) {
	o := &trustedHolder{
		TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Thing"},
		When:     metav1.Now(), Size: resource.MustParse("1Gi"), Port: intstr.FromInt32(80), Raw: json.RawMessage(`{}`),
	}
	o.Name, o.Namespace = "holder", "default"
	ml := &layout.ManifestLayout{Name: "root", Resources: []client.Object{o}}
	if _, err := fluxstack.UmbrellaTerminals(ml); err != nil && strings.Contains(err.Error(), "trusted package") {
		t.Errorf("got %v, want trusted types accepted", err)
	}
}

// A chain of Lists that each hold the next twice is 2^n Lists to a walk that
// does not remember where it has been: it is refused, and at once.
func TestCallerObject_ListsHeldTwiceAreRefusedAtOnce(t *testing.T) {
	var next runtime.Object = &metav1.List{}
	for i := 0; i < 60; i++ {
		l := &objectItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}, Items: []runtime.Object{next, next}}
		l.Name, l.Namespace = "holder", "default"
		next = l
	}
	done := make(chan error, 1)
	go func() {
		ml := &layout.ManifestLayout{Name: "root", Resources: []client.Object{next.(client.Object)}}
		_, err := fluxstack.UmbrellaTerminals(ml)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "more than") {
			t.Errorf("got %v, want a refusal naming the value limit", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the call did not end")
	}
}

// A typed nil resource is refused, naming the layout, its place in it and its
// Go type: a method called on it may panic.
func TestCallerObject_TypedNilResourceIsRefusedNamingIt(t *testing.T) {
	var cm *corev1.ConfigMap
	ml := &layout.ManifestLayout{Name: "root", Resources: []client.Object{cm}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("GenerateFromLayout panicked: %v", r)
		}
	}()
	_, err := fluxstack.NewResourceGenerator().GenerateFromLayout(ml, &stack.Cluster{Node: &stack.Node{Name: "root"}})
	if err == nil {
		t.Fatal("got no error, want a refusal")
	}
	for _, want := range []string{"layout", "resource 0", "nil *v1.ConfigMap"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// A typed nil item of a typed List is no object either.
func TestCallerObject_TypedNilListItemDoesNotPanic(t *testing.T) {
	var cm *corev1.ConfigMap
	l := &objectItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}, Items: []runtime.Object{cm}}
	l.Name, l.Namespace = "holder", "default"
	ml := &layout.ManifestLayout{Name: "root", Resources: []client.Object{l}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("GenerateFromLayout panicked: %v", r)
		}
	}()
	_, err := fluxstack.NewResourceGenerator().GenerateFromLayout(ml, &stack.Cluster{Node: &stack.Node{Name: "root"}})
	t.Logf("GenerateFromLayout: %v", err)
}

// A resource that embeds a nil pointer has a non-nil outer pointer, and the
// method it promotes from the nil one panics. It is refused, naming the layout,
// the resource's place in it and its Go type, and does not panic.
func TestCallerObject_ResourceEmbeddingNilPointerIsRefusedNamingIt(t *testing.T) {
	ml := &layout.ManifestLayout{Name: "root", Resources: []client.Object{
		unstructuredObj("v1", "ConfigMap", "default", "first"),
		&struct{ *corev1.ConfigMap }{},
	}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("GenerateFromLayout panicked: %v", r)
		}
	}()
	_, err := fluxstack.NewResourceGenerator().GenerateFromLayout(ml, &stack.Cluster{Node: &stack.Node{Name: "root"}})
	if err == nil {
		t.Fatal("got no error, want a refusal")
	}
	for _, want := range []string{"layout", "resource 1", "struct { *v1.ConfigMap }"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}
