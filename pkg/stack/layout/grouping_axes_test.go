package layout_test

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// axisObj returns a ConfigMap named name in namespace ns.
func axisObj(name, ns string) *client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName(name)
	obj.SetNamespace(ns)
	var o client.Object = obj
	return &o
}

// axisApp is an application emitting one ConfigMap named after the app.
func axisApp(name string) *stack.Application {
	return stack.NewApplication(name, "default", &fakeConfig{objs: []*client.Object{axisObj(name, "default")}})
}

// axisAugmenterApp is an augmenter application (wants its own layout) that
// emits one ConfigMap named after the app and adds values.yaml.
func axisAugmenterApp(name string) *stack.Application {
	return stack.NewApplication(name, "default", &fakeAugmentingConfig{objs: []*client.Object{axisObj(name, "default")}})
}

// axesCluster is the tree the grouping-axes tests walk:
//
//	r (no bundle)
//	└── c: bundle b [a1, a2, x (augmenter)], umbrella child u [ua]
func axesCluster() *stack.Cluster {
	u := &stack.Bundle{Name: "u", Applications: []*stack.Application{axisApp("ua")}}
	b := &stack.Bundle{
		Name:         "b",
		Applications: []*stack.Application{axisApp("a1"), axisApp("a2"), axisAugmenterApp("x")},
		Children:     []*stack.Bundle{u},
	}
	c := &stack.Node{Name: "c", Bundle: b}
	r := &stack.Node{Name: "r", Children: []*stack.Node{c}}
	c.SetParent(r)
	return &stack.Cluster{Name: "demo", Node: r}
}

// layoutResources maps every layout's FullRepoPath to the sorted names of the
// resources it holds. A path listed twice is reported as an error.
func layoutResources(t *testing.T, ml *layout.ManifestLayout) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		p := l.FullRepoPath()
		if _, dup := out[p]; dup {
			t.Errorf("two layouts resolve to %q", p)
		}
		names := []string{}
		for _, o := range l.Resources {
			names = append(names, o.GetName())
		}
		sort.Strings(names)
		out[p] = names
		for _, c := range l.Children {
			walk(c)
		}
	}
	walk(ml)
	return out
}

// originPaths maps each origin node and bundle name to the path of the layout
// that carries it.
func originPaths(ml *layout.ManifestLayout) (nodes, bundles map[string]string) {
	nodes, bundles = map[string]string{}, map[string]string{}
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		for _, n := range l.OriginNodes() {
			nodes[n.Name] = l.FullRepoPath()
		}
		for _, b := range l.OriginBundles() {
			bundles[b.Name] = l.FullRepoPath()
		}
		for _, c := range l.Children {
			walk(c)
		}
	}
	walk(ml)
	return nodes, bundles
}

// axisCase is one (Node, Bundle, App) grouping combination with its expected
// tree, relative to the root node's directory "r".
type axisCase struct {
	node, bundle, app layout.GroupingMode
	// layouts maps a path below "r" ("" is r itself) to its resources.
	layouts map[string][]string
	// cDir and bDir are the directories carrying node c and bundle b.
	cDir, bDir string
}

func axisCases() []axisCase {
	N, F := layout.GroupByName, layout.GroupFlat
	none := []string{}
	return []axisCase{
		{N, N, N, map[string][]string{"": none, "c": none, "c/b": none, "c/b/a1": {"a1"}, "c/b/a2": {"a2"}, "c/b/x": {"x"}, "c/b/u": none, "c/b/u/ua": {"ua"}}, "c", "c/b"},
		{N, N, F, map[string][]string{"": none, "c": none, "c/b": {"a1", "a2"}, "c/b/x": {"x"}, "c/b/u": {"ua"}}, "c", "c/b"},
		{N, F, N, map[string][]string{"": none, "c": none, "c/a1": {"a1"}, "c/a2": {"a2"}, "c/x": {"x"}, "c/u": none, "c/u/ua": {"ua"}}, "c", "c"},
		{N, F, F, map[string][]string{"": none, "c": {"a1", "a2"}, "c/x": {"x"}, "c/u": {"ua"}}, "c", "c"},
		{F, N, N, map[string][]string{"": none, "b": none, "b/a1": {"a1"}, "b/a2": {"a2"}, "b/x": {"x"}, "b/u": none, "b/u/ua": {"ua"}}, "", "b"},
		{F, N, F, map[string][]string{"": none, "b": {"a1", "a2"}, "b/x": {"x"}, "b/u": {"ua"}}, "", "b"},
		{F, F, N, map[string][]string{"": none, "a1": {"a1"}, "a2": {"a2"}, "x": {"x"}, "u": none, "u/ua": {"ua"}}, "", ""},
		{F, F, F, map[string][]string{"": {"a1", "a2"}, "x": {"x"}, "u": {"ua"}}, "", ""},
	}
}

func join(prefix, rel string) string {
	if rel == "" {
		return prefix
	}
	if prefix == "" {
		return rel
	}
	return prefix + "/" + rel
}

// TestWalkCluster_GroupingAxes pins what each grouping axis does, for every
// combination, in both walkers: an axis set to GroupByName gives its level a
// directory, GroupFlat merges the level into the layout above it. Umbrella
// children and augmenter applications always keep their own directory. Every
// resource is written exactly once and origins land on the absorbing layout.
func TestWalkCluster_GroupingAxes(t *testing.T) {
	type walker struct {
		name   string
		prefix string // directory of node r
		wrap   string // extra wrapper directory, "" for none
		walk   func(c *stack.Cluster, r layout.LayoutRules) (*layout.ManifestLayout, error)
	}
	walkers := []walker{
		{"WalkCluster", "r", "", layout.WalkCluster},
		{"WalkCluster_ClusterName", "prod/r", "prod", func(c *stack.Cluster, r layout.LayoutRules) (*layout.ManifestLayout, error) {
			r.ClusterName = "prod"
			return layout.WalkCluster(c, r)
		}},
		{"WalkClusterByPackage", "r", "", func(c *stack.Cluster, r layout.LayoutRules) (*layout.ManifestLayout, error) {
			pkgs, err := layout.WalkClusterByPackage(c, r)
			if err != nil {
				return nil, err
			}
			return pkgs["default"], nil
		}},
	}
	for _, w := range walkers {
		for _, tc := range axisCases() {
			name := w.name + "/" + string(tc.node) + "_" + string(tc.bundle) + "_" + string(tc.app)
			t.Run(name, func(t *testing.T) {
				rules := layout.LayoutRules{NodeGrouping: tc.node, BundleGrouping: tc.bundle, ApplicationGrouping: tc.app}
				ml, err := w.walk(axesCluster(), rules)
				if err != nil {
					t.Fatalf("walk: %v", err)
				}
				if ml == nil {
					t.Fatal("walk returned no layout")
				}
				want := map[string][]string{}
				for rel, res := range tc.layouts {
					want[join(w.prefix, rel)] = res
				}
				if w.wrap != "" {
					want[w.wrap] = []string{}
				}
				got := layoutResources(t, ml)
				if !reflect.DeepEqual(got, want) {
					t.Errorf("layout tree:\n got %v\nwant %v", got, want)
				}

				nodes, bundles := originPaths(ml)
				if got, want := nodes["c"], join(w.prefix, tc.cDir); got != want {
					t.Errorf("node c origin at %q, want %q", got, want)
				}
				if got, want := bundles["b"], join(w.prefix, tc.bDir); got != want {
					t.Errorf("bundle b origin at %q, want %q", got, want)
				}
				uDir := join(w.prefix, join(tc.bDir, "u"))
				if got := bundles["u"]; got != uDir {
					t.Errorf("umbrella bundle u origin at %q, want %q", got, uDir)
				}
			})
		}
	}
}

// TestWalkCluster_ClusterNameRootFollowsAxes pins that the root node's bundle
// and its child nodes follow the grouping axes under a ClusterName too; the
// ClusterName wrapper used to flatten the root bundle and keep immediate
// child nodes as directories regardless of the axes.
func TestWalkCluster_ClusterNameRootFollowsAxes(t *testing.T) {
	build := func() *stack.Cluster {
		cb := &stack.Bundle{Name: "cb", Applications: []*stack.Application{axisApp("ca")}}
		c := &stack.Node{Name: "c", Bundle: cb}
		rb := &stack.Bundle{Name: "rb", Applications: []*stack.Application{axisApp("ra")}}
		r := &stack.Node{Name: "r", Bundle: rb, Children: []*stack.Node{c}}
		c.SetParent(r)
		return &stack.Cluster{Name: "demo", Node: r}
	}
	N, F := layout.GroupByName, layout.GroupFlat
	none := []string{}
	tests := []struct {
		name              string
		node, bundle, app layout.GroupingMode
		want              map[string][]string
	}{
		{"nested", N, N, N, map[string][]string{"prod": none, "prod/r": none, "prod/r/rb": none, "prod/r/rb/ra": {"ra"}, "prod/r/c": none, "prod/r/c/cb": none, "prod/r/c/cb/ca": {"ca"}}},
		{"bundle_dirs", N, N, F, map[string][]string{"prod": none, "prod/r": none, "prod/r/rb": {"ra"}, "prod/r/c": none, "prod/r/c/cb": {"ca"}}},
		{"node_only", N, F, F, map[string][]string{"prod": none, "prod/r": {"ra"}, "prod/r/c": {"ca"}}},
		{"all_flat", F, F, F, map[string][]string{"prod": none, "prod/r": {"ca", "ra"}}},
		{"flat_nodes_bundle_dirs", F, N, F, map[string][]string{"prod": none, "prod/r": none, "prod/r/rb": {"ra"}, "prod/r/cb": {"ca"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ml, err := layout.WalkCluster(build(), layout.LayoutRules{
				ClusterName: "prod", NodeGrouping: tc.node, BundleGrouping: tc.bundle, ApplicationGrouping: tc.app,
			})
			if err != nil {
				t.Fatalf("walk: %v", err)
			}
			if got := layoutResources(t, ml); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("layout tree:\n got %v\nwant %v", got, tc.want)
			}
		})
	}
}

// TestWalkCluster_FlatMergeCollisionRefused pins that a merge which makes two
// layouts claim one directory, or two resources one Kubernetes identity, is
// refused before anything is written, by every writer.
func TestWalkCluster_FlatMergeCollisionRefused(t *testing.T) {
	flat := layout.LayoutRules{NodeGrouping: layout.GroupFlat, BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupFlat}
	tests := []struct {
		name    string
		apps    func(i string) []*stack.Application
		wantErr string
	}{
		{
			name:    "same_augmenter_directory",
			apps:    func(string) []*stack.Application { return []*stack.Application{axisAugmenterApp("x")} },
			wantErr: "resolve to the same directory",
		},
		{
			name: "same_object_identity",
			apps: func(string) []*stack.Application {
				return []*stack.Application{stack.NewApplication("dup", "default", &fakeConfig{objs: []*client.Object{axisObj("cm", "default")}})}
			},
			wantErr: "same object",
		},
		{
			// kustomize reads an omitted namespace as "default": the two
			// objects are one.
			name: "omitted_vs_default_namespace",
			apps: func(i string) []*stack.Application {
				ns := "default"
				if i == "1" {
					ns = ""
				}
				return []*stack.Application{stack.NewApplication("dup"+i, "default", &fakeConfig{objs: []*client.Object{axisObj("cm", ns)}})}
			},
			wantErr: "same object",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			build := func() *layout.ManifestLayout {
				c1 := &stack.Node{Name: "c1", Bundle: &stack.Bundle{Name: "b1", Applications: tc.apps("1")}}
				c2 := &stack.Node{Name: "c2", Bundle: &stack.Bundle{Name: "b2", Applications: tc.apps("2")}}
				r := &stack.Node{Name: "r", Children: []*stack.Node{c1, c2}}
				c1.SetParent(r)
				c2.SetParent(r)
				ml, err := layout.WalkCluster(&stack.Cluster{Name: "demo", Node: r}, flat)
				if err != nil {
					t.Fatalf("walk: %v", err)
				}
				return ml
			}
			dir := t.TempDir()
			errs := map[string]error{
				"WriteToDisk":   build().WriteToDisk(filepath.Join(dir, "disk")),
				"WriteToTar":    build().WriteToTar(&strings.Builder{}),
				"WriteManifest": layout.WriteManifest(filepath.Join(dir, "manifest"), layout.DefaultLayoutConfig(), build()),
			}
			for writer, err := range errs {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("%s: err = %v, want one containing %q", writer, err, tc.wantErr)
				}
			}
		})
	}
}

// TestWalkCluster_FilePerKindHonouredWhenFlat pins that FilePer is honoured
// in every grouping shape: node-only mode used to force per-resource files,
// and application and bundle layouts were built without FilePer.
func TestWalkCluster_FilePerKindHonouredWhenFlat(t *testing.T) {
	N, F := layout.GroupByName, layout.GroupFlat
	tests := []struct {
		name              string
		node, bundle, app layout.GroupingMode
		dir               string // directory holding a1 and a2, below the root r
	}{
		{"node_only", N, F, F, "r/c"},
		{"all_flat", F, F, F, "r"},
		{"bundle_dirs", N, N, F, "r/c/b"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ml, err := layout.WalkCluster(axesCluster(), layout.LayoutRules{
				NodeGrouping: tc.node, BundleGrouping: tc.bundle, ApplicationGrouping: tc.app, FilePer: layout.FilePerKind,
			})
			if err != nil {
				t.Fatalf("walk: %v", err)
			}
			out := t.TempDir()
			if err := ml.WriteToDisk(filepath.Join(out, "disk")); err != nil {
				t.Fatalf("WriteToDisk: %v", err)
			}
			// WriteManifest's Config says per-resource: the layouts' own
			// FilePer must win, so the walk's setting reaches every writer.
			cfg := layout.DefaultLayoutConfig()
			cfg.FilePer = layout.FilePerResource
			if err := layout.WriteManifest(filepath.Join(out, "manifest"), cfg, ml); err != nil {
				t.Fatalf("WriteManifest: %v", err)
			}
			for writer, dir := range map[string]string{
				"WriteToDisk":   filepath.Join(out, "disk", tc.dir),
				"WriteManifest": filepath.Join(out, "manifest", cfg.ManifestsDir, tc.dir),
			} {
				entries, err := os.ReadDir(dir)
				if err != nil {
					t.Fatalf("%s: read %s: %v", writer, dir, err)
				}
				var files []string
				for _, e := range entries {
					if !e.IsDir() && e.Name() != "kustomization.yaml" {
						files = append(files, e.Name())
					}
				}
				if want := []string{"default-configmap.yaml"}; !reflect.DeepEqual(files, want) {
					t.Errorf("%s: files in %s = %v, want %v (a1 and a2 grouped per kind)", writer, tc.dir, files, want)
				}
			}
		})
	}
}

// TestWalkCluster_AbsorbThenFlatten pins that FlattenSingleTier still runs
// after the grouping merge and carries an augmenter's ExtraFiles and
// ConfigMapGenerators when it collapses the augmenter's directory.
func TestWalkCluster_AbsorbThenFlatten(t *testing.T) {
	c := &stack.Node{Name: "c", Bundle: &stack.Bundle{Name: "b", Applications: []*stack.Application{axisAugmenterApp("x")}}}
	r := &stack.Node{Name: "r", Children: []*stack.Node{c}}
	c.SetParent(r)
	ml, err := layout.WalkCluster(&stack.Cluster{Name: "demo", Node: r}, layout.LayoutRules{
		NodeGrouping: layout.GroupFlat, BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupFlat,
		FlattenSingleTier: true,
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(ml.Children) != 0 {
		t.Fatalf("sole augmenter child not collapsed: %d children", len(ml.Children))
	}
	if len(ml.ExtraFiles) != 1 || len(ml.ConfigMapGenerators) != 1 {
		t.Errorf("collapsed augmenter lost its files: ExtraFiles=%d ConfigMapGenerators=%d", len(ml.ExtraFiles), len(ml.ConfigMapGenerators))
	}
	if got := layoutResources(t, ml); !reflect.DeepEqual(got, map[string][]string{"r": {"x"}}) {
		t.Errorf("layout tree = %v, want r:[x]", got)
	}
}

// TestWalkClusterByPackage_FlatEmptyBundleRetainsOrigins pins that a package
// whose only content is an empty bundle, merged by NodeGrouping flat into the
// wrapper of a root outside the package, is still returned: the wrapper
// renders that node and bundle even though it holds no resources.
func TestWalkClusterByPackage_FlatEmptyBundleRetainsOrigins(t *testing.T) {
	oci := &schema.GroupVersionKind{Group: "source.toolkit.fluxcd.io", Version: "v1", Kind: "OCIRepository"}
	c := &stack.Node{Name: "c", PackageRef: oci, Bundle: &stack.Bundle{Name: "cb"}}
	r := &stack.Node{Name: "r", Children: []*stack.Node{c}}
	c.SetParent(r)
	for _, node := range []layout.GroupingMode{layout.GroupByName, layout.GroupFlat} {
		t.Run(string(node), func(t *testing.T) {
			pkgs, err := layout.WalkClusterByPackage(&stack.Cluster{Name: "demo", Node: r}, layout.LayoutRules{NodeGrouping: node})
			if err != nil {
				t.Fatalf("walk: %v", err)
			}
			ml := pkgs[oci.String()]
			if ml == nil {
				t.Fatalf("package %s missing; got %v", oci.String(), pkgs)
			}
			_, bundles := originPaths(ml)
			if _, ok := bundles["cb"]; !ok {
				t.Errorf("empty bundle cb not rendered by any layout of the package")
			}
		})
	}
}

// TestWriters_DisjointLists pins that the writers' object-identity check reads
// a List's items, not its envelope: two unnamed v1 Lists holding different
// ConfigMaps build fine with kustomize and must be written, while two Lists
// carrying one ConfigMap are refused.
func TestWriters_DisjointLists(t *testing.T) {
	list := func(items ...string) *client.Object {
		l := &unstructured.UnstructuredList{}
		l.SetAPIVersion("v1")
		l.SetKind("List")
		for _, name := range items {
			l.Items = append(l.Items, *(*axisObj(name, "default")).(*unstructured.Unstructured))
		}
		u := &unstructured.Unstructured{}
		if err := u.UnmarshalJSON(mustJSON(t, l)); err != nil {
			t.Fatalf("list: %v", err)
		}
		var o client.Object = u
		return &o
	}
	for _, tc := range []struct {
		name    string
		second  string
		wantErr string
	}{
		{"disjoint", "cm-b", ""},
		{"shared_item", "cm-a", "same object"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			build := func() *layout.ManifestLayout {
				app := stack.NewApplication("lists", "default", &fakeConfig{objs: []*client.Object{list("cm-a"), list(tc.second)}})
				r := &stack.Node{Name: "r", Bundle: &stack.Bundle{Name: "b", Applications: []*stack.Application{app}}}
				ml, err := layout.WalkCluster(&stack.Cluster{Name: "demo", Node: r}, layout.LayoutRules{})
				if err != nil {
					t.Fatalf("walk: %v", err)
				}
				return ml
			}
			dir := t.TempDir()
			errs := map[string]error{
				"WriteToDisk":   build().WriteToDisk(filepath.Join(dir, "disk")),
				"WriteToTar":    build().WriteToTar(&strings.Builder{}),
				"WriteManifest": layout.WriteManifest(filepath.Join(dir, "manifest"), layout.DefaultLayoutConfig(), build()),
			}
			for writer, err := range errs {
				if tc.wantErr == "" && err != nil {
					t.Errorf("%s: %v", writer, err)
				}
				if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
					t.Errorf("%s: err = %v, want one containing %q", writer, err, tc.wantErr)
				}
			}
		})
	}
}

func mustJSON(t *testing.T, v interface{ MarshalJSON() ([]byte, error) }) []byte {
	t.Helper()
	b, err := v.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// TestWriters_IdentityIgnoresObjectsWithoutKind pins that the writers'
// identity check only compares objects it can identify: typed objects with an
// unset TypeMeta carry no kind, and two of different Go types sharing a name
// must not be refused as "the same object".
func TestWriters_IdentityIgnoresObjectsWithoutKind(t *testing.T) {
	cm := &corev1.ConfigMap{}
	cm.SetName("app")
	cm.SetNamespace("default")
	secret := &corev1.Secret{}
	secret.SetName("app")
	secret.SetNamespace("default")
	var a, b client.Object = cm, secret
	app := stack.NewApplication("typed", "default", &fakeConfig{objs: []*client.Object{&a, &b}})
	r := &stack.Node{Name: "r", Bundle: &stack.Bundle{Name: "b", Applications: []*stack.Application{app}}}
	ml, err := layout.WalkCluster(&stack.Cluster{Name: "demo", Node: r}, layout.LayoutRules{})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if err := ml.WriteToTar(&strings.Builder{}); err != nil && strings.Contains(err.Error(), "same object") {
		t.Errorf("WriteToTar refused two objects of different Go types: %v", err)
	}
}
