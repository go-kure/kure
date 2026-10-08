package fluxcd_test

import (
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for the refusal of what a layout sets for a Flux Kustomization of its
// own where it gets none (go-kure/kure#1032): under FluxIntegratedPerBundle and
// FluxSeparate no layout gets one, and under FluxIntegratedPerLayout the top of
// the tree, a layout that renders a bundle, an umbrella child and an
// AppFileSingle layout get none.

// layoutField is one of the eleven settings, with a value that is valid where
// a Kustomization carries it, so that only the refusal can refuse it.
type layoutField struct {
	name string
	set  func(l *layout.ManifestLayout)
}

func layoutFields() []layoutField {
	yes := true
	return []layoutField{
		{"DependsOn", func(l *layout.ManifestLayout) { l.DependsOn = []string{"external"} }},
		{"KustomizationName", func(l *layout.ManifestLayout) { l.KustomizationName = "custom" }},
		{"Wait", func(l *layout.ManifestLayout) { l.Wait = &yes }},
		{"Timeout", func(l *layout.ManifestLayout) { l.Timeout = "1m" }},
		{"RetryInterval", func(l *layout.ManifestLayout) { l.RetryInterval = "1m" }},
		{"Labels", func(l *layout.ManifestLayout) { l.Labels = map[string]string{"team": "shop"} }},
		{"Annotations", func(l *layout.ManifestLayout) { l.Annotations = map[string]string{"owner": "shop"} }},
		{"Interval", func(l *layout.ManifestLayout) { l.Interval = "1m" }},
		{"Prune", func(l *layout.ManifestLayout) { l.Prune = &yes }},
		{"Force", func(l *layout.ManifestLayout) { l.Force = &yes }},
		{"Suspend", func(l *layout.ManifestLayout) { l.Suspend = &yes }},
	}
}

// allLayoutFields is every field, in the order the refusal lists them.
const allLayoutFields = "DependsOn, KustomizationName, Wait, Timeout, RetryInterval, Labels, Annotations, Interval, Prune, Force, Suspend"

// unorderedShop is the bundle "shop" with the application "db" and the
// layouts of settingsAugmenter, none of which sets a field.
func unorderedShop() *stack.Cluster {
	return oneBundleCluster(srBundle("shop", stack.NewApplication("db", "default", settingsAugmenter{set: unordered(nil)})))
}

// integrateEdited walks c, lets edit change the walked tree, and integrates
// it. It returns the tree and the integration's error.
func integrateEdited(t *testing.T, c *stack.Cluster, rules layout.LayoutRules, edit func(ml *layout.ManifestLayout)) (*layout.ManifestLayout, error) {
	t.Helper()
	ml := mustWalk(t, c, rules)
	edit(ml)
	return ml, fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
}

// mustRefuseLayout fails unless err refuses the layout at path for setting
// fields, for the reason why, naming the placement that carries the fields.
func mustRefuseLayout(t *testing.T, err error, path, fields, why string) {
	t.Helper()
	mustContainAll(t, err,
		`layout "`+path+`" sets `+fields+`, but has no Flux Kustomization of its own: `+why,
		`FluxPlacement "integrated" (FluxIntegratedPerLayout)`)
}

// TestLayoutFields_Refused: each of the eleven fields, set on a layout that
// gets no Kustomization of its own, is refused under each placement. The
// error names the layout, the field and FluxIntegratedPerLayout, and the
// refused call places no Kustomization.
func TestLayoutFields_Refused(t *testing.T) {
	for _, tc := range []struct {
		placement layout.FluxPlacement
		path      string
		why       string
	}{
		{layout.FluxIntegratedPerLayout, "prod/shop", `it renders bundle "shop"`},
		{layout.FluxIntegratedPerBundle, "prod/shop/db/00-pre", `FluxPlacement "integrated-per-bundle" gives a Kustomization to bundles alone`},
		{layout.FluxSeparate, "prod/shop/db/00-pre", `ResourceGenerator.GenerateFromLayout, which FluxPlacement "separate" uses, gives a Kustomization to bundles alone`},
	} {
		for _, f := range layoutFields() {
			t.Run(string(tc.placement)+"/"+f.name, func(t *testing.T) {
				rules := layout.DefaultLayoutRules()
				rules.FluxPlacement = tc.placement
				ml, err := integrateEdited(t, unorderedShop(), rules, func(ml *layout.ManifestLayout) {
					f.set(layoutAtPath(t, ml, tc.path))
				})
				mustRefuseLayout(t, err, tc.path, f.name, tc.why)
				if names := kustomizationsByName(ml); len(names) != 0 {
					t.Errorf("the refused call left Kustomizations in the tree: %v", names)
				}
			})
		}
	}
}

// TestLayoutFields_NoOwnKustomizationUnderPerLayout: under
// FluxIntegratedPerLayout the layouts that can never get a Kustomization of
// their own refuse every field, and the error lists all of them in
// ManifestLayout's order.
func TestLayoutFields_NoOwnKustomizationUnderPerLayout(t *testing.T) {
	setAll := func(l *layout.ManifestLayout) {
		for _, f := range layoutFields() {
			f.set(l)
		}
	}
	for _, tc := range []struct {
		name    string
		cluster func() *stack.Cluster
		rules   func() layout.LayoutRules
		target  func(t *testing.T, ml *layout.ManifestLayout) *layout.ManifestLayout
		why     string
	}{
		{
			name:    "the top of the tree",
			cluster: unorderedShop,
			rules:   perLayoutRules,
			target:  func(_ *testing.T, ml *layout.ManifestLayout) *layout.ManifestLayout { return ml },
			why:     "it is the root of the tree, which the Flux bootstrap applies",
		},
		{
			name:    "a layout that renders a bundle",
			cluster: unorderedShop,
			rules:   perLayoutRules,
			target: func(t *testing.T, ml *layout.ManifestLayout) *layout.ManifestLayout {
				return layoutAtPath(t, ml, "prod/shop")
			},
			why: `it renders bundle "shop", and that bundle's Kustomization applies it`,
		},
		{
			// The walker's umbrella child layout renders the child bundle,
			// which is the reason given.
			name: "an umbrella child the walker made",
			cluster: func() *stack.Cluster {
				shop := srBundle("shop", cmApp("shop-app"))
				shop.Children = []*stack.Bundle{srBundle("infra", cmApp("infra-app"))}
				return oneBundleCluster(shop)
			},
			rules: func() layout.LayoutRules {
				r := perLayoutRules()
				r.ApplicationGrouping = layout.GroupByName
				return r
			},
			target: func(t *testing.T, ml *layout.ManifestLayout) *layout.ManifestLayout {
				l := umbrellaChildLayout(ml)
				if l == nil {
					t.Fatal("the walked tree holds no umbrella child layout")
				}
				return l
			},
			why: `it renders bundle "`,
		},
		{
			// One that renders no bundle is refused for being an umbrella
			// child.
			name:    "a layout a caller marked as an umbrella child",
			cluster: unorderedShop,
			rules:   perLayoutRules,
			target: func(t *testing.T, ml *layout.ManifestLayout) *layout.ManifestLayout {
				l := layoutAtPath(t, ml, "prod/shop/db/00-pre")
				l.UmbrellaChild = true
				return l
			},
			why: "it is an umbrella child, which its bundle's Kustomization applies",
		},
		{
			name:    "an AppFileSingle layout",
			cluster: unorderedShop,
			rules:   perLayoutRules,
			target: func(t *testing.T, ml *layout.ManifestLayout) *layout.ManifestLayout {
				l := layoutAtPath(t, ml, "prod/shop/db/00-pre")
				l.ApplicationFileMode = layout.AppFileSingle
				return l
			},
			why: "it is AppFileSingle, written as a file into its parent's directory",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var path string
			_, err := integrateEdited(t, tc.cluster(), tc.rules(), func(ml *layout.ManifestLayout) {
				l := tc.target(t, ml)
				setAll(l)
				path = l.FullRepoPath()
			})
			mustRefuseLayout(t, err, path, allLayoutFields, tc.why)
		})
	}
}

// umbrellaChildLayout returns the first layout in pre-order the walker made
// for an umbrella child, or nil.
func umbrellaChildLayout(l *layout.ManifestLayout) *layout.ManifestLayout {
	if l == nil {
		return nil
	}
	if l.UmbrellaChild {
		return l
	}
	for _, c := range l.Children {
		if found := umbrellaChildLayout(c); found != nil {
			return found
		}
	}
	return nil
}

// TestLayoutFields_GenerateFromLayout: ResourceGenerator.GenerateFromLayout
// gives a Kustomization to bundles alone, so it refuses a layout's field as
// FluxSeparate does.
func TestLayoutFields_GenerateFromLayout(t *testing.T) {
	c := unorderedShop()
	rules := layout.DefaultLayoutRules()
	rules.FluxPlacement = layout.FluxSeparate
	ml := mustWalk(t, c, rules)
	layoutAtPath(t, ml, "prod/shop/db/01-main").Timeout = "1m"
	_, err := fluxstack.NewResourceGenerator().GenerateFromLayout(ml, c)
	mustRefuseLayout(t, err, "prod/shop/db/01-main", "Timeout",
		`ResourceGenerator.GenerateFromLayout, which FluxPlacement "separate" uses`)
}

// TestLayoutFields_NodeValueRefusedOnce: the walker carries a node's
// KustomizationName and NamedDependsOn on the node's own layout. Where the
// node gets no Kustomization of its own, the node is refused by its path, and
// the layout carrying its values is not refused again.
func TestLayoutFields_NodeValueRefusedOnce(t *testing.T) {
	for _, tc := range []struct {
		name  string
		node  func(platform, apps *stack.Node) *stack.Node
		rules layout.LayoutRules
		path  string
	}{
		{"a group node under FluxIntegratedPerBundle", func(_, apps *stack.Node) *stack.Node { return apps },
			placed(propertyGroupings["nodeOnly"], layout.FluxIntegratedPerBundle), "prod/apps"},
		{"a group node under FluxSeparate", func(_, apps *stack.Node) *stack.Node { return apps },
			placed(propertyGroupings["nodeOnly"], layout.FluxSeparate), "prod/apps"},
		{"a node whose layout renders a bundle", func(_, apps *stack.Node) *stack.Node { return apps.Children[0] },
			perLayoutNodeOnly(), "prod/apps/shop"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, platform, apps := groupsCluster()
			n := tc.node(platform, apps)
			n.KustomizationName = "named"
			n.NamedDependsOn = []string{"external"}
			_, err := integrateCluster(c, tc.rules)
			mustContainAll(t, err, `node "`+tc.path+`" sets KustomizationName, NamedDependsOn`)
			if err != nil && strings.Contains(err.Error(), `layout "`+tc.path+`" sets`) {
				t.Errorf("the node's values are refused on its layout as well:\n%v", err)
			}
		})
	}
}
