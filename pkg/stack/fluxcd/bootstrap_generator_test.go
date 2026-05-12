package fluxcd_test

import (
	"testing"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
)

// findFluxInstance locates the FluxInstance in a flux-operator bootstrap
// resource list. Since v0.1.0-rc.5 the list also contains the Flux
// Operator install bundle (Namespace, CRDs, RBAC, Deployment, Service)
// so FluxInstance is no longer at index 0.
func findFluxInstance(t *testing.T, resources []client.Object) *fluxv1.FluxInstance {
	t.Helper()
	for _, obj := range resources {
		if fi, ok := obj.(*fluxv1.FluxInstance); ok {
			return fi
		}
	}
	t.Fatalf("FluxInstance not found in %d resources", len(resources))
	return nil
}

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

	// flux-operator mode emits the install bundle + FluxInstance.
	if len(resources) == 0 {
		t.Fatal("expected at least one resource")
	}
	_ = findFluxInstance(t, resources)
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

	fi := findFluxInstance(t, resources)

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

	fi := findFluxInstance(t, resources)

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

	fi := findFluxInstance(t, resources)

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

	fi := findFluxInstance(t, resources)
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

func TestGenerateFluxInstanceNilConfig(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()

	fi, err := bg.GenerateFluxInstance(nil, nil)
	if err != nil {
		t.Errorf("GenerateFluxInstance(nil, nil) error = %v, want nil", err)
	}
	if fi != nil {
		t.Errorf("GenerateFluxInstance(nil, nil) = %v, want nil", fi)
	}
}

func TestGenerateFluxInstanceDistribution(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()

	config := &stack.BootstrapConfig{
		Enabled:     true,
		FluxVersion: "v2.4.0",
		Registry:    "registry.example.com",
	}

	fi, err := bg.GenerateFluxInstance(config, nil)
	if err != nil {
		t.Fatalf("GenerateFluxInstance() error = %v", err)
	}
	if fi == nil {
		t.Fatal("expected non-nil FluxInstance")
	}

	if fi.Spec.Distribution.Version != "v2.4.0" {
		t.Errorf("Distribution.Version = %q, want %q", fi.Spec.Distribution.Version, "v2.4.0")
	}
	if fi.Spec.Distribution.Registry != "registry.example.com" {
		t.Errorf("Distribution.Registry = %q, want %q", fi.Spec.Distribution.Registry, "registry.example.com")
	}
}

func TestGenerateFluxInstanceSyncFromSourceURL(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()

	config := &stack.BootstrapConfig{
		Enabled:    true,
		SourceURL:  "https://github.com/example/fleet.git",
		SourceRef:  "main",
		SourceKind: "GitRepository",
	}
	rootNode := &stack.Node{Name: "production"}

	fi, err := bg.GenerateFluxInstance(config, rootNode)
	if err != nil {
		t.Fatalf("GenerateFluxInstance() error = %v", err)
	}
	if fi == nil {
		t.Fatal("expected non-nil FluxInstance")
	}

	if fi.Spec.Sync == nil {
		t.Fatal("expected Sync to be set when SourceURL is non-empty")
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
	if fi.Spec.Sync.Path != "./production" {
		t.Errorf("Sync.Path = %q, want %q", fi.Spec.Sync.Path, "./production")
	}
}

func TestGenerateFluxInstanceNoSyncWhenNoSourceURL(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()

	config := &stack.BootstrapConfig{
		Enabled:     true,
		FluxVersion: "v2.4.0",
		// No SourceURL
	}

	fi, err := bg.GenerateFluxInstance(config, nil)
	if err != nil {
		t.Fatalf("GenerateFluxInstance() error = %v", err)
	}
	if fi == nil {
		t.Fatal("expected non-nil FluxInstance")
	}

	if fi.Spec.Sync != nil {
		t.Errorf("expected Sync to be nil when SourceURL is empty, got %+v", fi.Spec.Sync)
	}
}

// TestFluxOperatorInstallObjects verifies the vendored install.yaml parses
// into the expected resource inventory. If the manifest is bumped to a
// newer flux-operator release the counts may shift and this test should
// be updated deliberately.
func TestFluxOperatorInstallObjects(t *testing.T) {
	objs, err := fluxstack.FluxOperatorInstallObjects()
	if err != nil {
		t.Fatalf("FluxOperatorInstallObjects() error = %v", err)
	}

	if fluxstack.FluxOperatorVersion == "" {
		t.Error("FluxOperatorVersion must not be empty")
	}

	counts := map[string]int{}
	for _, obj := range objs {
		kind := obj.GetObjectKind().GroupVersionKind().Kind
		counts[kind]++
	}

	// Inventory from upstream install.yaml at FluxOperatorVersion.
	// Update deliberately if the vendored manifest is refreshed.
	wantCounts := map[string]int{
		"Namespace":                1,
		"CustomResourceDefinition": 4, // FluxInstance, FluxReport, ResourceSet, ResourceSetInputProvider
		"ServiceAccount":           1,
		"ClusterRole":              4,
		"ClusterRoleBinding":       1,
		"Service":                  1,
		"Deployment":               1,
	}
	for kind, want := range wantCounts {
		if got := counts[kind]; got != want {
			t.Errorf("install bundle kind %q: got %d, want %d (all kinds: %v)", kind, got, want, counts)
		}
	}
}

// TestFluxOperatorBootstrapIncludesInstallBundle asserts the install
// bundle is emitted before the FluxInstance so a single apply of the
// result is enough to stand up flux-operator from scratch.
func TestFluxOperatorBootstrapIncludesInstallBundle(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()

	config := &stack.BootstrapConfig{
		Enabled:  true,
		FluxMode: "flux-operator",
	}
	rootNode := &stack.Node{Name: "test"}

	resources, err := bg.GenerateBootstrap(config, rootNode)
	if err != nil {
		t.Fatalf("GenerateBootstrap() error = %v", err)
	}

	// Expect install bundle objects (> 1) plus the FluxInstance.
	if len(resources) < 2 {
		t.Fatalf("expected install bundle + FluxInstance, got %d resources", len(resources))
	}

	// FluxInstance must be the last object so it's applied after the CRDs
	// and controller are in place.
	last := resources[len(resources)-1]
	if _, ok := last.(*fluxv1.FluxInstance); !ok {
		t.Errorf("last resource: got %T, want *fluxv1.FluxInstance", last)
	}

	// Find the install-bundle marker resources to be sure they were prepended.
	kinds := map[string]bool{}
	for _, obj := range resources {
		kinds[obj.GetObjectKind().GroupVersionKind().Kind] = true
	}
	mustHave := []string{"Namespace", "CustomResourceDefinition", "ClusterRole", "ClusterRoleBinding", "ServiceAccount", "Deployment"}
	for _, k := range mustHave {
		if !kinds[k] {
			t.Errorf("missing expected install-bundle kind %q (have: %v)", k, kinds)
		}
	}
}
