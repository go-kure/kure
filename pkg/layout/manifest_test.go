package layout_test

import (
	"os"
	"path/filepath"
	"strings"
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

func TestManifestLayoutSingleFile(t *testing.T) {
	obj1 := &unstructured.Unstructured{}
	obj1.SetAPIVersion("v1")
	obj1.SetKind("ConfigMap")
	obj1.SetName("one")
	obj1.SetNamespace("demo")

	obj2 := &unstructured.Unstructured{}
	obj2.SetAPIVersion("v1")
	obj2.SetKind("Secret")
	obj2.SetName("two")
	obj2.SetNamespace("demo")

	ml := &layout.ManifestLayout{
		Name:                "app",
		Namespace:           "demo",
		ApplicationFileMode: layout.AppFileSingle,
		Resources:           []client.Object{obj1, obj2},
	}

	dir := t.TempDir()
	if err := ml.WriteToDisk(dir); err != nil {
		t.Fatalf("write to disk: %v", err)
	}

	expected := filepath.Join(dir, "demo", "app", "app.yaml")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected single file not written: %v", err)
	}
}

func TestManifestLayoutRecursiveMode(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("test")
	obj.SetNamespace("default")

	child := &layout.ManifestLayout{
		Name:      "child",
		Namespace: "default",
		Mode:      layout.KustomizationRecursive,
		Resources: []client.Object{obj},
	}

	root := &layout.ManifestLayout{
		Name:      "root",
		Namespace: "default",
		Mode:      layout.KustomizationRecursive,
		Children:  []*layout.ManifestLayout{child},
	}

	dir := t.TempDir()
	if err := root.WriteToDisk(dir); err != nil {
		t.Fatalf("write recursive: %v", err)
	}

	rootK := filepath.Join(dir, "default", "root", "kustomization.yaml")
	data, err := os.ReadFile(rootK)
	if err != nil {
		t.Fatalf("read root kustomization: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("root kustomization empty")
	}
	if _, err := os.Stat(filepath.Join(dir, "default", "child", "kustomization.yaml")); !os.IsNotExist(err) {
		t.Fatalf("unexpected child kustomization")
	}
	if strings.Contains(string(data), "configmap") {
		t.Fatalf("unexpected manifest file reference")
	}
	if !strings.Contains(string(data), "../child") {
		t.Fatalf("missing child reference")
	}
}
