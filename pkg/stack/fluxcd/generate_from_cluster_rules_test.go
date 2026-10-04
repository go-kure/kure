package fluxcd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for GenerateFromCluster walking with the caller's layout rules
// (go-kure/kure#979): every spec.path it returns is a directory the same
// rules write, whatever the writer.

// unnamedRoot is a cluster whose root node has no name: under the default
// rules its content sits at "cluster", under a ClusterName in that directory.
func unnamedRoot() *stack.Cluster {
	web := &stack.Node{Name: "web", Bundle: srBundle("web", cmApp("web-app"))}
	root := &stack.Node{Bundle: srBundle("platform", cmApp("platform-app")), Children: []*stack.Node{web}}
	return &stack.Cluster{Name: "demo", Node: root}
}

var callerRulesCases = map[string]struct {
	build func() *stack.Cluster
	rules func() layout.LayoutRules
}{
	"default rules": {
		build: func() *stack.Cluster { return threeTier("platform", "apps", "web", nil) },
		rules: layout.DefaultLayoutRules,
	},
	"unnamed root, ClusterName .": {
		build: unnamedRoot,
		rules: func() layout.LayoutRules {
			r := layout.DefaultLayoutRules()
			r.ClusterName = "."
			return r
		},
	},
	"ClusterName clusters/prod": {
		build: func() *stack.Cluster { return threeTier("platform", "apps", "web", nil) },
		rules: func() layout.LayoutRules {
			r := layout.DefaultLayoutRules()
			r.ClusterName = "clusters/prod"
			return r
		},
	},
	"flat nodes": {
		build: func() *stack.Cluster { return threeTier("platform", "apps", "web", nil) },
		rules: func() layout.LayoutRules { return propertyGroupings["nodeFlat"] },
	},
}

// specPaths maps every Flux Kustomization in objs to its spec.path.
func specPaths(objs []client.Object) map[string]string {
	out := map[string]string{}
	for _, o := range objs {
		if k, ok := o.(*kustv1.Kustomization); ok {
			out[k.Name] = k.Spec.Path
		}
	}
	return out
}

// TestGenerateFromCluster_UsesCallerRules: the spec.path of every Kustomization
// is the directory a walk with the same rules writes its bundles to. The tree
// is written to disk and to a tar archive, which hold the same files, and
// every spec.path is a directory with a kustomization.yaml in both.
func TestGenerateFromCluster_UsesCallerRules(t *testing.T) {
	for name, tc := range callerRulesCases {
		t.Run(name, func(t *testing.T) {
			c := tc.build()
			ml, err := layout.WalkCluster(c, tc.rules())
			if err != nil {
				t.Fatalf("WalkCluster: %v", err)
			}
			ix, err := layout.IndexOrigins(ml, c)
			if err != nil {
				t.Fatalf("IndexOrigins: %v", err)
			}
			want := map[string]string{}
			for _, b := range reachableBundles(c) {
				want[ix.UnitName(b)] = ix.BundleLayout(b).FullRepoPath()
			}

			// A fresh cluster: walking mutates bundles.
			objs, err := fluxstack.NewResourceGenerator().GenerateFromCluster(tc.build(), tc.rules())
			if err != nil {
				t.Fatalf("GenerateFromCluster: %v", err)
			}
			got := specPaths(objs)
			if !mapsEqual(got, want) {
				t.Fatalf("GenerateFromCluster paths = %v, want the walked directories %v", got, want)
			}

			trees := writeAll(t, ml)
			for writer, w := range trees {
				for cr, p := range got {
					if _, err := os.Stat(filepath.Join(w.root, p, "kustomization.yaml")); err != nil {
						t.Errorf("%s: Kustomization %s has spec.path %q, which has no kustomization.yaml", writer, cr, p)
					}
				}
			}
			disk, tar := treeFiles(t, trees["WriteToDisk"].root), treeFiles(t, trees["WriteToTar"].root)
			if len(disk) == 0 {
				t.Fatal("WriteToDisk wrote no file")
			}
			if len(disk) != len(tar) {
				t.Errorf("WriteToDisk wrote %d files, WriteToTar %d", len(disk), len(tar))
			}
			for p, content := range disk {
				if other, ok := tar[p]; !ok || !bytes.Equal(content, other) {
					t.Errorf("%s differs between WriteToDisk and WriteToTar", p)
				}
			}
		})
	}
}

// TestGenerateFromCluster_UnnamedRootUnderClusterName pins the reported case:
// an unnamed root under ClusterName "." is written at the root of the tree,
// so no path names the "cluster" directory of a walk without a ClusterName.
func TestGenerateFromCluster_UnnamedRootUnderClusterName(t *testing.T) {
	rules := layout.DefaultLayoutRules()
	rules.ClusterName = "."
	objs, err := fluxstack.NewResourceGenerator().GenerateFromCluster(unnamedRoot(), rules)
	if err != nil {
		t.Fatalf("GenerateFromCluster: %v", err)
	}
	paths := specPaths(objs)
	if len(paths) == 0 {
		t.Fatal("no Kustomization generated")
	}
	for cr, p := range paths {
		if p == "cluster" || strings.HasPrefix(p, "cluster/") {
			t.Errorf("Kustomization %s has spec.path %q: the default rules' directory, not the caller's", cr, p)
		}
	}
}

// TestGenerateFromCluster_RefusesPerLayout: under FluxIntegratedPerLayout a
// written tree lists no directory child in its parent's kustomization.yaml,
// and only the integrator creates the Kustomization that applies one. A list
// of the bundle directories' Kustomizations would leave those children applied
// by nothing, so the generator and the engine refuse the placement and name
// the entry point that places the per-layout Kustomizations.
func TestGenerateFromCluster_RefusesPerLayout(t *testing.T) {
	byName := layout.DefaultLayoutRules()
	byName.ApplicationGrouping = layout.GroupByName
	rules := byName
	rules.FluxPlacement = layout.FluxIntegratedPerLayout
	build := func() *stack.Cluster { return threeTier("platform", "apps", "web", nil) }

	// The case the refusal is for: written to disk and to tar, the tree these
	// rules walk leaves directories out of their parents' kustomization.yaml
	// that the separate placement's tree lists (the application directories).
	unlisted := func(r layout.LayoutRules) map[string]int {
		ml, err := layout.WalkCluster(build(), r)
		if err != nil {
			t.Fatalf("WalkCluster: %v", err)
		}
		out := map[string]int{}
		for writer, w := range writeAll(t, ml) {
			files := treeFiles(t, w.root)
			for p := range files {
				dir := filepath.Dir(p)
				parent, ok := files[filepath.Join(filepath.Dir(dir), "kustomization.yaml")]
				if filepath.Base(p) != "kustomization.yaml" || !ok {
					continue
				}
				if !bytes.Contains(parent, []byte("- "+filepath.Base(dir)+"\n")) {
					out[writer]++
				}
			}
		}
		return out
	}
	separate, perLayout := unlisted(byName), unlisted(rules)
	for _, writer := range []string{"WriteToDisk", "WriteToTar"} {
		if perLayout[writer] <= separate[writer] {
			t.Fatalf("%s: the per-layout tree leaves %d directories unlisted, the separate tree %d: want more, or the list would be complete",
				writer, perLayout[writer], separate[writer])
		}
	}

	// The placement is refused whatever the groupings: which directories a
	// per-layout tree leaves unlisted depends on the cluster's content.
	flat := layout.DefaultLayoutRules()
	flat.FluxPlacement = layout.FluxIntegratedPerLayout
	calls := map[string]func() ([]client.Object, error){
		"generator": func() ([]client.Object, error) {
			return fluxstack.NewResourceGenerator().GenerateFromCluster(build(), rules)
		},
		"generator, flat applications": func() ([]client.Object, error) {
			return fluxstack.NewResourceGenerator().GenerateFromCluster(build(), flat)
		},
		"engine": func() ([]client.Object, error) {
			return fluxstack.NewWorkflowEngine().GenerateFromCluster(build(), rules)
		},
	}
	for name, call := range calls {
		objs, err := call()
		if err == nil || !strings.Contains(err.Error(), "FluxPlacement") || !strings.Contains(err.Error(), "CreateLayoutWithResources") {
			t.Errorf("%s: got %v, want a refusal of the placement that names CreateLayoutWithResources", name, err)
		}
		if objs != nil {
			t.Errorf("%s: got %d objects with the refusal", name, len(objs))
		}
	}
}

// TestGenerateFromCluster_PerBundleMatchesSeparate: a per-bundle tree lists
// every application directory in its bundle's kustomization.yaml, so the
// bundle directories' Kustomizations reconcile it and the placement changes
// nothing in the returned list.
func TestGenerateFromCluster_PerBundleMatchesSeparate(t *testing.T) {
	build := func() *stack.Cluster { return threeTier("platform", "apps", "web", nil) }
	separate, err := fluxstack.NewResourceGenerator().GenerateFromCluster(build(), layout.DefaultLayoutRules())
	if err != nil {
		t.Fatal(err)
	}
	rules := layout.DefaultLayoutRules()
	rules.FluxPlacement = layout.FluxIntegratedPerBundle
	perBundle, err := fluxstack.NewResourceGenerator().GenerateFromCluster(build(), rules)
	if err != nil {
		t.Fatalf("GenerateFromCluster: %v", err)
	}
	if len(perBundle) != len(separate) || !mapsEqual(specPaths(perBundle), specPaths(separate)) {
		t.Errorf("per-bundle list = %v (%d objects), want the separate placement's %v (%d objects)",
			specPaths(perBundle), len(perBundle), specPaths(separate), len(separate))
	}
}

// notLayoutRules satisfies stack.LayoutRulesProvider without being
// layout.LayoutRules.
type notLayoutRules struct{}

func (notLayoutRules) Validate() error { return nil }

// TestWorkflowEngine_GenerateFromCluster_Rules: the engine hands the caller's
// rules to the generator, and refuses rules that are not layout.LayoutRules
// (nil included) instead of walking with defaults.
func TestWorkflowEngine_GenerateFromCluster_Rules(t *testing.T) {
	rules := layout.DefaultLayoutRules()
	rules.ClusterName = "clusters/prod"
	build := func() *stack.Cluster { return threeTier("platform", "apps", "web", nil) }

	want, err := fluxstack.NewResourceGenerator().GenerateFromCluster(build(), rules)
	if err != nil {
		t.Fatal(err)
	}
	got, err := fluxstack.NewWorkflowEngine().GenerateFromCluster(build(), rules)
	if err != nil {
		t.Fatalf("GenerateFromCluster: %v", err)
	}
	if !mapsEqual(specPaths(got), specPaths(want)) {
		t.Errorf("engine paths = %v, want the generator's %v", specPaths(got), specPaths(want))
	}
	for cr, p := range specPaths(got) {
		if !strings.HasPrefix(p, "clusters/prod/") {
			t.Errorf("Kustomization %s has spec.path %q, want it under the caller's ClusterName", cr, p)
		}
	}

	for name, provider := range map[string]stack.LayoutRulesProvider{"nil": nil, "another type": notLayoutRules{}} {
		objs, err := fluxstack.NewWorkflowEngine().GenerateFromCluster(build(), provider)
		if err == nil || !strings.Contains(err.Error(), "rules must be of type layout.LayoutRules") {
			t.Errorf("%s rules: got %v, want the rules-type refusal", name, err)
		}
		if objs != nil {
			t.Errorf("%s rules: got %d objects with the refusal", name, len(objs))
		}
	}
}
