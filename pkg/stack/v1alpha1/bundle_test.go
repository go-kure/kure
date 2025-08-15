package v1alpha1

import (
	"testing"

	"github.com/go-kure/kure/internal/gvk"
)

func TestBundleConfig(t *testing.T) {
	tests := []struct {
		name    string
		bundle  *BundleConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid bundle config",
			bundle: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "app-bundle",
				},
				Spec: BundleSpec{
					ParentPath: "cluster/apps",
					Interval:   "5m",
					SourceRef: &SourceRef{
						Kind:      "GitRepository",
						Name:      "flux-system",
						Namespace: "flux-system",
					},
					Applications: []ApplicationReference{
						{Name: "app1", APIVersion: "v1", Kind: "Deployment"},
						{Name: "app2", APIVersion: "v1", Kind: "Service"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "bundle with dependencies",
			bundle: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "dependent-bundle",
				},
				Spec: BundleSpec{
					DependsOn: []BundleReference{
						{Name: "infrastructure"},
						{Name: "monitoring"},
					},
					Interval: "10m",
				},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			bundle: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Spec:       BundleSpec{},
			},
			wantErr: true,
			errMsg:  "metadata.name",
		},
		{
			name: "empty source ref kind",
			bundle: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "test-bundle",
				},
				Spec: BundleSpec{
					SourceRef: &SourceRef{
						Kind: "",
						Name: "source",
					},
				},
			},
			wantErr: true,
			errMsg:  "sourceRef kind cannot be empty",
		},
		{
			name: "empty source ref name",
			bundle: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "test-bundle",
				},
				Spec: BundleSpec{
					SourceRef: &SourceRef{
						Kind: "GitRepository",
						Name: "",
					},
				},
			},
			wantErr: true,
			errMsg:  "sourceRef name cannot be empty",
		},
		{
			name: "empty application name",
			bundle: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "test-bundle",
				},
				Spec: BundleSpec{
					Applications: []ApplicationReference{
						{Name: ""},
					},
				},
			},
			wantErr: true,
			errMsg:  "empty name",
		},
		{
			name: "duplicate applications",
			bundle: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "test-bundle",
				},
				Spec: BundleSpec{
					Applications: []ApplicationReference{
						{Name: "app1", APIVersion: "v1", Kind: "Deployment"},
						{Name: "app1", APIVersion: "v1", Kind: "Deployment"},
					},
				},
			},
			wantErr: true,
			errMsg:  "duplicate application reference",
		},
		{
			name: "self dependency",
			bundle: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "self-ref",
				},
				Spec: BundleSpec{
					DependsOn: []BundleReference{
						{Name: "self-ref"},
					},
				},
			},
			wantErr: true,
			errMsg:  "cannot depend on itself",
		},
		{
			name: "duplicate dependency",
			bundle: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "test-bundle",
				},
				Spec: BundleSpec{
					DependsOn: []BundleReference{
						{Name: "dep1"},
						{Name: "dep1"},
					},
				},
			},
			wantErr: true,
			errMsg:  "duplicate dependency",
		},
		{
			name:    "nil bundle",
			bundle:  nil,
			wantErr: true,
			errMsg:  "nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.bundle.Validate()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestBundleConfig_GettersSetters(t *testing.T) {
	bundle := NewBundleConfig("test-bundle")

	// Test initial values
	if bundle.GetName() != "test-bundle" {
		t.Errorf("expected name 'test-bundle', got %s", bundle.GetName())
	}

	if bundle.GetPath() != "test-bundle" {
		t.Errorf("expected path 'test-bundle', got %s", bundle.GetPath())
	}

	// Test default values
	if bundle.Spec.Interval != "5m" {
		t.Errorf("expected default interval '5m', got %s", bundle.Spec.Interval)
	}

	if !bundle.Spec.Prune {
		t.Error("expected Prune to be true by default")
	}

	if !bundle.Spec.Wait {
		t.Error("expected Wait to be true by default")
	}

	// Test with parent path
	bundle.Spec.ParentPath = "cluster/infrastructure"
	if bundle.GetPath() != "cluster/infrastructure/test-bundle" {
		t.Errorf("expected path 'cluster/infrastructure/test-bundle', got %s", bundle.GetPath())
	}

	// Test setters
	bundle.SetName("new-name")
	if bundle.GetName() != "new-name" {
		t.Errorf("expected name 'new-name', got %s", bundle.GetName())
	}

	bundle.SetNamespace("test-namespace")
	if bundle.GetNamespace() != "test-namespace" {
		t.Errorf("expected namespace 'test-namespace', got %s", bundle.GetNamespace())
	}
}

func TestBundleConfig_Helpers(t *testing.T) {
	bundle := NewBundleConfig("test-bundle")

	// Test AddApplication
	bundle.AddApplication("app1", "apps/v1", "Deployment")
	bundle.AddApplication("app2", "v1", "Service")

	if len(bundle.Spec.Applications) != 2 {
		t.Errorf("expected 2 applications, got %d", len(bundle.Spec.Applications))
	}

	if bundle.Spec.Applications[0].Name != "app1" {
		t.Errorf("expected first app 'app1', got %s", bundle.Spec.Applications[0].Name)
	}

	if bundle.Spec.Applications[0].Kind != "Deployment" {
		t.Errorf("expected first app kind 'Deployment', got %s", bundle.Spec.Applications[0].Kind)
	}

	// Test AddDependency
	bundle.AddDependency("infrastructure")
	bundle.AddDependency("monitoring")

	if len(bundle.Spec.DependsOn) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(bundle.Spec.DependsOn))
	}

	if bundle.Spec.DependsOn[0].Name != "infrastructure" {
		t.Errorf("expected first dependency 'infrastructure', got %s", bundle.Spec.DependsOn[0].Name)
	}

	// Test SetSourceRef
	bundle.SetSourceRef("GitRepository", "flux-system", "flux-system")

	if bundle.Spec.SourceRef == nil {
		t.Fatal("expected source ref to be set")
	}

	if bundle.Spec.SourceRef.Kind != "GitRepository" {
		t.Errorf("expected source ref kind 'GitRepository', got %s", bundle.Spec.SourceRef.Kind)
	}

	if bundle.Spec.SourceRef.Namespace != "flux-system" {
		t.Errorf("expected source ref namespace 'flux-system', got %s", bundle.Spec.SourceRef.Namespace)
	}
}

func TestBundleConfig_Conversion(t *testing.T) {
	bundle := NewBundleConfig("test-bundle")
	bundle.Spec.ParentPath = "cluster/apps"
	bundle.AddApplication("app1", "v1", "Deployment")
	bundle.AddDependency("infrastructure")
	bundle.SetSourceRef("GitRepository", "source", "default")

	// Test ConvertTo
	converted, err := bundle.ConvertTo("v1alpha1")
	if err != nil {
		t.Errorf("unexpected error converting to v1alpha1: %v", err)
	}

	if converted != bundle {
		t.Error("expected same instance when converting to same version")
	}

	// Test unsupported version
	_, err = bundle.ConvertTo("v2")
	if err == nil {
		t.Error("expected error for unsupported version")
	}

	// Test ConvertFrom
	newBundle := &BundleConfig{}
	err = newBundle.ConvertFrom(bundle)
	if err != nil {
		t.Errorf("unexpected error converting from BundleConfig: %v", err)
	}

	if newBundle.GetName() != bundle.GetName() {
		t.Errorf("expected name %s, got %s", bundle.GetName(), newBundle.GetName())
	}

	if len(newBundle.Spec.Applications) != len(bundle.Spec.Applications) {
		t.Errorf("expected %d applications, got %d",
			len(bundle.Spec.Applications), len(newBundle.Spec.Applications))
	}

	if len(newBundle.Spec.DependsOn) != len(bundle.Spec.DependsOn) {
		t.Errorf("expected %d dependencies, got %d",
			len(bundle.Spec.DependsOn), len(newBundle.Spec.DependsOn))
	}
}

func TestValidateInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval string
		wantErr  bool
		errMsg   string
	}{
		// Valid intervals
		{
			name:     "empty interval",
			interval: "",
			wantErr:  false,
		},
		{
			name:     "1 second",
			interval: "1s",
			wantErr:  false,
		},
		{
			name:     "30 seconds",
			interval: "30s",
			wantErr:  false,
		},
		{
			name:     "5 minutes",
			interval: "5m",
			wantErr:  false,
		},
		{
			name:     "1 hour",
			interval: "1h",
			wantErr:  false,
		},
		{
			name:     "24 hours",
			interval: "24h",
			wantErr:  false,
		},
		{
			name:     "complex duration",
			interval: "1h30m45s",
			wantErr:  false,
		},
		{
			name:     "decimal seconds",
			interval: "1.5s",
			wantErr:  false,
		},
		{
			name:     "decimal minutes",
			interval: "2.5m",
			wantErr:  false,
		},

		// Invalid intervals
		{
			name:     "invalid format - no unit",
			interval: "30",
			wantErr:  true,
			errMsg:   "invalid interval format",
		},
		{
			name:     "invalid format - wrong unit",
			interval: "5x",
			wantErr:  true,
			errMsg:   "invalid interval format",
		},
		{
			name:     "invalid format - negative",
			interval: "-5m",
			wantErr:  true,
			errMsg:   "invalid interval format",
		},
		{
			name:     "invalid format - mixed case",
			interval: "5M",
			wantErr:  true,
			errMsg:   "invalid interval format",
		},
		{
			name:     "zero duration",
			interval: "0s",
			wantErr:  true,
			errMsg:   "too short",
		},
		{
			name:     "too short - nanoseconds",
			interval: "500ns",
			wantErr:  true,
			errMsg:   "too short",
		},
		{
			name:     "too short - microseconds",
			interval: "500us",
			wantErr:  true,
			errMsg:   "too short",
		},
		{
			name:     "too short - milliseconds",
			interval: "500ms",
			wantErr:  true,
			errMsg:   "too short",
		},
		{
			name:     "too long - over 24 hours",
			interval: "25h",
			wantErr:  true,
			errMsg:   "too long",
		},
		{
			name:     "too long - days",
			interval: "48h",
			wantErr:  true,
			errMsg:   "too long",
		},
		{
			name:     "invalid format - spaces",
			interval: "5 m",
			wantErr:  true,
			errMsg:   "invalid interval format",
		},
		{
			name:     "invalid format - empty string in complex",
			interval: "1h30m45",
			wantErr:  true,
			errMsg:   "invalid interval format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInterval(tt.interval)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestBundleConfig_IntervalValidation(t *testing.T) {
	tests := []struct {
		name    string
		bundle  *BundleConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid interval",
			bundle: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "test-bundle",
				},
				Spec: BundleSpec{
					Interval: "5m",
				},
			},
			wantErr: false,
		},
		{
			name: "empty interval",
			bundle: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "test-bundle",
				},
				Spec: BundleSpec{
					Interval: "",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid interval format",
			bundle: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "test-bundle",
				},
				Spec: BundleSpec{
					Interval: "5 minutes",
				},
			},
			wantErr: true,
			errMsg:  "spec.interval",
		},
		{
			name: "too short interval",
			bundle: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "test-bundle",
				},
				Spec: BundleSpec{
					Interval: "500ms",
				},
			},
			wantErr: true,
			errMsg:  "too short",
		},
		{
			name: "too long interval",
			bundle: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "test-bundle",
				},
				Spec: BundleSpec{
					Interval: "48h",
				},
			},
			wantErr: true,
			errMsg:  "too long",
		},
		{
			name: "invalid timeout format",
			bundle: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "test-bundle",
				},
				Spec: BundleSpec{
					Timeout: "invalid",
				},
			},
			wantErr: true,
			errMsg:  "spec.timeout",
		},
		{
			name: "valid timeout",
			bundle: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "test-bundle",
				},
				Spec: BundleSpec{
					Timeout: "10m",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid retry interval format",
			bundle: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "test-bundle",
				},
				Spec: BundleSpec{
					RetryInterval: "2 minutes",
				},
			},
			wantErr: true,
			errMsg:  "spec.retryInterval",
		},
		{
			name: "valid retry interval",
			bundle: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "test-bundle",
				},
				Spec: BundleSpec{
					RetryInterval: "2m",
				},
			},
			wantErr: false,
		},
		{
			name: "all interval fields valid",
			bundle: &BundleConfig{
				APIVersion: "stack.gokure.dev/v1alpha1",
				Kind:       "Bundle",
				Metadata: gvk.BaseMetadata{
					Name: "test-bundle",
				},
				Spec: BundleSpec{
					Interval:      "5m",
					Timeout:       "10m",
					RetryInterval: "1m",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.bundle.Validate()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestBundleConfig_FluxIntegration(t *testing.T) {
	bundle := &BundleConfig{
		APIVersion: "stack.gokure.dev/v1alpha1",
		Kind:       "Bundle",
		Metadata: gvk.BaseMetadata{
			Name:      "flux-apps",
			Namespace: "flux-system",
		},
		Spec: BundleSpec{
			Interval:      "10m",
			RetryInterval: "2m",
			Timeout:       "5m",
			Prune:         true,
			Wait:          true,
			SourceRef: &SourceRef{
				Kind:       "GitRepository",
				Name:       "flux-system",
				Namespace:  "flux-system",
				APIVersion: "source.toolkit.fluxcd.io/v1",
			},
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "flux",
				"environment":                  "production",
			},
			Annotations: map[string]string{
				"flux.io/automated": "true",
			},
		},
	}

	err := bundle.Validate()
	if err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}

	// Verify Flux-specific settings
	if bundle.Spec.Interval != "10m" {
		t.Errorf("expected interval '10m', got %s", bundle.Spec.Interval)
	}

	if !bundle.Spec.Prune {
		t.Error("expected Prune to be true for Flux")
	}

	if bundle.Spec.SourceRef.Kind != "GitRepository" {
		t.Errorf("expected source ref kind 'GitRepository', got %s", bundle.Spec.SourceRef.Kind)
	}

	if bundle.Spec.Labels["app.kubernetes.io/managed-by"] != "flux" {
		t.Error("expected flux managed-by label")
	}
}
