package layout_test

import (
	"testing"

	"github.com/go-kure/kure/pkg/stack/layout"
)

func TestDefaultConfigForProfile(t *testing.T) {
	t.Run("FluxProfile", func(t *testing.T) {
		cfg := layout.DefaultConfigForProfile(layout.FluxProfile)

		// Should return default config for flux profile
		defaultCfg := layout.DefaultLayoutConfig()

		if cfg.ManifestsDir != defaultCfg.ManifestsDir {
			t.Errorf("FluxProfile ManifestsDir = %q, want %q", cfg.ManifestsDir, defaultCfg.ManifestsDir)
		}
	})

	t.Run("ArgoProfile", func(t *testing.T) {
		cfg := layout.DefaultConfigForProfile(layout.ArgoProfile)

		if cfg.ManifestsDir != "applications" {
			t.Errorf("ArgoProfile ManifestsDir = %q, want %q", cfg.ManifestsDir, "applications")
		}

		if cfg.FluxDir != "applications" {
			t.Errorf("ArgoProfile FluxDir = %q, want %q", cfg.FluxDir, "applications")
		}

		if cfg.FilePer != layout.FilePerResource {
			t.Errorf("ArgoProfile FilePer = %v, want %v", cfg.FilePer, layout.FilePerResource)
		}

		if cfg.ApplicationFileMode != layout.AppFileSingle {
			t.Errorf("ArgoProfile ApplicationFileMode = %v, want %v", cfg.ApplicationFileMode, layout.AppFileSingle)
		}

		// Test that ManifestFileName function is set
		if cfg.ManifestFileName == nil {
			t.Error("ArgoProfile ManifestFileName should not be nil")
		} else {
			// Verify the function works correctly
			result := cfg.ManifestFileName("default", "deployment", "myapp", layout.FilePerResource)
			if result == "" {
				t.Error("ManifestFileName should return non-empty string")
			}
		}

		// Test KustomizationFileName function
		if cfg.KustomizationFileName != nil {
			result := cfg.KustomizationFileName("test-app")
			expected := "application-test-app.yaml"
			if result != expected {
				t.Errorf("ArgoProfile KustomizationFileName(test-app) = %q, want %q", result, expected)
			}
		} else {
			t.Error("ArgoProfile KustomizationFileName should not be nil")
		}
	})

	t.Run("UnknownProfile", func(t *testing.T) {
		// Unknown profiles should fall back to FluxProfile (default)
		cfg := layout.DefaultConfigForProfile("unknown-profile")
		defaultCfg := layout.DefaultLayoutConfig()

		if cfg.ManifestsDir != defaultCfg.ManifestsDir {
			t.Errorf("Unknown profile should fallback to default config, ManifestsDir = %q, want %q", cfg.ManifestsDir, defaultCfg.ManifestsDir)
		}
	})

	t.Run("ProfileConstants", func(t *testing.T) {
		// Test that profile constants have expected values
		if layout.FluxProfile != "flux" {
			t.Errorf("FluxProfile = %q, want %q", layout.FluxProfile, "flux")
		}

		if layout.ArgoProfile != "argocd" {
			t.Errorf("ArgoProfile = %q, want %q", layout.ArgoProfile, "argocd")
		}
	})
}
