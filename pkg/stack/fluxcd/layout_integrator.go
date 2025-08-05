package fluxcd

import (
	"fmt"
	"path/filepath"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// LayoutIntegrator implements the workflow.LayoutIntegrator interface for Flux.
// It handles integration of Flux resources with manifest layouts.
type LayoutIntegrator struct {
	// ResourceGenerator generates the Flux resources
	Generator *ResourceGenerator
	// FluxPlacement controls where Flux resources are placed in the layout
	FluxPlacement layout.FluxPlacement
}

// NewLayoutIntegrator creates a FluxCD layout integrator.
func NewLayoutIntegrator(generator *ResourceGenerator) *LayoutIntegrator {
	return &LayoutIntegrator{
		Generator:     generator,
		FluxPlacement: layout.FluxIntegrated,
	}
}

// IntegrateWithLayout adds Flux resources to an existing manifest layout.
func (li *LayoutIntegrator) IntegrateWithLayout(ml *layout.ManifestLayout, c *stack.Cluster, rules layout.LayoutRules) error {
	if ml == nil || c == nil {
		return nil
	}
	
	switch li.FluxPlacement {
	case layout.FluxIntegrated:
		return li.addIntegratedFluxToLayout(ml, c, rules)
	case layout.FluxSeparate:
		return li.addSeparateFluxToLayout(ml, c, rules)
	default:
		return errors.NewValidationError("fluxPlacement", string(li.FluxPlacement), "LayoutIntegrator",
			[]string{string(layout.FluxIntegrated), string(layout.FluxSeparate)})
	}
}

// CreateLayoutWithResources creates a new layout that includes Flux resources.
func (li *LayoutIntegrator) CreateLayoutWithResources(c *stack.Cluster, rules layout.LayoutRules) (*layout.ManifestLayout, error) {
	if c == nil {
		return nil, nil
	}
	
	// Generate the base manifest layout first
	ml, err := layout.WalkCluster(c, rules)
	if err != nil {
		return nil, errors.ResourceValidationError("Cluster", c.Name, "layout",
			fmt.Sprintf("failed to create base layout: %v", err), err)
	}
	
	// Integrate Flux resources into the layout
	if err := li.IntegrateWithLayout(ml, c, rules); err != nil {
		return nil, errors.ResourceValidationError("Cluster", c.Name, "flux-integration",
			fmt.Sprintf("failed to integrate Flux resources: %v", err), err)
	}
	
	return ml, nil
}

// addIntegratedFluxToLayout places Flux Kustomizations alongside their target manifests.
func (li *LayoutIntegrator) addIntegratedFluxToLayout(ml *layout.ManifestLayout, c *stack.Cluster, rules layout.LayoutRules) error {
	return li.processNodeForIntegratedFlux(ml, c.Node, c.Name)
}

// processNodeForIntegratedFlux recursively processes nodes to add integrated Flux resources.
func (li *LayoutIntegrator) processNodeForIntegratedFlux(ml *layout.ManifestLayout, node *stack.Node, clusterName string) error {
	// Find the corresponding layout node
	layoutNode := li.findLayoutNode(ml, node)
	if layoutNode == nil {
		return errors.ResourceValidationError("Node", node.Name, "layout",
			"corresponding layout node not found", nil)
	}
	
	// Generate Flux resources for this node
	if node.Bundle != nil {
		fluxResources, err := li.Generator.GenerateFromBundle(node.Bundle)
		if err != nil {
			return errors.ResourceValidationError("Node", node.Name, "flux-resources",
				fmt.Sprintf("failed to generate Flux resources: %v", err), err)
		}
		
		// Add Flux resources to the layout node
		layoutNode.Resources = append(layoutNode.Resources, fluxResources...)
	}
	
	// Process child nodes
	for _, child := range node.Children {
		if err := li.processNodeForIntegratedFlux(layoutNode, child, clusterName); err != nil {
			return err
		}
	}
	
	return nil
}

// addSeparateFluxToLayout creates a separate flux-system directory for Flux resources.
func (li *LayoutIntegrator) addSeparateFluxToLayout(ml *layout.ManifestLayout, c *stack.Cluster, rules layout.LayoutRules) error {
	// Generate all Flux resources for the cluster
	fluxResources, err := li.Generator.GenerateFromCluster(c)
	if err != nil {
		return errors.ResourceValidationError("Cluster", c.Name, "flux-resources",
			fmt.Sprintf("failed to generate Flux resources: %v", err), err)
	}
	
	if len(fluxResources) == 0 {
		return nil
	}
	
	// Create a separate flux-system layout
	fluxLayout := &layout.ManifestLayout{
		Name:      "flux-system",
		Namespace: filepath.Join(ml.Namespace, "flux-system"),
		FilePer:   layout.FilePerResource,
		Mode:      layout.KustomizationExplicit,
		Resources: fluxResources,
	}
	
	// Add to the main layout
	ml.Children = append(ml.Children, fluxLayout)
	
	return nil
}

// findLayoutNode finds the layout node corresponding to a stack node.
func (li *LayoutIntegrator) findLayoutNode(ml *layout.ManifestLayout, node *stack.Node) *layout.ManifestLayout {
	// Check if this is the target node
	if ml.Name == node.Name {
		return ml
	}
	
	// Search in children
	for _, child := range ml.Children {
		if found := li.findLayoutNode(child, node); found != nil {
			return found
		}
	}
	
	return nil
}

// SetFluxPlacement configures where Flux resources should be placed in layouts.
func (li *LayoutIntegrator) SetFluxPlacement(placement layout.FluxPlacement) {
	li.FluxPlacement = placement
}