package fluxcd_test

import (
	"crypto/sha256"
	"encoding/hex"
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
// layout's. kure shortens only the "<unit name>-<layout name>" default of an
// application or augmenter layout (go-kure/kure#1030, go-kure/kure#1036).

// nameHash is the hash part of a shortened default, computed apart from the
// code under test.
func nameHash(composed string) string {
	sum := sha256.Sum256([]byte(composed))
	return hex.EncodeToString(sum[:])[:8]
}

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
// unit name; one over the limit whose layout name has more than 54 characters
// is shortened to the layout name's first 54 and a hash (go-kure/kure#1036).
// Setting the field on the walked layout uses the name set instead.
func TestLayoutKustomization_DefaultOverTheLimit(t *testing.T) {
	c, app, rules := longNameCluster()
	short := strings.Repeat("a", 54) + "-" + nameHash("web-"+app)
	if got := kustPaths(integrated(t, c, rules))[short]; got != "prod/web/"+app {
		t.Errorf("Kustomization %s applies %q, want the application's directory prod/web/%s", short, got, app)
	}

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

// shortenedCluster is a root node whose bundle "platform-services-payments"
// has two applications with their own directories: app, whose default
// "<unit name>-<layout name>" is over the limit, and "api", whose default fits.
func shortenedCluster(app string) (*stack.Cluster, layout.LayoutRules) {
	c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "prod",
		Bundle: srBundle("platform-services-payments", cmApp(app), cmApp("api"))}}
	return c, placed(layout.LayoutRules{BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupByName}, layout.FluxIntegratedPerLayout)
}

// TestLayoutKustomization_DefaultShortened: a default over the limit is
// shortened (go-kure/kure#1030): the CR integrates under the shortened name,
// a sibling's DependsOn entry naming the layout is written as that name, and
// the fitting default beside it is unchanged.
func TestLayoutKustomization_DefaultShortened(t *testing.T) {
	const app = "checkout-service-payments-reconciler-worker"
	const short = "platform-s-7365aca5-" + app
	c, rules := shortenedCluster(app)
	root := mustWalk(t, c, rules)
	layoutAtPath(t, root, "prod/platform-services-payments/api").DependsOn = []string{app}
	if err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(root, c, rules); err != nil {
		t.Fatalf("IntegrateWithLayout: %v", err)
	}

	if got := kustNamed(t, root, short).Spec.Path; got != "prod/platform-services-payments/"+app {
		t.Errorf("Kustomization %s applies %q, want the application's directory", short, got)
	}
	api := kustNamed(t, root, "platform-services-payments-api")
	if got := dependsOnInOrder(api); !slices.Equal(got, []string{short}) {
		t.Errorf("dependsOn of platform-services-payments-api = %v, want the shortened name [%s]", got, short)
	}
	writeAll(t, root)
}

// TestLayoutKustomization_ShortenedNameCollision: two defaults that differ
// only in the shortened part and whose 8-character hashes collide (the units
// platform-services-65327 and platform-services-103659 beside the same layout
// name, both 826dabca) give the same name, and the duplicate-name check
// refuses the tree, naming both layouts.
func TestLayoutKustomization_ShortenedNameCollision(t *testing.T) {
	const app = "checkout-service-payments-reconciler-worker"
	const short = "platform-s-826dabca-" + app
	c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "prod", Children: []*stack.Node{
		{Name: "a", Bundle: srBundle("platform-services-65327", cmApp(app))},
		{Name: "b", Bundle: srBundle("platform-services-103659", cmApp(app))},
	}}}
	rules := placed(layout.LayoutRules{BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupByName}, layout.FluxIntegratedPerLayout)
	_, err := integrateCluster(c, rules)
	mustContainAll(t, err, `"`+short+`"`, `layout "prod/a/`, `layout "prod/b/`)
}

// TestLayoutKustomization_ShortenBoundary: a default over the limit is
// shortened where the layout name has 54 characters to "<hash>-<name>", and
// where it has 55 to "<first 54 of the name>-<hash>" (go-kure/kure#1036).
func TestLayoutKustomization_ShortenBoundary(t *testing.T) {
	t.Run("54", func(t *testing.T) {
		app := strings.Repeat("n", 54)
		c, rules := shortenedCluster(app)
		ml := integrated(t, c, rules)
		var found bool
		for name, path := range kustPaths(ml) {
			if path != "prod/platform-services-payments/"+app {
				continue
			}
			found = true
			if len(name) != stack.KustomizationNameMaxLength || !strings.HasSuffix(name, "-"+app) || strings.HasPrefix(name, "platform") {
				t.Errorf("Kustomization of the application is %q, want <hash>-%s, 63 characters", name, app)
			}
		}
		if !found {
			t.Fatalf("no Kustomization applies the application's directory; have %v", kustPaths(ml))
		}
	})
	t.Run("55", func(t *testing.T) {
		app := strings.Repeat("n", 55)
		c, rules := shortenedCluster(app)
		short := strings.Repeat("n", 54) + "-" + nameHash("platform-services-payments-"+app)
		if got := kustPaths(integrated(t, c, rules))[short]; got != "prod/platform-services-payments/"+app {
			t.Errorf("Kustomization %s applies %q, want the application's directory", short, got)
		}
	})
}

// TestDefaultLayoutKustomizationName_MatchesIntegrator: the integrator names
// an application layout's Kustomization exactly as the exported
// DefaultLayoutKustomizationName predicts from the bundle's UnitName and the
// layout's name (go-kure/kure#1040), for a default that fits and for each
// shortened shape, also where the bundle sets its own KustomizationName. The
// shape check keeps a row from passing on a default that did not take the
// form it is there for.
func TestDefaultLayoutKustomizationName_MatchesIntegrator(t *testing.T) {
	const name76 = "checkout-service-payments-reconciler-worker-pre-install-hooks-schema-migrate"
	for _, tc := range []struct {
		desc, bundle, kustName, app string
		shape                       func(got, unit, app string) bool
	}{
		{
			desc: "fits", bundle: "platform-services-payments", app: "api",
			shape: func(got, unit, app string) bool { return got == unit+"-"+app },
		},
		{
			desc: "<unit prefix>-<hash>-<name>", bundle: "platform-services-payments", app: "checkout-service-payments-reconciler-worker",
			shape: func(got, unit, app string) bool {
				return strings.HasPrefix(got, "platform-s-") && strings.HasSuffix(got, "-"+app) && got != unit+"-"+app
			},
		},
		{
			desc: "<hash>-<name>", bundle: "platform-services-payments", app: strings.Repeat("n", 54),
			shape: func(got, unit, app string) bool {
				return len(got) == stack.KustomizationNameMaxLength && strings.HasSuffix(got, "-"+app) && !strings.HasPrefix(got, "platform")
			},
		},
		{
			desc: "<name prefix>-<hash>", bundle: "platform-services-payments", app: name76,
			shape: func(got, unit, app string) bool {
				return strings.HasPrefix(got, app[:54]+"-") && len(got) == stack.KustomizationNameMaxLength
			},
		},
		{
			desc: "bundle with its own KustomizationName", bundle: "web", kustName: "platform-services-payments", app: "checkout-service-payments-reconciler-worker",
			shape: func(got, unit, app string) bool {
				return strings.HasPrefix(got, "platform-s-") && strings.HasSuffix(got, "-"+app)
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			b := srBundle(tc.bundle, cmApp(tc.app))
			b.KustomizationName = tc.kustName
			c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "prod", Bundle: b}}
			rules := placed(layout.LayoutRules{BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupByName}, layout.FluxIntegratedPerLayout)
			ml := integrated(t, c, rules)

			want := fluxstack.DefaultLayoutKustomizationName(b.UnitName(), tc.app)
			if !tc.shape(want, b.UnitName(), tc.app) {
				t.Fatalf("DefaultLayoutKustomizationName(%q, %q) = %q, not of the shape %s", b.UnitName(), tc.app, want, tc.desc)
			}
			var emitted []string
			for name, path := range kustPaths(ml) {
				if path == "prod/"+tc.bundle+"/"+tc.app {
					emitted = append(emitted, name)
				}
			}
			if len(emitted) != 1 || emitted[0] != want {
				t.Fatalf("Kustomizations applying prod/%s/%s: %v, want exactly [%s]; have %v", tc.bundle, tc.app, emitted, want, kustPaths(ml))
			}
			writeAll(t, ml)
		})
	}
}

// TestDefaultLayoutKustomizationName_MergedBundles: where NodeGrouping
// GroupFlat merges a child node's bundle into the root's directory, the unit
// of the child bundle's application is the first bundle's UnitName, so that
// is what DefaultLayoutKustomizationName takes to predict the name; the
// application's own bundle's UnitName gives another name, which is not
// written.
func TestDefaultLayoutKustomizationName_MergedBundles(t *testing.T) {
	const app = "checkout-service-payments-reconciler-worker"
	first := srBundle("platform-services-payments", cmApp("api"))
	second := srBundle("apps", cmApp(app))
	child := &stack.Node{Name: "apps", Bundle: second}
	root := &stack.Node{Name: "prod", Bundle: first, Children: []*stack.Node{child}}
	child.SetParent(root)
	rules := placed(layout.LayoutRules{NodeGrouping: layout.GroupFlat, BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupByName}, layout.FluxIntegratedPerLayout)
	ml := integrated(t, &stack.Cluster{Name: "demo", Node: root}, rules)

	want := fluxstack.DefaultLayoutKustomizationName(first.UnitName(), app)
	if want != "platform-s-7365aca5-"+app {
		t.Fatalf("DefaultLayoutKustomizationName(%q, %q) = %q, want the shortened platform-s-7365aca5-%s", first.UnitName(), app, want, app)
	}
	if got := kustNamed(t, ml, want).Spec.Path; !strings.HasSuffix(got, "/"+app) {
		t.Errorf("Kustomization %s applies %q, want the application's directory", want, got)
	}
	own := fluxstack.DefaultLayoutKustomizationName(second.UnitName(), app)
	if _, ok := kustPaths(ml)[own]; ok {
		t.Errorf("a Kustomization is named %q, from the application's own bundle, not the unit's", own)
	}
	writeAll(t, ml)
}

// TestLayoutKustomization_LongNameCollision: two layout names over 54
// characters below one unit that share their first 54 and whose 8-character
// hashes collide (the names below, both 35a47603 below the unit "shop") give
// the same name, and the duplicate-name check refuses the tree, naming both
// layouts (go-kure/kure#1036).
func TestLayoutKustomization_LongNameCollision(t *testing.T) {
	const base = "checkout-service-payments-reconciler-worker-hook-group"
	const short = base + "-35a47603"
	c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "prod",
		Bundle: srBundle("shop", cmApp(base+"-46715"), cmApp(base+"-61807"))}}
	rules := placed(layout.LayoutRules{BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupByName}, layout.FluxIntegratedPerLayout)
	_, err := integrateCluster(c, rules)
	mustContainAll(t, err, `"`+short+`"`, `layout "prod/shop/`+base+`-46715"`, `layout "prod/shop/`+base+`-61807"`)
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

	// A set name is never shortened (go-kure/kure#1030), however long, and
	// its refusal does not speak of the shortening of a default.
	t.Run("application layout, over the limit", func(t *testing.T) {
		c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "prod", Bundle: srBundle("web", cmApp("api"))}}
		rules := placed(layout.LayoutRules{BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupByName}, layout.FluxIntegratedPerLayout)
		root := mustWalk(t, c, rules)
		long := strings.Repeat("k", stack.KustomizationNameMaxLength+1)
		root.Children[0].Children[0].KustomizationName = long
		err := li.IntegrateWithLayout(root, c, rules)
		mustContainAll(t, err,
			"ManifestLayout 'prod/web/api'", "field 'kustomizationName'", `"`+long+`"`, "at most 63 characters")
		if strings.Contains(err.Error(), "shortened") {
			t.Errorf("the refusal of a set name speaks of shortening a default:\n%v", err)
		}
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
