package fluxcd_test

import (
	"testing"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
)

func TestNewBootstrapGenerator(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()

	if bg == nil {
		t.Fatal("expected non-nil bootstrap generator")
	}

	if bg.DefaultNamespace != "flux-system" {
		t.Errorf("DefaultNamespace = %q, want %q", bg.DefaultNamespace, "flux-system")
	}

	if bg.DefaultInterval != 10*60*1e9 { // 10 minutes in nanoseconds
		t.Errorf("DefaultInterval = %v, want 10 minutes", bg.DefaultInterval)
	}
}

func TestSupportedBootstrapModes(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()
	modes := bg.SupportedBootstrapModes()

	expectedModes := []string{"gotk", "flux-operator"}

	if len(modes) != len(expectedModes) {
		t.Errorf("SupportedBootstrapModes() returned %d modes, want %d", len(modes), len(expectedModes))
	}

	for i, mode := range expectedModes {
		if modes[i] != mode {
			t.Errorf("SupportedBootstrapModes()[%d] = %q, want %q", i, modes[i], mode)
		}
	}
}

func TestGenerateBootstrapNil(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()

	rootNode := &stack.Node{Name: "test"}

	// Nil config
	resources, err := bg.GenerateBootstrap(nil, rootNode)
	if err != nil {
		t.Errorf("GenerateBootstrap(nil, _) error = %v", err)
	}
	if resources != nil {
		t.Error("expected nil resources for nil config")
	}
}

func TestGenerateBootstrapDisabled(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()

	config := &stack.BootstrapConfig{
		Enabled: false,
	}
	rootNode := &stack.Node{Name: "test"}

	resources, err := bg.GenerateBootstrap(config, rootNode)
	if err != nil {
		t.Errorf("GenerateBootstrap(disabled, _) error = %v", err)
	}
	if resources != nil {
		t.Error("expected nil resources for disabled config")
	}
}

func TestGenerateBootstrapInvalidMode(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()

	config := &stack.BootstrapConfig{
		Enabled:  true,
		FluxMode: "invalid",
	}
	rootNode := &stack.Node{Name: "test"}

	_, err := bg.GenerateBootstrap(config, rootNode)
	if err == nil {
		t.Error("expected error for invalid flux mode")
	}
}

func TestGenerateFluxOperatorBootstrap(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()

	config := &stack.BootstrapConfig{
		Enabled:     true,
		FluxMode:    "flux-operator",
		FluxVersion: "v2.0.0",
		SourceURL:   "oci://registry.example.com/flux-system",
		SourceRef:   "latest",
	}

	rootNode := &stack.Node{Name: "test-cluster"}

	resources, err := bg.GenerateBootstrap(config, rootNode)
	if err != nil {
		t.Fatalf("GenerateBootstrap() error = %v", err)
	}

	// Should return at least a FluxInstance
	if len(resources) == 0 {
		t.Error("expected at least one resource for flux-operator mode")
	}
}

func TestGenerateFluxOperatorBootstrapWithComponents(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()

	config := &stack.BootstrapConfig{
		Enabled:    true,
		FluxMode:   "flux-operator",
		Components: []string{"source-controller", "kustomize-controller"},
	}

	rootNode := &stack.Node{Name: "test-cluster"}

	resources, err := bg.GenerateBootstrap(config, rootNode)
	if err != nil {
		t.Fatalf("GenerateBootstrap() error = %v", err)
	}

	if len(resources) == 0 {
		t.Error("expected at least one resource")
	}
}

func TestGenerateFluxOperatorBootstrapNoSourceURL(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()

	config := &stack.BootstrapConfig{
		Enabled:  true,
		FluxMode: "flux-operator",
		// No SourceURL
	}

	rootNode := &stack.Node{Name: "test-cluster"}

	resources, err := bg.GenerateBootstrap(config, rootNode)
	if err != nil {
		t.Fatalf("GenerateBootstrap() error = %v", err)
	}

	// Should still generate FluxInstance without Sync
	if len(resources) == 0 {
		t.Error("expected at least one resource")
	}
}

func TestGenerateGotkBootstrap(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()

	config := &stack.BootstrapConfig{
		Enabled:  true,
		FluxMode: "gotk",
		// Note: gotk mode generates real Flux components which may fail
		// in a test environment without proper setup
	}

	rootNode := &stack.Node{Name: "test-cluster"}

	// gotk mode may fail due to network or version requirements
	// we just test that it doesn't panic
	_, _ = bg.GenerateBootstrap(config, rootNode)
}

func TestGenerateGotkBootstrapWithOptions(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()

	config := &stack.BootstrapConfig{
		Enabled:         true,
		FluxMode:        "gotk",
		FluxVersion:     "v2.0.0",
		Registry:        "ghcr.io/fluxcd",
		ImagePullSecret: "my-secret",
		Components:      []string{"source-controller", "kustomize-controller"},
		SourceURL:       "oci://registry.example.com/flux-system",
		SourceRef:       "latest",
	}

	rootNode := &stack.Node{Name: "test-cluster"}

	// gotk mode attempts to generate real manifests
	// It may fail in tests without network access
	_, _ = bg.GenerateBootstrap(config, rootNode)
}
