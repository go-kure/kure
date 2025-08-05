package layout_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack/layout"
	"github.com/go-kure/kure/pkg/stack"
)

// fakeConfig implements stack.ApplicationConfig for testing purposes.
type fakeConfig struct {
	objs []*client.Object
	err  error
}

func (f *fakeConfig) Generate(*stack.Application) ([]*client.Object, error) {
	return f.objs, f.err
}

func TestWalkCluster(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("cm")
	obj.SetNamespace("default")
	var o client.Object = obj

	app := stack.NewApplication("app", "ns", &fakeConfig{objs: []*client.Object{&o}})
	bundle := &stack.Bundle{Name: "bundle", Applications: []*stack.Application{app}}
	node := &stack.Node{Name: "apps", Bundle: bundle}
	root := &stack.Node{Name: "root", Children: []*stack.Node{node}}
	node.Parent = root
	cluster := &stack.Cluster{Name: "demo", Node: root}

	ml, err := layout.WalkCluster(cluster, layout.LayoutRules{
		BundleGrouping:      layout.GroupByName,
		ApplicationGrouping: layout.GroupByName,
	})
	if err != nil {
		t.Fatalf("walk cluster: %v", err)
	}
	if ml == nil {
		t.Fatalf("nil layout returned")
	}

	if len(ml.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(ml.Children))
	}
	nodeLayout := ml.Children[0]
	if nodeLayout.Name != "apps" {
		t.Fatalf("unexpected node name: %s", nodeLayout.Name)
	}
	if nodeLayout.Namespace != "root" {
		t.Fatalf("unexpected node namespace: %s", nodeLayout.Namespace)
	}
	if len(nodeLayout.Children) != 1 {
		t.Fatalf("expected bundle child, got %d children", len(nodeLayout.Children))
	}
	bundleLayout := nodeLayout.Children[0]
	if bundleLayout.Name != "bundle" {
		t.Fatalf("unexpected bundle name: %s", bundleLayout.Name)
	}
	if bundleLayout.Namespace != "root/apps" {
		t.Fatalf("unexpected bundle namespace: %s", bundleLayout.Namespace)
	}
	if len(bundleLayout.Children) != 1 {
		t.Fatalf("expected application child")
	}
	appLayout := bundleLayout.Children[0]
	if appLayout.Name != "app" {
		t.Fatalf("unexpected application name: %s", appLayout.Name)
	}
	if appLayout.Namespace != "root/apps/bundle" {
		t.Fatalf("unexpected application namespace: %s", appLayout.Namespace)
	}
	if len(appLayout.Resources) != 1 {
		t.Fatalf("expected one resource, got %d", len(appLayout.Resources))
	}
}

func TestWalkClusterNodeOnly(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("cm")
	obj.SetNamespace("default")
	var o client.Object = obj

	app := stack.NewApplication("app", "ns", &fakeConfig{objs: []*client.Object{&o}})
	bundle := &stack.Bundle{Name: "bundle", Applications: []*stack.Application{app}}
	node := &stack.Node{Name: "apps", Bundle: bundle}
	root := &stack.Node{Name: "root", Children: []*stack.Node{node}}
	node.Parent = root
	cluster := &stack.Cluster{Name: "demo", Node: root}

	rules := layout.LayoutRules{BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupFlat}
	ml, err := layout.WalkCluster(cluster, rules)
	if err != nil {
		t.Fatalf("walk cluster: %v", err)
	}
	if ml == nil {
		t.Fatalf("nil layout returned")
	}

	if len(ml.Children) != 1 {
		t.Fatalf("expected node child, got %d", len(ml.Children))
	}
	nodeLayout := ml.Children[0]
	if len(nodeLayout.Resources) != 1 {
		t.Fatalf("expected node resources, got %d", len(nodeLayout.Resources))
	}
	if len(nodeLayout.Children) != 0 {
		t.Fatalf("unexpected children: %d", len(nodeLayout.Children))
	}
	if nodeLayout.Namespace != "root" {
		t.Fatalf("unexpected namespace: %s", nodeLayout.Namespace)
	}
}

func TestWalkClusterByPackage(t *testing.T) {
	obj1 := &unstructured.Unstructured{}
	obj1.SetAPIVersion("v1")
	obj1.SetKind("ConfigMap")
	obj1.SetName("cm1")
	obj1.SetNamespace("default")
	var o1 client.Object = obj1

	obj2 := &unstructured.Unstructured{}
	obj2.SetAPIVersion("v1")
	obj2.SetKind("Secret")
	obj2.SetName("secret1")
	obj2.SetNamespace("default")
	var o2 client.Object = obj2

	// Define different package references
	ociPackageRef := &schema.GroupVersionKind{
		Group:   "source.toolkit.fluxcd.io",
		Version: "v1beta2",
		Kind:    "OCIRepository",
	}
	gitPackageRef := &schema.GroupVersionKind{
		Group:   "source.toolkit.fluxcd.io",
		Version: "v1",
		Kind:    "GitRepository",
	}

	// Create nodes with different package references
	app1 := stack.NewApplication("app1", "ns", &fakeConfig{objs: []*client.Object{&o1}})
	bundle1 := &stack.Bundle{Name: "bundle1", Applications: []*stack.Application{app1}}
	node1 := &stack.Node{Name: "apps1", Bundle: bundle1, PackageRef: ociPackageRef}

	app2 := stack.NewApplication("app2", "ns", &fakeConfig{objs: []*client.Object{&o2}})
	bundle2 := &stack.Bundle{Name: "bundle2", Applications: []*stack.Application{app2}}
	node2 := &stack.Node{Name: "apps2", Bundle: bundle2, PackageRef: gitPackageRef}

	root := &stack.Node{Name: "root", Children: []*stack.Node{node1, node2}}
	node1.Parent = root
	node2.Parent = root
	cluster := &stack.Cluster{Name: "demo", Node: root}

	packages, err := layout.WalkClusterByPackage(cluster, layout.LayoutRules{})
	if err != nil {
		t.Fatalf("walk cluster by package: %v", err)
	}

	if len(packages) != 3 {
		t.Logf("Packages found:")
		for k, v := range packages {
			t.Logf("  %s: %v", k, v != nil)
		}
		t.Fatalf("expected 3 packages (default + 2 specific), got %d", len(packages))
	}

	// Check OCI package
	ociKey := ociPackageRef.String()
	ociLayout, exists := packages[ociKey]
	if !exists {
		t.Fatalf("OCI package not found")
	}
	if ociLayout == nil {
		t.Fatalf("OCI layout is nil")
	}

	// Check Git package  
	gitKey := gitPackageRef.String()
	gitLayout, exists := packages[gitKey]
	if !exists {
		t.Fatalf("Git package not found")
	}
	if gitLayout == nil {
		t.Fatalf("Git layout is nil")
	}

	// Verify package separation - each should only contain its own resources
	if len(ociLayout.Children) != 1 {
		t.Fatalf("OCI package should have 1 child, got %d", len(ociLayout.Children))
	}
	if ociLayout.Children[0].Name != "apps1" {
		t.Fatalf("OCI package child should be 'apps1', got %s", ociLayout.Children[0].Name)
	}

	if len(gitLayout.Children) != 1 {
		t.Fatalf("Git package should have 1 child, got %d", len(gitLayout.Children))
	}
	if gitLayout.Children[0].Name != "apps2" {
		t.Fatalf("Git package child should be 'apps2', got %s", gitLayout.Children[0].Name)
	}
}

func TestWalkClusterByPackageWithInheritance(t *testing.T) {
	obj1 := &unstructured.Unstructured{}
	obj1.SetAPIVersion("v1")
	obj1.SetKind("ConfigMap")
	obj1.SetName("cm1")
	obj1.SetNamespace("default")
	var o1 client.Object = obj1

	obj2 := &unstructured.Unstructured{}
	obj2.SetAPIVersion("v1")
	obj2.SetKind("Secret")
	obj2.SetName("secret1")
	obj2.SetNamespace("default")
	var o2 client.Object = obj2

	// Define package reference
	ociPackageRef := &schema.GroupVersionKind{
		Group:   "source.toolkit.fluxcd.io",
		Version: "v1beta2",
		Kind:    "OCIRepository",
	}

	// Create hierarchy where parent has PackageRef and child inherits it
	app1 := stack.NewApplication("app1", "ns", &fakeConfig{objs: []*client.Object{&o1}})
	bundle1 := &stack.Bundle{Name: "bundle1", Applications: []*stack.Application{app1}}
	childNode := &stack.Node{Name: "child", Bundle: bundle1} // No PackageRef - should inherit

	app2 := stack.NewApplication("app2", "ns", &fakeConfig{objs: []*client.Object{&o2}})
	bundle2 := &stack.Bundle{Name: "bundle2", Applications: []*stack.Application{app2}}
	parentNode := &stack.Node{
		Name:       "parent",
		Bundle:     bundle2,
		PackageRef: ociPackageRef, // Parent has PackageRef
		Children:   []*stack.Node{childNode},
	}
	childNode.Parent = parentNode

	root := &stack.Node{Name: "root", Children: []*stack.Node{parentNode}}
	parentNode.Parent = root
	cluster := &stack.Cluster{Name: "demo", Node: root}

	packages, err := layout.WalkClusterByPackage(cluster, layout.LayoutRules{
		BundleGrouping:      layout.GroupByName,
		ApplicationGrouping: layout.GroupByName,
	})
	if err != nil {
		t.Fatalf("walk cluster by package: %v", err)
	}

	if len(packages) != 2 {
		t.Fatalf("expected 2 packages (default + OCI), got %d", len(packages))
	}

	// Both parent and child should be in the OCI package due to inheritance
	ociKey := ociPackageRef.String()
	ociLayout, exists := packages[ociKey]
	if !exists {
		t.Fatalf("OCI package not found")
	}
	if ociLayout == nil {
		t.Fatalf("OCI layout is nil")
	}

	// Should have parent node in OCI package
	if len(ociLayout.Children) != 1 {
		t.Fatalf("OCI package should have 1 child (parent), got %d", len(ociLayout.Children))
	}
	parentLayout := ociLayout.Children[0]
	if parentLayout.Name != "parent" {
		t.Fatalf("OCI package child should be 'parent', got %s", parentLayout.Name)
	}

	// Parent should have child in the same package
	if len(parentLayout.Children) != 2 { // bundle + child node
		t.Fatalf("Parent should have 2 children (bundle + child node), got %d", len(parentLayout.Children))
	}

	// Find the child node layout
	var childLayout *layout.ManifestLayout
	for _, child := range parentLayout.Children {
		if child.Name == "child" {
			childLayout = child
			break
		}
	}
	if childLayout == nil {
		t.Fatalf("Child node not found in parent's children")
	}
}

func TestWalkClusterByPackageDefaultPackage(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("cm")
	obj.SetNamespace("default")
	var o client.Object = obj

	// Create node without PackageRef - should go to default package
	app := stack.NewApplication("app", "ns", &fakeConfig{objs: []*client.Object{&o}})
	bundle := &stack.Bundle{Name: "bundle", Applications: []*stack.Application{app}}
	node := &stack.Node{Name: "apps", Bundle: bundle} // No PackageRef
	root := &stack.Node{Name: "root", Children: []*stack.Node{node}}
	node.Parent = root
	cluster := &stack.Cluster{Name: "demo", Node: root}

	packages, err := layout.WalkClusterByPackage(cluster, layout.LayoutRules{})
	if err != nil {
		t.Fatalf("walk cluster by package: %v", err)
	}

	if len(packages) != 1 {
		t.Fatalf("expected 1 package (default), got %d", len(packages))
	}

	// Should have default package
	defaultLayout, exists := packages["default"]
	if !exists {
		t.Fatalf("Default package not found")
	}
	if defaultLayout == nil {
		t.Fatalf("Default layout is nil")
	}

	if len(defaultLayout.Children) != 1 {
		t.Fatalf("Default package should have 1 child, got %d", len(defaultLayout.Children))
	}
	if defaultLayout.Children[0].Name != "apps" {
		t.Fatalf("Default package child should be 'apps', got %s", defaultLayout.Children[0].Name)
	}
}
