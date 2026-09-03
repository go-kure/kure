package fluxcd

import (
	"testing"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Tests for setters to verify the Style A pattern works end-to-end.

func TestGitRepositorySetters(t *testing.T) {
	obj := CreateGitRepository("test-repo", "flux-system")
	SetGitRepositoryURL(obj, "https://github.com/example/repo")
	SetGitRepositoryInterval(obj, metav1.Duration{Duration: 5 * 60 * 1000000000}) // 5m
	SetGitRepositoryReference(obj, &sourcev1.GitRepositoryRef{Branch: "main"})

	if obj.Spec.URL != "https://github.com/example/repo" {
		t.Errorf("expected URL 'https://github.com/example/repo', got %s", obj.Spec.URL)
	}
	if obj.Spec.Reference == nil {
		t.Fatal("expected non-nil Reference")
	}
	if obj.Spec.Reference.Branch != "main" {
		t.Errorf("expected branch 'main', got %s", obj.Spec.Reference.Branch)
	}
}

func TestHelmReleaseSetters(t *testing.T) {
	obj := CreateHelmRelease("my-nginx", "default")
	SetHelmReleaseChart(obj, &helmv2.HelmChartTemplate{
		Spec: helmv2.HelmChartTemplateSpec{
			Chart:   "nginx",
			Version: "1.2.3",
			SourceRef: helmv2.CrossNamespaceObjectReference{
				Kind: "HelmRepository",
				Name: "bitnami",
			},
		},
	})
	SetHelmReleaseReleaseName(obj, "nginx-release")

	if obj.Spec.Chart == nil {
		t.Fatal("expected non-nil Chart")
	}
	if obj.Spec.Chart.Spec.Chart != "nginx" {
		t.Errorf("expected Chart 'nginx', got %s", obj.Spec.Chart.Spec.Chart)
	}
	if obj.Spec.ReleaseName != "nginx-release" {
		t.Errorf("expected ReleaseName 'nginx-release', got %s", obj.Spec.ReleaseName)
	}
}

func TestKustomizationSetters(t *testing.T) {
	obj := CreateKustomization("app-kustomization", "default")
	SetKustomizationPath(obj, "./deploy")
	SetKustomizationPrune(obj, true)
	SetKustomizationSourceRef(obj, kustv1.CrossNamespaceSourceReference{
		Kind: "GitRepository",
		Name: "app-repo",
	})

	if obj.Spec.Path != "./deploy" {
		t.Errorf("expected Path './deploy', got %s", obj.Spec.Path)
	}
	if !obj.Spec.Prune {
		t.Error("expected Prune to be true")
	}
	if obj.Spec.SourceRef.Kind != "GitRepository" {
		t.Errorf("expected SourceRef.Kind 'GitRepository', got %s", obj.Spec.SourceRef.Kind)
	}
}
