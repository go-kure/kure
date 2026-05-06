package fluxcd

import (
	"strings"
	"testing"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	imagev1 "github.com/fluxcd/image-automation-controller/api/v1"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	notificationv1 "github.com/fluxcd/notification-controller/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
)

func TestGitRepository_Success(t *testing.T) {
	cfg := &GitRepositoryConfig{
		Name:      "test-repo",
		Namespace: "flux-system",
		URL:       "https://github.com/example/repo",
		Interval:  "5m",
		Ref:       "main",
	}

	repo := GitRepository(cfg)

	if repo == nil {
		t.Fatal("expected non-nil GitRepository")
	}

	if repo.Name != "test-repo" {
		t.Errorf("expected Name 'test-repo', got %s", repo.Name)
	}

	if repo.Namespace != "flux-system" {
		t.Errorf("expected Namespace 'flux-system', got %s", repo.Namespace)
	}

	if repo.Spec.URL != "https://github.com/example/repo" {
		t.Errorf("expected URL 'https://github.com/example/repo', got %s", repo.Spec.URL)
	}

	expectedDuration := 5 * time.Minute
	if repo.Spec.Interval.Duration != expectedDuration {
		t.Errorf("expected interval %v, got %v", expectedDuration, repo.Spec.Interval.Duration)
	}

	if repo.Spec.Reference == nil {
		t.Fatal("expected non-nil Reference")
	}

	if repo.Spec.Reference.Branch != "main" {
		t.Errorf("expected branch 'main', got %s", repo.Spec.Reference.Branch)
	}
}

func TestGitRepository_NilConfig(t *testing.T) {
	repo := GitRepository(nil)
	if repo != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestGitRepository_NoRef(t *testing.T) {
	cfg := &GitRepositoryConfig{
		Name:      "test-repo",
		Namespace: "flux-system",
		URL:       "https://github.com/example/repo",
		Interval:  "5m",
		// No Ref specified
	}

	repo := GitRepository(cfg)

	if repo == nil {
		t.Fatal("expected non-nil GitRepository")
	}

	// Reference should not be set when Ref is empty
	if repo.Spec.Reference != nil {
		t.Error("expected nil Reference when Ref is not specified")
	}
}

func TestHelmRepository_Success(t *testing.T) {
	cfg := &HelmRepositoryConfig{
		Name:      "bitnami",
		Namespace: "flux-system",
		URL:       "https://charts.bitnami.com/bitnami",
	}

	helmRepo := HelmRepository(cfg)

	if helmRepo == nil {
		t.Fatal("expected non-nil HelmRepository")
	}

	if helmRepo.Name != "bitnami" {
		t.Errorf("expected Name 'bitnami', got %s", helmRepo.Name)
	}

	if helmRepo.Spec.URL != "https://charts.bitnami.com/bitnami" {
		t.Errorf("expected URL 'https://charts.bitnami.com/bitnami', got %s", helmRepo.Spec.URL)
	}
}

func TestHelmRepository_NilConfig(t *testing.T) {
	helmRepo := HelmRepository(nil)
	if helmRepo != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestHelmRepository_NoType(t *testing.T) {
	cfg := &HelmRepositoryConfig{
		Name:      "bitnami",
		Namespace: "flux-system",
		URL:       "https://charts.bitnami.com/bitnami",
	}

	helmRepo := HelmRepository(cfg)

	if helmRepo == nil {
		t.Fatal("expected non-nil HelmRepository")
	}

	if helmRepo.Spec.Type != "" {
		t.Errorf("expected empty Type, got %s", helmRepo.Spec.Type)
	}
}

func TestHelmRepository_OCI(t *testing.T) {
	cfg := &HelmRepositoryConfig{
		Name:      "ghcr",
		Namespace: "flux-system",
		URL:       "oci://ghcr.io/example/charts",
		Type:      sourcev1.HelmRepositoryTypeOCI,
	}

	helmRepo := HelmRepository(cfg)

	if helmRepo == nil {
		t.Fatal("expected non-nil HelmRepository")
	}

	if helmRepo.Spec.URL != "oci://ghcr.io/example/charts" {
		t.Errorf("expected URL 'oci://ghcr.io/example/charts', got %s", helmRepo.Spec.URL)
	}

	if helmRepo.Spec.Type != sourcev1.HelmRepositoryTypeOCI {
		t.Errorf("expected Type %q, got %q", sourcev1.HelmRepositoryTypeOCI, helmRepo.Spec.Type)
	}
}

func TestBucket_Success(t *testing.T) {
	cfg := &BucketConfig{
		Name:       "s3-bucket",
		Namespace:  "flux-system",
		BucketName: "my-flux-bucket",
		Endpoint:   "s3.amazonaws.com",
		Interval:   "10m",
		Provider:   "aws",
	}

	bucket := Bucket(cfg)

	if bucket == nil {
		t.Fatal("expected non-nil Bucket")
	}

	if bucket.Name != "s3-bucket" {
		t.Errorf("expected Name 's3-bucket', got %s", bucket.Name)
	}

	if bucket.Spec.BucketName != "my-flux-bucket" {
		t.Errorf("expected BucketName 'my-flux-bucket', got %s", bucket.Spec.BucketName)
	}

	if bucket.Spec.Endpoint != "s3.amazonaws.com" {
		t.Errorf("expected Endpoint 's3.amazonaws.com', got %s", bucket.Spec.Endpoint)
	}

	expectedDuration := 10 * time.Minute
	if bucket.Spec.Interval.Duration != expectedDuration {
		t.Errorf("expected interval %v, got %v", expectedDuration, bucket.Spec.Interval.Duration)
	}

	if bucket.Spec.Provider != "aws" {
		t.Errorf("expected Provider 'aws', got %s", bucket.Spec.Provider)
	}
}

func TestBucket_NoProvider(t *testing.T) {
	cfg := &BucketConfig{
		Name:       "s3-bucket",
		Namespace:  "flux-system",
		BucketName: "my-flux-bucket",
		Endpoint:   "s3.amazonaws.com",
		Interval:   "10m",
		// No Provider specified
	}

	bucket := Bucket(cfg)

	if bucket == nil {
		t.Fatal("expected non-nil Bucket")
	}

	// Provider should remain empty when not specified
	if bucket.Spec.Provider != "" {
		t.Errorf("expected empty Provider, got %s", bucket.Spec.Provider)
	}
}

func TestHelmChart_Success(t *testing.T) {
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

	chart := HelmChart(cfg)

	if chart == nil {
		t.Fatal("expected non-nil HelmChart")
	}

	if chart.Name != "nginx-chart" {
		t.Errorf("expected Name 'nginx-chart', got %s", chart.Name)
	}

	if chart.Spec.Chart != "nginx" {
		t.Errorf("expected Chart 'nginx', got %s", chart.Spec.Chart)
	}

	if chart.Spec.Version != "1.0.0" {
		t.Errorf("expected Version '1.0.0', got %s", chart.Spec.Version)
	}

	if chart.Spec.SourceRef.Name != "bitnami" {
		t.Errorf("expected SourceRef.Name 'bitnami', got %s", chart.Spec.SourceRef.Name)
	}

	expectedDuration := 1 * time.Hour
	if chart.Spec.Interval.Duration != expectedDuration {
		t.Errorf("expected interval %v, got %v", expectedDuration, chart.Spec.Interval.Duration)
	}
}

func TestOCIRepository_Success(t *testing.T) {
	cfg := &OCIRepositoryConfig{
		Name:      "test-oci",
		Namespace: "flux-system",
		URL:       "oci://registry.example.com/repo",
		Ref:       "v1.0.0",
		Interval:  "1m",
	}

	ociRepo := OCIRepository(cfg)

	if ociRepo == nil {
		t.Fatal("expected non-nil OCIRepository")
	}

	if ociRepo.Name != "test-oci" {
		t.Errorf("expected Name 'test-oci', got %s", ociRepo.Name)
	}

	if ociRepo.Spec.URL != "oci://registry.example.com/repo" {
		t.Errorf("expected URL 'oci://registry.example.com/repo', got %s", ociRepo.Spec.URL)
	}

	if ociRepo.Spec.Reference == nil {
		t.Fatal("expected non-nil Reference")
	}

	if ociRepo.Spec.Reference.Tag != "v1.0.0" {
		t.Errorf("expected tag 'v1.0.0', got %s", ociRepo.Spec.Reference.Tag)
	}

	expectedDuration := 1 * time.Minute
	if ociRepo.Spec.Interval.Duration != expectedDuration {
		t.Errorf("expected interval %v, got %v", expectedDuration, ociRepo.Spec.Interval.Duration)
	}
}

func TestOCIRepository_Digest(t *testing.T) {
	cfg := &OCIRepositoryConfig{
		Name:      "test-oci",
		Namespace: "flux-system",
		URL:       "oci://registry.example.com/repo",
		Ref:       "v1.0.0",
		Digest:    "sha256:abc123",
		Interval:  "1m",
	}
	ociRepo := OCIRepository(cfg)
	if ociRepo == nil {
		t.Fatal("expected non-nil OCIRepository")
	}
	if ociRepo.Spec.Reference == nil {
		t.Fatal("expected non-nil Reference")
	}
	if ociRepo.Spec.Reference.Digest != "sha256:abc123" {
		t.Errorf("expected digest 'sha256:abc123', got %s", ociRepo.Spec.Reference.Digest)
	}
	if ociRepo.Spec.Reference.Tag != "" {
		t.Errorf("expected empty Tag when Digest is set, got %s", ociRepo.Spec.Reference.Tag)
	}
}

func TestOCIRepository_DigestOnly(t *testing.T) {
	cfg := &OCIRepositoryConfig{
		Name:      "test-oci",
		Namespace: "flux-system",
		URL:       "oci://registry.example.com/repo",
		Digest:    "sha256:def456",
		Interval:  "1m",
	}
	ociRepo := OCIRepository(cfg)
	if ociRepo == nil {
		t.Fatal("expected non-nil OCIRepository")
	}
	if ociRepo.Spec.Reference == nil {
		t.Fatal("expected non-nil Reference")
	}
	if ociRepo.Spec.Reference.Digest != "sha256:def456" {
		t.Errorf("expected digest 'sha256:def456', got %s", ociRepo.Spec.Reference.Digest)
	}
	if ociRepo.Spec.Reference.Tag != "" {
		t.Errorf("expected empty Tag, got %s", ociRepo.Spec.Reference.Tag)
	}
}

func TestHelmRepository_Interval(t *testing.T) {
	cfg := &HelmRepositoryConfig{
		Name:      "bitnami",
		Namespace: "flux-system",
		URL:       "https://charts.bitnami.com/bitnami",
		Interval:  "10m",
	}
	helmRepo := HelmRepository(cfg)
	if helmRepo == nil {
		t.Fatal("expected non-nil HelmRepository")
	}
	expected := 10 * time.Minute
	if helmRepo.Spec.Interval.Duration != expected {
		t.Errorf("expected interval %v, got %v", expected, helmRepo.Spec.Interval.Duration)
	}
}

func TestKustomization_Success(t *testing.T) {
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

	kustomization := Kustomization(cfg)

	if kustomization == nil {
		t.Fatal("expected non-nil Kustomization")
	}

	if kustomization.Name != "app-kustomization" {
		t.Errorf("expected Name 'app-kustomization', got %s", kustomization.Name)
	}

	if !kustomization.Spec.Prune {
		t.Error("expected Prune to be true")
	}

	if kustomization.Spec.Path != "./deploy" {
		t.Errorf("expected Path './deploy', got %s", kustomization.Spec.Path)
	}

	if kustomization.Spec.SourceRef.Kind != "GitRepository" {
		t.Errorf("expected SourceRef.Kind 'GitRepository', got %s", kustomization.Spec.SourceRef.Kind)
	}

	expectedDuration := 2 * time.Minute
	if kustomization.Spec.Interval.Duration != expectedDuration {
		t.Errorf("expected interval %v, got %v", expectedDuration, kustomization.Spec.Interval.Duration)
	}
}

func TestKustomization_NoPath(t *testing.T) {
	sourceRef := kustv1.CrossNamespaceSourceReference{
		Kind: "GitRepository",
		Name: "app-repo",
	}

	cfg := &KustomizationConfig{
		Name:      "app-kustomization",
		Namespace: "default",
		// No Path specified
		Interval:  "2m",
		Prune:     false,
		SourceRef: sourceRef,
	}

	kustomization := Kustomization(cfg)

	if kustomization == nil {
		t.Fatal("expected non-nil Kustomization")
	}

	// Path should remain empty when not specified
	if kustomization.Spec.Path != "" {
		t.Errorf("expected empty Path, got %s", kustomization.Spec.Path)
	}
}

func TestKustomization_TargetNamespaceAndWait(t *testing.T) {
	cfg := &KustomizationConfig{
		Name:            "app",
		Namespace:       "flux-system",
		Interval:        "5m",
		Prune:           true,
		SourceRef:       kustv1.CrossNamespaceSourceReference{Kind: "GitRepository", Name: "repo"},
		TargetNamespace: "production",
		Wait:            true,
	}
	ks := Kustomization(cfg)
	if ks == nil {
		t.Fatal("expected non-nil Kustomization")
	}
	if ks.Spec.TargetNamespace != "production" {
		t.Errorf("expected TargetNamespace 'production', got %s", ks.Spec.TargetNamespace)
	}
	if !ks.Spec.Wait {
		t.Error("expected Wait true")
	}
}

func TestHelmRelease_Values(t *testing.T) {
	cfg := &HelmReleaseConfig{
		Name:      "my-app",
		Namespace: "flux-system",
		Interval:  "10m",
		Chart:     "nginx",
		SourceRef: helmv2.CrossNamespaceObjectReference{Kind: "HelmRepository", Name: "bitnami"},
		Values: map[string]any{
			"replicaCount": 3,
			"image": map[string]any{
				"tag": "latest",
			},
		},
	}
	hr := HelmRelease(cfg)
	if hr == nil {
		t.Fatal("expected non-nil HelmRelease")
	}
	if hr.Spec.Values == nil {
		t.Fatal("expected non-nil Values")
	}
	raw := string(hr.Spec.Values.Raw)
	if raw == "" {
		t.Error("expected non-empty Values.Raw")
	}
	// Verify the JSON contains expected keys.
	if !strings.Contains(raw, `"replicaCount"`) {
		t.Errorf("expected Values.Raw to contain replicaCount, got %s", raw)
	}
	if !strings.Contains(raw, `"tag"`) {
		t.Errorf("expected Values.Raw to contain image.tag, got %s", raw)
	}
}

func TestHelmRelease_Success(t *testing.T) {
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

	helmRelease := HelmRelease(cfg)

	if helmRelease == nil {
		t.Fatal("expected non-nil HelmRelease")
	}

	if helmRelease.Name != "my-nginx" {
		t.Errorf("expected Name 'my-nginx', got %s", helmRelease.Name)
	}

	if helmRelease.Spec.Chart == nil {
		t.Fatal("expected non-nil Chart")
	}

	if helmRelease.Spec.Chart.Spec.Chart != "nginx" {
		t.Errorf("expected Chart 'nginx', got %s", helmRelease.Spec.Chart.Spec.Chart)
	}

	if helmRelease.Spec.Chart.Spec.Version != "1.2.3" {
		t.Errorf("expected Version '1.2.3', got %s", helmRelease.Spec.Chart.Spec.Version)
	}

	if helmRelease.Spec.Chart.Spec.SourceRef.Name != "bitnami" {
		t.Errorf("expected SourceRef.Name 'bitnami', got %s", helmRelease.Spec.Chart.Spec.SourceRef.Name)
	}

	if helmRelease.Spec.ReleaseName != "nginx-release" {
		t.Errorf("expected ReleaseName 'nginx-release', got %s", helmRelease.Spec.ReleaseName)
	}

	expectedDuration := 1 * time.Hour
	if helmRelease.Spec.Interval.Duration != expectedDuration {
		t.Errorf("expected interval %v, got %v", expectedDuration, helmRelease.Spec.Interval.Duration)
	}
}

func TestHelmRelease_ChartRef(t *testing.T) {
	cfg := &HelmReleaseConfig{
		Name:      "my-app",
		Namespace: "apps",
		Interval:  "10m",
		ChartRef: &ChartRefConfig{
			Kind:      "OCIRepository",
			Name:      "my-oci-source",
			Namespace: "flux-system",
		},
	}
	hr := HelmRelease(cfg)
	if hr == nil {
		t.Fatal("expected non-nil HelmRelease")
	}
	if hr.Spec.ChartRef == nil {
		t.Fatal("expected non-nil ChartRef")
	}
	if hr.Spec.ChartRef.Kind != "OCIRepository" {
		t.Errorf("expected ChartRef.Kind OCIRepository, got %s", hr.Spec.ChartRef.Kind)
	}
	if hr.Spec.ChartRef.Name != "my-oci-source" {
		t.Errorf("expected ChartRef.Name my-oci-source, got %s", hr.Spec.ChartRef.Name)
	}
	if hr.Spec.ChartRef.Namespace != "flux-system" {
		t.Errorf("expected ChartRef.Namespace flux-system, got %s", hr.Spec.ChartRef.Namespace)
	}
	if hr.Spec.Chart != nil {
		t.Error("expected nil Chart when ChartRef is set")
	}
}

func TestHelmRelease_ChartRefNoNamespace(t *testing.T) {
	cfg := &HelmReleaseConfig{
		Name:      "my-app",
		Namespace: "apps",
		Interval:  "10m",
		ChartRef:  &ChartRefConfig{Kind: "HelmChart", Name: "local-chart"},
	}
	hr := HelmRelease(cfg)
	if hr.Spec.ChartRef == nil {
		t.Fatal("expected non-nil ChartRef")
	}
	if hr.Spec.ChartRef.Namespace != "" {
		t.Errorf("expected empty Namespace, got %q", hr.Spec.ChartRef.Namespace)
	}
}

func TestHelmRelease_TargetNamespace(t *testing.T) {
	cfg := &HelmReleaseConfig{
		Name:            "my-app",
		Namespace:       "flux-system",
		Interval:        "10m",
		Chart:           "nginx",
		SourceRef:       helmv2.CrossNamespaceObjectReference{Kind: "HelmRepository", Name: "bitnami"},
		TargetNamespace: "production",
	}
	hr := HelmRelease(cfg)
	if hr.Spec.TargetNamespace != "production" {
		t.Errorf("expected TargetNamespace production, got %s", hr.Spec.TargetNamespace)
	}
}

func TestHelmRelease_DriftDetection(t *testing.T) {
	cfg := &HelmReleaseConfig{
		Name:               "my-app",
		Namespace:          "flux-system",
		Interval:           "10m",
		Chart:              "nginx",
		SourceRef:          helmv2.CrossNamespaceObjectReference{Kind: "HelmRepository", Name: "bitnami"},
		DriftDetectionMode: "enabled",
	}
	hr := HelmRelease(cfg)
	if hr.Spec.DriftDetection == nil {
		t.Fatal("expected non-nil DriftDetection")
	}
	if string(hr.Spec.DriftDetection.Mode) != "enabled" {
		t.Errorf("expected DriftDetection.Mode enabled, got %s", hr.Spec.DriftDetection.Mode)
	}
}

func TestHelmRelease_InstallUpgrade(t *testing.T) {
	retries := 3
	remediateLastFailure := true
	cfg := &HelmReleaseConfig{
		Name:                 "my-app",
		Namespace:            "flux-system",
		Interval:             "10m",
		Chart:                "nginx",
		SourceRef:            helmv2.CrossNamespaceObjectReference{Kind: "HelmRepository", Name: "bitnami"},
		InstallCRDs:          "CreateReplace",
		InstallRetries:       &retries,
		UpgradeCRDs:          "Skip",
		UpgradeRetries:       &retries,
		RemediateLastFailure: &remediateLastFailure,
		UpgradeCleanupOnFail: true,
	}
	hr := HelmRelease(cfg)
	if hr.Spec.Install == nil {
		t.Fatal("expected non-nil Install")
	}
	if string(hr.Spec.Install.CRDs) != "CreateReplace" {
		t.Errorf("expected Install.CRDs CreateReplace, got %s", hr.Spec.Install.CRDs)
	}
	if hr.Spec.Install.Remediation == nil || hr.Spec.Install.Remediation.Retries != 3 {
		t.Errorf("expected Install.Remediation.Retries 3, got %v", hr.Spec.Install.Remediation)
	}
	if hr.Spec.Upgrade == nil {
		t.Fatal("expected non-nil Upgrade")
	}
	if string(hr.Spec.Upgrade.CRDs) != "Skip" {
		t.Errorf("expected Upgrade.CRDs Skip, got %s", hr.Spec.Upgrade.CRDs)
	}
	if !hr.Spec.Upgrade.CleanupOnFail {
		t.Error("expected Upgrade.CleanupOnFail true")
	}
	if hr.Spec.Upgrade.Remediation == nil || hr.Spec.Upgrade.Remediation.Retries != 3 {
		t.Errorf("expected Upgrade.Remediation.Retries 3, got %v", hr.Spec.Upgrade.Remediation)
	}
	if hr.Spec.Upgrade.Remediation.RemediateLastFailure == nil || !*hr.Spec.Upgrade.Remediation.RemediateLastFailure {
		t.Error("expected RemediateLastFailure true")
	}
}

func TestHelmRelease_RollbackCleanupOnFail(t *testing.T) {
	cfg := &HelmReleaseConfig{
		Name:                  "my-app",
		Namespace:             "flux-system",
		Interval:              "10m",
		Chart:                 "nginx",
		SourceRef:             helmv2.CrossNamespaceObjectReference{Kind: "HelmRepository", Name: "bitnami"},
		RollbackCleanupOnFail: true,
	}
	hr := HelmRelease(cfg)
	if hr.Spec.Rollback == nil {
		t.Fatal("expected non-nil Rollback")
	}
	if !hr.Spec.Rollback.CleanupOnFail {
		t.Error("expected Rollback.CleanupOnFail true")
	}
}

func TestHelmRelease_ValuesFrom(t *testing.T) {
	cfg := &HelmReleaseConfig{
		Name:      "my-app",
		Namespace: "flux-system",
		Interval:  "10m",
		Chart:     "nginx",
		SourceRef: helmv2.CrossNamespaceObjectReference{Kind: "HelmRepository", Name: "bitnami"},
		ValuesFrom: []ValuesFromConfig{
			{Kind: "ConfigMap", Name: "my-values", ValuesKey: "values.yaml"},
			{Kind: "Secret", Name: "my-secret", TargetPath: "secret.key", Optional: true},
		},
	}
	hr := HelmRelease(cfg)
	if len(hr.Spec.ValuesFrom) != 2 {
		t.Fatalf("expected 2 ValuesFrom entries, got %d", len(hr.Spec.ValuesFrom))
	}
	if hr.Spec.ValuesFrom[0].Kind != "ConfigMap" || hr.Spec.ValuesFrom[0].Name != "my-values" {
		t.Errorf("unexpected ValuesFrom[0]: %+v", hr.Spec.ValuesFrom[0])
	}
	if hr.Spec.ValuesFrom[1].Kind != "Secret" || !hr.Spec.ValuesFrom[1].Optional {
		t.Errorf("unexpected ValuesFrom[1]: %+v", hr.Spec.ValuesFrom[1])
	}
	if hr.Spec.ValuesFrom[1].TargetPath != "secret.key" {
		t.Errorf("expected TargetPath secret.key, got %s", hr.Spec.ValuesFrom[1].TargetPath)
	}
}

func TestHelmRelease_NoDriftDetectionWhenEmpty(t *testing.T) {
	cfg := &HelmReleaseConfig{
		Name:      "my-app",
		Namespace: "flux-system",
		Interval:  "10m",
		Chart:     "nginx",
		SourceRef: helmv2.CrossNamespaceObjectReference{Kind: "HelmRepository", Name: "bitnami"},
	}
	hr := HelmRelease(cfg)
	if hr.Spec.DriftDetection != nil {
		t.Errorf("expected nil DriftDetection when mode is empty, got %+v", hr.Spec.DriftDetection)
	}
	if hr.Spec.Install != nil {
		t.Errorf("expected nil Install when no install fields set, got %+v", hr.Spec.Install)
	}
	if hr.Spec.Upgrade != nil {
		t.Errorf("expected nil Upgrade when no upgrade fields set, got %+v", hr.Spec.Upgrade)
	}
	if hr.Spec.Rollback != nil {
		t.Errorf("expected nil Rollback when RollbackCleanupOnFail is false, got %+v", hr.Spec.Rollback)
	}
}

func TestProvider_Success(t *testing.T) {
	cfg := &ProviderConfig{
		Name:      "slack-provider",
		Namespace: "flux-system",
		Type:      "slack",
		Address:   "https://hooks.slack.com/services/...",
		Channel:   "#alerts",
	}

	provider := Provider(cfg)

	if provider == nil {
		t.Fatal("expected non-nil Provider")
	}

	if provider.Name != "slack-provider" {
		t.Errorf("expected Name 'slack-provider', got %s", provider.Name)
	}

	if provider.Spec.Type != "slack" {
		t.Errorf("expected Type 'slack', got %s", provider.Spec.Type)
	}

	if provider.Spec.Address != "https://hooks.slack.com/services/..." {
		t.Errorf("expected specific Address, got %s", provider.Spec.Address)
	}

	if provider.Spec.Channel != "#alerts" {
		t.Errorf("expected Channel '#alerts', got %s", provider.Spec.Channel)
	}
}

func TestAlert_Success(t *testing.T) {
	eventSources := []notificationv1.CrossNamespaceObjectReference{
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

	alert := Alert(cfg)

	if alert == nil {
		t.Fatal("expected non-nil Alert")
	}

	if alert.Name != "app-alert" {
		t.Errorf("expected Name 'app-alert', got %s", alert.Name)
	}

	if alert.Spec.ProviderRef.Name != "slack-provider" {
		t.Errorf("expected ProviderRef.Name 'slack-provider', got %s", alert.Spec.ProviderRef.Name)
	}

	if len(alert.Spec.EventSources) != 2 {
		t.Errorf("expected 2 event sources, got %d", len(alert.Spec.EventSources))
	}

	if alert.Spec.EventSources[0].Kind != "Kustomization" {
		t.Errorf("expected first EventSource.Kind 'Kustomization', got %s", alert.Spec.EventSources[0].Kind)
	}

	if alert.Spec.EventSeverity != "error" {
		t.Errorf("expected EventSeverity 'error', got %s", alert.Spec.EventSeverity)
	}
}

func TestReceiver_Success(t *testing.T) {
	resources := []notificationv1.CrossNamespaceObjectReference{
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

	receiver := Receiver(cfg)

	if receiver == nil {
		t.Fatal("expected non-nil Receiver")
	}

	if receiver.Name != "webhook-receiver" {
		t.Errorf("expected Name 'webhook-receiver', got %s", receiver.Name)
	}

	if receiver.Spec.Type != "github" {
		t.Errorf("expected Type 'github', got %s", receiver.Spec.Type)
	}

	if receiver.Spec.SecretRef.Name != "webhook-secret" {
		t.Errorf("expected SecretRef.Name 'webhook-secret', got %s", receiver.Spec.SecretRef.Name)
	}

	if len(receiver.Spec.Resources) != 1 {
		t.Errorf("expected 1 resource, got %d", len(receiver.Spec.Resources))
	}

	if len(receiver.Spec.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(receiver.Spec.Events))
	}

	if receiver.Spec.Events[0] != "push" {
		t.Errorf("expected first event 'push', got %s", receiver.Spec.Events[0])
	}
}

func TestImageUpdateAutomation_Success(t *testing.T) {
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
		t.Fatal("expected non-nil ImageUpdateAutomation")
	}

	if imageUpdate.Name != "image-updater" {
		t.Errorf("expected Name 'image-updater', got %s", imageUpdate.Name)
	}

	if imageUpdate.Spec.SourceRef.Kind != "GitRepository" {
		t.Errorf("expected SourceRef.Kind 'GitRepository', got %s", imageUpdate.Spec.SourceRef.Kind)
	}

	expectedDuration := 30 * time.Minute
	if imageUpdate.Spec.Interval.Duration != expectedDuration {
		t.Errorf("expected interval %v, got %v", expectedDuration, imageUpdate.Spec.Interval.Duration)
	}
}

func TestResourceSet_Success(t *testing.T) {
	cfg := &ResourceSetConfig{
		Name:      "test-resourceset",
		Namespace: "flux-system",
	}

	resourceSet := ResourceSet(cfg)

	if resourceSet == nil {
		t.Fatal("expected non-nil ResourceSet")
	}

	if resourceSet.Name != "test-resourceset" {
		t.Errorf("expected Name 'test-resourceset', got %s", resourceSet.Name)
	}

	if resourceSet.Namespace != "flux-system" {
		t.Errorf("expected Namespace 'flux-system', got %s", resourceSet.Namespace)
	}
}

func TestResourceSetInputProvider_Success(t *testing.T) {
	cfg := &ResourceSetInputProviderConfig{
		Name:      "input-provider",
		Namespace: "flux-system",
		Type:      "http",
		URL:       "https://api.example.com/config",
	}

	provider := ResourceSetInputProvider(cfg)

	if provider == nil {
		t.Fatal("expected non-nil ResourceSetInputProvider")
	}

	if provider.Name != "input-provider" {
		t.Errorf("expected Name 'input-provider', got %s", provider.Name)
	}

	if provider.Spec.Type != "http" {
		t.Errorf("expected Type 'http', got %s", provider.Spec.Type)
	}

	if provider.Spec.URL != "https://api.example.com/config" {
		t.Errorf("expected URL 'https://api.example.com/config', got %s", provider.Spec.URL)
	}
}

func TestFluxInstance_Success(t *testing.T) {
	cfg := &FluxInstanceConfig{
		Name:      "flux-instance",
		Namespace: "flux-system",
		Version:   "v2.1.0",
		Registry:  "ghcr.io/fluxcd",
	}

	instance := FluxInstance(cfg)

	if instance == nil {
		t.Fatal("expected non-nil FluxInstance")
	}

	if instance.Name != "flux-instance" {
		t.Errorf("expected Name 'flux-instance', got %s", instance.Name)
	}

	if instance.Spec.Distribution.Version != "v2.1.0" {
		t.Errorf("expected Version 'v2.1.0', got %s", instance.Spec.Distribution.Version)
	}

	if instance.Spec.Distribution.Registry != "ghcr.io/fluxcd" {
		t.Errorf("expected Registry 'ghcr.io/fluxcd', got %s", instance.Spec.Distribution.Registry)
	}
}

func TestFluxReport_Success(t *testing.T) {
	cfg := &FluxReportConfig{
		Name:        "flux-report",
		Namespace:   "flux-system",
		Entitlement: "enterprise",
		Status:      "active",
	}

	report := FluxReport(cfg)

	if report == nil {
		t.Fatal("expected non-nil FluxReport")
	}

	if report.Name != "flux-report" {
		t.Errorf("expected Name 'flux-report', got %s", report.Name)
	}

	if report.Spec.Distribution.Entitlement != "enterprise" {
		t.Errorf("expected Entitlement 'enterprise', got %s", report.Spec.Distribution.Entitlement)
	}

	if report.Spec.Distribution.Status != "active" {
		t.Errorf("expected Status 'active', got %s", report.Spec.Distribution.Status)
	}
}

func TestParseDurationOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
	}{
		{
			name:     "valid duration",
			input:    "5m",
			expected: 5 * time.Minute,
		},
		{
			name:     "valid hour duration",
			input:    "1h",
			expected: 1 * time.Hour,
		},
		{
			name:     "valid seconds duration",
			input:    "30s",
			expected: 30 * time.Second,
		},
		{
			name:     "invalid duration",
			input:    "invalid",
			expected: 5 * time.Minute, // default
		},
		{
			name:     "empty string",
			input:    "",
			expected: 5 * time.Minute, // default
		},
		{
			name:     "malformed duration",
			input:    "5minutes",
			expected: 5 * time.Minute, // default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDurationOrDefault(tt.input)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestAllConstructorsWithNilConfig(t *testing.T) {
	// Test that all constructor functions handle nil config gracefully
	constructors := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{"GitRepository", func(t *testing.T) {
			if GitRepository(nil) != nil {
				t.Error("GitRepository should return nil for nil config")
			}
		}},
		{"HelmRepository", func(t *testing.T) {
			if HelmRepository(nil) != nil {
				t.Error("HelmRepository should return nil for nil config")
			}
		}},
		{"Bucket", func(t *testing.T) {
			if Bucket(nil) != nil {
				t.Error("Bucket should return nil for nil config")
			}
		}},
		{"HelmChart", func(t *testing.T) {
			if HelmChart(nil) != nil {
				t.Error("HelmChart should return nil for nil config")
			}
		}},
		{"OCIRepository", func(t *testing.T) {
			if OCIRepository(nil) != nil {
				t.Error("OCIRepository should return nil for nil config")
			}
		}},
		{"Kustomization", func(t *testing.T) {
			if Kustomization(nil) != nil {
				t.Error("Kustomization should return nil for nil config")
			}
		}},
		{"HelmRelease", func(t *testing.T) {
			if HelmRelease(nil) != nil {
				t.Error("HelmRelease should return nil for nil config")
			}
		}},
		{"Provider", func(t *testing.T) {
			if Provider(nil) != nil {
				t.Error("Provider should return nil for nil config")
			}
		}},
		{"Alert", func(t *testing.T) {
			if Alert(nil) != nil {
				t.Error("Alert should return nil for nil config")
			}
		}},
		{"Receiver", func(t *testing.T) {
			if Receiver(nil) != nil {
				t.Error("Receiver should return nil for nil config")
			}
		}},
		{"ImageUpdateAutomation", func(t *testing.T) {
			if ImageUpdateAutomation(nil) != nil {
				t.Error("ImageUpdateAutomation should return nil for nil config")
			}
		}},
		{"ResourceSet", func(t *testing.T) {
			if ResourceSet(nil) != nil {
				t.Error("ResourceSet should return nil for nil config")
			}
		}},
		{"ResourceSetInputProvider", func(t *testing.T) {
			if ResourceSetInputProvider(nil) != nil {
				t.Error("ResourceSetInputProvider should return nil for nil config")
			}
		}},
		{"FluxInstance", func(t *testing.T) {
			if FluxInstance(nil) != nil {
				t.Error("FluxInstance should return nil for nil config")
			}
		}},
		{"FluxReport", func(t *testing.T) {
			if FluxReport(nil) != nil {
				t.Error("FluxReport should return nil for nil config")
			}
		}},
	}

	for _, constructor := range constructors {
		t.Run(constructor.name, constructor.fn)
	}
}
