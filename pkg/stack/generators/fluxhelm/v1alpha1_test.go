package fluxhelm

import (
	"testing"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/generators"
	"github.com/go-kure/kure/pkg/stack/generators/fluxhelm/internal"
)

func TestConfigV1Alpha1_GetAPIVersion(t *testing.T) {
	cfg := &ConfigV1Alpha1{}
	expected := "generators.gokure.dev/v1alpha1"
	if got := cfg.GetAPIVersion(); got != expected {
		t.Errorf("GetAPIVersion() = %s, want %s", got, expected)
	}
}

func TestConfigV1Alpha1_GetKind(t *testing.T) {
	cfg := &ConfigV1Alpha1{}
	expected := "FluxHelm"
	if got := cfg.GetKind(); got != expected {
		t.Errorf("GetKind() = %s, want %s", got, expected)
	}
}

func TestConfigV1Alpha1_Generate_HelmRepository(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "postgresql",
			Namespace: "database",
		},
		Chart: internal.ChartConfig{
			Name:    "postgresql",
			Version: "12.0.0",
		},
		Source: internal.SourceConfig{
			Type: internal.HelmRepositorySource,
			URL:  "https://charts.bitnami.com/bitnami",
		},
		Values: map[string]interface{}{
			"auth": map[string]interface{}{
				"database": "myapp",
			},
		},
		Release: internal.ReleaseConfig{
			CreateNamespace: true,
		},
		Interval:       "15m",
		Timeout:        "5m",
		MaxHistory:     10,
		ServiceAccount: "postgresql-sa",
		Suspend:        false,
	}

	app := stack.NewApplication("postgresql", "database", cfg)
	objs, err := cfg.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(objs) != 2 {
		t.Errorf("Generate() returned %d objects, want 2", len(objs))
	}

	// Verify HelmRepository and HelmRelease
	var helmRepo *sourcev1.HelmRepository
	var helmRelease *helmv2.HelmRelease

	for _, obj := range objs {
		switch v := (*obj).(type) {
		case *sourcev1.HelmRepository:
			helmRepo = v
		case *helmv2.HelmRelease:
			helmRelease = v
		}
	}

	// Test HelmRepository
	if helmRepo == nil {
		t.Error("Expected HelmRepository object")
	} else {
		if helmRepo.Name != "postgresql-source" {
			t.Errorf("HelmRepository name = %s, want postgresql-source", helmRepo.Name)
		}
		if helmRepo.Spec.URL != "https://charts.bitnami.com/bitnami" {
			t.Errorf("HelmRepository URL = %s, want https://charts.bitnami.com/bitnami", helmRepo.Spec.URL)
		}
	}

	// Test HelmRelease
	if helmRelease == nil {
		t.Error("Expected HelmRelease object")
	} else {
		if helmRelease.Name != "postgresql" {
			t.Errorf("HelmRelease name = %s, want postgresql", helmRelease.Name)
		}
		if helmRelease.Spec.Chart.Spec.Chart != "postgresql" {
			t.Errorf("HelmRelease chart = %s, want postgresql", helmRelease.Spec.Chart.Spec.Chart)
		}
		if helmRelease.Spec.Chart.Spec.Version != "12.0.0" {
			t.Errorf("HelmRelease version = %s, want 12.0.0", helmRelease.Spec.Chart.Spec.Version)
		}
		if helmRelease.Spec.Chart.Spec.SourceRef.Kind != "HelmRepository" {
			t.Errorf("HelmRelease sourceRef kind = %s, want HelmRepository", helmRelease.Spec.Chart.Spec.SourceRef.Kind)
		}
		if helmRelease.Spec.ServiceAccountName != "postgresql-sa" {
			t.Errorf("HelmRelease serviceAccount = %s, want postgresql-sa", helmRelease.Spec.ServiceAccountName)
		}
		if *helmRelease.Spec.MaxHistory != 10 {
			t.Errorf("HelmRelease maxHistory = %d, want 10", *helmRelease.Spec.MaxHistory)
		}
		if helmRelease.Spec.Install.CreateNamespace != true {
			t.Error("HelmRelease createNamespace should be true")
		}
	}
}

func TestConfigV1Alpha1_Generate_GitRepository(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "app-chart",
			Namespace: "apps",
		},
		Chart: internal.ChartConfig{
			Name: "app-chart",
		},
		Source: internal.SourceConfig{
			Type:    internal.GitRepositorySource,
			GitURL:  "https://github.com/example/charts.git",
			GitRef:  "main",
			GitPath: "charts/app",
		},
	}

	app := stack.NewApplication("app-chart", "apps", cfg)
	objs, err := cfg.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(objs) != 2 {
		t.Errorf("Generate() returned %d objects, want 2", len(objs))
	}

	var gitRepo *sourcev1.GitRepository
	var helmRelease *helmv2.HelmRelease

	for _, obj := range objs {
		switch v := (*obj).(type) {
		case *sourcev1.GitRepository:
			gitRepo = v
		case *helmv2.HelmRelease:
			helmRelease = v
		}
	}

	// Test GitRepository
	if gitRepo == nil {
		t.Error("Expected GitRepository object")
	} else {
		if gitRepo.Name != "app-chart-source" {
			t.Errorf("GitRepository name = %s, want app-chart-source", gitRepo.Name)
		}
		if gitRepo.Spec.URL != "https://github.com/example/charts.git" {
			t.Errorf("GitRepository URL = %s, want https://github.com/example/charts.git", gitRepo.Spec.URL)
		}
		if gitRepo.Spec.Reference.Branch != "main" {
			t.Errorf("GitRepository branch = %s, want main", gitRepo.Spec.Reference.Branch)
		}
	}

	// Test HelmRelease source reference
	if helmRelease != nil && helmRelease.Spec.Chart.Spec.SourceRef.Kind != "GitRepository" {
		t.Errorf("HelmRelease sourceRef kind = %s, want GitRepository", helmRelease.Spec.Chart.Spec.SourceRef.Kind)
	}
}

func TestConfigV1Alpha1_Generate_OCIRepository(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "podinfo",
			Namespace: "apps",
		},
		Chart: internal.ChartConfig{
			Name:    "podinfo",
			Version: "6.*",
		},
		Source: internal.SourceConfig{
			Type:   internal.OCIRepositorySource,
			OCIUrl: "oci://ghcr.io/stefanprodan/charts/podinfo",
		},
		Values: map[string]interface{}{
			"replicaCount": 2,
		},
	}

	app := stack.NewApplication("podinfo", "apps", cfg)
	objs, err := cfg.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(objs) != 2 {
		t.Errorf("Generate() returned %d objects, want 2", len(objs))
	}

	var ociRepo *sourcev1.OCIRepository
	var helmRelease *helmv2.HelmRelease

	for _, obj := range objs {
		switch v := (*obj).(type) {
		case *sourcev1.OCIRepository:
			ociRepo = v
		case *helmv2.HelmRelease:
			helmRelease = v
		}
	}

	// Test OCIRepository
	if ociRepo == nil {
		t.Error("Expected OCIRepository object")
	} else {
		if ociRepo.Name != "podinfo-source" {
			t.Errorf("OCIRepository name = %s, want podinfo-source", ociRepo.Name)
		}
		if ociRepo.Spec.URL != "oci://ghcr.io/stefanprodan/charts/podinfo" {
			t.Errorf("OCIRepository URL = %s, want oci://ghcr.io/stefanprodan/charts/podinfo", ociRepo.Spec.URL)
		}
	}

	// Test HelmRelease source reference
	if helmRelease != nil && helmRelease.Spec.Chart.Spec.SourceRef.Kind != "OCIRepository" {
		t.Errorf("HelmRelease sourceRef kind = %s, want OCIRepository", helmRelease.Spec.Chart.Spec.SourceRef.Kind)
	}
}

func TestConfigV1Alpha1_Generate_Bucket(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "charts",
			Namespace: "flux-system",
		},
		Chart: internal.ChartConfig{
			Name:    "my-chart",
			Version: "1.0.0",
		},
		Source: internal.SourceConfig{
			Type:       internal.BucketSource,
			BucketName: "helm-charts",
			Endpoint:   "s3.amazonaws.com",
			Region:     "us-west-2",
			SecretRef:  "s3-credentials",
		},
	}

	app := stack.NewApplication("charts", "flux-system", cfg)
	objs, err := cfg.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(objs) != 2 {
		t.Errorf("Generate() returned %d objects, want 2", len(objs))
	}

	var bucket *sourcev1.Bucket
	var helmRelease *helmv2.HelmRelease

	for _, obj := range objs {
		switch v := (*obj).(type) {
		case *sourcev1.Bucket:
			bucket = v
		case *helmv2.HelmRelease:
			helmRelease = v
		}
	}

	// Test Bucket
	if bucket == nil {
		t.Error("Expected Bucket object")
	} else {
		if bucket.Name != "charts-source" {
			t.Errorf("Bucket name = %s, want charts-source", bucket.Name)
		}
		if bucket.Spec.BucketName != "helm-charts" {
			t.Errorf("Bucket bucketName = %s, want helm-charts", bucket.Spec.BucketName)
		}
		if bucket.Spec.Endpoint != "s3.amazonaws.com" {
			t.Errorf("Bucket endpoint = %s, want s3.amazonaws.com", bucket.Spec.Endpoint)
		}
		if bucket.Spec.Region != "us-west-2" {
			t.Errorf("Bucket region = %s, want us-west-2", bucket.Spec.Region)
		}
		if bucket.Spec.SecretRef.Name != "s3-credentials" {
			t.Errorf("Bucket secretRef = %s, want s3-credentials", bucket.Spec.SecretRef.Name)
		}
	}

	// Test HelmRelease source reference
	if helmRelease != nil && helmRelease.Spec.Chart.Spec.SourceRef.Kind != "Bucket" {
		t.Errorf("HelmRelease sourceRef kind = %s, want Bucket", helmRelease.Spec.Chart.Spec.SourceRef.Kind)
	}
}

func TestConfigV1Alpha1_Generate_WithDependencies(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "app",
			Namespace: "default",
		},
		Chart: internal.ChartConfig{
			Name: "app-chart",
		},
		Source: internal.SourceConfig{
			Type: internal.HelmRepositorySource,
			URL:  "https://charts.example.com",
		},
		DependsOn: []string{"postgres", "redis"},
	}

	app := stack.NewApplication("app", "default", cfg)
	objs, err := cfg.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var helmRelease *helmv2.HelmRelease
	for _, obj := range objs {
		if v, ok := (*obj).(*helmv2.HelmRelease); ok {
			helmRelease = v
			break
		}
	}

	if helmRelease == nil {
		t.Fatal("Expected HelmRelease object")
	}

	if len(helmRelease.Spec.DependsOn) != 2 {
		t.Errorf("HelmRelease dependsOn count = %d, want 2", len(helmRelease.Spec.DependsOn))
	}

	expectedDeps := []string{"postgres", "redis"}
	for i, dep := range helmRelease.Spec.DependsOn {
		if dep.Name != expectedDeps[i] {
			t.Errorf("HelmRelease dependency[%d] = %s, want %s", i, dep.Name, expectedDeps[i])
		}
	}
}

func TestConfigV1Alpha1_Generate_WithPostRenderers(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "app",
			Namespace: "default",
		},
		Chart: internal.ChartConfig{
			Name: "app-chart",
		},
		Source: internal.SourceConfig{
			Type: internal.HelmRepositorySource,
			URL:  "https://charts.example.com",
		},
		PostRenderers: []internal.PostRenderer{
			{
				Kustomize: &internal.KustomizePostRenderer{
					Patches: []internal.KustomizePatch{
						{
							Patch: `
- op: replace
  path: /spec/replicas
  value: 3
`,
						},
					},
					Images: []internal.KustomizeImage{
						{
							Name:   "app",
							NewTag: "v1.2.3",
						},
					},
				},
			},
		},
	}

	app := stack.NewApplication("app", "default", cfg)
	objs, err := cfg.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var helmRelease *helmv2.HelmRelease
	for _, obj := range objs {
		if v, ok := (*obj).(*helmv2.HelmRelease); ok {
			helmRelease = v
			break
		}
	}

	if helmRelease == nil {
		t.Fatal("Expected HelmRelease object")
	}

	if len(helmRelease.Spec.PostRenderers) != 1 {
		t.Errorf("HelmRelease postRenderers count = %d, want 1", len(helmRelease.Spec.PostRenderers))
	}

	kustomize := helmRelease.Spec.PostRenderers[0].Kustomize
	if kustomize == nil {
		t.Fatal("Expected Kustomize post renderer")
	}

	if len(kustomize.Patches) != 1 {
		t.Errorf("Kustomize patches count = %d, want 1", len(kustomize.Patches))
	}

	if len(kustomize.Images) != 1 {
		t.Errorf("Kustomize images count = %d, want 1", len(kustomize.Images))
	}

	if kustomize.Images[0].Name != "app" {
		t.Errorf("Kustomize image name = %s, want app", kustomize.Images[0].Name)
	}

	if kustomize.Images[0].NewTag != "v1.2.3" {
		t.Errorf("Kustomize image newTag = %s, want v1.2.3", kustomize.Images[0].NewTag)
	}
}

func TestConfigV1Alpha1_Generate_WithAdvancedReleaseOptions(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "advanced-app",
			Namespace: "default",
		},
		Chart: internal.ChartConfig{
			Name:    "app",
			Version: "1.0.0",
		},
		Source: internal.SourceConfig{
			Type: internal.HelmRepositorySource,
			URL:  "https://charts.example.com",
		},
		Release: internal.ReleaseConfig{
			CreateNamespace:          true,
			DisableWait:              true,
			DisableWaitForJobs:       true,
			DisableHooks:             true,
			DisableOpenAPIValidation: true,
			ResetValues:              true,
			ForceUpgrade:             true,
			PreserveValues:           true,
			CleanupOnFail:            true,
			Replace:                  true,
		},
		Suspend: true,
	}

	app := stack.NewApplication("advanced-app", "default", cfg)
	objs, err := cfg.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var helmRelease *helmv2.HelmRelease
	for _, obj := range objs {
		if v, ok := (*obj).(*helmv2.HelmRelease); ok {
			helmRelease = v
			break
		}
	}

	if helmRelease == nil {
		t.Fatal("Expected HelmRelease object")
	}

	// Test suspend
	if !helmRelease.Spec.Suspend {
		t.Error("HelmRelease suspend should be true")
	}

	// Test install options
	if !helmRelease.Spec.Install.CreateNamespace {
		t.Error("HelmRelease install.createNamespace should be true")
	}
	if !helmRelease.Spec.Install.DisableWait {
		t.Error("HelmRelease install.disableWait should be true")
	}
	if !helmRelease.Spec.Install.Replace {
		t.Error("HelmRelease install.replace should be true")
	}

	// Test upgrade options
	if !helmRelease.Spec.Upgrade.Force {
		t.Error("HelmRelease upgrade.force should be true")
	}
	if !helmRelease.Spec.Upgrade.PreserveValues {
		t.Error("HelmRelease upgrade.preserveValues should be true")
	}
	if !helmRelease.Spec.Upgrade.CleanupOnFail {
		t.Error("HelmRelease upgrade.cleanupOnFail should be true")
	}
}

func TestConfigV1Alpha1_Generate_WithTargetNamespaceAndReleaseName(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "cert-manager",
			Namespace: "flux-system",
		},
		TargetNamespace: "cert-manager",
		ReleaseName:     "cert-manager",
		Chart: internal.ChartConfig{
			Name:    "cert-manager",
			Version: "1.16.0",
		},
		Source: internal.SourceConfig{
			Type: internal.HelmRepositorySource,
			URL:  "https://charts.jetstack.io",
		},
	}

	app := stack.NewApplication("cert-manager", "flux-system", cfg)
	objs, err := cfg.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	helmRelease := findHelmRelease(objs)
	if helmRelease == nil {
		t.Fatal("Expected HelmRelease object")
	}

	if helmRelease.Spec.TargetNamespace != "cert-manager" {
		t.Errorf("TargetNamespace = %q, want %q", helmRelease.Spec.TargetNamespace, "cert-manager")
	}
	if helmRelease.Spec.ReleaseName != "cert-manager" {
		t.Errorf("ReleaseName = %q, want %q", helmRelease.Spec.ReleaseName, "cert-manager")
	}

	// Verify the HelmRelease itself is in flux-system namespace
	if helmRelease.Namespace != "flux-system" {
		t.Errorf("Namespace = %q, want %q", helmRelease.Namespace, "flux-system")
	}
}

func TestConfigV1Alpha1_Generate_WithValuesFrom(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "app",
			Namespace: "default",
		},
		Chart: internal.ChartConfig{
			Name: "app-chart",
		},
		Source: internal.SourceConfig{
			Type: internal.HelmRepositorySource,
			URL:  "https://charts.example.com",
		},
		ValuesFrom: []internal.ValuesReference{
			{Kind: "ConfigMap", Name: "common-values"},
			{Kind: "Secret", Name: "secret-values", ValuesKey: "password", TargetPath: "auth.password"},
		},
	}

	app := stack.NewApplication("app", "default", cfg)
	objs, err := cfg.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	helmRelease := findHelmRelease(objs)
	if helmRelease == nil {
		t.Fatal("Expected HelmRelease object")
	}

	if len(helmRelease.Spec.ValuesFrom) != 2 {
		t.Fatalf("ValuesFrom length = %d, want 2", len(helmRelease.Spec.ValuesFrom))
	}
	if helmRelease.Spec.ValuesFrom[0].Kind != "ConfigMap" {
		t.Errorf("ValuesFrom[0].Kind = %q, want %q", helmRelease.Spec.ValuesFrom[0].Kind, "ConfigMap")
	}
	if helmRelease.Spec.ValuesFrom[1].TargetPath != "auth.password" {
		t.Errorf("ValuesFrom[1].TargetPath = %q, want %q", helmRelease.Spec.ValuesFrom[1].TargetPath, "auth.password")
	}
}

func TestConfigV1Alpha1_Generate_WithCRDsPolicy(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "cert-manager",
			Namespace: "flux-system",
		},
		Chart: internal.ChartConfig{
			Name:    "cert-manager",
			Version: "1.16.0",
		},
		Source: internal.SourceConfig{
			Type: internal.HelmRepositorySource,
			URL:  "https://charts.jetstack.io",
		},
		Release: internal.ReleaseConfig{
			CreateNamespace: true,
			InstallCRDs:     internal.CRDsPolicyCreateReplace,
			UpgradeCRDs:     internal.CRDsPolicyCreateReplace,
		},
	}

	app := stack.NewApplication("cert-manager", "flux-system", cfg)
	objs, err := cfg.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	helmRelease := findHelmRelease(objs)
	if helmRelease == nil {
		t.Fatal("Expected HelmRelease object")
	}

	if helmRelease.Spec.Install.CRDs != "CreateReplace" {
		t.Errorf("Install.CRDs = %q, want %q", helmRelease.Spec.Install.CRDs, "CreateReplace")
	}
	if helmRelease.Spec.Upgrade.CRDs != "CreateReplace" {
		t.Errorf("Upgrade.CRDs = %q, want %q", helmRelease.Spec.Upgrade.CRDs, "CreateReplace")
	}
}

func TestConfigV1Alpha1_Generate_NoExplicitSource(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "no-source",
			Namespace: "default",
		},
		Chart: internal.ChartConfig{
			Name: "chart",
		},
		Source: internal.SourceConfig{
			// No explicit source type or URLs
		},
	}

	app := stack.NewApplication("no-source", "default", cfg)
	objs, err := cfg.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Should only generate HelmRelease when no source is specified
	if len(objs) != 1 {
		t.Errorf("Generate() returned %d objects, want 1", len(objs))
	}

	_, ok := (*objs[0]).(*helmv2.HelmRelease)
	if !ok {
		t.Errorf("Expected HelmRelease object, got %T", *objs[0])
	}
}

func TestConfigV1Alpha1_Generate_InferSourceFromURL(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "inferred",
			Namespace: "default",
		},
		Chart: internal.ChartConfig{
			Name: "chart",
		},
		Source: internal.SourceConfig{
			URL: "https://charts.example.com", // Should infer HelmRepository
		},
	}

	app := stack.NewApplication("inferred", "default", cfg)
	objs, err := cfg.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(objs) != 2 {
		t.Errorf("Generate() returned %d objects, want 2", len(objs))
	}

	var helmRepo *sourcev1.HelmRepository
	for _, obj := range objs {
		if v, ok := (*obj).(*sourcev1.HelmRepository); ok {
			helmRepo = v
			break
		}
	}

	if helmRepo == nil {
		t.Error("Expected HelmRepository to be inferred from URL")
	}
}

func TestConfigV1Alpha1_Generate_InferSourceFromOCIUrl(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "inferred-oci",
			Namespace: "default",
		},
		Chart: internal.ChartConfig{
			Name: "chart",
		},
		Source: internal.SourceConfig{
			OCIUrl: "oci://registry.example.com/chart", // Should infer OCIRepository
		},
	}

	app := stack.NewApplication("inferred-oci", "default", cfg)
	objs, err := cfg.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(objs) != 2 {
		t.Errorf("Generate() returned %d objects, want 2", len(objs))
	}

	var ociRepo *sourcev1.OCIRepository
	for _, obj := range objs {
		if v, ok := (*obj).(*sourcev1.OCIRepository); ok {
			ociRepo = v
			break
		}
	}

	if ociRepo == nil {
		t.Error("Expected OCIRepository to be inferred from OCIUrl")
	}
}

func TestRegistration(t *testing.T) {
	// Test that the FluxHelm generator is properly registered in the stack registry
	config, err := stack.CreateApplicationConfig("generators.gokure.dev/v1alpha1", "FluxHelm")
	if err != nil {
		t.Fatalf("FluxHelm generator not registered in stack package: %v", err)
	}

	if config == nil {
		t.Fatal("CreateApplicationConfig returned nil config")
	}

	fluxHelmConfig, ok := config.(*ConfigV1Alpha1)
	if !ok {
		t.Fatalf("CreateApplicationConfig returned wrong type: %T, want *ConfigV1Alpha1", config)
	}

	if fluxHelmConfig.GetAPIVersion() != "generators.gokure.dev/v1alpha1" {
		t.Errorf("Config APIVersion = %s, want generators.gokure.dev/v1alpha1", fluxHelmConfig.GetAPIVersion())
	}

	if fluxHelmConfig.GetKind() != "FluxHelm" {
		t.Errorf("Config Kind = %s, want FluxHelm", fluxHelmConfig.GetKind())
	}
}

func TestConfigV1Alpha1_BaseMetadata(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "test-base",
			Namespace: "test-namespace",
		},
		Chart: internal.ChartConfig{
			Name: "test-chart",
		},
		Source: internal.SourceConfig{
			Type: internal.HelmRepositorySource,
			URL:  "https://example.com",
		},
	}

	// Verify BaseMetadata fields are accessible
	if cfg.Name != "test-base" {
		t.Errorf("Name = %s, want test-base", cfg.Name)
	}

	if cfg.Namespace != "test-namespace" {
		t.Errorf("Namespace = %s, want test-namespace", cfg.Namespace)
	}
}

// Helper function to find specific object types in the result
func findHelmRelease(objs []*client.Object) *helmv2.HelmRelease {
	for _, obj := range objs {
		if hr, ok := (*obj).(*helmv2.HelmRelease); ok {
			return hr
		}
	}
	return nil
}

func TestConfigV1Alpha1_Generate_EmptyValues(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "empty-values",
			Namespace: "default",
		},
		Chart: internal.ChartConfig{
			Name: "chart",
		},
		Source: internal.SourceConfig{
			Type: internal.HelmRepositorySource,
			URL:  "https://charts.example.com",
		},
		Values: nil, // Explicitly nil values
	}

	app := stack.NewApplication("empty-values", "default", cfg)
	objs, err := cfg.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	helmRelease := findHelmRelease(objs)
	if helmRelease == nil {
		t.Fatal("Expected HelmRelease object")
	}

	// Values should be nil when not specified
	if helmRelease.Spec.Values != nil {
		t.Error("HelmRelease values should be nil when not specified")
	}
}

func TestConfigV1Alpha1_Generate_WithChartRef(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "podinfo",
			Namespace: "apps",
		},
		ChartRef: &internal.ChartRefConfig{
			Kind: "OCIRepository",
			Name: "podinfo-oci",
		},
		Source: internal.SourceConfig{
			Type:   internal.OCIRepositorySource,
			OCIUrl: "oci://ghcr.io/stefanprodan/charts/podinfo",
		},
		Values: map[string]interface{}{
			"replicaCount": 2,
		},
	}

	app := stack.NewApplication("podinfo", "apps", cfg)
	objs, err := cfg.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Should have 2 objects: OCIRepository source + HelmRelease
	if len(objs) != 2 {
		t.Errorf("Generate() returned %d objects, want 2", len(objs))
	}

	helmRelease := findHelmRelease(objs)
	if helmRelease == nil {
		t.Fatal("Expected HelmRelease object")
	}

	// ChartRef should be set
	if helmRelease.Spec.ChartRef == nil {
		t.Fatal("HelmRelease should have ChartRef set")
	}
	if helmRelease.Spec.ChartRef.Kind != "OCIRepository" {
		t.Errorf("ChartRef.Kind = %s, want OCIRepository", helmRelease.Spec.ChartRef.Kind)
	}
	if helmRelease.Spec.ChartRef.Name != "podinfo-oci" {
		t.Errorf("ChartRef.Name = %s, want podinfo-oci", helmRelease.Spec.ChartRef.Name)
	}

	// Chart should be nil (mutually exclusive)
	if helmRelease.Spec.Chart != nil {
		t.Error("HelmRelease should not have Chart set when using ChartRef")
	}

	// Values should still be set
	if helmRelease.Spec.Values == nil {
		t.Error("HelmRelease values should be set")
	}
}

func TestConfigV1Alpha1_Generate_WithChartRefCrossNamespace(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "shared-app",
			Namespace: "apps",
		},
		ChartRef: &internal.ChartRefConfig{
			Kind:      "HelmChart",
			Name:      "shared-chart",
			Namespace: "flux-system",
		},
		Source: internal.SourceConfig{
			Type: internal.HelmRepositorySource,
			URL:  "https://charts.example.com",
		},
	}

	app := stack.NewApplication("shared-app", "apps", cfg)
	objs, err := cfg.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	helmRelease := findHelmRelease(objs)
	if helmRelease == nil {
		t.Fatal("Expected HelmRelease object")
	}

	if helmRelease.Spec.ChartRef == nil {
		t.Fatal("HelmRelease should have ChartRef set")
	}
	if helmRelease.Spec.ChartRef.Namespace != "flux-system" {
		t.Errorf("ChartRef.Namespace = %s, want flux-system", helmRelease.Spec.ChartRef.Namespace)
	}
}

func TestSourceTypeValidation(t *testing.T) {
	tests := []struct {
		name       string
		sourceType internal.SourceType
		expectObj  bool
	}{
		{"HelmRepository", internal.HelmRepositorySource, true},
		{"GitRepository", internal.GitRepositorySource, true},
		{"OCIRepository", internal.OCIRepositorySource, true},
		{"Bucket", internal.BucketSource, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ConfigV1Alpha1{
				BaseMetadata: generators.BaseMetadata{
					Name:      "test",
					Namespace: "default",
				},
				Chart: internal.ChartConfig{
					Name: "test-chart",
				},
				Source: internal.SourceConfig{
					Type:       tt.sourceType,
					URL:        "https://example.com",
					GitURL:     "https://github.com/example/repo.git",
					OCIUrl:     "oci://registry.example.com/chart",
					BucketName: "bucket",
					Endpoint:   "s3.example.com",
				},
			}

			app := stack.NewApplication("test", "default", cfg)
			objs, err := cfg.Generate(app)
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}

			expectedCount := 1 // Always HelmRelease
			if tt.expectObj {
				expectedCount = 2 // HelmRelease + Source
			}

			if len(objs) != expectedCount {
				t.Errorf("Generate() returned %d objects, want %d", len(objs), expectedCount)
			}
		})
	}
}
