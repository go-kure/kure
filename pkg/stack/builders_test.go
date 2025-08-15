package stack

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MockApplicationConfig for testing
type MockApplicationConfig struct {
	name string
	labels map[string]string
}

func (mac *MockApplicationConfig) Generate(app *Application) ([]*client.Object, error) {
	// Mock implementation - returns empty slice
	return []*client.Object{}, nil
}

func NewMockApplicationConfig(name string) *MockApplicationConfig {
	return &MockApplicationConfig{
		name: name,
		labels: map[string]string{
			"app": name,
		},
	}
}

func TestNewClusterBuilder(t *testing.T) {
	builder := NewClusterBuilder("test-cluster")
	
	if builder == nil {
		t.Fatal("expected non-nil cluster builder")
	}
	
	// Test basic build
	cluster := builder.Build()
	if cluster == nil {
		t.Fatal("expected non-nil cluster")
	}
	
	if cluster.Name != "test-cluster" {
		t.Errorf("expected cluster name 'test-cluster', got %s", cluster.Name)
	}
}

func TestClusterBuilder_WithGitOps(t *testing.T) {
	gitOpsConfig := &GitOpsConfig{
		Type: "flux",
		Bootstrap: &BootstrapConfig{
			Enabled: true,
			FluxMode: "gitops-toolkit",
		},
	}
	
	cluster := NewClusterBuilder("test-cluster").
		WithGitOps(gitOpsConfig).
		Build()
	
	if cluster.GitOps == nil {
		t.Fatal("expected non-nil GitOps config")
	}
	
	if cluster.GitOps.Type != "flux" {
		t.Errorf("expected GitOps type 'flux', got %s", cluster.GitOps.Type)
	}
	
	if !cluster.GitOps.Bootstrap.Enabled {
		t.Error("expected GitOps bootstrap to be enabled")
	}
}

func TestClusterBuilder_WithNode(t *testing.T) {
	cluster := NewClusterBuilder("test-cluster").
		WithNode("infrastructure").
		End().
		Build()
	
	if cluster.Node == nil {
		t.Fatal("expected non-nil root node")
	}
	
	if cluster.Node.Name != "infrastructure" {
		t.Errorf("expected node name 'infrastructure', got %s", cluster.Node.Name)
	}
}

func TestNodeBuilder_WithChild(t *testing.T) {
	cluster := NewClusterBuilder("test-cluster").
		WithNode("infrastructure").
			WithChild("monitoring").
		Build()
	
	if cluster.Node == nil {
		t.Fatal("expected non-nil root node")
	}
	
	if len(cluster.Node.Children) != 1 {
		t.Fatalf("expected 1 child node, got %d", len(cluster.Node.Children))
	}
	
	childNode := cluster.Node.Children[0]
	if childNode.Name != "monitoring" {
		t.Errorf("expected child node name 'monitoring', got %s", childNode.Name)
	}
	
	if childNode.ParentPath != "infrastructure" {
		t.Errorf("expected child node parent path 'infrastructure', got %s", childNode.ParentPath)
	}
}

func TestNodeBuilder_WithPackageRef(t *testing.T) {
	packageRef := &schema.GroupVersionKind{
		Group:   "generators.gokure.dev",
		Version: "v1alpha1",
		Kind:    "AppWorkload",
	}
	
	cluster := NewClusterBuilder("test-cluster").
		WithNode("infrastructure").
			WithPackageRef(packageRef).
		End().
		Build()
	
	if cluster.Node == nil {
		t.Fatal("expected non-nil root node")
	}
	
	if cluster.Node.PackageRef == nil {
		t.Fatal("expected non-nil package reference")
	}
	
	if cluster.Node.PackageRef.Kind != "AppWorkload" {
		t.Errorf("expected package ref kind 'AppWorkload', got %s", cluster.Node.PackageRef.Kind)
	}
}

func TestBundleBuilder_WithApplication(t *testing.T) {
	appConfig := NewMockApplicationConfig("web-app")
	
	cluster := NewClusterBuilder("test-cluster").
		WithNode("applications").
			WithBundle("web").
				WithApplication("frontend", appConfig).
			End().
		End().
		Build()
	
	if cluster.Node == nil {
		t.Fatal("expected non-nil root node")
	}
	
	if cluster.Node.Bundle == nil {
		t.Fatal("expected non-nil bundle")
	}
	
	if len(cluster.Node.Bundle.Applications) != 1 {
		t.Fatalf("expected 1 application, got %d", len(cluster.Node.Bundle.Applications))
	}
	
	app := cluster.Node.Bundle.Applications[0]
	if app.Name != "frontend" {
		t.Errorf("expected application name 'frontend', got %s", app.Name)
	}
	
	if app.Config == nil {
		t.Fatal("expected non-nil application config")
	}
}

func TestBundleBuilder_WithSourceRef(t *testing.T) {
	sourceRef := &SourceRef{
		Kind:      "GitRepository",
		Name:      "app-repo",
		Namespace: "flux-system",
	}
	
	cluster := NewClusterBuilder("test-cluster").
		WithNode("applications").
			WithBundle("web").
				WithSourceRef(sourceRef).
			End().
		End().
		Build()
	
	if cluster.Node == nil {
		t.Fatal("expected non-nil root node")
	}
	
	if cluster.Node.Bundle == nil {
		t.Fatal("expected non-nil bundle")
	}
	
	if cluster.Node.Bundle.SourceRef == nil {
		t.Fatal("expected non-nil source reference")
	}
	
	if cluster.Node.Bundle.SourceRef.Kind != "GitRepository" {
		t.Errorf("expected source ref kind 'GitRepository', got %s", cluster.Node.Bundle.SourceRef.Kind)
	}
}

func TestComplexFluentBuilder(t *testing.T) {
	// Test a complex scenario with nested nodes and bundles
	appConfig1 := NewMockApplicationConfig("prometheus")
	
	sourceRef := &SourceRef{
		Kind:      "GitRepository",
		Name:      "app-repo",
		Namespace: "flux-system",
	}
	
	gitOpsConfig := &GitOpsConfig{
		Type: "flux",
		Bootstrap: &BootstrapConfig{
			Enabled: true,
			FluxMode: "gitops-toolkit",
		},
	}
	
	// Build a cluster with nested structure: infrastructure -> monitoring -> prometheus bundle
	cluster := NewClusterBuilder("production").
		WithGitOps(gitOpsConfig).
		WithNode("infrastructure").
			WithChild("monitoring").
				WithBundle("prometheus").
					WithApplication("prometheus", appConfig1).
					WithSourceRef(sourceRef).
				End().
			End().
		Build()
	
	// Validate the complex structure
	if cluster.Name != "production" {
		t.Errorf("expected cluster name 'production', got %s", cluster.Name)
	}
	
	if cluster.GitOps == nil || cluster.GitOps.Type != "flux" {
		t.Error("expected flux GitOps configuration")
	}
	
	// Validate root node
	rootNode := cluster.Node
	if rootNode == nil || rootNode.Name != "infrastructure" {
		t.Fatal("expected infrastructure root node")
	}
	
	if len(rootNode.Children) != 1 {
		t.Fatalf("expected 1 child node, got %d", len(rootNode.Children))
	}
	
	// Find monitoring child
	monitoringNode := rootNode.Children[0]
	if monitoringNode == nil || monitoringNode.Name != "monitoring" {
		t.Fatal("expected monitoring node")
	}
	
	if monitoringNode.Bundle == nil || monitoringNode.Bundle.Name != "prometheus" {
		t.Fatal("expected prometheus bundle in monitoring node")
	}
	
	if len(monitoringNode.Bundle.Applications) != 1 {
		t.Fatalf("expected 1 application in prometheus bundle, got %d", len(monitoringNode.Bundle.Applications))
	}
	
	// Verify the application
	app := monitoringNode.Bundle.Applications[0]
	if app.Name != "prometheus" {
		t.Errorf("expected application name 'prometheus', got %s", app.Name)
	}
	
	// Verify source ref was set
	if monitoringNode.Bundle.SourceRef == nil || monitoringNode.Bundle.SourceRef.Kind != "GitRepository" {
		t.Error("expected GitRepository source reference")
	}
}

func TestBuilderImmutability(t *testing.T) {
	// Test that builders create immutable copies
	builder1 := NewClusterBuilder("test-cluster")
	builder2 := builder1.WithNode("node1")
	builder3 := builder1.WithNode("node2")
	
	cluster1 := builder2.End().Build()
	cluster2 := builder3.End().Build()
	
	// Both clusters should have different node names
	if cluster1.Node.Name == cluster2.Node.Name {
		t.Error("expected different node names, builders should be immutable")
	}
	
	if cluster1.Node.Name != "node1" {
		t.Errorf("expected cluster1 node name 'node1', got %s", cluster1.Node.Name)
	}
	
	if cluster2.Node.Name != "node2" {
		t.Errorf("expected cluster2 node name 'node2', got %s", cluster2.Node.Name)
	}
}

func TestBuilderChainingSafety(t *testing.T) {
	// Test that method chaining doesn't affect original builders
	baseBuilder := NewClusterBuilder("test-cluster")
	
	// Branch off from base builder
	branch1 := baseBuilder.WithNode("branch1")
	branch2 := baseBuilder.WithNode("branch2")
	
	cluster1 := branch1.End().Build()
	cluster2 := branch2.End().Build()
	
	// Verify that branches are independent
	if cluster1.Node.Name != "branch1" {
		t.Errorf("expected cluster1 node name 'branch1', got %s", cluster1.Node.Name)
	}
	
	if cluster2.Node.Name != "branch2" {
		t.Errorf("expected cluster2 node name 'branch2', got %s", cluster2.Node.Name)
	}
	
	// Verify base builder is unchanged (should build empty cluster)
	baseCluster := baseBuilder.Build()
	if baseCluster.Node != nil {
		t.Error("expected base builder to remain unchanged (no node)")
	}
}

func TestPathInitialization(t *testing.T) {
	// Test that path maps are properly initialized
	cluster := NewClusterBuilder("test-cluster").
		WithNode("infrastructure").
			WithChild("monitoring").
		Build()
	
	// After Build(), path maps should be initialized
	if cluster.Node.pathMap == nil {
		t.Error("expected path map to be initialized after Build()")
	}
	
	// Verify parent-child relationships are set
	if len(cluster.Node.Children) == 0 {
		t.Fatal("expected at least one child node")
	}
	
	monitoringNode := cluster.Node.Children[0]
	if monitoringNode.parent != cluster.Node {
		t.Error("expected parent-child relationship to be established")
	}
	
	// Verify paths are correct
	if monitoringNode.GetPath() != "infrastructure/monitoring" {
		t.Errorf("expected monitoring node path 'infrastructure/monitoring', got %s", monitoringNode.GetPath())
	}
}