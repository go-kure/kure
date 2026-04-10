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

	// Fail fast on umbrella / disjointness / multi-package violations before
	// we walk the tree.
	if err := stack.ValidateCluster(c); err != nil {
		return nil, err
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
// The root parameter is always the top-level layout so that path-based lookups
// resolve against the full tree (node paths are absolute).
func (li *LayoutIntegrator) processNodeForIntegratedFlux(root *layout.ManifestLayout, node *stack.Node, clusterName string) error {
	// Find the corresponding layout node
	layoutNode := li.findLayoutNode(root, node)
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

		// For umbrella bundles, place child Flux Kustomization CRs at the
		// immediate enclosing parent layout directory (not in child subdirs).
		if len(node.Bundle.Children) > 0 {
			// In non-nodeOnly layouts, the walker creates an intermediate
			// bundle layout under the node layout. Umbrella children live
			// there. In nodeOnly layouts the umbrella children sit directly
			// under the node layout, so the node layout IS the parent.
			parentForChildren := layoutNode
			if bl := findBundleLayout(layoutNode, node.Bundle.Name); bl != nil {
				parentForChildren = bl
			}
			if err := li.placeUmbrellaChildrenFlux(parentForChildren, node.Bundle); err != nil {
				return errors.ResourceValidationError("Node", node.Name, "umbrella",
					fmt.Sprintf("failed to place umbrella child Flux resources: %v", err), err)
			}
		}
	}

	// Process child nodes — always search from root for path-based matching
	for _, child := range node.Children {
		if err := li.processNodeForIntegratedFlux(root, child, clusterName); err != nil {
			return err
		}
	}

	return nil
}

// placeUmbrellaChildrenFlux walks a bundle's umbrella Children subtree and
// places each child's Flux Kustomization CR (and Source CR if the child's
// SourceRef has a URL) at the PARENT layout node. Nested umbrella
// grandchildren are placed at their immediate enclosing umbrella child's
// layout node, which the walker has already marked with UmbrellaChild=true.
func (li *LayoutIntegrator) placeUmbrellaChildrenFlux(parentLayout *layout.ManifestLayout, umbrella *stack.Bundle) error {
	umbrella.InitializeUmbrella()
	for _, child := range umbrella.Children {
		if child == nil {
			continue
		}
		childKust := li.Generator.createKustomization(child)
		parentLayout.Resources = append(parentLayout.Resources, childKust)

		if child.SourceRef != nil && child.SourceRef.URL != "" {
			src, err := li.Generator.createSource(child.SourceRef, child.Name)
			if err != nil {
				return errors.ResourceValidationError("Bundle", child.Name, "source",
					fmt.Sprintf("failed to create source: %v", err), err)
			}
			if src != nil {
				parentLayout.Resources = append(parentLayout.Resources, src)
			}
		}

		if len(child.Children) > 0 {
			childLayoutNode := findUmbrellaChildLayout(parentLayout, child.Name)
			if childLayoutNode == nil {
				return errors.ResourceValidationError("Bundle", child.Name, "umbrella",
					"nested umbrella child layout not found", nil)
			}
			if err := li.placeUmbrellaChildrenFlux(childLayoutNode, child); err != nil {
				return err
			}
		}
	}
	return nil
}

// findBundleLayout returns the direct child layout named after the given
// bundle, if any. In non-nodeOnly layouts, the walker inserts an intermediate
// bundle layout between the node layout and its application/umbrella-child
// layouts — this helper locates it so umbrella children can be placed there.
func findBundleLayout(parent *layout.ManifestLayout, bundleName string) *layout.ManifestLayout {
	for _, c := range parent.Children {
		if c.Name == bundleName && !c.UmbrellaChild {
			return c
		}
	}
	return nil
}

// findUmbrellaChildLayout returns the direct umbrella-child sub-layout with
// the given name. Per-level lookup is sufficient because
// placeUmbrellaChildrenFlux recurses into nested umbrellas explicitly.
func findUmbrellaChildLayout(parent *layout.ManifestLayout, name string) *layout.ManifestLayout {
	for _, c := range parent.Children {
		if c.UmbrellaChild && c.Name == name {
			return c
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

// findLayoutNode finds the layout node corresponding to a stack node using path-based matching.
// It computes the layout's full path and compares against the node's path to avoid
// ambiguity when nodes at different hierarchy levels share the same name.
func (li *LayoutIntegrator) findLayoutNode(ml *layout.ManifestLayout, node *stack.Node) *layout.ManifestLayout {
	targetPath := node.GetPath()
	return li.findLayoutNodeByPath(ml, targetPath, "")
}

// findLayoutNodeByPath recursively searches the layout tree for a node whose
// accumulated path matches the target path.
func (li *LayoutIntegrator) findLayoutNodeByPath(ml *layout.ManifestLayout, targetPath string, parentPath string) *layout.ManifestLayout {
	// Build the current layout node's path
	currentPath := ml.Name
	if parentPath != "" && ml.Name != "" {
		currentPath = parentPath + "/" + ml.Name
	} else if parentPath != "" {
		currentPath = parentPath
	}

	if currentPath == targetPath {
		return ml
	}

	// Search in children
	for _, child := range ml.Children {
		if found := li.findLayoutNodeByPath(child, targetPath, currentPath); found != nil {
			return found
		}
	}

	return nil
}

// SetFluxPlacement configures where Flux resources should be placed in layouts.
func (li *LayoutIntegrator) SetFluxPlacement(placement layout.FluxPlacement) {
	li.FluxPlacement = placement
}
