package fluxcd_test

import (
	"testing"

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

	if integrator.FluxPlacement != layout.FluxIntegrated {
		t.Errorf("expected FluxIntegrated placement, got %s", integrator.FluxPlacement)
	}
}

func TestLayoutIntegrator_SetFluxPlacement(t *testing.T) {
	generator := fluxstack.NewResourceGenerator()
	integrator := fluxstack.NewLayoutIntegrator(generator)

	// Test setting to separate
	integrator.SetFluxPlacement(layout.FluxSeparate)
	if integrator.FluxPlacement != layout.FluxSeparate {
		t.Errorf("expected FluxSeparate, got %s", integrator.FluxPlacement)
	}

	// Test setting to integrated
	integrator.SetFluxPlacement(layout.FluxIntegrated)
	if integrator.FluxPlacement != layout.FluxIntegrated {
		t.Errorf("expected FluxIntegrated, got %s", integrator.FluxPlacement)
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

	// Set invalid placement
	integrator.FluxPlacement = layout.FluxPlacement("invalid")

	ml := &layout.ManifestLayout{Name: "test"}
	cluster := &stack.Cluster{
		Name: "test-cluster",
		Node: &stack.Node{Name: "root"},
	}

	err := integrator.IntegrateWithLayout(ml, cluster, layout.LayoutRules{})
	if err == nil {
		t.Error("expected error for invalid placement")
	}
}

func TestLayoutIntegrator_IntegrateWithLayout_Integrated(t *testing.T) {
	generator := fluxstack.NewResourceGenerator()
	integrator := fluxstack.NewLayoutIntegrator(generator)
	integrator.SetFluxPlacement(layout.FluxIntegrated)

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
	integrator.SetFluxPlacement(layout.FluxSeparate)

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
	integrator.SetFluxPlacement(layout.FluxIntegrated)

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
	integrator.SetFluxPlacement(layout.FluxSeparate)

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
