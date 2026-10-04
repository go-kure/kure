package layout_test

import (
	"slices"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for the per-application record of a walked layout: which objects each
// application rendered, wherever a grouping axis wrote them, so a workflow can
// act on one application's objects (go-kure/kure#974).

// hookConfig emits one ConfigMap and, as a LayoutAugmenter, adds a
// configMapGenerator to its layout and a child layout holding one more
// ConfigMap, with a layout below that holding another.
type hookConfig struct{ name string }

func namedConfigMap(name string) client.Object {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("v1")
	u.SetKind("ConfigMap")
	u.SetName(name)
	u.SetNamespace("default")
	return u
}

func (h *hookConfig) Generate(*stack.Application) ([]*client.Object, error) {
	o := namedConfigMap(h.name + "-cm")
	return []*client.Object{&o}, nil
}

func (h *hookConfig) AugmentLayout(ml *layout.ManifestLayout) error {
	ml.ExtraFiles = append(ml.ExtraFiles, layout.ExtraFile{Name: "values.yaml", Content: []byte("k: v\n")})
	ml.ConfigMapGenerators = append(ml.ConfigMapGenerators, layout.ConfigMapGeneratorSpec{Name: h.name + "-values", Files: []string{"values.yaml"}})
	hooks := &layout.ManifestLayout{
		Name:      h.name + "-hooks",
		Namespace: ml.FullRepoPath(),
		Resources: []client.Object{namedConfigMap(h.name + "-hook")},
	}
	hooks.Children = []*layout.ManifestLayout{{
		Name:      h.name + "-post",
		Namespace: hooks.FullRepoPath(),
		Resources: []client.Object{namedConfigMap(h.name + "-post")},
	}}
	ml.Children = append(ml.Children, hooks)
	return nil
}

// applicationObjects collects the per-application records of the whole tree,
// keyed by application name.
func applicationObjects(t *testing.T, root *layout.ManifestLayout) map[string]layout.ApplicationObjects {
	t.Helper()
	out := map[string]layout.ApplicationObjects{}
	var visit func(l *layout.ManifestLayout)
	visit = func(l *layout.ManifestLayout) {
		for _, rec := range l.OriginApplicationObjects() {
			if rec.Application == nil {
				t.Fatalf("%s: a record without an application", l.FullRepoPath())
			}
			if _, dup := out[rec.Application.Name]; dup {
				t.Fatalf("application %q is recorded twice", rec.Application.Name)
			}
			out[rec.Application.Name] = rec
		}
		for _, c := range l.Children {
			visit(c)
		}
	}
	visit(root)
	return out
}

func objectNames(objs []client.Object) []string {
	var out []string
	for _, o := range objs {
		out = append(out, o.GetName())
	}
	slices.Sort(out)
	return out
}

// chartCluster is node platform, whose bundle has a plain application and an
// augmenter application.
func chartCluster() *stack.Cluster {
	b := &stack.Bundle{Name: "platform", Applications: []*stack.Application{
		configMapApp("core"),
		stack.NewApplication("chart", "default", &hookConfig{name: "chart"}),
	}}
	return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: b}}
}

func TestOriginApplicationObjects_EveryGrouping(t *testing.T) {
	for name, rules := range map[string]layout.LayoutRules{"groupByName": groupByName, "nodeOnly": nodeOnly, "nodeFlat": nodeFlat} {
		t.Run(name, func(t *testing.T) {
			c := chartCluster()
			ml := walk(t, c, rules)
			recs := applicationObjects(t, ml)
			if len(recs) != 2 {
				t.Fatalf("recorded applications %d, want 2", len(recs))
			}

			core := recs["core"]
			if core.Application != c.Node.Bundle.Applications[0] {
				t.Errorf("core record names another application")
			}
			if len(core.Objects) != 1 {
				t.Fatalf("core objects = %v, want its one ConfigMap", objectNames(core.Objects))
			}
			// The record holds the very object the layout writes: the
			// application's own layout or, with ApplicationGrouping flat, the
			// directory that renders its bundle. For the root node's bundle
			// that is the root layout's unit (go-kure/kure#979).
			holder := core.Layout
			if holder == nil {
				holder = ml.OriginUnit()
			}
			if holder == nil {
				t.Fatalf("core has no layout of its own and the root layout has no unit: %v", collectRepoPaths(ml))
			}
			if !slices.Contains(holder.Resources, core.Objects[0]) {
				t.Errorf("core's recorded object is not the one in layout %q", holder.FullRepoPath())
			}
			if flat := rules.ApplicationGrouping == layout.GroupFlat; flat != (core.Layout == nil) {
				t.Errorf("core.Layout = %v with ApplicationGrouping %q", core.Layout, rules.ApplicationGrouping)
			}

			// The augmenter application has its own layout in every grouping,
			// and its record covers what the augmenter added below it.
			chart := recs["chart"]
			if got, want := objectNames(chart.Objects), []string{"chart-cm", "chart-hook", "chart-post"}; !slices.Equal(got, want) {
				t.Errorf("chart objects = %v, want %v", got, want)
			}
			if chart.Layout == nil || chart.Layout.OriginApplication() != chart.Application {
				t.Fatalf("chart.Layout = %v, want the application's own layout", chart.Layout)
			}
			if len(chart.Layout.ConfigMapGenerators) != 1 {
				t.Errorf("chart layout generators = %v, want the augmenter's", chart.Layout.ConfigMapGenerators)
			}
		})
	}
}

func TestOriginApplicationObjects_RecordedOnTheBundleLayout(t *testing.T) {
	c := umbrellaCluster()
	ml := walk(t, c, groupByName)
	for path, want := range map[string][]string{
		"platform/platform":            {"core"},
		"platform/platform/svc":        {"api"},
		"platform/platform/svc/svc-db": {"db"},
	} {
		var got []string
		for _, rec := range layoutAt(t, ml, path).OriginApplicationObjects() {
			got = append(got, rec.Application.Name)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s records applications %v, want %v", path, got, want)
		}
	}
	if got := layoutAt(t, ml, "platform").OriginApplicationObjects(); len(got) != 0 {
		t.Errorf("a node layout that renders no bundle records %d applications", len(got))
	}
}

func TestOriginApplicationObjects_HandBuiltLayoutHasNone(t *testing.T) {
	ml := &layout.ManifestLayout{Name: "p", Resources: []client.Object{namedConfigMap("x")}}
	if got := ml.OriginApplicationObjects(); got != nil {
		t.Errorf("hand-built layout records %v", got)
	}
}

// TestOriginApplicationObjects_FlattenSingleTier: the option collapses no
// directory that renders a bundle (go-kure/kure#979), so the records stay on
// the layout that renders the bundle and name the layouts they named. Each
// shape below collapsed before that change.
func TestOriginApplicationObjects_FlattenSingleTier(t *testing.T) {
	t.Run("child node's bundle layout", func(t *testing.T) {
		web := &stack.Node{Name: "web", Bundle: &stack.Bundle{Name: "web", Applications: []*stack.Application{configMapApp("frontend")}}}
		c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Children: []*stack.Node{web}}}
		rules := nodeOnly
		rules.FlattenSingleTier = true
		ml := walk(t, c, rules)
		if got := ml.OriginApplicationObjects(); len(got) != 0 {
			t.Errorf("the root layout, which renders no bundle, records %d applications", len(got))
		}
		unit := layoutAt(t, ml, "platform/web")
		recs := unit.OriginApplicationObjects()
		if len(recs) != 1 || recs[0].Application.Name != "frontend" || recs[0].Layout != nil {
			t.Fatalf("records of platform/web = %+v, want frontend, written into that directory", recs)
		}
		if len(recs[0].Objects) != 1 || !slices.Contains(unit.Resources, recs[0].Objects[0]) {
			t.Errorf("frontend's recorded object is not in the layout that renders its bundle")
		}
	})
	t.Run("root bundle with an augmenter application", func(t *testing.T) {
		b := &stack.Bundle{Name: "platform", Applications: []*stack.Application{stack.NewApplication("chart", "default", &hookConfig{name: "chart"})}}
		c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: b}}
		rules := layout.LayoutRules{BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupByName, FlattenSingleTier: true}
		ml := walk(t, c, rules)
		if got := ml.OriginApplicationObjects(); len(got) != 0 {
			t.Errorf("the root layout, which renders no bundle, records %d applications", len(got))
		}
		recs := layoutAt(t, ml, "platform/platform").OriginApplicationObjects()
		if len(recs) != 1 {
			t.Fatalf("records of platform/platform = %+v, want the chart application", recs)
		}
		if recs[0].Layout == nil || recs[0].Layout.OriginApplication() != recs[0].Application {
			t.Errorf("chart.Layout = %v, want the layout that renders it", recs[0].Layout)
		}
	})
	t.Run("root bundle with a plain application", func(t *testing.T) {
		b := &stack.Bundle{Name: "platform", Applications: []*stack.Application{configMapApp("core")}}
		c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: b}}
		rules := layout.LayoutRules{BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupByName, FlattenSingleTier: true}
		ml := walk(t, c, rules)
		if got := ml.OriginApplicationObjects(); len(got) != 0 {
			t.Errorf("the root layout, which renders no bundle, records %d applications", len(got))
		}
		recs := layoutAt(t, ml, "platform/platform").OriginApplicationObjects()
		own := layoutAt(t, ml, "platform/platform/core")
		if len(recs) != 1 || recs[0].Layout != own {
			t.Fatalf("records of platform/platform = %+v, want core with its own layout", recs)
		}
		if own.OriginApplication() != recs[0].Application {
			t.Errorf("platform/platform/core does not render the recorded application")
		}
	})
}

func TestOriginApplicationObjects_WalkClusterByPackage(t *testing.T) {
	pkgs, err := layout.WalkClusterByPackage(chartCluster(), groupByName)
	if err != nil {
		t.Fatal(err)
	}
	found, charts := 0, 0
	for _, ml := range pkgs {
		recs := applicationObjects(t, ml)
		found += len(recs)
		if chart, ok := recs["chart"]; ok {
			charts++
			if got, want := objectNames(chart.Objects), []string{"chart-cm", "chart-hook", "chart-post"}; !slices.Equal(got, want) {
				t.Errorf("chart objects = %v, want %v", got, want)
			}
		}
	}
	if found != 2 {
		t.Errorf("recorded applications %d, want 2", found)
	}
	if charts != 1 {
		t.Errorf("chart is recorded in %d packages, want one", charts)
	}
}
