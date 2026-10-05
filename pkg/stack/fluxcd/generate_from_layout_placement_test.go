package fluxcd_test

import (
	"fmt"
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for GenerateFromLayout refusing a tree that carries
// FluxIntegratedPerLayout (go-kure/kure#979): the writers list no directory
// child of such a layout in its kustomization.yaml, and only the integrator
// places the Kustomization that applies one.

// assertPerLayoutRefused checks the refusal: no object, and an error that
// names the placement, the layout carrying it and both integrator entry points.
func assertPerLayoutRefused(t *testing.T, name string, objs []client.Object, err error, carrier *layout.ManifestLayout) {
	t.Helper()
	if objs != nil {
		t.Errorf("%s: got %d objects with the refusal", name, len(objs))
	}
	if err == nil {
		t.Errorf("%s: no error, want a refusal of the placement", name)
		return
	}
	for _, want := range []string{
		fmt.Sprintf("FluxPlacement %q", layout.FluxIntegratedPerLayout),
		fmt.Sprintf("layout %q", carrier.FullRepoPath()),
		"CreateLayoutWithResources",
		"IntegrateWithLayout",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error %q does not name %s", name, err, want)
		}
	}
}

// TestGenerateFromLayout_RefusesPerLayout: a tree walked with
// FluxIntegratedPerLayout is refused whatever its groupings, with or without
// a cluster, and so is one the integrator already placed: the call would
// return a second copy of the bundle directories' Kustomizations and none of
// the others.
func TestGenerateFromLayout_RefusesPerLayout(t *testing.T) {
	byName := layout.DefaultLayoutRules()
	byName.ApplicationGrouping = layout.GroupByName
	rules := byName
	rules.FluxPlacement = layout.FluxIntegratedPerLayout
	flat := layout.DefaultLayoutRules()
	flat.FluxPlacement = layout.FluxIntegratedPerLayout
	build := func() *stack.Cluster { return threeTier("platform", "apps", "web", nil) }

	// The case the refusal is for: written to disk and to tar, the tree these
	// rules walk leaves directories out of their parents' kustomization.yaml
	// that the separate placement's tree lists (the application directories).
	separate, perLayout := unlistedDirs(t, build, byName), unlistedDirs(t, build, rules)
	for _, writer := range []string{"WriteToDisk", "WriteToTar"} {
		if perLayout[writer] <= separate[writer] {
			t.Fatalf("%s: the per-layout tree leaves %d directories unlisted, the separate tree %d: want more, or the list would be complete",
				writer, perLayout[writer], separate[writer])
		}
	}

	walked := func(rules layout.LayoutRules) (*layout.ManifestLayout, *stack.Cluster) {
		c := build()
		ml, err := layout.WalkCluster(c, rules)
		if err != nil {
			t.Fatalf("WalkCluster: %v", err)
		}
		return ml, c
	}
	gen := fluxstack.NewResourceGenerator()

	t.Run("a directory per application", func(t *testing.T) {
		ml, c := walked(rules)
		objs, err := gen.GenerateFromLayout(ml, c)
		assertPerLayoutRefused(t, "GenerateFromLayout", objs, err, ml)
	})
	// Refused although its list would be complete: the placement, not the
	// tree's shape, decides, as it does for GenerateFromCluster.
	t.Run("flat applications", func(t *testing.T) {
		ml, c := walked(flat)
		objs, err := gen.GenerateFromLayout(ml, c)
		assertPerLayoutRefused(t, "GenerateFromLayout", objs, err, ml)
	})
	t.Run("no cluster", func(t *testing.T) {
		ml, _ := walked(rules)
		objs, err := gen.GenerateFromLayout(ml, nil)
		assertPerLayoutRefused(t, "nil cluster", objs, err, ml)
		objs, err = gen.GenerateFromLayout(ml, &stack.Cluster{Name: "empty"})
		assertPerLayoutRefused(t, "cluster without a root node", objs, err, ml)
	})
	t.Run("already integrated", func(t *testing.T) {
		c := build()
		ml := integrated(t, c, rules)
		placed := len(kustomizations(ml))
		objs, err := gen.GenerateFromLayout(ml, c)
		assertPerLayoutRefused(t, "GenerateFromLayout", objs, err, ml)
		if got := len(kustomizations(ml)); got != placed {
			t.Errorf("the refused call changed the tree: %d Kustomizations, had %d", got, placed)
		}
	})
}

// TestGenerateFromLayout_RefusesPerLayoutAnywhere: the writers decide per
// parent, so one layout carrying the placement below a root that does not is
// enough to leave its directory children unlisted. The error names the first
// such layout in pre-order.
func TestGenerateFromLayout_RefusesPerLayoutAnywhere(t *testing.T) {
	byName := layout.DefaultLayoutRules()
	byName.ApplicationGrouping = layout.GroupByName
	build := func() *stack.Cluster { return threeTier("platform", "apps", "web", nil) }
	mixed := func(t *testing.T) (*layout.ManifestLayout, *layout.ManifestLayout, *stack.Cluster) {
		t.Helper()
		c := build()
		ml, err := layout.WalkCluster(c, byName)
		if err != nil {
			t.Fatalf("WalkCluster: %v", err)
		}
		apps := layoutAtPath(t, ml, "platform/apps")
		apps.FluxPlacement = layout.FluxIntegratedPerLayout
		web := layoutAtPath(t, ml, "platform/apps/web")
		web.FluxPlacement = layout.FluxIntegratedPerLayout
		return ml, apps, c
	}

	ml, _, _ := mixed(t)
	separate, got := unlistedDirs(t, build, byName), unlistedIn(t, ml)
	for _, writer := range []string{"WriteToDisk", "WriteToTar"} {
		if got[writer] <= separate[writer] {
			t.Fatalf("%s: the mixed tree leaves %d directories unlisted, the separate tree %d: want more, or the list would be complete",
				writer, got[writer], separate[writer])
		}
	}

	ml, apps, c := mixed(t)
	if ml.FluxPlacement == layout.FluxIntegratedPerLayout {
		t.Fatalf("the root carries %q: the case needs a root that does not", ml.FluxPlacement)
	}
	objs, err := fluxstack.NewResourceGenerator().GenerateFromLayout(ml, c)
	assertPerLayoutRefused(t, "GenerateFromLayout", objs, err, apps)
}

// TestGenerateFromLayout_OtherPlacementsNotRefused: the trees whose list is
// complete are generated as before, and the integrator, which calls
// GenerateFromLayout under FluxSeparate, is not refused by it for a tree that
// was walked per-layout: it sets its own placement on the tree first.
func TestGenerateFromLayout_OtherPlacementsNotRefused(t *testing.T) {
	byName := layout.DefaultLayoutRules()
	byName.ApplicationGrouping = layout.GroupByName
	build := func() *stack.Cluster { return threeTier("platform", "apps", "web", nil) }

	for _, placement := range []layout.FluxPlacement{layout.FluxSeparate, layout.FluxIntegratedPerBundle} {
		t.Run(string(placement), func(t *testing.T) {
			rules := byName
			rules.FluxPlacement = placement
			c := build()
			ml, err := layout.WalkCluster(c, rules)
			if err != nil {
				t.Fatalf("WalkCluster: %v", err)
			}
			objs, err := fluxstack.NewResourceGenerator().GenerateFromLayout(ml, c)
			if err != nil {
				t.Fatalf("GenerateFromLayout: %v", err)
			}
			if got := len(specPaths(objs)); got != 3 {
				t.Errorf("got %d Kustomizations, want one per bundle directory (3): %v", got, specPaths(objs))
			}
		})
	}

	t.Run("walked per-layout, integrated as separate", func(t *testing.T) {
		perLayout := byName
		perLayout.FluxPlacement = layout.FluxIntegratedPerLayout
		c := build()
		ml, err := layout.WalkCluster(c, perLayout)
		if err != nil {
			t.Fatalf("WalkCluster: %v", err)
		}
		separate := byName
		separate.FluxPlacement = layout.FluxSeparate
		if err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, separate); err != nil {
			t.Fatalf("IntegrateWithLayout: %v", err)
		}
		if got := len(kustomizations(ml)); got != 3 {
			t.Errorf("the integrated tree holds %d Kustomizations, want one per bundle directory (3)", got)
		}
	})
}
