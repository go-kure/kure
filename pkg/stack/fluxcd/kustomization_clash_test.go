package fluxcd_test

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for the refusal of a generated Kustomization that meets one already
// in the tree (go-kure/kure#979): the error says whose the one in the tree
// is, what the generated one is for, and what names the generated one apart.

// authoredApp is an application named name that holds a Flux Kustomization
// named kustomization: a component that is itself a Flux Kustomization.
func authoredApp(name, kustomization string) *stack.Application {
	authored := fluxKustomization(kustomization, "./")
	return stack.NewApplication(name, "default", &fakeAppConfig{objs: []*client.Object{&authored}})
}

// TestKustomizationClash_AuthoredKustomizationNamedLikeItsBundle: a bundle
// whose application holds a Flux Kustomization of the bundle's own name is
// refused under every placement, and the error names the authored one by the
// application and bundle that hold it, the generated one by the bundle it is
// for, and Bundle.KustomizationName on that bundle as the way out. With the
// field set the tree integrates, and both Kustomizations are in it.
func TestKustomizationClash_AuthoredKustomizationNamedLikeItsBundle(t *testing.T) {
	cluster := func(kustomizationName string) *stack.Cluster {
		shop := srBundle("shop", authoredApp("delivery", "shop"))
		shop.KustomizationName = kustomizationName
		apps := &stack.Node{Name: "apps", Bundle: shop}
		root := &stack.Node{Name: "platform", Bundle: srBundle("platform", cmApp("core")), Children: []*stack.Node{apps}}
		apps.SetParent(root)
		return &stack.Cluster{Name: "demo", Node: root}
	}
	// With applications grouped by name the authored Kustomization sits in the
	// application's own directory, with a flat grouping in the bundle's.
	for _, grouping := range []string{"nodeOnly", "GroupByName"} {
		for _, placement := range allPlacements {
			t.Run(grouping+"/"+string(placement), func(t *testing.T) {
				rules := placed(propertyGroupings[grouping], placement)
				integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())

				_, err := integrator.CreateLayoutWithResources(cluster(""), rules)
				mustContainAll(t, err,
					`already has Flux Kustomization "shop"`,
					`an object of application "delivery" of bundle "shop"`,
					`for bundle "shop"`,
					`the two share namespace and name`,
					`set Bundle.KustomizationName on bundle "shop" to name the generated one`,
					`or give the one already there another name`,
				)

				ml := integrated(t, cluster("apps-shop"), rules)
				generated := kustomizationsByName(ml)
				if _, ok := generated["apps-shop"]; !ok {
					t.Errorf("generated Kustomizations = %v, want apps-shop among them", slices.Sorted(maps.Keys(generated)))
				}
				if n := countPayloadKustomizations(ml, "shop"); n != 1 {
					t.Errorf("the tree holds %d authored Kustomizations named shop, want 1", n)
				}
			})
		}
	}
}

// TestKustomizationClash_NodeAndLayoutKustomizations: under
// FluxIntegratedPerLayout a bundle-less node and an application's own
// directory get a Kustomization each. One an application holds under that
// name is refused the same way, and the error names the node or the layout
// the generated one is for, and the field that names it: the node's
// KustomizationName, or the layout's. With the field set the tree integrates.
func TestKustomizationClash_NodeAndLayoutKustomizations(t *testing.T) {
	// platform -> group (no bundle) -> web: the group node's Kustomization is
	// named after its directory.
	cluster := func(authored, groupName string) *stack.Cluster {
		web := &stack.Node{Name: "web", Bundle: srBundle("web", authoredApp("delivery", authored))}
		group := &stack.Node{Name: "group", KustomizationName: groupName, Children: []*stack.Node{web}}
		root := &stack.Node{Name: "platform", Bundle: srBundle("platform", cmApp("core")), Children: []*stack.Node{group}}
		web.SetParent(group)
		group.SetParent(root)
		return &stack.Cluster{Name: "demo", Node: root}
	}
	li := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())

	t.Run("a node's Kustomization", func(t *testing.T) {
		rules := perLayoutNodeOnly()
		_, err := li.CreateLayoutWithResources(cluster("platform-group-node", ""), rules)
		mustContainAll(t, err,
			`already has Flux Kustomization "platform-group-node"`,
			`an object of application "delivery" of bundle "web"`,
			`for node "platform/group"`,
			`set Node.KustomizationName on node "platform/group" to name the generated one`,
			`or give the one already there another name`,
		)

		generated := kustomizationsByName(integrated(t, cluster("platform-group-node", "group"), rules))
		if _, ok := generated["group"]; !ok {
			t.Errorf("generated Kustomizations = %v, want group among them", slices.Sorted(maps.Keys(generated)))
		}
	})

	t.Run("a node's Kustomization named by the node", func(t *testing.T) {
		_, err := li.CreateLayoutWithResources(cluster("group", "group"), perLayoutNodeOnly())
		mustContainAll(t, err,
			`already has Flux Kustomization "group"`,
			`for node "platform/group"`,
			`set Node.KustomizationName on node "platform/group" to name the generated one`,
		)
	})

	// A name the caller sets on the node's walked layout is the one in effect,
	// whatever the node sets: the layout's field is the one that helps.
	t.Run("a node's Kustomization named on its layout", func(t *testing.T) {
		rules := perLayoutNodeOnly()
		walked := func(layoutName string) (*layout.ManifestLayout, *stack.Cluster) {
			c := cluster("delivery", "group")
			ml, err := layout.WalkCluster(c, rules)
			if err != nil {
				t.Fatalf("WalkCluster: %v", err)
			}
			layoutAtPath(t, ml, "platform/group").KustomizationName = layoutName
			return ml, c
		}

		ml, c := walked("delivery")
		err := li.IntegrateWithLayout(ml, c, rules)
		mustContainAll(t, err,
			`already has Flux Kustomization "delivery"`,
			`an object of application "delivery" of bundle "web"`,
			`for layout "platform/group"`,
			`set ManifestLayout.KustomizationName on layout "platform/group" to name the generated one`,
			`or give the one already there another name`,
		)
		if strings.Contains(err.Error(), "Node.KustomizationName") {
			t.Errorf("the error names the node's field, which does not name this Kustomization:\n%v", err)
		}

		ml, c = walked("group-own")
		if err := li.IntegrateWithLayout(ml, c, rules); err != nil {
			t.Fatalf("IntegrateWithLayout with the layout's KustomizationName set: %v", err)
		}
		if _, ok := kustomizationsByName(ml)["group-own"]; !ok {
			t.Errorf("generated Kustomizations = %v, want group-own among them", slices.Sorted(maps.Keys(kustomizationsByName(ml))))
		}
	})

	t.Run("an application layout's Kustomization", func(t *testing.T) {
		rules := placed(propertyGroupings["GroupByName"], layout.FluxIntegratedPerLayout)
		_, err := li.CreateLayoutWithResources(cluster("web-delivery", ""), rules)
		mustContainAll(t, err,
			`already has Flux Kustomization "web-delivery"`,
			`an object of application "delivery" of bundle "web"`,
			`for layout "platform/group/web/web/delivery"`,
			`set ManifestLayout.KustomizationName on layout "platform/group/web/web/delivery" to name the generated one`,
			`or give the one already there another name`,
		)

		c := cluster("web-delivery", "")
		ml, err := layout.WalkCluster(c, rules)
		if err != nil {
			t.Fatalf("WalkCluster: %v", err)
		}
		layoutAtPath(t, ml, "platform/group/web/web/delivery").KustomizationName = "web-delivery-app"
		if err := li.IntegrateWithLayout(ml, c, rules); err != nil {
			t.Fatalf("IntegrateWithLayout with the layout's KustomizationName set: %v", err)
		}
		if _, ok := kustomizationsByName(ml)["web-delivery-app"]; !ok {
			t.Errorf("generated Kustomizations = %v, want web-delivery-app among them", slices.Sorted(maps.Keys(kustomizationsByName(ml))))
		}
	})
}

// TestKustomizationClash_KustomizationNoApplicationHolds: a Kustomization a
// caller adds to a walked layout is no application's. The refusal names it by
// its layout alone, claims no origin for it, and names the same way out.
func TestKustomizationClash_KustomizationNoApplicationHolds(t *testing.T) {
	for _, placement := range allPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := placed(propertyGroupings["nodeOnly"], placement)
			c := propertyShapes["different-name"]()
			ml, err := layout.WalkCluster(c, rules)
			if err != nil {
				t.Fatalf("WalkCluster: %v", err)
			}
			ml.Resources = append(ml.Resources, fluxKustomization("web-bundle", "elsewhere"))

			err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
			mustContainAll(t, err,
				`already has Flux Kustomization "web-bundle"`,
				`for bundle "web-bundle"`,
				`set Bundle.KustomizationName on bundle "web-bundle" to name the generated one`,
				`or give the one already there another name`,
			)
			if strings.Contains(err.Error(), "an object of application") {
				t.Errorf("the error names an application for a Kustomization no application holds:\n%v", err)
			}
		})
	}
}

// TestKustomizationClash_RecordWithoutApplication: a walk record whose
// application a caller has cleared names no application. The refusal still
// comes back, as it does for any other Kustomization no application holds,
// also where the bundle lists a nil application the cleared record would
// match.
func TestKustomizationClash_RecordWithoutApplication(t *testing.T) {
	var clearRecords func(l *layout.ManifestLayout) int
	clearRecords = func(l *layout.ManifestLayout) int {
		n := 0
		recs := l.OriginApplicationObjects()
		for i := range recs {
			if recs[i].Application != nil && recs[i].Application.Name == "delivery" {
				recs[i].Application = nil
				n++
			}
		}
		for _, child := range l.Children {
			n += clearRecords(child)
		}
		return n
	}
	for _, placement := range allPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := placed(propertyGroupings["nodeOnly"], placement)
			shop := srBundle("shop", authoredApp("delivery", "shop"))
			apps := &stack.Node{Name: "apps", Bundle: shop}
			root := &stack.Node{Name: "platform", Bundle: srBundle("platform", cmApp("core")), Children: []*stack.Node{apps}}
			apps.SetParent(root)
			c := &stack.Cluster{Name: "demo", Node: root}
			ml, err := layout.WalkCluster(c, rules)
			if err != nil {
				t.Fatalf("WalkCluster: %v", err)
			}
			if n := clearRecords(ml); n != 1 {
				t.Fatalf("cleared %d records of application delivery, want 1", n)
			}
			shop.Applications = append(shop.Applications, nil)

			err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
			mustContainAll(t, err,
				`already has Flux Kustomization "shop"`,
				`for bundle "shop"`,
				`set Bundle.KustomizationName on bundle "shop" to name the generated one`,
			)
			if strings.Contains(err.Error(), "an object of application") {
				t.Errorf("the error names an application for a record that has none:\n%v", err)
			}
		})
	}
}
