package fluxcd_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
