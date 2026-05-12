package fluxcd

import (
	"testing"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	imagev1 "github.com/fluxcd/image-automation-controller/api/v1"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	notificationv1 "github.com/fluxcd/notification-controller/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	sourceWatcherv1beta1 "github.com/fluxcd/source-watcher/api/v2/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateGitRepository(t *testing.T) {
	obj := CreateGitRepository("test-repo", "flux-system")
	if obj == nil {
		t.Fatal("expected non-nil GitRepository")
	}
	if obj.Name != "test-repo" {
		t.Errorf("expected Name 'test-repo', got %s", obj.Name)
	}
	if obj.Namespace != "flux-system" {
		t.Errorf("expected Namespace 'flux-system', got %s", obj.Namespace)
	}
	if obj.Kind != "GitRepository" {
		t.Errorf("expected Kind 'GitRepository', got %s", obj.Kind)
	}
	if obj.APIVersion != sourcev1.GroupVersion.String() {
		t.Errorf("expected APIVersion %s, got %s", sourcev1.GroupVersion.String(), obj.APIVersion)
	}
}

func TestCreateHelmRepository(t *testing.T) {
	obj := CreateHelmRepository("bitnami", "flux-system")
	if obj == nil {
		t.Fatal("expected non-nil HelmRepository")
	}
	if obj.Name != "bitnami" {
		t.Errorf("expected Name 'bitnami', got %s", obj.Name)
	}
	if obj.Namespace != "flux-system" {
		t.Errorf("expected Namespace 'flux-system', got %s", obj.Namespace)
	}
}

func TestCreateOCIRepository(t *testing.T) {
	obj := CreateOCIRepository("test-oci", "flux-system")
	if obj == nil {
		t.Fatal("expected non-nil OCIRepository")
	}
	if obj.Name != "test-oci" {
		t.Errorf("expected Name 'test-oci', got %s", obj.Name)
	}
}

func TestCreateBucket(t *testing.T) {
	obj := CreateBucket("s3-bucket", "flux-system")
	if obj == nil {
		t.Fatal("expected non-nil Bucket")
	}
	if obj.Name != "s3-bucket" {
		t.Errorf("expected Name 's3-bucket', got %s", obj.Name)
	}
}

func TestCreateHelmChart(t *testing.T) {
	obj := CreateHelmChart("nginx-chart", "default")
	if obj == nil {
		t.Fatal("expected non-nil HelmChart")
	}
	if obj.Name != "nginx-chart" {
		t.Errorf("expected Name 'nginx-chart', got %s", obj.Name)
	}
}

func TestCreateKustomization(t *testing.T) {
	obj := CreateKustomization("app-kustomization", "default")
	if obj == nil {
		t.Fatal("expected non-nil Kustomization")
	}
	if obj.Name != "app-kustomization" {
		t.Errorf("expected Name 'app-kustomization', got %s", obj.Name)
	}
	if obj.Kind != kustv1.KustomizationKind {
		t.Errorf("expected Kind %s, got %s", kustv1.KustomizationKind, obj.Kind)
	}
}

func TestCreateHelmRelease(t *testing.T) {
	obj := CreateHelmRelease("my-nginx", "default")
	if obj == nil {
		t.Fatal("expected non-nil HelmRelease")
	}
	if obj.Name != "my-nginx" {
		t.Errorf("expected Name 'my-nginx', got %s", obj.Name)
	}
	if obj.Kind != helmv2.HelmReleaseKind {
		t.Errorf("expected Kind %s, got %s", helmv2.HelmReleaseKind, obj.Kind)
	}
}

func TestCreateProvider(t *testing.T) {
	obj := CreateProvider("slack-provider", "flux-system")
	if obj == nil {
		t.Fatal("expected non-nil Provider")
	}
	if obj.Name != "slack-provider" {
		t.Errorf("expected Name 'slack-provider', got %s", obj.Name)
	}
}

func TestCreateAlert(t *testing.T) {
	obj := CreateAlert("app-alert", "flux-system")
	if obj == nil {
		t.Fatal("expected non-nil Alert")
	}
	if obj.Name != "app-alert" {
		t.Errorf("expected Name 'app-alert', got %s", obj.Name)
	}
}

func TestCreateReceiver(t *testing.T) {
	obj := CreateReceiver("webhook-receiver", "flux-system")
	if obj == nil {
		t.Fatal("expected non-nil Receiver")
	}
	if obj.Name != "webhook-receiver" {
		t.Errorf("expected Name 'webhook-receiver', got %s", obj.Name)
	}
	if obj.Kind != notificationv1.ReceiverKind {
		t.Errorf("expected Kind %s, got %s", notificationv1.ReceiverKind, obj.Kind)
	}
}

func TestCreateImageUpdateAutomation(t *testing.T) {
	obj := CreateImageUpdateAutomation("image-updater", "flux-system")
	if obj == nil {
		t.Fatal("expected non-nil ImageUpdateAutomation")
	}
	if obj.Name != "image-updater" {
		t.Errorf("expected Name 'image-updater', got %s", obj.Name)
	}
	if obj.Kind != imagev1.ImageUpdateAutomationKind {
		t.Errorf("expected Kind %s, got %s", imagev1.ImageUpdateAutomationKind, obj.Kind)
	}
}

func TestCreateResourceSet(t *testing.T) {
	obj := CreateResourceSet("test-resourceset", "flux-system")
	if obj == nil {
		t.Fatal("expected non-nil ResourceSet")
	}
	if obj.Name != "test-resourceset" {
		t.Errorf("expected Name 'test-resourceset', got %s", obj.Name)
	}
	if obj.Namespace != "flux-system" {
		t.Errorf("expected Namespace 'flux-system', got %s", obj.Namespace)
	}
}

func TestCreateResourceSetInputProvider(t *testing.T) {
	obj := CreateResourceSetInputProvider("input-provider", "flux-system")
	if obj == nil {
		t.Fatal("expected non-nil ResourceSetInputProvider")
	}
	if obj.Name != "input-provider" {
		t.Errorf("expected Name 'input-provider', got %s", obj.Name)
	}
}

func TestCreateFluxInstance(t *testing.T) {
	obj := CreateFluxInstance("flux-instance", "flux-system")
	if obj == nil {
		t.Fatal("expected non-nil FluxInstance")
	}
	if obj.Name != "flux-instance" {
		t.Errorf("expected Name 'flux-instance', got %s", obj.Name)
	}
}

func TestCreateFluxReport(t *testing.T) {
	obj := CreateFluxReport("flux-report", "flux-system")
	if obj == nil {
		t.Fatal("expected non-nil FluxReport")
	}
	if obj.Name != "flux-report" {
		t.Errorf("expected Name 'flux-report', got %s", obj.Name)
	}
}

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

func TestCreateExternalArtifact(t *testing.T) {
	obj := CreateExternalArtifact("ext-artifact", "flux-system")
	if obj == nil {
		t.Fatal("expected non-nil ExternalArtifact")
	}
	if obj.Name != "ext-artifact" {
		t.Errorf("expected Name 'ext-artifact', got %s", obj.Name)
	}
	if obj.Namespace != "flux-system" {
		t.Errorf("expected Namespace 'flux-system', got %s", obj.Namespace)
	}
	if obj.Kind != sourcev1.ExternalArtifactKind {
		t.Errorf("expected Kind %q, got %q", sourcev1.ExternalArtifactKind, obj.Kind)
	}
	if obj.APIVersion != sourcev1.GroupVersion.String() {
		t.Errorf("expected APIVersion %q, got %q", sourcev1.GroupVersion.String(), obj.APIVersion)
	}
}

func TestCreateArtifactGenerator(t *testing.T) {
	obj := CreateArtifactGenerator("ag", "flux-system")
	if obj == nil {
		t.Fatal("expected non-nil ArtifactGenerator")
	}
	if obj.Name != "ag" {
		t.Errorf("expected Name 'ag', got %s", obj.Name)
	}
	if obj.Namespace != "flux-system" {
		t.Errorf("expected Namespace 'flux-system', got %s", obj.Namespace)
	}
	if obj.Kind != sourceWatcherv1beta1.ArtifactGeneratorKind {
		t.Errorf("expected Kind %q, got %q", sourceWatcherv1beta1.ArtifactGeneratorKind, obj.Kind)
	}
	if obj.APIVersion != sourceWatcherv1beta1.GroupVersion.String() {
		t.Errorf("expected APIVersion %q, got %q", sourceWatcherv1beta1.GroupVersion.String(), obj.APIVersion)
	}
}
