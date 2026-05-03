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

func TestWriteManifest_ClusterRootEmptyContainerNoKustomization(t *testing.T) {
	// Cluster root acting as a structural container: no own resources, only
	// child layouts. No kustomization.yaml at this level — children own their
	// own kustomization.yaml. Preserves the original walkClusterWithClusterName
	// behaviour.
	ml := &ManifestLayout{
		Name:      "",
		Namespace: "mycluster",
		Children: []*ManifestLayout{
			{
				Name:      "child",
				Namespace: "mycluster/child",
				Resources: []client.Object{testObject("v1", "Namespace", "default", "")},
			},
		},
	}

	cfg := DefaultLayoutConfig()
	dir := t.TempDir()

	if err := WriteManifest(dir, cfg, ml); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	kustomFile := filepath.Join(dir, "clusters", "mycluster", "kustomization.yaml")
	if _, err := os.Stat(kustomFile); !os.IsNotExist(err) {
		t.Errorf("kustomization.yaml should NOT be generated at empty cluster root (%s)", kustomFile)
	}
}

func TestWriteManifest_ClusterRootWithResourcesEmitsKustomization(t *testing.T) {
	// Cluster root carrying its own resources (e.g. after FlattenSingleTier
	// collapsed a child up): the directory has manifests, so it must have a
	// kustomization.yaml referencing them. Regression guard for the writer
	// relaxation.
	obj := testObject("v1", "Namespace", "default", "")

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

	resourceFile := filepath.Join(dir, "clusters", "mycluster", "cluster-namespace-default.yaml")
	if _, err := os.Stat(resourceFile); err != nil {
		t.Fatalf("expected resource file at %s: %v", resourceFile, err)
	}

	kustomFile := filepath.Join(dir, "clusters", "mycluster", "kustomization.yaml")
	data, err := os.ReadFile(kustomFile)
	if err != nil {
		t.Fatalf("expected kustomization.yaml at cluster root with resources: %v", err)
	}
	if !strings.Contains(string(data), "cluster-namespace-default.yaml") {
		t.Errorf("kustomization.yaml should reference the resource file, got:\n%s", data)
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
// FluxKustomizationMode per FluxPlacement tests (#265)
// ---------------------------------------------------------------------------

func TestWriteManifest_FluxKustomizationMode_PerPlacement(t *testing.T) {
	// Parent with FluxIntegrated, child with resources
	child := &ManifestLayout{
		Name:      "team-a",
		Namespace: "cl/ns/root/team-a",
		Resources: []client.Object{
			testObject("v1", "ConfigMap", "ca", "ns"),
			testObject("v1", "Secret", "sa", "ns"),
		},
	}

	root := &ManifestLayout{
		Name:          "root",
		Namespace:     "cl/ns",
		FluxPlacement: FluxIntegrated,
		Resources:     []client.Object{testObject("v1", "ConfigMap", "root-cfg", "ns")},
		Children:      []*ManifestLayout{child},
	}

	cfg := DefaultLayoutConfig()
	cfg.FluxKustomizationMode = map[FluxPlacement]KustomizationMode{
		FluxIntegrated: KustomizationRecursive,
	}
	dir := t.TempDir()

	if err := WriteManifest(dir, cfg, root); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	// Root's kustomization should NOT list its own resource files (recursive mode)
	rootK := filepath.Join(dir, "clusters", "cl", "ns", "root", "kustomization.yaml")
	data, err := os.ReadFile(rootK)
	if err != nil {
		t.Fatalf("read root kustomization: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "ns-configmap-root-cfg.yaml") {
		t.Errorf("recursive mode should NOT list resource files when children exist, got:\n%s", content)
	}
	// Should still reference child via flux kustomization reference
	if !strings.Contains(content, "flux-system-kustomization-team-a.yaml") {
		t.Errorf("expected flux kustomization reference for team-a, got:\n%s", content)
	}

	// Child (leaf) kustomization should list its own files regardless of parent mode
	childK := filepath.Join(dir, "clusters", "cl", "ns", "root", "team-a", "kustomization.yaml")
	childData, err := os.ReadFile(childK)
	if err != nil {
		t.Fatalf("read child kustomization: %v", err)
	}
	if !strings.Contains(string(childData), "ns-configmap-ca.yaml") {
		t.Errorf("child leaf kustomization should list its files, got:\n%s", childData)
	}
}

func TestWriteManifest_FluxKustomizationMode_NoOverride(t *testing.T) {
	// FluxSeparate without override should use default KustomizationMode
	child := &ManifestLayout{
		Name:      "apps",
		Namespace: "cl/ns/root/apps",
		Resources: []client.Object{testObject("v1", "ConfigMap", "c", "ns")},
	}

	root := &ManifestLayout{
		Name:          "root",
		Namespace:     "cl/ns",
		FluxPlacement: FluxSeparate,
		Resources:     []client.Object{testObject("v1", "Secret", "s", "ns")},
		Children:      []*ManifestLayout{child},
	}

	cfg := DefaultLayoutConfig()
	cfg.FluxKustomizationMode = map[FluxPlacement]KustomizationMode{
		FluxIntegrated: KustomizationRecursive,
	}
	dir := t.TempDir()

	if err := WriteManifest(dir, cfg, root); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	// Root's kustomization should list resource files (explicit is default)
	rootK := filepath.Join(dir, "clusters", "cl", "ns", "root", "kustomization.yaml")
	data, err := os.ReadFile(rootK)
	if err != nil {
		t.Fatalf("read root kustomization: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "ns-secret-s.yaml") {
		t.Errorf("explicit mode should list resource files, got:\n%s", content)
	}
	if !strings.Contains(content, "  - apps\n") {
		t.Errorf("FluxSeparate should reference child as plain directory, got:\n%s", content)
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
	if !strings.Contains(content, "apiVersion: kustomize.config.k8s.io/v1beta1") {
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

func TestWriteManifest_UmbrellaChild(t *testing.T) {
	// Mirrors the layout produced by the Flux LayoutIntegrator in
	// FluxIntegrated mode: the parent carries the child's Kustomization CR
	// in Resources (via placeUmbrellaChildrenFlux) and an UmbrellaChild
	// sub-layout in Children. Asserts the parent kustomization.yaml
	// references the child CR filename exactly once (no duplication), and
	// each child subdir carries its own workloads + own kustomization.yaml
	// with no flux CR file.
	childKustCR := testObject("kustomize.toolkit.fluxcd.io/v1", "Kustomization", "infra", "flux-system")

	child := &ManifestLayout{
		Name:          "infra",
		Namespace:     "mycluster/apps/platform/infra",
		UmbrellaChild: true,
		Resources: []client.Object{
			testObject("v1", "ConfigMap", "cfg", "default"),
		},
	}

	parent := &ManifestLayout{
		Name:      "platform",
		Namespace: "mycluster/apps/platform",
		Resources: []client.Object{childKustCR},
		Children:  []*ManifestLayout{child},
	}

	cfg := DefaultLayoutConfig()
	dir := t.TempDir()
	if err := WriteManifest(dir, cfg, parent); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	parentKustom := filepath.Join(dir, "clusters", "mycluster", "apps", "platform", "kustomization.yaml")
	data, err := os.ReadFile(parentKustom)
	if err != nil {
		t.Fatalf("failed to read parent kustomization.yaml: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "flux-system-kustomization-infra.yaml") {
		t.Errorf("parent kustomization should reference flux-system-kustomization-infra.yaml, got:\n%s", content)
	}
	if got := strings.Count(content, "flux-system-kustomization-infra.yaml"); got != 1 {
		t.Errorf("flux-system-kustomization-infra.yaml should appear exactly once in parent kustomization.yaml, got %d occurrences:\n%s", got, content)
	}
	if strings.Contains(content, "\n  - infra\n") {
		t.Errorf("umbrella child must NOT be referenced as plain subdirectory, got:\n%s", content)
	}

	// Parent directory also contains the child's Kustomization CR file.
	parentCRFile := filepath.Join(dir, "clusters", "mycluster", "apps", "platform", "flux-system-kustomization-infra.yaml")
	if _, err := os.Stat(parentCRFile); err != nil {
		t.Errorf("expected child Kustomization CR file at parent layer: %v", err)
	}

	// Child directory exists with its own workload + kustomization.yaml
	childDir := filepath.Join(dir, "clusters", "mycluster", "apps", "platform", "infra")
	if _, err := os.Stat(filepath.Join(childDir, "default-configmap-cfg.yaml")); err != nil {
		t.Errorf("expected umbrella child workload file: %v", err)
	}
	childKustomPath := filepath.Join(childDir, "kustomization.yaml")
	if _, err := os.Stat(childKustomPath); err != nil {
		t.Errorf("expected umbrella child kustomization.yaml: %v", err)
	}

	// Child dir must NOT contain any flux-system-kustomization-* file.
	entries, err := os.ReadDir(childDir)
	if err != nil {
		t.Fatalf("read child dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "flux-system-kustomization-") {
			t.Errorf("umbrella child subdir contains flux CR file it should not: %s", e.Name())
		}
	}
}

func TestWriteManifest_FileNamingKindName_FluxIntegrated(t *testing.T) {
	child := &ManifestLayout{
		Name:       "team-a",
		Namespace:  "cl/flux-system/team-a",
		FileNaming: FileNamingKindName,
		Resources:  []client.Object{testObject("v1", "ConfigMap", "ca", "flux-system")},
	}

	root := &ManifestLayout{
		Name:          "flux-root",
		Namespace:     "cl/flux-system",
		FluxPlacement: FluxIntegrated,
		FileNaming:    FileNamingKindName,
		Children:      []*ManifestLayout{child},
	}

	cfg := DefaultLayoutConfig()
	cfg.FileNaming = FileNamingKindName
	cfg.ManifestFileName = nil // Let FileNaming take effect
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
	// With FileNamingKindName, flux kustomization reference should be kustomization-team-a.yaml
	if !strings.Contains(content, "kustomization-team-a.yaml") {
		t.Errorf("expected kustomization-team-a.yaml reference, got:\n%s", content)
	}
	if strings.Contains(content, "flux-system-kustomization-team-a.yaml") {
		t.Errorf("should not have namespace-prefixed flux kustomization reference, got:\n%s", content)
	}

	// Child resource should use kind-name format
	childResFile := filepath.Join(dir, "clusters", "cl", "flux-system", "team-a", "configmap-ca.yaml")
	if _, err := os.Stat(childResFile); err != nil {
		t.Errorf("expected kind-name resource file at %s: %v", childResFile, err)
	}
}

func TestWriteToDisk_FileNamingKindName(t *testing.T) {
	ml := &ManifestLayout{
		Name:       "apps",
		Namespace:  "default",
		FilePer:    FilePerResource,
		FileNaming: FileNamingKindName,
		Mode:       KustomizationExplicit,
		Resources: []client.Object{
			testObject("v1", "Service", "web", "default"),
			testObject("apps/v1", "Deployment", "web", "default"),
		},
	}

	dir := t.TempDir()
	if err := ml.WriteToDisk(dir); err != nil {
		t.Fatalf("WriteToDisk failed: %v", err)
	}

	// With FileNamingKindName, files should be {kind}-{name}.yaml
	svcFile := filepath.Join(dir, "default", "apps", "service-web.yaml")
	if _, err := os.Stat(svcFile); err != nil {
		t.Errorf("expected service-web.yaml at %s: %v", svcFile, err)
	}
	deployFile := filepath.Join(dir, "default", "apps", "deployment-web.yaml")
	if _, err := os.Stat(deployFile); err != nil {
		t.Errorf("expected deployment-web.yaml at %s: %v", deployFile, err)
	}

	// Should NOT have namespace-prefixed files
	oldSvcFile := filepath.Join(dir, "default", "apps", "default-service-web.yaml")
	if _, err := os.Stat(oldSvcFile); err == nil {
		t.Errorf("unexpected namespace-prefixed file: %s", oldSvcFile)
	}

	// Kustomization should reference kind-name files
	kustomFile := filepath.Join(dir, "default", "apps", "kustomization.yaml")
	data, err := os.ReadFile(kustomFile)
	if err != nil {
		t.Fatalf("read kustomization: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "service-web.yaml") {
		t.Errorf("kustomization.yaml should reference service-web.yaml:\n%s", content)
	}
	if !strings.Contains(content, "deployment-web.yaml") {
		t.Errorf("kustomization.yaml should reference deployment-web.yaml:\n%s", content)
	}
}

// TestWriteToDisk_NamespaceDot_RootPaths verifies that Namespace:"." on layout
// nodes produces paths relative to basePath (no "cluster/" prefix), mirroring
// the tar behaviour. This is the OCI root convention from docs/oci-layout.md.
func TestWriteToDisk_NamespaceDot_RootPaths(t *testing.T) {
	fluxSystem := &ManifestLayout{
		Name:       "flux-system",
		Namespace:  ".",
		FilePer:    FilePerResource,
		FileNaming: FileNamingKindName,
		Mode:       KustomizationExplicit,
		Resources: []client.Object{
			testObject("v1", "ConfigMap", "gotk-components", "flux-system"),
		},
	}
	kustPlatform := &unstructured.Unstructured{}
	kustPlatform.SetAPIVersion("kustomize.toolkit.fluxcd.io/v1")
	kustPlatform.SetKind("Kustomization")
	kustPlatform.SetName("cert-manager")
	kustPlatform.SetNamespace("flux-system")
	kustPlatform.Object["spec"] = map[string]interface{}{
		"path": "./platform/cert-manager",
		"sourceRef": map[string]interface{}{
			"kind": "OCIRepository",
			"name": "stack-prod",
		},
	}
	fluxSystemPlatform := &ManifestLayout{
		Name:       "flux-system-platform",
		Namespace:  ".",
		FilePer:    FilePerResource,
		FileNaming: FileNamingKindName,
		Mode:       KustomizationExplicit,
		Resources:  []client.Object{kustPlatform},
	}
	certManager := &ManifestLayout{
		Name:       "cert-manager",
		Namespace:  "platform",
		FilePer:    FilePerResource,
		FileNaming: FileNamingKindName,
		Mode:       KustomizationExplicit,
		Resources: []client.Object{
			testObject("v1", "ConfigMap", "cert-manager-config", "cert-manager"),
		},
	}

	dir := t.TempDir()
	for _, ml := range []*ManifestLayout{fluxSystem, fluxSystemPlatform, certManager} {
		if err := ml.WriteToDisk(dir); err != nil {
			t.Fatalf("WriteToDisk(%s) failed: %v", ml.Name, err)
		}
	}

	// Layer 1: directly under basePath, not under basePath/cluster/
	if _, err := os.Stat(filepath.Join(dir, "flux-system", "configmap-gotk-components.yaml")); err != nil {
		t.Errorf("Layer 1: expected configmap-gotk-components.yaml at flux-system/: %v", err)
	}
	// Layer 2: kind-name filename (not namespace-prefixed), spec fields preserved.
	kustFile := filepath.Join(dir, "flux-system-platform", "kustomization-cert-manager.yaml")
	if _, err := os.Stat(kustFile); err != nil {
		t.Errorf("Layer 2: expected kustomization-cert-manager.yaml (FileNamingKindName): %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "flux-system-platform", "flux-system-kustomization-cert-manager.yaml")); err == nil {
		t.Error("Layer 2: namespace-prefixed filename found; FileNamingKindName must suppress it")
	}
	kustData, err := os.ReadFile(kustFile)
	if err != nil {
		t.Fatalf("read Layer 2 Kustomization: %v", err)
	}
	kustContent := string(kustData)
	if !strings.Contains(kustContent, "path: ./platform/cert-manager") {
		t.Errorf("Layer 2 Kustomization must have spec.path ./platform/cert-manager:\n%s", kustContent)
	}
	if !strings.Contains(kustContent, "name: stack-prod") {
		t.Errorf("Layer 2 Kustomization must have spec.sourceRef.name stack-prod:\n%s", kustContent)
	}
	// Layer 3: <group>/<appname>/ path
	if _, err := os.Stat(filepath.Join(dir, "platform", "cert-manager")); err != nil {
		t.Errorf("Layer 3: expected platform/cert-manager/ at basePath root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "platform", "cert-manager", "configmap-cert-manager-config.yaml")); err != nil {
		t.Errorf("Layer 3: expected resource file in platform/cert-manager/: %v", err)
	}
	// Must NOT have a "cluster/" directory.
	if _, err := os.Stat(filepath.Join(dir, "cluster")); err == nil {
		t.Error("unexpected 'cluster/' directory: Namespace:'.' should not create it")
	}
}

func TestWriteToDisk_FileNamingKindName_FluxIntegrated(t *testing.T) {
	child := &ManifestLayout{
		Name:       "team-a",
		Namespace:  "cl/flux-system/team-a",
		FilePer:    FilePerResource,
		FileNaming: FileNamingKindName,
		Mode:       KustomizationExplicit,
		Resources:  []client.Object{testObject("v1", "ConfigMap", "ca", "flux-system")},
	}

	root := &ManifestLayout{
		Name:          "flux-root",
		Namespace:     "cl/flux-system",
		FluxPlacement: FluxIntegrated,
		FilePer:       FilePerResource,
		FileNaming:    FileNamingKindName,
		Mode:          KustomizationExplicit,
		Children:      []*ManifestLayout{child},
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
	if !strings.Contains(content, "kustomization-team-a.yaml") {
		t.Errorf("expected kustomization-team-a.yaml reference, got:\n%s", content)
	}
	if strings.Contains(content, "flux-system-kustomization-team-a.yaml") {
		t.Errorf("should not have namespace-prefixed flux reference, got:\n%s", content)
	}
}
