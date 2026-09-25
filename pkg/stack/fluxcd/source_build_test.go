package fluxcd_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for hosting a Source that several bundles share once per kustomize
// build: kustomize refuses one object twice, so an identical Source in two
// layouts one build includes breaks that build, whichever layouts host it.

func sharedSR() *stack.SourceRef {
	return &stack.SourceRef{Kind: "GitRepository", Name: "shared", Namespace: "flux-system", URL: "https://example.com/shared.git", Branch: "main"}
}

func sharedBundle(name string) *stack.Bundle {
	return &stack.Bundle{Name: name, SourceRef: sharedSR(), Applications: []*stack.Application{cmApp(name + "-app")}}
}

// sharedSource is the GitRepository the generator derives from sharedSR.
func sharedSource(t *testing.T) client.Object {
	t.Helper()
	objs, err := fluxstack.NewResourceGenerator().GenerateForBundle(sharedBundle("x"), "x")
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range objs {
		if _, ok := o.(*sourcev1.GitRepository); ok {
			return o
		}
	}
	t.Fatal("no GitRepository generated")
	return nil
}

// issue873Tree: platform -> [api (bundle), group (no bundle) -> web (bundle)].
// platform hosts api's Source, group hosts web's, and platform's
// kustomization.yaml lists group.
func issue873Tree() *stack.Cluster {
	web := &stack.Node{Name: "web", Bundle: sharedBundle("web")}
	group := &stack.Node{Name: "group", Children: []*stack.Node{web}}
	api := &stack.Node{Name: "api", Bundle: sharedBundle("api")}
	return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Children: []*stack.Node{api, group}}}
}

// siblingTree: platform -> [groupA -> webA, groupB -> webB]. The two hosts are
// siblings, and platform's build includes both.
func siblingTree() *stack.Cluster {
	groupA := &stack.Node{Name: "groupA", Children: []*stack.Node{{Name: "webA", Bundle: sharedBundle("webA")}}}
	groupB := &stack.Node{Name: "groupB", Children: []*stack.Node{{Name: "webB", Bundle: sharedBundle("webB")}}}
	return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Children: []*stack.Node{groupA, groupB}}}
}

// nestedTree: platform -> a (bundle) -> b (bundle). platform hosts a's Source
// and platform/a hosts b's: two separate builds, each hosting its own copy.
func nestedTree() *stack.Cluster {
	b := &stack.Node{Name: "b", Bundle: sharedBundle("b")}
	a := &stack.Node{Name: "a", Bundle: sharedBundle("a"), Children: []*stack.Node{b}}
	return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Children: []*stack.Node{a}}}
}

// sourceCopies maps each layout path to the number of GitRepository objects
// named name it hosts.
func sourceCopies(ml *layout.ManifestLayout, name string) map[string]int {
	out := map[string]int{}
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		for _, r := range l.Resources {
			if r != nil && r.GetObjectKind().GroupVersionKind().Kind == "GitRepository" && r.GetName() == name {
				out[l.FullRepoPath()]++
			}
		}
		for _, c := range l.Children {
			walk(c)
		}
	}
	walk(ml)
	return out
}

// checkEveryBuild writes ml with every writer and kustomize-builds the root
// directory (applied by the Flux bootstrap) and every spec.path.
func checkEveryBuild(t *testing.T, ml *layout.ManifestLayout) {
	t.Helper()
	for writer, w := range writeAll(t, ml) {
		var dirs []string
		for _, top := range w.tops {
			dirs = append(dirs, filepath.Dir(top))
		}
		for _, k := range kustomizations(ml) {
			dirs = append(dirs, filepath.Join(w.root, k.Spec.Path))
		}
		for _, d := range dirs {
			if err := kustomizeBuild(d); err != nil {
				rel, _ := filepath.Rel(w.root, d)
				t.Errorf("%s: kustomize build %q: %v", writer, rel, err)
			}
		}
	}
}

func TestIntegrate_SharedSourceOncePerBuild(t *testing.T) {
	perBundle := propertyGroupings["nodeOnly"]
	perBundle.FluxPlacement = layout.FluxIntegratedPerBundle
	for _, tc := range []struct {
		name  string
		build func() *stack.Cluster
		want  map[string]int
	}{
		// platform's build holds both hosts: one copy, in the first.
		{"issue873", issue873Tree, map[string]int{"platform": 1}},
		{"siblings", siblingTree, map[string]int{"platform/groupA": 1}},
		// Two builds: each keeps the copy it hosts.
		{"separate builds", nestedTree, map[string]int{"platform": 1, "platform/a": 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ml := integrated(t, tc.build(), perBundle)
			if got := sourceCopies(ml, "shared"); !intMapsEqual(got, tc.want) {
				t.Errorf("GitRepository shared hosted %v, want %v", got, tc.want)
			}
			checkEveryBuild(t, ml)
			checkIdempotent(t, tc.build(), perBundle)
		})
	}
}

// TestIntegrate_SharedSourceKeepsTheTreesCopy: a Source already in a build —
// here one an application emits — is the one that build keeps; the copy the
// integration would add to another layout of that build is not added.
func TestIntegrate_SharedSourceKeepsTheTreesCopy(t *testing.T) {
	build := func(t *testing.T) *stack.Cluster {
		src := sharedSource(t)
		emitter := stack.NewApplication("api-src", "default", &fakeAppConfig{objs: []*client.Object{&src}})
		web := &stack.Node{Name: "web", Bundle: sharedBundle("web")}
		group := &stack.Node{Name: "group", Children: []*stack.Node{web}}
		api := &stack.Node{Name: "api", Bundle: srBundle("api", emitter), Children: []*stack.Node{group}}
		return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Children: []*stack.Node{api}}}
	}
	rules := propertyGroupings["nodeOnly"]
	rules.FluxPlacement = layout.FluxIntegratedPerBundle
	ml := integrated(t, build(t), rules)
	if got, want := sourceCopies(ml, "shared"), map[string]int{"platform/api": 1}; !intMapsEqual(got, want) {
		t.Errorf("GitRepository shared hosted %v, want only the application's copy %v", got, want)
	}
	checkEveryBuild(t, ml)
	checkIdempotent(t, build(t), rules)
}

// TestIntegrate_SharedSourceTwiceInOneBuildRefused: two copies the caller put
// in layouts one build includes cannot be reduced to one by the integration,
// and platform's kustomization.yaml would build both.
func TestIntegrate_SharedSourceTwiceInOneBuildRefused(t *testing.T) {
	rules := propertyGroupings["nodeOnly"]
	rules.FluxPlacement = layout.FluxIntegratedPerBundle
	c := issue873Tree()
	ml, err := layout.WalkCluster(c, rules)
	if err != nil {
		t.Fatal(err)
	}
	group := layoutAtPath(t, ml, "platform/group")
	ml.Resources = append(ml.Resources, sharedSource(t))
	group.Resources = append(group.Resources, sharedSource(t))
	before := countResources(ml)
	err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
	if err == nil || !strings.Contains(err.Error(), `"platform"`) || !strings.Contains(err.Error(), `"platform/group"`) {
		t.Fatalf("two caller copies of GitRepository shared in platform's build: got %v, want a refusal naming both layouts", err)
	}
	if after := countResources(ml); after != before {
		t.Errorf("refused integration left %d resources, want the caller's %d", after, before)
	}
}

func intMapsEqual(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// TestIntegrate_SharedSourceKeepsTheCallersDeeperCopy: the caller's copy is
// kept even where it is not the first in the build; the integration drops
// its own copy instead, whichever layout it placed it in.
func TestIntegrate_SharedSourceKeepsTheCallersDeeperCopy(t *testing.T) {
	rules := propertyGroupings["nodeOnly"]
	rules.FluxPlacement = layout.FluxIntegratedPerBundle
	c := issue873Tree()
	ml, err := layout.WalkCluster(c, rules)
	if err != nil {
		t.Fatal(err)
	}
	group := layoutAtPath(t, ml, "platform/group")
	callers := sharedSource(t)
	group.Resources = append(group.Resources, callers)
	integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	for i := range 2 {
		if err := integrator.IntegrateWithLayout(ml, c, rules); err != nil {
			t.Fatalf("IntegrateWithLayout %d: %v", i+1, err)
		}
		if got, want := sourceCopies(ml, "shared"), map[string]int{"platform/group": 1}; !intMapsEqual(got, want) {
			t.Errorf("integration %d: GitRepository shared hosted %v, want only the caller's copy %v", i+1, got, want)
		}
		if !slices.Contains(group.Resources, callers) {
			t.Errorf("integration %d dropped the caller's copy from platform/group", i+1)
		}
	}
	checkEveryBuild(t, ml)
}

// TestIntegrate_SharedSourceInSingleFileApplicationCounts: an AppFileSingle
// application's file is written into its parent's directory, so a copy it
// holds is in the parent's build, and the integration keeps no copy of its own
// there.
func TestIntegrate_SharedSourceInSingleFileApplicationCounts(t *testing.T) {
	for _, placement := range []layout.FluxPlacement{layout.FluxIntegratedPerBundle, layout.FluxIntegratedPerLayout} {
		t.Run(string(placement), func(t *testing.T) {
			src := sharedSource(t)
			emitter := stack.NewApplication("api-src", "default", &fakeAppConfig{objs: []*client.Object{&src}})
			web := &stack.Node{Name: "web", Bundle: sharedBundle("web")}
			api := &stack.Node{Name: "api", Bundle: srBundle("api", emitter, cmApp("api-cm")), Children: []*stack.Node{web}}
			c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Children: []*stack.Node{api}}}
			rules := layout.LayoutRules{BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupByName, FluxPlacement: placement}
			ml, err := layout.WalkCluster(c, rules)
			if err != nil {
				t.Fatal(err)
			}
			layoutAtPath(t, ml, "platform/api/api-src").ApplicationFileMode = layout.AppFileSingle
			if err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules); err != nil {
				t.Fatal(err)
			}
			if got, want := sourceCopies(ml, "shared"), map[string]int{"platform/api/api-src": 1}; !intMapsEqual(got, want) {
				t.Errorf("GitRepository shared hosted %v, want only the application's copy %v", got, want)
			}
			checkEveryBuild(t, ml)
		})
	}
}
