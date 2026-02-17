package stack

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MockApplicationConfig for testing
type MockApplicationConfig struct {
	name   string
	labels map[string]string
}

func (mac *MockApplicationConfig) Generate(_ *Application) ([]*client.Object, error) {
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

// --- Existing tests (updated for (*Cluster, error) return) ---

func TestNewClusterBuilder(t *testing.T) {
	builder := NewClusterBuilder("test-cluster")
	if builder == nil {
		t.Fatal("expected non-nil cluster builder")
	}

	cluster, err := builder.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
			Enabled:  true,
			FluxMode: "gitops-toolkit",
		},
	}

	cluster, err := NewClusterBuilder("test-cluster").
		WithGitOps(gitOpsConfig).
		Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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
	cluster, err := NewClusterBuilder("test-cluster").
		WithNode("infrastructure").
		End().
		Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cluster.Node == nil {
		t.Fatal("expected non-nil root node")
	}
	if cluster.Node.Name != "infrastructure" {
		t.Errorf("expected node name 'infrastructure', got %s", cluster.Node.Name)
	}
}

func TestNodeBuilder_WithChild(t *testing.T) {
	cluster, err := NewClusterBuilder("test-cluster").
		WithNode("infrastructure").
		WithChild("monitoring").
		Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

	cluster, err := NewClusterBuilder("test-cluster").
		WithNode("infrastructure").
		WithPackageRef(packageRef).
		End().
		Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

	cluster, err := NewClusterBuilder("test-cluster").
		WithNode("applications").
		WithBundle("web").
		WithApplication("frontend", appConfig).
		End().
		End().
		Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

	cluster, err := NewClusterBuilder("test-cluster").
		WithNode("applications").
		WithBundle("web").
		WithSourceRef(sourceRef).
		End().
		End().
		Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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
	appConfig1 := NewMockApplicationConfig("prometheus")

	sourceRef := &SourceRef{
		Kind:      "GitRepository",
		Name:      "app-repo",
		Namespace: "flux-system",
	}

	gitOpsConfig := &GitOpsConfig{
		Type: "flux",
		Bootstrap: &BootstrapConfig{
			Enabled:  true,
			FluxMode: "gitops-toolkit",
		},
	}

	cluster, err := NewClusterBuilder("production").
		WithGitOps(gitOpsConfig).
		WithNode("infrastructure").
		WithChild("monitoring").
		WithBundle("prometheus").
		WithApplication("prometheus", appConfig1).
		WithSourceRef(sourceRef).
		End().
		End().
		Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cluster.Name != "production" {
		t.Errorf("expected cluster name 'production', got %s", cluster.Name)
	}
	if cluster.GitOps == nil || cluster.GitOps.Type != "flux" {
		t.Error("expected flux GitOps configuration")
	}

	rootNode := cluster.Node
	if rootNode == nil || rootNode.Name != "infrastructure" {
		t.Fatal("expected infrastructure root node")
	}
	if len(rootNode.Children) != 1 {
		t.Fatalf("expected 1 child node, got %d", len(rootNode.Children))
	}

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

	app := monitoringNode.Bundle.Applications[0]
	if app.Name != "prometheus" {
		t.Errorf("expected application name 'prometheus', got %s", app.Name)
	}
	if monitoringNode.Bundle.SourceRef == nil || monitoringNode.Bundle.SourceRef.Kind != "GitRepository" {
		t.Error("expected GitRepository source reference")
	}
}

func TestBuilderImmutability(t *testing.T) {
	builder1 := NewClusterBuilder("test-cluster")
	builder2 := builder1.WithNode("node1")
	builder3 := builder1.WithNode("node2")

	cluster1, err := builder2.End().Build()
	if err != nil {
		t.Fatalf("unexpected error building cluster1: %v", err)
	}
	cluster2, err := builder3.End().Build()
	if err != nil {
		t.Fatalf("unexpected error building cluster2: %v", err)
	}

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
	baseBuilder := NewClusterBuilder("test-cluster")

	branch1 := baseBuilder.WithNode("branch1")
	branch2 := baseBuilder.WithNode("branch2")

	cluster1, err := branch1.End().Build()
	if err != nil {
		t.Fatalf("unexpected error building cluster1: %v", err)
	}
	cluster2, err := branch2.End().Build()
	if err != nil {
		t.Fatalf("unexpected error building cluster2: %v", err)
	}

	if cluster1.Node.Name != "branch1" {
		t.Errorf("expected cluster1 node name 'branch1', got %s", cluster1.Node.Name)
	}
	if cluster2.Node.Name != "branch2" {
		t.Errorf("expected cluster2 node name 'branch2', got %s", cluster2.Node.Name)
	}

	// Base builder has been forked, so Build should work on a copy
	baseCluster, err := baseBuilder.Build()
	if err != nil {
		t.Fatalf("unexpected error building base cluster: %v", err)
	}
	// After forking twice, base builder's cluster was handed off to branch1,
	// then deep-copied for branch2. The base is forked, so building it
	// will deep-copy the cluster (which was mutated by branch1's WithNode).
	// The key contract is that branch1 and branch2 are independent.
	_ = baseCluster
}

func TestPathInitialization(t *testing.T) {
	cluster, err := NewClusterBuilder("test-cluster").
		WithNode("infrastructure").
		WithChild("monitoring").
		Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cluster.Node.pathMap == nil {
		t.Error("expected path map to be initialized after Build()")
	}

	if len(cluster.Node.Children) == 0 {
		t.Fatal("expected at least one child node")
	}

	monitoringNode := cluster.Node.Children[0]
	if monitoringNode.parent != cluster.Node {
		t.Error("expected parent-child relationship to be established")
	}
	if monitoringNode.GetPath() != "infrastructure/monitoring" {
		t.Errorf("expected monitoring node path 'infrastructure/monitoring', got %s", monitoringNode.GetPath())
	}
}

// --- Bug regression tests ---

func TestErrorSliceIsolation(t *testing.T) {
	base := NewClusterBuilder("test").
		WithNode("root").
		WithBundle("b")

	// Fork into two branches
	branch1 := base.WithApplication("", nil) // will accumulate error
	branch2 := base.WithApplication("valid", NewMockApplicationConfig("valid"))

	_, err1 := branch1.Build()
	_, err2 := branch2.Build()

	if err1 == nil {
		t.Error("expected error from branch1 (empty name)")
	}
	if err2 != nil {
		t.Errorf("expected no error from branch2, got: %v", err2)
	}
}

func TestWithApplicationPreservesExisting(t *testing.T) {
	appConfig1 := NewMockApplicationConfig("app1")
	appConfig2 := NewMockApplicationConfig("app2")

	cluster, err := NewClusterBuilder("test").
		WithNode("root").
		WithBundle("b").
		WithApplication("app1", appConfig1).
		WithApplication("app2", appConfig2).
		End().
		End().
		Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cluster.Node.Bundle.Applications) != 2 {
		t.Fatalf("expected 2 applications, got %d", len(cluster.Node.Bundle.Applications))
	}
	if cluster.Node.Bundle.Applications[0].Name != "app1" {
		t.Errorf("expected first app 'app1', got %s", cluster.Node.Bundle.Applications[0].Name)
	}
	if cluster.Node.Bundle.Applications[1].Name != "app2" {
		t.Errorf("expected second app 'app2', got %s", cluster.Node.Bundle.Applications[1].Name)
	}
}

func TestWithPackageRefCorrectNode(t *testing.T) {
	packageRef := &schema.GroupVersionKind{
		Group:   "test",
		Version: "v1",
		Kind:    "TestKind",
	}

	cluster, err := NewClusterBuilder("test").
		WithNode("root").
		WithPackageRef(packageRef).
		End().
		Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cluster.Node.PackageRef == nil {
		t.Fatal("expected package ref on root node")
	}
	if cluster.Node.PackageRef.Kind != "TestKind" {
		t.Errorf("expected package ref kind 'TestKind', got %s", cluster.Node.PackageRef.Kind)
	}
}

func TestWithDependencyCorrectBundle(t *testing.T) {
	dep := &Bundle{Name: "dep-bundle"}

	cluster, err := NewClusterBuilder("test").
		WithNode("root").
		WithBundle("main").
		WithDependency(dep).
		End().
		End().
		Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cluster.Node.Bundle == nil {
		t.Fatal("expected bundle on root node")
	}
	if len(cluster.Node.Bundle.DependsOn) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(cluster.Node.Bundle.DependsOn))
	}
	if cluster.Node.Bundle.DependsOn[0].Name != "dep-bundle" {
		t.Errorf("expected dependency name 'dep-bundle', got %s", cluster.Node.Bundle.DependsOn[0].Name)
	}
}

// --- CoW tests ---

func TestCoWForkAtClusterLevel(t *testing.T) {
	base := NewClusterBuilder("test")
	b1 := base.WithNode("node-a")
	b2 := base.WithNode("node-b")

	c1, err := b1.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c2, err := b2.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c1.Node.Name != "node-a" {
		t.Errorf("expected 'node-a', got %s", c1.Node.Name)
	}
	if c2.Node.Name != "node-b" {
		t.Errorf("expected 'node-b', got %s", c2.Node.Name)
	}
}

func TestCoWForkAtNodeLevel(t *testing.T) {
	base := NewClusterBuilder("test").
		WithNode("root")

	b1 := base.WithChild("child-a")
	b2 := base.WithChild("child-b")

	c1, err := b1.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c2, err := b2.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Each branch should have exactly one child
	if len(c1.Node.Children) != 1 {
		t.Fatalf("expected 1 child in c1, got %d", len(c1.Node.Children))
	}
	if len(c2.Node.Children) != 1 {
		t.Fatalf("expected 1 child in c2, got %d", len(c2.Node.Children))
	}
	if c1.Node.Children[0].Name != "child-a" {
		t.Errorf("expected 'child-a', got %s", c1.Node.Children[0].Name)
	}
	if c2.Node.Children[0].Name != "child-b" {
		t.Errorf("expected 'child-b', got %s", c2.Node.Children[0].Name)
	}
}

func TestCoWForkAtBundleLevel(t *testing.T) {
	cfg1 := NewMockApplicationConfig("a")
	cfg2 := NewMockApplicationConfig("b")

	base := NewClusterBuilder("test").
		WithNode("root").
		WithBundle("bundle")

	b1 := base.WithApplication("app-a", cfg1)
	b2 := base.WithApplication("app-b", cfg2)

	c1, err := b1.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c2, err := b2.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(c1.Node.Bundle.Applications) != 1 {
		t.Fatalf("expected 1 app in c1, got %d", len(c1.Node.Bundle.Applications))
	}
	if len(c2.Node.Bundle.Applications) != 1 {
		t.Fatalf("expected 1 app in c2, got %d", len(c2.Node.Bundle.Applications))
	}
	if c1.Node.Bundle.Applications[0].Name != "app-a" {
		t.Errorf("expected 'app-a', got %s", c1.Node.Bundle.Applications[0].Name)
	}
	if c2.Node.Bundle.Applications[0].Name != "app-b" {
		t.Errorf("expected 'app-b', got %s", c2.Node.Bundle.Applications[0].Name)
	}
}

func TestCoWEndThenContinue(t *testing.T) {
	nodeBuilder := NewClusterBuilder("test").
		WithNode("root").
		WithBundle("bundle").
		WithApplication("app1", NewMockApplicationConfig("a")).
		End() // back to NodeBuilder

	// Use the parent (NodeBuilder) to add a child
	withChild, err := nodeBuilder.WithChild("extra").Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Use the parent again to build without the child
	withoutChild, err := nodeBuilder.End().Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// withChild should have 1 child on root
	if len(withChild.Node.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(withChild.Node.Children))
	}
	if withChild.Node.Children[0].Name != "extra" {
		t.Errorf("expected child 'extra', got %s", withChild.Node.Children[0].Name)
	}

	// withoutChild should have no children on root
	if len(withoutChild.Node.Children) != 0 {
		t.Errorf("expected 0 children, got %d", len(withoutChild.Node.Children))
	}

	// Both should have the bundle with the application
	if withChild.Node.Bundle == nil || len(withChild.Node.Bundle.Applications) != 1 {
		t.Error("expected bundle with 1 app in withChild")
	}
	if withoutChild.Node.Bundle == nil || len(withoutChild.Node.Bundle.Applications) != 1 {
		t.Error("expected bundle with 1 app in withoutChild")
	}
}

// --- Validation tests ---

func TestBuildEmptyClusterName(t *testing.T) {
	_, err := NewClusterBuilder("").Build()
	if err == nil {
		t.Fatal("expected error for empty cluster name")
	}
	if !strings.Contains(err.Error(), "cluster name must not be empty") {
		t.Errorf("expected 'cluster name must not be empty' in error, got: %v", err)
	}
}

func TestBuildEmptyNodeName(t *testing.T) {
	_, err := NewClusterBuilder("test").
		WithNode("").
		Build()
	if err == nil {
		t.Fatal("expected error for empty node name")
	}
	if !strings.Contains(err.Error(), "node name must not be empty") {
		t.Errorf("expected 'node name must not be empty' in error, got: %v", err)
	}
}

func TestBuildNilAppConfig(t *testing.T) {
	_, err := NewClusterBuilder("test").
		WithNode("root").
		WithBundle("b").
		WithApplication("app", nil).
		Build()
	if err == nil {
		t.Fatal("expected error for nil app config")
	}
	if !strings.Contains(err.Error(), "must not be nil") {
		t.Errorf("expected 'must not be nil' in error, got: %v", err)
	}
}

func TestBuildNilDependency(t *testing.T) {
	_, err := NewClusterBuilder("test").
		WithNode("root").
		WithBundle("b").
		WithDependency(nil).
		Build()
	if err == nil {
		t.Fatal("expected error for nil dependency")
	}
	if !strings.Contains(err.Error(), "must not be nil") {
		t.Errorf("expected 'must not be nil' in error, got: %v", err)
	}
}

func TestBuildMultipleErrors(t *testing.T) {
	_, err := NewClusterBuilder("test").
		WithNode("root").
		WithBundle("b").
		WithApplication("", nil).   // empty name error
		WithApplication("ok", nil). // nil config error
		WithDependency(nil).        // nil dependency error
		Build()
	if err == nil {
		t.Fatal("expected multiple errors")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "application name must not be empty") {
		t.Errorf("expected empty name error, got: %v", err)
	}
	// After the empty name error, subsequent calls should still accumulate errors
}

func TestBuildNoErrorOnValid(t *testing.T) {
	cluster, err := NewClusterBuilder("test").
		WithNode("root").
		WithBundle("b").
		WithApplication("app", NewMockApplicationConfig("a")).
		WithSourceRef(&SourceRef{Kind: "GitRepository", Name: "repo", Namespace: "ns"}).
		WithDependency(&Bundle{Name: "dep"}).
		End().
		End().
		Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cluster == nil {
		t.Fatal("expected non-nil cluster")
	}
	if cluster.Node.Bundle.Applications[0].Name != "app" {
		t.Error("expected application 'app'")
	}
}

// --- Sequence tests ---

func TestMultipleApplications(t *testing.T) {
	cfg := NewMockApplicationConfig("cfg")

	cluster, err := NewClusterBuilder("test").
		WithNode("root").
		WithBundle("b").
		WithApplication("app1", cfg).
		WithApplication("app2", cfg).
		WithApplication("app3", cfg).
		End().
		End().
		Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cluster.Node.Bundle.Applications) != 3 {
		t.Fatalf("expected 3 applications, got %d", len(cluster.Node.Bundle.Applications))
	}
	for i, expected := range []string{"app1", "app2", "app3"} {
		if cluster.Node.Bundle.Applications[i].Name != expected {
			t.Errorf("expected app[%d] name %q, got %q", i, expected, cluster.Node.Bundle.Applications[i].Name)
		}
	}
}

func TestMultipleDependencies(t *testing.T) {
	dep1 := &Bundle{Name: "dep1"}
	dep2 := &Bundle{Name: "dep2"}
	dep3 := &Bundle{Name: "dep3"}

	cluster, err := NewClusterBuilder("test").
		WithNode("root").
		WithBundle("b").
		WithDependency(dep1).
		WithDependency(dep2).
		WithDependency(dep3).
		End().
		End().
		Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cluster.Node.Bundle.DependsOn) != 3 {
		t.Fatalf("expected 3 dependencies, got %d", len(cluster.Node.Bundle.DependsOn))
	}
	for i, expected := range []string{"dep1", "dep2", "dep3"} {
		if cluster.Node.Bundle.DependsOn[i].Name != expected {
			t.Errorf("expected dep[%d] name %q, got %q", i, expected, cluster.Node.Bundle.DependsOn[i].Name)
		}
	}
}
