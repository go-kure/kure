package fluxcd_test

import (
	"maps"
	"slices"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for node-level Flux Kustomizations (go-kure/kure#973): a bundle-less
// node's Kustomization under FluxIntegratedPerLayout is named and ordered
// from the model, and the Kustomizations of application and augmenter layouts
// are named after their unit.

// groupsCluster is a root "prod" with two bundle-less group nodes, "platform"
// and "apps", each holding one node with a bundle.
func groupsCluster() (c *stack.Cluster, platform, apps *stack.Node) {
	platform = &stack.Node{Name: "platform", Children: []*stack.Node{
		{Name: "cert-manager", Bundle: srBundle("cert-manager", cmApp("cert-manager-app"))},
	}}
	apps = &stack.Node{Name: "apps", Children: []*stack.Node{
		{Name: "shop", Bundle: srBundle("shop", cmApp("shop-app"))},
	}}
	root := &stack.Node{Name: "prod", Children: []*stack.Node{platform, apps}}
	return &stack.Cluster{Name: "demo", Node: root}, platform, apps
}

func placed(r layout.LayoutRules, p layout.FluxPlacement) layout.LayoutRules {
	r.FluxPlacement = p
	return r
}

func perLayoutNodeOnly() layout.LayoutRules {
	return placed(propertyGroupings["nodeOnly"], layout.FluxIntegratedPerLayout)
}

// kustPaths maps every Kustomization in the tree to its spec.path.
func kustPaths(ml *layout.ManifestLayout) map[string]string {
	out := map[string]string{}
	for _, k := range kustomizations(ml) {
		out[k.Name] = k.Spec.Path
	}
	return out
}

func kustNamed(t *testing.T, ml *layout.ManifestLayout, name string) *kustv1.Kustomization {
	t.Helper()
	for _, k := range kustomizations(ml) {
		if k.Name == name {
			return k
		}
	}
	t.Fatalf("no Kustomization %q in the tree; have %v", name, kustPaths(ml))
	return nil
}

func dependsOnInOrder(k *kustv1.Kustomization) []string {
	var out []string
	for _, d := range k.Spec.DependsOn {
		out = append(out, d.Name)
	}
	return out
}

func integrateCluster(c *stack.Cluster, rules layout.LayoutRules) (*layout.ManifestLayout, error) {
	return fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
}

// mustContainAll fails unless err is non-nil and its text holds every want.
func mustContainAll(t *testing.T, err error, wants ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("accepted; want an error containing %q", wants)
	}
	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not contain %q:\n%v", want, err)
		}
	}
}

// TestNodeKustomization_NameAndNamedDependsOn: a group node's
// KustomizationName names its Kustomization, and its NamedDependsOn entries
// are written as given. The directories do not move, and the tree is
// writable.
func TestNodeKustomization_NameAndNamedDependsOn(t *testing.T) {
	c, platform, apps := groupsCluster()
	platform.KustomizationName = "platform"
	apps.KustomizationName = "apps"
	apps.NamedDependsOn = []string{"platform", "external"}

	ml := integrated(t, c, perLayoutNodeOnly())

	want := map[string]string{
		"platform":     "prod/platform",
		"apps":         "prod/apps",
		"cert-manager": "prod/platform/cert-manager",
		"shop":         "prod/apps/shop",
	}
	if got := kustPaths(ml); !mapsEqual(got, want) {
		t.Errorf("Kustomizations = %v, want %v", got, want)
	}
	if got := dependsOnInOrder(kustNamed(t, ml, "apps")); !slices.Equal(got, []string{"platform", "external"}) {
		t.Errorf("apps dependsOn = %v, want [platform external]", got)
	}
	if got := dependsOnInOrder(kustNamed(t, ml, "platform")); len(got) != 0 {
		t.Errorf("platform dependsOn = %v, want none", got)
	}
	writeAll(t, ml)
}

// TestNodeKustomization_DependsOnNode: Node.DependsOn renders the target
// node's Kustomization name in effect: its KustomizationName, or the derived
// "-node" name without one. A node without the fields keeps its "-node" name.
func TestNodeKustomization_DependsOnNode(t *testing.T) {
	for _, tc := range []struct {
		name       string
		targetName string
		wantTarget string
	}{
		{"target sets KustomizationName", "infra", "infra"},
		{"target keeps the derived name", "", "prod-platform-node"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, platform, apps := groupsCluster()
			platform.KustomizationName = tc.targetName
			apps.DependsOn = []*stack.Node{platform}
			apps.NamedDependsOn = []string{"external"}

			ml := integrated(t, c, perLayoutNodeOnly())

			k := kustNamed(t, ml, "prod-apps-node")
			if k.Spec.Path != "prod/apps" {
				t.Errorf("spec.path = %q, want prod/apps", k.Spec.Path)
			}
			if got, want := dependsOnInOrder(k), []string{tc.wantTarget, "external"}; !slices.Equal(got, want) {
				t.Errorf("dependsOn = %v, want %v", got, want)
			}
			if got := kustNamed(t, ml, tc.wantTarget).Spec.Path; got != "prod/platform" {
				t.Errorf("target spec.path = %q, want prod/platform", got)
			}
		})
	}
}

// TestNodeKustomization_DependsOnNodeWithoutKustomization: a DependsOn target
// that has no Kustomization of its own is refused, naming both nodes.
func TestNodeKustomization_DependsOnNodeWithoutKustomization(t *testing.T) {
	for _, tc := range []struct {
		name   string
		target func(c *stack.Cluster, platform *stack.Node) *stack.Node
		want   []string
	}{
		{
			name:   "target renders a bundle",
			target: func(_ *stack.Cluster, platform *stack.Node) *stack.Node { return platform.Children[0] },
			want:   []string{`"prod/apps"`, `"prod/platform/cert-manager"`, `bundle "cert-manager"`},
		},
		{
			name:   "target is the root of the tree",
			target: func(c *stack.Cluster, _ *stack.Node) *stack.Node { return c.Node },
			want:   []string{`"prod/apps"`, `"prod"`, "root"},
		},
		{
			name:   "target is not in the cluster",
			target: func(*stack.Cluster, *stack.Node) *stack.Node { return &stack.Node{Name: "elsewhere"} },
			want:   []string{`"prod/apps"`, `"elsewhere"`, "not a node of this cluster"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, platform, apps := groupsCluster()
			apps.DependsOn = []*stack.Node{tc.target(c, platform)}
			_, err := integrateCluster(c, perLayoutNodeOnly())
			mustContainAll(t, err, tc.want...)
		})
	}
}

// TestNodeKustomization_NameCollidesWithBundle: a node's Kustomization name
// takes part in the same uniqueness check as a bundle's; the error names the
// node and the bundle.
func TestNodeKustomization_NameCollidesWithBundle(t *testing.T) {
	c, _, apps := groupsCluster()
	apps.KustomizationName = "shop"
	_, err := integrateCluster(c, perLayoutNodeOnly())
	mustContainAll(t, err, `"shop"`, `node "prod/apps"`, `bundle "shop"`, "unique")

	// The bundle's name in effect is what counts.
	c, _, apps = groupsCluster()
	apps.KustomizationName = "apps-shop"
	apps.Children[0].Bundle.KustomizationName = "apps-shop"
	_, err = integrateCluster(c, perLayoutNodeOnly())
	mustContainAll(t, err, `"apps-shop"`, `node "prod/apps"`, `bundle "shop"`)

	// Two nodes with one name.
	c, platform, apps := groupsCluster()
	platform.KustomizationName = "group"
	apps.KustomizationName = "group"
	_, err = integrateCluster(c, perLayoutNodeOnly())
	mustContainAll(t, err, `"group"`, `node "prod/platform"`, `node "prod/apps"`)
}

// TestNodeKustomization_Cycle: two nodes that depend on each other are
// refused by the reconcile-order check, like any other dependsOn cycle.
func TestNodeKustomization_Cycle(t *testing.T) {
	c, platform, apps := groupsCluster()
	platform.DependsOn = []*stack.Node{apps}
	apps.DependsOn = []*stack.Node{platform}
	_, err := integrateCluster(c, perLayoutNodeOnly())
	mustContainAll(t, err, "prod-apps-node", "prod-platform-node", "waits for itself")
}

// nodeFieldSetters sets each of the three node fields in turn.
var nodeFieldSetters = map[string]func(n, other *stack.Node){
	"KustomizationName": func(n, _ *stack.Node) { n.KustomizationName = "group" },
	"DependsOn":         func(n, other *stack.Node) { n.DependsOn = []*stack.Node{other} },
	"NamedDependsOn":    func(n, _ *stack.Node) { n.NamedDependsOn = []string{"external"} },
}

// TestNodeKustomization_FieldsRefusedWithoutNodeKustomization: the fields are
// refused, not ignored, on a node that gets no Kustomization of its own. Each
// error names the node.
func TestNodeKustomization_FieldsRefusedWithoutNodeKustomization(t *testing.T) {
	for field, set := range nodeFieldSetters {
		t.Run(field, func(t *testing.T) {
			t.Run("other placement", func(t *testing.T) {
				for _, p := range []layout.FluxPlacement{layout.FluxIntegratedPerBundle, layout.FluxSeparate} {
					c, platform, apps := groupsCluster()
					set(apps, platform)
					_, err := integrateCluster(c, placed(propertyGroupings["nodeOnly"], p))
					mustContainAll(t, err, `node "prod/apps"`, field, string(layout.FluxIntegratedPerLayout))
				}
			})
			t.Run("generation without placement", func(t *testing.T) {
				c, platform, apps := groupsCluster()
				set(apps, platform)
				_, err := fluxstack.NewResourceGenerator().GenerateFromCluster(c, layout.DefaultLayoutRules())
				mustContainAll(t, err, `node "prod/apps"`, field, string(layout.FluxIntegratedPerLayout))

				c, platform, apps = groupsCluster()
				set(apps, platform)
				// a per-layout tree is refused before the node's fields are read
				ml := mustWalk(t, c, placed(propertyGroupings["nodeOnly"], layout.FluxSeparate))
				_, err = fluxstack.NewResourceGenerator().GenerateFromLayout(ml, c)
				mustContainAll(t, err, `node "prod/apps"`, field)
			})
			t.Run("node merged away by flat node grouping", func(t *testing.T) {
				c, platform, apps := groupsCluster()
				set(apps, platform)
				_, err := integrateCluster(c, placed(propertyGroupings["nodeFlat"], layout.FluxIntegratedPerLayout))
				mustContainAll(t, err, `node "prod/apps"`, field, "NodeGrouping")
			})
			t.Run("node whose layout renders a bundle", func(t *testing.T) {
				c, platform, apps := groupsCluster()
				shop := apps.Children[0]
				set(shop, platform)
				_, err := integrateCluster(c, perLayoutNodeOnly())
				mustContainAll(t, err, `node "prod/apps/shop"`, field, `bundle "shop"`)
			})
			t.Run("root of the tree", func(t *testing.T) {
				c, platform, _ := groupsCluster()
				set(c.Node, platform)
				_, err := integrateCluster(c, perLayoutNodeOnly())
				mustContainAll(t, err, `node "prod"`, field, "root")
			})
			t.Run("node layout a caller marked as an umbrella child", func(t *testing.T) {
				c, platform, apps := groupsCluster()
				set(apps, platform)
				rules := perLayoutNodeOnly()
				ml := mustWalk(t, c, rules)
				layoutAtPath(t, ml, "prod/apps").UmbrellaChild = true
				err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
				mustContainAll(t, err, `node "prod/apps"`, field, "UmbrellaChild")
			})
		})
	}
}

// TestNodeKustomization_RootBelowClusterDirectory: with a cluster directory
// above it the root node is not the top of the tree. Its directory is applied
// by a Kustomization of its own, hosted in the cluster directory, so the
// fields apply to it; whether a root has one depends on the layout, not on
// the node.
func TestNodeKustomization_RootBelowClusterDirectory(t *testing.T) {
	c, _, _ := groupsCluster()
	c.Node.KustomizationName = "prod-root"
	c.Node.NamedDependsOn = []string{"external"}
	rules := perLayoutNodeOnly()
	rules.ClusterName = "clusters"

	ml := integrated(t, c, rules)

	k := kustNamed(t, ml, "prod-root")
	if k.Spec.Path != "clusters/prod" {
		t.Errorf("spec.path = %q, want clusters/prod", k.Spec.Path)
	}
	if got := dependsOnInOrder(k); !slices.Equal(got, []string{"external"}) {
		t.Errorf("dependsOn = %v, want [external]", got)
	}
}

// TestNodeKustomization_BundleGroupByName: a node above its bundle's own
// directory has a Kustomization of its own, so the fields apply to it.
func TestNodeKustomization_BundleGroupByName(t *testing.T) {
	c, platform, apps := groupsCluster()
	shop := apps.Children[0]
	shop.KustomizationName = "shop-group"
	shop.DependsOn = []*stack.Node{platform}

	ml := integrated(t, c, placed(propertyGroupings["GroupByName"], layout.FluxIntegratedPerLayout))

	k := kustNamed(t, ml, "shop-group")
	if k.Spec.Path != "prod/apps/shop" {
		t.Errorf("spec.path = %q, want prod/apps/shop", k.Spec.Path)
	}
	if got := dependsOnInOrder(k); !slices.Equal(got, []string{"prod-platform-node"}) {
		t.Errorf("dependsOn = %v, want [prod-platform-node]", got)
	}
	if got := kustNamed(t, ml, "shop").Spec.Path; got != "prod/apps/shop/shop" {
		t.Errorf("bundle Kustomization spec.path = %q, want prod/apps/shop/shop", got)
	}
}

// TestLayoutKustomization_NamedAfterUnit: under FluxIntegratedPerLayout the
// Kustomization of an application or augmenter layout is named
// <unit name>-<layout name>, and a dependsOn entry that names a sibling layout
// is written as that sibling's Kustomization name.
func TestLayoutKustomization_NamedAfterUnit(t *testing.T) {
	root, unit, app, pre, hooks, c := buildAugmenterTestTree(t, layout.FluxIntegratedPerLayout, augmenterSR())
	li := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	if err := li.IntegrateWithLayout(root, c, layout.LayoutRules{FluxPlacement: layout.FluxIntegratedPerLayout}); err != nil {
		t.Fatalf("IntegrateWithLayout: %v", err)
	}
	want := map[string]string{
		"apps":                      unit.FullRepoPath(),
		"apps-myapp":                app.FullRepoPath(),
		"apps-myapp-00-pre-install": pre.FullRepoPath(),
		"apps-myapp-01-hooks":       hooks.FullRepoPath(),
	}
	if got := kustPaths(root); !mapsEqual(got, want) {
		t.Errorf("Kustomizations = %v, want %v", got, want)
	}
	if got := dependsOnInOrder(kustNamed(t, root, "apps-myapp-01-hooks")); !slices.Equal(got, []string{"apps-myapp-00-pre-install"}) {
		t.Errorf("hooks dependsOn = %v, want [apps-myapp-00-pre-install]", got)
	}
	writeAll(t, root)
}

// TestLayoutKustomization_UnitNameInEffect: the unit name is the bundle's
// Kustomization name in effect.
func TestLayoutKustomization_UnitNameInEffect(t *testing.T) {
	root, _, _, _, _, c := buildAugmenterTestTree(t, layout.FluxIntegratedPerLayout, augmenterSR())
	c.Node.Bundle.KustomizationName = "prod-apps"
	li := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	if err := li.IntegrateWithLayout(root, c, layout.LayoutRules{FluxPlacement: layout.FluxIntegratedPerLayout}); err != nil {
		t.Fatalf("IntegrateWithLayout: %v", err)
	}
	for _, name := range []string{"prod-apps", "prod-apps-myapp", "prod-apps-myapp-00-pre-install", "prod-apps-myapp-01-hooks"} {
		kustNamed(t, root, name)
	}
}

// TestLayoutKustomization_ApplicationNamedLikeItsBundle: an application named
// like its bundle used to give two Kustomizations one name; the application
// layout's is now named after the unit as well.
func TestLayoutKustomization_ApplicationNamedLikeItsBundle(t *testing.T) {
	c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "prod", Bundle: srBundle("web", cmApp("web"))}}
	rules := placed(layout.LayoutRules{BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupByName}, layout.FluxIntegratedPerLayout)
	ml := integrated(t, c, rules)
	want := map[string]string{"web": "prod/web", "web-web": "prod/web/web"}
	if got := kustPaths(ml); !mapsEqual(got, want) {
		t.Errorf("Kustomizations = %v, want %v", got, want)
	}
	writeAll(t, ml)
}

// namedHookAugmenter is hookGroupAugmenter with a Kustomization name set on
// its first hook group.
type namedHookAugmenter struct{}

func (namedHookAugmenter) Generate(*stack.Application) ([]*client.Object, error) { return nil, nil }

func (namedHookAugmenter) AugmentLayout(ml *layout.ManifestLayout) error {
	pre := &layout.ManifestLayout{
		Name:              "00-pre-install",
		Namespace:         ml.FullRepoPath(),
		FluxPlacement:     ml.FluxPlacement,
		KustomizationName: "myapp-pre",
		Resources:         []client.Object{*cmObj(ml.Name + "-pre")},
	}
	hooks := &layout.ManifestLayout{
		Name:          "01-hooks",
		Namespace:     ml.FullRepoPath(),
		FluxPlacement: ml.FluxPlacement,
		DependsOn:     []string{pre.Name, "external"},
		Resources:     []client.Object{*cmObj(ml.Name + "-hook")},
	}
	ml.Children = append(ml.Children, pre, hooks)
	return nil
}

// TestLayoutKustomization_NameSetOnTheLayout: a name set on a layout is used
// instead of the default, whether an augmenter sets it on a layout it creates
// or a caller sets it on a walked layout before integration. A sibling's
// dependsOn follows it; an entry that names no sibling is written as given.
func TestLayoutKustomization_NameSetOnTheLayout(t *testing.T) {
	bundle := &stack.Bundle{Name: "apps", SourceRef: augmenterSR(), Applications: []*stack.Application{
		stack.NewApplication("myapp", "default", namedHookAugmenter{}),
	}}
	c := &stack.Cluster{Name: "prod", Node: &stack.Node{Name: "prod", Bundle: bundle}}
	rules := layout.LayoutRules{FluxPlacement: layout.FluxIntegratedPerLayout}
	root := mustWalk(t, c, rules)
	// root is the node's layout, its one child the bundle's directory, and
	// the application's layout is below that.
	app := root.Children[0].Children[0]
	app.KustomizationName = "the-app"

	if err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(root, c, rules); err != nil {
		t.Fatalf("IntegrateWithLayout: %v", err)
	}
	want := map[string]string{
		"apps":          "prod/apps",
		"the-app":       "prod/apps/myapp",
		"myapp-pre":     "prod/apps/myapp/00-pre-install",
		"apps-01-hooks": "prod/apps/myapp/01-hooks",
	}
	if got := kustPaths(root); !mapsEqual(got, want) {
		t.Errorf("Kustomizations = %v, want %v", got, want)
	}
	if got := dependsOnInOrder(kustNamed(t, root, "apps-01-hooks")); !slices.Equal(got, []string{"myapp-pre", "external"}) {
		t.Errorf("hooks dependsOn = %v, want [myapp-pre external]", got)
	}
	writeAll(t, root)
}

// TestLayoutKustomization_CallerPlacedUnderTheOldName: a Kustomization a
// caller placed for an application layout under the name the layout's had
// before the rename is not recognised under the new default, so the tree gets
// a second one for the same directory. Naming the layout's Kustomization like
// the placed one keeps that one and generates none.
func TestLayoutKustomization_CallerPlacedUnderTheOldName(t *testing.T) {
	integrateWith := func(t *testing.T, name string) (root *layout.ManifestLayout, placed *kustv1.Kustomization) {
		t.Helper()
		bundle := &stack.Bundle{Name: "apps", SourceRef: augmenterSR(), Applications: []*stack.Application{
			stack.NewApplication("myapp", "default", namedHookAugmenter{}),
		}}
		c := &stack.Cluster{Name: "prod", Node: &stack.Node{Name: "prod", Bundle: bundle}}
		rules := layout.LayoutRules{FluxPlacement: layout.FluxIntegratedPerLayout}
		root = mustWalk(t, c, rules)
		// The root node's bundle renders in its own directory, which hosts
		// the Kustomization of the application layout below it.
		unit := root.Children[0]
		app := unit.Children[0]
		app.KustomizationName = name
		placed = &kustv1.Kustomization{}
		placed.Name = "myapp"
		placed.Namespace = "flux-system"
		placed.Spec.Path = app.FullRepoPath()
		unit.Resources = append(unit.Resources, placed)
		if err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(root, c, rules); err != nil {
			t.Fatalf("IntegrateWithLayout: %v", err)
		}
		return root, placed
	}
	applying := func(root *layout.ManifestLayout, path string) []string {
		var names []string
		for _, k := range kustomizations(root) {
			if k.Spec.Path == path {
				names = append(names, k.Name)
			}
		}
		slices.Sort(names)
		return names
	}

	t.Run("default name", func(t *testing.T) {
		root, _ := integrateWith(t, "")
		if got := applying(root, "prod/apps/myapp"); !slices.Equal(got, []string{"apps-myapp", "myapp"}) {
			t.Errorf("Kustomizations applying prod/apps/myapp = %v, want [apps-myapp myapp]", got)
		}
	})
	t.Run("old name set on the layout", func(t *testing.T) {
		root, placed := integrateWith(t, "myapp")
		if got := applying(root, "prod/apps/myapp"); !slices.Equal(got, []string{"myapp"}) {
			t.Errorf("Kustomizations applying prod/apps/myapp = %v, want [myapp]", got)
		}
		if kustNamed(t, root, "myapp") != placed {
			t.Error("the caller's Kustomization was replaced")
		}
	})
}

// augmentWith is an application config that wants its own layout and lets fn
// add child layouts to it. It renders no resources of its own.
type augmentWith struct {
	fn func(ml *layout.ManifestLayout)
}

func (augmentWith) Generate(*stack.Application) ([]*client.Object, error) { return nil, nil }

func (a augmentWith) AugmentLayout(ml *layout.ManifestLayout) error {
	if a.fn != nil {
		a.fn(ml)
	}
	return nil
}

// hookGroup is a child layout of parent, as an augmenter adds one.
func hookGroup(parent *layout.ManifestLayout, name string, dependsOn ...string) *layout.ManifestLayout {
	return &layout.ManifestLayout{
		Name:          name,
		Namespace:     parent.FullRepoPath(),
		FluxPlacement: parent.FluxPlacement,
		DependsOn:     dependsOn,
		Resources:     []client.Object{*cmObj(parent.Name + "-" + name)},
	}
}

// augmentedCluster is one node "prod" whose bundle "apps" holds the given
// applications, each with a layout of its own.
func augmentedCluster(apps map[string]func(ml *layout.ManifestLayout)) *stack.Cluster {
	bundle := &stack.Bundle{Name: "apps", SourceRef: augmenterSR()}
	for _, name := range slices.Sorted(maps.Keys(apps)) {
		bundle.Applications = append(bundle.Applications, stack.NewApplication(name, "default", augmentWith{fn: apps[name]}))
	}
	return &stack.Cluster{Name: "prod", Node: &stack.Node{Name: "prod", Bundle: bundle}}
}

// TestLayoutKustomization_DependsOnLayoutOfTheSameUnit: a DependsOn entry is
// a layout name, and no layout's Kustomization is named like the layout any
// more. An entry that names a layout of the same unit is written as that
// layout's Kustomization name, whether it is a sibling, the parent or a
// layout elsewhere in the unit; a name that is no such layout's is kept.
func TestLayoutKustomization_DependsOnLayoutOfTheSameUnit(t *testing.T) {
	c := augmentedCluster(map[string]func(ml *layout.ManifestLayout){
		"a": func(ml *layout.ManifestLayout) {
			ml.Children = append(ml.Children,
				hookGroup(ml, "pre"),
				hookGroup(ml, "post", "pre", "a", "b", "external"))
		},
		"b": nil,
	})
	ml := integrated(t, c, layout.LayoutRules{FluxPlacement: layout.FluxIntegratedPerLayout})
	want := []string{"apps-pre", "apps-a", "apps-b", "external"}
	if got := dependsOnInOrder(kustNamed(t, ml, "apps-post")); !slices.Equal(got, want) {
		t.Errorf("post dependsOn = %v, want %v", got, want)
	}
	writeAll(t, ml)
}

// TestLayoutKustomization_DependsOnAmbiguousLayoutName: two layouts of one
// unit may share a Name once one of them sets KustomizationName. A sibling
// with that name is the one meant; without a sibling the entry does not say
// which, and is refused naming the layout, the entry and two candidates.
func TestLayoutKustomization_DependsOnAmbiguousLayoutName(t *testing.T) {
	build := func(lateDependsOn ...string) *stack.Cluster {
		return augmentedCluster(map[string]func(ml *layout.ManifestLayout){
			"a": func(ml *layout.ManifestLayout) {
				ml.Children = append(ml.Children, hookGroup(ml, "hooks"), hookGroup(ml, "post", "hooks"))
			},
			"b": func(ml *layout.ManifestLayout) {
				hooks := hookGroup(ml, "hooks")
				hooks.KustomizationName = "b-hooks"
				ml.Children = append(ml.Children, hooks)
			},
			"c": func(ml *layout.ManifestLayout) {
				ml.Children = append(ml.Children, hookGroup(ml, "late", lateDependsOn...))
			},
		})
	}
	rules := layout.LayoutRules{FluxPlacement: layout.FluxIntegratedPerLayout}

	ml := integrated(t, build(), rules)
	if got := dependsOnInOrder(kustNamed(t, ml, "apps-post")); !slices.Equal(got, []string{"apps-hooks"}) {
		t.Errorf("post dependsOn = %v, want [apps-hooks], its sibling", got)
	}

	_, err := integrateCluster(build("hooks"), rules)
	mustContainAll(t, err, `layout "prod/apps/c/late"`, `"hooks"`, `"prod/apps/a/hooks"`, `"prod/apps/b/hooks"`, "KustomizationName")
}

// TestNodeKustomization_IntegrationIsRepeatable: integrating a tree a second
// time adds nothing and refuses nothing.
func TestNodeKustomization_IntegrationIsRepeatable(t *testing.T) {
	c, platform, apps := groupsCluster()
	apps.KustomizationName = "apps"
	apps.DependsOn = []*stack.Node{platform}
	rules := perLayoutNodeOnly()
	ml := integrated(t, c, rules)
	before := kustPaths(ml)
	if err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules); err != nil {
		t.Fatalf("second IntegrateWithLayout: %v", err)
	}
	if got := kustPaths(ml); !mapsEqual(got, before) {
		t.Errorf("Kustomizations after a second integration = %v, want %v", got, before)
	}
	if n := len(kustomizations(ml)); n != len(before) {
		t.Errorf("%d Kustomizations after a second integration, want %d", n, len(before))
	}
}

// TestNodeKustomization_KeptKustomizationStillChecked: a second integration
// keeps the node Kustomization the first one placed. What the node asks for
// is checked all the same: a dependency on a node without a Kustomization is
// refused, and so is a dependency the kept Kustomization does not carry,
// which would otherwise be dropped without a word.
func TestNodeKustomization_KeptKustomizationStillChecked(t *testing.T) {
	again := func(t *testing.T, change func(c *stack.Cluster, platform, apps *stack.Node)) error {
		t.Helper()
		c, platform, apps := groupsCluster()
		apps.KustomizationName = "apps"
		rules := perLayoutNodeOnly()
		ml := integrated(t, c, rules)
		change(c, platform, apps)
		return fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
	}

	t.Run("target without a Kustomization", func(t *testing.T) {
		err := again(t, func(c *stack.Cluster, _, apps *stack.Node) {
			apps.DependsOn = []*stack.Node{c.Node}
		})
		mustContainAll(t, err, `node "prod/apps"`, `node "prod"`, "no Flux Kustomization of its own")
	})
	t.Run("dependency the kept Kustomization lacks", func(t *testing.T) {
		err := again(t, func(_ *stack.Cluster, platform, apps *stack.Node) {
			apps.DependsOn = []*stack.Node{platform}
		})
		mustContainAll(t, err, `node "prod/apps"`, `"apps"`, "prod-platform-node", "already has")
	})
	t.Run("nothing changed", func(t *testing.T) {
		if err := again(t, func(*stack.Cluster, *stack.Node, *stack.Node) {}); err != nil {
			t.Fatalf("second integration of an unchanged model: %v", err)
		}
	})

	// kept integrates a cluster whose apps group waits for the platform
	// group and for "external", lets edit change the Kustomization that
	// integration placed for apps, and integrates the same tree again.
	kept := func(t *testing.T, edit func(k *kustv1.Kustomization)) error {
		t.Helper()
		c, platform, apps := groupsCluster()
		apps.KustomizationName = "apps"
		apps.DependsOn = []*stack.Node{platform}
		apps.NamedDependsOn = []string{"external"}
		rules := perLayoutNodeOnly()
		ml := integrated(t, c, rules)
		edit(kustNamed(t, ml, "apps"))
		return fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
	}
	t.Run("kept dependency names another namespace", func(t *testing.T) {
		err := kept(t, func(k *kustv1.Kustomization) {
			k.Spec.DependsOn[0].Namespace = "other"
		})
		mustContainAll(t, err, `node "prod/apps"`, "prod-platform-node", "other/prod-platform-node", "already has")
	})
	t.Run("kept dependencies in another order, with one more", func(t *testing.T) {
		err := kept(t, func(k *kustv1.Kustomization) {
			slices.Reverse(k.Spec.DependsOn)
			k.Spec.DependsOn = append(k.Spec.DependsOn, kustv1.DependencyReference{Name: "extra"})
			k.Spec.DependsOn[0].Namespace = k.Namespace
		})
		if err != nil {
			t.Fatalf("a kept Kustomization that carries every dependency the node asks for was refused: %v", err)
		}
	})
}

// TestNodeKustomization_KeptKustomizationOfNodeWithoutFields: a node that
// sets none of the fields is not held to the check. A caller who edited the
// walked node layout's DependsOn and placed a Kustomization of their own had
// that tree accepted before the node fields existed, and still has.
func TestNodeKustomization_KeptKustomizationOfNodeWithoutFields(t *testing.T) {
	c, _, _ := groupsCluster()
	rules := perLayoutNodeOnly()
	ml := integrated(t, c, rules)
	layoutAtPath(t, ml, "prod/apps").DependsOn = []string{"external-a", "external-b"}
	kustNamed(t, ml, "prod-apps-node").Spec.DependsOn = []kustv1.DependencyReference{{Name: "external-b"}}

	if err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules); err != nil {
		t.Fatalf("second integration: %v", err)
	}
}

// TestNodeKustomization_NodeChangedAfterTheWalk: the walker copies a node's
// KustomizationName and NamedDependsOn onto the node's layout, and the
// Kustomization is built from the layout. A name or an entry the node gets
// after the walk is not on the layout: it is refused, naming the node, and
// not ignored, on a first integration and on a second one alike. Carried by
// the layout, it is accepted and written.
func TestNodeKustomization_NodeChangedAfterTheWalk(t *testing.T) {
	rules := perLayoutNodeOnly()
	integrate := func(ml *layout.ManifestLayout, c *stack.Cluster) error {
		return fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
	}
	walked := func(t *testing.T, c *stack.Cluster) *layout.ManifestLayout {
		t.Helper()
		ml, err := layout.WalkCluster(c, rules)
		if err != nil {
			t.Fatalf("WalkCluster: %v", err)
		}
		return ml
	}

	t.Run("named dependency added before the first integration", func(t *testing.T) {
		c, _, apps := groupsCluster()
		ml := walked(t, c)
		apps.NamedDependsOn = []string{"prod-platform-node"}
		mustContainAll(t, integrate(ml, c), `node "prod/apps"`, `"prod-platform-node"`, `layout "prod/apps"`, "after the layout was walked")

		layoutAtPath(t, ml, "prod/apps").DependsOn = []string{"prod-platform-node"}
		if err := integrate(ml, c); err != nil {
			t.Fatalf("the layout carries the entry: %v", err)
		}
		if got := dependsOnInOrder(kustNamed(t, ml, "prod-apps-node")); !slices.Equal(got, []string{"prod-platform-node"}) {
			t.Errorf("dependsOn = %v, want [prod-platform-node]", got)
		}
	})
	t.Run("named dependency added before a second integration", func(t *testing.T) {
		c, _, apps := groupsCluster()
		ml := integrated(t, c, rules)
		apps.NamedDependsOn = []string{"prod-platform-node"}
		mustContainAll(t, integrate(ml, c), `node "prod/apps"`, `"prod-platform-node"`, "after the layout was walked")
	})
	t.Run("name set before the first integration", func(t *testing.T) {
		c, _, apps := groupsCluster()
		ml := walked(t, c)
		apps.KustomizationName = "apps"
		mustContainAll(t, integrate(ml, c), `node "prod/apps"`, `"apps"`, `layout "prod/apps"`, "after the layout was walked")

		layoutAtPath(t, ml, "prod/apps").KustomizationName = "apps"
		if err := integrate(ml, c); err != nil {
			t.Fatalf("the layout carries the name: %v", err)
		}
		if got := kustNamed(t, ml, "apps").Spec.Path; got != "prod/apps" {
			t.Errorf("Kustomization apps has spec.path %q, want prod/apps", got)
		}
	})
	t.Run("name set before a second integration", func(t *testing.T) {
		c, platform, _ := groupsCluster()
		ml := integrated(t, c, rules)
		platform.KustomizationName = "platform"
		mustContainAll(t, integrate(ml, c), `node "prod/platform"`, `"platform"`, "after the layout was walked")
	})
	// The walk refuses an empty or a repeated entry (ValidateCluster); the
	// integration does not validate the cluster again, so it refuses one a
	// node got afterwards itself, also when the layout was given it too.
	t.Run("empty named dependency after the walk", func(t *testing.T) {
		c, _, apps := groupsCluster()
		ml := walked(t, c)
		apps.NamedDependsOn = []string{""}
		layoutAtPath(t, ml, "prod/apps").DependsOn = []string{""}
		mustContainAll(t, integrate(ml, c), `node "prod/apps"`, "NamedDependsOn holds an empty name")
	})
	t.Run("repeated named dependency after the walk", func(t *testing.T) {
		c, _, apps := groupsCluster()
		ml := walked(t, c)
		apps.NamedDependsOn = []string{"external", "external"}
		layoutAtPath(t, ml, "prod/apps").DependsOn = []string{"external", "external"}
		mustContainAll(t, integrate(ml, c), `node "prod/apps"`, `NamedDependsOn lists "external" twice`)
	})
	t.Run("layout named otherwise than its node", func(t *testing.T) {
		c, _, apps := groupsCluster()
		apps.KustomizationName = "apps"
		ml := walked(t, c)
		layoutAtPath(t, ml, "prod/apps").KustomizationName = "applications"
		if err := integrate(ml, c); err != nil {
			t.Fatalf("a caller named the walked layout: %v", err)
		}
		kustNamed(t, ml, "applications")
	})
}
