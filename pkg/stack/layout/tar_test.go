package layout

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"sort"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestWriteToTar_BasicLayout(t *testing.T) {
	ml := &ManifestLayout{
		Name:      "apps",
		Namespace: "default",
		FilePer:   FilePerResource,
		Mode:      KustomizationExplicit,
		Resources: []client.Object{
			&corev1.Service{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "web",
					Namespace: "default",
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := ml.WriteToTar(&buf); err != nil {
		t.Fatalf("WriteToTar failed: %v", err)
	}

	files := extractTarFiles(t, &buf)

	// Expect directory + resource file + kustomization.yaml
	expectedFiles := []string{
		"default/apps/",
		"default/apps/default-service-web.yaml",
		"default/apps/kustomization.yaml",
	}
	for _, expected := range expectedFiles {
		if _, ok := files[expected]; !ok {
			t.Errorf("missing expected file: %s (got: %v)", expected, fileNames(files))
		}
	}

	// Verify kustomization.yaml references the service file
	kustomContent := string(files["default/apps/kustomization.yaml"])
	if !bytes.Contains([]byte(kustomContent), []byte("default-service-web.yaml")) {
		t.Errorf("kustomization.yaml should reference service file:\n%s", kustomContent)
	}
}

func TestWriteToTar_NestedLayout(t *testing.T) {
	child := &ManifestLayout{
		Name:      "child",
		Namespace: "default/parent/child",
		FilePer:   FilePerResource,
		Mode:      KustomizationExplicit,
		Resources: []client.Object{
			&appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app",
					Namespace: "default",
				},
			},
		},
	}

	parent := &ManifestLayout{
		Name:      "parent",
		Namespace: "default/parent",
		FilePer:   FilePerResource,
		Mode:      KustomizationExplicit,
		Children:  []*ManifestLayout{child},
	}

	var buf bytes.Buffer
	if err := parent.WriteToTar(&buf); err != nil {
		t.Fatalf("WriteToTar failed: %v", err)
	}

	files := extractTarFiles(t, &buf)

	// Check parent has kustomization referencing child
	parentKustom := string(files["default/parent/kustomization.yaml"])
	if !bytes.Contains([]byte(parentKustom), []byte("- child")) {
		t.Errorf("parent kustomization should reference child:\n%s", parentKustom)
	}

	// Check child has deployment
	if _, ok := files["default/parent/child/default-deployment-app.yaml"]; !ok {
		t.Errorf("missing child deployment file (got: %v)", fileNames(files))
	}
}

func TestWriteToTar_Deterministic(t *testing.T) {
	ml := &ManifestLayout{
		Name:      "apps",
		Namespace: "ns",
		FilePer:   FilePerResource,
		Mode:      KustomizationExplicit,
		Resources: []client.Object{
			&corev1.Service{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
				ObjectMeta: metav1.ObjectMeta{Name: "bravo", Namespace: "ns"},
			},
			&corev1.Service{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
				ObjectMeta: metav1.ObjectMeta{Name: "alpha", Namespace: "ns"},
			},
		},
	}

	// Write twice and compare
	var buf1, buf2 bytes.Buffer
	if err := ml.WriteToTar(&buf1); err != nil {
		t.Fatalf("first WriteToTar failed: %v", err)
	}
	if err := ml.WriteToTar(&buf2); err != nil {
		t.Fatalf("second WriteToTar failed: %v", err)
	}

	if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		t.Error("WriteToTar output is not deterministic")
	}
}

func TestWriteToTar_FluxIntegrated(t *testing.T) {
	child := &ManifestLayout{
		Name:      "team-a",
		Namespace: "cl/flux-system/team-a",
		FilePer:   FilePerResource,
		Mode:      KustomizationExplicit,
		Resources: []client.Object{
			&corev1.ConfigMap{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
				ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: "flux-system"},
			},
		},
	}

	root := &ManifestLayout{
		Name:          "flux-root",
		Namespace:     "cl/flux-system",
		FluxPlacement: FluxIntegrated,
		FilePer:       FilePerResource,
		Mode:          KustomizationExplicit,
		Children:      []*ManifestLayout{child},
	}

	var buf bytes.Buffer
	if err := root.WriteToTar(&buf); err != nil {
		t.Fatalf("WriteToTar failed: %v", err)
	}

	files := extractTarFiles(t, &buf)

	rootKustom := string(files["cl/flux-system/flux-root/kustomization.yaml"])
	if !bytes.Contains([]byte(rootKustom), []byte("flux-system-kustomization-team-a.yaml")) {
		t.Errorf("expected flux kustomization reference for team-a, got:\n%s", rootKustom)
	}
	if bytes.Contains([]byte(rootKustom), []byte("  - team-a\n")) {
		t.Errorf("should not reference child as plain directory in flux integrated mode, got:\n%s", rootKustom)
	}
}

func TestWriteToTar_UmbrellaChild(t *testing.T) {
	// Mirrors the layout produced by the Flux LayoutIntegrator in
	// FluxIntegrated mode: the umbrella parent carries the child's
	// Kustomization CR in Resources (placed there by placeUmbrellaChildrenFlux),
	// and an UmbrellaChild sub-layout in Children (carrying the child's
	// workloads). Verifies:
	//   1) The parent kustomization.yaml references the child CR filename
	//      exactly once (duplication regression test — the Children-loop
	//      UmbrellaChild branch used to emit the same entry a second time).
	//   2) The umbrella child is NOT referenced as a plain subdirectory.
	//   3) The child subdirectory contains its own workload + kustomization.yaml.
	//   4) The child subdirectory does NOT contain any flux-system-kustomization-* file.
	childKust := &unstructured.Unstructured{}
	childKust.SetAPIVersion("kustomize.toolkit.fluxcd.io/v1")
	childKust.SetKind("Kustomization")
	childKust.SetName("infra")
	childKust.SetNamespace("flux-system")

	child := &ManifestLayout{
		Name:          "infra",
		Namespace:     "cl/apps/platform/infra",
		FilePer:       FilePerResource,
		Mode:          KustomizationExplicit,
		UmbrellaChild: true,
		Resources: []client.Object{
			&corev1.ConfigMap{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
				ObjectMeta: metav1.ObjectMeta{Name: "cfg", Namespace: "default"},
			},
		},
	}

	parent := &ManifestLayout{
		Name:      "platform",
		Namespace: "cl/apps/platform",
		FilePer:   FilePerResource,
		Mode:      KustomizationExplicit,
		Resources: []client.Object{childKust},
		Children:  []*ManifestLayout{child},
	}

	var buf bytes.Buffer
	if err := parent.WriteToTar(&buf); err != nil {
		t.Fatalf("WriteToTar failed: %v", err)
	}

	files := extractTarFiles(t, &buf)

	parentKustom, ok := files["cl/apps/platform/kustomization.yaml"]
	if !ok {
		t.Fatalf("missing parent kustomization.yaml, got files: %v", fileNames(files))
	}
	if !bytes.Contains(parentKustom, []byte("flux-system-kustomization-infra.yaml")) {
		t.Errorf("expected parent to reference flux-system-kustomization-infra.yaml, got:\n%s", parentKustom)
	}
	if got := bytes.Count(parentKustom, []byte("flux-system-kustomization-infra.yaml")); got != 1 {
		t.Errorf("flux-system-kustomization-infra.yaml should appear exactly once in parent kustomization.yaml, got %d occurrences:\n%s", got, parentKustom)
	}
	if bytes.Contains(parentKustom, []byte("  - infra\n")) {
		t.Errorf("umbrella child should NOT be referenced as plain subdirectory, got:\n%s", parentKustom)
	}

	// The parent directory also contains the child's Kustomization CR file.
	if _, ok := files["cl/apps/platform/flux-system-kustomization-infra.yaml"]; !ok {
		t.Errorf("missing child Kustomization CR file at parent layer, got: %v", fileNames(files))
	}

	// Child subdir contains workload + its own kustomization.yaml
	if _, ok := files["cl/apps/platform/infra/default-configmap-cfg.yaml"]; !ok {
		t.Errorf("missing umbrella child workload file, got: %v", fileNames(files))
	}
	childKustom, ok := files["cl/apps/platform/infra/kustomization.yaml"]
	if !ok {
		t.Errorf("missing umbrella child kustomization.yaml, got: %v", fileNames(files))
	}
	if !bytes.Contains(childKustom, []byte("default-configmap-cfg.yaml")) {
		t.Errorf("umbrella child kustomization should list workload file, got:\n%s", childKustom)
	}

	// The umbrella child subdir must NOT contain any flux-system-kustomization-* file
	for name := range files {
		if bytes.HasPrefix([]byte(name), []byte("cl/apps/platform/infra/flux-system-kustomization-")) {
			t.Errorf("umbrella child subdir contains flux CR file it should not: %s", name)
		}
	}
}

func TestWriteToTar_FileNamingKindName(t *testing.T) {
	ml := &ManifestLayout{
		Name:       "apps",
		Namespace:  "default",
		FilePer:    FilePerResource,
		FileNaming: FileNamingKindName,
		Mode:       KustomizationExplicit,
		Resources: []client.Object{
			&corev1.Service{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
				ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
			},
			&appsv1.Deployment{
				TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
				ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
			},
		},
	}

	var buf bytes.Buffer
	if err := ml.WriteToTar(&buf); err != nil {
		t.Fatalf("WriteToTar failed: %v", err)
	}

	files := extractTarFiles(t, &buf)

	// With FileNamingKindName, files should be {kind}-{name}.yaml (no namespace prefix)
	expectedFiles := []string{
		"default/apps/",
		"default/apps/service-web.yaml",
		"default/apps/deployment-web.yaml",
		"default/apps/kustomization.yaml",
	}
	for _, expected := range expectedFiles {
		if _, ok := files[expected]; !ok {
			t.Errorf("missing expected file: %s (got: %v)", expected, fileNames(files))
		}
	}

	// Should NOT have namespace-prefixed files
	for name := range files {
		if name == "default/apps/default-service-web.yaml" || name == "default/apps/default-deployment-web.yaml" {
			t.Errorf("unexpected namespace-prefixed file: %s", name)
		}
	}

	// Kustomization should reference the kind-name files
	kustomContent := string(files["default/apps/kustomization.yaml"])
	if !bytes.Contains([]byte(kustomContent), []byte("service-web.yaml")) {
		t.Errorf("kustomization.yaml should reference service-web.yaml:\n%s", kustomContent)
	}
	if !bytes.Contains([]byte(kustomContent), []byte("deployment-web.yaml")) {
		t.Errorf("kustomization.yaml should reference deployment-web.yaml:\n%s", kustomContent)
	}
}

func TestWriteToTar_FileNamingKindName_FluxIntegrated(t *testing.T) {
	child := &ManifestLayout{
		Name:       "team-a",
		Namespace:  "cl/flux-system/team-a",
		FilePer:    FilePerResource,
		FileNaming: FileNamingKindName,
		Mode:       KustomizationExplicit,
		Resources: []client.Object{
			&corev1.ConfigMap{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
				ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: "flux-system"},
			},
		},
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

	var buf bytes.Buffer
	if err := root.WriteToTar(&buf); err != nil {
		t.Fatalf("WriteToTar failed: %v", err)
	}

	files := extractTarFiles(t, &buf)

	// With FileNamingKindName, the flux kustomization reference should be
	// kustomization-team-a.yaml (not flux-system-kustomization-team-a.yaml)
	rootKustom := string(files["cl/flux-system/flux-root/kustomization.yaml"])
	if !bytes.Contains([]byte(rootKustom), []byte("kustomization-team-a.yaml")) {
		t.Errorf("expected kustomization-team-a.yaml reference, got:\n%s", rootKustom)
	}
	if bytes.Contains([]byte(rootKustom), []byte("flux-system-kustomization-team-a.yaml")) {
		t.Errorf("should not have namespace-prefixed flux kustomization reference, got:\n%s", rootKustom)
	}

	// Child resource should use kind-name format
	if _, ok := files["cl/flux-system/team-a/configmap-ca.yaml"]; !ok {
		t.Errorf("missing kind-name child resource file (got: %v)", fileNames(files))
	}
}

func TestWriteToTar_FilePerKind_FluxIntegrated(t *testing.T) {
	// Regression: FilePerKind must not collapse FluxIntegrated kustomization
	// references — each child needs a unique filename including child.Name.
	childA := &ManifestLayout{
		Name:      "team-a",
		Namespace: "cl/flux-system/team-a",
		FilePer:   FilePerKind,
		Mode:      KustomizationExplicit,
		Resources: []client.Object{
			&corev1.ConfigMap{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
				ObjectMeta: metav1.ObjectMeta{Name: "cfg-a", Namespace: "flux-system"},
			},
		},
	}
	childB := &ManifestLayout{
		Name:      "team-b",
		Namespace: "cl/flux-system/team-b",
		FilePer:   FilePerKind,
		Mode:      KustomizationExplicit,
		Resources: []client.Object{
			&corev1.ConfigMap{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
				ObjectMeta: metav1.ObjectMeta{Name: "cfg-b", Namespace: "flux-system"},
			},
		},
	}

	root := &ManifestLayout{
		Name:          "flux-root",
		Namespace:     "cl/flux-system",
		FluxPlacement: FluxIntegrated,
		FilePer:       FilePerKind,
		Mode:          KustomizationExplicit,
		Children:      []*ManifestLayout{childA, childB},
	}

	var buf bytes.Buffer
	if err := root.WriteToTar(&buf); err != nil {
		t.Fatalf("WriteToTar failed: %v", err)
	}

	files := extractTarFiles(t, &buf)
	rootKustom := string(files["cl/flux-system/flux-root/kustomization.yaml"])

	// Each child must have a distinct reference
	if !bytes.Contains([]byte(rootKustom), []byte("flux-system-kustomization-team-a.yaml")) {
		t.Errorf("expected flux-system-kustomization-team-a.yaml reference, got:\n%s", rootKustom)
	}
	if !bytes.Contains([]byte(rootKustom), []byte("flux-system-kustomization-team-b.yaml")) {
		t.Errorf("expected flux-system-kustomization-team-b.yaml reference, got:\n%s", rootKustom)
	}
}

// ---------------------------------------------------------------------------
// OCI layout pattern tests (docs/oci-layout.md)
// ---------------------------------------------------------------------------

// TestWriteToTar_NamespaceDot_TarRoot verifies that Namespace:"." on a layout
// node produces paths relative to the archive root (no extra prefix directory).
// This is the convention used by crane when emitting OCI artifact layouts;
// see docs/oci-layout.md §Kure responsibilities.
func TestWriteToTar_NamespaceDot_TarRoot(t *testing.T) {
	child := &ManifestLayout{
		Name:       "flux-system",
		Namespace:  ".",
		FilePer:    FilePerResource,
		FileNaming: FileNamingKindName,
		Mode:       KustomizationExplicit,
		Resources: []client.Object{
			&corev1.ConfigMap{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
				ObjectMeta: metav1.ObjectMeta{Name: "gotk-components", Namespace: "flux-system"},
			},
		},
	}
	root := &ManifestLayout{
		Name:      "",
		Namespace: ".",
		FilePer:   FilePerResource,
		Mode:      KustomizationExplicit,
		Children:  []*ManifestLayout{child},
	}

	var buf bytes.Buffer
	if err := root.WriteToTar(&buf); err != nil {
		t.Fatalf("WriteToTar failed: %v", err)
	}

	files := extractTarFiles(t, &buf)

	// Directory must be at archive root, not under "cluster/" or any prefix.
	if _, ok := files["flux-system/"]; !ok {
		t.Errorf("expected flux-system/ at tar root, got: %v", fileNames(files))
	}
	// File uses kind-name naming and lives directly under flux-system/.
	if _, ok := files["flux-system/configmap-gotk-components.yaml"]; !ok {
		t.Errorf("expected configmap-gotk-components.yaml in flux-system/, got: %v", fileNames(files))
	}
	// No spurious "cluster/" prefix anywhere.
	for name := range files {
		if strings.HasPrefix(name, "cluster/") {
			t.Errorf("unexpected 'cluster/' prefix in tar entry %q", name)
		}
	}
}

// TestWriteToTar_OCILayer2GroupNaming verifies that a Layer 2 directory uses the
// flux-system-<group> naming convention (e.g. flux-system-frontend/) and that
// Kustomization CRs inside it use kind-name filenames (kustomization-<app>.yaml).
// See docs/oci-layout.md Layer 2 row.
func TestWriteToTar_OCILayer2GroupNaming(t *testing.T) {
	kustCR := &unstructured.Unstructured{}
	kustCR.SetAPIVersion("kustomize.toolkit.fluxcd.io/v1")
	kustCR.SetKind("Kustomization")
	kustCR.SetName("storefront")
	kustCR.SetNamespace("flux-system")

	ml := &ManifestLayout{
		Name:       "flux-system-frontend",
		Namespace:  ".",
		FilePer:    FilePerResource,
		FileNaming: FileNamingKindName,
		Mode:       KustomizationExplicit,
		Resources:  []client.Object{kustCR},
	}

	var buf bytes.Buffer
	if err := ml.WriteToTar(&buf); err != nil {
		t.Fatalf("WriteToTar failed: %v", err)
	}

	files := extractTarFiles(t, &buf)

	if _, ok := files["flux-system-frontend/"]; !ok {
		t.Errorf("expected flux-system-frontend/ at tar root, got: %v", fileNames(files))
	}
	// Layer 2 Kustomization CR uses kind-name format: kustomization-<appname>.yaml
	if _, ok := files["flux-system-frontend/kustomization-storefront.yaml"]; !ok {
		t.Errorf("expected kustomization-storefront.yaml in flux-system-frontend/, got: %v", fileNames(files))
	}
	kustom := string(files["flux-system-frontend/kustomization.yaml"])
	if !bytes.Contains([]byte(kustom), []byte("kustomization-storefront.yaml")) {
		t.Errorf("flux-system-frontend kustomization.yaml should reference kustomization-storefront.yaml:\n%s", kustom)
	}
}

// TestWriteToTar_OCILayer3PayloadPath verifies that a Layer 3 application
// payload directory follows the <group>/<appname>/ path convention (e.g.
// frontend/storefront/), constructed from Namespace:<group> and Name:<appname>.
// See docs/oci-layout.md Layer 3 row.
func TestWriteToTar_OCILayer3PayloadPath(t *testing.T) {
	ml := &ManifestLayout{
		Name:       "storefront",
		Namespace:  "frontend",
		FilePer:    FilePerResource,
		FileNaming: FileNamingKindName,
		Mode:       KustomizationExplicit,
		Resources: []client.Object{
			&appsv1.Deployment{
				TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
				ObjectMeta: metav1.ObjectMeta{Name: "storefront", Namespace: "frontend"},
			},
			&corev1.Service{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
				ObjectMeta: metav1.ObjectMeta{Name: "storefront", Namespace: "frontend"},
			},
		},
	}

	var buf bytes.Buffer
	if err := ml.WriteToTar(&buf); err != nil {
		t.Fatalf("WriteToTar failed: %v", err)
	}

	files := extractTarFiles(t, &buf)

	// Layer 3 path: <group>/<appname>/ — both path segments present.
	if _, ok := files["frontend/storefront/"]; !ok {
		t.Errorf("expected frontend/storefront/ (Layer 3 path), got: %v", fileNames(files))
	}
	// FileNamingKindName: {kind}-{name}.yaml (no namespace prefix).
	if _, ok := files["frontend/storefront/deployment-storefront.yaml"]; !ok {
		t.Errorf("expected deployment-storefront.yaml in Layer 3 dir, got: %v", fileNames(files))
	}
	if _, ok := files["frontend/storefront/service-storefront.yaml"]; !ok {
		t.Errorf("expected service-storefront.yaml in Layer 3 dir, got: %v", fileNames(files))
	}
	// Must NOT use namespace-prefixed names.
	for name := range files {
		if strings.HasPrefix(name, "frontend/storefront/frontend-") {
			t.Errorf("namespace-prefixed filename found in Layer 3: %s", name)
		}
	}
}

// TestWriteToTar_OCIMonolithic_SiblingLayers verifies that the three OCI layers
// are siblings at the archive root (no nesting under a prefix directory) and
// that each layer uses the correct naming and file conventions:
//
//	Layer 1: flux-system/               – bootstrap root
//	Layer 2: flux-system-platform/      – group Kustomization CRs
//	Layer 3: platform/cert-manager/     – application manifests
//
// All nodes use Namespace:"." and FileNamingKindName, matching the contract
// documented in docs/oci-layout.md §Kure responsibilities.
func TestWriteToTar_OCIMonolithic_SiblingLayers(t *testing.T) {
	// Layer 1: flux-system/ with an OCIRepository CR
	ociCR := &unstructured.Unstructured{}
	ociCR.SetAPIVersion("source.toolkit.fluxcd.io/v1beta2")
	ociCR.SetKind("OCIRepository")
	ociCR.SetName("stack-prod")
	ociCR.SetNamespace("flux-system")

	fluxSystem := &ManifestLayout{
		Name:       "flux-system",
		Namespace:  ".",
		FilePer:    FilePerResource,
		FileNaming: FileNamingKindName,
		Mode:       KustomizationExplicit,
		Resources:  []client.Object{ociCR},
	}

	// Layer 2: flux-system-platform/ with a Kustomization CR
	kustCR := &unstructured.Unstructured{}
	kustCR.SetAPIVersion("kustomize.toolkit.fluxcd.io/v1")
	kustCR.SetKind("Kustomization")
	kustCR.SetName("cert-manager")
	kustCR.SetNamespace("flux-system")

	fluxSystemPlatform := &ManifestLayout{
		Name:       "flux-system-platform",
		Namespace:  ".",
		FilePer:    FilePerResource,
		FileNaming: FileNamingKindName,
		Mode:       KustomizationExplicit,
		Resources:  []client.Object{kustCR},
	}

	// Layer 3: platform/cert-manager/ with application manifests
	certManagerPayload := &ManifestLayout{
		Name:       "cert-manager",
		Namespace:  "platform",
		FilePer:    FilePerResource,
		FileNaming: FileNamingKindName,
		Mode:       KustomizationExplicit,
		Resources: []client.Object{
			&corev1.ConfigMap{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
				ObjectMeta: metav1.ObjectMeta{Name: "cert-manager-config", Namespace: "cert-manager"},
			},
		},
	}

	// Root at archive root (Namespace:"."), children are all three layers.
	root := &ManifestLayout{
		Name:      "",
		Namespace: ".",
		FilePer:   FilePerResource,
		Mode:      KustomizationExplicit,
		Children:  []*ManifestLayout{fluxSystem, fluxSystemPlatform, certManagerPayload},
	}

	var buf bytes.Buffer
	if err := root.WriteToTar(&buf); err != nil {
		t.Fatalf("WriteToTar failed: %v", err)
	}

	files := extractTarFiles(t, &buf)

	// All three layers must be at the archive root (no intermediate prefix).
	for _, dir := range []string{"flux-system/", "flux-system-platform/", "platform/cert-manager/"} {
		if _, ok := files[dir]; !ok {
			t.Errorf("expected %s at tar root (sibling), got: %v", dir, fileNames(files))
		}
	}

	// Layer 1 contents.
	if _, ok := files["flux-system/ocirepository-stack-prod.yaml"]; !ok {
		t.Errorf("Layer 1: missing ocirepository-stack-prod.yaml, got: %v", fileNames(files))
	}

	// Layer 2 contents — Kustomization CR uses kind-name format.
	if _, ok := files["flux-system-platform/kustomization-cert-manager.yaml"]; !ok {
		t.Errorf("Layer 2: missing kustomization-cert-manager.yaml, got: %v", fileNames(files))
	}

	// Layer 3 contents.
	if _, ok := files["platform/cert-manager/configmap-cert-manager-config.yaml"]; !ok {
		t.Errorf("Layer 3: missing resource file in platform/cert-manager/, got: %v", fileNames(files))
	}

	// Nothing has a "cluster/" prefix — Namespace:"." means archive root.
	for name := range files {
		if strings.HasPrefix(name, "cluster/") {
			t.Errorf("unexpected 'cluster/' prefix in tar entry: %s", name)
		}
	}
}

// extractTarFiles reads all entries from a tar archive into a map.
func extractTarFiles(t *testing.T, buf *bytes.Buffer) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	tr := tar.NewReader(buf)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("tar read error: %v", err)
		}
		var data []byte
		if hdr.Typeflag == tar.TypeReg {
			data, err = io.ReadAll(tr)
			if err != nil {
				t.Fatalf("tar read file error: %v", err)
			}
		}
		files[hdr.Name] = data
	}
	return files
}

// fileNames returns sorted keys from a file map for diagnostics.
func fileNames(files map[string][]byte) []string {
	names := make([]string, 0, len(files))
	for k := range files {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
