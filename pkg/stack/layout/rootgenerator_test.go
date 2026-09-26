package layout_test

import (
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

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

// clusterGenRoot is WriteManifest's synthetic cluster root (Name "", a
// single-segment Namespace, no resources) carrying a configMapGenerator, with
// a directory child apps.
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

// TestWriteManifest_RefuseGeneratorsOnClusterRoot: WriteManifest writes the
// synthetic cluster root no kustomization.yaml when it has no resources, so
// it refuses the root's ConfigMapGenerators. WriteToDisk and WriteToTar write
// that root a kustomization.yaml that generates the ConfigMap, and
// WriteToDisk's output builds with kustomize.
func TestWriteManifest_RefuseGeneratorsOnClusterRoot(t *testing.T) {
	want := `layout "cluster" gets no kustomization.yaml, so its ConfigMapGenerators have nowhere to go`
	err := writeRefused(t, "WriteManifest", layout.DefaultLayoutConfig(), clusterGenRoot())
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("WriteManifest: err = %v, want it to contain %q", err, want)
	}
	for _, writer := range []string{"WriteToDisk", "WriteToTar"} {
		files := writtenFiles(t, writer, layout.DefaultLayoutConfig(), clusterGenRoot())
		if k := files["cluster/kustomization.yaml"]; !strings.Contains(k, "- name: x") {
			t.Errorf("%s: cluster/kustomization.yaml does not generate x:\n%v", writer, files)
		}
	}
	kustomizeBuildsAll(t, clusterGenRoot())
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
// writes the synthetic cluster root a kustomization.yaml when it lists an
// AppFileSingle child's file, and that kustomization.yaml generates the
// root's ConfigMap. A child with no resources writes no file, so the root
// gets none and its generators are refused.
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
	files := writtenFiles(t, "WriteManifest", layout.DefaultLayoutConfig(), tree(true))
	generated := false
	for name, content := range files {
		if strings.HasSuffix(name, "cluster/kustomization.yaml") && strings.Contains(content, "- name: x") {
			generated = true
		}
	}
	if !generated {
		t.Errorf("WriteManifest: no cluster/kustomization.yaml generates x:\n%v", files)
	}
	err := writeRefused(t, "WriteManifest", layout.DefaultLayoutConfig(), tree(false))
	want := `layout "cluster" gets no kustomization.yaml, so its ConfigMapGenerators have nowhere to go`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("WriteManifest: err = %v, want it to contain %q", err, want)
	}
}
