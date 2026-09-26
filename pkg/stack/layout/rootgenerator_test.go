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
// that root a kustomization.yaml, and it generates the ConfigMap.
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
