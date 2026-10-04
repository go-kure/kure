package layout_test

import (
	"os"
	"path/filepath"
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
	// The root node's layout renders no bundle (go-kure/kure#979): its bundle
	// has a directory under a flat bundle axis too. A child node's has not.
	assertOrigin(t, layoutAt(t, ml, "platform"), []string{"platform"}, nil)
	unit := layoutAt(t, ml, "platform/platform")
	assertOrigin(t, unit, nil, []string{"platform"})
	if unit.Mode != layout.KustomizationExplicit {
		t.Errorf("root bundle layout Mode = %q, want %q", unit.Mode, layout.KustomizationExplicit)
	}
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
		assertOrigin(t, layoutAt(t, ml, "platform"), []string{"platform"}, nil)
		assertOrigin(t, layoutAt(t, ml, "platform/platform"), nil, []string{"platform"})
		assertOrigin(t, layoutAt(t, ml, "platform/platform/svc"), nil, []string{"svc"})
		assertOrigin(t, layoutAt(t, ml, "platform/platform/svc/svc-db"), nil, []string{"svc-db"})
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
	// Every node merges into the root node's layout; their bundles stay one
	// unit, in one directory named after the first (go-kure/kure#979).
	if got, want := collectRepoPaths(ml), []string{"platform", "platform/platform"}; !slices.Equal(got, want) {
		t.Fatalf("nodeFlat layouts = %v, want %v", got, want)
	}
	assertOrigin(t, ml, []string{"platform", "web", "api"}, nil)
	assertOrigin(t, ml.Children[0], nil, []string{"platform", "web", "api"})
}

func TestWalkCluster_NodeFlatReparentsChildLayouts(t *testing.T) {
	for name, child := range map[string]*stack.Node{
		"umbrella child": {Name: "web", Bundle: &stack.Bundle{Name: "web", Children: []*stack.Bundle{{Name: "web-db"}}}},
		"augmenter app": {Name: "web", Bundle: &stack.Bundle{Name: "web", Applications: []*stack.Application{
			stack.NewApplication("chart", "default", &fakeAugmentingConfig{objs: nil}),
		}}},
	} {
		t.Run(name, func(t *testing.T) {
			root := &stack.Node{Name: "platform", Bundle: &stack.Bundle{Name: "platform"}, Children: []*stack.Node{child}}
			ml := walk(t, &stack.Cluster{Name: "demo", Node: root}, nodeFlat)
			// The merged node's child layout moves under the absorbing unit
			// instead of being dropped: web's node origin lands on the root,
			// its bundle origin on the unit's directory.
			assertOrigin(t, ml, []string{"platform", "web"}, nil)
			if len(ml.Children) != 1 {
				t.Fatalf("root has %d child layouts, want the unit's directory only", len(ml.Children))
			}
			unit := ml.Children[0]
			assertOrigin(t, unit, nil, []string{"platform", "web"})
			if len(unit.Children) != 1 {
				t.Fatalf("unit has %d child layouts, want web's one child layout re-parented", len(unit.Children))
			}
			if got, want := unit.Children[0].Namespace, unit.FullRepoPath(); got != want {
				t.Errorf("re-parented layout Namespace = %q, want the absorbing unit's directory %q", got, want)
			}
		})
	}
}

func TestWalkClusterWithClusterName_Origins_UnnamedRoot(t *testing.T) {
	web := &stack.Node{Name: "web", Bundle: &stack.Bundle{Name: "web", Applications: []*stack.Application{configMapApp("web")}}}
	t.Run("with bundle", func(t *testing.T) {
		root := &stack.Node{Bundle: &stack.Bundle{Name: "root", Applications: []*stack.Application{configMapApp("core")}}, Children: []*stack.Node{web}}
		ml := walk(t, &stack.Cluster{Name: "demo", Node: root}, withClusterName(nodeOnly, "prod"))
		assertOrigin(t, layoutAt(t, ml, "prod"), []string{""}, nil)
		assertOrigin(t, layoutAt(t, ml, "prod/root"), nil, []string{"root"})
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
			// The root bundle is rendered by its own layout under either
			// bundle grouping: the root node's layout renders no bundle
			// (go-kure/kure#979).
			assertOrigin(t, layoutAt(t, ml, tc.rootPath), []string{"platform"}, nil)
			assertOrigin(t, layoutAt(t, ml, tc.rootPath+"/platform"), nil, []string{"platform"})
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
		assertOrigin(t, layoutAt(t, pkgs["default"], "platform"), []string{"platform"}, nil)
		assertOrigin(t, layoutAt(t, pkgs["default"], "platform/platform"), nil, []string{"platform"})
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
		{"ClusterName prod nodeOnly", &stack.Cluster{Name: "demo", Node: prod}, withClusterName(nodeOnly, "prod"), map[string]string{"platform": "prod/platform/platform", "apps-bundle": "prod/platform/apps"}},
		{"unnamed root ClusterName dot", &stack.Cluster{Name: "demo", Node: unnamed}, withClusterName(nodeOnly, "."), map[string]string{"web": "web"}},
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

// TestWalk_RefusesChildNodeNamedLikeTheRootBundlesDirectory: with a flat
// BundleGrouping the root node's bundle has a directory named after it inside
// the root node's (go-kure/kure#979), which is where a child node of that name
// is written too. Both walkers refuse the pair, with and without a
// ClusterName; under BundleGrouping by name the walk is unchanged.
func TestWalk_RefusesChildNodeNamedLikeTheRootBundlesDirectory(t *testing.T) {
	const want = `node "web" and bundle "web" are both rendered to directory`
	for name, rules := range map[string]layout.LayoutRules{
		"no ClusterName":   nodeOnly,
		"ClusterName prod": withClusterName(nodeOnly, "prod"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := layout.WalkCluster(twoTier("web", "web-bundle"), rules); err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("WalkCluster: got %v, want the refusal %q", err, want)
			}
			if _, err := layout.WalkClusterByPackage(twoTier("web", "web-bundle"), rules); err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("WalkClusterByPackage: got %v, want the refusal %q", err, want)
			}
		})
	}

	// The names differ: nothing is refused, and the two are siblings.
	ml := walk(t, twoTier("platform", "web-bundle"), nodeOnly)
	if got, want := collectRepoPaths(ml), []string{"platform", "platform/platform", "platform/web"}; !slices.Equal(got, want) {
		t.Errorf("layouts = %v, want %v", got, want)
	}
	// With every node merged into the root there is no child node directory.
	ml = walk(t, twoTier("web", "web-bundle"), nodeFlat)
	if got, want := collectRepoPaths(ml), []string{"platform", "platform/web"}; !slices.Equal(got, want) {
		t.Errorf("NodeGrouping flat: layouts = %v, want %v", got, want)
	}
	// By name the walk never refused the pair and still does not.
	if _, err := layout.WalkCluster(twoTier("web", "web-bundle"), groupByName); err != nil {
		t.Errorf("BundleGrouping by name: %v", err)
	}
}

// TestFlattenSingleTier_KeepsBundleDirectories: FlattenSingleTier never
// collapses a directory that renders a bundle into the top of the tree, which
// has no parent to host its Flux Kustomization (go-kure/kure#979). What it
// still collapses is a single child node that has neither bundle nor children,
// and that node's origin moves with it.
func TestFlattenSingleTier_KeepsBundleDirectories(t *testing.T) {
	flatten := func(r layout.LayoutRules) layout.LayoutRules {
		r.FlattenSingleTier = true
		return r
	}
	configMapBundle := func(name string) *stack.Bundle {
		return &stack.Bundle{Name: name, Applications: []*stack.Application{configMapApp(name)}}
	}
	t.Run("unnamed root with bundle and one child node", func(t *testing.T) {
		web := &stack.Node{Name: "web", Bundle: configMapBundle("web")}
		root := &stack.Node{Bundle: &stack.Bundle{Name: "root"}, Children: []*stack.Node{web}}
		c := &stack.Cluster{Name: "demo", Node: root}
		ml := walk(t, c, flatten(withClusterName(nodeOnly, "prod")))
		if got, want := collectRepoPaths(ml), []string{"prod", "prod/root", "prod/web"}; !slices.Equal(got, want) {
			t.Fatalf("layouts = %v, want %v", got, want)
		}
		ix, err := layout.IndexOrigins(ml, c)
		if err != nil {
			t.Fatalf("IndexOrigins: %v", err)
		}
		for b, want := range map[*stack.Bundle]string{root.Bundle: "prod/root", web.Bundle: "prod/web"} {
			if p, _ := ix.KustomizationPath(b); p != want {
				t.Errorf("KustomizationPath(%s) = %q, want %q", b.Name, p, want)
			}
		}
	})
	t.Run("single child node rendering its bundle", func(t *testing.T) {
		web := &stack.Node{Name: "web", Bundle: configMapBundle("web")}
		root := &stack.Node{Name: "platform", Children: []*stack.Node{web}}
		ml := walk(t, &stack.Cluster{Name: "demo", Node: root}, flatten(nodeOnly))
		if got, want := collectRepoPaths(ml), []string{"platform", "platform/web"}; !slices.Equal(got, want) {
			t.Fatalf("layouts = %v, want %v", got, want)
		}
		assertOrigin(t, layoutAt(t, ml, "platform/web"), []string{"web"}, []string{"web"})
	})
	t.Run("wrapper above a root node with a bundle", func(t *testing.T) {
		root := &stack.Node{Name: "apps", Bundle: configMapBundle("web")}
		ml := walk(t, &stack.Cluster{Name: "demo", Node: root}, flatten(withClusterName(nodeFlat, "prod")))
		if got, want := collectRepoPaths(ml), []string{"prod", "prod/apps", "prod/apps/web"}; !slices.Equal(got, want) {
			t.Fatalf("layouts = %v, want %v", got, want)
		}
	})
	t.Run("single child node without bundle or children collapses", func(t *testing.T) {
		root := &stack.Node{Name: "platform", Children: []*stack.Node{{Name: "empty"}}}
		ml := walk(t, &stack.Cluster{Name: "demo", Node: root}, flatten(nodeOnly))
		if got, want := collectRepoPaths(ml), []string{"platform"}; !slices.Equal(got, want) {
			t.Fatalf("layouts = %v, want %v", got, want)
		}
		assertOrigin(t, ml, []string{"platform", "empty"}, nil)
	})
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

// TestWriteManifest_RefusesAppFileSingleOrigin: a node or bundle layout set to
// AppFileSingle would put its files into its Namespace, not the directory its
// Flux path names, so it is refused. Config's ApplicationFileMode is the
// default for application layouts only: under AppFileSingle there (the Argo
// profile's setting) node and bundle layouts keep their own directories and
// only application layouts become single files.
func TestWriteManifest_RefusesAppFileSingleOrigin(t *testing.T) {
	c := twoTier("platform", "web-bundle")
	ml := walk(t, c, nodeOnly)
	out := t.TempDir()
	if err := layout.WriteManifest(out, layout.DefaultConfigForProfile(layout.ArgoProfile), ml); err != nil {
		t.Fatalf("Argo profile (Config AppFileSingle): %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "applications", "platform", "web", "kustomization.yaml")); err != nil {
		t.Errorf("Argo profile: node layout platform/web not written as a directory: %v", err)
	}
	apps := walk(t, twoTier("platform", "web-bundle"), groupByName)
	out = t.TempDir()
	if err := layout.WriteManifest(out, layout.DefaultConfigForProfile(layout.ArgoProfile), apps); err != nil {
		t.Fatalf("Argo profile, per-app layouts: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "applications", "platform", "web", "web-bundle", "frontend.yaml")); err != nil {
		t.Errorf("Argo profile: application layout frontend not written as a single file: %v", err)
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

// TestIndexOrigins_Units pins the reconciliation units of a flat tree: one per
// directory that renders bundles, named after its first bundle, with
// dependencies mapped by bundle name (a copy of a bundle resolves like the
// original) and dependencies inside a unit dropped.
func TestIndexOrigins_Units(t *testing.T) {
	flat := layout.LayoutRules{NodeGrouping: layout.GroupFlat, BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupFlat}
	u := &stack.Bundle{Name: "u", Applications: []*stack.Application{configMapApp("ua")}}
	b2 := &stack.Bundle{Name: "b2", Applications: []*stack.Application{configMapApp("two")}}
	copyOfB2 := *b2
	b1 := &stack.Bundle{Name: "b1", Applications: []*stack.Application{configMapApp("one")},
		DependsOn: []*stack.Bundle{&copyOfB2, u}, NamedDependsOn: []string{"rb", "external"}}
	rb := &stack.Bundle{Name: "rb", Applications: []*stack.Application{configMapApp("core")}, Children: []*stack.Bundle{u}}
	c1 := &stack.Node{Name: "c1", Bundle: b1}
	c2 := &stack.Node{Name: "c2", Bundle: b2}
	r := &stack.Node{Name: "r", Bundle: rb, Children: []*stack.Node{c1, c2}}
	c1.SetParent(r)
	c2.SetParent(r)
	c := &stack.Cluster{Name: "demo", Node: r}
	ml := walk(t, c, flat)
	ix, err := layout.IndexOrigins(ml, c)
	if err != nil {
		t.Fatalf("IndexOrigins: %v", err)
	}
	var units []string
	for _, l := range ix.Units() {
		units = append(units, l.FullRepoPath())
	}
	if want := []string{"r/rb", "r/rb/u"}; !slices.Equal(units, want) {
		t.Errorf("Units = %v, want %v", units, want)
	}
	for b, want := range map[*stack.Bundle]string{rb: "rb", b1: "rb", b2: "rb", &copyOfB2: "rb", u: "u", {Name: "outside"}: "outside"} {
		if got := ix.UnitName(b); got != want {
			t.Errorf("UnitName(%s) = %q, want %q", b.Name, got, want)
		}
	}
	root := ix.Units()[0]
	if got := ix.UnitDependencies(root); !slices.Equal(got, []string{"u"}) {
		t.Errorf("UnitDependencies = %v, want [u] (b2 and its copy are inside rb's unit)", got)
	}
	if got := ix.UnitNamedDependencies(root); !slices.Equal(got, []string{"external"}) {
		t.Errorf("UnitNamedDependencies = %v, want [external] (rb is rb's own unit)", got)
	}

	// b1 -> u -> b2 is acyclic until b1 and b2 merge into rb's unit:
	// rb -> u -> rb.
	u.DependsOn = []*stack.Bundle{b2}
	b1.DependsOn = []*stack.Bundle{u}
	if _, err := layout.IndexOrigins(walk(t, c, flat), c); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Errorf("got %v, want a unit cycle refusal", err)
	}
}
