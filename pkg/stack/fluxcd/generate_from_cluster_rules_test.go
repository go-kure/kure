package fluxcd_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kerrors "github.com/go-kure/kure/pkg/errors"
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

// TestFluxSeparate_UnnamedRootBuildAppliesNoBundle: an unnamed root is
// rendered into the ClusterName directory, and its bundle one directory lower
// (go-kure/kure#979). That directory keeps its kustomization.yaml in every
// writer, listing the Flux directory only: without one, the Kustomization that
// builds it would take in every file below, and apply each bundle's objects
// beside the bundle's own Kustomization.
func TestFluxSeparate_UnnamedRootBuildAppliesNoBundle(t *testing.T) {
	for _, clusterName := range []string{".", "prod"} {
		t.Run(clusterName, func(t *testing.T) {
			rules := layout.DefaultLayoutRules()
			rules.FluxPlacement = layout.FluxSeparate
			rules.ClusterName = clusterName
			ml := integrated(t, unnamedRoot(), rules)
			for writer, w := range writeAll(t, ml) {
				top := filepath.Join(w.root, ml.FullRepoPath(), "kustomization.yaml")
				data, err := os.ReadFile(top)
				if err != nil {
					t.Errorf("%s: the root node's directory has no kustomization.yaml: %v", writer, err)
					continue
				}
				if !bytes.Contains(data, []byte("- "+fluxstack.DefaultFluxDirName+"\n")) {
					t.Errorf("%s: the root kustomization.yaml does not list %s:\n%s", writer, fluxstack.DefaultFluxDirName, data)
				}
				objs := fluxBuild(t, w.root, ml.FullRepoPath(), unstructured.Unstructured{Object: map[string]any{}})
				crs := 0
				for id := range objs {
					if strings.Contains(id, "ConfigMap") {
						t.Errorf("%s: the build of the root node's directory applies %s, a bundle's object", writer, id)
					}
					if strings.Contains(id, "Kustomization") {
						crs++
					}
				}
				if crs != 2 {
					t.Errorf("%s: the build of the root node's directory holds %d Kustomizations, want the two bundles': %v", writer, crs, keys(objs))
				}
			}
		})
	}
}

// TestFluxSeparate_RefusesLayoutInTheFluxDirectory: the root node's bundle is
// rendered in a directory named after it, inside the root node's
// (go-kure/kure#979), and a child node in one named after the node. Named like
// the Flux directory, either would share it with the Flux resources, so the
// integration refuses it before anything is written. The integrated placements
// have no Flux directory, and write the tree.
func TestFluxSeparate_RefusesLayoutInTheFluxDirectory(t *testing.T) {
	bundleNamed := func(name string) *stack.Cluster { return threeTier(name, "apps", "web", nil) }
	nodeNamed := func(name string) *stack.Cluster {
		c := threeTier("platform", "apps", "web", nil)
		c.Node.Children[0].Name = name
		return c
	}
	rules := layout.DefaultLayoutRules()
	rules.FluxPlacement = layout.FluxSeparate
	for name, tc := range map[string]struct {
		build func() *stack.Cluster
		want  string
	}{
		"root bundle":            {func() *stack.Cluster { return bundleNamed("flux-system") }, `bundle "flux-system" is rendered to "platform/flux-system", the directory the Flux resources are written to`},
		"root bundle, case only": {func() *stack.Cluster { return bundleNamed("Flux-System") }, `bundle "Flux-System" is rendered to "platform/Flux-System", the directory the Flux resources are written to`},
		"child node":             {func() *stack.Cluster { return nodeNamed("flux-system") }, `node "flux-system" is rendered to "platform/flux-system", the directory the Flux resources are written to`},
		// A name that resolves to the same directory is the same directory.
		"child node, rooted name":    {func() *stack.Cluster { return nodeNamed("/flux-system") }, `node "/flux-system" is rendered to "platform/flux-system", the directory the Flux resources are written to`},
		"child node, dot-slash name": {func() *stack.Cluster { return nodeNamed("./flux-system") }, `node "./flux-system" is rendered to "platform/flux-system", the directory the Flux resources are written to`},
	} {
		t.Run(name, func(t *testing.T) {
			li := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
			if _, err := li.CreateLayoutWithResources(tc.build(), rules); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("CreateLayoutWithResources: got %v, want the refusal %q", err, tc.want)
			}
			c := tc.build()
			ml, err := layout.WalkCluster(c, rules)
			if err != nil {
				t.Fatalf("WalkCluster: %v", err)
			}
			before := len(ml.Children)
			if err := li.IntegrateWithLayout(ml, c, rules); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("IntegrateWithLayout: got %v, want the refusal %q", err, tc.want)
			}
			if len(ml.Children) != before {
				t.Errorf("the refused integration left %d children, the walk %d", len(ml.Children), before)
			}
		})
	}
	for _, placement := range []layout.FluxPlacement{layout.FluxIntegratedPerBundle, layout.FluxIntegratedPerLayout} {
		t.Run(string(placement), func(t *testing.T) {
			r := rules
			r.FluxPlacement = placement
			writeAll(t, integrated(t, bundleNamed("flux-system"), r))
		})
	}
	// A rooted ClusterName that ends in the root node's name: the writers
	// resolve it under their output directory, and so does the refusal.
	t.Run("rooted ClusterName", func(t *testing.T) {
		r := rules
		r.ClusterName = "/platform"
		const want = `node "flux-system" is rendered to`
		li := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
		if _, err := li.CreateLayoutWithResources(nodeNamed("flux-system"), r); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("CreateLayoutWithResources: got %v, want the refusal %q", err, want)
		}
	})
	// The comparison is the writers': a name they write to a directory of its
	// own is not refused. U+017F equals "s" under Unicode case folding and
	// is not its lower case, which is what the writers compare.
	t.Run("a name the writers keep apart", func(t *testing.T) {
		writeAll(t, integrated(t, nodeNamed("flux-ſystem"), rules))
	})
}

// unlistedDirs walks build() with rules, writes the tree with every layout
// writer and counts, per writer, the directories with a kustomization.yaml
// that their parent's kustomization.yaml does not list.
func unlistedDirs(t *testing.T, build func() *stack.Cluster, rules layout.LayoutRules) map[string]int {
	t.Helper()
	ml, err := layout.WalkCluster(build(), rules)
	if err != nil {
		t.Fatalf("WalkCluster: %v", err)
	}
	out := map[string]int{}
	for writer, w := range writeAll(t, ml) {
		files := treeFiles(t, w.root)
		out[writer] = 0
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
	separate, perLayout := unlistedDirs(t, build, byName), unlistedDirs(t, build, rules)
	for _, writer := range []string{"WriteToDisk", "WriteToTar"} {
		if perLayout[writer] <= separate[writer] {
			t.Fatalf("%s: the per-layout tree leaves %d directories unlisted, the separate tree %d: want more, or the list would be complete",
				writer, perLayout[writer], separate[writer])
		}
	}

	// The placement is refused whatever the groupings and the cluster, an
	// absent or empty one included: which directories a per-layout tree leaves
	// unlisted depends on the cluster's content.
	flat := layout.DefaultLayoutRules()
	flat.FluxPlacement = layout.FluxIntegratedPerLayout
	calls := map[string]func() ([]client.Object, error){
		"generator": func() ([]client.Object, error) {
			return fluxstack.NewResourceGenerator().GenerateFromCluster(build(), rules)
		},
		"generator, flat applications": func() ([]client.Object, error) {
			return fluxstack.NewResourceGenerator().GenerateFromCluster(build(), flat)
		},
		"generator, nil cluster": func() ([]client.Object, error) {
			return fluxstack.NewResourceGenerator().GenerateFromCluster(nil, rules)
		},
		"generator, cluster without a root node": func() ([]client.Object, error) {
			return fluxstack.NewResourceGenerator().GenerateFromCluster(&stack.Cluster{Name: "empty"}, rules)
		},
		"engine": func() ([]client.Object, error) {
			return fluxstack.NewWorkflowEngine().GenerateFromCluster(build(), rules)
		},
		"engine, nil cluster": func() ([]client.Object, error) {
			return fluxstack.NewWorkflowEngine().GenerateFromCluster(nil, rules)
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

// TestGenerateFromCluster_PerBundleMatchesSeparate: with a directory per
// application, a per-bundle tree written to disk and to tar leaves no more
// directories unlisted than the separate placement's (the application
// directories are listed by their bundle's kustomization.yaml), so the bundle
// directories' Kustomizations reconcile it and the returned objects are those
// of the separate placement.
func TestGenerateFromCluster_PerBundleMatchesSeparate(t *testing.T) {
	build := func() *stack.Cluster { return threeTier("platform", "apps", "web", nil) }
	byName := layout.DefaultLayoutRules()
	byName.ApplicationGrouping = layout.GroupByName
	rules := byName
	rules.FluxPlacement = layout.FluxIntegratedPerBundle

	separateUnlisted, perBundleUnlisted := unlistedDirs(t, build, byName), unlistedDirs(t, build, rules)
	for _, writer := range []string{"WriteToDisk", "WriteToTar"} {
		if perBundleUnlisted[writer] != separateUnlisted[writer] {
			t.Errorf("%s: the per-bundle tree leaves %d directories unlisted, the separate tree %d",
				writer, perBundleUnlisted[writer], separateUnlisted[writer])
		}
	}

	separate, err := fluxstack.NewResourceGenerator().GenerateFromCluster(build(), byName)
	if err != nil {
		t.Fatal(err)
	}
	perBundle, err := fluxstack.NewResourceGenerator().GenerateFromCluster(build(), rules)
	if err != nil {
		t.Fatalf("GenerateFromCluster: %v", err)
	}
	if len(separate) == 0 {
		t.Fatal("no object generated")
	}
	if !reflect.DeepEqual(perBundle, separate) {
		t.Errorf("per-bundle objects differ from the separate placement's:\n got %v\nwant %v", specPaths(perBundle), specPaths(separate))
	}
}

// invalidRules are rules LayoutRules.Validate refuses, by the field its error
// names: one unknown option value each, and a ClusterName with a ".." segment.
func invalidRules() map[string]layout.LayoutRules {
	out := map[string]layout.LayoutRules{}
	for field, set := range map[string]func(*layout.LayoutRules){
		"NodeGrouping":        func(r *layout.LayoutRules) { r.NodeGrouping = "sideways" },
		"BundleGrouping":      func(r *layout.LayoutRules) { r.BundleGrouping = "sideways" },
		"ApplicationGrouping": func(r *layout.LayoutRules) { r.ApplicationGrouping = "sideways" },
		"FilePer":             func(r *layout.LayoutRules) { r.FilePer = "sideways" },
		"FluxPlacement":       func(r *layout.LayoutRules) { r.FluxPlacement = "sideways" },
		"FileNaming":          func(r *layout.LayoutRules) { r.FileNaming = "sideways" },
		"ClusterName":         func(r *layout.LayoutRules) { r.ClusterName = "a/../b" },
	} {
		rules := layout.DefaultLayoutRules()
		set(&rules)
		out[field] = rules
	}
	return out
}

// TestGenerateFromCluster_InvalidRules: rules the walk refuses are an error
// from the generator and the engine whatever the cluster is, an absent or
// empty one included, and no object comes back. The error carries the rules'
// validation error, which names the field.
func TestGenerateFromCluster_InvalidRules(t *testing.T) {
	clusters := map[string]func() *stack.Cluster{
		"cluster with bundles":        func() *stack.Cluster { return threeTier("platform", "apps", "web", nil) },
		"nil cluster":                 func() *stack.Cluster { return nil },
		"cluster without a root node": func() *stack.Cluster { return &stack.Cluster{Name: "empty"} },
	}
	for field, rules := range invalidRules() {
		if err := rules.Validate(); err == nil {
			t.Fatalf("%s: the rules are valid, so the case tests nothing", field)
		}
		for clusterName, build := range clusters {
			calls := map[string]func() ([]client.Object, error){
				"generator": func() ([]client.Object, error) {
					return fluxstack.NewResourceGenerator().GenerateFromCluster(build(), rules)
				},
				"engine": func() ([]client.Object, error) {
					return fluxstack.NewWorkflowEngine().GenerateFromCluster(build(), rules)
				},
			}
			for caller, call := range calls {
				objs, err := call()
				var verr *kerrors.ValidationError
				if !errors.As(err, &verr) || verr.Field != field {
					t.Errorf("%s, %s, invalid %s: got %v, want the rules' validation error for that field", caller, clusterName, field, err)
				}
				if objs != nil {
					t.Errorf("%s, %s, invalid %s: got %d objects with the refusal", caller, clusterName, field, len(objs))
				}
			}
		}
	}

	// Valid rules: an absent or empty cluster yields nothing and no error.
	for clusterName, build := range clusters {
		if clusterName == "cluster with bundles" {
			continue
		}
		objs, err := fluxstack.NewResourceGenerator().GenerateFromCluster(build(), layout.DefaultLayoutRules())
		if err != nil || objs != nil {
			t.Errorf("%s, valid rules: got %v, %v; want nothing", clusterName, objs, err)
		}
	}

	// The per-layout placement is refused before the other rules are read:
	// with an invalid rule as well, the caller gets the placement refusal.
	for field, rules := range invalidRules() {
		if field == "FluxPlacement" {
			continue
		}
		rules.FluxPlacement = layout.FluxIntegratedPerLayout
		for clusterName, build := range clusters {
			_, err := fluxstack.NewResourceGenerator().GenerateFromCluster(build(), rules)
			if err == nil || !strings.Contains(err.Error(), "CreateLayoutWithResources") {
				t.Errorf("%s, per-layout with invalid %s: got %v, want the placement refusal", clusterName, field, err)
			}
		}
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
