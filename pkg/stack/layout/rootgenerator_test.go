package layout_test

import (
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for ConfigMapGenerators on a root the writer writes no
// kustomization.yaml for (go-kure/kure#899): the generators would be dropped,
// so every writer that writes none refuses them before writing anything.

// singleGenRoot is AppFileSingle root r carrying a configMapGenerator, with
// no resources and no children.
func singleGenRoot() *layout.ManifestLayout {
	r := &layout.ManifestLayout{
		Name:                "r",
		Namespace:           ".",
		Mode:                layout.KustomizationExplicit,
		ApplicationFileMode: layout.AppFileSingle,
	}
	return withGenerator(r, "x")
}

// clusterGenRoot is a layout shaped like the ClusterName directory at the top
// of a walked tree (Name "", a single-segment Namespace, no resources)
// carrying a configMapGenerator, with a directory child apps.
func clusterGenRoot() *layout.ManifestLayout {
	r := &layout.ManifestLayout{
		Name:                "",
		Namespace:           "cluster",
		Mode:                layout.KustomizationExplicit,
		ApplicationFileMode: layout.AppFilePerResource,
	}
	child(r, "apps", layout.KustomizationExplicit)
	r.ExtraFiles = append(r.ExtraFiles, layout.ExtraFile{Name: "values.txt", Content: []byte("k: v\n")})
	r.ConfigMapGenerators = append(r.ConfigMapGenerators, layout.ConfigMapGeneratorSpec{Name: "x", Files: []string{"values.txt"}})
	return r
}

// TestWriters_RefuseGeneratorsOnSingleRootWithoutFile: an AppFileSingle root
// with no resources and no children gets no kustomization.yaml from any
// writer, so each refuses its ConfigMapGenerators.
func TestWriters_RefuseGeneratorsOnSingleRootWithoutFile(t *testing.T) {
	want := `layout "r" gets no kustomization.yaml, so its ConfigMapGenerators have nowhere to go`
	for _, writer := range allWriters {
		err := writeRefused(t, writer, layout.DefaultLayoutConfig(), singleGenRoot())
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want it to contain %q", writer, err, want)
		}
		if err != nil && !strings.Contains(err.Error(), "r.yaml") {
			t.Errorf("%s: err = %v, want it to name the root's file r.yaml", writer, err)
		}
	}
}

// TestWriters_AcceptGeneratorsOnSingleRootWithFile: an AppFileSingle root
// with a resource writes a kustomization.yaml next to its file, and that
// kustomization.yaml generates the ConfigMap.
func TestWriters_AcceptGeneratorsOnSingleRootWithFile(t *testing.T) {
	tree := func() *layout.ManifestLayout {
		r := singleGenRoot()
		r.Resources = []client.Object{testObj("v1", "Secret", "r")}
		return r
	}
	for _, writer := range allWriters {
		files := writtenFiles(t, writer, layout.DefaultLayoutConfig(), tree())
		if k := files["kustomization.yaml"]; !strings.Contains(k, "- name: x") {
			t.Errorf("%s: kustomization.yaml does not generate x:\n%v", writer, files)
		}
	}
	kustomizeBuildsAll(t, tree())
}

// TestWriters_ClusterRootGeneratesConfigMap: every writer writes the cluster
// root a kustomization.yaml, also when it has no resources, and that file
// generates the root's ConfigMap. WriteManifest used to write none there and
// refused the generators instead (go-kure/kure#899); since it writes the file
// (go-kure/kure#979) nothing is dropped and nothing is refused. Each writer's
// output builds with kustomize.
func TestWriters_ClusterRootGeneratesConfigMap(t *testing.T) {
	for _, writer := range allWriters {
		files := writtenFiles(t, writer, layout.DefaultLayoutConfig(), clusterGenRoot())
		generated := false
		for name, content := range files {
			if strings.HasSuffix(name, "cluster/kustomization.yaml") && strings.Contains(content, "- name: x") {
				generated = true
			}
		}
		if !generated {
			t.Errorf("%s: no cluster/kustomization.yaml generates x:\n%v", writer, files)
		}
	}
	kustomizeBuildsAll(t, clusterGenRoot())
}

// TestWriters_ClusterRootBuildIsChecked: the cluster root's kustomization.yaml
// lists its children, so every writer holds the build that file starts to the
// checks every other directory with one gets. WriteManifest used to write no
// file into a cluster root that held nothing itself, and so wrote these trees
// (go-kure/kure#979); WriteToDisk and WriteToTar refused them. All but the last
// are built by hand; the last is walked.
func TestWriters_ClusterRootBuildIsChecked(t *testing.T) {
	root := func(children ...*layout.ManifestLayout) *layout.ManifestLayout {
		return &layout.ManifestLayout{Name: "", Namespace: "demo", Children: children}
	}
	cases := map[string]struct {
		tree func(t *testing.T) *layout.ManifestLayout
		want string
	}{
		// Children a and b each hold ConfigMap default/same: the root lists
		// both, and kustomize refuses one object twice in a build.
		"one object in two listed children": {
			tree: func(*testing.T) *layout.ManifestLayout {
				a, b := cmLayout("same", "demo"), cmLayout("same", "demo")
				a.Name, b.Name = "a", "b"
				return root(a, b)
			},
			want: `layouts "demo/a" and "demo/b" both hold the object v1 ConfigMap default/same, and the kustomization.yaml of layout "demo" builds both`,
		},
		// The root lists child a, and a KustomizationRecursive directory has
		// no kustomization.yaml for the entry to name.
		"listed KustomizationRecursive child": {
			tree: func(*testing.T) *layout.ManifestLayout {
				a := cmLayout("a", "demo")
				a.Mode = layout.KustomizationRecursive
				return root(a)
			},
			want: `layout "demo/a" is KustomizationRecursive and writes no kustomization.yaml, but the kustomization.yaml of layout "demo" lists its directory`,
		},
		// The child's directory is the path of the file the root writes.
		"child directory named kustomization.yaml": {
			tree: func(*testing.T) *layout.ManifestLayout {
				a := cmLayout("a", "demo")
				a.Name = "kustomization.yaml"
				return root(a)
			},
			want: `the directory of layout "demo/kustomization.yaml" would replace the kustomization.yaml of layout "demo"`,
		},
		// The same tree from a walk: a root node of that name below a
		// ClusterName is the directory <ClusterName>/kustomization.yaml.
		"root node named kustomization.yaml, walked": {
			tree: func(t *testing.T) *layout.ManifestLayout {
				t.Helper()
				rules := layout.DefaultLayoutRules()
				rules.ClusterName = "demo"
				ml, err := layout.WalkCluster(&stack.Cluster{Name: "demo", Node: &stack.Node{Name: "kustomization.yaml"}}, rules)
				if err != nil {
					t.Fatalf("WalkCluster: %v", err)
				}
				return ml
			},
			want: `the directory of layout "demo/kustomization.yaml" would replace the kustomization.yaml of layout "demo"`,
		},
	}
	for name, tc := range cases {
		for _, writer := range allWriters {
			t.Run(name+"/"+writer, func(t *testing.T) {
				err := writeRefused(t, writer, layout.Config{}, tc.tree(t))
				if err == nil {
					t.Fatal("the tree was written: every writer writes the cluster root a kustomization.yaml and must refuse it")
				}
				if !strings.Contains(err.Error(), tc.want) {
					t.Errorf("refusal does not hold %s:\n%v", tc.want, err)
				}
			})
		}
	}
}

// TestWriters_RefuseGeneratorsOnRecursiveSingleRoot: a KustomizationRecursive
// root writes no kustomization.yaml whether or not it is AppFileSingle, with or
// without resources, so every writer refuses its ConfigMapGenerators. So does
// WriteManifest when Config makes a root with unset modes both.
func TestWriters_RefuseGeneratorsOnRecursiveSingleRoot(t *testing.T) {
	want := `layout "r" is KustomizationRecursive, which writes no kustomization.yaml, so its ConfigMapGenerators have nowhere to go`
	cases := map[string]func() *layout.ManifestLayout{
		"without resources": func() *layout.ManifestLayout {
			r := singleGenRoot()
			r.Mode = layout.KustomizationRecursive
			return r
		},
		"with resources": func() *layout.ManifestLayout {
			r := singleGenRoot()
			r.Mode = layout.KustomizationRecursive
			r.Resources = []client.Object{testObj("v1", "Secret", "r")}
			return r
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			for _, writer := range allWriters {
				err := writeRefused(t, writer, layout.DefaultLayoutConfig(), build())
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Errorf("%s: err = %v, want it to contain %q", writer, err, want)
				}
				if err != nil && !strings.Contains(err.Error(), "r.yaml") {
					t.Errorf("%s: err = %v, want it to name the root's file r.yaml", writer, err)
				}
			}
		})
	}
	t.Run("from Config", func(t *testing.T) {
		r := singleGenRoot()
		r.Mode = layout.KustomizationUnset
		r.ApplicationFileMode = layout.AppFileUnset
		r.Resources = []client.Object{testObj("v1", "Secret", "r")}
		cfg := layout.DefaultLayoutConfig()
		cfg.ApplicationFileMode = layout.AppFileSingle
		cfg.KustomizationMode = layout.KustomizationRecursive
		err := writeRefused(t, "WriteManifest", cfg, r)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("WriteManifest: err = %v, want it to contain %q", err, want)
		}
	})
}

// TestWriteManifest_RefuseGeneratorsOnSingleRootFromConfig: WriteManifest
// writes a root whose own mode is unset AppFileSingle when Config says so, and
// with no resources and no children it gets no kustomization.yaml.
func TestWriteManifest_RefuseGeneratorsOnSingleRootFromConfig(t *testing.T) {
	r := singleGenRoot()
	r.ApplicationFileMode = layout.AppFileUnset
	cfg := layout.DefaultLayoutConfig()
	cfg.ApplicationFileMode = layout.AppFileSingle
	err := writeRefused(t, "WriteManifest", cfg, r)
	want := `layout "r" gets no kustomization.yaml, so its ConfigMapGenerators have nowhere to go`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("WriteManifest: err = %v, want it to contain %q", err, want)
	}
	if err != nil && !strings.Contains(err.Error(), "r.yaml") {
		t.Errorf("WriteManifest: err = %v, want it to name the root's file r.yaml", err)
	}
}

// TestWriteManifest_ClusterRootWithSingleChildGenerators: WriteManifest
// writes the cluster root a kustomization.yaml that generates the root's
// ConfigMap, whether its AppFileSingle child writes a file, which the root
// then lists, or has no resources and writes none. In the second case
// WriteManifest used to write the root no kustomization.yaml and refused its
// generators (go-kure/kure#979).
func TestWriteManifest_ClusterRootWithSingleChildGenerators(t *testing.T) {
	tree := func(withResource bool) *layout.ManifestLayout {
		r := clusterGenRoot()
		r.Children = nil
		s := child(r, "s", layout.KustomizationUnset)
		s.ApplicationFileMode = layout.AppFileSingle
		if !withResource {
			s.Resources = nil
		}
		return r
	}
	for name, withResource := range map[string]bool{"child with a resource": true, "child without resources": false} {
		t.Run(name, func(t *testing.T) {
			files := writtenFiles(t, "WriteManifest", layout.DefaultLayoutConfig(), tree(withResource))
			var root string
			for file, content := range files {
				if strings.HasSuffix(file, "cluster/kustomization.yaml") {
					root = content
				}
			}
			if !strings.Contains(root, "- name: x") {
				t.Errorf("no cluster/kustomization.yaml generates x:\n%v", files)
			}
			if listed := strings.Contains(root, "s.yaml"); listed != withResource {
				t.Errorf("cluster/kustomization.yaml lists s.yaml: %t, want %t:\n%s", listed, withResource, root)
			}
			kustomizeBuildsAll(t, tree(withResource))
		})
	}
}
