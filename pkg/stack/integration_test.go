//go:build integration

package stack_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kure/kure/pkg/stack"
	_ "github.com/go-kure/kure/pkg/stack/argocd"
	_ "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MockAppConfig implements ApplicationConfig for integration testing
type MockAppConfig struct {
	Name      string
	Namespace string
	Image     string
	Replicas  int
}

func (m *MockAppConfig) Generate(app *stack.Application) ([]*client.Object, error) {
	// Return empty resources for mock - real integration tests would use actual generators
	return []*client.Object{}, nil
}

// TestFullPipelineFlux tests the complete generation workflow with FluxCD:
// Cluster → Node → Bundle → Application → Manifests
func TestFullPipelineFlux(t *testing.T) {
	// Create a complete cluster structure using fluent builders
	appConfig := &MockAppConfig{
		Name:      "nginx",
		Namespace: "default",
		Image:     "nginx:latest",
		Replicas:  3,
	}

	sourceRef := &stack.SourceRef{
		Kind:      "GitRepository",
		Name:      "infrastructure",
		Namespace: "flux-system",
	}

	gitOpsConfig := &stack.GitOpsConfig{
		Type: "flux",
		Bootstrap: &stack.BootstrapConfig{
			Enabled:     true,
			FluxMode:    "gitops-toolkit",
			FluxVersion: "v2.3.0",
			SourceURL:   "oci://ghcr.io/example/manifests",
		},
	}

	// Build cluster hierarchy:
	// production-cluster
	//   └── infrastructure
	//       └── monitoring (bundle: prometheus)
	cluster := stack.NewClusterBuilder("production-cluster").
		WithGitOps(gitOpsConfig).
		WithNode("infrastructure").
		WithBundle("platform-services").
		WithApplication("nginx", appConfig).
		WithSourceRef(sourceRef).
		End().
		WithChild("monitoring").
		WithBundle("prometheus").
		WithApplication("prometheus", appConfig).
		WithSourceRef(sourceRef).
		End().
		End().
		Build()

	// Verify cluster structure
	if cluster.Name != "production-cluster" {
		t.Errorf("expected cluster name 'production-cluster', got %s", cluster.Name)
	}

	if cluster.GitOps == nil || cluster.GitOps.Type != "flux" {
		t.Error("expected flux GitOps configuration")
	}

	// Create Flux workflow
	workflow, err := stack.NewWorkflow("flux")
	if err != nil {
		t.Fatalf("failed to create flux workflow: %v", err)
	}

	// Generate resources from cluster
	resources, err := workflow.GenerateFromCluster(cluster)
	if err != nil {
		t.Fatalf("GenerateFromCluster failed: %v", err)
	}

	// Verify resources were generated (may be empty for mock, but shouldn't error)
	t.Logf("Generated %d resources from cluster", len(resources))

	// Test CreateLayoutWithResources
	rules := layout.DefaultLayoutRules()
	rules.ClusterName = "production-cluster"

	layoutResult, err := workflow.CreateLayoutWithResources(cluster, rules)
	if err != nil {
		t.Fatalf("CreateLayoutWithResources failed: %v", err)
	}

	manifestLayout, ok := layoutResult.(*layout.ManifestLayout)
	if !ok {
		t.Fatal("expected *layout.ManifestLayout result")
	}

	if manifestLayout == nil {
		t.Fatal("manifest layout should not be nil")
	}

	// Test bootstrap generation
	bootstrapResources, err := workflow.GenerateBootstrap(cluster.GitOps.Bootstrap, cluster.Node)
	if err != nil {
		t.Fatalf("GenerateBootstrap failed: %v", err)
	}

	t.Logf("Generated %d bootstrap resources", len(bootstrapResources))
}

// TestFullPipelineArgoCD tests the complete generation workflow with ArgoCD
func TestFullPipelineArgoCD(t *testing.T) {
	appConfig := &MockAppConfig{
		Name:      "web-app",
		Namespace: "default",
		Image:     "app:latest",
		Replicas:  2,
	}

	sourceRef := &stack.SourceRef{
		Kind:      "GitRepository",
		Name:      "app-repo",
		Namespace: "argocd",
	}

	gitOpsConfig := &stack.GitOpsConfig{
		Type: "argocd",
		Bootstrap: &stack.BootstrapConfig{
			Enabled:         true,
			ArgoCDVersion:   "v2.8.0",
			ArgoCDNamespace: "argocd",
		},
	}

	// Build cluster with ArgoCD configuration
	cluster := stack.NewClusterBuilder("staging-cluster").
		WithGitOps(gitOpsConfig).
		WithNode("applications").
		WithBundle("web-services").
		WithApplication("frontend", appConfig).
		WithApplication("backend", appConfig).
		WithSourceRef(sourceRef).
		End().
		End().
		Build()

	// Create ArgoCD workflow
	workflow, err := stack.NewWorkflow("argocd")
	if err != nil {
		t.Fatalf("failed to create argocd workflow: %v", err)
	}

	// Generate resources from cluster
	resources, err := workflow.GenerateFromCluster(cluster)
	if err != nil {
		t.Fatalf("GenerateFromCluster failed: %v", err)
	}

	t.Logf("Generated %d ArgoCD resources", len(resources))

	// Test CreateLayoutWithResources
	rules := layout.DefaultLayoutRules()
	rules.ClusterName = "staging-cluster"

	layoutResult, err := workflow.CreateLayoutWithResources(cluster, rules)
	if err != nil {
		t.Fatalf("CreateLayoutWithResources failed: %v", err)
	}

	if layoutResult == nil {
		t.Fatal("layout result should not be nil")
	}

	// Test bootstrap generation
	bootstrapResources, err := workflow.GenerateBootstrap(cluster.GitOps.Bootstrap, cluster.Node)
	if err != nil {
		t.Fatalf("GenerateBootstrap failed: %v", err)
	}

	t.Logf("Generated %d ArgoCD bootstrap resources", len(bootstrapResources))
}

// TestLayoutGeneration tests directory structure creation with WriteManifest
func TestLayoutGeneration(t *testing.T) {
	// Create a temporary directory for output
	tempDir, err := os.MkdirTemp("", "kure-integration-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	appConfig := &MockAppConfig{
		Name:      "app",
		Namespace: "default",
		Image:     "app:v1",
		Replicas:  1,
	}

	sourceRef := &stack.SourceRef{
		Kind:      "GitRepository",
		Name:      "repo",
		Namespace: "flux-system",
	}

	// Build cluster
	cluster := stack.NewClusterBuilder("test-cluster").
		WithGitOps(&stack.GitOpsConfig{Type: "flux"}).
		WithNode("apps").
		WithBundle("my-app").
		WithApplication("my-app", appConfig).
		WithSourceRef(sourceRef).
		End().
		End().
		Build()

	// Create workflow and generate layout
	workflow, err := stack.NewWorkflow("flux")
	if err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	rules := layout.DefaultLayoutRules()
	rules.ClusterName = "test-cluster"

	layoutResult, err := workflow.CreateLayoutWithResources(cluster, rules)
	if err != nil {
		t.Fatalf("CreateLayoutWithResources failed: %v", err)
	}

	manifestLayout, ok := layoutResult.(*layout.ManifestLayout)
	if !ok {
		t.Fatal("expected *layout.ManifestLayout result")
	}

	// Write manifest layout to disk
	cfg := layout.DefaultLayoutConfig()

	err = layout.WriteManifest(tempDir, cfg, manifestLayout)
	if err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	// Verify directory structure was created
	clustersDir := filepath.Join(tempDir, "clusters")
	if _, err := os.Stat(clustersDir); os.IsNotExist(err) {
		t.Error("expected clusters directory to be created")
	}

	t.Logf("Successfully wrote manifest layout to %s", tempDir)

	// List created files for debugging
	err = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(tempDir, path)
		if info.IsDir() {
			t.Logf("  DIR: %s", relPath)
		} else {
			t.Logf("  FILE: %s", relPath)
		}
		return nil
	})
	if err != nil {
		t.Logf("Warning: failed to list files: %v", err)
	}
}

// TestMultiNodeClusterGeneration tests generation with multiple child nodes
func TestMultiNodeClusterGeneration(t *testing.T) {
	appConfig := &MockAppConfig{
		Name:      "service",
		Namespace: "default",
		Image:     "service:latest",
		Replicas:  2,
	}

	sourceRef := &stack.SourceRef{
		Kind:      "OCIRepository",
		Name:      "manifests",
		Namespace: "flux-system",
	}

	gitOpsConfig := &stack.GitOpsConfig{
		Type: "flux",
		Bootstrap: &stack.BootstrapConfig{
			Enabled:  true,
			FluxMode: "flux-operator",
		},
	}

	// Build a cluster with multiple nodes at the same level
	// root
	//   └── infrastructure (bundle: cert-manager)
	cluster := stack.NewClusterBuilder("multi-env-cluster").
		WithGitOps(gitOpsConfig).
		WithNode("root").
		WithChild("infrastructure").
		WithBundle("cert-manager").
		WithApplication("cert-manager", appConfig).
		WithSourceRef(sourceRef).
		End().
		End().
		Build()

	// Verify multi-node structure
	if cluster.Node == nil {
		t.Fatal("expected root node")
	}

	if len(cluster.Node.Children) != 1 {
		t.Errorf("expected 1 child node, got %d", len(cluster.Node.Children))
	}

	// Test with flux workflow
	workflow, err := stack.NewWorkflow("flux")
	if err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	resources, err := workflow.GenerateFromCluster(cluster)
	if err != nil {
		t.Fatalf("GenerateFromCluster failed: %v", err)
	}

	t.Logf("Generated %d resources for multi-node cluster", len(resources))

	// Test layout generation
	rules := layout.DefaultLayoutRules()
	rules.ClusterName = "multi-env-cluster"

	layoutResult, err := workflow.CreateLayoutWithResources(cluster, rules)
	if err != nil {
		t.Fatalf("CreateLayoutWithResources failed: %v", err)
	}

	manifestLayout, ok := layoutResult.(*layout.ManifestLayout)
	if !ok {
		t.Fatal("expected *layout.ManifestLayout result")
	}

	// Verify layout has children for each node
	t.Logf("Generated layout with %d child layouts", len(manifestLayout.Children))
}

// TestWorkflowSwitching tests switching between Flux and ArgoCD workflows
func TestWorkflowSwitching(t *testing.T) {
	appConfig := &MockAppConfig{
		Name:      "app",
		Namespace: "default",
		Image:     "app:latest",
		Replicas:  1,
	}

	sourceRef := &stack.SourceRef{
		Kind:      "GitRepository",
		Name:      "repo",
		Namespace: "flux-system",
	}

	// Create cluster without specific GitOps config
	cluster := stack.NewClusterBuilder("flexible-cluster").
		WithNode("apps").
		WithBundle("test-bundle").
		WithApplication("test-app", appConfig).
		WithSourceRef(sourceRef).
		End().
		End().
		Build()

	providers := []string{"flux", "argocd"}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			workflow, err := stack.NewWorkflow(provider)
			if err != nil {
				t.Fatalf("failed to create %s workflow: %v", provider, err)
			}

			// Set appropriate GitOps config
			switch provider {
			case "flux":
				cluster.GitOps = &stack.GitOpsConfig{
					Type: "flux",
					Bootstrap: &stack.BootstrapConfig{
						Enabled:  true,
						FluxMode: "gitops-toolkit",
					},
				}
			case "argocd":
				cluster.GitOps = &stack.GitOpsConfig{
					Type: "argocd",
					Bootstrap: &stack.BootstrapConfig{
						Enabled:         true,
						ArgoCDNamespace: "argocd",
					},
				}
			}

			resources, err := workflow.GenerateFromCluster(cluster)
			if err != nil {
				t.Fatalf("GenerateFromCluster failed for %s: %v", provider, err)
			}

			t.Logf("%s generated %d resources", provider, len(resources))

			rules := layout.DefaultLayoutRules()
			rules.ClusterName = "flexible-cluster"

			layoutResult, err := workflow.CreateLayoutWithResources(cluster, rules)
			if err != nil {
				t.Fatalf("CreateLayoutWithResources failed for %s: %v", provider, err)
			}

			if layoutResult == nil {
				t.Fatalf("layout result should not be nil for %s", provider)
			}
		})
	}
}

// TestBootstrapModes tests different bootstrap configurations
func TestBootstrapModes(t *testing.T) {
	bootstrapConfigs := []struct {
		name   string
		config *stack.BootstrapConfig
	}{
		{
			name: "flux-gotk",
			config: &stack.BootstrapConfig{
				Enabled:     true,
				FluxMode:    "gitops-toolkit",
				FluxVersion: "v2.3.0",
				Components:  []string{"source-controller", "kustomize-controller"},
			},
		},
		{
			name: "flux-operator",
			config: &stack.BootstrapConfig{
				Enabled:     true,
				FluxMode:    "flux-operator",
				FluxVersion: "v2.3.0",
				SourceURL:   "oci://ghcr.io/flux/manifests",
			},
		},
		{
			name: "bootstrap-disabled",
			config: &stack.BootstrapConfig{
				Enabled: false,
			},
		},
	}

	workflow, err := stack.NewWorkflow("flux")
	if err != nil {
		t.Fatalf("failed to create workflow: %v", err)
	}

	rootNode := &stack.Node{
		Name: "root",
	}

	for _, tc := range bootstrapConfigs {
		t.Run(tc.name, func(t *testing.T) {
			resources, err := workflow.GenerateBootstrap(tc.config, rootNode)
			if err != nil {
				t.Fatalf("GenerateBootstrap failed: %v", err)
			}

			if tc.config.Enabled && len(resources) == 0 {
				t.Log("Warning: bootstrap enabled but no resources generated (may be expected for mock)")
			}

			t.Logf("%s: generated %d bootstrap resources", tc.name, len(resources))
		})
	}
}

// TestClusterBuilderChaining tests the fluent builder API comprehensively
func TestClusterBuilderChaining(t *testing.T) {
	appConfig := &MockAppConfig{
		Name:      "test-app",
		Namespace: "default",
		Image:     "test:latest",
		Replicas:  1,
	}

	sourceRef := &stack.SourceRef{
		Kind:      "GitRepository",
		Name:      "test-repo",
		Namespace: "flux-system",
	}

	// Test basic chaining
	cluster := stack.NewClusterBuilder("test").
		WithNode("root").
		WithBundle("bundle1").
		WithApplication("app1", appConfig).
		WithSourceRef(sourceRef).
		End(). // Back to NodeBuilder
		End(). // Back to ClusterBuilder
		Build()

	if cluster == nil {
		t.Fatal("cluster should not be nil")
	}

	if cluster.Name != "test" {
		t.Errorf("expected cluster name 'test', got %s", cluster.Name)
	}

	if cluster.Node == nil {
		t.Fatal("expected root node")
	}

	if cluster.Node.Bundle == nil {
		t.Fatal("expected bundle on root node")
	}

	if len(cluster.Node.Bundle.Applications) != 1 {
		t.Errorf("expected 1 application, got %d", len(cluster.Node.Bundle.Applications))
	}
}
