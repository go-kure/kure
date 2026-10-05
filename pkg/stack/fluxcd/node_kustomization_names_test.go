package fluxcd_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for the name rule of a per-layout Kustomization (go-kure/kure#973):
// the name in effect is checked where the Kustomization is created, and the
// refusal names what the caller sets to change it, a node's field or a
// layout's. kure shortens nothing.

// TestNodeKustomization_DerivedNameOverTheLimit: a node whose derived
// "<path>-node" name is over the limit is refused, naming the node and the
// field to set. Setting the field renders the same cluster.
func TestNodeKustomization_DerivedNameOverTheLimit(t *testing.T) {
	long := strings.Repeat("g", stack.KustomizationNameMaxLength)
	build := func(name string) *stack.Cluster {
		group := &stack.Node{Name: long, KustomizationName: name, Children: []*stack.Node{
			{Name: "shop", Bundle: srBundle("shop", cmApp("shop-app"))},
		}}
		return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "prod", Children: []*stack.Node{group}}}
	}

	_, err := integrateCluster(build(""), perLayoutNodeOnly())
	mustContainAll(t, err,
		"Node 'prod/"+long+"'", "field 'kustomizationName'",
		`"prod-`+long+`-node"`, "at most 63 characters", "set Node.KustomizationName")

	ml := integrated(t, build("group"), perLayoutNodeOnly())
	if got := kustPaths(ml)["group"]; got != "prod/"+long {
		t.Errorf("Kustomization group applies %q, want the node's directory prod/%s", got, long)
	}
	writeAll(t, ml)
}

// TestNodeKustomization_NameValueRefused: a node's KustomizationName that
// Flux cannot reconcile is refused, naming the node and the field: one that
// is no DNS-1123 subdomain, and one over the limit.
func TestNodeKustomization_NameValueRefused(t *testing.T) {
	for name, reason := range map[string]string{
		"Apps_Group": "is not a valid Flux Kustomization name",
		strings.Repeat("a", stack.KustomizationNameMaxLength+1): "at most 63 characters",
	} {
		t.Run(reason, func(t *testing.T) {
			c, _, apps := groupsCluster()
			apps.KustomizationName = name
			_, err := integrateCluster(c, perLayoutNodeOnly())
			mustContainAll(t, err, "Node 'prod/apps'", "field 'kustomizationName'", `"`+name+`"`, reason)
			if strings.Contains(err.Error(), "ManifestLayout") {
				t.Errorf("error names a layout for a name the node set:\n%v", err)
			}
		})
	}
}

// longNameCluster is a root node whose bundle "web" has one application with
// its own directory, named so that the default "<unit name>-<layout name>" is
// one character over the limit while the directory name itself is within it.
func longNameCluster() (c *stack.Cluster, app string, rules layout.LayoutRules) {
	app = strings.Repeat("a", stack.KustomizationNameMaxLength-len("web-")+1)
	c = &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "prod", Bundle: srBundle("web", cmApp(app))}}
	rules = placed(layout.LayoutRules{BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupByName}, layout.FluxIntegratedPerLayout)
	return c, app, rules
}

// TestLayoutKustomization_DefaultOverTheLimit: the default name of an
// application layout's Kustomization is longer than the layout's name by the
// unit name, and one over the limit is refused, naming the layout and the
// field to set. Setting the field on the walked layout renders the same tree.
func TestLayoutKustomization_DefaultOverTheLimit(t *testing.T) {
	c, app, rules := longNameCluster()
	_, err := integrateCluster(c, rules)
	mustContainAll(t, err,
		"ManifestLayout 'prod/web/"+app+"'", "field 'kustomizationName'",
		`"web-`+app+`"`, "at most 63 characters", "set ManifestLayout.KustomizationName")

	c, app, rules = longNameCluster()
	root := mustWalk(t, c, rules)
	root.Children[0].Children[0].KustomizationName = "web-app"
	if err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(root, c, rules); err != nil {
		t.Fatalf("IntegrateWithLayout with the name set: %v", err)
	}
	if got := kustPaths(root)["web-app"]; got != "prod/web/"+app {
		t.Errorf("Kustomization web-app applies %q, want the application's directory prod/web/%s", got, app)
	}
	writeAll(t, root)
}

// TestLayoutKustomization_NameValueRefused: a KustomizationName set on a
// layout that Flux cannot reconcile is refused, naming the layout and the
// field, on an application layout and on a node's own layout whose name a
// caller replaced after the walk.
func TestLayoutKustomization_NameValueRefused(t *testing.T) {
	li := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())

	t.Run("application layout", func(t *testing.T) {
		c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "prod", Bundle: srBundle("web", cmApp("api"))}}
		rules := placed(layout.LayoutRules{BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupByName}, layout.FluxIntegratedPerLayout)
		root := mustWalk(t, c, rules)
		root.Children[0].Children[0].KustomizationName = "Web_API"
		mustContainAll(t, li.IntegrateWithLayout(root, c, rules),
			"ManifestLayout 'prod/web/api'", "field 'kustomizationName'", `"Web_API" is not a valid Flux Kustomization name`)
	})

	t.Run("node layout renamed after the walk", func(t *testing.T) {
		c, _, apps := groupsCluster()
		apps.KustomizationName = "apps"
		root := mustWalk(t, c, perLayoutNodeOnly())
		var renamed bool
		for _, child := range root.Children {
			if child.KustomizationName == "apps" {
				child.KustomizationName = "Apps_Group"
				renamed = true
			}
		}
		if !renamed {
			t.Fatal("no layout carries the node's KustomizationName")
		}
		mustContainAll(t, li.IntegrateWithLayout(root, c, perLayoutNodeOnly()),
			"ManifestLayout 'prod/apps'", "field 'kustomizationName'", `"Apps_Group" is not a valid Flux Kustomization name`)
	})
}

// TestNodeKustomization_NamedDependsOnEntries: an entry of a node's
// NamedDependsOn is a caller-supplied reference, written as given whatever
// its value, like a bundle's. What is refused is what is refused for a
// bundle: an empty entry, a repeated one, and one that names the
// Kustomization of a node in DependsOn.
func TestNodeKustomization_NamedDependsOnEntries(t *testing.T) {
	t.Run("value written as given", func(t *testing.T) {
		long := strings.Repeat("x", stack.KustomizationNameMaxLength+1)
		c, _, apps := groupsCluster()
		apps.NamedDependsOn = []string{"Other_Cluster", long}
		ml := integrated(t, c, perLayoutNodeOnly())
		if got := dependsOnInOrder(kustNamed(t, ml, "prod-apps-node")); !slices.Equal(got, []string{"Other_Cluster", long}) {
			t.Errorf("dependsOn = %v, want the two entries as given", got)
		}
	})
	t.Run("empty entry", func(t *testing.T) {
		c, _, apps := groupsCluster()
		apps.NamedDependsOn = []string{"external", ""}
		_, err := integrateCluster(c, perLayoutNodeOnly())
		mustContainAll(t, err, `node "prod/apps"`, "NamedDependsOn holds an empty name")
	})
	t.Run("repeated entry", func(t *testing.T) {
		c, _, apps := groupsCluster()
		apps.NamedDependsOn = []string{"external", "external"}
		_, err := integrateCluster(c, perLayoutNodeOnly())
		mustContainAll(t, err, `node "prod/apps"`, `NamedDependsOn lists "external" twice`)
	})
	t.Run("entry that names a DependsOn node's Kustomization", func(t *testing.T) {
		c, platform, apps := groupsCluster()
		apps.DependsOn = []*stack.Node{platform}
		apps.NamedDependsOn = []string{"prod-platform-node"}
		_, err := integrateCluster(c, perLayoutNodeOnly())
		mustContainAll(t, err, `node "prod/apps"`, `"prod-platform-node" appears in both DependsOn and NamedDependsOn`, `node "prod/platform"`)
	})
}
