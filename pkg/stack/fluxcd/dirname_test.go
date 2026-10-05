package fluxcd_test

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for Bundle.DirName in the Flux workflow: the bundle's directory takes
// that name, its Kustomization's spec.path follows the directory, and the
// Kustomization's name and every reference to it do not.

// orderedShopCluster is platform -> apps, whose bundle shop (DirName 10-shop)
// is an umbrella over shop-infra (DirName 00-infra) and shop-services (DirName
// 01-services, depends on shop-infra). dirNames overrides the DirName per
// bundle name.
func orderedShopCluster(dirNames map[string]string) *stack.Cluster {
	bundle := func(name, dirName string) *stack.Bundle {
		b := srBundle(name, cmApp(name+"-app"))
		b.DirName = dirName
		if override, ok := dirNames[name]; ok {
			b.DirName = override
		}
		return b
	}
	infra := bundle("shop-infra", "00-infra")
	services := bundle("shop-services", "01-services")
	services.DependsOn = []*stack.Bundle{infra}
	shop := bundle("shop", "10-shop")
	shop.Children = []*stack.Bundle{infra, services}
	apps := &stack.Node{Name: "apps", Bundle: shop}
	root := &stack.Node{Name: "platform", Bundle: bundle("platform", ""), Children: []*stack.Node{apps}}
	apps.SetParent(root)
	return &stack.Cluster{Name: "demo", Node: root}
}

// TestDirName_DirectoryAndKustomizationPath: in every placement an umbrella
// child with DirName 00-infra and Name shop-infra is rendered at
// <umbrella>/00-infra and applied by the Kustomization shop-infra, whose
// spec.path is that directory. Names and references are those of the bundles.
// The umbrella's own DirName applies where it has a directory of its own
// (BundleGrouping GroupByName) and nowhere else. Every writer writes the
// directory, none writes one named after the bundle, and the tar archive holds
// the same files with the same content as the directory on disk.
func TestDirName_DirectoryAndKustomizationPath(t *testing.T) {
	shopDirs := map[string]string{"nodeOnly": "platform/apps", "GroupByName": "platform/apps/10-shop"}
	for grouping, shopDir := range shopDirs {
		for _, placement := range allPlacements {
			t.Run(grouping+"/"+string(placement), func(t *testing.T) {
				rules := propertyGroupings[grouping]
				rules.FluxPlacement = placement
				ml := integrated(t, orderedShopCluster(nil), rules)
				got := kustomizationsByName(ml)

				wantPaths := map[string]string{
					"shop":          shopDir,
					"shop-infra":    shopDir + "/00-infra",
					"shop-services": shopDir + "/01-services",
				}
				for name, want := range wantPaths {
					k, ok := got[name]
					if !ok {
						t.Fatalf("no Kustomization named %q; have %v", name, slices.Sorted(maps.Keys(got)))
					}
					if k.Spec.Path != want {
						t.Errorf("Kustomization %q has spec.path %q, want %q", name, k.Spec.Path, want)
					}
				}
				for _, dirName := range []string{"00-infra", "01-services", "10-shop"} {
					if _, ok := got[dirName]; ok {
						t.Errorf("a Kustomization is named %q, a directory name", dirName)
					}
				}
				if deps := dependsOnNames(got["shop-services"]); !slices.Equal(deps, []string{"shop-infra"}) {
					t.Errorf("shop-services dependsOn = %v, want [shop-infra]", deps)
				}
				if checks := kustomizationHealthChecks(got["shop"]); !slices.Equal(checks, []string{"shop-infra", "shop-services"}) {
					t.Errorf("shop health checks = %v, want [shop-infra shop-services]", checks)
				}

				written := writeAll(t, ml)
				for writer, tree := range written {
					for _, dir := range []string{shopDir + "/00-infra", shopDir + "/01-services"} {
						if _, err := os.Stat(filepath.Join(tree.root, dir, "kustomization.yaml")); err != nil {
							t.Errorf("%s: no kustomization.yaml in %q: %v", writer, dir, err)
						}
					}
					for _, dir := range []string{shopDir + "/shop-infra", shopDir + "/shop-services", "platform/apps/shop"} {
						if _, err := os.Stat(filepath.Join(tree.root, dir)); err == nil {
							t.Errorf("%s wrote %q, a directory named after a bundle's Name", writer, dir)
						}
					}
				}
				disk, tarball := treeBytes(t, written["WriteToDisk"].root), treeBytes(t, written["WriteToTar"].root)
				if !bytes.Equal(disk, tarball) {
					t.Errorf("WriteToDisk and WriteToTar differ\n\ndisk:\n%s\ntar:\n%s", disk, tarball)
				}
			})
		}
	}
}

// TestDirName_GenerateFromCluster: the default-rules entry point derives each
// spec.path from the same directories.
func TestDirName_GenerateFromCluster(t *testing.T) {
	objs, err := fluxstack.NewResourceGenerator().GenerateFromCluster(orderedShopCluster(nil), layout.DefaultLayoutRules())
	if err != nil {
		t.Fatalf("GenerateFromCluster: %v", err)
	}
	got := map[string]string{}
	for _, o := range objs {
		if k, ok := o.(*kustv1.Kustomization); ok {
			got[k.Name] = k.Spec.Path
		}
	}
	// The root node's bundle sets no DirName: its directory inside the root
	// node's is named after it.
	want := map[string]string{
		"platform":      "platform/platform",
		"shop":          "platform/apps",
		"shop-infra":    "platform/apps/00-infra",
		"shop-services": "platform/apps/01-services",
	}
	if !maps.Equal(got, want) {
		t.Errorf("Kustomization paths = %v, want %v", got, want)
	}
}

// TestDirName_RootBundleDirectory: the directory the root node's bundle has
// inside the root node's under a flat BundleGrouping is named by its DirName
// too. In every placement the Kustomization keeps the bundle's name, its
// spec.path is that directory, every writer writes it and none writes one
// named after the bundle, and disk and tar hold the same bytes.
func TestDirName_RootBundleDirectory(t *testing.T) {
	for _, placement := range allPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := propertyGroupings["nodeOnly"]
			rules.FluxPlacement = placement
			ml := integrated(t, orderedShopCluster(map[string]string{"platform": "00-platform"}), rules)
			got := kustomizationsByName(ml)
			k, ok := got["platform"]
			if !ok {
				t.Fatalf("no Kustomization named %q; have %v", "platform", slices.Sorted(maps.Keys(got)))
			}
			if k.Spec.Path != "platform/00-platform" {
				t.Errorf("Kustomization %q has spec.path %q, want %q", "platform", k.Spec.Path, "platform/00-platform")
			}
			if _, ok := got["00-platform"]; ok {
				t.Error(`a Kustomization is named "00-platform", a directory name`)
			}
			written := writeAll(t, ml)
			for writer, tree := range written {
				if _, err := os.Stat(filepath.Join(tree.root, "platform", "00-platform", "kustomization.yaml")); err != nil {
					t.Errorf("%s: no kustomization.yaml in %q: %v", writer, "platform/00-platform", err)
				}
				if _, err := os.Stat(filepath.Join(tree.root, "platform", "platform")); err == nil {
					t.Errorf("%s wrote %q, a directory named after the bundle's Name", writer, "platform/platform")
				}
			}
			disk, tarball := treeBytes(t, written["WriteToDisk"].root), treeBytes(t, written["WriteToTar"].root)
			if !bytes.Equal(disk, tarball) {
				t.Errorf("WriteToDisk and WriteToTar differ\n\ndisk:\n%s\ntar:\n%s", disk, tarball)
			}
		})
	}
}

// TestDirName_MergedDirectory: bundles a flat grouping merges into the root
// node share one directory and one Kustomization. The Kustomization is named
// after the first of them and the directory by that bundle's DirName; a
// DirName on a later one has no effect.
func TestDirName_MergedDirectory(t *testing.T) {
	tests := map[string]struct {
		edit func(rb, b1, b2 *stack.Bundle)
		want string
	}{
		"first sets it":            {func(rb, _, _ *stack.Bundle) { rb.DirName = "00-r" }, "r/00-r"},
		"all set it":               {func(rb, b1, b2 *stack.Bundle) { rb.DirName, b1.DirName, b2.DirName = "00-r", "10-one", "20-two" }, "r/00-r"},
		"only later ones set it":   {func(_, b1, b2 *stack.Bundle) { b1.DirName, b2.DirName = "10-one", "20-two" }, "r/rb"},
		"none sets it, as on main": {nil, "r/rb"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			kusts, err := generateUnits(t, mergedCluster(tt.edit), allFlat)
			if err != nil {
				t.Fatalf("GenerateFromLayout: %v", err)
			}
			if len(kusts) != 1 {
				t.Fatalf("got %d Kustomizations, want one for the one written directory", len(kusts))
			}
			if k := kusts[0]; k.Name != "rb" || k.Spec.Path != tt.want {
				t.Errorf("Kustomization %s at %q, want rb at %q", k.Name, k.Spec.Path, tt.want)
			}
		})
	}
}

// TestDirName_FluxDirectoryReserved: under FluxSeparate the Flux resources'
// directory must be free, and the check compares the directory the root
// node's bundle gets. A DirName that names it is refused, also in another
// case (a DirName is no object name and may hold upper case), and the error
// names the bundle by its Name. A bundle named like that directory is written
// once its DirName names another.
func TestDirName_FluxDirectoryReserved(t *testing.T) {
	rules := propertyGroupings["nodeOnly"]
	rules.FluxPlacement = layout.FluxSeparate
	li := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	for _, dirName := range []string{"flux-system", "Flux-System"} {
		want := `bundle "platform" is rendered to "platform/` + dirName + `", the directory the Flux resources are written to`
		_, err := li.CreateLayoutWithResources(orderedShopCluster(map[string]string{"platform": dirName}), rules)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("DirName %q: got %v, want the refusal %q", dirName, err, want)
		}
		if err != nil && !strings.Contains(err.Error(), "DirName") {
			t.Errorf("DirName %q: error %q does not say the directory is named by DirName", dirName, err)
		}
	}

	c := orderedShopCluster(map[string]string{"platform": "00-platform"})
	c.Node.Bundle.Name = "flux-system"
	ml, err := li.CreateLayoutWithResources(c, rules)
	if err != nil {
		t.Fatalf("bundle flux-system with DirName 00-platform: %v", err)
	}
	if err := ml.WriteToDisk(t.TempDir()); err != nil {
		t.Errorf("bundle flux-system with DirName 00-platform, WriteToDisk: %v", err)
	}
}

// TestDirName_UnsetOutputUnchanged: a cluster that sets no DirName keeps
// producing the trees stored under testdata, which predate the field, byte
// for byte (TestKustomizationName_UnsetOutputUnchanged writes the same
// cluster and compares with the same files). A DirName equal to the bundle's
// Name is the unset case spelled out, and produces the same bytes.
func TestDirName_UnsetOutputUnchanged(t *testing.T) {
	cases := map[string]layout.LayoutRules{"merged": allFlat}
	for _, placement := range allPlacements {
		rules := propertyGroupings["GroupByName"]
		rules.FluxPlacement = placement
		cases[string(placement)] = rules
	}
	var setToName func(b *stack.Bundle)
	setToName = func(b *stack.Bundle) {
		b.DirName = b.Name
		for _, child := range b.Children {
			setToName(child)
		}
	}
	var eachNode func(n *stack.Node, do func(b *stack.Bundle))
	eachNode = func(n *stack.Node, do func(b *stack.Bundle)) {
		if n.Bundle != nil {
			do(n.Bundle)
		}
		for _, child := range n.Children {
			eachNode(child, do)
		}
	}
	var assertUnset func(b *stack.Bundle)
	assertUnset = func(b *stack.Bundle) {
		if b.DirName != "" {
			t.Errorf("the stored trees are those of a cluster without DirName; bundle %q sets %q", b.Name, b.DirName)
		}
		for _, child := range b.Children {
			assertUnset(child)
		}
	}
	for name, rules := range cases {
		stored := filepath.Join("testdata", "unset-kustomization-name", name+".txt")
		t.Run(name+"/unset", func(t *testing.T) {
			c := unnamedCluster()
			eachNode(c.Node, assertUnset)
			dir := t.TempDir()
			if err := integrated(t, c, rules).WriteToDisk(dir); err != nil {
				t.Fatalf("WriteToDisk: %v", err)
			}
			compareWithStored(t, stored, treeBytes(t, dir))
		})
		t.Run(name+"/equal to Name", func(t *testing.T) {
			c := unnamedCluster()
			eachNode(c.Node, setToName)
			dir := t.TempDir()
			if err := integrated(t, c, rules).WriteToDisk(dir); err != nil {
				t.Fatalf("WriteToDisk: %v", err)
			}
			compareWithStored(t, stored, treeBytes(t, dir))
		})
	}
}

// TestDirName_SameDirectoryRefused: two umbrella children with one directory
// name are refused in every placement before any Kustomization is generated,
// and by the entry point that only generates them; the error names both
// bundles by their paths and the directory they share.
func TestDirName_SameDirectoryRefused(t *testing.T) {
	cluster := func() *stack.Cluster {
		return orderedShopCluster(map[string]string{"shop-services": "00-infra"})
	}
	check := func(t *testing.T, what string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s accepted two bundles with one directory", what)
		}
		for _, want := range []string{`"shop/shop-infra"`, `"shop/shop-services"`, `"platform/apps/00-infra"`} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: error %q does not contain %s", what, err, want)
			}
		}
	}
	for _, placement := range allPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := propertyGroupings["nodeOnly"]
			rules.FluxPlacement = placement
			_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(cluster(), rules)
			check(t, "CreateLayoutWithResources", err)
		})
	}
	t.Run("GenerateFromCluster", func(t *testing.T) {
		_, err := fluxstack.NewResourceGenerator().GenerateFromCluster(cluster(), layout.DefaultLayoutRules())
		check(t, "GenerateFromCluster", err)
	})
}

// TestDirName_RenamedLayoutRefused: renaming a bundle's layout between the walk
// and the integration, which used to give the directory another name than the
// Kustomization, is refused in every placement and points at DirName.
func TestDirName_RenamedLayoutRefused(t *testing.T) {
	for _, placement := range allPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := propertyGroupings["nodeOnly"]
			rules.FluxPlacement = placement
			c := orderedShopCluster(map[string]string{"shop-infra": "", "shop-services": "", "shop": ""})
			ml, err := layout.WalkCluster(c, rules)
			if err != nil {
				t.Fatalf("WalkCluster: %v", err)
			}
			layoutAtPath(t, ml, "platform/apps/shop-infra").Name = "00-infra"
			err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
			if err == nil {
				t.Fatal("IntegrateWithLayout accepted a renamed bundle layout")
			}
			for _, want := range []string{`"shop/shop-infra"`, `"00-infra"`, "DirName"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %s", err, want)
				}
			}
		})
	}
}
