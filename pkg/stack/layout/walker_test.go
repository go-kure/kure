package layout_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// fakeConfig implements stack.ApplicationConfig for testing purposes.
type fakeConfig struct {
	objs []*client.Object
	err  error
}

func (f *fakeConfig) Generate(*stack.Application) ([]*client.Object, error) {
	return f.objs, f.err
}

// fakeAugmentingConfig implements both stack.ApplicationConfig and
// layout.LayoutAugmenter. It records the layout it was called with.
// extraFileName and cmgName default to "values.yaml" and "augmented-values";
// override per-instance to verify multi-app no-collision in shared bundles.
type fakeAugmentingConfig struct {
	objs          []*client.Object
	augmentErr    error
	called        *layout.ManifestLayout
	extraFileName string
	cmgName       string
}

func (f *fakeAugmentingConfig) Generate(*stack.Application) ([]*client.Object, error) {
	return f.objs, nil
}

func (f *fakeAugmentingConfig) AugmentLayout(ml *layout.ManifestLayout) error {
	f.called = ml
	if f.augmentErr != nil {
		return f.augmentErr
	}
	name := f.extraFileName
	if name == "" {
		name = "values.yaml"
	}
	cmg := f.cmgName
	if cmg == "" {
		cmg = "augmented-values"
	}
	ml.ExtraFiles = append(ml.ExtraFiles, layout.ExtraFile{Name: name, Content: []byte("k: v\n")})
	ml.ConfigMapGenerators = append(ml.ConfigMapGenerators, layout.ConfigMapGeneratorSpec{
		Name:  cmg,
		Files: []string{name},
	})
	return nil
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
	node.SetParent(root)
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
	node.SetParent(root)
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

func TestWalkClusterFlatRoot(t *testing.T) {
	obj1 := &unstructured.Unstructured{}
	obj1.SetAPIVersion("v1")
	obj1.SetKind("ConfigMap")
	obj1.SetName("cm1")
	obj1.SetNamespace("default")
	var o1 client.Object = obj1

	obj2 := &unstructured.Unstructured{}
	obj2.SetAPIVersion("v1")
	obj2.SetKind("Secret")
	obj2.SetName("sec1")
	obj2.SetNamespace("default")
	var o2 client.Object = obj2

	app1 := stack.NewApplication("app1", "ns", &fakeConfig{objs: []*client.Object{&o1}})
	app2 := stack.NewApplication("app2", "ns", &fakeConfig{objs: []*client.Object{&o2}})
	bundle1 := &stack.Bundle{Name: "bundle1", Applications: []*stack.Application{app1}}
	bundle2 := &stack.Bundle{Name: "bundle2", Applications: []*stack.Application{app2}}
	node1 := &stack.Node{Name: "infra", Bundle: bundle1}
	node2 := &stack.Node{Name: "apps", Bundle: bundle2}
	root := &stack.Node{Name: "root", Children: []*stack.Node{node1, node2}}
	node1.SetParent(root)
	node2.SetParent(root)
	cluster := &stack.Cluster{Name: "demo", Node: root}

	// All flat: NodeGrouping=GroupFlat, BundleGrouping=GroupFlat, ApplicationGrouping=GroupFlat
	rules := layout.LayoutRules{
		NodeGrouping:        layout.GroupFlat,
		BundleGrouping:      layout.GroupFlat,
		ApplicationGrouping: layout.GroupFlat,
	}
	ml, err := layout.WalkCluster(cluster, rules)
	if err != nil {
		t.Fatalf("walk cluster: %v", err)
	}
	if ml == nil {
		t.Fatalf("nil layout returned")
	}

	// With flat root output, all child node resources are merged into root
	if len(ml.Children) != 0 {
		t.Fatalf("expected no children (flat root), got %d", len(ml.Children))
	}
	if len(ml.Resources) != 2 {
		t.Fatalf("expected 2 resources from both nodes, got %d", len(ml.Resources))
	}
}

func TestWalkClusterFlatRoot_DeepHierarchy(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("cm")
	obj.SetNamespace("default")
	var o client.Object = obj

	app := stack.NewApplication("app", "ns", &fakeConfig{objs: []*client.Object{&o}})
	bundle := &stack.Bundle{Name: "bundle", Applications: []*stack.Application{app}}
	grandchild := &stack.Node{Name: "grandchild", Bundle: bundle}
	child := &stack.Node{Name: "child", Children: []*stack.Node{grandchild}}
	grandchild.SetParent(child)
	root := &stack.Node{Name: "root", Children: []*stack.Node{child}}
	child.SetParent(root)
	cluster := &stack.Cluster{Name: "demo", Node: root}

	rules := layout.LayoutRules{
		NodeGrouping:        layout.GroupFlat,
		BundleGrouping:      layout.GroupFlat,
		ApplicationGrouping: layout.GroupFlat,
	}
	ml, err := layout.WalkCluster(cluster, rules)
	if err != nil {
		t.Fatalf("walk cluster: %v", err)
	}
	if ml == nil {
		t.Fatalf("nil layout returned")
	}

	// Deep hierarchy should be fully flattened
	if len(ml.Children) != 0 {
		t.Fatalf("expected no children (flat root), got %d", len(ml.Children))
	}
	if len(ml.Resources) != 1 {
		t.Fatalf("expected 1 resource from grandchild, got %d", len(ml.Resources))
	}
}

// TestWalkCluster_ClusterNameWithChildNodes verifies that when rules.ClusterName
// is set, child-node sub-layouts are nested under the root node layout (not as
// siblings of it under the cluster-level layout). The Flux integrator's
// path-based layout lookup uses stack.Node.GetPath() — e.g. "root/apps" for a
// child named "apps" of a root named "root" — so the layout tree must mirror
// that hierarchy.
func TestWalkCluster_ClusterNameWithChildNodes(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("cm")
	obj.SetNamespace("default")
	var o client.Object = obj

	app := stack.NewApplication("app", "ns", &fakeConfig{objs: []*client.Object{&o}})
	appsBundle := &stack.Bundle{Name: "apps-bundle", Applications: []*stack.Application{app}}
	appsNode := &stack.Node{Name: "apps", Bundle: appsBundle}
	rootBundle := &stack.Bundle{Name: "root-bundle"}
	rootNode := &stack.Node{Name: "flux-system", Bundle: rootBundle, Children: []*stack.Node{appsNode}}
	appsNode.SetParent(rootNode)
	cluster := &stack.Cluster{Name: "demo", Node: rootNode}

	rules := layout.DefaultLayoutRules()
	rules.ClusterName = "demo"
	ml, err := layout.WalkCluster(cluster, rules)
	if err != nil {
		t.Fatalf("walk cluster: %v", err)
	}
	if ml == nil {
		t.Fatalf("nil layout returned")
	}

	// The cluster layout should contain exactly the root node layout —
	// child nodes must NOT appear as siblings here.
	if len(ml.Children) != 1 {
		t.Fatalf("cluster layout should have exactly 1 child (root node), got %d", len(ml.Children))
	}
	rootLayout := ml.Children[0]
	if rootLayout.Name != "flux-system" {
		t.Fatalf("expected root layout name %q, got %q", "flux-system", rootLayout.Name)
	}
	if rootLayout.Namespace != "demo/flux-system" {
		t.Fatalf("expected root layout namespace %q, got %q", "demo/flux-system", rootLayout.Namespace)
	}

	// The child node layout must be nested under rootLayout, not under
	// clusterLayout — this is what the fix enforces.
	if len(rootLayout.Children) != 1 {
		t.Fatalf("root layout should have 1 child (apps node), got %d", len(rootLayout.Children))
	}
	appsLayout := rootLayout.Children[0]
	if appsLayout.Name != "apps" {
		t.Fatalf("expected apps layout name %q, got %q", "apps", appsLayout.Name)
	}
	if appsLayout.Namespace != "demo/flux-system" {
		t.Fatalf("expected apps layout namespace %q, got %q", "demo/flux-system", appsLayout.Namespace)
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
		Version: "v1",
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
	node1.SetParent(root)
	node2.SetParent(root)
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
		Version: "v1",
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
	childNode.SetParent(parentNode)

	root := &stack.Node{Name: "root", Children: []*stack.Node{parentNode}}
	parentNode.SetParent(root)
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
	node.SetParent(root)
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

func TestWalkCluster_InvalidUmbrellaRejected(t *testing.T) {
	// Shared pointer is both a child node Bundle and an umbrella child —
	// ValidateCluster must reject.
	shared := &stack.Bundle{Name: "shared"}
	root := &stack.Node{
		Name:   "root",
		Bundle: &stack.Bundle{Name: "root", Children: []*stack.Bundle{shared}},
		Children: []*stack.Node{
			{Name: "child", Bundle: shared},
		},
	}
	c := &stack.Cluster{Name: "c", Node: root}

	if _, err := layout.WalkCluster(c, layout.LayoutRules{}); err == nil {
		t.Fatal("expected invalid umbrella cluster to be rejected by WalkCluster")
	}
}

func TestWalkClusterByPackage_InvalidUmbrellaRejected(t *testing.T) {
	shared := &stack.Bundle{Name: "shared"}
	root := &stack.Node{
		Name:   "root",
		Bundle: &stack.Bundle{Name: "root", Children: []*stack.Bundle{shared}},
		Children: []*stack.Node{
			{Name: "child", Bundle: shared},
		},
	}
	c := &stack.Cluster{Name: "c", Node: root}

	if _, err := layout.WalkClusterByPackage(c, layout.LayoutRules{}); err == nil {
		t.Fatal("expected invalid umbrella cluster to be rejected by WalkClusterByPackage")
	}
}

// umbrellaObj returns a single-object ConfigMap the fakeConfig can emit.
func umbrellaObj(name string) client.Object {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("v1")
	u.SetKind("ConfigMap")
	u.SetName(name)
	u.SetNamespace("default")
	return u
}

// makeUmbrellaApp builds a stack.Application whose Generate() returns the
// given ConfigMap wrapped in the single-object slice the layout walker expects.
func makeUmbrellaApp(appName, cmName string) *stack.Application {
	o := umbrellaObj(cmName)
	return stack.NewApplication(appName, "ns", &fakeConfig{objs: []*client.Object{&o}})
}

func TestWalkCluster_Umbrella_NonNodeOnly(t *testing.T) {
	// In non-nodeOnly mode, umbrella children are siblings of application
	// sub-layouts within the bundle layout.
	parentApp := makeUmbrellaApp("parent-app", "cm-parent")
	childApp := makeUmbrellaApp("child-app", "cm-child")

	childBundle := &stack.Bundle{
		Name:         "leaf",
		Applications: []*stack.Application{childApp},
	}
	umbrella := &stack.Bundle{
		Name:         "platform",
		Applications: []*stack.Application{parentApp},
		Children:     []*stack.Bundle{childBundle},
	}
	node := &stack.Node{Name: "apps", Bundle: umbrella}
	root := &stack.Node{Name: "root", Children: []*stack.Node{node}}
	node.SetParent(root)
	cluster := &stack.Cluster{Name: "demo", Node: root}

	ml, err := layout.WalkCluster(cluster, layout.LayoutRules{
		BundleGrouping:      layout.GroupByName,
		ApplicationGrouping: layout.GroupByName,
	})
	if err != nil {
		t.Fatalf("walk cluster: %v", err)
	}

	// root -> apps -> platform bundle
	if len(ml.Children) != 1 {
		t.Fatalf("expected 1 node child, got %d", len(ml.Children))
	}
	nodeLayout := ml.Children[0]
	if len(nodeLayout.Children) != 1 {
		t.Fatalf("expected 1 bundle child, got %d", len(nodeLayout.Children))
	}
	bundleLayout := nodeLayout.Children[0]
	if bundleLayout.Name != "platform" {
		t.Fatalf("expected bundle name platform, got %q", bundleLayout.Name)
	}

	// bundle layout should contain parent-app (application) AND leaf (umbrella child)
	if len(bundleLayout.Children) != 2 {
		t.Fatalf("expected 2 children (parent-app + leaf), got %d", len(bundleLayout.Children))
	}

	var umbrellaChildLayout *layout.ManifestLayout
	var appLayout *layout.ManifestLayout
	for _, c := range bundleLayout.Children {
		if c.UmbrellaChild {
			umbrellaChildLayout = c
		} else {
			appLayout = c
		}
	}
	if umbrellaChildLayout == nil {
		t.Fatal("expected an UmbrellaChild sub-layout under the bundle")
	}
	if appLayout == nil {
		t.Fatal("expected a non-UmbrellaChild application sub-layout under the bundle")
	}
	if umbrellaChildLayout.Name != "leaf" {
		t.Errorf("umbrella child Name = %q, want %q", umbrellaChildLayout.Name, "leaf")
	}
	if umbrellaChildLayout.Namespace != "root/apps/platform" {
		t.Errorf("umbrella child Namespace = %q, want root/apps/platform", umbrellaChildLayout.Namespace)
	}
	// Child workload should live in the umbrella child's Resources.
	if len(umbrellaChildLayout.Resources) != 1 {
		t.Errorf("expected 1 resource in umbrella child layout, got %d", len(umbrellaChildLayout.Resources))
	}
}

func TestWalkCluster_Umbrella_NodeOnly(t *testing.T) {
	// In nodeOnly mode (GroupFlat everywhere), umbrella child sub-layouts
	// hang directly off the node layout with no intermediate bundle layer.
	parentApp := makeUmbrellaApp("parent-app", "cm-parent")
	childApp := makeUmbrellaApp("child-app", "cm-child")

	childBundle := &stack.Bundle{
		Name:         "leaf",
		Applications: []*stack.Application{childApp},
	}
	umbrella := &stack.Bundle{
		Name:         "platform",
		Applications: []*stack.Application{parentApp},
		Children:     []*stack.Bundle{childBundle},
	}
	node := &stack.Node{Name: "apps", Bundle: umbrella}
	root := &stack.Node{Name: "root", Children: []*stack.Node{node}}
	node.SetParent(root)
	cluster := &stack.Cluster{Name: "demo", Node: root}

	rules := layout.LayoutRules{
		BundleGrouping:      layout.GroupFlat,
		ApplicationGrouping: layout.GroupFlat,
	}
	ml, err := layout.WalkCluster(cluster, rules)
	if err != nil {
		t.Fatalf("walk cluster: %v", err)
	}

	if len(ml.Children) != 1 {
		t.Fatalf("expected 1 node child, got %d", len(ml.Children))
	}
	nodeLayout := ml.Children[0]

	// Node layout carries the parent app's resources directly (nodeOnly).
	if len(nodeLayout.Resources) != 1 {
		t.Errorf("expected 1 resource on node layout, got %d", len(nodeLayout.Resources))
	}

	// Umbrella child hangs directly off the node layout.
	if len(nodeLayout.Children) != 1 {
		t.Fatalf("expected 1 umbrella child, got %d", len(nodeLayout.Children))
	}
	uc := nodeLayout.Children[0]
	if !uc.UmbrellaChild {
		t.Error("expected UmbrellaChild=true on nodeOnly umbrella sub-layout")
	}
	if uc.Name != "leaf" {
		t.Errorf("umbrella child Name = %q, want leaf", uc.Name)
	}
	// In nodeOnly mode, the umbrella child sits under the node path (no bundle layer).
	if uc.Namespace != "root/apps" {
		t.Errorf("umbrella child Namespace = %q, want root/apps", uc.Namespace)
	}
}

func TestWalkCluster_Umbrella_Nested(t *testing.T) {
	grandchildApp := makeUmbrellaApp("gc-app", "cm-gc")
	grandchild := &stack.Bundle{
		Name:         "networking",
		Applications: []*stack.Application{grandchildApp},
	}
	child := &stack.Bundle{
		Name:     "infra",
		Children: []*stack.Bundle{grandchild},
	}
	umbrella := &stack.Bundle{
		Name:     "platform",
		Children: []*stack.Bundle{child},
	}
	node := &stack.Node{Name: "apps", Bundle: umbrella}
	root := &stack.Node{Name: "root", Children: []*stack.Node{node}}
	node.SetParent(root)
	cluster := &stack.Cluster{Name: "demo", Node: root}

	ml, err := layout.WalkCluster(cluster, layout.LayoutRules{
		BundleGrouping:      layout.GroupByName,
		ApplicationGrouping: layout.GroupByName,
	})
	if err != nil {
		t.Fatalf("walk cluster: %v", err)
	}

	// root -> apps -> platform -> infra (UmbrellaChild) -> networking (UmbrellaChild)
	bundleLayout := ml.Children[0].Children[0]
	if bundleLayout.Name != "platform" {
		t.Fatalf("expected platform bundle, got %q", bundleLayout.Name)
	}
	if len(bundleLayout.Children) != 1 {
		t.Fatalf("expected 1 umbrella child, got %d", len(bundleLayout.Children))
	}
	infra := bundleLayout.Children[0]
	if !infra.UmbrellaChild || infra.Name != "infra" {
		t.Fatalf("expected infra as UmbrellaChild, got name=%q UmbrellaChild=%v", infra.Name, infra.UmbrellaChild)
	}
	if len(infra.Children) != 1 {
		t.Fatalf("expected 1 nested umbrella child, got %d", len(infra.Children))
	}
	nw := infra.Children[0]
	if !nw.UmbrellaChild || nw.Name != "networking" {
		t.Fatalf("expected networking as nested UmbrellaChild, got name=%q UmbrellaChild=%v", nw.Name, nw.UmbrellaChild)
	}
	if nw.Namespace != "root/apps/platform/infra" {
		t.Errorf("nested umbrella Namespace = %q, want root/apps/platform/infra", nw.Namespace)
	}
}

func TestWalkClusterByPackage_PropagatesFileNaming(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("cm")
	obj.SetNamespace("default")
	var o client.Object = obj

	app := stack.NewApplication("app1", "ns", &fakeConfig{objs: []*client.Object{&o}})
	bundle := &stack.Bundle{Name: "bundle1", Applications: []*stack.Application{app}}
	root := &stack.Node{Name: "root", Bundle: bundle}
	cluster := &stack.Cluster{Name: "demo", Node: root}

	packages, err := layout.WalkClusterByPackage(cluster, layout.LayoutRules{
		FileNaming: layout.FileNamingKindName,
	})
	if err != nil {
		t.Fatalf("walk cluster by package: %v", err)
	}

	for _, ml := range packages {
		if ml.FileNaming != layout.FileNamingKindName {
			t.Errorf("root layout FileNaming = %q, want %q", ml.FileNaming, layout.FileNamingKindName)
		}
		// Check children recursively
		var check func(l *layout.ManifestLayout)
		check = func(l *layout.ManifestLayout) {
			if l.FileNaming != layout.FileNamingKindName {
				t.Errorf("layout %q FileNaming = %q, want %q", l.Name, l.FileNaming, layout.FileNamingKindName)
			}
			for _, c := range l.Children {
				check(c)
			}
		}
		check(ml)
	}
}

func TestWalkCluster_LayoutAugmenter(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("cm")
	obj.SetNamespace("default")
	var o client.Object = obj

	cfg := &fakeAugmentingConfig{objs: []*client.Object{&o}}
	app := stack.NewApplication("app", "ns", cfg)
	bundle := &stack.Bundle{Name: "bundle", Applications: []*stack.Application{app}}
	root := &stack.Node{Name: "root", Bundle: bundle}
	cluster := &stack.Cluster{Name: "demo", Node: root}

	ml, err := layout.WalkCluster(cluster, layout.LayoutRules{
		BundleGrouping:      layout.GroupByName,
		ApplicationGrouping: layout.GroupByName,
	})
	if err != nil {
		t.Fatalf("walk cluster: %v", err)
	}

	bundleLayout := ml.Children[0]
	if len(bundleLayout.Children) != 1 {
		t.Fatalf("expected one app layout, got %d", len(bundleLayout.Children))
	}
	appLayout := bundleLayout.Children[0]
	if cfg.called != appLayout {
		t.Errorf("AugmentLayout was not called with the per-app layout")
	}
	if len(appLayout.ExtraFiles) != 1 || appLayout.ExtraFiles[0].Name != "values.yaml" {
		t.Errorf("expected ExtraFiles attached, got %+v", appLayout.ExtraFiles)
	}
	if len(appLayout.ConfigMapGenerators) != 1 || appLayout.ConfigMapGenerators[0].Name != "augmented-values" {
		t.Errorf("expected ConfigMapGenerators attached, got %+v", appLayout.ConfigMapGenerators)
	}
}

func TestWalkCluster_LayoutAugmenter_Error(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("cm")
	obj.SetNamespace("default")
	var o client.Object = obj

	wantErr := errors.New("augment failed")
	cfg := &fakeAugmentingConfig{objs: []*client.Object{&o}, augmentErr: wantErr}
	app := stack.NewApplication("app", "ns", cfg)
	bundle := &stack.Bundle{Name: "bundle", Applications: []*stack.Application{app}}
	root := &stack.Node{Name: "root", Bundle: bundle}
	cluster := &stack.Cluster{Name: "demo", Node: root}

	_, err := layout.WalkCluster(cluster, layout.LayoutRules{
		BundleGrouping:      layout.GroupByName,
		ApplicationGrouping: layout.GroupByName,
	})
	if err == nil {
		t.Fatalf("expected error from augmenter, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wrapped wantErr, got: %v", err)
	}
}

// makeCM returns a client.Object pointer for a ConfigMap with the given name.
// Test helper used by the app-scoped augmentation cases below.
func makeCM(name string) *client.Object {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("v1")
	u.SetKind("ConfigMap")
	u.SetName(name)
	u.SetNamespace("default")
	var o client.Object = u
	return &o
}

// TestWalkCluster_NodeOnly_SingleAugmenter verifies that a single augmenter
// app in a flat (nodeOnly) bundle gets its own per-app sub-layout with the
// augmenter's ExtraFile + ConfigMapGenerator attached. Before app-scoped
// augmentation, the augmenter was never invoked on this path.
func TestWalkCluster_NodeOnly_SingleAugmenter(t *testing.T) {
	cfg := &fakeAugmentingConfig{objs: []*client.Object{makeCM("a")}}
	app := stack.NewApplication("a", "ns", cfg)
	bundle := &stack.Bundle{Name: "bundle", Applications: []*stack.Application{app}}
	node := &stack.Node{Name: "apps", Bundle: bundle}
	root := &stack.Node{Name: "root", Children: []*stack.Node{node}}
	node.SetParent(root)
	cluster := &stack.Cluster{Name: "demo", Node: root}

	ml, err := layout.WalkCluster(cluster, layout.DefaultLayoutRules())
	if err != nil {
		t.Fatalf("walk cluster: %v", err)
	}
	// root/apps node ML is at ml.Children[0]
	if len(ml.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(ml.Children))
	}
	nodeML := ml.Children[0]
	if len(nodeML.Children) != 1 {
		t.Fatalf("expected one per-app sub-layout under nodeML, got %d", len(nodeML.Children))
	}
	appLayout := nodeML.Children[0]
	if appLayout.Name != "a" {
		t.Errorf("appLayout.Name = %q, want %q", appLayout.Name, "a")
	}
	if cfg.called != appLayout {
		t.Errorf("AugmentLayout was not called with the per-app layout")
	}
	if len(appLayout.ExtraFiles) != 1 || appLayout.ExtraFiles[0].Name != "values.yaml" {
		t.Errorf("ExtraFiles missing or wrong: %+v", appLayout.ExtraFiles)
	}
	if len(appLayout.ConfigMapGenerators) != 1 || appLayout.ConfigMapGenerators[0].Name != "augmented-values" {
		t.Errorf("CMG missing or wrong: %+v", appLayout.ConfigMapGenerators)
	}
}

// TestWalkCluster_NodeOnly_MultiAugmenter verifies that two augmenter apps in
// the same flat bundle get separate sub-layouts so their ExtraFiles and
// ConfigMapGenerators do not collide. A shared-ML augmenter call would have
// caused the second app's values.yaml to overwrite the first.
func TestWalkCluster_NodeOnly_MultiAugmenter(t *testing.T) {
	cfgA := &fakeAugmentingConfig{
		objs:          []*client.Object{makeCM("a")},
		extraFileName: "a-values.yaml",
		cmgName:       "a-values",
	}
	cfgB := &fakeAugmentingConfig{
		objs:          []*client.Object{makeCM("b")},
		extraFileName: "b-values.yaml",
		cmgName:       "b-values",
	}
	appA := stack.NewApplication("a", "ns", cfgA)
	appB := stack.NewApplication("b", "ns", cfgB)
	bundle := &stack.Bundle{Name: "bundle", Applications: []*stack.Application{appA, appB}}
	node := &stack.Node{Name: "apps", Bundle: bundle}
	root := &stack.Node{Name: "root", Children: []*stack.Node{node}}
	node.SetParent(root)
	cluster := &stack.Cluster{Name: "demo", Node: root}

	ml, err := layout.WalkCluster(cluster, layout.DefaultLayoutRules())
	if err != nil {
		t.Fatalf("walk cluster: %v", err)
	}
	nodeML := ml.Children[0]
	if len(nodeML.Children) != 2 {
		t.Fatalf("expected 2 per-app sub-layouts, got %d", len(nodeML.Children))
	}
	byName := map[string]*layout.ManifestLayout{}
	for _, c := range nodeML.Children {
		byName[c.Name] = c
	}
	for _, name := range []string{"a", "b"} {
		c, ok := byName[name]
		if !ok {
			t.Fatalf("missing sub-layout for app %q", name)
		}
		if len(c.ExtraFiles) != 1 {
			t.Errorf("app %q: ExtraFiles len = %d, want 1", name, len(c.ExtraFiles))
		}
		wantFile := name + "-values.yaml"
		if got := c.ExtraFiles[0].Name; got != wantFile {
			t.Errorf("app %q: ExtraFile name = %q, want %q", name, got, wantFile)
		}
		if len(c.ConfigMapGenerators) != 1 {
			t.Errorf("app %q: CMG len = %d, want 1", name, len(c.ConfigMapGenerators))
		}
	}
}

// TestWalkCluster_NodeOnly_MixedBundle verifies that an augmenter app and a
// non-augmenter app sharing the same flat bundle co-exist correctly: the
// augmenter app gets its own sub-layout, the non-augmenter app's resources
// stay flat in the parent ml.Resources (preserving existing behavior for
// configs that don't implement LayoutAugmenter).
func TestWalkCluster_NodeOnly_MixedBundle(t *testing.T) {
	augCfg := &fakeAugmentingConfig{objs: []*client.Object{makeCM("a")}}
	plainCfg := &fakeConfig{objs: []*client.Object{makeCM("b")}}
	appA := stack.NewApplication("a", "ns", augCfg)
	appB := stack.NewApplication("b", "ns", plainCfg)
	bundle := &stack.Bundle{Name: "bundle", Applications: []*stack.Application{appA, appB}}
	node := &stack.Node{Name: "apps", Bundle: bundle}
	root := &stack.Node{Name: "root", Children: []*stack.Node{node}}
	node.SetParent(root)
	cluster := &stack.Cluster{Name: "demo", Node: root}

	ml, err := layout.WalkCluster(cluster, layout.DefaultLayoutRules())
	if err != nil {
		t.Fatalf("walk cluster: %v", err)
	}
	nodeML := ml.Children[0]
	if len(nodeML.Children) != 1 {
		t.Fatalf("expected 1 augmenter sub-layout, got %d", len(nodeML.Children))
	}
	if nodeML.Children[0].Name != "a" {
		t.Errorf("augmenter sub-layout name = %q, want %q", nodeML.Children[0].Name, "a")
	}
	if len(nodeML.Resources) != 1 {
		t.Fatalf("expected non-augmenter app's resource in nodeML.Resources, got %d", len(nodeML.Resources))
	}
	if nodeML.Resources[0].GetName() != "b" {
		t.Errorf("flat resource = %q, want %q", nodeML.Resources[0].GetName(), "b")
	}
}

// TestWalkCluster_ClusterName_UnnamedRoot_Augmenter verifies that an augmenter
// app placed in the unnamed root node (Node.Name == "") of a cluster with
// rules.ClusterName set goes through the walkClusterWithClusterName unnamed-
// root branch and still gets a per-app sub-layout.
func TestWalkCluster_ClusterName_UnnamedRoot_Augmenter(t *testing.T) {
	cfg := &fakeAugmentingConfig{objs: []*client.Object{makeCM("a")}}
	app := stack.NewApplication("a", "ns", cfg)
	bundle := &stack.Bundle{Name: "bundle", Applications: []*stack.Application{app}}
	root := &stack.Node{Name: "", Bundle: bundle}
	cluster := &stack.Cluster{Name: "demo", Node: root}

	rules := layout.DefaultLayoutRules()
	rules.ClusterName = "."
	ml, err := layout.WalkCluster(cluster, rules)
	if err != nil {
		t.Fatalf("walk cluster: %v", err)
	}
	if len(ml.Children) != 1 {
		t.Fatalf("expected 1 per-app sub-layout under synthetic root, got %d", len(ml.Children))
	}
	if cfg.called == nil {
		t.Errorf("AugmentLayout was not called")
	}
	if cfg.called != ml.Children[0] {
		t.Errorf("AugmentLayout received the wrong layout")
	}
	if len(ml.Children[0].ExtraFiles) != 1 {
		t.Errorf("ExtraFiles missing: %+v", ml.Children[0].ExtraFiles)
	}
}

// TestWalkCluster_ClusterName_UnnamedRoot_Augmenter_WriteToDisk is the
// writer-level regression for the synthetic-root case via the
// ManifestLayout.WriteToDisk method path (used by crane runtime via
// WriteToTar and by the test harness via WriteToDisk). When all apps in
// the unnamed root become augmenter-driven per-app sub-layouts, the
// synthetic root has Children but zero local Resources; the writer must
// still emit a kustomization.yaml that references those children so
// they aren't stranded on disk.
func TestWalkCluster_ClusterName_UnnamedRoot_Augmenter_WriteToDisk(t *testing.T) {
	cfgA := &fakeAugmentingConfig{
		objs:          []*client.Object{makeCM("a")},
		extraFileName: "a-values.yaml",
		cmgName:       "a-values",
	}
	cfgB := &fakeAugmentingConfig{
		objs:          []*client.Object{makeCM("b")},
		extraFileName: "b-values.yaml",
		cmgName:       "b-values",
	}
	appA := stack.NewApplication("a", "ns", cfgA)
	appB := stack.NewApplication("b", "ns", cfgB)
	bundle := &stack.Bundle{Name: "bundle", Applications: []*stack.Application{appA, appB}}
	root := &stack.Node{Name: "", Bundle: bundle}
	cluster := &stack.Cluster{Name: "demo", Node: root}

	rules := layout.DefaultLayoutRules()
	rules.ClusterName = "demo"
	ml, err := layout.WalkCluster(cluster, rules)
	if err != nil {
		t.Fatalf("walk cluster: %v", err)
	}

	dir := t.TempDir()
	if err := ml.WriteToDisk(dir); err != nil {
		t.Fatalf("WriteToDisk: %v", err)
	}

	rootKust := filepath.Join(dir, "demo", "kustomization.yaml")
	data, err := os.ReadFile(rootKust)
	if err != nil {
		t.Fatalf("synthetic-root kustomization.yaml missing: %v", err)
	}
	got := string(data)
	for _, name := range []string{"a", "b"} {
		if !strings.Contains(got, name) {
			t.Errorf("synthetic-root kustomization.yaml does not reference child %q. got:\n%s", name, got)
		}
	}
	// Each per-app sub-directory must exist on disk too.
	for _, name := range []string{"a", "b"} {
		appDir := filepath.Join(dir, "demo", name)
		if _, err := os.Stat(appDir); err != nil {
			t.Errorf("app dir for %q missing: %v", name, err)
		}
	}
}

// Note: WriteManifest (write.go) intentionally skips the synthetic-root
// kustomization.yaml when the root has no local files (see
// TestWriteManifest_ClusterRootEmptyContainerNoKustomization). That writer is
// not used by crane runtime; ManifestLayout.WriteToDisk and WriteToTar — the
// writers crane uses — always emit synthetic-root kustomization.yaml when
// children exist, which is what the *_WriteToDisk test above asserts.

// TestWalkCluster_ClusterName_NamedRoot_Augmenter verifies that an augmenter
// app in a named root node bundle goes through the walkClusterWithClusterName
// named-root branch and gets its own per-app sub-layout under the root
// layout.
func TestWalkCluster_ClusterName_NamedRoot_Augmenter(t *testing.T) {
	cfg := &fakeAugmentingConfig{objs: []*client.Object{makeCM("a")}}
	app := stack.NewApplication("a", "ns", cfg)
	bundle := &stack.Bundle{Name: "bundle", Applications: []*stack.Application{app}}
	root := &stack.Node{Name: "root", Bundle: bundle}
	cluster := &stack.Cluster{Name: "demo", Node: root}

	rules := layout.DefaultLayoutRules()
	rules.ClusterName = "demo"
	ml, err := layout.WalkCluster(cluster, rules)
	if err != nil {
		t.Fatalf("walk cluster: %v", err)
	}
	// Find the root node layout under the cluster layout.
	if len(ml.Children) != 1 {
		t.Fatalf("expected 1 child under clusterLayout, got %d", len(ml.Children))
	}
	rootML := ml.Children[0]
	if len(rootML.Children) != 1 {
		t.Fatalf("expected 1 per-app sub-layout under rootML, got %d", len(rootML.Children))
	}
	appLayout := rootML.Children[0]
	if cfg.called != appLayout {
		t.Errorf("AugmentLayout was not called with the per-app layout")
	}
	if len(appLayout.ExtraFiles) != 1 {
		t.Errorf("ExtraFiles missing: %+v", appLayout.ExtraFiles)
	}
}
