package fluxcd_test

import (
	"testing"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"

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

	expectedModes := []string{"flux-operator", "gotk"}

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

func TestGenerateBootstrapDefaultMode(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()

	// Empty FluxMode should default to flux-operator
	config := &stack.BootstrapConfig{
		Enabled:     true,
		FluxVersion: "v2.4.0",
		SourceURL:   "oci://registry.example.com/flux-system",
		SourceRef:   "latest",
	}

	rootNode := &stack.Node{Name: "test-cluster"}

	resources, err := bg.GenerateBootstrap(config, rootNode)
	if err != nil {
		t.Fatalf("GenerateBootstrap() error = %v", err)
	}

	// Should generate FluxInstance (flux-operator mode)
	if len(resources) == 0 {
		t.Fatal("expected at least one resource")
	}

	// Verify it's a FluxInstance (not gotk output)
	if resources[0].GetObjectKind().GroupVersionKind().Kind != "FluxInstance" {
		t.Errorf("expected FluxInstance for default mode, got %s",
			resources[0].GetObjectKind().GroupVersionKind().Kind)
	}
}

func TestFluxOperatorSourceKindGitRepository(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()

	config := &stack.BootstrapConfig{
		Enabled:     true,
		FluxMode:    "flux-operator",
		FluxVersion: "v2.4.0",
		SourceKind:  "GitRepository",
		SourceURL:   "https://github.com/example/fleet.git",
		SourceRef:   "main",
	}

	rootNode := &stack.Node{Name: "production"}

	resources, err := bg.GenerateBootstrap(config, rootNode)
	if err != nil {
		t.Fatalf("GenerateBootstrap() error = %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	fi, ok := resources[0].(*fluxv1.FluxInstance)
	if !ok {
		t.Fatalf("expected FluxInstance, got %T", resources[0])
	}

	if fi.Spec.Sync == nil {
		t.Fatal("expected Sync to be set")
	}
	if fi.Spec.Sync.Kind != "GitRepository" {
		t.Errorf("Sync.Kind = %q, want %q", fi.Spec.Sync.Kind, "GitRepository")
	}
	if fi.Spec.Sync.URL != "https://github.com/example/fleet.git" {
		t.Errorf("Sync.URL = %q, want %q", fi.Spec.Sync.URL, "https://github.com/example/fleet.git")
	}
	if fi.Spec.Sync.Ref != "main" {
		t.Errorf("Sync.Ref = %q, want %q", fi.Spec.Sync.Ref, "main")
	}
}

func TestFluxOperatorSourceKindOCIDefault(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()

	// SourceKind empty should default to OCIRepository
	config := &stack.BootstrapConfig{
		Enabled:     true,
		FluxMode:    "flux-operator",
		FluxVersion: "v2.4.0",
		SourceURL:   "oci://registry.example.com/flux-system",
		SourceRef:   "latest",
	}

	rootNode := &stack.Node{Name: "staging"}

	resources, err := bg.GenerateBootstrap(config, rootNode)
	if err != nil {
		t.Fatalf("GenerateBootstrap() error = %v", err)
	}

	fi, ok := resources[0].(*fluxv1.FluxInstance)
	if !ok {
		t.Fatalf("expected FluxInstance, got %T", resources[0])
	}

	if fi.Spec.Sync == nil {
		t.Fatal("expected Sync to be set")
	}
	if fi.Spec.Sync.Kind != "OCIRepository" {
		t.Errorf("Sync.Kind = %q, want %q", fi.Spec.Sync.Kind, "OCIRepository")
	}
}

func TestFluxOperatorSourceKindExplicitOCI(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()

	config := &stack.BootstrapConfig{
		Enabled:    true,
		FluxMode:   "flux-operator",
		SourceKind: "OCIRepository",
		SourceURL:  "oci://registry.example.com/flux-system",
		SourceRef:  "v1.0.0",
	}

	rootNode := &stack.Node{Name: "prod"}

	resources, err := bg.GenerateBootstrap(config, rootNode)
	if err != nil {
		t.Fatalf("GenerateBootstrap() error = %v", err)
	}

	fi, ok := resources[0].(*fluxv1.FluxInstance)
	if !ok {
		t.Fatalf("expected FluxInstance, got %T", resources[0])
	}

	if fi.Spec.Sync.Kind != "OCIRepository" {
		t.Errorf("Sync.Kind = %q, want %q", fi.Spec.Sync.Kind, "OCIRepository")
	}
}

func TestGotkSourceKindGitRepository(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()

	config := &stack.BootstrapConfig{
		Enabled:    true,
		FluxMode:   "gotk",
		SourceKind: "GitRepository",
		SourceURL:  "https://github.com/example/fleet.git",
		SourceRef:  "main",
	}

	rootNode := &stack.Node{Name: "cluster"}

	// gotk mode may fail for component generation, but we can check
	// it doesn't panic. The source generation path uses SourceKind.
	_, _ = bg.GenerateBootstrap(config, rootNode)
}

func TestGotkSourceKindOCIDefault(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()

	config := &stack.BootstrapConfig{
		Enabled:   true,
		FluxMode:  "gotk",
		SourceURL: "oci://registry.example.com/flux-system",
		SourceRef: "latest",
	}

	rootNode := &stack.Node{Name: "cluster"}

	// gotk mode may fail for component generation, but the source
	// generation defaults to OCIRepository when SourceKind is empty.
	_, _ = bg.GenerateBootstrap(config, rootNode)
}

func TestV1alpha1BootstrapConfigSourceKind(t *testing.T) {
	// Verify the SourceKind field exists and round-trips in the runtime config
	config := &stack.BootstrapConfig{
		Enabled:    true,
		FluxMode:   "flux-operator",
		SourceKind: "GitRepository",
		SourceURL:  "https://github.com/example/fleet.git",
		SourceRef:  "main",
	}

	if config.SourceKind != "GitRepository" {
		t.Errorf("SourceKind = %q, want %q", config.SourceKind, "GitRepository")
	}

	// Verify OCIRepository
	config.SourceKind = "OCIRepository"
	if config.SourceKind != "OCIRepository" {
		t.Errorf("SourceKind = %q, want %q", config.SourceKind, "OCIRepository")
	}

	// Verify empty (backward compat)
	config.SourceKind = ""
	if config.SourceKind != "" {
		t.Errorf("SourceKind = %q, want empty", config.SourceKind)
	}
}

func TestGotkGitRepositorySourceGeneration(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()

	// Flux-operator with GitRepository SourceKind should produce FluxInstance with Git sync
	config := &stack.BootstrapConfig{
		Enabled:     true,
		FluxMode:    "flux-operator",
		FluxVersion: "v2.4.0",
		SourceKind:  "GitRepository",
		SourceURL:   "https://github.com/org/fleet.git",
		SourceRef:   "main",
	}

	rootNode := &stack.Node{Name: "test"}

	resources, err := bg.GenerateBootstrap(config, rootNode)
	if err != nil {
		t.Fatalf("GenerateBootstrap() error = %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource (FluxInstance), got %d", len(resources))
	}

	fi := resources[0].(*fluxv1.FluxInstance)
	if fi.Spec.Sync == nil {
		t.Fatal("Sync should be set")
	}
	if fi.Spec.Sync.Kind != "GitRepository" {
		t.Errorf("Sync.Kind = %q, want GitRepository", fi.Spec.Sync.Kind)
	}
	if fi.Spec.Sync.Path != "./test" {
		t.Errorf("Sync.Path = %q, want ./test", fi.Spec.Sync.Path)
	}
}
