package fluxcd

import (
	"testing"
	"time"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	imagev1 "github.com/fluxcd/image-automation-controller/api/v1beta2"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	notificationv1beta2 "github.com/fluxcd/notification-controller/api/v1beta2"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Helper function to create a pointer to a boolean
func boolPtr(b bool) *bool {
	return &b
}

func TestSetGitRepositorySpec(t *testing.T) {
	// Create a GitRepository using the constructor first
	cfg := &GitRepositoryConfig{
		Name:      "test-repo",
		Namespace: "flux-system",
		URL:       "https://github.com/example/repo",
		Interval:  "5m",
	}

	repo := GitRepository(cfg)
	if repo == nil {
		t.Fatal("failed to create GitRepository")
	}

	// Create a new spec to set
	newSpec := sourcev1.GitRepositorySpec{
		URL:       "https://github.com/example/new-repo",
		Interval:  metav1.Duration{Duration: 10 * time.Minute},
		Reference: &sourcev1.GitRepositoryRef{Branch: "develop"},
	}

	// Set the new spec
	SetGitRepositorySpec(repo, newSpec)

	// Verify the spec was replaced
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
	cfg := &HelmRepositoryConfig{
		Name:      "bitnami",
		Namespace: "flux-system",
		URL:       "https://charts.bitnami.com/bitnami",
	}

	helmRepo := HelmRepository(cfg)
	if helmRepo == nil {
		t.Fatal("failed to create HelmRepository")
	}

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
	cfg := &BucketConfig{
		Name:       "s3-bucket",
		Namespace:  "flux-system",
		BucketName: "my-flux-bucket",
		Endpoint:   "s3.amazonaws.com",
		Interval:   "10m",
	}

	bucket := Bucket(cfg)
	if bucket == nil {
		t.Fatal("failed to create Bucket")
	}

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
	cfg := &OCIRepositoryConfig{
		Name:      "test-oci",
		Namespace: "flux-system",
		URL:       "oci://registry.example.com/repo",
		Ref:       "v1.0.0",
		Interval:  "1m",
	}

	ociRepo := OCIRepository(cfg)
	if ociRepo == nil {
		t.Fatal("failed to create OCIRepository")
	}

	newSpec := sourcev1beta2.OCIRepositorySpec{
		URL:       "oci://registry.example.com/new-repo",
		Reference: &sourcev1beta2.OCIRepositoryRef{Tag: "v2.0.0"},
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

func TestSetKustomizationSpec(t *testing.T) {
	sourceRef := kustv1.CrossNamespaceSourceReference{
		Kind: "GitRepository",
		Name: "app-repo",
	}

	cfg := &KustomizationConfig{
		Name:      "app-kustomization",
		Namespace: "default",
		Interval:  "2m",
		Prune:     true,
		SourceRef: sourceRef,
	}

	kustomization := Kustomization(cfg)
	if kustomization == nil {
		t.Fatal("failed to create Kustomization")
	}

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
	sourceRef := helmv2.CrossNamespaceObjectReference{
		Kind: "HelmRepository",
		Name: "bitnami",
	}

	cfg := &HelmReleaseConfig{
		Name:      "my-nginx",
		Namespace: "default",
		Chart:     "nginx",
		Version:   "1.2.3",
		SourceRef: sourceRef,
		Interval:  "1h",
	}

	helmRelease := HelmRelease(cfg)
	if helmRelease == nil {
		t.Fatal("failed to create HelmRelease")
	}

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
	cfg := &ProviderConfig{
		Name:      "slack-provider",
		Namespace: "flux-system",
		Type:      "slack",
		Channel:   "#alerts",
	}

	provider := Provider(cfg)
	if provider == nil {
		t.Fatal("failed to create Provider")
	}

	newSpec := notificationv1beta2.ProviderSpec{
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
	cfg := &AlertConfig{
		Name:        "app-alert",
		Namespace:   "flux-system",
		ProviderRef: "slack-provider",
	}

	alert := Alert(cfg)
	if alert == nil {
		t.Fatal("failed to create Alert")
	}

	newSpec := notificationv1beta2.AlertSpec{
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
	cfg := &ReceiverConfig{
		Name:       "webhook-receiver",
		Namespace:  "flux-system",
		Type:       "github",
		SecretName: "webhook-secret",
	}

	receiver := Receiver(cfg)
	if receiver == nil {
		t.Fatal("failed to create Receiver")
	}

	newSpec := notificationv1beta2.ReceiverSpec{
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
	sourceRef := imagev1.CrossNamespaceSourceReference{
		Kind: "GitRepository",
		Name: "app-repo",
	}

	cfg := &ImageUpdateAutomationConfig{
		Name:      "image-updater",
		Namespace: "flux-system",
		Interval:  "30m",
		SourceRef: sourceRef,
	}

	imageUpdate := ImageUpdateAutomation(cfg)
	if imageUpdate == nil {
		t.Fatal("failed to create ImageUpdateAutomation")
	}

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
	cfg := &ResourceSetConfig{
		Name:      "test-resourceset",
		Namespace: "flux-system",
	}

	resourceSet := ResourceSet(cfg)
	if resourceSet == nil {
		t.Fatal("failed to create ResourceSet")
	}

	newSpec := fluxv1.ResourceSetSpec{
		Wait: true,
	}

	SetResourceSetSpec(resourceSet, newSpec)

	if !resourceSet.Spec.Wait {
		t.Error("expected Wait to be true")
	}
}

func TestSetResourceSetInputProviderSpec(t *testing.T) {
	cfg := &ResourceSetInputProviderConfig{
		Name:      "input-provider",
		Namespace: "flux-system",
		Type:      "http",
	}

	provider, err := ResourceSetInputProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("failed to create ResourceSetInputProvider")
	}

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
	cfg := &FluxInstanceConfig{
		Name:      "flux-instance",
		Namespace: "flux-system",
		Version:   "v2.1.0",
		Registry:  "ghcr.io/fluxcd",
	}

	instance := FluxInstance(cfg)
	if instance == nil {
		t.Fatal("failed to create FluxInstance")
	}

	newSpec := fluxv1.FluxInstanceSpec{
		Distribution: fluxv1.Distribution{
			Version:  "v2.2.0",
			Registry: "quay.io/fluxcd",
		},
		Wait: boolPtr(true),
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
	cfg := &FluxReportConfig{
		Name:        "flux-report",
		Namespace:   "flux-system",
		Entitlement: "enterprise",
		Status:      "active",
	}

	report := FluxReport(cfg)
	if report == nil {
		t.Fatal("failed to create FluxReport")
	}

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
	cfg := &FluxInstanceConfig{
		Name:      "flux-instance",
		Namespace: "flux-system",
		Version:   "v2.1.0",
		Registry:  "ghcr.io/fluxcd",
	}

	instance := FluxInstance(cfg)
	if instance == nil {
		t.Fatal("failed to create FluxInstance")
	}

	// Test AddFluxInstanceComponent
	component := fluxv1.Component("source-controller")
	err := AddFluxInstanceComponent(instance, component)
	if err != nil {
		t.Errorf("AddFluxInstanceComponent failed: %v", err)
	}

	// Test SetFluxInstanceDistribution
	dist := fluxv1.Distribution{
		Version:  "v2.2.0",
		Registry: "quay.io/fluxcd",
	}
	err = SetFluxInstanceDistribution(instance, dist)
	if err != nil {
		t.Errorf("SetFluxInstanceDistribution failed: %v", err)
	}

	// Test SetFluxInstanceWait
	err = SetFluxInstanceWait(instance, true)
	if err != nil {
		t.Errorf("SetFluxInstanceWait failed: %v", err)
	}
}

func TestFluxReportHelpers(t *testing.T) {
	cfg := &FluxReportConfig{
		Name:        "flux-report",
		Namespace:   "flux-system",
		Entitlement: "enterprise",
		Status:      "active",
	}

	report := FluxReport(cfg)
	if report == nil {
		t.Fatal("failed to create FluxReport")
	}

	// Test AddFluxReportComponentStatus
	componentStatus := fluxv1.FluxComponentStatus{
		Name:   "kustomize-controller",
		Status: "running",
	}
	err := AddFluxReportComponentStatus(report, componentStatus)
	if err != nil {
		t.Errorf("AddFluxReportComponentStatus failed: %v", err)
	}

	// Test SetFluxReportDistribution
	dist := fluxv1.FluxDistributionStatus{
		Entitlement: "community",
		Status:      "inactive",
	}
	err = SetFluxReportDistribution(report, dist)
	if err != nil {
		t.Errorf("SetFluxReportDistribution failed: %v", err)
	}
}

func TestResourceSetHelpers(t *testing.T) {
	cfg := &ResourceSetConfig{
		Name:      "test-resourceset",
		Namespace: "flux-system",
	}

	resourceSet := ResourceSet(cfg)
	if resourceSet == nil {
		t.Fatal("failed to create ResourceSet")
	}

	// Test AddResourceSetInput
	input := fluxv1.ResourceSetInput{
		"test-input": &apiextensionsv1.JSON{Raw: []byte(`"value"`)},
	}
	err := AddResourceSetInput(resourceSet, input)
	if err != nil {
		t.Errorf("AddResourceSetInput failed: %v", err)
	}

	// Test AddResourceSetInputFrom
	inputRef := fluxv1.InputProviderReference{
		Name: "input-provider",
	}
	err = AddResourceSetInputFrom(resourceSet, inputRef)
	if err != nil {
		t.Errorf("AddResourceSetInputFrom failed: %v", err)
	}

	// Test SetResourceSetWait
	err = SetResourceSetWait(resourceSet, true)
	if err != nil {
		t.Errorf("SetResourceSetWait failed: %v", err)
	}

	// Test SetResourceSetServiceAccountName
	err = SetResourceSetServiceAccountName(resourceSet, "flux")
	if err != nil {
		t.Errorf("SetResourceSetServiceAccountName failed: %v", err)
	}
}

func TestProviderHelpers(t *testing.T) {
	cfg := &ProviderConfig{
		Name:      "slack-provider",
		Namespace: "flux-system",
		Type:      "slack",
	}

	provider := Provider(cfg)
	if provider == nil {
		t.Fatal("failed to create Provider")
	}

	// Test SetProviderType
	SetProviderType(provider, "discord")
	if provider.Spec.Type != "discord" {
		t.Errorf("expected Type 'discord', got %s", provider.Spec.Type)
	}

	// Test SetProviderInterval
	interval := metav1.Duration{Duration: 10 * time.Minute}
	SetProviderInterval(provider, interval)
	if provider.Spec.Interval.Duration != 10*time.Minute {
		t.Errorf("expected interval 10m, got %v", provider.Spec.Interval.Duration)
	}

	// Test SetProviderChannel
	SetProviderChannel(provider, "#notifications")
	if provider.Spec.Channel != "#notifications" {
		t.Errorf("expected Channel '#notifications', got %s", provider.Spec.Channel)
	}

	// Test SetProviderUsername
	SetProviderUsername(provider, "fluxbot")
	if provider.Spec.Username != "fluxbot" {
		t.Errorf("expected Username 'fluxbot', got %s", provider.Spec.Username)
	}

	// Test SetProviderAddress
	SetProviderAddress(provider, "https://discord.com/api/webhooks/...")
	if provider.Spec.Address != "https://discord.com/api/webhooks/..." {
		t.Errorf("expected specific Address, got %s", provider.Spec.Address)
	}

	// Test SetProviderTimeout
	timeout := metav1.Duration{Duration: 30 * time.Second}
	SetProviderTimeout(provider, timeout)
	if provider.Spec.Timeout.Duration != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", provider.Spec.Timeout.Duration)
	}

	// Test SetProviderProxy
	SetProviderProxy(provider, "http://proxy.example.com:8080")
	if provider.Spec.Proxy != "http://proxy.example.com:8080" {
		t.Errorf("expected proxy 'http://proxy.example.com:8080', got %s", provider.Spec.Proxy)
	}

	// Test SetProviderSecretRef
	secretRef := &meta.LocalObjectReference{Name: "discord-secret"}
	SetProviderSecretRef(provider, secretRef)
	if provider.Spec.SecretRef == nil || provider.Spec.SecretRef.Name != "discord-secret" {
		t.Error("expected SecretRef.Name 'discord-secret'")
	}

	// Test SetProviderCertSecretRef
	certSecretRef := &meta.LocalObjectReference{Name: "cert-secret"}
	SetProviderCertSecretRef(provider, certSecretRef)
	if provider.Spec.CertSecretRef == nil || provider.Spec.CertSecretRef.Name != "cert-secret" {
		t.Error("expected CertSecretRef.Name 'cert-secret'")
	}

	// Test SetProviderSuspend
	SetProviderSuspend(provider, true)
	if !provider.Spec.Suspend {
		t.Error("expected Suspend to be true")
	}
}

func TestResourceSetInputProviderHelpers(t *testing.T) {
	cfg := &ResourceSetInputProviderConfig{
		Name:      "input-provider",
		Namespace: "flux-system",
		Type:      "http",
	}

	provider, err := ResourceSetInputProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("failed to create ResourceSetInputProvider")
	}

	// Test SetResourceSetInputProviderType (via the function that delegates)
	err = SetResourceSetInputProviderType(provider, "oci")
	if err != nil {
		t.Errorf("SetResourceSetInputProviderType failed: %v", err)
	}

	// Test SetResourceSetInputProviderURL
	err = SetResourceSetInputProviderURL(provider, "oci://registry.example.com/config")
	if err != nil {
		t.Errorf("SetResourceSetInputProviderURL failed: %v", err)
	}

	// Test SetResourceSetInputProviderServiceAccountName
	err = SetResourceSetInputProviderServiceAccountName(provider, "flux")
	if err != nil {
		t.Errorf("SetResourceSetInputProviderServiceAccountName failed: %v", err)
	}

	// Test SetResourceSetInputProviderSecretRef
	secretRef := &meta.LocalObjectReference{Name: "registry-secret"}
	err = SetResourceSetInputProviderSecretRef(provider, secretRef)
	if err != nil {
		t.Errorf("SetResourceSetInputProviderSecretRef failed: %v", err)
	}

	// Test SetResourceSetInputProviderCertSecretRef
	certSecretRef := &meta.LocalObjectReference{Name: "cert-secret"}
	err = SetResourceSetInputProviderCertSecretRef(provider, certSecretRef)
	if err != nil {
		t.Errorf("SetResourceSetInputProviderCertSecretRef failed: %v", err)
	}

	// Test AddResourceSetInputProviderSchedule
	schedule := fluxv1.Schedule{
		Cron: "0 */6 * * *", // Every 6 hours
	}
	err = AddResourceSetInputProviderSchedule(provider, schedule)
	if err != nil {
		t.Errorf("AddResourceSetInputProviderSchedule failed: %v", err)
	}
}

func TestResourceSetAdvancedHelpers(t *testing.T) {
	cfg := &ResourceSetConfig{
		Name:      "test-resourceset",
		Namespace: "flux-system",
	}

	resourceSet := ResourceSet(cfg)
	if resourceSet == nil {
		t.Fatal("failed to create ResourceSet")
	}

	// Test AddResourceSetResource
	resource := &apiextensionsv1.JSON{Raw: []byte(`{"apiVersion": "v1", "kind": "ConfigMap"}`)}
	err := AddResourceSetResource(resourceSet, resource)
	if err != nil {
		t.Errorf("AddResourceSetResource failed: %v", err)
	}

	// Test SetResourceSetResourcesTemplate
	template := "{{ .Values.configMap }}"
	err = SetResourceSetResourcesTemplate(resourceSet, template)
	if err != nil {
		t.Errorf("SetResourceSetResourcesTemplate failed: %v", err)
	}

	// Test AddResourceSetDependency
	dependency := fluxv1.Dependency{
		Name: "prerequisite-resource",
	}
	err = AddResourceSetDependency(resourceSet, dependency)
	if err != nil {
		t.Errorf("AddResourceSetDependency failed: %v", err)
	}

	// Test SetResourceSetCommonMetadata
	commonMetadata := &fluxv1.CommonMetadata{
		Labels: map[string]string{
			"app": "test",
		},
	}
	err = SetResourceSetCommonMetadata(resourceSet, commonMetadata)
	if err != nil {
		t.Errorf("SetResourceSetCommonMetadata failed: %v", err)
	}
}
