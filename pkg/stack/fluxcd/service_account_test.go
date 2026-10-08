package fluxcd_test

import (
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for Bundle.ServiceAccountName (go-kure/kure#1034): the bundle's
// Kustomization carries it as spec.serviceAccountName, the per-layout
// Kustomizations of its applications under FluxIntegratedPerLayout inherit
// it, and an umbrella's is not passed to its children. Unset, every file is
// the one testdata/unset-kustomization-name holds
// (TestKustomizationName_UnsetOutputUnchanged).

// accountCluster is unnamedCluster with service accounts on three of its five
// bundles: the umbrella shop names shop-deployer, its child shop-db names
// db-deployer, and monitoring names ops-deployer. The umbrella's other child
// shop-api and the root node's bundle platform name none.
func accountCluster() *stack.Cluster {
	c := unnamedCluster()
	shop := c.Node.Children[0].Bundle
	shop.ServiceAccountName = "shop-deployer"
	shop.Children[0].ServiceAccountName = "db-deployer"
	c.Node.Children[1].Bundle.ServiceAccountName = "ops-deployer"
	return c
}

// accountsByKustomization is the service account each Kustomization of
// accountCluster carries. A bundle's Kustomization has its bundle's; the
// per-layout one of an application's layout, only under
// FluxIntegratedPerLayout, has that of the bundle holding the application. A
// node Kustomization ("-node") and the Kustomizations of shop-api and
// platform carry none: shop-api is not given the umbrella's.
var accountsByKustomization = map[layout.FluxPlacement]map[string]string{
	layout.FluxSeparate: {
		"platform": "", "shop": "shop-deployer", "shop-db": "db-deployer", "shop-api": "", "monitoring": "ops-deployer",
	},
	layout.FluxIntegratedPerBundle: {
		"platform": "", "shop": "shop-deployer", "shop-db": "db-deployer", "shop-api": "", "monitoring": "ops-deployer",
	},
	layout.FluxIntegratedPerLayout: {
		"platform": "", "shop": "shop-deployer", "shop-db": "db-deployer", "shop-api": "", "monitoring": "ops-deployer",
		"platform-platform-app":     "",
		"shop-shop-app":             "shop-deployer",
		"shop-db-shop-db-app":       "db-deployer",
		"shop-api-shop-api-app":     "",
		"monitoring-monitoring-app": "ops-deployer",
		"platform-apps-node":        "",
		"platform-ops-node":         "",
	},
}

// TestServiceAccountName_EachPlacement writes accountCluster under each
// placement and checks the service account of every Kustomization against
// accountsByKustomization, which names every Kustomization the placement
// generates, and the written tree against the stored one.
func TestServiceAccountName_EachPlacement(t *testing.T) {
	for _, placement := range allPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := propertyGroupings["GroupByName"]
			rules.FluxPlacement = placement
			ml := integrated(t, accountCluster(), rules)

			want := accountsByKustomization[placement]
			got := map[string]string{}
			for _, k := range kustomizations(ml) {
				got[k.Name] = k.Spec.ServiceAccountName
			}
			names := slices.Sorted(maps.Keys(got))
			if len(got) != len(want) {
				t.Errorf("Kustomizations %q, want the %d of the table", names, len(want))
			}
			for name, account := range want {
				if g, ok := got[name]; !ok {
					t.Errorf("no Kustomization %q among %q", name, names)
				} else if g != account {
					t.Errorf("%s: serviceAccountName = %q, want %q", name, g, account)
				}
			}

			dir := t.TempDir()
			if err := ml.WriteToDisk(dir); err != nil {
				t.Fatalf("WriteToDisk: %v", err)
			}
			compareWithStored(t, filepath.Join("testdata", "service-account-name", string(placement)+".txt"), treeBytes(t, dir))
		})
	}
}

// TestServiceAccountName_MergedBundles: bundles a flat grouping merges into one
// directory share one Kustomization, which holds one service account. The
// same name on both is that Kustomization's; two names are refused, naming
// the field and both bundles.
func TestServiceAccountName_MergedBundles(t *testing.T) {
	build := func(platform, apps string) *stack.Cluster {
		root := &stack.Node{Name: "platform", Bundle: srBundle("platform", cmApp("platform-app"))}
		child := &stack.Node{Name: "apps", Bundle: srBundle("apps", cmApp("apps-app"))}
		root.Bundle.ServiceAccountName = platform
		child.Bundle.ServiceAccountName = apps
		root.Children = []*stack.Node{child}
		child.SetParent(root)
		return &stack.Cluster{Name: "demo", Node: root}
	}

	ml := integrated(t, build("deployer", "deployer"), allFlat)
	if got := mustKustomization(t, ml, "platform").Spec.ServiceAccountName; got != "deployer" {
		t.Errorf("merged Kustomization: serviceAccountName = %q, want deployer", got)
	}

	_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(build("deployer", "other"), allFlat)
	if err == nil {
		t.Fatal("two service accounts in one directory: want an error, got none")
	}
	for _, part := range []string{"serviceAccountName", `"platform"`, `"apps"`, "one Kustomization holds one value"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("error %q does not contain %q", err, part)
		}
	}
}

// TestServiceAccountName_GenerateForBundle: a bundle generated on its own,
// without Validate, gets its service account, and an invalid name is refused
// there too, naming the bundle by its path and the field.
func TestServiceAccountName_GenerateForBundle(t *testing.T) {
	gen := fluxstack.NewResourceGenerator()
	b := srBundle("shop", cmApp("shop-app"))
	b.ServiceAccountName = "shop-deployer"
	objs, err := gen.GenerateForBundle(b, "clusters/prod/shop")
	if err != nil {
		t.Fatalf("GenerateForBundle: %v", err)
	}
	k, ok := objs[0].(*kustv1.Kustomization)
	if !ok {
		t.Fatalf("first object is %T, want a Kustomization", objs[0])
	}
	if k.Spec.ServiceAccountName != "shop-deployer" {
		t.Errorf("serviceAccountName = %q, want shop-deployer", k.Spec.ServiceAccountName)
	}

	child := srBundle("db", cmApp("db-app"))
	child.ServiceAccountName = "DB_Deployer"
	umbrella := srBundle("shop", cmApp("shop-app"))
	umbrella.Children = []*stack.Bundle{child}
	umbrella.InitializeUmbrella()
	_, err = gen.GenerateForBundle(child, "clusters/prod/shop/db")
	if err == nil {
		t.Fatal("invalid service account name: want an error, got none")
	}
	for _, part := range []string{"shop/db", "serviceAccountName", `"DB_Deployer" is not a valid service account name`} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("error %q does not contain %q", err, part)
		}
	}
}
