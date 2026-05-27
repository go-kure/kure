package fluxcd_test

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

func TestNewLayoutIntegrator(t *testing.T) {
	generator := fluxstack.NewResourceGenerator()
	integrator := fluxstack.NewLayoutIntegrator(generator)

	if integrator == nil {
		t.Fatal("expected non-nil integrator")
	}

	if integrator.Generator != generator {
		t.Error("generator not set correctly")
	}
}

func TestLayoutIntegrator_IntegrateWithLayout_NilInputs(t *testing.T) {
	generator := fluxstack.NewResourceGenerator()
	integrator := fluxstack.NewLayoutIntegrator(generator)

	// Test with nil layout
	err := integrator.IntegrateWithLayout(nil, &stack.Cluster{}, layout.LayoutRules{})
	if err != nil {
		t.Errorf("expected no error for nil layout, got %v", err)
	}

	// Test with nil cluster
	ml := &layout.ManifestLayout{}
	err = integrator.IntegrateWithLayout(ml, nil, layout.LayoutRules{})
	if err != nil {
		t.Errorf("expected no error for nil cluster, got %v", err)
	}

	// Test with both nil
	err = integrator.IntegrateWithLayout(nil, nil, layout.LayoutRules{})
	if err != nil {
		t.Errorf("expected no error for nil inputs, got %v", err)
	}
}

func TestLayoutIntegrator_IntegrateWithLayout_InvalidPlacement(t *testing.T) {
	generator := fluxstack.NewResourceGenerator()
	integrator := fluxstack.NewLayoutIntegrator(generator)

	ml := &layout.ManifestLayout{Name: "test"}
	cluster := &stack.Cluster{
		Name: "test-cluster",
		Node: &stack.Node{Name: "root"},
	}

	// Set invalid placement on the rules (the sole authority).
	rules := layout.LayoutRules{FluxPlacement: layout.FluxPlacement("invalid")}
	err := integrator.IntegrateWithLayout(ml, cluster, rules)
	if err == nil {
		t.Error("expected error for invalid placement")
	}
}

func TestLayoutIntegrator_IntegrateWithLayout_Integrated(t *testing.T) {
	generator := fluxstack.NewResourceGenerator()
	integrator := fluxstack.NewLayoutIntegrator(generator)

	// Create a simple cluster with a bundle
	bundle := &stack.Bundle{
		Name: "test-bundle",
		SourceRef: &stack.SourceRef{
			Kind:      "GitRepository",
			Name:      "test-source",
			Namespace: "flux-system",
		},
	}

	node := &stack.Node{
		Name:   "test-node",
		Bundle: bundle,
	}

	cluster := &stack.Cluster{
		Name: "test-cluster",
		Node: node,
	}

	// Create a matching layout
	ml := &layout.ManifestLayout{
		Name:      "test-node",
		Namespace: "clusters/test-cluster",
	}

	rules := layout.DefaultLayoutRules()
	rules.FluxPlacement = layout.FluxIntegrated
	err := integrator.IntegrateWithLayout(ml, cluster, rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify flux resources were added
	if len(ml.Resources) == 0 {
		t.Error("expected Flux resources to be added to layout")
	}
}

func TestLayoutIntegrator_IntegrateWithLayout_Separate(t *testing.T) {
	generator := fluxstack.NewResourceGenerator()
	integrator := fluxstack.NewLayoutIntegrator(generator)

	// Create a cluster with bundles
	bundle := &stack.Bundle{
		Name: "test-bundle",
		SourceRef: &stack.SourceRef{
			Kind:      "GitRepository",
			Name:      "test-source",
			Namespace: "flux-system",
		},
	}

	node := &stack.Node{
		Name:   "test-node",
		Bundle: bundle,
	}

	cluster := &stack.Cluster{
		Name: "test-cluster",
		Node: node,
	}

	// Create a layout
	ml := &layout.ManifestLayout{
		Name:      "test-cluster",
		Namespace: "clusters",
	}

	rules := layout.DefaultLayoutRules()
	err := integrator.IntegrateWithLayout(ml, cluster, rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify a flux-system child was added
	found := false
	for _, child := range ml.Children {
		if child.Name == "flux-system" {
			found = true
			if len(child.Resources) == 0 {
				t.Error("expected Flux resources in flux-system child")
			}
			break
		}
	}
	if !found {
		t.Error("expected flux-system child to be added")
	}
}

func TestLayoutIntegrator_CreateLayoutWithResources(t *testing.T) {
	generator := fluxstack.NewResourceGenerator()
	integrator := fluxstack.NewLayoutIntegrator(generator)

	// Test with nil cluster
	ml, err := integrator.CreateLayoutWithResources(nil, layout.LayoutRules{})
	if err != nil {
		t.Errorf("expected no error for nil cluster, got %v", err)
	}
	if ml != nil {
		t.Error("expected nil layout for nil cluster")
	}

	// Test with valid cluster
	bundle := &stack.Bundle{
		Name: "test-bundle",
		SourceRef: &stack.SourceRef{
			Kind:      "GitRepository",
			Name:      "test-source",
			Namespace: "flux-system",
		},
	}

	node := &stack.Node{
		Name:   "test-node",
		Bundle: bundle,
	}

	cluster := &stack.Cluster{
		Name: "test-cluster",
		Node: node,
	}

	rules := layout.DefaultLayoutRules()
	ml, err = integrator.CreateLayoutWithResources(cluster, rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ml == nil {
		t.Fatal("expected non-nil layout")
	}
}

func TestLayoutIntegrator_IntegrateWithNestedNodes(t *testing.T) {
	generator := fluxstack.NewResourceGenerator()
	integrator := fluxstack.NewLayoutIntegrator(generator)

	// Create a nested hierarchy
	grandchildBundle := &stack.Bundle{
		Name: "grandchild-bundle",
		SourceRef: &stack.SourceRef{
			Kind:      "GitRepository",
			Name:      "test-source",
			Namespace: "flux-system",
		},
	}

	grandchild := &stack.Node{
		Name:       "grandchild",
		ParentPath: "root/child",
		Bundle:     grandchildBundle,
	}

	childBundle := &stack.Bundle{
		Name: "child-bundle",
		SourceRef: &stack.SourceRef{
			Kind:      "GitRepository",
			Name:      "test-source",
			Namespace: "flux-system",
		},
	}

	child := &stack.Node{
		Name:       "child",
		ParentPath: "root",
		Bundle:     childBundle,
		Children:   []*stack.Node{grandchild},
	}

	rootBundle := &stack.Bundle{
		Name: "root-bundle",
		SourceRef: &stack.SourceRef{
			Kind:      "GitRepository",
			Name:      "test-source",
			Namespace: "flux-system",
		},
	}

	root := &stack.Node{
		Name:     "root",
		Bundle:   rootBundle,
		Children: []*stack.Node{child},
	}

	cluster := &stack.Cluster{
		Name: "test-cluster",
		Node: root,
	}

	// Create matching nested layout
	grandchildLayout := &layout.ManifestLayout{
		Name: "grandchild",
	}

	childLayout := &layout.ManifestLayout{
		Name:     "child",
		Children: []*layout.ManifestLayout{grandchildLayout},
	}

	rootLayout := &layout.ManifestLayout{
		Name:     "root",
		Children: []*layout.ManifestLayout{childLayout},
	}

	rules := layout.DefaultLayoutRules()
	rules.FluxPlacement = layout.FluxIntegrated
	err := integrator.IntegrateWithLayout(rootLayout, cluster, rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify resources were added at all levels
	if len(rootLayout.Resources) == 0 {
		t.Error("expected resources in root layout")
	}
	if len(childLayout.Resources) == 0 {
		t.Error("expected resources in child layout")
	}
	if len(grandchildLayout.Resources) == 0 {
		t.Error("expected resources in grandchild layout")
	}
}

func TestLayoutIntegrator_SeparateMode_EmptyCluster(t *testing.T) {
	generator := fluxstack.NewResourceGenerator()
	integrator := fluxstack.NewLayoutIntegrator(generator)

	// Create a cluster without bundles
	node := &stack.Node{
		Name: "test-node",
	}

	cluster := &stack.Cluster{
		Name: "test-cluster",
		Node: node,
	}

	ml := &layout.ManifestLayout{
		Name:      "test-cluster",
		Namespace: "clusters",
	}

	rules := layout.DefaultLayoutRules()
	err := integrator.IntegrateWithLayout(ml, cluster, rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No flux-system child should be added if there are no resources
	for _, child := range ml.Children {
		if child.Name == "flux-system" {
			t.Error("did not expect flux-system child for cluster with no bundles")
		}
	}
}

// TestCreateLayoutWithResources_ClusterNameWithChildNodes verifies that
// setting rules.ClusterName on a cluster whose root node has child nodes
// produces a layout tree whose paths match stack.Node.GetPath(), so the
// Flux integrator's path-based lookup can find each child node's layout.
// Previously walkClusterWithClusterName flattened child nodes to cluster
// siblings, causing CreateLayoutWithResources to fail with "corresponding
// layout node not found". This matches the shape of examples/demo/clusters/
// basic/cluster.yaml.
func TestCreateLayoutWithResources_ClusterNameWithChildNodes(t *testing.T) {
	rootBundle := &stack.Bundle{Name: "root-bundle", SourceRef: testSR()}
	appsBundle := &stack.Bundle{Name: "apps-bundle", SourceRef: testSR()}
	infraBundle := &stack.Bundle{Name: "infra-bundle", SourceRef: testSR()}

	appsNode := &stack.Node{Name: "apps", Bundle: appsBundle}
	infraNode := &stack.Node{Name: "infra", Bundle: infraBundle}
	rootNode := &stack.Node{
		Name:     "flux-system",
		Bundle:   rootBundle,
		Children: []*stack.Node{appsNode, infraNode},
	}
	appsNode.SetParent(rootNode)
	infraNode.SetParent(rootNode)
	cluster := &stack.Cluster{Name: "demo", Node: rootNode}

	integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())

	rules := layout.DefaultLayoutRules()
	rules.ClusterName = "demo"
	rules.FluxPlacement = layout.FluxIntegrated

	ml, err := integrator.CreateLayoutWithResources(cluster, rules)
	if err != nil {
		t.Fatalf("CreateLayoutWithResources failed: %v", err)
	}
	if ml == nil {
		t.Fatal("expected non-nil layout")
	}

	// Cluster layout contains exactly the root node layout.
	if len(ml.Children) != 1 {
		t.Fatalf("cluster layout should have 1 child (root node), got %d", len(ml.Children))
	}
	rootLayout := ml.Children[0]
	if rootLayout.Name != "flux-system" {
		t.Fatalf("expected root layout name %q, got %q", "flux-system", rootLayout.Name)
	}

	// Child nodes must be nested under the root layout.
	childNames := map[string]bool{}
	for _, c := range rootLayout.Children {
		childNames[c.Name] = true
	}
	for _, want := range []string{"apps", "infra"} {
		if !childNames[want] {
			t.Errorf("expected child node layout %q under root layout, not found", want)
		}
	}
}

func TestCreateLayoutWithResources_InvalidUmbrellaRejected(t *testing.T) {
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

	integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	if _, err := integrator.CreateLayoutWithResources(c, layout.LayoutRules{}); err == nil {
		t.Fatal("expected invalid umbrella cluster to be rejected by CreateLayoutWithResources")
	}
}

func TestCreateLayoutWithResources_UmbrellaIntegratedPlacement(t *testing.T) {
	// Integrated placement: the umbrella bundle's own Flux CR goes to the
	// node layout (existing behavior, referenced via the bundle-named
	// FluxIntegrated child reference), while each umbrella child's Flux CR
	// lands at the bundle layout (where the bundle-dir kustomization.yaml
	// references them via UmbrellaChild). Child sub-layouts carry no Flux CRs.
	umbrella := &stack.Bundle{
		Name:      "platform",
		SourceRef: testSR(),
		Children: []*stack.Bundle{
			{Name: "infra", SourceRef: testSR()},
			{Name: "services", SourceRef: testSR()},
		},
	}
	node := &stack.Node{Name: "apps", Bundle: umbrella}
	root := &stack.Node{Name: "root", Children: []*stack.Node{node}}
	node.SetParent(root)
	cluster := &stack.Cluster{Name: "demo", Node: root}

	integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())

	ml, err := integrator.CreateLayoutWithResources(cluster, layout.LayoutRules{
		BundleGrouping:      layout.GroupByName,
		ApplicationGrouping: layout.GroupByName,
		FluxPlacement:       layout.FluxIntegrated,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Walk ml to find layouts: root -> apps -> platform
	if len(ml.Children) != 1 {
		t.Fatalf("expected 1 node child, got %d", len(ml.Children))
	}
	nodeLayout := ml.Children[0]

	// Node layout carries the umbrella's own Flux CR.
	nodeKustNames := map[string]bool{}
	for _, r := range nodeLayout.Resources {
		if k, ok := r.(*kustv1.Kustomization); ok {
			nodeKustNames[k.Name] = true
		}
	}
	if !nodeKustNames["platform"] {
		t.Error("expected platform Kustomization at node layout")
	}
	if nodeKustNames["infra"] || nodeKustNames["services"] {
		t.Error("umbrella child CRs should not be at node layout")
	}

	if len(nodeLayout.Children) != 1 {
		t.Fatalf("expected 1 bundle child, got %d", len(nodeLayout.Children))
	}
	bundleLayout := nodeLayout.Children[0]
	if bundleLayout.Name != "platform" {
		t.Fatalf("expected platform bundle, got %q", bundleLayout.Name)
	}

	// Bundle layout carries the umbrella children's Flux CRs.
	bundleKustNames := map[string]bool{}
	for _, r := range bundleLayout.Resources {
		if k, ok := r.(*kustv1.Kustomization); ok {
			bundleKustNames[k.Name] = true
		}
	}
	for _, want := range []string{"infra", "services"} {
		if !bundleKustNames[want] {
			t.Errorf("missing umbrella child Kustomization %q at bundle layout", want)
		}
	}
	if bundleKustNames["platform"] {
		t.Error("umbrella self CR should NOT be at bundle layout (it lives at node layout)")
	}

	// Umbrella child sub-layouts should carry NO Flux CRs.
	for _, c := range bundleLayout.Children {
		if !c.UmbrellaChild {
			continue
		}
		for _, r := range c.Resources {
			if _, ok := r.(*kustv1.Kustomization); ok {
				t.Errorf("umbrella child %q contains Flux Kustomization CR — expected none", c.Name)
			}
		}
	}
}

func TestCreateLayoutWithResources_UmbrellaNodeOnlyPlacement(t *testing.T) {
	// In nodeOnly (GroupFlat) mode, the umbrella child Flux CRs should land
	// at the node layout directly (no intermediate bundle layer).
	umbrella := &stack.Bundle{
		Name:      "platform",
		SourceRef: testSR(),
		Children: []*stack.Bundle{
			{Name: "infra", SourceRef: testSR()},
		},
	}
	node := &stack.Node{Name: "apps", Bundle: umbrella}
	root := &stack.Node{Name: "root", Children: []*stack.Node{node}}
	node.SetParent(root)
	cluster := &stack.Cluster{Name: "demo", Node: root}

	integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())

	ml, err := integrator.CreateLayoutWithResources(cluster, layout.LayoutRules{
		BundleGrouping:      layout.GroupFlat,
		ApplicationGrouping: layout.GroupFlat,
		FluxPlacement:       layout.FluxIntegrated,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ml.Children) != 1 {
		t.Fatalf("expected 1 node child, got %d", len(ml.Children))
	}
	nodeLayout := ml.Children[0]

	// Node layout should carry umbrella self + umbrella child CRs = 2 Kustomizations
	kustCount := 0
	childNames := map[string]bool{}
	for _, r := range nodeLayout.Resources {
		if k, ok := r.(*kustv1.Kustomization); ok {
			kustCount++
			childNames[k.Name] = true
		}
	}
	if kustCount != 2 {
		t.Errorf("expected 2 Kustomizations at node layout, got %d", kustCount)
	}
	if !childNames["platform"] {
		t.Error("missing umbrella platform Kustomization")
	}
	if !childNames["infra"] {
		t.Error("missing umbrella child infra Kustomization")
	}
}

func TestCreateLayoutWithResources_UmbrellaNestedIntegratedPlacement(t *testing.T) {
	// Nested umbrellas: infra child has its own grandchild. The grandchild's
	// Flux CR should land at the infra umbrella child layout, not the
	// platform bundle layout or the grandchild sub-layout.
	umbrella := &stack.Bundle{
		Name:      "platform",
		SourceRef: testSR(),
		Children: []*stack.Bundle{
			{
				Name:      "infra",
				SourceRef: testSR(),
				Children: []*stack.Bundle{
					{Name: "networking", SourceRef: testSR()},
				},
			},
		},
	}
	node := &stack.Node{Name: "apps", Bundle: umbrella}
	root := &stack.Node{Name: "root", Children: []*stack.Node{node}}
	node.SetParent(root)
	cluster := &stack.Cluster{Name: "demo", Node: root}

	integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())

	ml, err := integrator.CreateLayoutWithResources(cluster, layout.LayoutRules{
		BundleGrouping:      layout.GroupByName,
		ApplicationGrouping: layout.GroupByName,
		FluxPlacement:       layout.FluxIntegrated,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ml -> apps(nodeLayout) -> platform(bundleLayout) -> infra(UmbrellaChild) -> networking(UmbrellaChild)
	// Platform's own CR lives at the node layout (via GenerateFromBundle).
	// The platform bundle layout carries only the direct umbrella children (infra).
	nodeLayout := ml.Children[0]
	nodeKustNames := map[string]bool{}
	for _, r := range nodeLayout.Resources {
		if k, ok := r.(*kustv1.Kustomization); ok {
			nodeKustNames[k.Name] = true
		}
	}
	if !nodeKustNames["platform"] {
		t.Error("expected platform Kustomization at node layout")
	}

	bundleLayout := nodeLayout.Children[0]
	if bundleLayout.Name != "platform" {
		t.Fatalf("expected platform bundle layout, got %q", bundleLayout.Name)
	}
	bundleKustNames := map[string]bool{}
	for _, r := range bundleLayout.Resources {
		if k, ok := r.(*kustv1.Kustomization); ok {
			bundleKustNames[k.Name] = true
		}
	}
	if !bundleKustNames["infra"] {
		t.Error("expected infra Kustomization at platform bundle layout")
	}
	if bundleKustNames["platform"] {
		t.Error("umbrella self CR should NOT be at bundle layout")
	}
	if bundleKustNames["networking"] {
		t.Error("nested grandchild CR should NOT be at platform bundle layout; it belongs at infra")
	}

	// infra umbrella child layout should have the networking Kustomization = 1
	var infra *layout.ManifestLayout
	for _, c := range bundleLayout.Children {
		if c.UmbrellaChild && c.Name == "infra" {
			infra = c
		}
	}
	if infra == nil {
		t.Fatal("missing infra umbrella child layout")
	}
	infraCount := 0
	for _, r := range infra.Resources {
		if k, ok := r.(*kustv1.Kustomization); ok {
			infraCount++
			if k.Name != "networking" {
				t.Errorf("unexpected Kustomization at infra layout: %q", k.Name)
			}
		}
	}
	if infraCount != 1 {
		t.Errorf("expected 1 Kustomization at infra layout, got %d", infraCount)
	}
}

func TestCreateLayoutWithResources_UmbrellaChildWithSource(t *testing.T) {
	// When an umbrella child has a SourceRef with URL, the Source CR should
	// be placed at the parent layout alongside the child Kustomization.
	umbrella := &stack.Bundle{
		Name:      "platform",
		SourceRef: testSR(),
		Children: []*stack.Bundle{
			{
				Name: "ext",
				SourceRef: &stack.SourceRef{
					Kind:   "GitRepository",
					Name:   "ext-repo",
					URL:    "https://github.com/example/ext",
					Branch: "main",
				},
			},
		},
	}
	node := &stack.Node{Name: "apps", Bundle: umbrella}
	root := &stack.Node{Name: "root", Children: []*stack.Node{node}}
	node.SetParent(root)
	cluster := &stack.Cluster{Name: "demo", Node: root}

	integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())

	ml, err := integrator.CreateLayoutWithResources(cluster, layout.LayoutRules{
		BundleGrouping:      layout.GroupByName,
		ApplicationGrouping: layout.GroupByName,
		FluxPlacement:       layout.FluxIntegrated,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bundleLayout := ml.Children[0].Children[0]
	sawSource := false
	for _, r := range bundleLayout.Resources {
		if _, ok := r.(*sourcev1.GitRepository); ok {
			sawSource = true
		}
	}
	if !sawSource {
		t.Error("expected umbrella child's GitRepository at parent bundle layout")
	}
}

// TestIntegrateWithLayout_AppliesFlattenPathRewrites confirms that
// IntegrateWithLayout invokes layout.ApplyFlattenPathRewrites before
// returning, so callers using WalkCluster + IntegrateWithLayout directly
// (without going through CreateLayoutWithResources) still get rewritten
// Spec.Path values on Flux Kustomization CRs.
func TestIntegrateWithLayout_AppliesFlattenPathRewrites(t *testing.T) {
	generator := fluxstack.NewResourceGenerator()
	integrator := fluxstack.NewLayoutIntegrator(generator)

	cluster := &stack.Cluster{
		Name: "arc-runners",
		Node: &stack.Node{Name: "apps", Bundle: &stack.Bundle{Name: "bundle"}},
	}

	// Walk + collapse via the public API; this populates flattenInfo on
	// the absorbing root.
	walked, err := layout.WalkCluster(cluster, layout.LayoutRules{
		ClusterName:         "arc-runners",
		BundleGrouping:      layout.GroupFlat,
		ApplicationGrouping: layout.GroupFlat,
		FlattenSingleTier:   true,
	})
	if err != nil {
		t.Fatalf("WalkCluster: %v", err)
	}

	// Pre-plant a Kustomization CR whose Spec.Path matches the recorded
	// rewrite. After IntegrateWithLayout returns, the post-pass must have
	// rewritten it.
	preplanted := &kustv1.Kustomization{}
	preplanted.Spec.Path = "arc-runners/apps"
	walked.Resources = append(walked.Resources, preplanted)

	if err := integrator.IntegrateWithLayout(walked, cluster, layout.LayoutRules{}); err != nil {
		t.Fatalf("IntegrateWithLayout: %v", err)
	}

	if preplanted.Spec.Path != "arc-runners" {
		t.Errorf("expected Spec.Path rewritten to 'arc-runners', got %q", preplanted.Spec.Path)
	}
}

// TestIntegrateWithLayout_RepeatedCallSucceeds confirms that calling
// IntegrateWithLayout twice on the same flattened layout works. The first
// call must not destroy the alias state that integrated placement depends
// on for resolving the collapsed node path on the second call.
func TestIntegrateWithLayout_RepeatedCallSucceeds(t *testing.T) {
	generator := fluxstack.NewResourceGenerator()
	integrator := fluxstack.NewLayoutIntegrator(generator)
	integratedRules := layout.LayoutRules{FluxPlacement: layout.FluxIntegrated}

	cluster := &stack.Cluster{
		Name: "arc-runners",
		Node: &stack.Node{
			Name: "apps",
			Bundle: &stack.Bundle{
				Name: "bundle",
				SourceRef: &stack.SourceRef{
					Kind:      "GitRepository",
					Name:      "test-source",
					Namespace: "flux-system",
				},
			},
		},
	}

	walked, err := layout.WalkCluster(cluster, layout.LayoutRules{
		ClusterName:         "arc-runners",
		BundleGrouping:      layout.GroupFlat,
		ApplicationGrouping: layout.GroupFlat,
		FlattenSingleTier:   true,
	})
	if err != nil {
		t.Fatalf("WalkCluster: %v", err)
	}

	if err := integrator.IntegrateWithLayout(walked, cluster, integratedRules); err != nil {
		t.Fatalf("first IntegrateWithLayout: %v", err)
	}

	// Second call must still resolve the collapsed "apps" node via the
	// alias fallback rather than failing with "layout node not found".
	if err := integrator.IntegrateWithLayout(walked, cluster, integratedRules); err != nil {
		t.Fatalf("second IntegrateWithLayout: %v (alias state was destroyed by the first pass)", err)
	}
}

// ---------------------------------------------------------------------------
// Augmenter child layout CR generation (#571)
// ---------------------------------------------------------------------------

// TestAugmenterChildrenGetFluxCRs verifies that:
//
//	(a) direct app-layout children of the node layout receive CRs in
//	    nodeLayout.Resources (flat/nodeOnly path), and
//	(b) augmenter sub-layouts (children of app layouts) receive CRs in
//	    app.Resources with correct DependsOn wiring.
func TestAugmenterChildrenGetFluxCRs(t *testing.T) {
	root, nodeLayout, app, preInstall, hooks, cluster := buildAugmenterTestTree()

	li := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	rules := layout.LayoutRules{FluxPlacement: layout.FluxIntegrated}
	if err := li.IntegrateWithLayout(root, cluster, rules); err != nil {
		t.Fatalf("IntegrateWithLayout: %v", err)
	}

	// (a) nodeLayout.Resources must contain a CR for the direct child "myapp".
	if !augHasCR(nodeLayout.Resources, "myapp") {
		t.Error("expected CR for 'myapp' in nodeLayout.Resources (flat path)")
	}

	// (b) app.Resources must contain CRs for both augmenter sub-layouts.
	if len(app.Resources) != 2 {
		t.Fatalf("expected 2 CRs in app.Resources, got %d", len(app.Resources))
	}
	k0 := augMustKustomization(t, app.Resources[0])
	if k0.Name != preInstall.Name {
		t.Errorf("unexpected first CR name: got %q, want %q", k0.Name, preInstall.Name)
	}
	if len(k0.Spec.DependsOn) != 0 {
		t.Errorf("expected no dependsOn on pre-install CR, got %v", k0.Spec.DependsOn)
	}
	k1 := augMustKustomization(t, app.Resources[1])
	if k1.Name != hooks.Name {
		t.Errorf("unexpected second CR name: got %q, want %q", k1.Name, hooks.Name)
	}
	if len(k1.Spec.DependsOn) != 1 || k1.Spec.DependsOn[0].Name != preInstall.Name {
		t.Errorf("unexpected dependsOn on hooks CR: %v", k1.Spec.DependsOn)
	}
	// spec.path must equal FullRepoPath().
	if k0.Spec.Path != preInstall.FullRepoPath() {
		t.Errorf("spec.path: got %q, want %q", k0.Spec.Path, preInstall.FullRepoPath())
	}
}

// TestAugmenterChildrenNoDuplicateInKustomizationYAML verifies that WriteToDisk
// does not emit the same flux-system-kustomization-*.yaml filename twice in
// either the node-level or app-level kustomization.yaml.
func TestAugmenterChildrenNoDuplicateInKustomizationYAML(t *testing.T) {
	root, nodeLayout, app, preInstall, _, cluster := buildAugmenterTestTree()

	li := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	rules := layout.LayoutRules{FluxPlacement: layout.FluxIntegrated}
	if err := li.IntegrateWithLayout(root, cluster, rules); err != nil {
		t.Fatalf("IntegrateWithLayout: %v", err)
	}

	dir := t.TempDir()
	if err := root.WriteToDisk(dir); err != nil {
		t.Fatalf("WriteToDisk: %v", err)
	}

	// Node-level kustomization.yaml must reference "myapp" exactly once.
	augAssertOnce(t, dir, nodeLayout.FullRepoPath(), "flux-system-kustomization-myapp.yaml")

	// App-level kustomization.yaml must reference the pre-install CR exactly once.
	augAssertOnce(t, dir, app.FullRepoPath(),
		"flux-system-kustomization-"+preInstall.Name+".yaml")
}

// TestAugmenterChildrenWriteToTar verifies no duplicate entries in the tar
// output. Crane consumes WriteToTar for OCI artifacts.
func TestAugmenterChildrenWriteToTar(t *testing.T) {
	root, nodeLayout, app, preInstall, _, cluster := buildAugmenterTestTree()

	li := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	rules := layout.LayoutRules{FluxPlacement: layout.FluxIntegrated}
	if err := li.IntegrateWithLayout(root, cluster, rules); err != nil {
		t.Fatalf("IntegrateWithLayout: %v", err)
	}

	var buf bytes.Buffer
	if err := root.WriteToTar(&buf); err != nil {
		t.Fatalf("WriteToTar: %v", err)
	}

	files := map[string]string{}
	tr := tar.NewReader(&buf)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar.Next: %v", err)
		}
		if strings.HasSuffix(hdr.Name, "kustomization.yaml") {
			data, _ := io.ReadAll(tr)
			files[hdr.Name] = string(data)
		}
	}

	nodeKust := augFindKust(t, files, nodeLayout.FullRepoPath())
	augAssertCount(t, nodeKust, "flux-system-kustomization-myapp.yaml", 1)

	appKust := augFindKust(t, files, app.FullRepoPath())
	augAssertCount(t, appKust, "flux-system-kustomization-"+preInstall.Name+".yaml", 1)
}

// TestAugmenterChildrenErrorWhenNoSourceRef verifies that IntegrateWithLayout
// returns a blocking error when augmenter children need CRs but the bundle
// has a nil, empty, or incomplete SourceRef.
func TestAugmenterChildrenErrorWhenNoSourceRef(t *testing.T) {
	cases := []struct {
		name string
		sr   *stack.SourceRef
	}{
		{"nil", nil},
		{"empty struct", &stack.SourceRef{}},
		{"missing Kind", &stack.SourceRef{Name: "flux-system"}},
		{"missing Name", &stack.SourceRef{Kind: "GitRepository"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			appChild := &layout.ManifestLayout{
				Name:          "myapp",
				Namespace:     "clusters/prod",
				FluxPlacement: layout.FluxIntegrated,
				Children: []*layout.ManifestLayout{{
					Name:          "myapp-00-pre-install",
					Namespace:     "clusters/prod/myapp/myapp-00-pre-install",
					FluxPlacement: layout.FluxIntegrated,
				}},
			}
			bundle := &stack.Bundle{Name: "apps", SourceRef: tc.sr}
			node := &stack.Node{Name: "prod", Bundle: bundle}
			cluster := &stack.Cluster{Node: node}
			nodeLayout := &layout.ManifestLayout{
				Name:          "prod",
				FluxPlacement: layout.FluxIntegrated,
				Children:      []*layout.ManifestLayout{appChild},
			}
			root := &layout.ManifestLayout{Children: []*layout.ManifestLayout{nodeLayout}}

			li := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
			rules := layout.LayoutRules{FluxPlacement: layout.FluxIntegrated}
			if err := li.IntegrateWithLayout(root, cluster, rules); err == nil {
				t.Fatalf("sourceRef=%v: expected error, got nil", tc.sr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Umbrella child augmenter sub-layout CR generation (#578)
// ---------------------------------------------------------------------------

// buildUmbrellaAugmenterTree constructs a layout+cluster tree simulating
// helm-multi-tier with FluxIntegrated: a platform umbrella node with a
// platform-apps umbrella child layout, which has a redis sub-layout added by
// Crane's helmchart augmenter (invisible to the bundle model).
//
// parentSR and childSR allow tests to use distinct SourceRefs to verify
// correct SourceRef ownership. Pass testSR() for both in the common case.
func buildUmbrellaAugmenterTree(parentSR, childSR *stack.SourceRef) (
	root, platformLayout, platformApps, redis *layout.ManifestLayout,
	cluster *stack.Cluster,
) {
	redis = &layout.ManifestLayout{
		Name:          "redis",
		Namespace:     "platform/platform-apps/redis",
		FluxPlacement: layout.FluxIntegrated,
		Mode:          layout.KustomizationExplicit,
	}
	platformApps = &layout.ManifestLayout{
		Name:          "platform-apps",
		Namespace:     "platform/platform-apps",
		FluxPlacement: layout.FluxIntegrated,
		Mode:          layout.KustomizationExplicit,
		UmbrellaChild: true,
		Children:      []*layout.ManifestLayout{redis},
	}
	platformLayout = &layout.ManifestLayout{
		Name:      "platform",
		Namespace: "platform",
		Children:  []*layout.ManifestLayout{platformApps},
	}
	root = &layout.ManifestLayout{
		Children: []*layout.ManifestLayout{platformLayout},
	}
	umbrella := &stack.Bundle{
		Name:      "platform",
		SourceRef: parentSR,
		Children: []*stack.Bundle{
			{Name: "platform-apps", SourceRef: childSR},
		},
	}
	platformNode := &stack.Node{Name: "platform", Bundle: umbrella}
	cluster = &stack.Cluster{Name: "demo", Node: platformNode}
	return
}

// TestUmbrellaChildAugmenterSubLayoutGetFluxCR verifies that IntegrateWithLayout
// places a Kustomization CR in an umbrella child layout's Resources for any
// augmenter-added sub-layout. Simulates helm-multi-tier: platform-apps is an
// umbrella child, redis is added by the helmchart augmenter as a layout child
// of platform-apps but is not in the umbrella bundle model.
func TestUmbrellaChildAugmenterSubLayoutGetFluxCR(t *testing.T) {
	root, _, platformApps, _, cluster := buildUmbrellaAugmenterTree(testSR(), testSR())

	li := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	rules := layout.LayoutRules{FluxPlacement: layout.FluxIntegrated}
	if err := li.IntegrateWithLayout(root, cluster, rules); err != nil {
		t.Fatalf("IntegrateWithLayout: %v", err)
	}

	if !augHasCR(platformApps.Resources, "redis") {
		t.Errorf("expected Kustomization CR for 'redis' in platform-apps.Resources; got %d resources", len(platformApps.Resources))
	}
}

// TestUmbrellaChildAugmenterSubLayoutNoDanglingReference verifies that
// WriteToDisk does not produce a dangling reference in
// platform-apps/kustomization.yaml when redis is an augmenter-added sub-layout.
// The test derives the expected CR filename directly from kustomization.yaml to
// avoid coupling to a specific FileNaming mode.
func TestUmbrellaChildAugmenterSubLayoutNoDanglingReference(t *testing.T) {
	root, _, platformApps, _, cluster := buildUmbrellaAugmenterTree(testSR(), testSR())

	li := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	rules := layout.LayoutRules{FluxPlacement: layout.FluxIntegrated}
	if err := li.IntegrateWithLayout(root, cluster, rules); err != nil {
		t.Fatalf("IntegrateWithLayout: %v", err)
	}

	dir := t.TempDir()
	if err := root.WriteToDisk(dir); err != nil {
		t.Fatalf("WriteToDisk: %v", err)
	}

	// Read kustomization.yaml and extract the redis CR filename from the
	// resources section. Fail if not exactly one redis entry is found.
	kustPath := filepath.Join(dir, platformApps.FullRepoPath(), "kustomization.yaml")
	kustData, err := os.ReadFile(kustPath)
	if err != nil {
		t.Fatalf("read platform-apps/kustomization.yaml: %v", err)
	}
	var redisEntries []string
	for line := range strings.SplitSeq(string(kustData), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") && strings.Contains(trimmed, "redis") {
			redisEntries = append(redisEntries, strings.TrimPrefix(trimmed, "- "))
		}
	}
	if len(redisEntries) == 0 {
		t.Fatalf("platform-apps/kustomization.yaml contains no redis resource entry:\n%s", kustData)
	}
	if len(redisEntries) > 1 {
		t.Fatalf("platform-apps/kustomization.yaml contains %d redis resource entries (want 1): %v", len(redisEntries), redisEntries)
	}

	// The referenced CR file must exist on disk — this is the dangling reference check.
	crFile := redisEntries[0]
	crPath := filepath.Join(dir, platformApps.FullRepoPath(), crFile)
	if _, err := os.Stat(crPath); os.IsNotExist(err) {
		t.Errorf("dangling reference: kustomization.yaml references %q but the file does not exist", crFile)
	}
}

// TestUmbrellaChildAugmenterSubLayoutUsesChildSourceRef verifies that the
// Kustomization CR emitted for an augmenter-added sub-layout under an umbrella
// child uses the child bundle's SourceRef, not the parent umbrella bundle's.
func TestUmbrellaChildAugmenterSubLayoutUsesChildSourceRef(t *testing.T) {
	parentSR := &stack.SourceRef{Kind: "GitRepository", Name: "parent-source", Namespace: "flux-system"}
	childSR := &stack.SourceRef{Kind: "GitRepository", Name: "child-source", Namespace: "flux-system"}
	root, _, platformApps, _, cluster := buildUmbrellaAugmenterTree(parentSR, childSR)

	li := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	rules := layout.LayoutRules{FluxPlacement: layout.FluxIntegrated}
	if err := li.IntegrateWithLayout(root, cluster, rules); err != nil {
		t.Fatalf("IntegrateWithLayout: %v", err)
	}

	var redisCR *kustv1.Kustomization
	for _, r := range platformApps.Resources {
		if k, ok := r.(*kustv1.Kustomization); ok && k.Name == "redis" {
			redisCR = k
			break
		}
	}
	if redisCR == nil {
		t.Fatal("Kustomization CR for 'redis' not found in platform-apps.Resources")
	}
	if redisCR.Spec.SourceRef.Name != "child-source" {
		t.Errorf("redis CR sourceRef.name: got %q, want %q (child-source, not parent-source)",
			redisCR.Spec.SourceRef.Name, "child-source")
	}
}

// TestAugmenterGrandchildErrorWhenNoSourceRef verifies the edge case where a
// direct eligible child ALREADY has a Kustomization CR (placed externally, as
// GenerateFromBundle does for bundle sub-layouts), so newCRChildren is empty
// and the fast-path error check does not fire — but that child has its own
// eligible sub-layout (grandchild) that needs a new CR. With an empty/nil
// SourceRef on the ancestor bundle, generateChildFluxCRs must still error
// rather than silently skipping the grandchild and leaving a dangling writer
// reference.
func TestAugmenterGrandchildErrorWhenNoSourceRef(t *testing.T) {
	grandchild := &layout.ManifestLayout{
		Name:          "myapp-00-pre-install",
		Namespace:     "clusters/prod/myapp/myapp-00-pre-install",
		FluxPlacement: layout.FluxIntegrated,
	}
	appLayout := &layout.ManifestLayout{
		Name:          "myapp",
		Namespace:     "clusters/prod",
		FluxPlacement: layout.FluxIntegrated,
		Children:      []*layout.ManifestLayout{grandchild},
	}

	// Pre-place a CR for "myapp" so newCRChildren is empty — the fast-path
	// error check for direct children is skipped.
	existingCR := &kustv1.Kustomization{}
	existingCR.Name = "myapp"
	existingCR.Namespace = "flux-system"

	bundle := &stack.Bundle{Name: "apps", SourceRef: nil}
	node := &stack.Node{Name: "prod", Bundle: bundle}
	cluster := &stack.Cluster{Node: node}

	nodeLayout := &layout.ManifestLayout{
		Name:          "prod",
		FluxPlacement: layout.FluxIntegrated,
		Resources:     []client.Object{existingCR},
		Children:      []*layout.ManifestLayout{appLayout},
	}
	root := &layout.ManifestLayout{Children: []*layout.ManifestLayout{nodeLayout}}

	li := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	rules := layout.LayoutRules{FluxPlacement: layout.FluxIntegrated}
	if err := li.IntegrateWithLayout(root, cluster, rules); err == nil {
		t.Fatal("expected error: grandchild needs a CR but ancestor bundle has no SourceRef")
	}
}

// --- shared test helpers for augmenter tests ---

// buildAugmenterTestTree constructs a layout+cluster tree simulating crane's
// augmentLayoutTemplate output: a node layout with one app layout, which has
// two hook-group sub-layouts (pre-install and hooks with DependsOn).
//
// Root is intentionally unnamed so findLayoutNode accumulates the path for
// nodeLayout as "prod", matching node.GetPath() == "prod".
func buildAugmenterTestTree() (root, nodeLayout, app, preInstall, hooks *layout.ManifestLayout, cluster *stack.Cluster) {
	preInstall = &layout.ManifestLayout{
		Name:          "myapp-00-pre-install",
		Namespace:     "clusters/prod/myapp/myapp-00-pre-install",
		FluxPlacement: layout.FluxIntegrated,
	}
	hooks = &layout.ManifestLayout{
		Name:          "myapp-01-hooks",
		Namespace:     "clusters/prod/myapp/myapp-01-hooks",
		FluxPlacement: layout.FluxIntegrated,
		DependsOn:     []string{"myapp-00-pre-install"},
	}
	app = &layout.ManifestLayout{
		Name:          "myapp",
		Namespace:     "clusters/prod",
		FluxPlacement: layout.FluxIntegrated,
		Children:      []*layout.ManifestLayout{preInstall, hooks},
	}
	sr := &stack.SourceRef{Kind: "GitRepository", Name: "flux-system", Namespace: "flux-system"}
	bundle := &stack.Bundle{Name: "apps", SourceRef: sr}
	node := &stack.Node{Name: "prod", Bundle: bundle}
	cluster = &stack.Cluster{Node: node}
	nodeLayout = &layout.ManifestLayout{
		Name:          "prod",
		FluxPlacement: layout.FluxIntegrated,
		Children:      []*layout.ManifestLayout{app},
	}
	root = &layout.ManifestLayout{Children: []*layout.ManifestLayout{nodeLayout}}
	return
}

func augHasCR(resources []client.Object, name string) bool {
	for _, r := range resources {
		if k, ok := r.(*kustv1.Kustomization); ok && k.Name == name {
			return true
		}
	}
	return false
}

func augMustKustomization(t *testing.T, obj client.Object) *kustv1.Kustomization {
	t.Helper()
	k, ok := obj.(*kustv1.Kustomization)
	if !ok {
		t.Fatalf("expected *kustv1.Kustomization, got %T", obj)
	}
	return k
}

func augAssertOnce(t *testing.T, baseDir, layoutPath, target string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(baseDir, layoutPath, "kustomization.yaml"))
	if err != nil {
		t.Fatalf("read kustomization.yaml under %s: %v", layoutPath, err)
	}
	if count := strings.Count(string(data), target); count != 1 {
		t.Errorf("expected exactly 1 reference to %q in %s/kustomization.yaml, got %d:\n%s",
			target, layoutPath, count, data)
	}
}

func augFindKust(t *testing.T, files map[string]string, layoutPath string) string {
	t.Helper()
	key := filepath.ToSlash(filepath.Join(layoutPath, "kustomization.yaml"))
	for k, v := range files {
		if strings.HasSuffix(k, key) {
			return v
		}
	}
	t.Fatalf("kustomization.yaml not found for layout path %q; available: %v", layoutPath, augKeysOf(files))
	return ""
}

func augAssertCount(t *testing.T, content, target string, want int) {
	t.Helper()
	if got := strings.Count(content, target); got != want {
		t.Errorf("expected %d reference(s) to %q, got %d:\n%s", want, target, got, content)
	}
}

func augKeysOf(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestCreateLayoutWithResources_FluxIntegrated_RejectsInvalidSourceRef(t *testing.T) {
	// Validator fires before WalkCluster; no ApplicationConfig needed.
	c := &stack.Cluster{
		Name: "test",
		Node: &stack.Node{
			Name:   "prod",
			Bundle: &stack.Bundle{Name: "apps"},
		},
	}
	li := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	rules := layout.LayoutRules{FluxPlacement: layout.FluxIntegrated}

	if _, err := li.CreateLayoutWithResources(c, rules); err == nil {
		t.Fatal("expected error for FluxIntegrated with nil SourceRef, got nil")
	}
}

func TestCreateLayoutWithResources_FluxSeparate_AllowsMissingSourceRef(t *testing.T) {
	bundle := &stack.Bundle{Name: "apps"}
	bundle.Applications = []*stack.Application{fakeUmbrellaApp("myapp", "my-cm")}
	node := &stack.Node{Name: "prod", Bundle: bundle}
	c := &stack.Cluster{Node: node, Name: "test-cluster"}

	li := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	rules := layout.LayoutRules{FluxPlacement: layout.FluxSeparate}

	if _, err := li.CreateLayoutWithResources(c, rules); err != nil {
		t.Fatalf("FluxSeparate: unexpected error for nil SourceRef: %v", err)
	}
}

// TestIntegrateWithLayout_RulesFluxSeparate_NoDuplicateChildCRs is a
// regression guard for #576. Before the fix, IntegrateWithLayout read
// li.FluxPlacement (constructor default = FluxIntegrated) and ignored
// rules.FluxPlacement, so the FluxIntegrated emission block ran even when
// the caller asked for FluxSeparate via rules. That produced duplicate
// Kustomization CRs for augmenter-added child layouts. After the fix,
// rules.FluxPlacement is the sole authority and the FluxSeparate path must
// not emit any of those per-child CRs.
func TestIntegrateWithLayout_RulesFluxSeparate_NoDuplicateChildCRs(t *testing.T) {
	root, _, app, preInstall, hooks, cluster := buildAugmenterTestTree()

	li := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	rules := layout.LayoutRules{FluxPlacement: layout.FluxSeparate}
	if err := li.IntegrateWithLayout(root, cluster, rules); err != nil {
		t.Fatalf("IntegrateWithLayout (FluxSeparate): %v", err)
	}

	// FluxSeparate must not emit augmenter-child CRs into app.Resources
	// (that emission only fires on the FluxIntegrated code path).
	if augHasCR(app.Resources, preInstall.Name) {
		t.Errorf("FluxSeparate emitted a Kustomization CR for augmenter child %q at app.Resources; FluxIntegrated leaked into the FluxSeparate path", preInstall.Name)
	}
	if augHasCR(app.Resources, hooks.Name) {
		t.Errorf("FluxSeparate emitted a Kustomization CR for augmenter child %q at app.Resources; FluxIntegrated leaked into the FluxSeparate path", hooks.Name)
	}
}

// testSR returns a minimal valid SourceRef for use in FluxIntegrated fixtures.
func testSR() *stack.SourceRef {
	return &stack.SourceRef{Kind: "GitRepository", Name: "flux-system", Namespace: "flux-system"}
}
