package fluxcd

import (
	"testing"

	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateOCIRepository(t *testing.T) {
	tests := []struct {
		name      string
		inputName string
		inputNS   string
		inputSpec sourcev1beta2.OCIRepositorySpec
		expected  *sourcev1beta2.OCIRepository
	}{
		{
			name:      "Valid input",
			inputName: "test-oci-repo",
			inputNS:   "default",
			inputSpec: sourcev1beta2.OCIRepositorySpec{
				URL: "https://registry.example.com/repo",
			},
			expected: &sourcev1beta2.OCIRepository{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HelmRepository",
					APIVersion: sourcev1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-oci-repo",
					Namespace: "default",
				},
				Spec: sourcev1beta2.OCIRepositorySpec{
					URL: "https://registry.example.com/repo",
				},
			},
		},
		{
			name:      "Empty name",
			inputName: "",
			inputNS:   "default",
			inputSpec: sourcev1beta2.OCIRepositorySpec{
				URL: "https://registry.example.com/repo",
			},
			expected: &sourcev1beta2.OCIRepository{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HelmRepository",
					APIVersion: sourcev1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "",
					Namespace: "default",
				},
				Spec: sourcev1beta2.OCIRepositorySpec{
					URL: "https://registry.example.com/repo",
				},
			},
		},
		{
			name:      "Empty namespace",
			inputName: "test-oci-repo",
			inputNS:   "",
			inputSpec: sourcev1beta2.OCIRepositorySpec{
				URL: "https://registry.example.com/repo",
			},
			expected: &sourcev1beta2.OCIRepository{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HelmRepository",
					APIVersion: sourcev1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-oci-repo",
					Namespace: "",
				},
				Spec: sourcev1beta2.OCIRepositorySpec{
					URL: "https://registry.example.com/repo",
				},
			},
		},
		{
			name:      "Empty spec",
			inputName: "test-oci-repo",
			inputNS:   "default",
			inputSpec: sourcev1beta2.OCIRepositorySpec{},
			expected: &sourcev1beta2.OCIRepository{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HelmRepository",
					APIVersion: sourcev1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-oci-repo",
					Namespace: "default",
				},
				Spec: sourcev1beta2.OCIRepositorySpec{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CreateOCIRepository(tt.inputName, tt.inputNS, tt.inputSpec)
			if result.TypeMeta != tt.expected.TypeMeta ||
				result.ObjectMeta.Name != tt.expected.ObjectMeta.Name ||
				result.ObjectMeta.Namespace != tt.expected.ObjectMeta.Namespace ||
				result.Spec.URL != tt.expected.Spec.URL {
				t.Errorf("CreateOCIRepository(%v, %v, %v) = %v, want %v",
					tt.inputName, tt.inputNS, tt.inputSpec, result, tt.expected)
			}
		})
	}
}

func TestCreateGitRepository(t *testing.T) {
	tests := []struct {
		name      string
		inputName string
		inputNS   string
		inputSpec sourcev1.GitRepositorySpec
		expected  *sourcev1.GitRepository
	}{
		{
			name:      "Valid input",
			inputName: "test-repo",
			inputNS:   "default",
			inputSpec: sourcev1.GitRepositorySpec{
				URL: "https://example.com/repo.git",
			},
			expected: &sourcev1.GitRepository{
				TypeMeta: metav1.TypeMeta{
					Kind:       "GitRepository",
					APIVersion: sourcev1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "default",
				},
				Spec: sourcev1.GitRepositorySpec{
					URL: "https://example.com/repo.git",
				},
			},
		},
		{
			name:      "Empty name",
			inputName: "",
			inputNS:   "default",
			inputSpec: sourcev1.GitRepositorySpec{
				URL: "https://example.com/repo.git",
			},
			expected: &sourcev1.GitRepository{
				TypeMeta: metav1.TypeMeta{
					Kind:       "GitRepository",
					APIVersion: sourcev1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "",
					Namespace: "default",
				},
				Spec: sourcev1.GitRepositorySpec{
					URL: "https://example.com/repo.git",
				},
			},
		},
		{
			name:      "Empty namespace",
			inputName: "test-repo",
			inputNS:   "",
			inputSpec: sourcev1.GitRepositorySpec{
				URL: "https://example.com/repo.git",
			},
			expected: &sourcev1.GitRepository{
				TypeMeta: metav1.TypeMeta{
					Kind:       "GitRepository",
					APIVersion: sourcev1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "",
				},
				Spec: sourcev1.GitRepositorySpec{
					URL: "https://example.com/repo.git",
				},
			},
		},
		{
			name:      "Empty spec",
			inputName: "test-repo",
			inputNS:   "default",
			inputSpec: sourcev1.GitRepositorySpec{},
			expected: &sourcev1.GitRepository{
				TypeMeta: metav1.TypeMeta{
					Kind:       "GitRepository",
					APIVersion: sourcev1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "default",
				},
				Spec: sourcev1.GitRepositorySpec{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CreateGitRepository(tt.inputName, tt.inputNS, tt.inputSpec)
			if result.TypeMeta != tt.expected.TypeMeta ||
				result.ObjectMeta.Name != tt.expected.ObjectMeta.Name ||
				result.ObjectMeta.Namespace != tt.expected.ObjectMeta.Namespace ||
				result.Spec.URL != tt.expected.Spec.URL {
				t.Errorf("CreateGitRepository(%v, %v, %v) = %v, want %v",
					tt.inputName, tt.inputNS, tt.inputSpec, result, tt.expected)
			}
		})
	}
}
func TestCreateHelmRepository(t *testing.T) {
	tests := []struct {
		name      string
		inputName string
		inputNS   string
		inputSpec sourcev1.HelmRepositorySpec
		expected  *sourcev1.HelmRepository
	}{
		{
			name:      "Valid input",
			inputName: "test-helm-repo",
			inputNS:   "default",
			inputSpec: sourcev1.HelmRepositorySpec{
				URL: "https://charts.example.com/repo",
			},
			expected: &sourcev1.HelmRepository{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HelmRepository",
					APIVersion: sourcev1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-helm-repo",
					Namespace: "default",
				},
				Spec: sourcev1.HelmRepositorySpec{
					URL: "https://charts.example.com/repo",
				},
			},
		},
		{
			name:      "Empty name",
			inputName: "",
			inputNS:   "default",
			inputSpec: sourcev1.HelmRepositorySpec{
				URL: "https://charts.example.com/repo",
			},
			expected: &sourcev1.HelmRepository{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HelmRepository",
					APIVersion: sourcev1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "",
					Namespace: "default",
				},
				Spec: sourcev1.HelmRepositorySpec{
					URL: "https://charts.example.com/repo",
				},
			},
		},
		{
			name:      "Empty namespace",
			inputName: "test-helm-repo",
			inputNS:   "",
			inputSpec: sourcev1.HelmRepositorySpec{
				URL: "https://charts.example.com/repo",
			},
			expected: &sourcev1.HelmRepository{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HelmRepository",
					APIVersion: sourcev1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-helm-repo",
					Namespace: "",
				},
				Spec: sourcev1.HelmRepositorySpec{
					URL: "https://charts.example.com/repo",
				},
			},
		},
		{
			name:      "Empty spec",
			inputName: "test-helm-repo",
			inputNS:   "default",
			inputSpec: sourcev1.HelmRepositorySpec{},
			expected: &sourcev1.HelmRepository{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HelmRepository",
					APIVersion: sourcev1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-helm-repo",
					Namespace: "default",
				},
				Spec: sourcev1.HelmRepositorySpec{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CreateHelmRepository(tt.inputName, tt.inputNS, tt.inputSpec)
			if result.TypeMeta != tt.expected.TypeMeta ||
				result.ObjectMeta.Name != tt.expected.ObjectMeta.Name ||
				result.ObjectMeta.Namespace != tt.expected.ObjectMeta.Namespace ||
				result.Spec.URL != tt.expected.Spec.URL {
				t.Errorf("CreateHelmRepository(%v, %v, %v) = %v, want %v",
					tt.inputName, tt.inputNS, tt.inputSpec, result, tt.expected)
			}
		})
	}
}
