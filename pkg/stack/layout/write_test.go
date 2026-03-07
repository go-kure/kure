package layout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func testObject(apiVersion, kind, name, namespace string) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(apiVersion)
	obj.SetKind(kind)
	obj.SetName(name)
	obj.SetNamespace(namespace)
	return obj
}

// ---------------------------------------------------------------------------
// WriteManifest tests
// ---------------------------------------------------------------------------

func TestWriteManifest_BasicResource(t *testing.T) {
	obj := testObject("v1", "ConfigMap", "my-config", "default")

	ml := &ManifestLayout{
		Name:      "app",
		Namespace: "mycluster/myns",
		Resources: []client.Object{obj},
	}

	cfg := DefaultLayoutConfig()
	dir := t.TempDir()

	if err := WriteManifest(dir, cfg, ml); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	// Expected path: dir/clusters/mycluster/myns/app/<file>
	resourceFile := filepath.Join(dir, "clusters", "mycluster", "myns", "app", "default-configmap-my-config.yaml")
	if _, err := os.Stat(resourceFile); err != nil {
		t.Fatalf("expected resource file at %s: %v", resourceFile, err)
	}

	kustomFile := filepath.Join(dir, "clusters", "mycluster", "myns", "app", "kustomization.yaml")
	data, err := os.ReadFile(kustomFile)
	if err != nil {
		t.Fatalf("expected kustomization.yaml at %s: %v", kustomFile, err)
	}
	if !strings.Contains(string(data), "default-configmap-my-config.yaml") {
		t.Errorf("kustomization.yaml does not reference resource file, got:\n%s", data)
	}
}

func TestWriteManifest_DefaultConfig(t *testing.T) {
	obj := testObject("v1", "ConfigMap", "test", "ns")

	ml := &ManifestLayout{
		Name:      "svc",
		Namespace: "cluster1/ns",
		Resources: []client.Object{obj},
	}

	// Empty config — ManifestFileName nil, ManifestsDir empty.
	cfg := Config{}
	dir := t.TempDir()

	if err := WriteManifest(dir, cfg, ml); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	// Defaults: ManifestsDir="clusters", ManifestFileName=DefaultManifestFileName
	resourceFile := filepath.Join(dir, "clusters", "cluster1", "ns", "svc", "ns-configmap-test.yaml")
	if _, err := os.Stat(resourceFile); err != nil {
		t.Fatalf("expected default-named resource file at %s: %v", resourceFile, err)
	}
}

func TestWriteManifest_AppFileSingle(t *testing.T) {
	obj := testObject("v1", "ConfigMap", "one", "demo")

	ml := &ManifestLayout{
		Name:                "myapp",
		Namespace:           "mycluster/demo",
		ApplicationFileMode: AppFileSingle,
		Resources:           []client.Object{obj},
	}

	cfg := DefaultLayoutConfig()
	dir := t.TempDir()

	if err := WriteManifest(dir, cfg, ml); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	// AppFileSingle: fullPath = basePath/ManifestsDir/Namespace, file = Name.yaml
	singleFile := filepath.Join(dir, "clusters", "mycluster", "demo", "myapp.yaml")
	if _, err := os.Stat(singleFile); err != nil {
		t.Fatalf("expected single app file at %s: %v", singleFile, err)
	}

	// The per-resource directory should not exist.
	perResourceDir := filepath.Join(dir, "clusters", "mycluster", "demo", "myapp")
	if _, err := os.Stat(perResourceDir); !os.IsNotExist(err) {
		t.Errorf("per-resource directory should not exist at %s", perResourceDir)
	}
}

func TestWriteManifest_MultipleResources(t *testing.T) {
	objs := []client.Object{
		testObject("v1", "ConfigMap", "cm1", "ns"),
		testObject("v1", "Secret", "sec1", "ns"),
		testObject("apps/v1", "Deployment", "dep1", "ns"),
	}

	ml := &ManifestLayout{
		Name:      "stack",
		Namespace: "cl/ns",
		Resources: objs,
	}

	cfg := DefaultLayoutConfig()
	dir := t.TempDir()

	if err := WriteManifest(dir, cfg, ml); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	base := filepath.Join(dir, "clusters", "cl", "ns", "stack")
	expectedFiles := []string{
		"ns-configmap-cm1.yaml",
		"ns-secret-sec1.yaml",
		"ns-deployment-dep1.yaml",
	}
	for _, f := range expectedFiles {
		p := filepath.Join(base, f)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected file %s: %v", p, err)
		}
	}

	kData, err := os.ReadFile(filepath.Join(base, "kustomization.yaml"))
	if err != nil {
		t.Fatalf("read kustomization.yaml: %v", err)
	}
	for _, f := range expectedFiles {
		if !strings.Contains(string(kData), f) {
			t.Errorf("kustomization.yaml missing reference to %s", f)
		}
	}
}

func TestWriteManifest_FilePerKind(t *testing.T) {
	objs := []client.Object{
		testObject("v1", "ConfigMap", "cm1", "ns"),
		testObject("v1", "ConfigMap", "cm2", "ns"),
		testObject("v1", "Secret", "sec1", "ns"),
	}

	ml := &ManifestLayout{
		Name:      "app",
		Namespace: "cl/ns",
		FilePer:   FilePerKind,
		Resources: objs,
	}

	cfg := DefaultLayoutConfig()
	dir := t.TempDir()

	if err := WriteManifest(dir, cfg, ml); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	base := filepath.Join(dir, "clusters", "cl", "ns", "app")

	// FilePerKind groups by kind: ns-configmap.yaml (both CMs), ns-secret.yaml
	cmFile := filepath.Join(base, "ns-configmap.yaml")
	if _, err := os.Stat(cmFile); err != nil {
		t.Fatalf("expected grouped configmap file at %s: %v", cmFile, err)
	}

	secFile := filepath.Join(base, "ns-secret.yaml")
	if _, err := os.Stat(secFile); err != nil {
		t.Fatalf("expected grouped secret file at %s: %v", secFile, err)
	}

	// There should be no per-resource files.
	perResource := filepath.Join(base, "ns-configmap-cm1.yaml")
	if _, err := os.Stat(perResource); !os.IsNotExist(err) {
		t.Errorf("per-resource file should not exist at %s", perResource)
	}

	// Verify kustomization references the grouped files.
	kData, err := os.ReadFile(filepath.Join(base, "kustomization.yaml"))
	if err != nil {
		t.Fatalf("read kustomization.yaml: %v", err)
	}
	if !strings.Contains(string(kData), "ns-configmap.yaml") {
		t.Errorf("kustomization.yaml missing ns-configmap.yaml reference")
	}
	if !strings.Contains(string(kData), "ns-secret.yaml") {
		t.Errorf("kustomization.yaml missing ns-secret.yaml reference")
	}
}

func TestWriteManifest_KustomizationExplicit(t *testing.T) {
	obj := testObject("v1", "ConfigMap", "cfg", "ns")
	child := &ManifestLayout{
		Name:      "child",
		Namespace: "cl/ns/parent/child",
		Resources: []client.Object{testObject("v1", "Secret", "s1", "ns")},
	}

	ml := &ManifestLayout{
		Name:      "parent",
		Namespace: "cl/ns",
		Mode:      KustomizationExplicit,
		Resources: []client.Object{obj},
		Children:  []*ManifestLayout{child},
	}

	cfg := DefaultLayoutConfig()
	cfg.KustomizationMode = KustomizationExplicit
	dir := t.TempDir()

	if err := WriteManifest(dir, cfg, ml); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	parentK := filepath.Join(dir, "clusters", "cl", "ns", "parent", "kustomization.yaml")
	data, err := os.ReadFile(parentK)
	if err != nil {
		t.Fatalf("read parent kustomization: %v", err)
	}

	content := string(data)
	// Explicit mode: lists both resource files AND child directories.
	if !strings.Contains(content, "ns-configmap-cfg.yaml") {
		t.Errorf("explicit kustomization missing resource file reference, got:\n%s", content)
	}
	if !strings.Contains(content, "child") {
		t.Errorf("explicit kustomization missing child reference, got:\n%s", content)
	}
}

func TestWriteManifest_KustomizationRecursive(t *testing.T) {
	obj := testObject("v1", "ConfigMap", "cfg", "ns")
	child := &ManifestLayout{
		Name:      "child",
		Namespace: "cl/ns/parent/child",
		Resources: []client.Object{testObject("v1", "Secret", "s1", "ns")},
	}

	ml := &ManifestLayout{
		Name:      "parent",
		Namespace: "cl/ns",
		Mode:      KustomizationRecursive,
		Resources: []client.Object{obj},
		Children:  []*ManifestLayout{child},
	}

	cfg := DefaultLayoutConfig()
	cfg.KustomizationMode = KustomizationRecursive
	dir := t.TempDir()

	if err := WriteManifest(dir, cfg, ml); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	parentK := filepath.Join(dir, "clusters", "cl", "ns", "parent", "kustomization.yaml")
	data, err := os.ReadFile(parentK)
	if err != nil {
		t.Fatalf("read parent kustomization: %v", err)
	}

	content := string(data)
	// Recursive mode with children: parent kustomization references child dirs, NOT resource files.
	if strings.Contains(content, "ns-configmap-cfg.yaml") {
		t.Errorf("recursive kustomization should NOT list resource files when children exist, got:\n%s", content)
	}
	if !strings.Contains(content, "child") {
		t.Errorf("recursive kustomization missing child directory reference, got:\n%s", content)
	}

	// Child (leaf) kustomization SHOULD list its own files.
	childK := filepath.Join(dir, "clusters", "cl", "ns", "parent", "child", "kustomization.yaml")
	childData, err := os.ReadFile(childK)
	if err != nil {
		t.Fatalf("read child kustomization: %v", err)
	}
	if !strings.Contains(string(childData), "ns-secret-s1.yaml") {
		t.Errorf("child leaf kustomization should list its files, got:\n%s", childData)
	}
}

func TestWriteManifest_WithChildren(t *testing.T) {
	childA := &ManifestLayout{
		Name:      "alpha",
		Namespace: "cl/ns/root/alpha",
		Resources: []client.Object{testObject("v1", "ConfigMap", "a", "ns")},
	}
	childB := &ManifestLayout{
		Name:      "beta",
		Namespace: "cl/ns/root/beta",
		Resources: []client.Object{testObject("v1", "Secret", "b", "ns")},
	}

	root := &ManifestLayout{
		Name:      "root",
		Namespace: "cl/ns",
		Children:  []*ManifestLayout{childA, childB},
	}

	cfg := DefaultLayoutConfig()
	dir := t.TempDir()

	if err := WriteManifest(dir, cfg, root); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	// Verify child subdirectories and files created.
	alphaFile := filepath.Join(dir, "clusters", "cl", "ns", "root", "alpha", "ns-configmap-a.yaml")
	if _, err := os.Stat(alphaFile); err != nil {
		t.Errorf("expected child alpha file at %s: %v", alphaFile, err)
	}
	betaFile := filepath.Join(dir, "clusters", "cl", "ns", "root", "beta", "ns-secret-b.yaml")
	if _, err := os.Stat(betaFile); err != nil {
		t.Errorf("expected child beta file at %s: %v", betaFile, err)
	}

	// Parent kustomization should reference child directories.
	parentK := filepath.Join(dir, "clusters", "cl", "ns", "root", "kustomization.yaml")
	data, err := os.ReadFile(parentK)
	if err != nil {
		t.Fatalf("read parent kustomization: %v", err)
	}
	if !strings.Contains(string(data), "alpha") {
		t.Errorf("parent kustomization missing alpha child, got:\n%s", data)
	}
	if !strings.Contains(string(data), "beta") {
		t.Errorf("parent kustomization missing beta child, got:\n%s", data)
	}
}

func TestWriteManifest_ClusterRootNoKustomization(t *testing.T) {
	obj := testObject("v1", "Namespace", "default", "")

	// Cluster root: Namespace without separator, Name empty.
	ml := &ManifestLayout{
		Name:      "",
		Namespace: "mycluster",
		Resources: []client.Object{obj},
	}

	cfg := DefaultLayoutConfig()
	dir := t.TempDir()

	if err := WriteManifest(dir, cfg, ml); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	// Resource file should still be written.
	resourceFile := filepath.Join(dir, "clusters", "mycluster", "cluster-namespace-default.yaml")
	if _, err := os.Stat(resourceFile); err != nil {
		t.Fatalf("expected resource file at %s: %v", resourceFile, err)
	}

	// Kustomization should NOT be generated at cluster root.
	kustomFile := filepath.Join(dir, "clusters", "mycluster", "kustomization.yaml")
	if _, err := os.Stat(kustomFile); !os.IsNotExist(err) {
		t.Errorf("kustomization.yaml should NOT be generated at cluster root (%s)", kustomFile)
	}
}

func TestWriteManifest_ClusterNamespace(t *testing.T) {
	// Resource with empty namespace gets "cluster" prefix in filename.
	obj := testObject("v1", "Namespace", "kube-system", "")

	ml := &ManifestLayout{
		Name:      "infra",
		Namespace: "cl/ns",
		Resources: []client.Object{obj},
	}

	cfg := DefaultLayoutConfig()
	dir := t.TempDir()

	if err := WriteManifest(dir, cfg, ml); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	// Empty namespace -> "cluster" prefix.
	clusterFile := filepath.Join(dir, "clusters", "cl", "ns", "infra", "cluster-namespace-kube-system.yaml")
	if _, err := os.Stat(clusterFile); err != nil {
		t.Fatalf("expected cluster-prefixed file at %s: %v", clusterFile, err)
	}
}

func TestWriteManifest_FluxIntegrated(t *testing.T) {
	childA := &ManifestLayout{
		Name:      "team-a",
		Namespace: "cl/flux-system/team-a",
		Resources: []client.Object{testObject("v1", "ConfigMap", "ca", "flux-system")},
	}
	childB := &ManifestLayout{
		Name:      "team-b",
		Namespace: "cl/flux-system/team-b",
		Resources: []client.Object{testObject("v1", "ConfigMap", "cb", "flux-system")},
	}

	root := &ManifestLayout{
		Name:          "flux-root",
		Namespace:     "cl/flux-system",
		FluxPlacement: FluxIntegrated,
		Children:      []*ManifestLayout{childA, childB},
	}

	cfg := DefaultLayoutConfig()
	dir := t.TempDir()

	if err := WriteManifest(dir, cfg, root); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	kustomFile := filepath.Join(dir, "clusters", "cl", "flux-system", "flux-root", "kustomization.yaml")
	data, err := os.ReadFile(kustomFile)
	if err != nil {
		t.Fatalf("read kustomization: %v", err)
	}

	content := string(data)
	// FluxIntegrated: children referenced as flux-system-kustomization-<name>.yaml
	if !strings.Contains(content, "flux-system-kustomization-team-a.yaml") {
		t.Errorf("expected flux kustomization reference for team-a, got:\n%s", content)
	}
	if !strings.Contains(content, "flux-system-kustomization-team-b.yaml") {
		t.Errorf("expected flux kustomization reference for team-b, got:\n%s", content)
	}
	// Should NOT reference child directory names directly.
	if strings.Contains(content, "  - team-a\n") {
		t.Errorf("should not reference child as plain directory in flux integrated mode, got:\n%s", content)
	}
}

func TestWriteManifest_ChildAppFileSingle(t *testing.T) {
	child := &ManifestLayout{
		Name:                "my-svc",
		Namespace:           "cl/ns/parent",
		ApplicationFileMode: AppFileSingle,
		Resources:           []client.Object{testObject("v1", "ConfigMap", "c", "ns")},
	}

	parent := &ManifestLayout{
		Name:      "parent",
		Namespace: "cl/ns",
		Resources: []client.Object{testObject("v1", "Secret", "s", "ns")},
		Children:  []*ManifestLayout{child},
	}

	cfg := DefaultLayoutConfig()
	dir := t.TempDir()

	if err := WriteManifest(dir, cfg, parent); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	// Child written as single file: <parent-dir>/my-svc.yaml
	childFile := filepath.Join(dir, "clusters", "cl", "ns", "parent", "my-svc.yaml")
	if _, err := os.Stat(childFile); err != nil {
		t.Fatalf("expected child single file at %s: %v", childFile, err)
	}

	// Parent kustomization should reference child as name.yaml.
	parentK := filepath.Join(dir, "clusters", "cl", "ns", "parent", "kustomization.yaml")
	data, err := os.ReadFile(parentK)
	if err != nil {
		t.Fatalf("read parent kustomization: %v", err)
	}
	if !strings.Contains(string(data), "my-svc.yaml") {
		t.Errorf("parent kustomization should reference child as my-svc.yaml, got:\n%s", data)
	}
}

// WriteToDisk FluxPlacement tests (bug fix #264)
// ---------------------------------------------------------------------------

func TestWriteToDisk_FluxIntegrated(t *testing.T) {
	childA := &ManifestLayout{
		Name:      "team-a",
		Namespace: "cl/flux-system/team-a",
		Resources: []client.Object{testObject("v1", "ConfigMap", "ca", "flux-system")},
	}
	childB := &ManifestLayout{
		Name:      "team-b",
		Namespace: "cl/flux-system/team-b",
		Resources: []client.Object{testObject("v1", "ConfigMap", "cb", "flux-system")},
	}

	root := &ManifestLayout{
		Name:          "flux-root",
		Namespace:     "cl/flux-system",
		FluxPlacement: FluxIntegrated,
		Children:      []*ManifestLayout{childA, childB},
	}

	dir := t.TempDir()
	if err := root.WriteToDisk(dir); err != nil {
		t.Fatalf("WriteToDisk failed: %v", err)
	}

	kustomFile := filepath.Join(dir, "cl", "flux-system", "flux-root", "kustomization.yaml")
	data, err := os.ReadFile(kustomFile)
	if err != nil {
		t.Fatalf("read kustomization: %v", err)
	}

	content := string(data)
	// FluxIntegrated: children referenced as flux-system-kustomization-<name>.yaml
	if !strings.Contains(content, "flux-system-kustomization-team-a.yaml") {
		t.Errorf("expected flux kustomization reference for team-a, got:\n%s", content)
	}
	if !strings.Contains(content, "flux-system-kustomization-team-b.yaml") {
		t.Errorf("expected flux kustomization reference for team-b, got:\n%s", content)
	}
	// Should NOT reference child directory names directly.
	if strings.Contains(content, "  - team-a\n") {
		t.Errorf("should not reference child as plain directory in flux integrated mode, got:\n%s", content)
	}
}

func TestWriteToDisk_FluxSeparateDefault(t *testing.T) {
	child := &ManifestLayout{
		Name:      "apps",
		Namespace: "cl/apps",
		Resources: []client.Object{testObject("v1", "ConfigMap", "ca", "default")},
	}

	root := &ManifestLayout{
		Name:          "root",
		Namespace:     "cl",
		FluxPlacement: FluxSeparate,
		Children:      []*ManifestLayout{child},
	}

	dir := t.TempDir()
	if err := root.WriteToDisk(dir); err != nil {
		t.Fatalf("WriteToDisk failed: %v", err)
	}

	kustomFile := filepath.Join(dir, "cl", "root", "kustomization.yaml")
	data, err := os.ReadFile(kustomFile)
	if err != nil {
		t.Fatalf("read kustomization: %v", err)
	}

	content := string(data)
	// FluxSeparate: children referenced as plain directory names
	if !strings.Contains(content, "  - apps\n") {
		t.Errorf("expected plain directory reference for apps, got:\n%s", content)
	}
	// Should NOT contain flux kustomization references
	if strings.Contains(content, "flux-system-kustomization") {
		t.Errorf("should not contain flux kustomization references in separate mode, got:\n%s", content)
	}
}

// ---------------------------------------------------------------------------
// Golden file test: Pattern A preset end-to-end
// ---------------------------------------------------------------------------

func TestWriteManifest_PatternA_EndToEnd(t *testing.T) {
	objs := []client.Object{
		testObject("apps/v1", "Deployment", "myapp-web", "default"),
		testObject("v1", "Service", "myapp-web", "default"),
		testObject("v1", "ConfigMap", "myapp-config", "default"),
	}

	ml := &ManifestLayout{
		Name:      "myapp",
		Namespace: "cl/applications",
		Resources: objs,
	}

	cfg, err := ConfigForPreset(PresetCentralizedControlPlane)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dir := t.TempDir()

	if err := WriteManifest(dir, cfg, ml); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	// Pattern A uses {kind}-{name}.yaml naming
	base := filepath.Join(dir, "clusters", "cl", "applications", "myapp")
	expectedFiles := []string{
		"deployment-myapp-web.yaml",
		"service-myapp-web.yaml",
		"configmap-myapp-config.yaml",
	}
	for _, f := range expectedFiles {
		p := filepath.Join(base, f)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected file %s: %v", p, err)
		}
	}

	// Kustomization should reference {kind}-{name}.yaml files
	kData, err := os.ReadFile(filepath.Join(base, "kustomization.yaml"))
	if err != nil {
		t.Fatalf("read kustomization.yaml: %v", err)
	}
	for _, f := range expectedFiles {
		if !strings.Contains(string(kData), f) {
			t.Errorf("kustomization.yaml missing reference to %s", f)
		}
	}

	// Should NOT contain namespace-prefixed file names
	if strings.Contains(string(kData), "default-deployment") {
		t.Errorf("Pattern A should not use namespace-prefixed file names, got:\n%s", kData)
	}
}

// ---------------------------------------------------------------------------
// FileNaming tests (#266)
// ---------------------------------------------------------------------------

func TestWriteManifest_FileNamingKindName(t *testing.T) {
	objs := []client.Object{
		testObject("apps/v1", "Deployment", "web", "default"),
		testObject("v1", "Service", "web", "default"),
		testObject("v1", "ConfigMap", "config", "default"),
	}

	ml := &ManifestLayout{
		Name:      "myapp",
		Namespace: "cl/apps",
		Resources: objs,
	}

	cfg := DefaultLayoutConfig()
	cfg.FileNaming = FileNamingKindName
	cfg.ManifestFileName = nil // Use FileNaming to resolve
	dir := t.TempDir()

	if err := WriteManifest(dir, cfg, ml); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	base := filepath.Join(dir, "clusters", "cl", "apps", "myapp")
	expectedFiles := []string{
		"deployment-web.yaml",
		"service-web.yaml",
		"configmap-config.yaml",
	}
	for _, f := range expectedFiles {
		p := filepath.Join(base, f)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected file %s: %v", p, err)
		}
	}

	// Should NOT contain namespace-prefixed file names
	kData, err := os.ReadFile(filepath.Join(base, "kustomization.yaml"))
	if err != nil {
		t.Fatalf("read kustomization.yaml: %v", err)
	}
	if strings.Contains(string(kData), "default-deployment") {
		t.Errorf("FileNamingKindName should not use namespace-prefixed names, got:\n%s", kData)
	}
	for _, f := range expectedFiles {
		if !strings.Contains(string(kData), f) {
			t.Errorf("kustomization.yaml missing reference to %s", f)
		}
	}
}

func TestWriteManifest_FileNamingDefault_Unchanged(t *testing.T) {
	obj := testObject("v1", "ConfigMap", "test", "ns")

	ml := &ManifestLayout{
		Name:      "app",
		Namespace: "cl/ns",
		Resources: []client.Object{obj},
	}

	cfg := DefaultLayoutConfig()
	cfg.FileNaming = FileNamingDefault
	dir := t.TempDir()

	if err := WriteManifest(dir, cfg, ml); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	// Default naming: {ns}-{kind}-{name}.yaml
	resourceFile := filepath.Join(dir, "clusters", "cl", "ns", "app", "ns-configmap-test.yaml")
	if _, err := os.Stat(resourceFile); err != nil {
		t.Fatalf("expected default-named file at %s: %v", resourceFile, err)
	}
}

// ---------------------------------------------------------------------------
// WriteToDisk tests
// ---------------------------------------------------------------------------

func TestWriteToDisk_Basic(t *testing.T) {
	obj := testObject("v1", "ConfigMap", "cfg1", "default")

	ml := &ManifestLayout{
		Name:      "svc",
		Namespace: "default",
		Resources: []client.Object{obj},
	}

	dir := t.TempDir()
	if err := ml.WriteToDisk(dir); err != nil {
		t.Fatalf("WriteToDisk failed: %v", err)
	}

	// WriteToDisk: basePath/FullRepoPath()/file = dir/default/svc/<file>
	resourceFile := filepath.Join(dir, "default", "svc", "default-configmap-cfg1.yaml")
	if _, err := os.Stat(resourceFile); err != nil {
		t.Fatalf("expected resource file at %s: %v", resourceFile, err)
	}

	data, err := os.ReadFile(resourceFile)
	if err != nil {
		t.Fatalf("read resource file: %v", err)
	}
	if !strings.Contains(string(data), "kind: ConfigMap") {
		t.Errorf("resource file should contain kind: ConfigMap, got:\n%s", data)
	}
}

func TestWriteToDisk_AppFileSingle(t *testing.T) {
	objs := []client.Object{
		testObject("v1", "ConfigMap", "cm", "ns"),
		testObject("v1", "Secret", "sec", "ns"),
	}

	ml := &ManifestLayout{
		Name:                "myapp",
		Namespace:           "myns",
		ApplicationFileMode: AppFileSingle,
		Resources:           objs,
	}

	dir := t.TempDir()
	if err := ml.WriteToDisk(dir); err != nil {
		t.Fatalf("WriteToDisk failed: %v", err)
	}

	// AppFileSingle: fullPath = basePath/Namespace, file = Name.yaml
	singleFile := filepath.Join(dir, "myns", "myapp.yaml")
	if _, err := os.Stat(singleFile); err != nil {
		t.Fatalf("expected single file at %s: %v", singleFile, err)
	}

	data, err := os.ReadFile(singleFile)
	if err != nil {
		t.Fatalf("read single file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "ConfigMap") {
		t.Errorf("single file should contain ConfigMap, got:\n%s", content)
	}
	if !strings.Contains(content, "Secret") {
		t.Errorf("single file should contain Secret, got:\n%s", content)
	}
}

func TestWriteToDisk_KustomizationGenerated(t *testing.T) {
	objs := []client.Object{
		testObject("v1", "ConfigMap", "cm", "ns"),
		testObject("v1", "Service", "svc", "ns"),
	}

	ml := &ManifestLayout{
		Name:      "app",
		Namespace: "ns",
		Resources: objs,
	}

	dir := t.TempDir()
	if err := ml.WriteToDisk(dir); err != nil {
		t.Fatalf("WriteToDisk failed: %v", err)
	}

	kustomFile := filepath.Join(dir, "ns", "app", "kustomization.yaml")
	data, err := os.ReadFile(kustomFile)
	if err != nil {
		t.Fatalf("expected kustomization.yaml at %s: %v", kustomFile, err)
	}

	content := string(data)
	if !strings.Contains(content, "apiVersion: kustomize.config.kubernetes.io/v1beta1") {
		t.Errorf("kustomization missing apiVersion, got:\n%s", content)
	}
	if !strings.Contains(content, "kind: Kustomization") {
		t.Errorf("kustomization missing kind, got:\n%s", content)
	}
	if !strings.Contains(content, "resources:") {
		t.Errorf("kustomization missing resources section, got:\n%s", content)
	}
	if !strings.Contains(content, "ns-configmap-cm.yaml") {
		t.Errorf("kustomization missing configmap reference, got:\n%s", content)
	}
	if !strings.Contains(content, "ns-service-svc.yaml") {
		t.Errorf("kustomization missing service reference, got:\n%s", content)
	}
}
