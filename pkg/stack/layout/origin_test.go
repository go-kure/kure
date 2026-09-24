package layout_test

import (
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for layout origins: every walked layout records the stack objects it
// renders, and IndexOrigins resolves a bundle to the one directory that holds
// its resources. That directory is the only source of a Flux or ArgoCD path.

var (
	groupByName = layout.LayoutRules{BundleGrouping: layout.GroupByName, ApplicationGrouping: layout.GroupByName}
	nodeOnly    = layout.LayoutRules{BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupFlat}
	nodeFlat    = layout.LayoutRules{NodeGrouping: layout.GroupFlat, BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupFlat}
)

func withClusterName(r layout.LayoutRules, name string) layout.LayoutRules {
	r.ClusterName = name
	return r
}

// layoutAt returns the layout in the tree whose FullRepoPath is p, or fails.
func layoutAt(t *testing.T, root *layout.ManifestLayout, p string) *layout.ManifestLayout {
	t.Helper()
	var found *layout.ManifestLayout
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		if l.FullRepoPath() == p && found == nil {
			found = l
		}
		for _, c := range l.Children {
			walk(c)
		}
	}
	walk(root)
	if found == nil {
		t.Fatalf("no layout at %q; have %v", p, collectRepoPaths(root))
	}
	return found
}

func bundleNames(bs []*stack.Bundle) []string {
	var out []string
	for _, b := range bs {
		out = append(out, b.Name)
	}
	return out
}

func nodeNames(ns []*stack.Node) []string {
	var out []string
	for _, n := range ns {
		out = append(out, n.Name)
	}
	return out
}

// assertOrigin checks the nodes and bundles a layout records, by name and in
// order.
func assertOrigin(t *testing.T, l *layout.ManifestLayout, nodes, bundles []string) {
	t.Helper()
	if got := nodeNames(l.OriginNodes()); !slices.Equal(got, nodes) {
		t.Errorf("%s: OriginNodes = %v, want %v", l.FullRepoPath(), got, nodes)
	}
	if got := bundleNames(l.OriginBundles()); !slices.Equal(got, bundles) {
		t.Errorf("%s: OriginBundles = %v, want %v", l.FullRepoPath(), got, bundles)
	}
}

func walk(t *testing.T, c *stack.Cluster, rules layout.LayoutRules) *layout.ManifestLayout {
	t.Helper()
	ml, err := layout.WalkCluster(c, rules)
	if err != nil {
		t.Fatalf("WalkCluster: %v", err)
	}
	return ml
}

// twoTier is node platform (bundle platformBundle) with child node web
// (bundle webBundle); each bundle has one application.
func twoTier(platformBundle, webBundle string) *stack.Cluster {
	web := &stack.Node{Name: "web", Bundle: &stack.Bundle{Name: webBundle, Applications: []*stack.Application{configMapApp("frontend")}}}
	root := &stack.Node{Name: "platform", Bundle: &stack.Bundle{Name: platformBundle, Applications: []*stack.Application{configMapApp("core")}}, Children: []*stack.Node{web}}
	return &stack.Cluster{Name: "demo", Node: root}
}

func TestWalkCluster_Origins_GroupByName(t *testing.T) {
	c := twoTier("platform", "web-bundle")
	ml := walk(t, c, groupByName)

	assertOrigin(t, layoutAt(t, ml, "platform"), []string{"platform"}, nil)
	bl := layoutAt(t, ml, "platform/platform")
	assertOrigin(t, bl, nil, []string{"platform"})
	// A bundle layout lists its own files: CRs hosted there (umbrella
	// children, PerLayout app CRs) must be applied.
	if bl.Mode != layout.KustomizationExplicit {
		t.Errorf("bundle layout Mode = %q, want %q", bl.Mode, layout.KustomizationExplicit)
	}
	app := layoutAt(t, ml, "platform/platform/core")
	if app.OriginApplication() != c.Node.Bundle.Applications[0] {
		t.Errorf("app layout OriginApplication = %v, want the core application", app.OriginApplication())
	}
	assertOrigin(t, app, nil, nil)
	assertOrigin(t, layoutAt(t, ml, "platform/web"), []string{"web"}, nil)
	assertOrigin(t, layoutAt(t, ml, "platform/web/web-bundle"), nil, []string{"web-bundle"})
}

func TestWalkCluster_Origins_NodeOnly(t *testing.T) {
	ml := walk(t, twoTier("platform", "web-bundle"), nodeOnly)
	assertOrigin(t, layoutAt(t, ml, "platform"), []string{"platform"}, []string{"platform"})
	assertOrigin(t, layoutAt(t, ml, "platform/web"), []string{"web"}, []string{"web-bundle"})
}

func umbrellaCluster() *stack.Cluster {
	db := &stack.Bundle{Name: "svc-db", Applications: []*stack.Application{configMapApp("db")}}
	svc := &stack.Bundle{Name: "svc", Applications: []*stack.Application{configMapApp("api")}, Children: []*stack.Bundle{db}}
	root := &stack.Bundle{Name: "platform", Applications: []*stack.Application{configMapApp("core")}, Children: []*stack.Bundle{svc}}
	return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: root}}
}

func TestWalkCluster_Origins_UmbrellaChildren(t *testing.T) {
	t.Run("nodeOnly", func(t *testing.T) {
		ml := walk(t, umbrellaCluster(), nodeOnly)
		assertOrigin(t, layoutAt(t, ml, "platform"), []string{"platform"}, []string{"platform"})
		assertOrigin(t, layoutAt(t, ml, "platform/svc"), nil, []string{"svc"})
		assertOrigin(t, layoutAt(t, ml, "platform/svc/svc-db"), nil, []string{"svc-db"})
	})
	t.Run("GroupByName", func(t *testing.T) {
		ml := walk(t, umbrellaCluster(), groupByName)
		assertOrigin(t, layoutAt(t, ml, "platform"), []string{"platform"}, nil)
		assertOrigin(t, layoutAt(t, ml, "platform/platform"), nil, []string{"platform"})
		assertOrigin(t, layoutAt(t, ml, "platform/platform/svc"), nil, []string{"svc"})
		assertOrigin(t, layoutAt(t, ml, "platform/platform/svc/svc-db"), nil, []string{"svc-db"})
	})
}

func TestWalkCluster_Origins_NodeFlatMergesBundles(t *testing.T) {
	api := &stack.Node{Name: "api", Bundle: &stack.Bundle{Name: "api", Applications: []*stack.Application{configMapApp("api")}}}
	web := &stack.Node{Name: "web", Bundle: &stack.Bundle{Name: "web", Applications: []*stack.Application{configMapApp("web")}}, Children: []*stack.Node{api}}
	root := &stack.Node{Name: "platform", Bundle: &stack.Bundle{Name: "platform", Applications: []*stack.Application{configMapApp("core")}}, Children: []*stack.Node{web}}
	ml := walk(t, &stack.Cluster{Name: "demo", Node: root}, nodeFlat)
	if len(ml.Children) != 0 {
		t.Fatalf("nodeFlat root has children %v, want all merged", collectRepoPaths(ml))
	}
	assertOrigin(t, ml, []string{"platform", "web", "api"}, []string{"platform", "web", "api"})
}

func TestWalkCluster_NodeFlatRefusesChildLayouts(t *testing.T) {
	for name, child := range map[string]*stack.Node{
		"umbrella child": {Name: "web", Bundle: &stack.Bundle{Name: "web", Children: []*stack.Bundle{{Name: "web-db"}}}},
		"augmenter app": {Name: "web", Bundle: &stack.Bundle{Name: "web", Applications: []*stack.Application{
			stack.NewApplication("chart", "default", &fakeAugmentingConfig{objs: nil}),
		}}},
	} {
		t.Run(name, func(t *testing.T) {
			root := &stack.Node{Name: "platform", Bundle: &stack.Bundle{Name: "platform"}, Children: []*stack.Node{child}}
			_, err := layout.WalkCluster(&stack.Cluster{Name: "demo", Node: root}, nodeFlat)
			if err == nil {
				t.Fatal("WalkCluster merged a node that has child layouts; want a refusal (the merge drops them)")
			}
			if !strings.Contains(err.Error(), `cannot merge node "web"`) {
				t.Errorf("error %q does not name the refused merge", err)
			}
		})
	}
}

func TestWalkClusterWithClusterName_Origins_UnnamedRoot(t *testing.T) {
	web := &stack.Node{Name: "web", Bundle: &stack.Bundle{Name: "web", Applications: []*stack.Application{configMapApp("web")}}}
	t.Run("with bundle", func(t *testing.T) {
		root := &stack.Node{Bundle: &stack.Bundle{Name: "root", Applications: []*stack.Application{configMapApp("core")}}, Children: []*stack.Node{web}}
		ml := walk(t, &stack.Cluster{Name: "demo", Node: root}, withClusterName(nodeOnly, "prod"))
		assertOrigin(t, layoutAt(t, ml, "prod"), []string{""}, []string{"root"})
		assertOrigin(t, layoutAt(t, ml, "prod/web"), []string{"web"}, []string{"web"})
	})
	t.Run("without bundle", func(t *testing.T) {
		root := &stack.Node{Children: []*stack.Node{web}}
		ml := walk(t, &stack.Cluster{Name: "demo", Node: root}, withClusterName(nodeOnly, "prod"))
		assertOrigin(t, layoutAt(t, ml, "prod"), []string{""}, nil)
	})
}

func TestWalkClusterWithClusterName_Origins_NamedRoot(t *testing.T) {
	for _, tc := range []struct {
		name     string
		rules    layout.LayoutRules
		rootPath string
		wrapper  string // "" when the wrapper is elided
	}{
		{"nodeOnly prod", withClusterName(nodeOnly, "prod"), "prod/platform", "prod"},
		{"GroupByName prod", withClusterName(groupByName, "prod"), "prod/platform", "prod"},
		{"ClusterName equals root", withClusterName(nodeOnly, "platform"), "platform", ""},
		{"ClusterName dot", withClusterName(nodeOnly, "."), "platform", "."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ml := walk(t, twoTier("platform", "web-bundle"), tc.rules)
			// The root bundle is always flattened into the root node layout.
			assertOrigin(t, layoutAt(t, ml, tc.rootPath), []string{"platform"}, []string{"platform"})
			if tc.wrapper != "" {
				assertOrigin(t, layoutAt(t, ml, tc.wrapper), nil, nil)
			}
		})
	}
}

func TestWalkClusterByPackage_Origins(t *testing.T) {
	oci := &schema.GroupVersionKind{Group: "source.toolkit.fluxcd.io", Version: "v1", Kind: "OCIRepository"}
	build := func() *stack.Cluster {
		web := &stack.Node{Name: "web", PackageRef: oci, Bundle: &stack.Bundle{Name: "web-bundle", Applications: []*stack.Application{configMapApp("frontend")}}}
		root := &stack.Node{Name: "platform", Bundle: &stack.Bundle{Name: "platform", Applications: []*stack.Application{configMapApp("core")}}, Children: []*stack.Node{web}}
		return &stack.Cluster{Name: "demo", Node: root}
	}
	t.Run("nodeOnly", func(t *testing.T) {
		pkgs, err := layout.WalkClusterByPackage(build(), nodeOnly)
		if err != nil {
			t.Fatal(err)
		}
		assertOrigin(t, layoutAt(t, pkgs["default"], "platform"), []string{"platform"}, []string{"platform"})
		ociRoot := pkgs[oci.String()]
		// The unnamed wrapper for the excluded root (at cluster/) carries nothing.
		assertOrigin(t, ociRoot, nil, nil)
		assertOrigin(t, layoutAt(t, ociRoot, "cluster/web"), []string{"web"}, []string{"web-bundle"})
	})
	t.Run("GroupByName", func(t *testing.T) {
		c := build()
		pkgs, err := layout.WalkClusterByPackage(c, groupByName)
		if err != nil {
			t.Fatal(err)
		}
		def := pkgs["default"]
		assertOrigin(t, layoutAt(t, def, "platform"), []string{"platform"}, nil)
		assertOrigin(t, layoutAt(t, def, "platform/platform"), nil, []string{"platform"})
		if got := layoutAt(t, def, "platform/platform/core").OriginApplication(); got != c.Node.Bundle.Applications[0] {
			t.Errorf("package app layout OriginApplication = %v, want the core application", got)
		}
		assertOrigin(t, layoutAt(t, pkgs[oci.String()], "cluster/web/web-bundle"), nil, []string{"web-bundle"})
	})
}

// explicitPathClusters are the four clusters the KustomizationPath table and
// the A-vs-B byte-identity probe share.
func explicitPathClusters() []struct {
	name  string
	c     *stack.Cluster
	rules layout.LayoutRules
	want  map[string]string // bundle name -> directory
} {
	platformWeb := &stack.Node{Name: "platform", Bundle: &stack.Bundle{Name: "web", Applications: []*stack.Application{configMapApp("frontend")}}}
	platformPlatform := &stack.Node{Name: "platform", Bundle: &stack.Bundle{Name: "platform", Applications: []*stack.Application{configMapApp("core")}}}
	apps := &stack.Node{Name: "apps", Bundle: &stack.Bundle{Name: "apps-bundle", Applications: []*stack.Application{configMapApp("frontend")}}}
	prod := &stack.Node{Name: "platform", Bundle: &stack.Bundle{Name: "platform", Applications: []*stack.Application{configMapApp("core")}}, Children: []*stack.Node{apps}}
	unnamed := &stack.Node{Bundle: &stack.Bundle{Name: "web", Applications: []*stack.Application{configMapApp("frontend")}}}
	return []struct {
		name  string
		c     *stack.Cluster
		rules layout.LayoutRules
		want  map[string]string
	}{
		{"platform/web", &stack.Cluster{Name: "demo", Node: platformWeb}, groupByName, map[string]string{"web": "platform/web"}},
		{"platform/platform", &stack.Cluster{Name: "demo", Node: platformPlatform}, groupByName, map[string]string{"platform": "platform/platform"}},
		{"ClusterName prod nodeOnly", &stack.Cluster{Name: "demo", Node: prod}, withClusterName(nodeOnly, "prod"), map[string]string{"platform": "prod/platform", "apps-bundle": "prod/platform/apps"}},
		{"unnamed root ClusterName dot", &stack.Cluster{Name: "demo", Node: unnamed}, withClusterName(nodeOnly, "."), map[string]string{"web": "."}},
	}
}

func TestIndexOrigins_KustomizationPath_Explicit(t *testing.T) {
	for _, tc := range explicitPathClusters() {
		t.Run(tc.name, func(t *testing.T) {
			ml := walk(t, tc.c, tc.rules)
			ix, err := layout.IndexOrigins(ml, tc.c)
			if err != nil {
				t.Fatalf("IndexOrigins: %v", err)
			}
			got := map[string]string{}
			for _, b := range ix.Bundles() {
				p, err := ix.KustomizationPath(b)
				if err != nil {
					t.Fatalf("KustomizationPath(%s): %v", b.Name, err)
				}
				got[b.Name] = p
				if p != ix.BundleLayout(b).FullRepoPath() {
					t.Errorf("KustomizationPath(%s) = %q, BundleLayout dir %q", b.Name, p, ix.BundleLayout(b).FullRepoPath())
				}
			}
			if len(got) != len(tc.want) {
				t.Errorf("paths = %v, want %v", got, tc.want)
			}
			for name, want := range tc.want {
				if got[name] != want {
					t.Errorf("KustomizationPath(%s) = %q, want %q", name, got[name], want)
				}
			}
		})
	}
}

func TestIndexOrigins_ParentAndNodeLayout(t *testing.T) {
	c := twoTier("platform", "web-bundle")
	ml := walk(t, c, groupByName)
	ix, err := layout.IndexOrigins(ml, c)
	if err != nil {
		t.Fatal(err)
	}
	web := c.Node.Children[0]
	if got := ix.NodeLayout(web).FullRepoPath(); got != "platform/web" {
		t.Errorf("NodeLayout(web) = %q, want platform/web", got)
	}
	bl := ix.BundleLayout(web.Bundle)
	if got := ix.Parent(bl); got != ix.NodeLayout(web) {
		t.Errorf("Parent(bundle layout) = %v, want the web node layout", got)
	}
	if ix.Parent(ml) != nil {
		t.Error("Parent(root) is not nil")
	}
	if got := bundleNames(ix.Bundles()); !slices.Equal(got, []string{"platform", "web-bundle"}) {
		t.Errorf("Bundles() = %v, want layout pre-order [platform web-bundle]", got)
	}
}

func assertIndexError(t *testing.T, ml *layout.ManifestLayout, c *stack.Cluster, want string) {
	t.Helper()
	_, err := layout.IndexOrigins(ml, c)
	if err == nil {
		t.Fatalf("IndexOrigins accepted the tree; want an error containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("IndexOrigins error %q does not contain %q", err, want)
	}
}

func TestIndexOrigins_RejectsDuplicateBundle(t *testing.T) {
	shared := &stack.Bundle{Name: "shared", Applications: []*stack.Application{configMapApp("x")}}
	root := &stack.Node{Name: "platform", Children: []*stack.Node{{Name: "a", Bundle: shared}, {Name: "b", Bundle: shared}}}
	c := &stack.Cluster{Name: "demo", Node: root}
	assertIndexError(t, walk(t, c, nodeOnly), c, `bundle "shared" is rendered by two layouts`)
}

func TestIndexOrigins_RejectsDuplicateName(t *testing.T) {
	root := &stack.Node{Name: "platform", Children: []*stack.Node{
		{Name: "a", Bundle: &stack.Bundle{Name: "web"}},
		{Name: "b", Bundle: &stack.Bundle{Name: "web"}},
	}}
	c := &stack.Cluster{Name: "demo", Node: root}
	assertIndexError(t, walk(t, c, nodeOnly), c, `two bundles are named "web"`)
}

func TestIndexOrigins_RejectsMissingBundle(t *testing.T) {
	c := twoTier("platform", "web-bundle")
	ml := walk(t, c, nodeOnly)
	ml.Children = nil // drop the web node's layout
	assertIndexError(t, ml, c, `layout does not render cluster "demo": missing`)
	assertIndexError(t, ml, c, `bundle "web-bundle"`)
}

func TestIndexOrigins_RejectsOtherCluster(t *testing.T) {
	a := twoTier("platform", "web-bundle")
	b := twoTier("platform", "web-bundle")
	assertIndexError(t, walk(t, a, nodeOnly), b, "not reachable from cluster")
}

func TestIndexOrigins_RejectsHandBuilt(t *testing.T) {
	c := twoTier("platform", "web-bundle")
	hand := &layout.ManifestLayout{Name: "platform", Namespace: ".", Children: []*layout.ManifestLayout{{Name: "web", Namespace: "platform"}}}
	assertIndexError(t, hand, c, "build it with layout.WalkCluster")
}

func TestIndexOrigins_RejectsAppFileSingleOrigin(t *testing.T) {
	c := twoTier("platform", "web-bundle")
	ml := walk(t, c, nodeOnly)
	layoutAt(t, ml, "platform/web").ApplicationFileMode = layout.AppFileSingle
	assertIndexError(t, ml, c, "AppFileSingle")
}

func TestFlattenSingleTier_TransfersOrigins(t *testing.T) {
	// The unnamed root carries its bundle (no applications, so no resources)
	// and has exactly one terminal child: the collapse lifts the child into
	// the cluster directory, and both bundles now live there.
	web := &stack.Node{Name: "web", Bundle: &stack.Bundle{Name: "web", Applications: []*stack.Application{configMapApp("web")}}}
	root := &stack.Node{Bundle: &stack.Bundle{Name: "root"}, Children: []*stack.Node{web}}
	c := &stack.Cluster{Name: "demo", Node: root}
	rules := withClusterName(nodeOnly, "prod")
	rules.FlattenSingleTier = true
	ml := walk(t, c, rules)
	if len(ml.Children) != 0 {
		t.Fatalf("expected the single tier collapsed, have %v", collectRepoPaths(ml))
	}
	assertOrigin(t, ml, []string{"", "web"}, []string{"root", "web"})
	ix, err := layout.IndexOrigins(ml, c)
	if err != nil {
		t.Fatalf("IndexOrigins: %v", err)
	}
	for _, b := range []*stack.Bundle{root.Bundle, web.Bundle} {
		if p, _ := ix.KustomizationPath(b); p != "prod" {
			t.Errorf("KustomizationPath(%s) = %q, want the surviving directory prod", b.Name, p)
		}
	}
}

func TestIndexOrigins_EdgeRefusals(t *testing.T) {
	c := twoTier("platform", "web-bundle")
	ml := walk(t, c, nodeOnly)
	if _, err := layout.IndexOrigins(nil, c); err == nil {
		t.Error("IndexOrigins(nil layout) returned no error")
	}
	if _, err := layout.IndexOrigins(ml, &stack.Cluster{Name: "empty"}); err == nil {
		t.Error("IndexOrigins with a node-less cluster returned no error")
	}
	ix, err := layout.IndexOrigins(ml, c)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range []*stack.Bundle{{Name: "stray"}, nil} {
		if _, err := ix.KustomizationPath(b); err == nil || !strings.Contains(err.Error(), "not rendered by the indexed layout") {
			t.Errorf("KustomizationPath(%v): got %v, want a not-rendered error", b, err)
		}
	}

	// A node reachable from two parents is walked twice.
	shared := &stack.Node{Name: "shared"}
	twice := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Children: []*stack.Node{
		{Name: "a", Children: []*stack.Node{shared}},
		{Name: "b", Children: []*stack.Node{shared}},
	}}}
	assertIndexError(t, walk(t, twice, nodeOnly), twice, `node "shared" is rendered by two layouts`)
}

// TestWriteManifest_RefusesAppFileSingleOrigin: a node or bundle layout written
// as AppFileSingle — its own mode, or Config's for an unset one — would put its
// files into its Namespace, not the directory its Flux path names.
func TestWriteManifest_RefusesAppFileSingleOrigin(t *testing.T) {
	c := twoTier("platform", "web-bundle")
	ml := walk(t, c, nodeOnly)
	if err := layout.WriteManifest(t.TempDir(), layout.Config{ApplicationFileMode: layout.AppFileSingle}, ml); err == nil || !strings.Contains(err.Error(), "AppFileSingle") {
		t.Errorf("Config AppFileSingle: got %v, want a refusal", err)
	}
	layoutAt(t, ml, "platform/web").ApplicationFileMode = layout.AppFileSingle
	if err := layout.WriteManifest(t.TempDir(), layout.DefaultLayoutConfig(), ml); err == nil || !strings.Contains(err.Error(), `"platform/web"`) {
		t.Errorf("own AppFileSingle: got %v, want a refusal naming platform/web", err)
	}
	// An application layout in AppFileSingle mode is that mode's purpose.
	app := walk(t, twoTier("platform", "web-bundle"), groupByName)
	layoutAt(t, app, "platform/web/web-bundle/frontend").ApplicationFileMode = layout.AppFileSingle
	if err := layout.WriteManifest(t.TempDir(), layout.DefaultLayoutConfig(), app); err != nil {
		t.Errorf("application layout in AppFileSingle: %v", err)
	}
}
