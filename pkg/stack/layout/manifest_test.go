package layout_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack/layout"
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

	expected := filepath.Join(dir, "demo", "app.yaml")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected single file not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "demo", "app")); !os.IsNotExist(err) {
		t.Fatalf("unexpected application directory created")
	}
}

func TestManifestLayoutSingleFileWithParent(t *testing.T) {
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

	child := &layout.ManifestLayout{
		Name:                "app",
		Namespace:           filepath.Join("demo", "parent"),
		ApplicationFileMode: layout.AppFileSingle,
		Resources:           []client.Object{obj1, obj2},
	}

	parent := &layout.ManifestLayout{
		Name:      "parent",
		Namespace: "demo",
		Children:  []*layout.ManifestLayout{child},
	}

	dir := t.TempDir()
	if err := parent.WriteToDisk(dir); err != nil {
		t.Fatalf("write to disk: %v", err)
	}

	expected := filepath.Join(dir, "demo", "parent", "app.yaml")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected application file not written: %v", err)
	}

	kustomPath := filepath.Join(dir, "demo", "parent", "kustomization.yaml")
	data, err := os.ReadFile(kustomPath)
	if err != nil {
		t.Fatalf("read kustomization: %v", err)
	}
	if !strings.Contains(string(data), "app.yaml") {
		t.Fatalf("expected app.yaml reference in kustomization")
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
	if !strings.Contains(string(data), "child") {
		t.Fatalf("missing child reference")
	}
}

func TestManifestLayoutYAMLFormat(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("test-config")
	obj.SetNamespace("test-ns")
	obj.Object["data"] = map[string]interface{}{
		"key": "value",
	}

	ml := &layout.ManifestLayout{
		Name:      "test",
		Namespace: "test",
		FilePer:   layout.FilePerResource,
		Resources: []client.Object{obj},
	}

	dir := t.TempDir()
	if err := ml.WriteToDisk(dir); err != nil {
		t.Fatalf("write to disk: %v", err)
	}

	// Check the generated YAML file
	yamlPath := filepath.Join(dir, "test", "test", "test-ns-configmap-test-config.yaml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("read YAML file: %v", err)
	}

	yamlContent := string(data)
	
	// Verify proper Kubernetes YAML format
	if !strings.Contains(yamlContent, "apiVersion: v1") {
		t.Errorf("Expected proper apiVersion field, got: %s", yamlContent)
	}
	if !strings.Contains(yamlContent, "kind: ConfigMap") {
		t.Errorf("Expected proper kind field, got: %s", yamlContent)
	}
	if !strings.Contains(yamlContent, "metadata:") {
		t.Errorf("Expected proper metadata field, got: %s", yamlContent)
	}
	if !strings.Contains(yamlContent, "name: test-config") {
		t.Errorf("Expected proper name field, got: %s", yamlContent)
	}
	if !strings.Contains(yamlContent, "namespace: test-ns") {
		t.Errorf("Expected proper namespace field, got: %s", yamlContent)
	}
	
	// Verify it's NOT using the old lowercase format
	if strings.Contains(yamlContent, "typemeta:") {
		t.Errorf("Found old lowercase typemeta format in: %s", yamlContent)
	}
	if strings.Contains(yamlContent, "objectmeta:") {
		t.Errorf("Found old lowercase objectmeta format in: %s", yamlContent)
	}
}

func TestManifestLayoutKustomizationFormat(t *testing.T) {
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

	// Check the generated kustomization.yaml file
	kustomPath := filepath.Join(dir, "default", "test", "kustomization.yaml")
	data, err := os.ReadFile(kustomPath)
	if err != nil {
		t.Fatalf("read kustomization file: %v", err)
	}

	content := string(data)
	
	// Verify proper kustomization format
	if !strings.Contains(content, "apiVersion: kustomize.config.k8s.io/v1beta1") {
		t.Errorf("Expected proper apiVersion, got: %s", content)
	}
	if !strings.Contains(content, "kind: Kustomization") {
		t.Errorf("Expected proper kind, got: %s", content)
	}
	if !strings.Contains(content, "resources:") {
		t.Errorf("Expected resources section, got: %s", content)
	}
	if !strings.Contains(content, "- default-configmap-test.yaml") {
		t.Errorf("Expected resource file reference, got: %s", content)
	}
	
	// Verify proper line endings
	lines := strings.Split(content, "\n")
	if len(lines) < 4 {
		t.Errorf("Expected at least 4 lines in kustomization.yaml, got %d", len(lines))
	}
}

func TestWritePackagesToDisk(t *testing.T) {
	obj1 := &unstructured.Unstructured{}
	obj1.SetAPIVersion("v1")
	obj1.SetKind("ConfigMap")
	obj1.SetName("config1")
	obj1.SetNamespace("default")

	obj2 := &unstructured.Unstructured{}
	obj2.SetAPIVersion("v1")
	obj2.SetKind("Secret")
	obj2.SetName("secret1")
	obj2.SetNamespace("default")

	// Create packages with different PackageRef values
	ociPackageRef := &schema.GroupVersionKind{
		Group:   "source.toolkit.fluxcd.io",
		Version: "v1beta2",
		Kind:    "OCIRepository",
	}
	gitPackageRef := &schema.GroupVersionKind{
		Group:   "source.toolkit.fluxcd.io",
		Version: "v1",
		Kind:    "GitRepository",
	}

	packages := map[string]*layout.ManifestLayout{
		"default": {
			Name:      "default",
			Namespace: ".",
			Resources: []client.Object{obj1},
		},
		ociPackageRef.String(): {
			Name:       "oci-package",
			Namespace:  ".",
			PackageRef: ociPackageRef,
			Resources:  []client.Object{obj2},
		},
		gitPackageRef.String(): {
			Name:       "git-package",
			Namespace:  ".",
			PackageRef: gitPackageRef,
		},
	}

	dir := t.TempDir()
	if err := layout.WritePackagesToDisk(packages, dir); err != nil {
		t.Fatalf("write packages to disk: %v", err)
	}

	// Check that directories were created with sanitized names
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read output directory: %v", err)
	}

	expectedDirs := []string{"default", "source.toolkit.fluxcd.io-v1beta2-Kind-OCIRepository", "source.toolkit.fluxcd.io-v1-Kind-GitRepository"}
	found := make(map[string]bool)
	
	for _, entry := range entries {
		if entry.IsDir() {
			found[entry.Name()] = true
		}
	}

	for _, expected := range expectedDirs {
		if !found[expected] {
			t.Errorf("Expected directory %s not found. Found: %v", expected, found)
		}
	}

	// Verify files were written in the correct directories
	defaultConfig := filepath.Join(dir, "default", ".", "default", "default-configmap-config1.yaml")
	if _, err := os.Stat(defaultConfig); err != nil {
		t.Errorf("Expected default config file not found: %v", err)
	}

	ociSecret := filepath.Join(dir, "source.toolkit.fluxcd.io-v1beta2-Kind-OCIRepository", ".", "oci-package", "default-secret-secret1.yaml")
	if _, err := os.Stat(ociSecret); err != nil {
		t.Errorf("Expected OCI secret file not found: %v", err)
	}
}

func TestSanitizePackageKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"default", "default"},
		{"source.toolkit.fluxcd.io/v1beta2, Kind=OCIRepository", "source.toolkit.fluxcd.io-v1beta2-Kind-OCIRepository"},
		{"example.com:8080/repo", "example.com-8080-repo"},
		{"namespace/name?version=1.0", "namespace-name-version-1.0"},
		{"test::with::colons", "test-with-colons"},
		{"test  with  spaces", "test-with-spaces"},
		{"test---multiple---dashes", "test-multiple-dashes"},
		{"---leading-and-trailing---", "leading-and-trailing"},
		{"", "unknown-package"},
		{"!@#$%^&*()", "unknown-package"},
	}

	for _, test := range tests {
		// We can't directly test the internal function, but we can test through WritePackagesToDisk
		packages := map[string]*layout.ManifestLayout{
			test.input: {
				Name:      "test",
				Namespace: ".",
			},
		}

		dir := t.TempDir()
		if err := layout.WritePackagesToDisk(packages, dir); err != nil {
			t.Fatalf("write packages for input %q: %v", test.input, err)
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read dir for input %q: %v", test.input, err)
		}

		if len(entries) != 1 {
			t.Fatalf("expected 1 directory for input %q, got %d", test.input, len(entries))
		}

		if entries[0].Name() != test.expected {
			t.Errorf("sanitize %q: expected %q, got %q", test.input, test.expected, entries[0].Name())
		}
	}
}
