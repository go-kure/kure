package fluxcd

import (
	"testing"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
)

// Tests for setters to verify the Style A pattern works end-to-end.

func TestGitRepositorySetters(t *testing.T) {
	obj := CreateGitRepository("test-repo", "flux-system")
	SetGitRepositoryReference(obj, &sourcev1.GitRepositoryRef{Branch: "main"})

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
	if obj.Spec.Chart == nil {
		t.Fatal("expected Chart to be set")
	}
	if obj.Spec.Chart.Spec.Chart != "nginx" {
		t.Errorf("expected Chart 'nginx', got %s", obj.Spec.Chart.Spec.Chart)
	}
}
