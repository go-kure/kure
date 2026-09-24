package fluxcd_test

import (
	"os"
	"path/filepath"
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
			if _, err := generateUnits(t, c, allFlat); err == nil || !strings.Contains(err.Error(), "never") {
				t.Fatalf("got %v, want a reconcile-order refusal (rb waits for u, u depends on rb)", err)
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

// TestGenerateFromLayout_HealthChecksMapToUnits pins that a health check on a
// Flux Kustomization follows the merge: one naming a bundle whose Kustomization
// the merge removed names that bundle's unit, and one on the unit itself is
// dropped (it would wait for itself). Other health checks are kept.
func TestGenerateFromLayout_HealthChecksMapToUnits(t *testing.T) {
	kust := func(name string) stack.HealthCheck {
		return stack.HealthCheck{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization", Name: name, Namespace: "flux-system"}
	}
	deploy := stack.HealthCheck{APIVersion: "apps/v1", Kind: "Deployment", Name: "core", Namespace: "default"}
	c := mergedCluster(func(rb, b1, b2 *stack.Bundle) {
		rb.HealthChecks = []stack.HealthCheck{kust("b1"), deploy}
		b2.HealthChecks = []stack.HealthCheck{kust("external")}
	})
	kusts, err := generateUnits(t, c, allFlat)
	if err != nil {
		t.Fatalf("GenerateFromLayout: %v", err)
	}
	var got []string
	for _, hc := range kusts[0].Spec.HealthChecks {
		got = append(got, hc.Kind+"/"+hc.Name)
	}
	sort.Strings(got)
	if want := []string{"Deployment/core", "Kustomization/external"}; !reflect.DeepEqual(got, want) {
		t.Errorf("health checks = %v, want %v (b1 is rb's own unit)", got, want)
	}
}

// TestGenerateFromLayout_MergedEquivalentSourceNamespaces pins that SourceRefs
// are compared by effective value: an omitted namespace is the generator's
// default namespace.
func TestGenerateFromLayout_MergedEquivalentSourceNamespaces(t *testing.T) {
	c := mergedCluster(func(rb, b1, b2 *stack.Bundle) {
		for _, b := range []*stack.Bundle{rb, b1} {
			b.SourceRef = &stack.SourceRef{Kind: "GitRepository", Name: "repo", Namespace: "flux-system"}
		}
		b2.SourceRef = &stack.SourceRef{Kind: "GitRepository", Name: "repo"}
	})
	if _, err := generateUnits(t, c, allFlat); err != nil {
		t.Fatalf("equivalent SourceRefs refused: %v", err)
	}
}

// TestIntegrateWithLayout_RefusesDependencyOnHostedDescendant pins that a
// Kustomization cannot depend on one whose CR only it creates: a depends on
// its child node's bundle b, and under PerLayout b's CR lives in a's
// directory, which only a's Kustomization applies. Separate placement (every
// CR in flux-system, applied by the bootstrap) and PerBundle (child
// directories listed from the root) create b's CR without a.
func TestIntegrateWithLayout_RefusesDependencyOnHostedDescendant(t *testing.T) {
	for _, placement := range []layout.FluxPlacement{layout.FluxSeparate, layout.FluxIntegratedPerLayout, layout.FluxIntegratedPerBundle} {
		t.Run(string(placement), func(t *testing.T) {
			b := &stack.Node{Name: "b", Bundle: srBundle("b", cmApp("b-app"))}
			a := &stack.Node{Name: "a", Bundle: srBundle("a", cmApp("a-app")), Children: []*stack.Node{b}}
			a.Bundle.DependsOn = []*stack.Bundle{b.Bundle}
			r := &stack.Node{Name: "r", Children: []*stack.Node{a}}
			b.SetParent(a)
			a.SetParent(r)
			rules := propertyGroupings["nodeOnly"]
			rules.FluxPlacement = placement
			_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(&stack.Cluster{Name: "demo", Node: r}, rules)
			if placement != layout.FluxIntegratedPerLayout {
				if err != nil {
					t.Fatalf("%s: %v", placement, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "never") {
				t.Fatalf("got %v, want a reconcile-order refusal: a depends on b, whose CR only a creates", err)
			}
		})
	}
}

// TestIntegrateWithLayout_Idempotent_Merged pins re-integration of a merged
// (NodeGrouping flat) tree: the one unit Kustomization is recognised, not
// added again.
func TestIntegrateWithLayout_Idempotent_Merged(t *testing.T) {
	for _, placement := range []layout.FluxPlacement{layout.FluxSeparate, layout.FluxIntegratedPerLayout, layout.FluxIntegratedPerBundle} {
		t.Run(string(placement), func(t *testing.T) {
			rules := allFlat
			rules.FluxPlacement = placement
			checkIdempotent(t, mergedCluster(nil), rules)
		})
	}
}

// TestGenerateFromLayout_RefusesHealthCheckCycleAfterMerge pins that a health
// check a merge turns into a cycle is refused: u checks b2's Kustomization,
// b2 merges into rb's unit, and rb waits for its umbrella child u.
func TestGenerateFromLayout_RefusesHealthCheckCycleAfterMerge(t *testing.T) {
	for _, placement := range []layout.FluxPlacement{layout.FluxSeparate, layout.FluxIntegratedPerLayout, layout.FluxIntegratedPerBundle} {
		t.Run(string(placement), func(t *testing.T) {
			u := &stack.Bundle{Name: "u", SourceRef: testSR(), Applications: []*stack.Application{cmApp("ua")},
				HealthChecks: []stack.HealthCheck{{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization", Name: "b2", Namespace: "flux-system"}}}
			c := mergedCluster(func(rb, _, _ *stack.Bundle) { rb.Children = []*stack.Bundle{u} })
			rules := allFlat
			rules.FluxPlacement = placement
			_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
			if err == nil || !strings.Contains(err.Error(), "never") {
				t.Fatalf("got %v, want a reconcile-order refusal (rb waits for u, u waits for rb)", err)
			}
		})
	}
}

// TestIntegrateWithLayout_RefusesTransitiveDependencyOnHostedDescendant pins
// the creation rule through a dependency chain: a depends on c, c on b, and
// b's CR is created by a (PerLayout hosts it in a's directory).
func TestIntegrateWithLayout_RefusesTransitiveDependencyOnHostedDescendant(t *testing.T) {
	b := &stack.Node{Name: "b", Bundle: srBundle("b", cmApp("b-app"))}
	a := &stack.Node{Name: "a", Bundle: srBundle("a", cmApp("a-app")), Children: []*stack.Node{b}}
	cn := &stack.Node{Name: "c", Bundle: srBundle("c", cmApp("c-app"))}
	a.Bundle.DependsOn = []*stack.Bundle{cn.Bundle}
	cn.Bundle.DependsOn = []*stack.Bundle{b.Bundle}
	r := &stack.Node{Name: "r", Children: []*stack.Node{a, cn}}
	b.SetParent(a)
	a.SetParent(r)
	cn.SetParent(r)
	rules := propertyGroupings["nodeOnly"]
	rules.FluxPlacement = layout.FluxIntegratedPerLayout
	_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(&stack.Cluster{Name: "demo", Node: r}, rules)
	if err == nil || !strings.Contains(err.Error(), "never") {
		t.Fatalf("got %v, want a reconcile-order refusal (a -> c -> b, b created by a)", err)
	}
}

// TestGenerateFromLayout_RefusesMergeHealthCheckCycle pins the reconcile-order
// check on GenerateFromLayout itself, not only through the integrator.
func TestGenerateFromLayout_RefusesMergeHealthCheckCycle(t *testing.T) {
	u := &stack.Bundle{Name: "u", SourceRef: testSR(), Applications: []*stack.Application{cmApp("ua")},
		HealthChecks: []stack.HealthCheck{{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization", Name: "b2", Namespace: "flux-system"}}}
	c := mergedCluster(func(rb, _, _ *stack.Bundle) { rb.Children = []*stack.Bundle{u} })
	if _, err := generateUnits(t, c, allFlat); err == nil || !strings.Contains(err.Error(), "never") {
		t.Fatalf("got %v, want a reconcile-order refusal (rb waits for u, u waits for rb)", err)
	}
}

// TestReconcileOrder_Scope pins what the reconcile-order check models: the
// Kustomizations kure generates, identified by namespace and name, with a
// health check ignored when Wait is set, as Flux ignores it.
func TestReconcileOrder_Scope(t *testing.T) {
	yes := true
	t.Run("other namespace, same name", func(t *testing.T) {
		c := mergedCluster(func(rb, _, _ *stack.Bundle) {
			rb.HealthChecks = []stack.HealthCheck{{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization", Name: "rb", Namespace: "other"}}
		})
		if _, err := generateUnits(t, c, allFlat); err != nil {
			t.Fatalf("a health check on other/rb is not rb itself: %v", err)
		}
	})
	t.Run("wait ignores health checks", func(t *testing.T) {
		a := srBundle("a", cmApp("a-app"))
		b := srBundle("b", cmApp("b-app"))
		a.Wait = &yes
		a.HealthChecks = []stack.HealthCheck{{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization", Name: "b", Namespace: "flux-system"}}
		b.DependsOn = []*stack.Bundle{a}
		an := &stack.Node{Name: "a", Bundle: a}
		bn := &stack.Node{Name: "b", Bundle: b}
		r := &stack.Node{Name: "r", Children: []*stack.Node{an, bn}}
		an.SetParent(r)
		bn.SetParent(r)
		rules := propertyGroupings["nodeOnly"]
		rules.FluxPlacement = layout.FluxSeparate
		if _, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(&stack.Cluster{Name: "demo", Node: r}, rules); err != nil {
			t.Fatalf("with wait, Flux ignores a's health check on b: %v", err)
		}
	})
}

// TestReconcileOrder_WaitOnRootReachableCR pins that with wait a Kustomization
// waits for every CR it applies, even one the bootstrap also creates: a
// (PerLayout, at the root) waits for b, whose CR it applies, and b depends on
// a.
func TestReconcileOrder_WaitOnRootReachableCR(t *testing.T) {
	yes := true
	b := &stack.Node{Name: "b", Bundle: srBundle("b", cmApp("b-app"))}
	root := &stack.Node{Name: "r", Bundle: srBundle("a", cmApp("a-app")), Children: []*stack.Node{b}}
	root.Bundle.Wait = &yes
	b.Bundle.DependsOn = []*stack.Bundle{root.Bundle}
	b.SetParent(root)
	for _, placement := range []layout.FluxPlacement{layout.FluxSeparate, layout.FluxIntegratedPerLayout} {
		rules := propertyGroupings["nodeOnly"]
		rules.FluxPlacement = placement
		_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(&stack.Cluster{Name: "demo", Node: root}, rules)
		if err == nil || !strings.Contains(err.Error(), "never") {
			t.Errorf("%s: got %v, want a reconcile-order refusal (a waits for b, b depends on a)", placement, err)
		}
	}
}

// TestPerLayout_SharedSourceEffectiveNamespace pins that a bundle-less
// layout's source is chosen by effective SourceRef: an omitted namespace is
// the generator's default, so the two bundles below share one source.
func TestPerLayout_SharedSourceEffectiveNamespace(t *testing.T) {
	a := &stack.Node{Name: "a", Bundle: &stack.Bundle{Name: "a", SourceRef: &stack.SourceRef{Kind: "GitRepository", Name: "repo"}, Applications: []*stack.Application{cmApp("a-app")}}}
	b := &stack.Node{Name: "b", Bundle: &stack.Bundle{Name: "b", SourceRef: &stack.SourceRef{Kind: "GitRepository", Name: "repo", Namespace: "flux-system"}, Applications: []*stack.Application{cmApp("b-app")}}}
	group := &stack.Node{Name: "group", Children: []*stack.Node{a, b}}
	root := &stack.Node{Name: "root", Children: []*stack.Node{group}}
	a.SetParent(group)
	b.SetParent(group)
	group.SetParent(root)
	rules := propertyGroupings["nodeOnly"]
	rules.FluxPlacement = layout.FluxIntegratedPerLayout
	if _, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(&stack.Cluster{Name: "demo", Node: root}, rules); err != nil {
		t.Fatalf("equivalent SourceRefs below a bundle-less layout refused: %v", err)
	}
}

// cycleAugmenter gives its application layout a hooks child, and makes the
// two layout CRs depend on each other.
type cycleAugmenter struct{ app string }

func (c *cycleAugmenter) Generate(*stack.Application) ([]*client.Object, error) {
	return []*client.Object{cmObj(c.app + "-cm")}, nil
}

func (c *cycleAugmenter) AugmentLayout(ml *layout.ManifestLayout) error {
	hooks := &layout.ManifestLayout{
		Name:          c.app + "-hooks",
		Namespace:     ml.FullRepoPath(),
		Resources:     []client.Object{*cmObj(c.app + "-hook")},
		FluxPlacement: ml.FluxPlacement,
		DependsOn:     []string{ml.Name},
	}
	ml.DependsOn = []string{hooks.Name}
	ml.Children = append(ml.Children, hooks)
	return nil
}

// TestIntegrateWithLayout_RepeatedCycleRefusal pins that a refused cycle stays
// refused when the same tree is integrated again: layout CRs an earlier call
// placed are still part of what the check covers.
func TestIntegrateWithLayout_RepeatedCycleRefusal(t *testing.T) {
	yes := true
	cases := map[string]func() (*stack.Cluster, layout.LayoutRules){
		"PerLayout layout CRs": func() (*stack.Cluster, layout.LayoutRules) {
			c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform",
				stack.NewApplication("chart", "default", &cycleAugmenter{app: "chart"}))}}
			rules := propertyGroupings["GroupByName"]
			rules.FluxPlacement = layout.FluxIntegratedPerLayout
			return c, rules
		},
		"Separate wait on the root": func() (*stack.Cluster, layout.LayoutRules) {
			b := &stack.Node{Name: "b", Bundle: srBundle("b", cmApp("b-app"))}
			root := &stack.Node{Name: "r", Bundle: srBundle("a", cmApp("a-app")), Children: []*stack.Node{b}}
			root.Bundle.Wait = &yes
			b.Bundle.DependsOn = []*stack.Bundle{root.Bundle}
			b.SetParent(root)
			rules := propertyGroupings["nodeOnly"]
			rules.FluxPlacement = layout.FluxSeparate
			return &stack.Cluster{Name: "demo", Node: root}, rules
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			c, rules := build()
			ml, err := layout.WalkCluster(c, rules)
			if err != nil {
				t.Fatalf("walk: %v", err)
			}
			integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
			for call := 1; call <= 2; call++ {
				if err := integrator.IntegrateWithLayout(ml, c, rules); err == nil || !strings.Contains(err.Error(), "never") {
					t.Fatalf("call %d: got %v, want the reconcile-order refusal", call, err)
				}
			}
		})
	}
}

// TestFluxSeparate_SharedGeneratedSource pins that bundles sharing one
// URL-bearing SourceRef get that Source once, in every writer and on a repeat
// integration; two different definitions of one Source are refused.
func TestFluxSeparate_SharedGeneratedSource(t *testing.T) {
	shared := func() *stack.SourceRef {
		return &stack.SourceRef{Kind: "GitRepository", Name: "shared", URL: "https://example.com/repo.git", Branch: "main"}
	}
	build := func(bSrc *stack.SourceRef) *stack.Cluster {
		a := &stack.Node{Name: "a", Bundle: &stack.Bundle{Name: "a", SourceRef: shared(), Applications: []*stack.Application{cmApp("a-app")}}}
		b := &stack.Node{Name: "b", Bundle: &stack.Bundle{Name: "b", SourceRef: bSrc, Applications: []*stack.Application{cmApp("b-app")}}}
		r := &stack.Node{Name: "r", Children: []*stack.Node{a, b}}
		a.SetParent(r)
		b.SetParent(r)
		return &stack.Cluster{Name: "demo", Node: r}
	}
	rules := propertyGroupings["nodeOnly"]
	rules.FluxPlacement = layout.FluxSeparate
	integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	c := build(shared())
	ml, err := integrator.CreateLayoutWithResources(c, rules)
	if err != nil {
		t.Fatalf("shared Source: %v", err)
	}
	if err := integrator.IntegrateWithLayout(ml, c, rules); err != nil {
		t.Fatalf("repeat integration: %v", err)
	}
	writeAll(t, ml)

	other := shared()
	other.Branch = "develop"
	if _, err := integrator.CreateLayoutWithResources(build(other), rules); err == nil || !strings.Contains(err.Error(), "shared") {
		t.Errorf("two definitions of GitRepository shared: got %v, want a refusal naming it", err)
	}
}

// TestIntegrateWithLayout_SourceDefaultNamespaceIdentity pins that a Source
// already in its host without a namespace is the same object as the one
// generated in "default": a repeat integration keeps it instead of adding a
// second copy the writers would refuse.
func TestIntegrateWithLayout_SourceDefaultNamespaceIdentity(t *testing.T) {
	gen := fluxstack.NewResourceGenerator()
	gen.DefaultNamespace = "default"
	src := &stack.SourceRef{Kind: "GitRepository", Name: "shared", URL: "https://example.com/repo.git", Branch: "main"}
	c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: &stack.Bundle{Name: "platform", SourceRef: src, Applications: []*stack.Application{cmApp("core")}}}}
	rules := propertyGroupings["nodeOnly"]
	rules.FluxPlacement = layout.FluxIntegratedPerLayout
	integrator := fluxstack.NewLayoutIntegrator(gen)
	ml, err := integrator.CreateLayoutWithResources(c, rules)
	if err != nil {
		t.Fatalf("CreateLayoutWithResources: %v", err)
	}
	sources := func() int {
		n := 0
		var walk func(l *layout.ManifestLayout)
		walk = func(l *layout.ManifestLayout) {
			for _, o := range l.Resources {
				if o.GetObjectKind().GroupVersionKind().Kind == "GitRepository" {
					n++
					o.SetNamespace("")
				}
			}
			for _, ch := range l.Children {
				walk(ch)
			}
		}
		walk(ml)
		return n
	}
	before := sources()
	if err := integrator.IntegrateWithLayout(ml, c, rules); err != nil {
		t.Fatalf("repeat integration: %v", err)
	}
	if after := sources(); after != before {
		t.Errorf("GitRepository count %d -> %d: the Source without a namespace was not recognised", before, after)
	}
	writeAll(t, ml)
}

// TestWriteManifest_PerLayout_ConfigSinglePreservesCRTargets pins that a
// layout a PerLayout Kustomization targets keeps its directory under a Config
// whose ApplicationFileMode is AppFileSingle: that setting is a default for
// application files, not for directories a CR applies.
func TestWriteManifest_PerLayout_ConfigSinglePreservesCRTargets(t *testing.T) {
	c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "root", Bundle: srBundle("root", stack.NewApplication("app", "default", &fakeAppConfig{}))}}
	rules := propertyGroupings["GroupByName"]
	rules.FluxPlacement = layout.FluxIntegratedPerLayout
	ml := integrated(t, c, rules)
	cfg := layout.DefaultLayoutConfig()
	cfg.ApplicationFileMode = layout.AppFileSingle
	out := t.TempDir()
	if err := layout.WriteManifest(out, cfg, ml); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	for _, k := range kustomizations(ml) {
		if _, err := os.Stat(filepath.Join(out, cfg.ManifestsDir, k.Spec.Path, "kustomization.yaml")); err != nil {
			t.Errorf("Kustomization %s targets %q, which WriteManifest did not write as a directory: %v", k.Name, k.Spec.Path, err)
		}
	}
}

// TestGenerateFromLayout_HealthChecksNormalizeDefaultNamespace pins the
// health-check remap with an empty DefaultNamespace: a check on
// "default/b2" names the same generated Kustomization as one on "b2".
func TestGenerateFromLayout_HealthChecksNormalizeDefaultNamespace(t *testing.T) {
	c := mergedCluster(func(rb, b1, _ *stack.Bundle) {
		b1.HealthChecks = []stack.HealthCheck{{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization", Name: "b2", Namespace: "default"}}
	})
	ml, err := layout.WalkCluster(c, allFlat)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	gen := fluxstack.NewResourceGenerator()
	gen.DefaultNamespace = ""
	objs, err := gen.GenerateFromLayout(ml, c)
	if err != nil {
		t.Fatalf("GenerateFromLayout: %v", err)
	}
	for _, o := range objs {
		if k, ok := o.(*kustv1.Kustomization); ok {
			for _, hc := range k.Spec.HealthChecks {
				if hc.Name == "b2" {
					t.Errorf("%s still health-checks b2, which the merge removed", k.Name)
				}
			}
		}
	}
}
