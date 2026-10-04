package fluxcd_test

import (
	"maps"
	"slices"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for Bundle.KustomizationName in the Flux workflow: the generated
// Kustomization takes that name, every reference to it follows, and the
// bundle's directory does not.

var allPlacements = []layout.FluxPlacement{layout.FluxSeparate, layout.FluxIntegratedPerBundle, layout.FluxIntegratedPerLayout}

// shopCluster is platform -> {apps, ops}. apps holds the umbrella bundle shop
// with children shop-db and shop-api (which depends on shop-db); ops holds
// monitoring, which depends on shop. names sets KustomizationName per bundle
// name.
func shopCluster(names map[string]string) *stack.Cluster {
	bundle := func(name string) *stack.Bundle {
		b := srBundle(name, cmApp(name+"-app"))
		b.KustomizationName = names[name]
		return b
	}
	db := bundle("shop-db")
	api := bundle("shop-api")
	api.DependsOn = []*stack.Bundle{db}
	shop := bundle("shop")
	shop.Children = []*stack.Bundle{db, api}
	monitoring := bundle("monitoring")
	monitoring.DependsOn = []*stack.Bundle{shop}
	apps := &stack.Node{Name: "apps", Bundle: shop}
	ops := &stack.Node{Name: "ops", Bundle: monitoring}
	root := &stack.Node{Name: "platform", Bundle: bundle("platform"), Children: []*stack.Node{apps, ops}}
	apps.SetParent(root)
	ops.SetParent(root)
	return &stack.Cluster{Name: "demo", Node: root}
}

func kustomizationsByName(ml *layout.ManifestLayout) map[string]*kustv1.Kustomization {
	out := map[string]*kustv1.Kustomization{}
	for _, k := range kustomizations(ml) {
		out[k.Name] = k
	}
	return out
}

// kustomizationHealthChecks returns the names of the Flux Kustomizations k
// health-checks.
func kustomizationHealthChecks(k *kustv1.Kustomization) []string {
	var out []string
	for _, hc := range k.Spec.HealthChecks {
		if hc.Kind == "Kustomization" {
			out = append(out, hc.Name)
		}
	}
	return out
}

// TestKustomizationName_NamesTheKustomizationAndItsReferences: in every
// placement a bundle's Kustomization is named by KustomizationName, a
// dependant's dependsOn and an umbrella's health check name it the same way,
// a bundle without the field keeps its name, and every spec.path is the one
// the same cluster gets without the field: the directory follows Name.
func TestKustomizationName_NamesTheKustomizationAndItsReferences(t *testing.T) {
	renamed := map[string]string{"shop": "apps-shop", "shop-db": "apps-shop-db"}
	for _, placement := range allPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := propertyGroupings["GroupByName"]
			rules.FluxPlacement = placement
			baseline := kustomizationsByName(integrated(t, shopCluster(nil), rules))
			got := kustomizationsByName(integrated(t, shopCluster(renamed), rules))

			for _, name := range []string{"shop", "shop-db"} {
				if _, ok := got[name]; ok {
					t.Errorf("a Kustomization is named %q, the bundle's Name, although the bundle sets KustomizationName", name)
				}
			}
			if !slices.Equal(slices.Sorted(maps.Keys(got)), renamedKeys(baseline, renamed)) {
				t.Fatalf("Kustomizations = %v, want those of the unnamed cluster %v with shop and shop-db renamed",
					slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(baseline)))
			}
			for name, base := range baseline {
				effective := name
				if n := renamed[name]; n != "" {
					effective = n
				}
				if got[effective].Spec.Path != base.Spec.Path {
					t.Errorf("Kustomization %q has spec.path %q, want %q: the directory follows the bundle's Name",
						effective, got[effective].Spec.Path, base.Spec.Path)
				}
			}
			if deps := dependsOnNames(got["shop-api"]); !slices.Equal(deps, []string{"apps-shop-db"}) {
				t.Errorf("shop-api dependsOn = %v, want [apps-shop-db]", deps)
			}
			if deps := dependsOnNames(got["monitoring"]); !slices.Equal(deps, []string{"apps-shop"}) {
				t.Errorf("monitoring dependsOn = %v, want [apps-shop]", deps)
			}
			if checks := kustomizationHealthChecks(got["apps-shop"]); !slices.Equal(checks, []string{"apps-shop-db", "shop-api"}) {
				t.Errorf("apps-shop health checks = %v, want [apps-shop-db shop-api]", checks)
			}
		})
	}
}

// renamedKeys returns baseline's names, sorted, with each renamed one replaced.
func renamedKeys(baseline map[string]*kustv1.Kustomization, renamed map[string]string) []string {
	var out []string
	for name := range baseline {
		if n := renamed[name]; n != "" {
			name = n
		}
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

// TestKustomizationName_PayloadKustomizationNamedLikeTheBundle: a bundle
// whose application emits a Flux Kustomization named like the bundle collides
// with the one generated for it, in every placement; with KustomizationName
// set the generated one has another name and the payload object is kept.
func TestKustomizationName_PayloadKustomizationNamedLikeTheBundle(t *testing.T) {
	cluster := func(kustomizationName string) *stack.Cluster {
		payload := fluxKustomization("shop", "./")
		shop := srBundle("shop", stack.NewApplication("shop-delivery", "default", &fakeAppConfig{objs: []*client.Object{&payload}}))
		shop.KustomizationName = kustomizationName
		apps := &stack.Node{Name: "apps", Bundle: shop}
		root := &stack.Node{Name: "platform", Bundle: srBundle("platform", cmApp("core")), Children: []*stack.Node{apps}}
		apps.SetParent(root)
		return &stack.Cluster{Name: "demo", Node: root}
	}
	for _, placement := range allPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := propertyGroupings["nodeOnly"]
			rules.FluxPlacement = placement
			integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())

			_, err := integrator.CreateLayoutWithResources(cluster(""), rules)
			if err == nil || !strings.Contains(err.Error(), `already has Flux Kustomization "shop"`) {
				t.Fatalf("without KustomizationName: got %v, want the collision with the payload Kustomization", err)
			}

			ml := integrated(t, cluster("apps-shop"), rules)
			generated := kustomizationsByName(ml)
			if _, ok := generated["apps-shop"]; !ok {
				t.Errorf("generated Kustomizations = %v, want apps-shop among them", slices.Sorted(maps.Keys(generated)))
			}
			if _, ok := generated["shop"]; ok {
				t.Error(`a generated Kustomization is named "shop", the payload's name`)
			}
			if n := countPayloadKustomizations(ml, "shop"); n != 1 {
				t.Errorf("the tree holds %d payload Kustomizations named shop, want 1", n)
			}
		})
	}
}

// countPayloadKustomizations counts the unstructured Flux Kustomizations
// named name in the tree: what an application emitted, not what the
// generator did.
func countPayloadKustomizations(ml *layout.ManifestLayout, name string) int {
	n := 0
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		for _, r := range l.Resources {
			if u, ok := r.(*unstructured.Unstructured); ok && u.GetKind() == "Kustomization" && u.GetName() == name {
				n++
			}
		}
		for _, c := range l.Children {
			walk(c)
		}
	}
	walk(ml)
	return n
}

// TestKustomizationName_DuplicateRefused: two bundles whose Kustomizations
// would get one name are refused in every placement, and the error names
// both bundles by their paths.
func TestKustomizationName_DuplicateRefused(t *testing.T) {
	for _, placement := range allPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := propertyGroupings["nodeOnly"]
			rules.FluxPlacement = placement
			_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).
				CreateLayoutWithResources(shopCluster(map[string]string{"shop-db": "data", "monitoring": "data"}), rules)
			if err == nil {
				t.Fatal("CreateLayoutWithResources accepted two bundles with one Kustomization name")
			}
			for _, want := range []string{`"shop/shop-db"`, `"monitoring"`, `"data"`} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %s", err, want)
				}
			}
		})
	}
}

// TestKustomizationName_GenerateFromCluster: the default-rules entry point
// names the Kustomizations and their references the same way.
func TestKustomizationName_GenerateFromCluster(t *testing.T) {
	objs, err := fluxstack.NewResourceGenerator().GenerateFromCluster(
		shopCluster(map[string]string{"shop": "apps-shop", "shop-db": "apps-shop-db"}))
	if err != nil {
		t.Fatalf("GenerateFromCluster: %v", err)
	}
	got := map[string]*kustv1.Kustomization{}
	for _, o := range objs {
		if k, ok := o.(*kustv1.Kustomization); ok {
			got[k.Name] = k
		}
	}
	if want := []string{"apps-shop", "apps-shop-db", "monitoring", "platform", "shop-api"}; !slices.Equal(slices.Sorted(maps.Keys(got)), want) {
		t.Fatalf("Kustomizations = %v, want %v", slices.Sorted(maps.Keys(got)), want)
	}
	if deps := dependsOnNames(got["monitoring"]); !slices.Equal(deps, []string{"apps-shop"}) {
		t.Errorf("monitoring dependsOn = %v, want [apps-shop]", deps)
	}
	if checks := kustomizationHealthChecks(got["apps-shop"]); !slices.Equal(checks, []string{"apps-shop-db", "shop-api"}) {
		t.Errorf("apps-shop health checks = %v, want [apps-shop-db shop-api]", checks)
	}
}

// TestKustomizationName_GenerateForBundle: the one-bundle entry point, which
// has no layout index, names the Kustomization, its umbrella health checks
// and its dependencies by each bundle's KustomizationName.
func TestKustomizationName_GenerateForBundle(t *testing.T) {
	db := srBundle("db")
	db.KustomizationName = "db-cr"
	child := srBundle("shop-api")
	child.KustomizationName = "apps-shop-api"
	shop := srBundle("shop")
	shop.KustomizationName = "apps-shop"
	shop.Children = []*stack.Bundle{child, srBundle("shop-ui")}
	shop.DependsOn = []*stack.Bundle{db}
	shop.NamedDependsOn = []string{"external"}

	objs, err := fluxstack.NewResourceGenerator().GenerateForBundle(shop, "clusters/demo/shop")
	if err != nil {
		t.Fatalf("GenerateForBundle: %v", err)
	}
	var kust *kustv1.Kustomization
	for _, o := range objs {
		if k, ok := o.(*kustv1.Kustomization); ok {
			kust = k
		}
	}
	if kust == nil {
		t.Fatal("GenerateForBundle returned no Kustomization")
	}
	if kust.Name != "apps-shop" {
		t.Errorf("Kustomization name = %q, want apps-shop", kust.Name)
	}
	if kust.Spec.Path != "clusters/demo/shop" {
		t.Errorf("spec.path = %q, want the path passed in", kust.Spec.Path)
	}
	if checks := kustomizationHealthChecks(kust); !slices.Equal(checks, []string{"apps-shop-api", "shop-ui"}) {
		t.Errorf("health checks = %v, want [apps-shop-api shop-ui]", checks)
	}
	if deps := dependsOnNames(kust); !slices.Equal(deps, []string{"db-cr", "external"}) {
		t.Errorf("dependsOn = %v, want [db-cr external]", deps)
	}
}

// TestKustomizationName_GenerateForBundleRefusesItsOwnName: GenerateForBundle
// sees one bundle and the bundles it points at, with no origin index to
// refuse two bundles with one Kustomization name. A dependency or an umbrella
// child that would get the bundle's own name is refused there, naming both
// bundles, instead of a Kustomization that depends on itself or waits for
// itself.
func TestKustomizationName_GenerateForBundleRefusesItsOwnName(t *testing.T) {
	tests := []struct {
		name  string
		build func() *stack.Bundle
		want  []string
	}{
		{
			name: "dependency with the bundle's KustomizationName",
			build: func() *stack.Bundle {
				db := srBundle("db")
				db.KustomizationName = "shared"
				web := srBundle("web")
				web.KustomizationName = "shared"
				web.DependsOn = []*stack.Bundle{db}
				return web
			},
			want: []string{`"web"`, `"db"`, `"shared"`},
		},
		{
			name: "dependency whose KustomizationName is the bundle's Name",
			build: func() *stack.Bundle {
				db := srBundle("db")
				db.KustomizationName = "web"
				web := srBundle("web")
				web.DependsOn = []*stack.Bundle{db}
				return web
			},
			want: []string{`"web"`, `"db"`},
		},
		{
			name: "umbrella child with the bundle's KustomizationName",
			build: func() *stack.Bundle {
				api := srBundle("shop-api")
				api.KustomizationName = "apps-shop"
				shop := srBundle("shop")
				shop.KustomizationName = "apps-shop"
				shop.Children = []*stack.Bundle{api}
				return shop
			},
			want: []string{`"shop"`, `"shop/shop-api"`, `"apps-shop"`},
		},
		{
			name: "NamedDependsOn entry with the bundle's KustomizationName",
			build: func() *stack.Bundle {
				web := srBundle("web")
				web.KustomizationName = "shared"
				web.NamedDependsOn = []string{"db", "shared"}
				return web
			},
			want: []string{`"web"`, `"shared"`, "NamedDependsOn"},
		},
		{
			name: "NamedDependsOn entry with the bundle's Name, no KustomizationName",
			build: func() *stack.Bundle {
				web := srBundle("web")
				web.NamedDependsOn = []string{"web"}
				return web
			},
			want: []string{`"web"`, "NamedDependsOn"},
		},
		{
			name: "the bundle itself in DependsOn",
			build: func() *stack.Bundle {
				web := srBundle("web")
				web.DependsOn = []*stack.Bundle{web}
				return web
			},
			want: []string{`"web"`, "DependsOn"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs, err := fluxstack.NewResourceGenerator().GenerateForBundle(tt.build(), "clusters/demo/x")
			if err == nil {
				t.Fatalf("GenerateForBundle returned %d objects for a Kustomization that refers to itself", len(objs))
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %s", err, want)
				}
			}
		})
	}
}
