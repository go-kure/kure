package layout

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestRenderConfigMapGeneratorBlock_Empty(t *testing.T) {
	if got := renderConfigMapGeneratorBlock(nil); got != "" {
		t.Errorf("expected empty string for nil specs, got %q", got)
	}
	if got := renderConfigMapGeneratorBlock([]ConfigMapGeneratorSpec{}); got != "" {
		t.Errorf("expected empty string for empty specs, got %q", got)
	}
}

func TestRenderConfigMapGeneratorBlock_Multiple(t *testing.T) {
	specs := []ConfigMapGeneratorSpec{
		{Name: "myapp-values", Files: []string{"values.yaml"}},
		{Name: "extra", Files: []string{"a.txt", "b.txt"}},
	}
	got := renderConfigMapGeneratorBlock(specs)
	want := "configMapGenerator:\n" +
		"  - name: myapp-values\n" +
		"    files:\n" +
		"      - values.yaml\n" +
		"  - name: extra\n" +
		"    files:\n" +
		"      - a.txt\n" +
		"      - b.txt\n"
	if got != want {
		t.Errorf("unexpected block:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestWriteToDisk_ExtraFilesAndConfigMapGenerator(t *testing.T) {
	obj := testObject("v1", "ConfigMap", "test", "default")
	ml := &ManifestLayout{
		Name:      "test",
		Namespace: "default",
		FilePer:   FilePerResource,
		Resources: []client.Object{obj},
		ExtraFiles: []ExtraFile{
			{Name: "values.yaml", Content: []byte("foo: bar\n")},
		},
		ConfigMapGenerators: []ConfigMapGeneratorSpec{
			{Name: "myapp-values", Files: []string{"values.yaml"}},
		},
	}

	dir := t.TempDir()
	if err := ml.WriteToDisk(dir); err != nil {
		t.Fatalf("WriteToDisk: %v", err)
	}

	valuesPath := filepath.Join(dir, "default", "test", "values.yaml")
	got, err := os.ReadFile(valuesPath)
	if err != nil {
		t.Fatalf("read values.yaml: %v", err)
	}
	if string(got) != "foo: bar\n" {
		t.Errorf("values.yaml content: got %q", string(got))
	}

	kustomBytes, err := os.ReadFile(filepath.Join(dir, "default", "test", "kustomization.yaml"))
	if err != nil {
		t.Fatalf("read kustomization.yaml: %v", err)
	}
	kustom := string(kustomBytes)
	if !strings.Contains(kustom, "configMapGenerator:") {
		t.Errorf("kustomization.yaml missing configMapGenerator: section:\n%s", kustom)
	}
	if !strings.Contains(kustom, "name: myapp-values") {
		t.Errorf("kustomization.yaml missing generator name:\n%s", kustom)
	}
	if !strings.Contains(kustom, "- values.yaml") {
		t.Errorf("kustomization.yaml missing generator file ref:\n%s", kustom)
	}
	if strings.Index(kustom, "resources:") > strings.Index(kustom, "configMapGenerator:") {
		t.Errorf("configMapGenerator must follow resources:\n%s", kustom)
	}
}

func TestWriteManifest_ExtraFilesAndConfigMapGenerator(t *testing.T) {
	obj := testObject("v1", "ConfigMap", "test", "default")
	ml := &ManifestLayout{
		Name:      "test",
		Namespace: "default",
		FilePer:   FilePerResource,
		Resources: []client.Object{obj},
		ExtraFiles: []ExtraFile{
			{Name: "values.yaml", Content: []byte("a: 1\n")},
		},
		ConfigMapGenerators: []ConfigMapGeneratorSpec{
			{Name: "test-values", Files: []string{"values.yaml"}},
		},
	}

	dir := t.TempDir()
	cfg := Config{ManifestsDir: "clusters", FilePer: FilePerResource}
	if err := WriteManifest(dir, cfg, ml); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	base := filepath.Join(dir, "clusters", "default", "test")
	if got, err := os.ReadFile(filepath.Join(base, "values.yaml")); err != nil {
		t.Fatalf("read values.yaml: %v", err)
	} else if string(got) != "a: 1\n" {
		t.Errorf("values.yaml content: got %q", string(got))
	}

	kustomBytes, err := os.ReadFile(filepath.Join(base, "kustomization.yaml"))
	if err != nil {
		t.Fatalf("read kustomization.yaml: %v", err)
	}
	kustom := string(kustomBytes)
	if !strings.Contains(kustom, "configMapGenerator:") {
		t.Errorf("kustomization.yaml missing configMapGenerator:\n%s", kustom)
	}
	if !strings.Contains(kustom, "name: test-values") {
		t.Errorf("kustomization.yaml missing generator name:\n%s", kustom)
	}
}

func TestWriteToTar_ExtraFilesAndConfigMapGenerator(t *testing.T) {
	obj := testObject("v1", "ConfigMap", "test", "default")
	ml := &ManifestLayout{
		Name:      "test",
		Namespace: "default",
		FilePer:   FilePerResource,
		Resources: []client.Object{obj},
		ExtraFiles: []ExtraFile{
			{Name: "values.yaml", Content: []byte("hello: world\n")},
		},
		ConfigMapGenerators: []ConfigMapGeneratorSpec{
			{Name: "tar-values", Files: []string{"values.yaml"}},
		},
	}

	var buf bytes.Buffer
	if err := ml.WriteToTar(&buf); err != nil {
		t.Fatalf("WriteToTar: %v", err)
	}

	files := extractTarFiles(t, &buf)
	values, ok := files["default/test/values.yaml"]
	if !ok {
		t.Fatalf("values.yaml missing from tar (got: %v)", fileNames(files))
	}
	if !bytes.Equal(values, []byte("hello: world\n")) {
		t.Errorf("values.yaml content: got %q", string(values))
	}

	kustom, ok := files["default/test/kustomization.yaml"]
	if !ok {
		t.Fatalf("kustomization.yaml missing from tar")
	}
	if !bytes.Contains(kustom, []byte("configMapGenerator:")) {
		t.Errorf("kustomization.yaml missing configMapGenerator:\n%s", kustom)
	}
	if !bytes.Contains(kustom, []byte("name: tar-values")) {
		t.Errorf("kustomization.yaml missing generator name:\n%s", kustom)
	}
}
