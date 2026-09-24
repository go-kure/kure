package fluxcd_test

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// allFlat merges every node, bundle and application into the root's
// directory: the whole cluster is one reconciliation unit.
var allFlat = layout.LayoutRules{
	NodeGrouping: layout.GroupFlat, BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupFlat,
	FluxPlacement: layout.FluxSeparate,
}

// mergedCluster is r (bundle rb) with child nodes c1 (bundle b1) and c2
// (bundle b2); edit tweaks the three bundles before the walk.
func mergedCluster(edit func(rb, b1, b2 *stack.Bundle)) *stack.Cluster {
	rb := srBundle("rb", cmApp("core"))
	b1 := srBundle("b1", cmApp("one"))
	b2 := srBundle("b2", cmApp("two"))
	if edit != nil {
		edit(rb, b1, b2)
	}
	c1 := &stack.Node{Name: "c1", Bundle: b1}
	c2 := &stack.Node{Name: "c2", Bundle: b2}
	r := &stack.Node{Name: "r", Bundle: rb, Children: []*stack.Node{c1, c2}}
	c1.SetParent(r)
	c2.SetParent(r)
	return &stack.Cluster{Name: "demo", Node: r}
}

func generateUnits(t *testing.T, c *stack.Cluster, rules layout.LayoutRules) ([]*kustv1.Kustomization, error) {
	t.Helper()
	ml, err := layout.WalkCluster(c, rules)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	objs, err := fluxstack.NewResourceGenerator().GenerateFromLayout(ml, c)
	if err != nil {
		return nil, err
	}
	var out []*kustv1.Kustomization
	for _, o := range objs {
		if k, ok := o.(*kustv1.Kustomization); ok {
			out = append(out, k)
		}
	}
	return out, nil
}

func dependsOnNames(k *kustv1.Kustomization) []string {
	var out []string
	for _, d := range k.Spec.DependsOn {
		out = append(out, d.Name)
	}
	sort.Strings(out)
	return out
}

// TestGenerateFromLayout_OneKustomizationPerDirectory pins that a directory
// rendering several bundles gets one Kustomization, named after its first
// bundle, with the bundles' health checks, labels and targeted patches
// combined.
func TestGenerateFromLayout_OneKustomizationPerDirectory(t *testing.T) {
	c := mergedCluster(func(rb, b1, b2 *stack.Bundle) {
		rb.HealthChecks = []stack.HealthCheck{{APIVersion: "apps/v1", Kind: "Deployment", Name: "core", Namespace: "default"}}
		b1.HealthChecks = []stack.HealthCheck{{APIVersion: "apps/v1", Kind: "Deployment", Name: "one", Namespace: "default"}}
		b2.HealthChecks = rb.HealthChecks // a duplicate is listed once
		rb.Labels = map[string]string{"team": "platform"}
		b1.Labels = map[string]string{"tier": "web"}
		b1.Patches = []stack.Patch{{Patch: "- op: add", Target: &stack.PatchSelector{Kind: "Deployment", Name: "one"}}}
		b2.NamedDependsOn = []string{"external"}
	})
	kusts, err := generateUnits(t, c, allFlat)
	if err != nil {
		t.Fatalf("GenerateFromLayout: %v", err)
	}
	if len(kusts) != 1 {
		t.Fatalf("got %d Kustomizations, want one for the one written directory", len(kusts))
	}
	k := kusts[0]
	if k.Name != "rb" || k.Spec.Path != "r" {
		t.Errorf("Kustomization %s at %q, want rb at %q", k.Name, k.Spec.Path, "r")
	}
	if len(k.Spec.HealthChecks) != 2 {
		t.Errorf("health checks = %v, want core and one, each once", k.Spec.HealthChecks)
	}
	if want := map[string]string{"team": "platform", "tier": "web"}; !reflect.DeepEqual(k.Labels, want) {
		t.Errorf("labels = %v, want %v", k.Labels, want)
	}
	if len(k.Spec.Patches) != 1 {
		t.Errorf("patches = %v, want b1's one targeted patch", k.Spec.Patches)
	}
	if got := dependsOnNames(k); !reflect.DeepEqual(got, []string{"external"}) {
		t.Errorf("dependsOn = %v, want [external]", got)
	}
}

// TestGenerateFromLayout_MergedSettingsMustAgree pins that bundles merged
// into one directory must agree on every setting a Kustomization holds once,
// and that the refusal names the setting and the bundles.
func TestGenerateFromLayout_MergedSettingsMustAgree(t *testing.T) {
	yes := true
	tests := map[string]func(b2 *stack.Bundle){
		"interval":      func(b2 *stack.Bundle) { b2.Interval = "5m" },
		"timeout":       func(b2 *stack.Bundle) { b2.Timeout = "1m" },
		"retryInterval": func(b2 *stack.Bundle) { b2.RetryInterval = "30s" },
		"prune":         func(b2 *stack.Bundle) { b2.Prune = &yes },
		"wait":          func(b2 *stack.Bundle) { b2.Wait = &yes },
		"force":         func(b2 *stack.Bundle) { b2.Force = &yes },
		"suspend":       func(b2 *stack.Bundle) { b2.Suspend = &yes },
		"sourceRef": func(b2 *stack.Bundle) {
			b2.SourceRef = &stack.SourceRef{Kind: "GitRepository", Name: "other", Namespace: "flux-system"}
		},
		"postBuild": func(b2 *stack.Bundle) {
			b2.PostBuild = &stack.PostBuild{Substitute: map[string]string{"x": "1"}}
		},
		"labels": func(b2 *stack.Bundle) { b2.Labels = map[string]string{"team": "other"} },
		"patches": func(b2 *stack.Bundle) {
			b2.Patches = []stack.Patch{{Patch: "- op: add"}} // no Target
		},
	}
	for field, edit := range tests {
		t.Run(field, func(t *testing.T) {
			c := mergedCluster(func(rb, _, b2 *stack.Bundle) {
				rb.Labels = map[string]string{"team": "platform"}
				edit(b2)
			})
			_, err := generateUnits(t, c, allFlat)
			if err == nil {
				t.Fatalf("merging b2 with a different %s into rb's directory: no error", field)
			}
			for _, want := range []string{field, "rb", "b2"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not name %q", err, want)
				}
			}
		})
	}
}

// TestGenerateFromLayout_DependsOnMapsToUnits pins how dependsOn survives a
// merge: a dependency between bundles merged into one directory is dropped,
// whether it is a bundle pointer, a copy of that bundle (the fluent builder
// copies bundles) or a NamedDependsOn naming it; a dependency on another unit
// names that unit's Kustomization; an external name is kept.
func TestGenerateFromLayout_DependsOnMapsToUnits(t *testing.T) {
	u := &stack.Bundle{Name: "u", SourceRef: testSR(), Applications: []*stack.Application{cmApp("ua")}}
	c := mergedCluster(func(rb, b1, b2 *stack.Bundle) {
		rb.Children = []*stack.Bundle{u}
		copyOfB2 := *b2
		b1.DependsOn = []*stack.Bundle{b2, &copyOfB2, u}
		b1.NamedDependsOn = []string{"rb", "external"}
	})
	kusts, err := generateUnits(t, c, allFlat)
	if err != nil {
		t.Fatalf("GenerateFromLayout: %v", err)
	}
	byName := map[string]*kustv1.Kustomization{}
	for _, k := range kusts {
		byName[k.Name] = k
	}
	if len(kusts) != 2 || byName["rb"] == nil || byName["u"] == nil {
		t.Fatalf("Kustomizations = %v, want rb (r) and u (its umbrella directory)", byName)
	}
	if got := dependsOnNames(byName["rb"]); !reflect.DeepEqual(got, []string{"external", "u"}) {
		t.Errorf("rb dependsOn = %v, want [external u]", got)
	}
}

// TestGenerateFromLayout_RefusesUmbrellaReadinessCycle pins that an umbrella
// parent waits for its children (health checks): an umbrella child that
// depends on a bundle merged into its parent's unit closes a cycle, by
// pointer or by name.
func TestGenerateFromLayout_RefusesUmbrellaReadinessCycle(t *testing.T) {
	for name, edit := range map[string]func(u, b2 *stack.Bundle){
		"DependsOn":      func(u, b2 *stack.Bundle) { u.DependsOn = []*stack.Bundle{b2} },
		"NamedDependsOn": func(u, _ *stack.Bundle) { u.NamedDependsOn = []string{"b2"} },
	} {
		t.Run(name, func(t *testing.T) {
			u := &stack.Bundle{Name: "u", SourceRef: testSR(), Applications: []*stack.Application{cmApp("ua")}}
			c := mergedCluster(func(rb, _, b2 *stack.Bundle) {
				rb.Children = []*stack.Bundle{u}
				edit(u, b2)
			})
			if _, err := generateUnits(t, c, allFlat); err == nil || !strings.Contains(err.Error(), "cycle") {
				t.Fatalf("got %v, want a cycle refusal (rb waits for u, u depends on rb)", err)
			}
		})
	}
}

// TestGenerateFromLayout_RefusesUnitCycle pins that a dependency cycle between
// Kustomizations is refused: here the merge turns b1 -> u -> b2 into
// rb -> u -> rb.
func TestGenerateFromLayout_RefusesUnitCycle(t *testing.T) {
	u := &stack.Bundle{Name: "u", SourceRef: testSR(), Applications: []*stack.Application{cmApp("ua")}}
	c := mergedCluster(func(rb, b1, b2 *stack.Bundle) {
		rb.Children = []*stack.Bundle{u}
		b1.DependsOn = []*stack.Bundle{u}
		u.DependsOn = []*stack.Bundle{b2}
	})
	if _, err := generateUnits(t, c, allFlat); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("got %v, want a dependency cycle refusal", err)
	}
}

// TestIntegrateWithLayout_OneKustomizationPerDirectory pins the same rule in
// every placement the integrator writes.
func TestIntegrateWithLayout_OneKustomizationPerDirectory(t *testing.T) {
	for _, placement := range []layout.FluxPlacement{layout.FluxSeparate, layout.FluxIntegratedPerLayout, layout.FluxIntegratedPerBundle} {
		t.Run(string(placement), func(t *testing.T) {
			rules := allFlat
			rules.FluxPlacement = placement
			ml := integrated(t, mergedCluster(nil), rules)
			var names []string
			for _, k := range kustomizations(ml) {
				names = append(names, k.Name)
			}
			if !reflect.DeepEqual(names, []string{"rb"}) {
				t.Errorf("Kustomizations = %v, want [rb]", names)
			}
		})
	}
}

var _ client.Object = (*kustv1.Kustomization)(nil)
