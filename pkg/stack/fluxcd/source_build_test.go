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

// nestedTree: platform -> a (bundle) -> b (bundle). a's Kustomization is in
// platform and b's in platform/a: two separate builds (go-kure/kure#876).
func nestedTree() *stack.Cluster {
	b := &stack.Node{Name: "b", Bundle: sharedBundle("b")}
	a := &stack.Node{Name: "a", Bundle: sharedBundle("a"), Children: []*stack.Node{b}}
	return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Children: []*stack.Node{a}}}
}

// siblingBuildsTree: platform -> [a (bundle) -> x (bundle), c (bundle) -> y
// (bundle)]. Only x and y use the shared Source, and their Kustomizations are
// in platform/a and platform/c: two sibling builds, neither of which the root
// build includes.
func siblingBuildsTree() *stack.Cluster {
	x := &stack.Node{Name: "x", Bundle: sharedBundle("x")}
	y := &stack.Node{Name: "y", Bundle: sharedBundle("y")}
	a := &stack.Node{Name: "a", Bundle: srBundle("a", cmApp("a-app")), Children: []*stack.Node{x}}
	c := &stack.Node{Name: "c", Bundle: srBundle("c", cmApp("c-app")), Children: []*stack.Node{y}}
	return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Children: []*stack.Node{a, c}}}
}

// deepTree: platform -> a (bundle) -> b (bundle), where only b uses the shared
// Source. Its one consumer's Kustomization is in platform/a.
func deepTree() *stack.Cluster {
	b := &stack.Node{Name: "b", Bundle: sharedBundle("b")}
	a := &stack.Node{Name: "a", Bundle: srBundle("a", cmApp("a-app")), Children: []*stack.Node{b}}
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
			if c != nil {
				walk(c)
			}
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

// TestIntegrate_SharedSourceAtRoot: every Source the integration derives is
// hosted once, in the root node's layout, whichever layouts host the
// Kustomizations that use it (go-kure/kure#876). The Flux bootstrap applies
// the root, so the Source is in one build, and it exists before any
// Kustomization using it: each of those is applied by the root build or by a
// build below it.
func TestIntegrate_SharedSourceAtRoot(t *testing.T) {
	for _, placement := range []layout.FluxPlacement{layout.FluxIntegratedPerBundle, layout.FluxIntegratedPerLayout} {
		for _, tc := range []struct {
			name  string
			build func() *stack.Cluster
			// perBundleOnly: FluxIntegratedPerLayout refuses the tree, as
			// the bundle-less group's own CR has no SourceRef to use.
			perBundleOnly bool
		}{
			{"issue873", issue873Tree, true},
			{"siblings", siblingTree, true},
			{"separate builds", nestedTree, false},
			{"sibling builds", siblingBuildsTree, false},
			{"one deep consumer", deepTree, false},
		} {
			if tc.perBundleOnly && placement == layout.FluxIntegratedPerLayout {
				continue
			}
			t.Run(string(placement)+"/"+tc.name, func(t *testing.T) {
				rules := propertyGroupings["nodeOnly"]
				rules.FluxPlacement = placement
				ml := integrated(t, tc.build(), rules)
				if got, want := sourceCopies(ml, "shared"), map[string]int{"platform": 1}; !intMapsEqual(got, want) {
					t.Errorf("GitRepository shared hosted %v, want %v", got, want)
				}
				checkEveryBuild(t, ml)
				checkIdempotent(t, tc.build(), rules)
			})
		}
	}
}

// TestIntegrate_SharedSourceAtRootNodeUnderClusterName: with a ClusterName
// the walker keeps a wrapper layout above the root node's. The bootstrap sync
// path, ./<root>, names the root node's directory, not the wrapper's, so the
// Source goes there.
func TestIntegrate_SharedSourceAtRootNodeUnderClusterName(t *testing.T) {
	for _, placement := range []layout.FluxPlacement{layout.FluxIntegratedPerBundle, layout.FluxIntegratedPerLayout} {
		for clusterName, want := range map[string]string{".": "platform", "prod": "prod/platform"} {
			t.Run(string(placement)+"/"+clusterName, func(t *testing.T) {
				rules := propertyGroupings["nodeOnly"]
				rules.FluxPlacement = placement
				rules.ClusterName = clusterName
				ml := integrated(t, deepTree(), rules)
				if got, want := sourceCopies(ml, "shared"), map[string]int{want: 1}; !intMapsEqual(got, want) {
					t.Errorf("GitRepository shared hosted %v, want %v", got, want)
				}
				checkEveryBuild(t, ml)
				checkIdempotent(t, deepTree(), rules)
			})
		}
	}
}

// TestIntegrate_SharedSourceCopyInTheClusterNameWrapper: under PerBundle the
// wrapper's kustomization.yaml lists the root node's directory, so a caller
// copy in the wrapper shares a build with the integration's root copy. Both
// cannot stay, the caller's is not dropped, and without the root's copy no
// build the bootstrap applies holds the Source: refused, the tree untouched.
// Under PerLayout the root node's directory is its own layout CR's build, so
// the caller's copy is in another build and kept beside the integration's.
func TestIntegrate_SharedSourceCopyInTheClusterNameWrapper(t *testing.T) {
	for placement, want := range map[layout.FluxPlacement]map[string]int{
		layout.FluxIntegratedPerBundle: nil,
		layout.FluxIntegratedPerLayout: {".": 1, "platform": 1},
	} {
		t.Run(string(placement), func(t *testing.T) {
			rules := propertyGroupings["nodeOnly"]
			rules.FluxPlacement = placement
			rules.ClusterName = "."
			c := deepTree()
			ml, err := layout.WalkCluster(c, rules)
			if err != nil {
				t.Fatal(err)
			}
			ml.Resources = append(ml.Resources, sharedSource(t))
			before := countResources(ml)
			err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
			if want == nil {
				if err == nil || !strings.Contains(err.Error(), `layout "." holds GitRepository "shared"`) || !strings.Contains(err.Error(), `hosts in "platform"`) {
					t.Fatalf("caller copy in the wrapper's build: got %v, want a refusal naming both layouts", err)
				}
				if after := countResources(ml); after != before {
					t.Errorf("refused integration left %d resources, want the caller's %d", after, before)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := sourceCopies(ml, "shared"); !intMapsEqual(got, want) {
				t.Errorf("GitRepository shared hosted %v, want %v", got, want)
			}
			checkEveryBuild(t, ml)
			checkIdempotent(t, c, rules)
		})
	}
}

// TestIntegrate_SharedSourceKeepsTheTreesCopy: a Source already in the root
// build — here one an application of the root bundle emits — is the one kept;
// the integration adds no copy of its own.
func TestIntegrate_SharedSourceKeepsTheTreesCopy(t *testing.T) {
	build := func(t *testing.T) *stack.Cluster {
		src := sharedSource(t)
		emitter := stack.NewApplication("api-src", "default", &fakeAppConfig{objs: []*client.Object{&src}})
		web := &stack.Node{Name: "web", Bundle: sharedBundle("web")}
		group := &stack.Node{Name: "group", Children: []*stack.Node{web}}
		return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", emitter), Children: []*stack.Node{group}}}
	}
	rules := propertyGroupings["nodeOnly"]
	rules.FluxPlacement = layout.FluxIntegratedPerBundle
	ml := integrated(t, build(t), rules)
	if got, want := sourceCopies(ml, "shared"), map[string]int{"platform": 1}; !intMapsEqual(got, want) {
		t.Errorf("GitRepository shared hosted %v, want only the application's copy %v", got, want)
	}
	checkEveryBuild(t, ml)
	checkIdempotent(t, build(t), rules)
}

// TestIntegrate_SharedSourceBesideACopyOutsideTheRootBuild: a copy an
// application emits in a build other than the root's is the application's to
// keep; the integration still hosts its own at the root, which applies before
// any Kustomization that uses it. The application's copy is then a second
// owner, which the integration does not remove.
func TestIntegrate_SharedSourceBesideACopyOutsideTheRootBuild(t *testing.T) {
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
	if got, want := sourceCopies(ml, "shared"), map[string]int{"platform": 1, "platform/api": 1}; !intMapsEqual(got, want) {
		t.Errorf("GitRepository shared hosted %v, want the root's copy and the application's %v", got, want)
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
// holds in the root's application is in the root build, and the integration
// adds no copy of its own there.
func TestIntegrate_SharedSourceInSingleFileApplicationCounts(t *testing.T) {
	for _, placement := range []layout.FluxPlacement{layout.FluxIntegratedPerBundle, layout.FluxIntegratedPerLayout} {
		t.Run(string(placement), func(t *testing.T) {
			src := sharedSource(t)
			emitter := stack.NewApplication("api-src", "default", &fakeAppConfig{objs: []*client.Object{&src}})
			web := &stack.Node{Name: "web", Bundle: sharedBundle("web")}
			c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", emitter, cmApp("platform-cm")), Children: []*stack.Node{web}}}
			rules := layout.LayoutRules{BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupByName, FluxPlacement: placement}
			ml, err := layout.WalkCluster(c, rules)
			if err != nil {
				t.Fatal(err)
			}
			layoutAtPath(t, ml, "platform/api-src").ApplicationFileMode = layout.AppFileSingle
			if err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules); err != nil {
				t.Fatal(err)
			}
			if got, want := sourceCopies(ml, "shared"), map[string]int{"platform/api-src": 1}; !intMapsEqual(got, want) {
				t.Errorf("GitRepository shared hosted %v, want only the application's copy %v", got, want)
			}
			checkEveryBuild(t, ml)
		})
	}
}
