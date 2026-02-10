package layout

import (
	"archive/tar"
	"bytes"
	"io"
	"sort"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// extractTarFiles reads all entries from a tar archive into a map.
func extractTarFiles(t *testing.T, buf *bytes.Buffer) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	tr := tar.NewReader(buf)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
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
