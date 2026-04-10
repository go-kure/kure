package layout

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"sort"
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
