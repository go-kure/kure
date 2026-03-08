package layout_test

import (
	"testing"

	"github.com/go-kure/kure/pkg/stack/layout"
)

func TestResolveManifestFileName_Default(t *testing.T) {
	cfg := layout.Config{}
	fn := cfg.ResolveManifestFileName()
	result := fn("default", "deployment", "myapp", layout.FilePerResource)
	if result != "default-deployment-myapp.yaml" {
		t.Errorf("expected default-deployment-myapp.yaml, got %s", result)
	}
}

func TestResolveManifestFileName_KindName(t *testing.T) {
	cfg := layout.Config{FileNaming: layout.FileNamingKindName}
	fn := cfg.ResolveManifestFileName()
	result := fn("default", "deployment", "myapp", layout.FilePerResource)
	if result != "deployment-myapp.yaml" {
		t.Errorf("expected deployment-myapp.yaml, got %s", result)
	}
}

func TestResolveManifestFileName_ExplicitFuncOverridesFileNaming(t *testing.T) {
	custom := func(_, _, name string, _ layout.FileExportMode) string {
		return name + ".yaml"
	}
	cfg := layout.Config{
		FileNaming:       layout.FileNamingKindName,
		ManifestFileName: custom,
	}
	fn := cfg.ResolveManifestFileName()
	result := fn("ns", "deployment", "myapp", layout.FilePerResource)
	if result != "myapp.yaml" {
		t.Errorf("explicit func should override FileNaming, got %s", result)
	}
}

func TestResolveKustomizationMode_Default(t *testing.T) {
	cfg := layout.Config{}
	mode := cfg.ResolveKustomizationMode(layout.FluxSeparate)
	if mode != layout.KustomizationExplicit {
		t.Errorf("expected KustomizationExplicit, got %s", mode)
	}
}

func TestResolveKustomizationMode_GlobalOverride(t *testing.T) {
	cfg := layout.Config{KustomizationMode: layout.KustomizationRecursive}
	mode := cfg.ResolveKustomizationMode(layout.FluxSeparate)
	if mode != layout.KustomizationRecursive {
		t.Errorf("expected KustomizationRecursive, got %s", mode)
	}
}

func TestResolveKustomizationMode_PerFluxPlacement(t *testing.T) {
	cfg := layout.Config{
		KustomizationMode: layout.KustomizationExplicit,
		FluxKustomizationMode: map[layout.FluxPlacement]layout.KustomizationMode{
			layout.FluxIntegrated: layout.KustomizationRecursive,
		},
	}
	// FluxIntegrated should use the override
	mode := cfg.ResolveKustomizationMode(layout.FluxIntegrated)
	if mode != layout.KustomizationRecursive {
		t.Errorf("expected KustomizationRecursive for FluxIntegrated, got %s", mode)
	}
	// FluxSeparate should fall back to global
	mode = cfg.ResolveKustomizationMode(layout.FluxSeparate)
	if mode != layout.KustomizationExplicit {
		t.Errorf("expected KustomizationExplicit for FluxSeparate, got %s", mode)
	}
}
func TestDefaultKustomizationFileName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple name",
			input:    "test",
			expected: "kustomization-test.yaml",
		},
		{
			name:     "name with dashes",
			input:    "my-cluster",
			expected: "kustomization-my-cluster.yaml",
		},
		{
			name:     "empty name",
			input:    "",
			expected: "kustomization-.yaml",
		},
		{
			name:     "name with underscores",
			input:    "test_kust",
			expected: "kustomization-test_kust.yaml",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := layout.DefaultKustomizationFileName(test.input)
			if result != test.expected {
				t.Errorf("expected %q, got %q", test.expected, result)
			}
		})
	}
}
