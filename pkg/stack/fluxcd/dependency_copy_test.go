package fluxcd_test

import (
	"slices"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for a DependsOn bundle that is a copy of a bundle of the cluster: the
// generator resolves the copy by its Name, so it stands for that bundle
// whatever KustomizationName it carries.

// copyCluster is platform -> {dbn, webn}: dbn holds db (KustomizationName
// db-cr), webn holds web, which depends on a copy of db with KustomizationName
// copyName and names the Kustomizations in named. It returns the copy as well.
func copyCluster(copyName string, named ...string) (*stack.Cluster, *stack.Bundle) {
	db := srBundle("db", cmApp("db-app"))
	db.KustomizationName = "db-cr"
	dbCopy := *db
	dbCopy.KustomizationName = copyName
	web := srBundle("web", cmApp("web-app"))
	web.DependsOn = []*stack.Bundle{&dbCopy}
	web.NamedDependsOn = named
	dbn := &stack.Node{Name: "dbn", Bundle: db}
	webn := &stack.Node{Name: "webn", Bundle: web}
	root := &stack.Node{Name: "platform", Bundle: srBundle("platform", cmApp("core")), Children: []*stack.Node{dbn, webn}}
	dbn.SetParent(root)
	webn.SetParent(root)
	return &stack.Cluster{Name: "demo", Node: root}, &dbCopy
}

// The two refusals of a dependency copy, as substrings of the error.
var (
	contradictoryCopy = []string{`bundle "web"`, `copy of bundle "db"`, `KustomizationName "x"`, `KustomizationName "db-cr"`}
	copyInBothLists   = []string{`bundle "web"`, `dependency "db-cr" appears in both DependsOn and NamedDependsOn`, `DependsOn bundle "db"`}
)

func assertCopyRefused(t *testing.T, what string, err error, want []string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s accepted the dependency copy", what)
	}
	for _, w := range want {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("%s: error %q does not contain %s", what, err, w)
		}
	}
}

// TestDependencyCopy_Refused: every entry point that takes a cluster refuses a
// copy that sets another KustomizationName than the bundle it stands for, and
// says so also where the copy's name is one of the dependant's NamedDependsOn
// entries, which Bundle.Validate alone reports as one dependency in both
// lists. A name-only copy of a bundle whose Kustomization name is a
// NamedDependsOn entry is one dependency in both lists, which Bundle.Validate
// alone does not notice.
func TestDependencyCopy_Refused(t *testing.T) {
	for _, tc := range []struct {
		name     string
		copyName string
		named    []string
		want     []string
	}{
		{"another name", "x", nil, contradictoryCopy},
		{"another name, also a named dependency", "x", []string{"x"}, contradictoryCopy},
		{"no name, the bundle's is a named dependency", "", []string{"db-cr"}, copyInBothLists},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, placement := range allPlacements {
				rules := propertyGroupings["nodeOnly"]
				rules.FluxPlacement = placement
				c, _ := copyCluster(tc.copyName, tc.named...)
				_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
				assertCopyRefused(t, "CreateLayoutWithResources/"+string(placement), err, tc.want)
			}
			c, _ := copyCluster(tc.copyName, tc.named...)
			_, err := fluxstack.NewResourceGenerator().GenerateFromCluster(c, layout.DefaultLayoutRules())
			assertCopyRefused(t, "GenerateFromCluster", err, tc.want)

			c, _ = copyCluster(tc.copyName, tc.named...)
			_, err = layout.WalkCluster(c, propertyGroupings["nodeOnly"])
			assertCopyRefused(t, "WalkCluster", err, tc.want)
		})
	}
}

// TestDependencyCopy_RefusedOnAWalkedTree: the entry points that take a tree
// walked earlier refuse a copy changed since the walk, in every placement and
// before anything is generated.
func TestDependencyCopy_RefusedOnAWalkedTree(t *testing.T) {
	for _, placement := range allPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := propertyGroupings["nodeOnly"]
			rules.FluxPlacement = placement
			c, dbCopy := copyCluster("db-cr")
			ml, err := layout.WalkCluster(c, rules)
			if err != nil {
				t.Fatalf("WalkCluster with a true copy: %v", err)
			}
			dbCopy.KustomizationName = "x"

			objs, err := fluxstack.NewResourceGenerator().GenerateFromLayout(ml, c)
			assertCopyRefused(t, "GenerateFromLayout", err, contradictoryCopy)
			if objs != nil {
				t.Errorf("GenerateFromLayout returned %d objects next to the error", len(objs))
			}
			err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
			assertCopyRefused(t, "IntegrateWithLayout", err, contradictoryCopy)
			if got := kustomizations(ml); len(got) != 0 {
				t.Errorf("IntegrateWithLayout left %d Kustomizations in the refused tree", len(got))
			}

			dbCopy.KustomizationName = ""
			c.Node.Children[1].Bundle.NamedDependsOn = []string{"db-cr"}
			_, err = fluxstack.NewResourceGenerator().GenerateFromLayout(ml, c)
			assertCopyRefused(t, "GenerateFromLayout", err, copyInBothLists)
		})
	}
}

// TestDependencyCopy_Resolves: a copy that carries the bundle's
// KustomizationName and one that carries none are both the bundle, in every
// placement: the dependant's dependsOn names the bundle's Kustomization, once,
// beside a named dependency on another Kustomization.
func TestDependencyCopy_Resolves(t *testing.T) {
	for _, copyName := range []string{"db-cr", ""} {
		for _, placement := range allPlacements {
			t.Run(copyName+"/"+string(placement), func(t *testing.T) {
				rules := propertyGroupings["nodeOnly"]
				rules.FluxPlacement = placement
				c, _ := copyCluster(copyName, "external")
				got := kustomizationsByName(integrated(t, c, rules))
				if got["web"] == nil {
					t.Fatal("no Kustomization named web")
				}
				if deps := dependsOnNames(got["web"]); !slices.Equal(deps, []string{"db-cr", "external"}) {
					t.Errorf("web dependsOn = %v, want [db-cr external]", deps)
				}
			})
		}
	}
}

// TestDependencyCopy_ResolvesOnTheNameInEffect: a bundle without a
// KustomizationName is named by its Name, so a copy that sets that Name as its
// KustomizationName names the same Kustomization. It is the bundle in every
// placement and on the entry point without a layout.
func TestDependencyCopy_ResolvesOnTheNameInEffect(t *testing.T) {
	cluster := func() *stack.Cluster {
		c, dbCopy := copyCluster("db")
		c.Node.Children[0].Bundle.KustomizationName = ""
		if dbCopy.UnitName() != c.Node.Children[0].Bundle.UnitName() {
			t.Fatalf("the copy is named %q, the bundle %q", dbCopy.UnitName(), c.Node.Children[0].Bundle.UnitName())
		}
		return c
	}
	for _, placement := range allPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := propertyGroupings["nodeOnly"]
			rules.FluxPlacement = placement
			got := kustomizationsByName(integrated(t, cluster(), rules))
			if got["web"] == nil {
				t.Fatal("no Kustomization named web")
			}
			if deps := dependsOnNames(got["web"]); !slices.Equal(deps, []string{"db"}) {
				t.Errorf("web dependsOn = %v, want [db]", deps)
			}
		})
	}
	if _, err := fluxstack.NewResourceGenerator().GenerateFromCluster(cluster(), layout.DefaultLayoutRules()); err != nil {
		t.Errorf("GenerateFromCluster: %v", err)
	}
}

// TestDependencyCopy_GenerateForBundleWritesTheNameGiven: the one-bundle entry
// point has no cluster to compare a dependency with. It is unchanged: it
// writes the name the DependsOn bundle carries.
func TestDependencyCopy_GenerateForBundleWritesTheNameGiven(t *testing.T) {
	c, _ := copyCluster("x")
	web := c.Node.Children[1].Bundle
	objs, err := fluxstack.NewResourceGenerator().GenerateForBundle(web, "clusters/demo/web")
	if err != nil {
		t.Fatalf("GenerateForBundle: %v", err)
	}
	var deps []string
	for _, o := range objs {
		if k, ok := o.(*kustv1.Kustomization); ok {
			deps = dependsOnNames(k)
		}
	}
	if !slices.Equal(deps, []string{"x"}) {
		t.Errorf("dependsOn = %v, want [x], the name the DependsOn bundle carries", deps)
	}
}
