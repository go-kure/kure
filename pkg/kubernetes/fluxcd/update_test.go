package fluxcd

import (
	"testing"
	"time"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	imagev1 "github.com/fluxcd/image-automation-controller/api/v1"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	notificationv1 "github.com/fluxcd/notification-controller/api/v1"
	notificationv1beta3 "github.com/fluxcd/notification-controller/api/v1beta3"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	sourceWatcherv1beta1 "github.com/fluxcd/source-watcher/api/v2/v1beta1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetGitRepositorySpec(t *testing.T) {
	repo := CreateGitRepository("test-repo", "flux-system")
	SetGitRepositoryURL(repo, "https://github.com/example/repo")

	newSpec := sourcev1.GitRepositorySpec{
		URL:       "https://github.com/example/new-repo",
		Interval:  metav1.Duration{Duration: 10 * time.Minute},
		Reference: &sourcev1.GitRepositoryRef{Branch: "develop"},
	}

	SetGitRepositorySpec(repo, newSpec)

	if repo.Spec.URL != "https://github.com/example/new-repo" {
		t.Errorf("expected URL 'https://github.com/example/new-repo', got %s", repo.Spec.URL)
	}

	if repo.Spec.Interval.Duration != 10*time.Minute {
		t.Errorf("expected interval 10m, got %v", repo.Spec.Interval.Duration)
	}

	if repo.Spec.Reference == nil || repo.Spec.Reference.Branch != "develop" {
		t.Error("expected Reference.Branch 'develop'")
	}
}

func TestSetHelmRepositorySpec(t *testing.T) {
	helmRepo := CreateHelmRepository("bitnami", "flux-system")
	SetHelmRepositoryURL(helmRepo, "https://charts.bitnami.com/bitnami")

	newSpec := sourcev1.HelmRepositorySpec{
		URL:      "https://charts.example.com/charts",
		Interval: metav1.Duration{Duration: 1 * time.Hour},
	}

	SetHelmRepositorySpec(helmRepo, newSpec)

	if helmRepo.Spec.URL != "https://charts.example.com/charts" {
		t.Errorf("expected URL 'https://charts.example.com/charts', got %s", helmRepo.Spec.URL)
	}

	if helmRepo.Spec.Interval.Duration != 1*time.Hour {
		t.Errorf("expected interval 1h, got %v", helmRepo.Spec.Interval.Duration)
	}
}

func TestSetBucketSpec(t *testing.T) {
	bucket := CreateBucket("s3-bucket", "flux-system")

	newSpec := sourcev1.BucketSpec{
		BucketName: "new-bucket",
		Endpoint:   "minio.example.com",
		Interval:   metav1.Duration{Duration: 30 * time.Minute},
		Provider:   "generic",
	}

	SetBucketSpec(bucket, newSpec)

	if bucket.Spec.BucketName != "new-bucket" {
		t.Errorf("expected BucketName 'new-bucket', got %s", bucket.Spec.BucketName)
	}

	if bucket.Spec.Provider != "generic" {
		t.Errorf("expected Provider 'generic', got %s", bucket.Spec.Provider)
	}
}

func TestSetOCIRepositorySpec(t *testing.T) {
	ociRepo := CreateOCIRepository("test-oci", "flux-system")
	SetOCIRepositoryURL(ociRepo, "oci://registry.example.com/repo")

	newSpec := sourcev1.OCIRepositorySpec{
		URL:       "oci://registry.example.com/new-repo",
		Reference: &sourcev1.OCIRepositoryRef{Tag: "v2.0.0"},
		Interval:  metav1.Duration{Duration: 15 * time.Minute},
	}

	SetOCIRepositorySpec(ociRepo, newSpec)

	if ociRepo.Spec.URL != "oci://registry.example.com/new-repo" {
		t.Errorf("expected URL 'oci://registry.example.com/new-repo', got %s", ociRepo.Spec.URL)
	}

	if ociRepo.Spec.Reference == nil || ociRepo.Spec.Reference.Tag != "v2.0.0" {
		t.Error("expected Reference.Tag 'v2.0.0'")
	}
}

func TestSetExternalArtifactSpec(t *testing.T) {
	ea := CreateExternalArtifact("ext", "flux-system")
	ref := &meta.NamespacedObjectKindReference{
		APIVersion: "source.toolkit.fluxcd.io/v1",
		Kind:       "OCIRepository",
		Name:       "my-oci",
	}
	newSpec := sourcev1.ExternalArtifactSpec{SourceRef: ref}
	SetExternalArtifactSpec(ea, newSpec)
	if ea.Spec.SourceRef != ref {
		t.Error("expected SourceRef to be set after SetExternalArtifactSpec")
	}
}

func TestSetArtifactGeneratorSpec(t *testing.T) {
	ag := CreateArtifactGenerator("ag", "flux-system")
	src := sourceWatcherv1beta1.SourceReference{Alias: "apps", Name: "my-repo", Kind: "GitRepository"}
	out := sourceWatcherv1beta1.OutputArtifact{
		Name: "combined",
		Copy: []sourceWatcherv1beta1.CopyOperation{{From: "@apps/deploy/", To: "@artifact/deploy/"}},
	}
	newSpec := sourceWatcherv1beta1.ArtifactGeneratorSpec{
		Sources:         []sourceWatcherv1beta1.SourceReference{src},
		OutputArtifacts: []sourceWatcherv1beta1.OutputArtifact{out},
	}
	SetArtifactGeneratorSpec(ag, newSpec)
	if len(ag.Spec.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(ag.Spec.Sources))
	}
	if ag.Spec.Sources[0].Alias != "apps" {
		t.Errorf("got Alias %q", ag.Spec.Sources[0].Alias)
	}
	if len(ag.Spec.OutputArtifacts) != 1 {
		t.Fatalf("expected 1 output, got %d", len(ag.Spec.OutputArtifacts))
	}
}

func TestSetKustomizationSpec(t *testing.T) {
	kustomization := CreateKustomization("app-kustomization", "default")
	SetKustomizationSourceRef(kustomization, kustv1.CrossNamespaceSourceReference{Kind: "GitRepository", Name: "app-repo"})

	newSourceRef := kustv1.CrossNamespaceSourceReference{
		Kind: "OCIRepository",
		Name: "oci-repo",
	}

	newSpec := kustv1.KustomizationSpec{
		Path:      "./manifests",
		Prune:     false,
		SourceRef: newSourceRef,
		Interval:  metav1.Duration{Duration: 5 * time.Minute},
	}

	SetKustomizationSpec(kustomization, newSpec)

	if kustomization.Spec.Path != "./manifests" {
		t.Errorf("expected Path './manifests', got %s", kustomization.Spec.Path)
	}

	if kustomization.Spec.Prune {
		t.Error("expected Prune to be false")
	}

	if kustomization.Spec.SourceRef.Kind != "OCIRepository" {
		t.Errorf("expected SourceRef.Kind 'OCIRepository', got %s", kustomization.Spec.SourceRef.Kind)
	}
}

func TestSetHelmReleaseSpec(t *testing.T) {
	helmRelease := CreateHelmRelease("my-nginx", "default")
	SetHelmReleaseChart(helmRelease, &helmv2.HelmChartTemplate{
		Spec: helmv2.HelmChartTemplateSpec{
			Chart:   "nginx",
			Version: "1.2.3",
			SourceRef: helmv2.CrossNamespaceObjectReference{
				Kind: "HelmRepository",
				Name: "bitnami",
			},
		},
	})

	newChart := helmv2.HelmChartTemplate{
		Spec: helmv2.HelmChartTemplateSpec{
			Chart:   "apache",
			Version: "2.0.0",
			SourceRef: helmv2.CrossNamespaceObjectReference{
				Kind: "HelmRepository",
				Name: "apache-charts",
			},
		},
	}

	newSpec := helmv2.HelmReleaseSpec{
		Chart:       &newChart,
		Interval:    metav1.Duration{Duration: 30 * time.Minute},
		ReleaseName: "apache-release",
	}

	SetHelmReleaseSpec(helmRelease, newSpec)

	if helmRelease.Spec.Chart.Spec.Chart != "apache" {
		t.Errorf("expected Chart 'apache', got %s", helmRelease.Spec.Chart.Spec.Chart)
	}

	if helmRelease.Spec.ReleaseName != "apache-release" {
		t.Errorf("expected ReleaseName 'apache-release', got %s", helmRelease.Spec.ReleaseName)
	}
}

func TestSetProviderSpec(t *testing.T) {
	provider := CreateProvider("slack-provider", "flux-system")
	SetProviderType(provider, "slack")

	newSpec := notificationv1beta3.ProviderSpec{
		Type:    "discord",
		Channel: "#notifications",
		Address: "https://discord.com/api/webhooks/...",
	}

	SetProviderSpec(provider, newSpec)

	if provider.Spec.Type != "discord" {
		t.Errorf("expected Type 'discord', got %s", provider.Spec.Type)
	}

	if provider.Spec.Channel != "#notifications" {
		t.Errorf("expected Channel '#notifications', got %s", provider.Spec.Channel)
	}
}

func TestSetAlertSpec(t *testing.T) {
	alert := CreateAlert("app-alert", "flux-system")
	SetAlertProviderRef(alert, meta.LocalObjectReference{Name: "slack-provider"})

	newSpec := notificationv1beta3.AlertSpec{
		ProviderRef: meta.LocalObjectReference{Name: "discord-provider"},
		Summary:     "warning",
	}

	SetAlertSpec(alert, newSpec)

	if alert.Spec.ProviderRef.Name != "discord-provider" {
		t.Errorf("expected ProviderRef.Name 'discord-provider', got %s", alert.Spec.ProviderRef.Name)
	}

	if alert.Spec.Summary != "warning" {
		t.Errorf("expected Summary 'warning', got %s", alert.Spec.Summary)
	}
}

func TestSetReceiverSpec(t *testing.T) {
	receiver := CreateReceiver("webhook-receiver", "flux-system")
	SetReceiverType(receiver, "github")

	newSpec := notificationv1.ReceiverSpec{
		Type:      "gitlab",
		SecretRef: meta.LocalObjectReference{Name: "gitlab-secret"},
		Events:    []string{"merge", "push"},
	}

	SetReceiverSpec(receiver, newSpec)

	if receiver.Spec.Type != "gitlab" {
		t.Errorf("expected Type 'gitlab', got %s", receiver.Spec.Type)
	}

	if receiver.Spec.SecretRef.Name != "gitlab-secret" {
		t.Errorf("expected SecretRef.Name 'gitlab-secret', got %s", receiver.Spec.SecretRef.Name)
	}
}

func TestSetImageUpdateAutomationSpec(t *testing.T) {
	imageUpdate := CreateImageUpdateAutomation("image-updater", "flux-system")
	SetImageUpdateAutomationSourceRef(imageUpdate, imagev1.CrossNamespaceSourceReference{
		Kind: "GitRepository",
		Name: "app-repo",
	})

	newSourceRef := imagev1.CrossNamespaceSourceReference{
		Kind: "GitRepository",
		Name: "new-repo",
	}

	newSpec := imagev1.ImageUpdateAutomationSpec{
		SourceRef: newSourceRef,
		Interval:  metav1.Duration{Duration: 1 * time.Hour},
	}

	SetImageUpdateAutomationSpec(imageUpdate, newSpec)

	if imageUpdate.Spec.SourceRef.Name != "new-repo" {
		t.Errorf("expected SourceRef.Name 'new-repo', got %s", imageUpdate.Spec.SourceRef.Name)
	}

	if imageUpdate.Spec.Interval.Duration != 1*time.Hour {
		t.Errorf("expected interval 1h, got %v", imageUpdate.Spec.Interval.Duration)
	}
}

func TestSetResourceSetSpec(t *testing.T) {
	resourceSet := CreateResourceSet("test-resourceset", "flux-system")

	newSpec := fluxv1.ResourceSetSpec{
		Wait: true,
	}

	SetResourceSetSpec(resourceSet, newSpec)

	if !resourceSet.Spec.Wait {
		t.Error("expected Wait to be true")
	}
}

func TestSetResourceSetInputProviderSpec(t *testing.T) {
	provider := CreateResourceSetInputProvider("input-provider", "flux-system")
	SetResourceSetInputProviderType(provider, "http")

	newSpec := fluxv1.ResourceSetInputProviderSpec{
		Type: "oci",
		URL:  "oci://registry.example.com/config",
	}

	SetResourceSetInputProviderSpec(provider, newSpec)

	if provider.Spec.Type != "oci" {
		t.Errorf("expected Type 'oci', got %s", provider.Spec.Type)
	}

	if provider.Spec.URL != "oci://registry.example.com/config" {
		t.Errorf("expected URL 'oci://registry.example.com/config', got %s", provider.Spec.URL)
	}
}

func TestSetFluxInstanceSpec(t *testing.T) {
	instance := CreateFluxInstance("flux-instance", "flux-system")
	SetFluxInstanceDistribution(instance, fluxv1.Distribution{
		Version:  "v2.1.0",
		Registry: "ghcr.io/fluxcd",
	})

	wait := true
	newSpec := fluxv1.FluxInstanceSpec{
		Distribution: fluxv1.Distribution{
			Version:  "v2.2.0",
			Registry: "quay.io/fluxcd",
		},
		Wait: &wait,
	}

	SetFluxInstanceSpec(instance, newSpec)

	if instance.Spec.Distribution.Version != "v2.2.0" {
		t.Errorf("expected Version 'v2.2.0', got %s", instance.Spec.Distribution.Version)
	}

	if instance.Spec.Distribution.Registry != "quay.io/fluxcd" {
		t.Errorf("expected Registry 'quay.io/fluxcd', got %s", instance.Spec.Distribution.Registry)
	}

	if instance.Spec.Wait == nil || !*instance.Spec.Wait {
		t.Error("expected Wait to be true")
	}
}

func TestSetFluxReportSpec(t *testing.T) {
	report := CreateFluxReport("flux-report", "flux-system")

	newSpec := fluxv1.FluxReportSpec{
		Distribution: fluxv1.FluxDistributionStatus{
			Entitlement: "community",
			Status:      "inactive",
		},
	}

	SetFluxReportSpec(report, newSpec)

	if report.Spec.Distribution.Entitlement != "community" {
		t.Errorf("expected Entitlement 'community', got %s", report.Spec.Distribution.Entitlement)
	}

	if report.Spec.Distribution.Status != "inactive" {
		t.Errorf("expected Status 'inactive', got %s", report.Spec.Distribution.Status)
	}
}

func TestFluxInstanceHelpers(t *testing.T) {
	instance := CreateFluxInstance("flux-instance", "flux-system")

	component := fluxv1.Component("source-controller")
	AddFluxInstanceComponent(instance, component)

	dist := fluxv1.Distribution{
		Version:  "v2.2.0",
		Registry: "quay.io/fluxcd",
	}
	SetFluxInstanceDistribution(instance, dist)

	SetFluxInstanceWait(instance, true)
}

func TestFluxReportHelpers(t *testing.T) {
	report := CreateFluxReport("flux-report", "flux-system")

	componentStatus := fluxv1.FluxComponentStatus{
		Name:   "kustomize-controller",
		Status: "running",
	}
	AddFluxReportComponentStatus(report, componentStatus)

	dist := fluxv1.FluxDistributionStatus{
		Entitlement: "community",
		Status:      "inactive",
	}
	SetFluxReportDistribution(report, dist)
}

func TestResourceSetHelpers(t *testing.T) {
	resourceSet := CreateResourceSet("test-resourceset", "flux-system")

	input := fluxv1.ResourceSetInput{
		"test-input": &apiextensionsv1.JSON{Raw: []byte(`"value"`)},
	}
	AddResourceSetInput(resourceSet, input)

	inputRef := fluxv1.InputProviderReference{
		Name: "input-provider",
	}
	AddResourceSetInputFrom(resourceSet, inputRef)

	SetResourceSetWait(resourceSet, true)

	SetResourceSetServiceAccountName(resourceSet, "flux")
}

func TestProviderHelpers(t *testing.T) {
	provider := CreateProvider("slack-provider", "flux-system")

	SetProviderType(provider, "discord")
	if provider.Spec.Type != "discord" {
		t.Errorf("expected Type 'discord', got %s", provider.Spec.Type)
	}

	interval := metav1.Duration{Duration: 10 * time.Minute}
	SetProviderInterval(provider, interval)
	if provider.Spec.Interval.Duration != 10*time.Minute {
		t.Errorf("expected interval 10m, got %v", provider.Spec.Interval.Duration)
	}

	SetProviderChannel(provider, "#notifications")
	if provider.Spec.Channel != "#notifications" {
		t.Errorf("expected Channel '#notifications', got %s", provider.Spec.Channel)
	}

	SetProviderUsername(provider, "fluxbot")
	if provider.Spec.Username != "fluxbot" {
		t.Errorf("expected Username 'fluxbot', got %s", provider.Spec.Username)
	}

	SetProviderAddress(provider, "https://discord.com/api/webhooks/...")
	if provider.Spec.Address != "https://discord.com/api/webhooks/..." {
		t.Errorf("expected specific Address, got %s", provider.Spec.Address)
	}

	timeout := metav1.Duration{Duration: 30 * time.Second}
	SetProviderTimeout(provider, timeout)
	if provider.Spec.Timeout.Duration != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", provider.Spec.Timeout.Duration)
	}

	SetProviderProxy(provider, "http://proxy.example.com:8080")
	if provider.Spec.Proxy != "http://proxy.example.com:8080" {
		t.Errorf("expected proxy 'http://proxy.example.com:8080', got %s", provider.Spec.Proxy)
	}

	secretRef := &meta.LocalObjectReference{Name: "discord-secret"}
	SetProviderSecretRef(provider, secretRef)
	if provider.Spec.SecretRef == nil || provider.Spec.SecretRef.Name != "discord-secret" {
		t.Error("expected SecretRef.Name 'discord-secret'")
	}

	certSecretRef := &meta.LocalObjectReference{Name: "cert-secret"}
	SetProviderCertSecretRef(provider, certSecretRef)
	if provider.Spec.CertSecretRef == nil || provider.Spec.CertSecretRef.Name != "cert-secret" {
		t.Error("expected CertSecretRef.Name 'cert-secret'")
	}

	SetProviderSuspend(provider, true)
	if !provider.Spec.Suspend {
		t.Error("expected Suspend to be true")
	}
}

func TestResourceSetInputProviderHelpers(t *testing.T) {
	provider := CreateResourceSetInputProvider("input-provider", "flux-system")

	SetResourceSetInputProviderType(provider, "oci")

	SetResourceSetInputProviderURL(provider, "oci://registry.example.com/config")

	SetResourceSetInputProviderServiceAccountName(provider, "flux")

	secretRef := &meta.LocalObjectReference{Name: "registry-secret"}
	SetResourceSetInputProviderSecretRef(provider, secretRef)

	certSecretRef := &meta.LocalObjectReference{Name: "cert-secret"}
	SetResourceSetInputProviderCertSecretRef(provider, certSecretRef)

	schedule := fluxv1.Schedule{
		Cron: "0 */6 * * *",
	}
	AddResourceSetInputProviderSchedule(provider, schedule)
}

func TestResourceSetAdvancedHelpers(t *testing.T) {
	resourceSet := CreateResourceSet("test-resourceset", "flux-system")

	resource := &apiextensionsv1.JSON{Raw: []byte(`{"apiVersion": "v1", "kind": "ConfigMap"}`)}
	AddResourceSetResource(resourceSet, resource)

	template := "{{ .Values.configMap }}"
	SetResourceSetResourcesTemplate(resourceSet, template)

	dependency := fluxv1.Dependency{
		Name: "prerequisite-resource",
	}
	AddResourceSetDependency(resourceSet, dependency)

	commonMetadata := &fluxv1.CommonMetadata{
		Labels: map[string]string{
			"app": "test",
		},
	}
	SetResourceSetCommonMetadata(resourceSet, commonMetadata)
}
