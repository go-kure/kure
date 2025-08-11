package argocd

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

func TestEngine(t *testing.T) {
	engine := Engine()
	if engine == nil {
		t.Fatal("expected non-nil WorkflowEngine")
	}

	if engine.RepoURL != "https://github.com/example/manifests.git" {
		t.Errorf("expected default RepoURL 'https://github.com/example/manifests.git', got %s", engine.RepoURL)
	}

	if engine.DefaultNamespace != "argocd" {
		t.Errorf("expected default DefaultNamespace 'argocd', got %s", engine.DefaultNamespace)
	}
}

func TestWorkflowEngineInterface(t *testing.T) {
	var engine interface{} = Engine()
	
	// Test that it implements stack.Workflow interface
	if _, ok := engine.(stack.Workflow); !ok {
		t.Error("WorkflowEngine should implement stack.Workflow interface")
	}
}

func TestGetName(t *testing.T) {
	engine := Engine()
	name := engine.GetName()
	
	if name != "ArgoCD Workflow Engine" {
		t.Errorf("expected name 'ArgoCD Workflow Engine', got %s", name)
	}
}

func TestGetVersion(t *testing.T) {
	engine := Engine()
	version := engine.GetVersion()
	
	if version != "v1.0.0" {
		t.Errorf("expected version 'v1.0.0', got %s", version)
	}
}

func TestSetRepoURL(t *testing.T) {
	engine := Engine()
	newURL := "https://github.com/test/repo.git"
	
	engine.SetRepoURL(newURL)
	
	if engine.RepoURL != newURL {
		t.Errorf("expected RepoURL '%s', got %s", newURL, engine.RepoURL)
	}
}

func TestSetDefaultNamespace(t *testing.T) {
	engine := Engine()
	newNamespace := "custom-argocd"
	
	engine.SetDefaultNamespace(newNamespace)
	
	if engine.DefaultNamespace != newNamespace {
		t.Errorf("expected DefaultNamespace '%s', got %s", newNamespace, engine.DefaultNamespace)
	}
}

func TestSupportedBootstrapModes(t *testing.T) {
	engine := Engine()
	modes := engine.SupportedBootstrapModes()
	
	expectedModes := []string{"argocd", "app-of-apps"}
	if len(modes) != len(expectedModes) {
		t.Errorf("expected %d modes, got %d", len(expectedModes), len(modes))
		return
	}
	
	for i, expected := range expectedModes {
		if modes[i] != expected {
			t.Errorf("expected mode[%d] '%s', got '%s'", i, expected, modes[i])
		}
	}
}

func TestGenerateFromCluster_NilCluster(t *testing.T) {
	engine := Engine()
	
	objs, err := engine.GenerateFromCluster(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if objs != nil {
		t.Error("expected nil objects for nil cluster")
	}
}

func TestGenerateFromCluster_NilNode(t *testing.T) {
	engine := Engine()
	cluster := &stack.Cluster{Node: nil}
	
	objs, err := engine.GenerateFromCluster(cluster)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if objs != nil {
		t.Error("expected nil objects for cluster with nil node")
	}
}

func TestGenerateFromCluster_Success(t *testing.T) {
	engine := Engine()
	
	bundle := &stack.Bundle{
		Name:   "test-bundle",
		Labels: map[string]string{"app": "test"},
	}
	
	node := &stack.Node{
		Bundle: bundle,
	}
	
	cluster := &stack.Cluster{
		Node: node,
	}
	
	objs, err := engine.GenerateFromCluster(cluster)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	
	if len(objs) != 1 {
		t.Errorf("expected 1 object, got %d", len(objs))
	}
}

func TestGenerateFromNode_NilNode(t *testing.T) {
	engine := Engine()
	
	objs, err := engine.GenerateFromNode(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if objs != nil {
		t.Error("expected nil objects for nil node")
	}
}

func TestGenerateFromNode_WithBundle(t *testing.T) {
	engine := Engine()
	
	bundle := &stack.Bundle{
		Name: "test-bundle",
	}
	
	node := &stack.Node{
		Bundle: bundle,
	}
	
	objs, err := engine.GenerateFromNode(node)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	
	if len(objs) != 1 {
		t.Errorf("expected 1 object, got %d", len(objs))
	}
}

func TestGenerateFromNode_WithChildren(t *testing.T) {
	engine := Engine()
	
	childBundle1 := &stack.Bundle{Name: "child1"}
	childBundle2 := &stack.Bundle{Name: "child2"}
	
	child1 := &stack.Node{Bundle: childBundle1}
	child2 := &stack.Node{Bundle: childBundle2}
	
	parentBundle := &stack.Bundle{Name: "parent"}
	parent := &stack.Node{
		Bundle:   parentBundle,
		Children: []*stack.Node{child1, child2},
	}
	
	objs, err := engine.GenerateFromNode(parent)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	
	// Should have 3 applications: parent + 2 children
	if len(objs) != 3 {
		t.Errorf("expected 3 objects, got %d", len(objs))
	}
}

func TestGenerateFromBundle_NilBundle(t *testing.T) {
	engine := Engine()
	
	objs, err := engine.GenerateFromBundle(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if objs != nil {
		t.Error("expected nil objects for nil bundle")
	}
}

func TestGenerateFromBundle_Success(t *testing.T) {
	engine := Engine()
	engine.SetRepoURL("https://github.com/test/repo.git")
	engine.SetDefaultNamespace("test-namespace")
	
	bundle := &stack.Bundle{
		Name:   "test-bundle",
		Labels: map[string]string{"app": "test", "env": "dev"},
		DependsOn: []*stack.Bundle{
			{Name: "dependency1"},
			{Name: "dependency2"},
		},
	}
	
	objs, err := engine.GenerateFromBundle(bundle)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	
	if len(objs) != 1 {
		t.Errorf("expected 1 object, got %d", len(objs))
	}
	
	// Verify the generated Application
	app := objs[0].(*unstructured.Unstructured)
	
	if app.GetAPIVersion() != "argoproj.io/v1alpha1" {
		t.Errorf("expected APIVersion 'argoproj.io/v1alpha1', got %s", app.GetAPIVersion())
	}
	
	if app.GetKind() != "Application" {
		t.Errorf("expected Kind 'Application', got %s", app.GetKind())
	}
	
	if app.GetName() != "test-bundle" {
		t.Errorf("expected name 'test-bundle', got %s", app.GetName())
	}
	
	if app.GetNamespace() != "test-namespace" {
		t.Errorf("expected namespace 'test-namespace', got %s", app.GetNamespace())
	}
	
	// Check labels
	labels := app.GetLabels()
	if labels["app"] != "test" {
		t.Errorf("expected label app='test', got %s", labels["app"])
	}
	if labels["env"] != "dev" {
		t.Errorf("expected label env='dev', got %s", labels["env"])
	}
	
	// Check source configuration
	source, found, err := unstructured.NestedMap(app.Object, "spec", "source")
	if err != nil {
		t.Errorf("error getting source: %v", err)
	}
	if !found {
		t.Error("source not found in spec")
	}
	if source["repoURL"] != "https://github.com/test/repo.git" {
		t.Errorf("expected repoURL 'https://github.com/test/repo.git', got %s", source["repoURL"])
	}
	
	// Check destination configuration
	dest, found, err := unstructured.NestedMap(app.Object, "spec", "destination")
	if err != nil {
		t.Errorf("error getting destination: %v", err)
	}
	if !found {
		t.Error("destination not found in spec")
	}
	if dest["server"] != "https://kubernetes.default.svc" {
		t.Errorf("expected server 'https://kubernetes.default.svc', got %s", dest["server"])
	}
	if dest["namespace"] != "default" {
		t.Errorf("expected namespace 'default', got %s", dest["namespace"])
	}
	
	// Check dependencies
	deps, found, err := unstructured.NestedStringSlice(app.Object, "spec", "dependencies")
	if err != nil {
		t.Errorf("error getting dependencies: %v", err)
	}
	if !found {
		t.Error("dependencies not found in spec")
	}
	expectedDeps := []string{"dependency1", "dependency2"}
	if len(deps) != len(expectedDeps) {
		t.Errorf("expected %d dependencies, got %d", len(expectedDeps), len(deps))
	}
	for i, expected := range expectedDeps {
		if deps[i] != expected {
			t.Errorf("expected dependency[%d] '%s', got '%s'", i, expected, deps[i])
		}
	}
}

func TestGenerateFromBundle_NoDependencies(t *testing.T) {
	engine := Engine()
	
	bundle := &stack.Bundle{
		Name: "simple-bundle",
	}
	
	objs, err := engine.GenerateFromBundle(bundle)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	
	if len(objs) != 1 {
		t.Errorf("expected 1 object, got %d", len(objs))
	}
	
	app := objs[0].(*unstructured.Unstructured)
	
	// Dependencies should not be set
	_, found, err := unstructured.NestedStringSlice(app.Object, "spec", "dependencies")
	if err != nil {
		t.Errorf("error checking dependencies: %v", err)
	}
	if found {
		t.Error("dependencies should not be found for bundle with no dependencies")
	}
}

func TestIntegrateWithLayout(t *testing.T) {
	engine := Engine()
	ml := &layout.ManifestLayout{}
	cluster := &stack.Cluster{}
	rules := layout.LayoutRules{}
	
	// Should return nil error as ArgoCD doesn't need layout integration
	err := engine.IntegrateWithLayout(ml, cluster, rules)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateLayoutWithResources_InvalidRules(t *testing.T) {
	engine := Engine()
	cluster := &stack.Cluster{}
	
	// Pass invalid rules type
	result, err := engine.CreateLayoutWithResources(cluster, "invalid")
	if err == nil {
		t.Error("expected error for invalid rules type")
	}
	if result != nil {
		t.Error("expected nil result for invalid rules")
	}
}

func TestCreateLayoutWithResources_Success(t *testing.T) {
	engine := Engine()
	
	bundle := &stack.Bundle{Name: "test-bundle"}
	node := &stack.Node{Bundle: bundle, Name: "test-node"}
	cluster := &stack.Cluster{
		Name: "test-cluster",
		Node: node,
	}
	
	rules := layout.LayoutRules{
		BundleGrouping:      layout.GroupFlat,
		ApplicationGrouping: layout.GroupFlat,
	}
	
	result, err := engine.CreateLayoutWithResources(cluster, rules)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	
	ml, ok := result.(*layout.ManifestLayout)
	if !ok {
		t.Error("expected ManifestLayout result")
		return
	}
	
	// The layout name comes from the node name
	if ml.Name != "test-node" {
		t.Errorf("expected layout name 'test-node', got %s", ml.Name)
	}
	
	// Should have one child for argocd directory
	if len(ml.Children) != 1 {
		t.Errorf("expected 1 child, got %d", len(ml.Children))
		return
	}
	
	argoCDLayout := ml.Children[0]
	if argoCDLayout.Name != "argocd" {
		t.Errorf("expected child name 'argocd', got %s", argoCDLayout.Name)
	}
	
	if len(argoCDLayout.Resources) != 1 {
		t.Errorf("expected 1 resource in argocd layout, got %d", len(argoCDLayout.Resources))
	}
}

func TestGenerateBootstrap_Disabled(t *testing.T) {
	engine := Engine()
	config := &stack.BootstrapConfig{Enabled: false}
	node := &stack.Node{}
	
	objs, err := engine.GenerateBootstrap(config, node)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if objs != nil {
		t.Error("expected nil objects for disabled bootstrap")
	}
}

func TestGenerateBootstrap_NilConfig(t *testing.T) {
	engine := Engine()
	node := &stack.Node{}
	
	objs, err := engine.GenerateBootstrap(nil, node)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if objs != nil {
		t.Error("expected nil objects for nil config")
	}
}

func TestGenerateBootstrap_Enabled(t *testing.T) {
	engine := Engine()
	config := &stack.BootstrapConfig{Enabled: true}
	node := &stack.Node{}
	
	objs, err := engine.GenerateBootstrap(config, node)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	
	// Current implementation returns empty slice
	if objs == nil {
		t.Error("expected non-nil objects slice")
	}
	if len(objs) != 0 {
		t.Errorf("expected 0 objects (mock implementation), got %d", len(objs))
	}
}

func TestBundlePath(t *testing.T) {
	engine := Engine()
	
	tests := []struct {
		name         string
		bundle       *stack.Bundle
		expectedPath string
	}{
		{
			name:         "simple bundle",
			bundle:       &stack.Bundle{Name: "app"},
			expectedPath: "app",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := engine.bundlePath(tt.bundle)
			if path != tt.expectedPath {
				t.Errorf("expected path '%s', got '%s'", tt.expectedPath, path)
			}
		})
	}
}

func TestBundlePath_EmptyName(t *testing.T) {
	engine := Engine()
	
	// Test empty name handling
	emptyBundle := &stack.Bundle{Name: ""}
	path := engine.bundlePath(emptyBundle)
	if path != "" {
		t.Errorf("expected empty path for empty name, got '%s'", path)
	}
}

func TestWorkflowEngineImplementsInterfaces(t *testing.T) {
	engine := Engine()
	
	// Test type assertions for main interface
	var _ stack.Workflow = engine
}

func TestGenerateFromBundle_ClientObjectInterface(t *testing.T) {
	engine := Engine()
	bundle := &stack.Bundle{Name: "test"}
	
	objs, err := engine.GenerateFromBundle(bundle)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	
	if len(objs) != 1 {
		t.Errorf("expected 1 object, got %d", len(objs))
		return
	}
	
	// Verify it implements client.Object
	obj := objs[0]
	if obj == nil {
		t.Error("object should not be nil")
		return
	}
	
	// Test client.Object methods
	if obj.GetName() != "test" {
		t.Errorf("expected name 'test', got %s", obj.GetName())
	}
	
	// Test that it's an unstructured object
	if unstructuredObj, ok := obj.(*unstructured.Unstructured); ok {
		if unstructuredObj.GetKind() != "Application" {
			t.Errorf("expected kind 'Application', got %s", unstructuredObj.GetKind())
		}
	} else {
		t.Error("expected object to be *unstructured.Unstructured")
	}
	
	// Test DeepCopyObject
	copied := obj.DeepCopyObject()
	if copied == nil {
		t.Error("DeepCopyObject should not return nil")
	}
	
	// Type assert back to client.Object to access GetName
	if copiedClientObj, ok := copied.(interface{ GetName() string }); ok {
		if copiedClientObj.GetName() != obj.GetName() {
			t.Error("copied object should have same name as original")
		}
	} else {
		t.Error("copied object should implement GetName method")
	}
}