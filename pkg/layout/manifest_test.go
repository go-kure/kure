package layout_test

import (
	"os"
	"path/filepath"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/layout"
)

func TestManifestLayoutWrite(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("test")
	obj.SetNamespace("default")

	ml := &layout.ManifestLayout{
		Name:      "test",
		Namespace: "default",
		FilePer:   layout.FilePerResource,
		Resources: []client.Object{obj},
	}

	dir := t.TempDir()
	if err := ml.WriteToDisk(dir); err != nil {
		t.Fatalf("write to disk: %v", err)
	}
	path := filepath.Join(dir, "default", "test", "default-configmap-test.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file not written: %v", err)
	}
}

func TestManifestLayoutWriteWithConfig(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("demo")
	obj.SetNamespace("demo")

	ml := &layout.ManifestLayout{
		Name:      "app",
		Namespace: "demo",
		Resources: []client.Object{obj},
	}

	cfg := layout.DefaultLayoutConfig()
	cfg.ManifestsDir = "manifests"
	cfg.ManifestFileName = func(ns, kind, name string, _ layout.FileExportMode) string {
		return ns + "_" + kind + "_" + name + ".yml"
	}

	dir := t.TempDir()
	if err := layout.WriteManifest(dir, cfg, ml); err != nil {
		t.Fatalf("write with config: %v", err)
	}

	expected := filepath.Join(dir, "manifests", "demo", "app", "demo_configmap_demo.yml")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected file not written: %v", err)
	}
}
