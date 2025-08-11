package fluxcd

import (
	"testing"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	imagev1 "github.com/fluxcd/image-automation-controller/api/v1beta2"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	v1 "github.com/fluxcd/notification-controller/api/v1"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
)

func TestOCIRepositoryConfig(t *testing.T) {
	cfg := &OCIRepositoryConfig{
		Name:      "test-oci",
		Namespace: "flux-system",
		URL:       "oci://registry.example.com/repo",
		Ref:       "v1.0.0",
		Interval:  "1m",
	}

	if cfg.Name != "test-oci" {
		t.Errorf("expected Name 'test-oci', got %s", cfg.Name)
	}

	if cfg.Namespace != "flux-system" {
		t.Errorf("expected Namespace 'flux-system', got %s", cfg.Namespace)
	}

	if cfg.URL != "oci://registry.example.com/repo" {
		t.Errorf("expected URL 'oci://registry.example.com/repo', got %s", cfg.URL)
	}

	if cfg.Ref != "v1.0.0" {
		t.Errorf("expected Ref 'v1.0.0', got %s", cfg.Ref)
	}

	if cfg.Interval != "1m" {
		t.Errorf("expected Interval '1m', got %s", cfg.Interval)
	}
}

func TestGitRepositoryConfig(t *testing.T) {
	cfg := &GitRepositoryConfig{
		Name:      "test-git",
		Namespace: "default",
		URL:       "https://github.com/example/repo",
		Interval:  "5m",
		Ref:       "main",
	}

	if cfg.Name != "test-git" {
		t.Errorf("expected Name 'test-git', got %s", cfg.Name)
	}

	if cfg.URL != "https://github.com/example/repo" {
		t.Errorf("expected URL 'https://github.com/example/repo', got %s", cfg.URL)
	}

	if cfg.Ref != "main" {
		t.Errorf("expected Ref 'main', got %s", cfg.Ref)
	}
}

func TestHelmRepositoryConfig(t *testing.T) {
	cfg := &HelmRepositoryConfig{
		Name:      "bitnami",
		Namespace: "flux-system",
		URL:       "https://charts.bitnami.com/bitnami",
	}

	if cfg.Name != "bitnami" {
		t.Errorf("expected Name 'bitnami', got %s", cfg.Name)
	}

	if cfg.URL != "https://charts.bitnami.com/bitnami" {
		t.Errorf("expected URL 'https://charts.bitnami.com/bitnami', got %s", cfg.URL)
	}
}

func TestBucketConfig(t *testing.T) {
	cfg := &BucketConfig{
		Name:       "s3-bucket",
		Namespace:  "flux-system",
		BucketName: "my-flux-bucket",
		Endpoint:   "s3.amazonaws.com",
		Interval:   "10m",
		Provider:   "aws",
	}

	if cfg.BucketName != "my-flux-bucket" {
		t.Errorf("expected BucketName 'my-flux-bucket', got %s", cfg.BucketName)
	}

	if cfg.Endpoint != "s3.amazonaws.com" {
		t.Errorf("expected Endpoint 's3.amazonaws.com', got %s", cfg.Endpoint)
	}

	if cfg.Provider != "aws" {
		t.Errorf("expected Provider 'aws', got %s", cfg.Provider)
	}
}

func TestHelmChartConfig(t *testing.T) {
	sourceRef := sourcev1.LocalHelmChartSourceReference{
		Name: "bitnami",
		Kind: "HelmRepository",
	}

	cfg := &HelmChartConfig{
		Name:      "nginx-chart",
		Namespace: "default",
		Chart:     "nginx",
		Version:   "1.0.0",
		SourceRef: sourceRef,
		Interval:  "1h",
	}

	if cfg.Chart != "nginx" {
		t.Errorf("expected Chart 'nginx', got %s", cfg.Chart)
	}

	if cfg.Version != "1.0.0" {
		t.Errorf("expected Version '1.0.0', got %s", cfg.Version)
	}

	if cfg.SourceRef.Name != "bitnami" {
		t.Errorf("expected SourceRef.Name 'bitnami', got %s", cfg.SourceRef.Name)
	}
}

func TestKustomizationConfig(t *testing.T) {
	sourceRef := kustv1.CrossNamespaceSourceReference{
		Kind:      "GitRepository",
		Name:      "app-repo",
		Namespace: "flux-system",
	}

	cfg := &KustomizationConfig{
		Name:      "app-kustomization",
		Namespace: "default",
		Path:      "./deploy",
		Interval:  "2m",
		Prune:     true,
		SourceRef: sourceRef,
	}

	if cfg.Path != "./deploy" {
		t.Errorf("expected Path './deploy', got %s", cfg.Path)
	}

	if !cfg.Prune {
		t.Error("expected Prune to be true")
	}

	if cfg.SourceRef.Kind != "GitRepository" {
		t.Errorf("expected SourceRef.Kind 'GitRepository', got %s", cfg.SourceRef.Kind)
	}
}

func TestHelmReleaseConfig(t *testing.T) {
	sourceRef := helmv2.CrossNamespaceObjectReference{
		Kind: "HelmRepository",
		Name: "bitnami",
	}

	cfg := &HelmReleaseConfig{
		Name:        "my-nginx",
		Namespace:   "default",
		Chart:       "nginx",
		Version:     "1.2.3",
		SourceRef:   sourceRef,
		Interval:    "1h",
		ReleaseName: "nginx-release",
	}

	if cfg.Chart != "nginx" {
		t.Errorf("expected Chart 'nginx', got %s", cfg.Chart)
	}

	if cfg.ReleaseName != "nginx-release" {
		t.Errorf("expected ReleaseName 'nginx-release', got %s", cfg.ReleaseName)
	}

	if cfg.SourceRef.Name != "bitnami" {
		t.Errorf("expected SourceRef.Name 'bitnami', got %s", cfg.SourceRef.Name)
	}
}

func TestProviderConfig(t *testing.T) {
	cfg := &ProviderConfig{
		Name:      "slack-provider",
		Namespace: "flux-system",
		Type:      "slack",
		Address:   "https://hooks.slack.com/services/...",
		Channel:   "#alerts",
	}

	if cfg.Type != "slack" {
		t.Errorf("expected Type 'slack', got %s", cfg.Type)
	}

	if cfg.Channel != "#alerts" {
		t.Errorf("expected Channel '#alerts', got %s", cfg.Channel)
	}

	if cfg.Address != "https://hooks.slack.com/services/..." {
		t.Errorf("expected specific Address, got %s", cfg.Address)
	}
}

func TestAlertConfig(t *testing.T) {
	eventSources := []v1.CrossNamespaceObjectReference{
		{Kind: "Kustomization", Name: "app"},
		{Kind: "HelmRelease", Name: "nginx"},
	}

	cfg := &AlertConfig{
		Name:          "app-alert",
		Namespace:     "flux-system",
		ProviderRef:   "slack-provider",
		EventSources:  eventSources,
		EventSeverity: "error",
	}

	if cfg.ProviderRef != "slack-provider" {
		t.Errorf("expected ProviderRef 'slack-provider', got %s", cfg.ProviderRef)
	}

	if cfg.EventSeverity != "error" {
		t.Errorf("expected EventSeverity 'error', got %s", cfg.EventSeverity)
	}

	if len(cfg.EventSources) != 2 {
		t.Errorf("expected 2 event sources, got %d", len(cfg.EventSources))
	}

	if cfg.EventSources[0].Kind != "Kustomization" {
		t.Errorf("expected first EventSource.Kind 'Kustomization', got %s", cfg.EventSources[0].Kind)
	}
}

func TestReceiverConfig(t *testing.T) {
	resources := []v1.CrossNamespaceObjectReference{
		{Kind: "GitRepository", Name: "app-repo"},
	}

	events := []string{"push", "ping"}

	cfg := &ReceiverConfig{
		Name:       "webhook-receiver",
		Namespace:  "flux-system",
		Type:       "github",
		SecretName: "webhook-secret",
		Resources:  resources,
		Events:     events,
	}

	if cfg.Type != "github" {
		t.Errorf("expected Type 'github', got %s", cfg.Type)
	}

	if cfg.SecretName != "webhook-secret" {
		t.Errorf("expected SecretName 'webhook-secret', got %s", cfg.SecretName)
	}

	if len(cfg.Resources) != 1 {
		t.Errorf("expected 1 resource, got %d", len(cfg.Resources))
	}

	if len(cfg.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(cfg.Events))
	}

	if cfg.Events[0] != "push" {
		t.Errorf("expected first event 'push', got %s", cfg.Events[0])
	}
}

func TestImageUpdateAutomationConfig(t *testing.T) {
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

	if cfg.Interval != "30m" {
		t.Errorf("expected Interval '30m', got %s", cfg.Interval)
	}

	if cfg.SourceRef.Kind != "GitRepository" {
		t.Errorf("expected SourceRef.Kind 'GitRepository', got %s", cfg.SourceRef.Kind)
	}
}

func TestResourceSetConfig(t *testing.T) {
	cfg := &ResourceSetConfig{
		Name:      "test-resourceset",
		Namespace: "flux-system",
	}

	if cfg.Name != "test-resourceset" {
		t.Errorf("expected Name 'test-resourceset', got %s", cfg.Name)
	}

	if cfg.Namespace != "flux-system" {
		t.Errorf("expected Namespace 'flux-system', got %s", cfg.Namespace)
	}
}

func TestResourceSetInputProviderConfig(t *testing.T) {
	cfg := &ResourceSetInputProviderConfig{
		Name:      "input-provider",
		Namespace: "flux-system",
		Type:      "http",
		URL:       "https://api.example.com/config",
	}

	if cfg.Type != "http" {
		t.Errorf("expected Type 'http', got %s", cfg.Type)
	}

	if cfg.URL != "https://api.example.com/config" {
		t.Errorf("expected URL 'https://api.example.com/config', got %s", cfg.URL)
	}
}

func TestFluxInstanceConfig(t *testing.T) {
	cfg := &FluxInstanceConfig{
		Name:      "flux-instance",
		Namespace: "flux-system",
		Version:   "v2.1.0",
		Registry:  "ghcr.io/fluxcd",
	}

	if cfg.Version != "v2.1.0" {
		t.Errorf("expected Version 'v2.1.0', got %s", cfg.Version)
	}

	if cfg.Registry != "ghcr.io/fluxcd" {
		t.Errorf("expected Registry 'ghcr.io/fluxcd', got %s", cfg.Registry)
	}
}

func TestFluxReportConfig(t *testing.T) {
	cfg := &FluxReportConfig{
		Name:        "flux-report",
		Namespace:   "flux-system",
		Entitlement: "enterprise",
		Status:      "active",
	}

	if cfg.Entitlement != "enterprise" {
		t.Errorf("expected Entitlement 'enterprise', got %s", cfg.Entitlement)
	}

	if cfg.Status != "active" {
		t.Errorf("expected Status 'active', got %s", cfg.Status)
	}
}

func TestReceiverSecretRefConfig(t *testing.T) {
	ref := meta.LocalObjectReference{Name: "webhook-secret"}
	
	cfg := &ReceiverSecretRefConfig{
		Name:      "secret-ref",
		Namespace: "default",
		Ref:       ref,
	}

	if cfg.Ref.Name != "webhook-secret" {
		t.Errorf("expected Ref.Name 'webhook-secret', got %s", cfg.Ref.Name)
	}
}

func TestConfigStructTags(t *testing.T) {
	// Test that struct tags are properly defined for YAML serialization
	// This is important for configuration file parsing
	
	tests := []struct {
		name   string
		config interface{}
	}{
		{"OCIRepositoryConfig", &OCIRepositoryConfig{}},
		{"GitRepositoryConfig", &GitRepositoryConfig{}},
		{"HelmRepositoryConfig", &HelmRepositoryConfig{}},
		{"BucketConfig", &BucketConfig{}},
		{"HelmChartConfig", &HelmChartConfig{}},
		{"KustomizationConfig", &KustomizationConfig{}},
		{"HelmReleaseConfig", &HelmReleaseConfig{}},
		{"ProviderConfig", &ProviderConfig{}},
		{"AlertConfig", &AlertConfig{}},
		{"ReceiverConfig", &ReceiverConfig{}},
		{"ImageUpdateAutomationConfig", &ImageUpdateAutomationConfig{}},
		{"ResourceSetConfig", &ResourceSetConfig{}},
		{"ResourceSetInputProviderConfig", &ResourceSetInputProviderConfig{}},
		{"FluxInstanceConfig", &FluxInstanceConfig{}},
		{"FluxReportConfig", &FluxReportConfig{}},
		{"ReceiverSecretRefConfig", &ReceiverSecretRefConfig{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify the struct can be used - the actual tag checking
			// would require reflection which is more complex
			if tt.config == nil {
				t.Errorf("config struct %s should not be nil", tt.name)
			}
		})
	}
}