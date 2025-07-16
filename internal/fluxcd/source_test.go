package fluxcd

import (
	"testing"
	"time"

	meta "github.com/fluxcd/pkg/apis/meta"
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
					Kind:       "OCIRepository",
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
					Kind:       "OCIRepository",
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
					Kind:       "OCIRepository",
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
					Kind:       "OCIRepository",
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

func TestGitRepositoryHelpers(t *testing.T) {
	repo := CreateGitRepository("git", "ns", sourcev1.GitRepositorySpec{})
	SetGitRepositoryURL(repo, "https://example.com/repo.git")
	sr := meta.LocalObjectReference{Name: "secret"}
	SetGitRepositorySecretRef(repo, &sr)
	interval := metav1.Duration{Duration: time.Minute}
	SetGitRepositoryInterval(repo, interval)
	include := sourcev1.GitRepositoryInclude{GitRepositoryRef: meta.LocalObjectReference{Name: "other"}}
	AddGitRepositoryInclude(repo, include)

	if repo.Spec.URL != "https://example.com/repo.git" {
		t.Errorf("unexpected url %s", repo.Spec.URL)
	}
	if repo.Spec.SecretRef == nil || repo.Spec.SecretRef.Name != "secret" {
		t.Errorf("secret ref not set")
	}
	if repo.Spec.Interval != interval {
		t.Errorf("interval not set")
	}
	if len(repo.Spec.Include) != 1 || repo.Spec.Include[0].GitRepositoryRef.Name != "other" {
		t.Errorf("include not added")
	}
}

func TestHelmRepositoryHelpers(t *testing.T) {
	repo := CreateHelmRepository("hr", "ns", sourcev1.HelmRepositorySpec{})
	SetHelmRepositoryURL(repo, "https://charts.example.com")
	sr := meta.LocalObjectReference{Name: "cred"}
	SetHelmRepositorySecretRef(repo, &sr)
	SetHelmRepositoryInterval(repo, metav1.Duration{Duration: time.Hour})
	SetHelmRepositoryProvider(repo, "aws")
	if repo.Spec.URL != "https://charts.example.com" {
		t.Errorf("url not set")
	}
	if repo.Spec.SecretRef == nil || repo.Spec.SecretRef.Name != "cred" {
		t.Errorf("secret not set")
	}
	if repo.Spec.Interval.Duration != time.Hour {
		t.Errorf("interval not set")
	}
	if repo.Spec.Provider != "aws" {
		t.Errorf("provider not set")
	}
}

func TestBucketHelpers(t *testing.T) {
	b := CreateBucket("b", "ns", sourcev1.BucketSpec{})
	SetBucketName(b, "bucket")
	SetBucketEndpoint(b, "https://s3.example.com")
	SetBucketInterval(b, metav1.Duration{Duration: time.Hour})
	ref := meta.LocalObjectReference{Name: "creds"}
	SetBucketSecretRef(b, &ref)
	if b.Spec.BucketName != "bucket" {
		t.Errorf("bucket name not set")
	}
	if b.Spec.Endpoint != "https://s3.example.com" {
		t.Errorf("endpoint not set")
	}
	if b.Spec.Interval.Duration != time.Hour {
		t.Errorf("interval not set")
	}
	if b.Spec.SecretRef == nil || b.Spec.SecretRef.Name != "creds" {
		t.Errorf("secret ref not set")
	}
}

func TestHelmChartHelpers(t *testing.T) {
	hc := CreateHelmChart("chart", "ns", sourcev1.HelmChartSpec{})
	SetHelmChartChart(hc, "app")
	SetHelmChartVersion(hc, "1.0.0")
	SetHelmChartInterval(hc, metav1.Duration{Duration: time.Minute})
	AddHelmChartValuesFile(hc, "values.yaml")
	if hc.Spec.Chart != "app" {
		t.Errorf("chart not set")
	}
	if hc.Spec.Version != "1.0.0" {
		t.Errorf("version not set")
	}
	if hc.Spec.Interval.Duration != time.Minute {
		t.Errorf("interval not set")
	}
	if len(hc.Spec.ValuesFiles) != 1 || hc.Spec.ValuesFiles[0] != "values.yaml" {
		t.Errorf("values file not added")
	}
}

func TestOCIRepositoryHelpers(t *testing.T) {
	or := CreateOCIRepository("oci", "ns", sourcev1beta2.OCIRepositorySpec{})
	SetOCIRepositoryURL(or, "oci://repo")
	SetOCIRepositoryInterval(or, metav1.Duration{Duration: time.Minute})
	SetOCIRepositoryProvider(or, "aws")
	ref := meta.LocalObjectReference{Name: "creds"}
	SetOCIRepositorySecretRef(or, &ref)
	if or.Spec.URL != "oci://repo" {
		t.Errorf("url not set")
	}
	if or.Spec.Interval.Duration != time.Minute {
		t.Errorf("interval not set")
	}
	if or.Spec.Provider != "aws" {
		t.Errorf("provider not set")
	}
	if or.Spec.SecretRef == nil || or.Spec.SecretRef.Name != "creds" {
		t.Errorf("secret not set")
	}
}
