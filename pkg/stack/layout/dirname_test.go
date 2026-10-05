package layout_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for Bundle.DirName: the walker names a bundle's own directory by it,
// the origin index refuses such a directory under any other name, and the
// writers name the bundles when two of them resolve to one directory.

// dirNameCluster is r (no bundle) -> c: bundle b, DirName 10-b, with
// application a1 and umbrella child u, DirName 00-u, with application ua.
func dirNameCluster() *stack.Cluster {
	u := &stack.Bundle{Name: "u", DirName: "00-u", Applications: []*stack.Application{axisApp("ua")}}
	b := &stack.Bundle{Name: "b", DirName: "10-b", Applications: []*stack.Application{axisApp("a1")}, Children: []*stack.Bundle{u}}
	c := &stack.Node{Name: "c", Bundle: b}
	r := &stack.Node{Name: "r", Children: []*stack.Node{c}}
	c.SetParent(r)
	return &stack.Cluster{Name: "demo", Node: r}
}

// TestWalkCluster_DirName pins, for every grouping combination and in both
// walkers, where DirName applies. An umbrella child's directory takes it in
// every mode; a node's bundle takes it wherever the bundle's name becomes a
// directory: under BundleGrouping GroupByName, and with both node and bundle
// grouping flat, where the bundle is merged into the root node and rendered
// in the directory the root node's bundles get inside the root's. Under
// GroupFlat below the root it is rendered in its node's directory and DirName
// has no effect. No directory is named after the bundle's Name instead, and
// what lies below a bundle's directory follows it.
func TestWalkCluster_DirName(t *testing.T) {
	N, F := layout.GroupByName, layout.GroupFlat
	walkers := map[string]struct {
		prefix string // directory of node r
		walk   func(c *stack.Cluster, r layout.LayoutRules) (*layout.ManifestLayout, error)
	}{
		"WalkCluster": {"r", layout.WalkCluster},
		"WalkCluster_ClusterName": {"prod/r", func(c *stack.Cluster, r layout.LayoutRules) (*layout.ManifestLayout, error) {
			r.ClusterName = "prod"
			return layout.WalkCluster(c, r)
		}},
		"WalkClusterByPackage": {"r", func(c *stack.Cluster, r layout.LayoutRules) (*layout.ManifestLayout, error) {
			pkgs, err := layout.WalkClusterByPackage(c, r)
			if err != nil {
				return nil, err
			}
			return pkgs["default"], nil
		}},
	}
	for wname, w := range walkers {
		for _, node := range []layout.GroupingMode{N, F} {
			for _, bundle := range []layout.GroupingMode{N, F} {
				for _, app := range []layout.GroupingMode{N, F} {
					t.Run(wname+"/"+string(node)+"_"+string(bundle)+"_"+string(app), func(t *testing.T) {
						ml, err := w.walk(dirNameCluster(), layout.LayoutRules{NodeGrouping: node, BundleGrouping: bundle, ApplicationGrouping: app})
						if err != nil {
							t.Fatalf("walk: %v", err)
						}
						cDir := w.prefix
						if node == N {
							cDir = join(w.prefix, "c")
						}
						// Flat in a child node's directory, the bundle has no
						// directory; merged into the root node (node flat)
						// it has one, by either bundle grouping.
						bDir := cDir
						if bundle == N || node == F {
							bDir = join(cDir, "10-b")
						}
						uDir := join(bDir, "00-u")

						_, bundles := originPaths(ml)
						if got := bundles["b"]; got != bDir {
							t.Errorf("bundle b is rendered at %q, want %q", got, bDir)
						}
						if got := bundles["u"]; got != uDir {
							t.Errorf("umbrella child u is rendered at %q, want %q", got, uDir)
						}
						paths := layoutResources(t, ml)
						for p := range paths {
							for segment := range strings.SplitSeq(p, "/") {
								if segment == "b" || segment == "u" {
									t.Errorf("layout %q has a directory named after a bundle's Name, not its DirName", p)
								}
							}
						}
						if app == N {
							for _, want := range []string{join(bDir, "a1"), join(uDir, "ua")} {
								if _, ok := paths[want]; !ok {
									t.Errorf("no application layout at %q; have %v", want, collectRepoPaths(ml))
								}
							}
						}
					})
				}
			}
		}
	}
}

// TestIndexOrigins_BundleDirectoryName: a layout that is a bundle's own
// directory (an umbrella child, or a node's bundle under GroupByName) must be
// named by the bundle's directory name, DirName or else Name. Renaming such a
// layout after the walk is refused, and the error names the bundle and both
// names. A node layout that renders a differently named bundle is no bundle's
// own directory and is not affected.
func TestIndexOrigins_BundleDirectoryName(t *testing.T) {
	cluster := func(dirName string) *stack.Cluster {
		infra := &stack.Bundle{Name: "shop-infra", DirName: dirName, Applications: []*stack.Application{configMapApp("infra")}}
		shop := &stack.Bundle{Name: "shop", Applications: []*stack.Application{configMapApp("core")}, Children: []*stack.Bundle{infra}}
		apps := &stack.Node{Name: "apps", Bundle: shop}
		r := &stack.Node{Name: "r", Children: []*stack.Node{apps}}
		apps.SetParent(r)
		return &stack.Cluster{Name: "demo", Node: r}
	}

	t.Run("walked tree with DirName", func(t *testing.T) {
		for name, rules := range map[string]layout.LayoutRules{"nodeOnly": nodeOnly, "groupByName": groupByName} {
			c := cluster("00-infra")
			if _, err := layout.IndexOrigins(walk(t, c, rules), c); err != nil {
				t.Errorf("%s: IndexOrigins: %v", name, err)
			}
		}
	})

	tests := []struct {
		name    string
		dirName string
		rules   layout.LayoutRules
		at      string // layout to rename
		rename  string
		want    []string
	}{
		{
			name: "umbrella child renamed", rules: nodeOnly, at: "r/apps/shop-infra", rename: "00-infra",
			want: []string{`"shop/shop-infra"`, `"00-infra"`, `"shop-infra"`, "DirName"},
		},
		{
			name: "umbrella child with DirName renamed", dirName: "00-infra", rules: nodeOnly, at: "r/apps/00-infra", rename: "shop-infra",
			want: []string{`"shop/shop-infra"`, `"00-infra"`, `"shop-infra"`, "DirName"},
		},
		{
			name: "GroupByName bundle renamed", rules: groupByName, at: "r/apps/shop", rename: "10-shop",
			want: []string{`"shop"`, `"10-shop"`, "DirName"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := cluster(tt.dirName)
			ml := walk(t, c, tt.rules)
			layoutAt(t, ml, tt.at).Name = tt.rename
			_, err := layout.IndexOrigins(ml, c)
			if err == nil {
				t.Fatal("IndexOrigins accepted a bundle's own directory under another name")
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("IndexOrigins error %q does not contain %s", err, want)
				}
			}
		})
	}

	t.Run("node layout rendering a differently named bundle", func(t *testing.T) {
		c := twoTier("platform-bundle", "web-bundle")
		if _, err := layout.IndexOrigins(walk(t, c, nodeOnly), c); err != nil {
			t.Errorf("IndexOrigins: %v", err)
		}
		// DirName has no effect on a bundle rendered in its node's directory.
		c = twoTier("platform-bundle", "web-bundle")
		c.Node.Children[0].Bundle.DirName = "00-web"
		ml := walk(t, c, nodeOnly)
		if _, err := layout.IndexOrigins(ml, c); err != nil {
			t.Errorf("IndexOrigins with an unused DirName: %v", err)
		}
		assertOrigin(t, layoutAt(t, ml, "platform/web"), []string{"web"}, []string{"web-bundle"})
	})
}

// TestWalkCluster_DirNameAtTheRoot: with a flat BundleGrouping the root
// node's bundle has a directory inside the root node's (go-kure/kure#979),
// and that directory is named by the bundle's DirName like any other a
// bundle's name becomes. Both walkers, with and without a ClusterName. The
// origin index accepts the tree and refuses the directory under another name.
func TestWalkCluster_DirNameAtTheRoot(t *testing.T) {
	cluster := func() *stack.Cluster {
		c := twoTier("platform-bundle", "web-bundle")
		c.Node.Bundle.DirName = "00-platform"
		return c
	}
	ml := walk(t, cluster(), nodeOnly)
	if got, want := collectRepoPaths(ml), []string{"platform", "platform/00-platform", "platform/web"}; !slices.Equal(got, want) {
		t.Errorf("WalkCluster: layouts = %v, want %v", got, want)
	}
	assertOrigin(t, layoutAt(t, ml, "platform/00-platform"), nil, []string{"platform-bundle"})

	ml = walk(t, cluster(), withClusterName(nodeOnly, "prod"))
	if got, want := collectRepoPaths(ml), []string{"prod", "prod/platform", "prod/platform/00-platform", "prod/platform/web"}; !slices.Equal(got, want) {
		t.Errorf("WalkCluster with a ClusterName: layouts = %v, want %v", got, want)
	}

	pkgs, err := layout.WalkClusterByPackage(cluster(), nodeOnly)
	if err != nil {
		t.Fatalf("WalkClusterByPackage: %v", err)
	}
	if got, want := collectRepoPaths(pkgs["default"]), []string{"platform", "platform/00-platform", "platform/web"}; !slices.Equal(got, want) {
		t.Errorf("WalkClusterByPackage: layouts = %v, want %v", got, want)
	}

	// The directory is named on the bundle here too: the index accepts the
	// walked tree and refuses the layout under another name.
	c := cluster()
	ml = walk(t, c, nodeOnly)
	if _, err := layout.IndexOrigins(ml, c); err != nil {
		t.Errorf("IndexOrigins: %v", err)
	}
	layoutAt(t, ml, "platform/00-platform").Name = "platform-bundle"
	_, err = layout.IndexOrigins(ml, c)
	if err == nil {
		t.Fatal("IndexOrigins accepted the root node's bundle directory under another name")
	}
	for _, want := range []string{`bundle "platform-bundle"`, `"00-platform"`, "DirName"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("IndexOrigins error %q does not contain %s", err, want)
		}
	}
}

// TestWalkCluster_DirNameOfMergedBundles: with node and bundle grouping flat,
// every bundle is merged into the root node and they share one directory
// inside the root's. It takes the first merged bundle's DirName, or its Name
// without one. A DirName on a later bundle of that directory has no effect,
// as that bundle's Name has none.
func TestWalkCluster_DirNameOfMergedBundles(t *testing.T) {
	tests := []struct {
		name          string
		first, second string // DirName of the root node's bundle and of the child node's
		want          string // the shared directory
	}{
		{name: "first sets it", first: "00-platform", want: "platform/00-platform"},
		{name: "both set it", first: "00-platform", second: "10-web", want: "platform/00-platform"},
		{name: "only a later one sets it", second: "10-web", want: "platform/platform-bundle"},
		{name: "none sets it", want: "platform/platform-bundle"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			build := func() *stack.Cluster {
				c := twoTier("platform-bundle", "web-bundle")
				c.Node.Bundle.DirName = tt.first
				c.Node.Children[0].Bundle.DirName = tt.second
				return c
			}
			c := build()
			ml := walk(t, c, nodeFlat)
			if got, want := collectRepoPaths(ml), []string{"platform", tt.want}; !slices.Equal(got, want) {
				t.Errorf("WalkCluster: layouts = %v, want %v", got, want)
			}
			assertOrigin(t, layoutAt(t, ml, tt.want), nil, []string{"platform-bundle", "web-bundle"})
			if _, err := layout.IndexOrigins(ml, c); err != nil {
				t.Errorf("IndexOrigins: %v", err)
			}
			pkgs, err := layout.WalkClusterByPackage(build(), nodeFlat)
			if err != nil {
				t.Fatalf("WalkClusterByPackage: %v", err)
			}
			if got, want := collectRepoPaths(pkgs["default"]), []string{"platform", tt.want}; !slices.Equal(got, want) {
				t.Errorf("WalkClusterByPackage: layouts = %v, want %v", got, want)
			}
		})
	}

	// The root node has no bundle: the first merged bundle is a child node's.
	c1 := &stack.Node{Name: "c1", Bundle: &stack.Bundle{Name: "b1", DirName: "00-first", Applications: []*stack.Application{axisApp("one")}}}
	c2 := &stack.Node{Name: "c2", Bundle: &stack.Bundle{Name: "b2", DirName: "10-second", Applications: []*stack.Application{axisApp("two")}}}
	r := &stack.Node{Name: "r", Children: []*stack.Node{c1, c2}}
	c1.SetParent(r)
	c2.SetParent(r)
	ml := walk(t, &stack.Cluster{Name: "demo", Node: r}, nodeFlat)
	if got, want := collectRepoPaths(ml), []string{"r", "r/00-first"}; !slices.Equal(got, want) {
		t.Errorf("root node without a bundle: layouts = %v, want %v", got, want)
	}
}

// TestWalk_RootDirNameDirectoryChecks: the checks that compare the directory
// of the root node's bundle with what else is rendered compare the directory
// it gets, so they follow a DirName: a child node of that name is refused,
// naming the bundle by its Name; so is a layout further down the tree in that
// directory; and under BundleGrouping by name the writers refuse the pair.
// A bundle whose Name is a child node's is no longer refused once its DirName
// names another directory.
func TestWalk_RootDirNameDirectoryChecks(t *testing.T) {
	rootDir := func(name, dirName string) *stack.Cluster {
		c := twoTier(name, "web-bundle")
		c.Node.Bundle.DirName = dirName
		return c
	}
	// DirName is no object name, so it can differ from a node's in case only.
	for dirName, want := range map[string]string{
		"web": `node "web" and bundle "platform-bundle" are both rendered to directory "platform/web"`,
		"WEB": `node "web" and bundle "platform-bundle" are both rendered to directory "platform/WEB"`,
	} {
		for name, rules := range map[string]layout.LayoutRules{"no ClusterName": nodeOnly, "ClusterName prod": withClusterName(nodeOnly, "prod")} {
			want := want
			if rules.ClusterName != "" {
				want = strings.Replace(want, `"platform/`, `"prod/platform/`, 1)
			}
			if _, err := layout.WalkCluster(rootDir("platform-bundle", dirName), rules); err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("%s, DirName %q, WalkCluster: got %v, want the refusal %q", name, dirName, err, want)
			}
		}
		if _, err := layout.WalkClusterByPackage(rootDir("platform-bundle", dirName), nodeOnly); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("DirName %q, WalkClusterByPackage: got %v, want the refusal %q", dirName, err, want)
		}
	}

	// The way out of the refusal of a bundle named like a child node.
	ml := walk(t, rootDir("web", "00-web"), nodeOnly)
	if got, want := collectRepoPaths(ml), []string{"platform", "platform/00-web", "platform/web"}; !slices.Equal(got, want) {
		t.Errorf("bundle web with DirName 00-web: layouts = %v, want %v", got, want)
	}

	// A layout further down the tree in the bundle's directory.
	stray := rootDir("platform-bundle", "core")
	stray.Node.Children[0].Bundle.Applications = []*stack.Application{
		stack.NewApplication("frontend", "default", &strayLayoutConfig{"core", "platform"}),
	}
	const wantBelow = `bundle "platform-bundle" would be rendered to directory "platform/core", which layout "core" further down the tree already takes`
	if _, err := layout.WalkCluster(stray, nodeOnly); err == nil || !strings.Contains(err.Error(), wantBelow) || !strings.Contains(err.Error(), "field 'dirName'") {
		t.Errorf("WalkCluster: got %v, want the refusal %q on field 'dirName'", err, wantBelow)
	}

	// By name the walk refuses nothing and the writers refuse the directory
	// the bundle and the node share, naming the bundle.
	ml, err := layout.WalkCluster(rootDir("platform-bundle", "web"), groupByName)
	if err != nil {
		t.Fatalf("BundleGrouping by name: %v", err)
	}
	err = ml.WriteToDisk(t.TempDir())
	if err == nil {
		t.Fatal("WriteToDisk accepted a bundle directory that is a child node's")
	}
	for _, want := range []string{"resolve to the same directory", `bundle "platform-bundle"`, `layout "platform/web"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("WriteToDisk: error %q does not contain %s", err, want)
		}
	}
}

// TestWalk_RefusesDirNameThatIsNoPathSegment: a DirName that is not one path
// segment could name the directory above the bundle's, or one further down.
// stack.ValidateCluster refuses it for both walkers before any directory is
// made from it, on a node's bundle and on an umbrella child, whether or not
// the rules give that bundle a directory.
func TestWalk_RefusesDirNameThatIsNoPathSegment(t *testing.T) {
	for name, rules := range map[string]layout.LayoutRules{"nodeOnly": nodeOnly, "nodeFlat": nodeFlat, "groupByName": groupByName} {
		for dirName, want := range map[string]string{
			".":       `"." is not a directory name of its own`,
			"..":      `".." is not a directory name of its own`,
			"../web":  `"../web" contains a path separator`,
			"web/api": `"web/api" contains a path separator`,
			`..\web`:  `"..\\web" contains a path separator`,
		} {
			clusters := map[string]func() *stack.Cluster{
				"root node's bundle": func() *stack.Cluster {
					c := twoTier("platform-bundle", "web-bundle")
					c.Node.Bundle.DirName = dirName
					return c
				},
				"child node's bundle": func() *stack.Cluster {
					c := twoTier("platform-bundle", "web-bundle")
					c.Node.Children[0].Bundle.DirName = dirName
					return c
				},
				"umbrella child": func() *stack.Cluster {
					c := twoTier("platform-bundle", "web-bundle")
					c.Node.Children[0].Bundle.Children = []*stack.Bundle{{Name: "web-db", DirName: dirName}}
					return c
				},
			}
			for where, build := range clusters {
				if _, err := layout.WalkCluster(build(), rules); err == nil || !strings.Contains(err.Error(), want) {
					t.Errorf("%s, %s, WalkCluster: got %v, want the refusal %q", name, where, err, want)
				}
				if _, err := layout.WalkClusterByPackage(build(), rules); err == nil || !strings.Contains(err.Error(), want) {
					t.Errorf("%s, %s, WalkClusterByPackage: got %v, want the refusal %q", name, where, err, want)
				}
			}
		}
	}
}

// TestSameBundleDirectoryRefused: two bundles whose own directories resolve to
// one are refused by the origin index, which the Flux and ArgoCD generators
// build first, and by every writer before anything is written; the error
// names both bundles by their paths, not the one directory twice. A bundle's
// directory that collides with a layout that is no bundle's is the writers'
// to refuse, as before, and the error names the bundle.
func TestSameBundleDirectoryRefused(t *testing.T) {
	app := func(name string) []*stack.Application { return []*stack.Application{axisApp(name)} }
	umbrella := func(children ...*stack.Bundle) *stack.Cluster {
		shop := &stack.Bundle{Name: "shop", Applications: app("core"), Children: children}
		apps := &stack.Node{Name: "apps", Bundle: shop}
		r := &stack.Node{Name: "r", Children: []*stack.Node{apps}}
		apps.SetParent(r)
		return &stack.Cluster{Name: "demo", Node: r}
	}
	tests := []struct {
		name    string
		cluster func() *stack.Cluster
		rules   layout.LayoutRules
		want    []string
		// writersOnly: no two bundles collide, so the origin index accepts.
		writersOnly bool
	}{
		{
			name: "two umbrella children with one DirName",
			cluster: func() *stack.Cluster {
				return umbrella(
					&stack.Bundle{Name: "shop-infra", DirName: "00-base", Applications: app("infra")},
					&stack.Bundle{Name: "shop-services", DirName: "00-base", Applications: app("services")})
			},
			rules: nodeOnly,
			want:  []string{`bundles "shop/shop-infra" and "shop/shop-services"`, `"r/apps/00-base"`},
		},
		{
			// A DirName may hold upper case, and the writers compare
			// directories without regard to it.
			name: "two umbrella children whose DirNames differ in case only",
			cluster: func() *stack.Cluster {
				return umbrella(
					&stack.Bundle{Name: "shop-infra", DirName: "00-Base", Applications: app("infra")},
					&stack.Bundle{Name: "shop-services", DirName: "00-base", Applications: app("services")})
			},
			rules: nodeOnly,
			want:  []string{`bundles "shop/shop-infra" and "shop/shop-services"`, `"r/apps/00-base"`},
		},
		{
			name: "a DirName equal to a sibling's Name",
			cluster: func() *stack.Cluster {
				return umbrella(
					&stack.Bundle{Name: "shop-infra", Applications: app("infra")},
					&stack.Bundle{Name: "shop-services", DirName: "shop-infra", Applications: app("services")})
			},
			rules: nodeOnly,
			want:  []string{`bundles "shop/shop-infra" and "shop/shop-services"`, `"r/apps/shop-infra"`},
		},
		{
			name: "two node bundles merged into one node directory",
			cluster: func() *stack.Cluster {
				c1 := &stack.Node{Name: "c1", Bundle: &stack.Bundle{Name: "b1", DirName: "shared", Applications: app("one")}}
				c2 := &stack.Node{Name: "c2", Bundle: &stack.Bundle{Name: "b2", DirName: "shared", Applications: app("two")}}
				r := &stack.Node{Name: "r", Children: []*stack.Node{c1, c2}}
				c1.SetParent(r)
				c2.SetParent(r)
				return &stack.Cluster{Name: "demo", Node: r}
			},
			rules: layout.LayoutRules{NodeGrouping: layout.GroupFlat, BundleGrouping: layout.GroupByName, ApplicationGrouping: layout.GroupFlat},
			want:  []string{`bundles "b1" and "b2"`, `"r/shared"`},
		},
		{
			name: "an umbrella child's DirName equal to an application directory",
			cluster: func() *stack.Cluster {
				return umbrella(&stack.Bundle{Name: "shop-infra", DirName: "core", Applications: app("infra")})
			},
			rules:       layout.LayoutRules{BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupByName},
			want:        []string{`bundle "shop/shop-infra"`, `layout "r/apps/core"`},
			writersOnly: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.cluster()
			_, err := layout.IndexOrigins(walk(t, c, tt.rules), c)
			switch {
			case tt.writersOnly && err != nil:
				t.Errorf("IndexOrigins: %v", err)
			case !tt.writersOnly && err == nil:
				t.Error("IndexOrigins accepted two bundles in one directory")
			case !tt.writersOnly:
				for _, want := range append([]string{"resolve to the same directory"}, tt.want...) {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("IndexOrigins: error %q does not contain %s", err, want)
					}
				}
			}

			build := func() *layout.ManifestLayout { return walk(t, tt.cluster(), tt.rules) }
			dir := t.TempDir()
			errs := map[string]error{
				"WriteToDisk":   build().WriteToDisk(filepath.Join(dir, "disk")),
				"WriteToTar":    build().WriteToTar(&strings.Builder{}),
				"WriteManifest": layout.WriteManifest(filepath.Join(dir, "manifest"), layout.DefaultLayoutConfig(), build()),
			}
			for writer, err := range errs {
				if err == nil {
					t.Errorf("%s accepted two layouts in one directory", writer)
					continue
				}
				for _, want := range append([]string{"resolve to the same directory"}, tt.want...) {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("%s: error %q does not contain %s", writer, err, want)
					}
				}
			}
		})
	}
}
