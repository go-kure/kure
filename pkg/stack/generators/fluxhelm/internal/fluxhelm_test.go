package internal

import (
	"encoding/json"
	"testing"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"gopkg.in/yaml.v3"
)

func TestGenerateResources(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		wantObjects   int
		wantSourceErr bool
		wantReleaseErr bool
	}{
		{
			name: "HelmRepository source",
			config: &Config{
				Name:      "test-release",
				Namespace: "test-namespace",
				Chart: ChartConfig{
					Name:    "nginx",
					Version: "1.0.0",
				},
				Source: SourceConfig{
					Type: HelmRepositorySource,
					URL:  "https://charts.bitnami.com/bitnami",
				},
			},
			wantObjects: 2, // HelmRepository + HelmRelease
		},
		{
			name: "GitRepository source",
			config: &Config{
				Name:      "test-release",
				Namespace: "test-namespace",
				Chart: ChartConfig{
					Name:    "nginx",
					Version: "1.0.0",
				},
				Source: SourceConfig{
					Type:   GitRepositorySource,
					GitURL: "https://github.com/example/charts",
					GitRef: "main",
				},
			},
			wantObjects: 2, // GitRepository + HelmRelease
		},
		{
			name: "OCIRepository source",
			config: &Config{
				Name:      "test-release",
				Namespace: "test-namespace",
				Chart: ChartConfig{
					Name:    "nginx",
					Version: "1.0.0",
				},
				Source: SourceConfig{
					Type:   OCIRepositorySource,
					OCIUrl: "oci://ghcr.io/example/charts",
				},
			},
			wantObjects: 2, // OCIRepository + HelmRelease
		},
		{
			name: "Bucket source",
			config: &Config{
				Name:      "test-release",
				Namespace: "test-namespace",
				Chart: ChartConfig{
					Name:    "nginx",
					Version: "1.0.0",
				},
				Source: SourceConfig{
					Type:       BucketSource,
					BucketName: "my-bucket",
					Endpoint:   "s3.amazonaws.com",
					Region:     "us-west-2",
				},
			},
			wantObjects: 2, // Bucket + HelmRelease
		},
		{
			name: "No source (existing source)",
			config: &Config{
				Name:      "test-release",
				Namespace: "test-namespace",
				Chart: ChartConfig{
					Name:    "nginx",
					Version: "1.0.0",
				},
				Source: SourceConfig{
					Type: "", // Empty type, no source will be created
				},
			},
			wantObjects: 1, // Only HelmRelease
		},
		{
			name: "Invalid interval",
			config: &Config{
				Name:      "test-release",
				Namespace: "test-namespace",
				Chart: ChartConfig{
					Name:    "nginx",
					Version: "1.0.0",
				},
				Source: SourceConfig{
					Type:     HelmRepositorySource,
					URL:      "https://charts.bitnami.com/bitnami",
					Interval: "invalid",
				},
			},
			wantSourceErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects, err := GenerateResources(tt.config)

			if tt.wantSourceErr || tt.wantReleaseErr {
				if err == nil {
					t.Errorf("GenerateResources() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("GenerateResources() error = %v", err)
				return
			}

			if len(objects) != tt.wantObjects {
				t.Errorf("GenerateResources() got %d objects, want %d", len(objects), tt.wantObjects)
			}

			// Verify last object is always HelmRelease
			if len(objects) > 0 {
				lastObj := *objects[len(objects)-1]
				if hr, ok := lastObj.(*helmv2.HelmRelease); ok {
					if hr.Name != tt.config.Name {
						t.Errorf("HelmRelease name = %s, want %s", hr.Name, tt.config.Name)
					}
					if hr.Namespace != tt.config.Namespace {
						t.Errorf("HelmRelease namespace = %s, want %s", hr.Namespace, tt.config.Namespace)
					}
				} else {
					t.Errorf("Last object is not a HelmRelease: %T", lastObj)
				}
			}
		})
	}
}

func TestGenerateHelmRepository(t *testing.T) {
	config := &Config{
		Name:      "test-release",
		Namespace: "test-namespace",
		Source: SourceConfig{
			Type:      HelmRepositorySource,
			URL:       "https://charts.bitnami.com/bitnami",
			Interval:  "5m",
			SecretRef: "my-secret",
		},
	}

	obj, err := config.generateHelmRepository()
	if err != nil {
		t.Fatalf("generateHelmRepository() error = %v", err)
	}

	if obj == nil {
		t.Fatal("generateHelmRepository() returned nil object")
	}

	repo, ok := (*obj).(*sourcev1.HelmRepository)
	if !ok {
		t.Fatalf("Expected HelmRepository, got %T", *obj)
	}

	if repo.Name != "test-release-source" {
		t.Errorf("Name = %s, want test-release-source", repo.Name)
	}
	if repo.Namespace != "test-namespace" {
		t.Errorf("Namespace = %s, want test-namespace", repo.Namespace)
	}
	if repo.Spec.URL != "https://charts.bitnami.com/bitnami" {
		t.Errorf("URL = %s, want https://charts.bitnami.com/bitnami", repo.Spec.URL)
	}
	if repo.Spec.Interval.Duration != 5*time.Minute {
		t.Errorf("Interval = %v, want 5m", repo.Spec.Interval.Duration)
	}
	if repo.Spec.SecretRef == nil || repo.Spec.SecretRef.Name != "my-secret" {
		t.Errorf("SecretRef = %v, want my-secret", repo.Spec.SecretRef)
	}
}

func TestGenerateGitRepository(t *testing.T) {
	config := &Config{
		Name:      "test-release",
		Namespace: "test-namespace",
		Source: SourceConfig{
			Type:      GitRepositorySource,
			GitURL:    "https://github.com/example/charts",
			GitRef:    "main",
			Interval:  "10m",
			SecretRef: "git-secret",
		},
	}

	obj, err := config.generateGitRepository()
	if err != nil {
		t.Fatalf("generateGitRepository() error = %v", err)
	}

	if obj == nil {
		t.Fatal("generateGitRepository() returned nil object")
	}

	repo, ok := (*obj).(*sourcev1.GitRepository)
	if !ok {
		t.Fatalf("Expected GitRepository, got %T", *obj)
	}

	if repo.Name != "test-release-source" {
		t.Errorf("Name = %s, want test-release-source", repo.Name)
	}
	if repo.Spec.URL != "https://github.com/example/charts" {
		t.Errorf("URL = %s, want https://github.com/example/charts", repo.Spec.URL)
	}
	if repo.Spec.Reference == nil || repo.Spec.Reference.Branch != "main" {
		t.Errorf("Branch = %v, want main", repo.Spec.Reference)
	}
	if repo.Spec.SecretRef == nil || repo.Spec.SecretRef.Name != "git-secret" {
		t.Errorf("SecretRef = %v, want git-secret", repo.Spec.SecretRef)
	}
}

func TestGenerateOCIRepository(t *testing.T) {
	config := &Config{
		Name:      "test-release",
		Namespace: "test-namespace",
		Source: SourceConfig{
			Type:      OCIRepositorySource,
			OCIUrl:    "oci://ghcr.io/example/charts",
			Interval:  "15m",
			SecretRef: "oci-secret",
		},
	}

	obj, err := config.generateOCIRepository()
	if err != nil {
		t.Fatalf("generateOCIRepository() error = %v", err)
	}

	repo, ok := (*obj).(*sourcev1.OCIRepository)
	if !ok {
		t.Fatalf("Expected OCIRepository, got %T", *obj)
	}

	if repo.Name != "test-release-source" {
		t.Errorf("Name = %s, want test-release-source", repo.Name)
	}
	if repo.Spec.URL != "oci://ghcr.io/example/charts" {
		t.Errorf("URL = %s, want oci://ghcr.io/example/charts", repo.Spec.URL)
	}
	if repo.Spec.Interval.Duration != 15*time.Minute {
		t.Errorf("Interval = %v, want 15m", repo.Spec.Interval.Duration)
	}
}

func TestGenerateBucket(t *testing.T) {
	config := &Config{
		Name:      "test-release",
		Namespace: "test-namespace",
		Source: SourceConfig{
			Type:       BucketSource,
			BucketName: "my-bucket",
			Endpoint:   "s3.amazonaws.com",
			Region:     "us-west-2",
			Interval:   "20m",
			SecretRef:  "s3-secret",
		},
	}

	obj, err := config.generateBucket()
	if err != nil {
		t.Fatalf("generateBucket() error = %v", err)
	}

	bucket, ok := (*obj).(*sourcev1.Bucket)
	if !ok {
		t.Fatalf("Expected Bucket, got %T", *obj)
	}

	if bucket.Name != "test-release-source" {
		t.Errorf("Name = %s, want test-release-source", bucket.Name)
	}
	if bucket.Spec.BucketName != "my-bucket" {
		t.Errorf("BucketName = %s, want my-bucket", bucket.Spec.BucketName)
	}
	if bucket.Spec.Endpoint != "s3.amazonaws.com" {
		t.Errorf("Endpoint = %s, want s3.amazonaws.com", bucket.Spec.Endpoint)
	}
	if bucket.Spec.Region != "us-west-2" {
		t.Errorf("Region = %s, want us-west-2", bucket.Spec.Region)
	}
}

func TestGenerateHelmRelease(t *testing.T) {
	values := map[string]interface{}{
		"replicaCount": 3,
		"image": map[string]interface{}{
			"repository": "nginx",
			"tag":        "1.21.0",
		},
	}

	config := &Config{
		Name:      "test-release",
		Namespace: "test-namespace",
		Chart: ChartConfig{
			Name:    "nginx",
			Version: "1.0.0",
		},
		Values:         values,
		Interval:       "5m",
		Timeout:        "10m",
		MaxHistory:     5,
		ServiceAccount: "helm-sa",
		Suspend:        false,
		DependsOn:      []string{"dependency1", "dependency2"},
		Source: SourceConfig{
			Type: HelmRepositorySource,
		},
		Release: ReleaseConfig{
			CreateNamespace:          true,
			DisableWait:              false,
			DisableWaitForJobs:       true,
			DisableHooks:             false,
			DisableOpenAPIValidation: true,
			ResetValues:              false,
			ForceUpgrade:             true,
			PreserveValues:           false,
			CleanupOnFail:            true,
			Replace:                  false,
		},
		PostRenderers: []PostRenderer{
			{
				Kustomize: &KustomizePostRenderer{
					Patches: []KustomizePatch{
						{
							Patch: "- op: replace\n  path: /spec/replicas\n  value: 5",
						},
					},
					Images: []KustomizeImage{
						{
							Name:    "nginx",
							NewName: "nginx-custom",
							NewTag:  "1.21.1",
						},
					},
				},
			},
		},
	}

	hr, err := config.generateHelmRelease()
	if err != nil {
		t.Fatalf("generateHelmRelease() error = %v", err)
	}

	release, ok := hr.(*helmv2.HelmRelease)
	if !ok {
		t.Fatalf("Expected HelmRelease, got %T", hr)
	}

	// Test basic properties
	if release.Name != "test-release" {
		t.Errorf("Name = %s, want test-release", release.Name)
	}
	if release.Namespace != "test-namespace" {
		t.Errorf("Namespace = %s, want test-namespace", release.Namespace)
	}
	if release.Spec.Interval.Duration != 5*time.Minute {
		t.Errorf("Interval = %v, want 5m", release.Spec.Interval.Duration)
	}

	// Test chart configuration
	if release.Spec.Chart.Spec.Chart != "nginx" {
		t.Errorf("Chart name = %s, want nginx", release.Spec.Chart.Spec.Chart)
	}
	if release.Spec.Chart.Spec.Version != "1.0.0" {
		t.Errorf("Chart version = %s, want 1.0.0", release.Spec.Chart.Spec.Version)
	}
	if release.Spec.Chart.Spec.SourceRef.Name != "test-release-source" {
		t.Errorf("SourceRef name = %s, want test-release-source", release.Spec.Chart.Spec.SourceRef.Name)
	}
	if release.Spec.Chart.Spec.SourceRef.Kind != "HelmRepository" {
		t.Errorf("SourceRef kind = %s, want HelmRepository", release.Spec.Chart.Spec.SourceRef.Kind)
	}

	// Test values - the values are stored as YAML in the Raw field
	if release.Spec.Values == nil {
		t.Fatal("Values is nil")
	}
	if len(release.Spec.Values.Raw) == 0 {
		t.Fatal("Values Raw data is empty")
	}
	// Just verify the values were set - detailed parsing isn't crucial for this test
	// since the values are YAML marshaled in the implementation

	// Test timeout
	if release.Spec.Timeout == nil || release.Spec.Timeout.Duration != 10*time.Minute {
		t.Errorf("Timeout = %v, want 10m", release.Spec.Timeout)
	}

	// Test max history
	if release.Spec.MaxHistory == nil || *release.Spec.MaxHistory != 5 {
		t.Errorf("MaxHistory = %v, want 5", release.Spec.MaxHistory)
	}

	// Test service account
	if release.Spec.ServiceAccountName != "helm-sa" {
		t.Errorf("ServiceAccountName = %s, want helm-sa", release.Spec.ServiceAccountName)
	}

	// Test suspend
	if release.Spec.Suspend != false {
		t.Errorf("Suspend = %v, want false", release.Spec.Suspend)
	}

	// Test dependencies
	if len(release.Spec.DependsOn) != 2 {
		t.Fatalf("DependsOn length = %d, want 2", len(release.Spec.DependsOn))
	}
	if release.Spec.DependsOn[0].Name != "dependency1" {
		t.Errorf("First dependency = %s, want dependency1", release.Spec.DependsOn[0].Name)
	}

	// Test install configuration
	if release.Spec.Install == nil {
		t.Fatal("Install config is nil")
	}
	if release.Spec.Install.CreateNamespace != true {
		t.Errorf("CreateNamespace = %v, want true", release.Spec.Install.CreateNamespace)
	}
	if release.Spec.Install.DisableWaitForJobs != true {
		t.Errorf("DisableWaitForJobs = %v, want true", release.Spec.Install.DisableWaitForJobs)
	}

	// Test upgrade configuration
	if release.Spec.Upgrade == nil {
		t.Fatal("Upgrade config is nil")
	}
	if release.Spec.Upgrade.Force != true {
		t.Errorf("Force = %v, want true", release.Spec.Upgrade.Force)
	}
	if release.Spec.Upgrade.CleanupOnFail != true {
		t.Errorf("CleanupOnFail = %v, want true", release.Spec.Upgrade.CleanupOnFail)
	}

	// Test post-renderers
	if len(release.Spec.PostRenderers) != 1 {
		t.Fatalf("PostRenderers length = %d, want 1", len(release.Spec.PostRenderers))
	}
	if release.Spec.PostRenderers[0].Kustomize == nil {
		t.Fatal("Kustomize post-renderer is nil")
	}
	if len(release.Spec.PostRenderers[0].Kustomize.Patches) != 1 {
		t.Errorf("Patches length = %d, want 1", len(release.Spec.PostRenderers[0].Kustomize.Patches))
	}
	if len(release.Spec.PostRenderers[0].Kustomize.Images) != 1 {
		t.Errorf("Images length = %d, want 1", len(release.Spec.PostRenderers[0].Kustomize.Images))
	}
}

func TestGenerateHelmReleaseDefaults(t *testing.T) {
	config := &Config{
		Name:      "minimal-release",
		Namespace: "default",
		Chart: ChartConfig{
			Name: "nginx",
		},
		Source: SourceConfig{
			Type: GitRepositorySource,
		},
	}

	hr, err := config.generateHelmRelease()
	if err != nil {
		t.Fatalf("generateHelmRelease() error = %v", err)
	}

	release := hr.(*helmv2.HelmRelease)

	// Test defaults
	if release.Spec.Interval.Duration != 10*time.Minute {
		t.Errorf("Default interval = %v, want 10m", release.Spec.Interval.Duration)
	}
	if release.Spec.Chart.Spec.SourceRef.Kind != "GitRepository" {
		t.Errorf("SourceRef kind = %s, want GitRepository", release.Spec.Chart.Spec.SourceRef.Kind)
	}
	if release.Spec.Timeout != nil {
		t.Errorf("Timeout should be nil by default, got %v", release.Spec.Timeout)
	}
	if release.Spec.MaxHistory != nil {
		t.Errorf("MaxHistory should be nil by default, got %v", release.Spec.MaxHistory)
	}
	if release.Spec.ServiceAccountName != "" {
		t.Errorf("ServiceAccountName should be empty by default, got %s", release.Spec.ServiceAccountName)
	}
	if release.Spec.Suspend != false {
		t.Errorf("Suspend should be false by default, got %v", release.Spec.Suspend)
	}
}

func TestGenerateSourceInferred(t *testing.T) {
	tests := []struct {
		name       string
		config     *Config
		wantType   SourceType
		wantSource bool
	}{
		{
			name: "Infer HelmRepository from URL",
			config: &Config{
				Name:      "test",
				Namespace: "default",
				Source: SourceConfig{
					URL: "https://charts.bitnami.com/bitnami",
				},
			},
			wantType:   HelmRepositorySource,
			wantSource: true,
		},
		{
			name: "Infer OCIRepository from OCIUrl",
			config: &Config{
				Name:      "test",
				Namespace: "default",
				Source: SourceConfig{
					OCIUrl: "oci://ghcr.io/example/charts",
				},
			},
			wantType:   OCIRepositorySource,
			wantSource: true,
		},
		{
			name: "No source when no URL provided",
			config: &Config{
				Name:      "test",
				Namespace: "default",
				Source:    SourceConfig{},
			},
			wantSource: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, err := tt.config.generateSource()
			if err != nil {
				t.Fatalf("generateSource() error = %v", err)
			}

			if !tt.wantSource {
				if obj != nil {
					t.Errorf("Expected no source, got %T", *obj)
				}
				return
			}

			if obj == nil {
				t.Fatal("Expected source object, got nil")
			}

			switch tt.wantType {
			case HelmRepositorySource:
				if _, ok := (*obj).(*sourcev1.HelmRepository); !ok {
					t.Errorf("Expected HelmRepository, got %T", *obj)
				}
			case OCIRepositorySource:
				if _, ok := (*obj).(*sourcev1.OCIRepository); !ok {
					t.Errorf("Expected OCIRepository, got %T", *obj)
				}
			}
		})
	}
}

func TestInvalidIntervals(t *testing.T) {
	tests := []struct {
		name     string
		interval string
	}{
		{"Invalid format", "invalid"},
		{"Empty string with space", " "},
		{"Random string", "not-a-duration"},
	}

	config := &Config{
		Name:      "test",
		Namespace: "default",
		Source: SourceConfig{
			Type: HelmRepositorySource,
			URL:  "https://charts.bitnami.com/bitnami",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Source.Interval = tt.interval
			_, err := config.generateHelmRepository()
			if err == nil {
				t.Errorf("Expected error for invalid interval %s", tt.interval)
			}
		})
	}
}

func TestSourceTypeConstants(t *testing.T) {
	tests := []struct {
		constant SourceType
		expected string
	}{
		{HelmRepositorySource, "HelmRepository"},
		{GitRepositorySource, "GitRepository"},
		{BucketSource, "Bucket"},
		{OCIRepositorySource, "OCIRepository"},
	}

	for _, tt := range tests {
		if string(tt.constant) != tt.expected {
			t.Errorf("SourceType constant %v = %s, want %s", tt.constant, string(tt.constant), tt.expected)
		}
	}
}

func TestConfigStructTags(t *testing.T) {
	// Test that our config structs have proper YAML and JSON tags
	// This ensures they can be properly serialized/deserialized

	config := &Config{
		Name:      "test",
		Namespace: "default",
		Chart: ChartConfig{
			Name:    "nginx",
			Version: "1.0.0",
		},
		Values: map[string]interface{}{
			"test": "value",
		},
		Source: SourceConfig{
			Type: HelmRepositorySource,
			URL:  "https://example.com",
		},
		Release: ReleaseConfig{
			CreateNamespace: true,
		},
	}

	// Test JSON marshaling/unmarshaling
	jsonData, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config to JSON: %v", err)
	}

	var unmarshaledConfig Config
	if err := json.Unmarshal(jsonData, &unmarshaledConfig); err != nil {
		t.Fatalf("Failed to unmarshal config from JSON: %v", err)
	}

	if unmarshaledConfig.Name != config.Name {
		t.Errorf("Name mismatch after JSON round-trip: got %s, want %s", unmarshaledConfig.Name, config.Name)
	}
	if unmarshaledConfig.Chart.Name != config.Chart.Name {
		t.Errorf("Chart name mismatch after JSON round-trip: got %s, want %s", unmarshaledConfig.Chart.Name, config.Chart.Name)
	}
}

func TestEmptyValues(t *testing.T) {
	config := &Config{
		Name:      "test-release",
		Namespace: "default",
		Chart: ChartConfig{
			Name: "nginx",
		},
		Values: nil,
		Source: SourceConfig{
			Type: HelmRepositorySource,
		},
	}

	hr, err := config.generateHelmRelease()
	if err != nil {
		t.Fatalf("generateHelmRelease() with nil values error = %v", err)
	}

	release := hr.(*helmv2.HelmRelease)
	if release.Spec.Values != nil {
		t.Errorf("Values should be nil when no values provided, got %v", release.Spec.Values)
	}
}

func TestComplexValues(t *testing.T) {
	complexValues := map[string]interface{}{
		"global": map[string]interface{}{
			"imageRegistry": "my-registry.com",
		},
		"nginx": map[string]interface{}{
			"replicaCount": 3,
			"resources": map[string]interface{}{
				"limits": map[string]interface{}{
					"cpu":    "500m",
					"memory": "512Mi",
				},
			},
		},
		"ingress": map[string]interface{}{
			"enabled": true,
			"annotations": map[string]interface{}{
				"kubernetes.io/ingress.class": "nginx",
			},
		},
	}

	config := &Config{
		Name:      "complex-release",
		Namespace: "default",
		Chart: ChartConfig{
			Name: "nginx",
		},
		Values: complexValues,
		Source: SourceConfig{
			Type: HelmRepositorySource,
		},
	}

	hr, err := config.generateHelmRelease()
	if err != nil {
		t.Fatalf("generateHelmRelease() with complex values error = %v", err)
	}

	release := hr.(*helmv2.HelmRelease)
	if release.Spec.Values == nil {
		t.Fatal("Values should not be nil")
	}

	var parsedValues map[string]interface{}
	if err := yaml.Unmarshal(release.Spec.Values.Raw, &parsedValues); err != nil {
		t.Fatalf("Failed to parse values YAML: %v", err)
	}

	// Check nested values
	global, ok := parsedValues["global"].(map[string]interface{})
	if !ok {
		t.Fatal("global values not found or not a map")
	}
	if global["imageRegistry"] != "my-registry.com" {
		t.Errorf("global.imageRegistry = %v, want my-registry.com", global["imageRegistry"])
	}
}

func TestPostRenderersEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		postRenderers []PostRenderer
		expectPatches int
		expectImages  int
	}{
		{
			name:          "Empty post-renderers",
			postRenderers: []PostRenderer{},
			expectPatches: 0,
			expectImages:  0,
		},
		{
			name: "Nil kustomize",
			postRenderers: []PostRenderer{
				{
					Kustomize: nil,
				},
			},
			expectPatches: 0,
			expectImages:  0,
		},
		{
			name: "Empty kustomize patches and images",
			postRenderers: []PostRenderer{
				{
					Kustomize: &KustomizePostRenderer{
						Patches: []KustomizePatch{},
						Images:  []KustomizeImage{},
					},
				},
			},
			expectPatches: 0,
			expectImages:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Name:      "test",
				Namespace: "default",
				Chart: ChartConfig{
					Name: "nginx",
				},
				Source: SourceConfig{
					Type: HelmRepositorySource,
				},
				PostRenderers: tt.postRenderers,
			}

			hr, err := config.generateHelmRelease()
			if err != nil {
				t.Fatalf("generateHelmRelease() error = %v", err)
			}

			release := hr.(*helmv2.HelmRelease)

			actualPatches := 0
			actualImages := 0
			for _, pr := range release.Spec.PostRenderers {
				if pr.Kustomize != nil {
					actualPatches += len(pr.Kustomize.Patches)
					actualImages += len(pr.Kustomize.Images)
				}
			}

			if actualPatches != tt.expectPatches {
				t.Errorf("Expected %d patches, got %d", tt.expectPatches, actualPatches)
			}
			if actualImages != tt.expectImages {
				t.Errorf("Expected %d images, got %d", tt.expectImages, actualImages)
			}
		})
	}
}