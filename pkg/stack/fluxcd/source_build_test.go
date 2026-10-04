package fluxcd_test

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	fluxkustomize "github.com/fluxcd/pkg/kustomize"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/provider"
	"sigs.k8s.io/kustomize/kyaml/resid"

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

// siblingTree: platform -> [groupA -> web-a, groupB -> web-b]. The two hosts are
// siblings, and platform's build includes both.
func siblingTree() *stack.Cluster {
	groupA := &stack.Node{Name: "groupA", Children: []*stack.Node{{Name: "web-a", Bundle: sharedBundle("web-a")}}}
	groupB := &stack.Node{Name: "groupB", Children: []*stack.Node{{Name: "web-b", Bundle: sharedBundle("web-b")}}}
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

// TestIntegrate_SharedSourceBesideTheRootBundlesCopy: a copy an application
// of the root bundle emits is in the bundle's own directory, a build of its
// own and not the root's (go-kure/kure#979). As for any other bundle
// (TestIntegrate_SharedSourceBesideACopyOutsideTheRootBuild), the integration
// hosts its own at the root and leaves the application's beside it. A copy in
// the root build itself is the one kept: case 10 of
// TestIntegrate_RootBuildChangesHostedSource.
func TestIntegrate_SharedSourceBesideTheRootBundlesCopy(t *testing.T) {
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
	if got, want := sourceCopies(ml, "shared"), map[string]int{"platform": 1, "platform/platform": 1}; !intMapsEqual(got, want) {
		t.Errorf("GitRepository shared hosted %v, want the root's copy and the application's %v", got, want)
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

// TestIntegrate_SharedSourceInSingleFileLayoutCounts: an AppFileSingle
// layout's file is written into its parent's directory, so a copy one holds
// directly below the root node's layout is in the root build, and the
// integration adds no copy of its own there. A walked tree has no application
// layout there (the root node's bundle renders one directory lower,
// go-kure/kure#979), so the layout is the caller's.
func TestIntegrate_SharedSourceInSingleFileLayoutCounts(t *testing.T) {
	for _, placement := range []layout.FluxPlacement{layout.FluxIntegratedPerBundle, layout.FluxIntegratedPerLayout} {
		t.Run(string(placement), func(t *testing.T) {
			web := &stack.Node{Name: "web", Bundle: sharedBundle("web")}
			c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", cmApp("platform-cm")), Children: []*stack.Node{web}}}
			rules := layout.LayoutRules{BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupByName, FluxPlacement: placement}
			ml, err := layout.WalkCluster(c, rules)
			if err != nil {
				t.Fatal(err)
			}
			ml.Children = append(ml.Children, &layout.ManifestLayout{
				Name: "api-src", Namespace: ml.FullRepoPath(), ApplicationFileMode: layout.AppFileSingle,
				Resources: []client.Object{sharedSource(t)},
			})
			if err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules); err != nil {
				t.Fatal(err)
			}
			if got, want := sourceCopies(ml, "shared"), map[string]int{"platform/api-src": 1}; !intMapsEqual(got, want) {
				t.Errorf("GitRepository shared hosted %v, want only the caller's copy %v", got, want)
			}
			checkEveryBuild(t, ml)
		})
	}
}

// TestIntegrate_SharedSourceInTheRootBundlesSingleFileApplication: the same
// application file in the root node's bundle lands in the bundle's directory,
// a build of its own, so the integration hosts its copy at the root beside it.
func TestIntegrate_SharedSourceInTheRootBundlesSingleFileApplication(t *testing.T) {
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
			layoutAtPath(t, ml, "platform/platform/api-src").ApplicationFileMode = layout.AppFileSingle
			if err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules); err != nil {
				t.Fatal(err)
			}
			if got, want := sourceCopies(ml, "shared"), map[string]int{"platform": 1, "platform/platform/api-src": 1}; !intMapsEqual(got, want) {
				t.Errorf("GitRepository shared hosted %v, want the root's copy and the application's %v", got, want)
			}
			checkEveryBuild(t, ml)
		})
	}
}

// sourcePatch is a JSON6902 patch that changes a GitRepository's interval.
const sourcePatch = "- op: replace\n  path: /spec/interval\n  value: 5m\n"

// rootBundleTree is deepTree with a bundle on the root node, whose
// Kustomization builds the bundle's own directory inside the root node's
// layout (go-kure/kure#979), and b's SourceRef URL set to url when url is not
// empty. decorate sets the root bundle's and a's patches or postBuild.
func rootBundleTree(url string, decorate func(root, a *stack.Bundle)) func() *stack.Cluster {
	return func() *stack.Cluster {
		c := deepTree()
		c.Node.Bundle = srBundle("platform", cmApp("platform-app"))
		a := c.Node.Children[0]
		if url != "" {
			a.Children[0].Bundle.SourceRef.URL = url
		}
		decorate(c.Node.Bundle, a.Bundle)
		return c
	}
}

// rootLayoutCRName integrates build() with rules and returns the name of the
// Kustomization that builds dir, the root node's layout.
func rootLayoutCRName(t *testing.T, build func() *stack.Cluster, rules layout.LayoutRules, dir string) string {
	t.Helper()
	for _, k := range kustomizations(integrated(t, build(), rules)) {
		if filepath.Clean(k.Spec.Path) == dir {
			return k.Name
		}
	}
	t.Fatalf("no Kustomization builds %q", dir)
	return ""
}

// TestIntegrate_RootBuildChangesHostedSource: the integration hosts b's Source
// in the root node's layout, the directory the Flux bootstrap applies. A
// Kustomization that builds that directory with a patch which applies to the
// Source, or a postBuild that substitutes into it, would make the two apply
// the Source differently: refused, the tree untouched (go-kure/kure#908).
// Anything else is accepted, and the Source that Kustomization's Flux build
// applies is the one the bootstrap's build applies.
//
// The root bundle's Kustomization is no such build: the bundle has a
// directory of its own inside the root node's layout (go-kure/kure#979). Each
// case is run with its patch or postBuild on the root bundle, where it is
// accepted and the bundle's build holds no Source, in both placements. The one
// build of the root node's layout the integration places or keeps is its
// layout Kustomization, under FluxIntegratedPerLayout below a ClusterName
// wrapper: each case is run again with the root bundle's patches and postBuild
// on a copy of that Kustomization, which the integration keeps.
func TestIntegrate_RootBuildChangesHostedSource(t *testing.T) {
	const templatedURL = "https://${GIT_HOST}/shared.git"
	rootPatch := func(p stack.Patch) func(root, a *stack.Bundle) {
		return func(root, _ *stack.Bundle) { root.Patches = []stack.Patch{p} }
	}
	rootPostBuild := func(pb *stack.PostBuild) func(root, a *stack.Bundle) {
		return func(root, _ *stack.Bundle) { root.PostBuild = pb }
	}
	sourceKind := stack.Patch{Patch: sourcePatch, Target: &stack.PatchSelector{Kind: "GitRepository"}}
	for _, tc := range []struct {
		name  string
		build func() *stack.Cluster
		// callerCopy puts the caller's copy of the Source in the root
		// before integrating.
		callerCopy bool
		// refused is what the refusal names besides the CR and the
		// Source; empty means accepted.
		refused string
		// layoutPatch replaces the root bundle's patch on the root node's
		// layout Kustomization, whose build holds none of the bundle's
		// objects: a target-less patch must name an object of that build.
		layoutPatch string
	}{
		{name: "1 patch selecting the Source by kind", build: rootBundleTree("", rootPatch(sourceKind)), refused: "patch 0 (target {kind: GitRepository}) selects it"},
		{name: "2 patch selecting the Source by kind regex", build: rootBundleTree("", rootPatch(stack.Patch{Patch: sourcePatch, Target: &stack.PatchSelector{Kind: "Git.*"}})), refused: "patch 0"},
		{name: "3 patch selecting another kind", build: rootBundleTree("", rootPatch(stack.Patch{Patch: sourcePatch, Target: &stack.PatchSelector{Kind: "Deployment"}}))},
		{name: "4 patch selecting by a label the Source lacks", build: rootBundleTree("", rootPatch(stack.Patch{Patch: sourcePatch, Target: &stack.PatchSelector{LabelSelector: "app=x"}}))},
		{name: "5 target-less patch naming the Source", build: rootBundleTree("", rootPatch(stack.Patch{Patch: "apiVersion: source.toolkit.fluxcd.io/v1\nkind: GitRepository\nmetadata:\n  name: shared\n  namespace: flux-system\nspec:\n  interval: 5m\n"})), refused: "patch 0 (no target) names it"},
		{name: "6 target-less patch naming another object", build: rootBundleTree("", rootPatch(stack.Patch{Patch: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: platform-app-cm\n  namespace: default\ndata:\n  patched: \"true\"\n"})),
			layoutPatch: "apiVersion: kustomize.toolkit.fluxcd.io/v1\nkind: Kustomization\nmetadata:\n  name: a\n  namespace: flux-system\nspec:\n  interval: 5m\n"},
		{name: "7 postBuild over a plain Source", build: rootBundleTree("", rootPostBuild(&stack.PostBuild{Substitute: map[string]string{"GIT_HOST": "git.example.com"}}))},
		{name: "8 postBuild substituting into the Source", build: rootBundleTree(templatedURL, rootPostBuild(&stack.PostBuild{Substitute: map[string]string{"GIT_HOST": "git.example.com"}})), refused: "postBuild substitution changes it"},
		{name: "8b postBuild substituteFrom only", build: rootBundleTree(templatedURL, rootPostBuild(&stack.PostBuild{SubstituteFrom: []stack.SubstituteRef{{Kind: "ConfigMap", Name: "cluster-vars"}}})), refused: "postBuild substitution reads GIT_HOST, which the inline vars do not set and substituteFrom can"},
		// $$ escapes a $ and reads no var: only running the substitution
		// without inline vars, as Flux does with substituteFrom set, shows
		// that it changes the URL.
		{name: "8e postBuild substituteFrom only, escaped expression", build: rootBundleTree("https://git.example.com/$${REPO}.git", rootPostBuild(&stack.PostBuild{SubstituteFrom: []stack.SubstituteRef{{Kind: "ConfigMap", Name: "cluster-vars"}}})), refused: "postBuild substitution changes it"},
		// The inline PREFIX reproduces the expression offline, but a SUFFIX
		// from the cluster's ConfigMap would change the URL.
		{name: "8c postBuild substituteFrom var the offline result hides", build: rootBundleTree("https://git.example.com/${PREFIX}${SUFFIX}.git", rootPostBuild(&stack.PostBuild{
			Substitute:     map[string]string{"PREFIX": "${PREFIX}${SUFFIX}"},
			SubstituteFrom: []stack.SubstituteRef{{Kind: "ConfigMap", Name: "cluster-vars"}},
		})), refused: "postBuild substitution reads SUFFIX, which the inline vars do not set and substituteFrom can"},
		// An inline var overrides substituteFrom's, so an expression reading
		// only inline vars is decided offline: here it is unchanged.
		{name: "8d postBuild substituteFrom with an inline var that keeps the Source", build: rootBundleTree("https://git.example.com/${REPO}.git", rootPostBuild(&stack.PostBuild{
			Substitute:     map[string]string{"REPO": "${REPO}"},
			SubstituteFrom: []stack.SubstituteRef{{Kind: "ConfigMap", Name: "cluster-vars"}},
		}))},
		{name: "9 the patch on a bundle below the root", build: rootBundleTree("", func(_, a *stack.Bundle) { a.Patches = []stack.Patch{sourceKind} })},
		{name: "10 the patch selecting the caller's copy in the root", build: rootBundleTree("", rootPatch(sourceKind)), callerCopy: true},
	} {
		for _, placement := range []layout.FluxPlacement{layout.FluxIntegratedPerBundle, layout.FluxIntegratedPerLayout} {
			t.Run("root bundle/"+string(placement)+"/"+tc.name, func(t *testing.T) {
				rules := propertyGroupings["nodeOnly"]
				rules.FluxPlacement = placement
				integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
				switch {
				case tc.callerCopy:
					// checkIdempotent rebuilds from the cluster, which drops
					// the caller's copy; the root CR's patch changes that
					// copy by design, so it is not compared either.
					c := tc.build()
					ml, err := layout.WalkCluster(c, rules)
					if err != nil {
						t.Fatal(err)
					}
					callers := sharedSource(t)
					ml.Resources = append(ml.Resources, callers)
					count := 0
					for i := range 2 {
						if err := integrator.IntegrateWithLayout(ml, c, rules); err != nil {
							t.Fatalf("IntegrateWithLayout %d: %v", i+1, err)
						}
						if i == 0 {
							count = countResources(ml)
						} else if after := countResources(ml); after != count {
							t.Errorf("second IntegrateWithLayout changed the resource count %d -> %d", count, after)
						}
						if got, want := sourceCopies(ml, "shared"), map[string]int{"platform": 1}; !intMapsEqual(got, want) {
							t.Errorf("integration %d: GitRepository shared hosted %v, want only the caller's copy %v", i+1, got, want)
						}
						if !slices.Contains(ml.Resources, callers) {
							t.Errorf("integration %d dropped the caller's copy from the root", i+1)
						}
					}
					checkEveryBuild(t, ml)
				default:
					ml := integrated(t, tc.build(), rules)
					if got, want := sourceCopies(ml, "shared"), map[string]int{"platform": 1}; !intMapsEqual(got, want) {
						t.Errorf("GitRepository shared hosted %v, want %v", got, want)
					}
					checkEveryBuild(t, ml)
					checkIdempotent(t, tc.build(), rules)
					checkRootBundleBuildHoldsNoSource(t, ml)
				}
			})
		}

		t.Run("root node's layout/"+tc.name, func(t *testing.T) {
			const rootDir = "prod/platform"
			rules := propertyGroupings["nodeOnly"]
			rules.FluxPlacement = layout.FluxIntegratedPerLayout
			rules.ClusterName = "prod"
			var bundleCR *kustv1.Kustomization
			for _, k := range kustomizations(integrated(t, tc.build(), rules)) {
				if k.Name == "platform" {
					bundleCR = k
				}
			}
			if bundleCR == nil {
				t.Fatal("no Kustomization for the root bundle")
			}
			rootCR := rootLayoutCRName(t, tc.build, rules, rootDir)
			ml, c := keptIn(t, tc.build, rules, rootCR, "typed", func(k *kustv1.Kustomization) {
				k.Spec.Patches = bundleCR.Spec.Patches
				k.Spec.PostBuild = bundleCR.Spec.PostBuild
				if tc.layoutPatch != "" {
					k.Spec.Patches = []kustomize.Patch{{Patch: tc.layoutPatch}}
				}
			}, nil)
			var callers client.Object
			if tc.callerCopy {
				callers = sharedSource(t)
				root := layoutAtPath(t, ml, rootDir)
				root.Resources = append(root.Resources, callers)
			}
			before := countResources(ml)
			err := integrate(ml, c, rules)
			if tc.refused != "" && !tc.callerCopy {
				if err == nil {
					t.Fatalf("got no error, want a refusal naming %q", tc.refused)
				}
				for _, want := range []string{fmt.Sprintf("Flux Kustomization %q", rootCR), `GitRepository "shared"`, tc.refused} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("refusal %q does not name %q", err, want)
					}
				}
				if after := countResources(ml); after != before {
					t.Errorf("refused integration left %d resources, want the %d before it", after, before)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got, want := sourceCopies(ml, "shared"), map[string]int{rootDir: 1}; !intMapsEqual(got, want) {
				t.Errorf("GitRepository shared hosted %v, want %v", got, want)
			}
			checkEveryBuild(t, ml)
			if tc.callerCopy {
				// The patch changes the caller's copy by design, so the two
				// builds are not compared.
				if !slices.Contains(layoutAtPath(t, ml, rootDir).Resources, callers) {
					t.Error("the integration dropped the caller's copy from the root node's layout")
				}
				return
			}
			checkCRAppliesTheBootstrapSource(t, ml, rootDir)
		})
	}
}

// sharedSourceID is the kustomize identity of the GitRepository sharedSR
// names.
func sharedSourceID() string {
	return resid.NewResIdWithNamespace(resid.NewGvk(sourcev1.GroupVersion.Group, sourcev1.GroupVersion.Version, "GitRepository"), "shared", "flux-system").String()
}

// checkRootBundleBuildHoldsNoSource Flux-builds the directory the root
// bundle's Kustomization names, which is not the root node's, and requires it
// to hold no copy of the hosted Source: whatever patches or postBuild that
// Kustomization has, it does not apply the Source the bootstrap applies. The
// directory is built without the Kustomization's patches: one that names the
// Source matches nothing there, and kustomize refuses such a build.
func checkRootBundleBuildHoldsNoSource(t *testing.T, ml *layout.ManifestLayout) {
	t.Helper()
	var cr *kustv1.Kustomization
	for _, k := range kustomizations(ml) {
		if k.Name == "platform" {
			cr = k
		}
		if filepath.Clean(k.Spec.Path) == ml.FullRepoPath() {
			t.Errorf("Kustomization %q builds the root node's layout %q", k.Name, k.Spec.Path)
		}
	}
	if cr == nil {
		t.Fatal("no Kustomization for the root bundle")
	}
	if want := ml.FullRepoPath() + "/platform"; filepath.Clean(cr.Spec.Path) != want {
		t.Fatalf("the root bundle's Kustomization builds %q, want %q", cr.Spec.Path, want)
	}
	for writer, w := range writeAll(t, ml) {
		if built := fluxBuild(t, w.root, cr.Spec.Path, unstructured.Unstructured{Object: map[string]any{}}); built[sharedSourceID()] != "" {
			t.Errorf("%s: the root bundle's build holds %s", writer, sharedSourceID())
		}
	}
}

// checkCRAppliesTheBootstrapSource Flux-builds dir, the root node's directory,
// twice, as the bootstrap does (no patches, no postBuild) and as the
// Kustomization that names it does, and requires the hosted Source to come out
// the same. fluxBuild runs no postBuild, so when the Kustomization has one
// every object it built is passed through Flux's substitution with its vars
// (dry run, and always when substituteFrom is set: its values are in the
// cluster), and the Source is compared parsed, as the substitution round-trips
// it through JSON.
func checkCRAppliesTheBootstrapSource(t *testing.T, ml *layout.ManifestLayout, dir string) {
	t.Helper()
	var cr *kustv1.Kustomization
	for _, k := range kustomizations(ml) {
		if filepath.Clean(k.Spec.Path) == dir {
			cr = k
		}
	}
	if cr == nil {
		t.Fatalf("no Kustomization builds %q", dir)
	}
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cr)
	if err != nil {
		t.Fatal(err)
	}
	kust := unstructured.Unstructured{Object: content}
	id := sharedSourceID()
	rf := provider.NewDefaultDepProvider().GetResourceFactory()
	parse := func(y string) map[string]any {
		t.Helper()
		res, err := rf.FromBytes([]byte(y))
		if err != nil {
			t.Fatal(err)
		}
		m, err := res.Map()
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	for writer, w := range writeAll(t, ml) {
		bootstrap := fluxBuild(t, w.root, dir, unstructured.Unstructured{Object: map[string]any{}})[id]
		built := fluxBuild(t, w.root, cr.Spec.Path, kust)
		if bootstrap == "" || built[id] == "" {
			t.Errorf("%s: the bootstrap build or %q's holds no %s", writer, cr.Name, id)
			continue
		}
		if cr.Spec.PostBuild == nil {
			if built[id] != bootstrap {
				t.Errorf("%s: %q applies the Source as\n%s\nthe bootstrap as\n%s", writer, cr.Name, built[id], bootstrap)
			}
			continue
		}
		opts := []fluxkustomize.SubstituteOption{fluxkustomize.SubstituteWithDryRun(true)}
		if len(cr.Spec.PostBuild.SubstituteFrom) > 0 {
			opts = append(opts, fluxkustomize.SubstituteWithAlways(true))
		}
		var got map[string]any
		for objID, y := range built {
			res, err := rf.FromBytes([]byte(y))
			if err != nil {
				t.Fatal(err)
			}
			out, err := fluxkustomize.SubstituteVariables(context.Background(), nil, kust, res, opts...)
			if err != nil {
				t.Errorf("%s: postBuild of %s: %v", writer, objID, err)
				continue
			}
			if out == nil {
				out = res
			}
			if objID == id {
				if got, err = out.Map(); err != nil {
					t.Fatal(err)
				}
			}
		}
		if want := parse(bootstrap); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %q applies the Source as %v after postBuild, the bootstrap as %v", writer, cr.Name, got, want)
		}
	}
}
