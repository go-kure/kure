package fluxcd

import (
	"fmt"
	"path/filepath"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
//
// If the layout was post-processed by FlattenSingleTier (recorded as
// flattenInfo on the absorbing layouts), this method consults nodeAliases
// during integrated placement (see findLayoutNode) and rewrites Flux
// Kustomization Spec.Path values via layout.ApplyFlattenPathRewrites before
// returning, regardless of placement mode.
func (li *LayoutIntegrator) IntegrateWithLayout(ml *layout.ManifestLayout, c *stack.Cluster, rules layout.LayoutRules) error {
	if ml == nil || c == nil {
		return nil
	}

	var err error
	switch li.FluxPlacement {
	case layout.FluxIntegrated:
		err = li.addIntegratedFluxToLayout(ml, c, rules)
	case layout.FluxSeparate:
		err = li.addSeparateFluxToLayout(ml, c, rules)
	default:
		return errors.NewValidationError("fluxPlacement", string(li.FluxPlacement), "LayoutIntegrator",
			[]string{string(layout.FluxIntegrated), string(layout.FluxSeparate)})
	}
	if err != nil {
		return err
	}

	layout.ApplyFlattenPathRewrites(ml)
	return nil
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

	// In FluxIntegrated mode, emit Kustomization CRs for all eligible direct
	// children of layoutNode and recurse into their subtrees.
	//
	// "Eligible" mirrors the writer condition: !UmbrellaChild &&
	// ApplicationFileMode != AppFileSingle. Stack-node children are skipped
	// because processNodeForIntegratedFlux handles them recursively below.
	//
	// This covers two cases with the same code:
	//   (a) Flat/nodeOnly: app layouts are direct children of the node layout;
	//       writers reference each as flux-system-kustomization-{name}.yaml
	//       from the node kustomization.yaml, so a CR is required at that level.
	//   (b) Augmenter sub-layouts: hook-group children of app layouts;
	//       generateChildFluxCRs recurses and places those CRs in the app
	//       layout's Resources.
	if node.Bundle != nil {
		var eligibleChildren []*layout.ManifestLayout
		for _, child := range layoutNode.Children {
			if child.UmbrellaChild || child.ApplicationFileMode == layout.AppFileSingle {
				continue
			}
			if li.isStackNodeChild(child.Name, node) {
				continue
			}
			eligibleChildren = append(eligibleChildren, child)
		}

		if len(eligibleChildren) > 0 {
			// Determine which eligible children need a NEW CR (not already placed
			// by GenerateFromBundle, e.g. umbrella bundle layouts already have one).
			var newCRChildren []*layout.ManifestLayout
			for _, child := range eligibleChildren {
				if !li.hasKustomizationCR(layoutNode.Resources, child.Name) {
					newCRChildren = append(newCRChildren, child)
				}
			}

			// Resolve sourceRef once; used for both new CRs and recursion.
			var sr kustv1.CrossNamespaceSourceReference
			if node.Bundle.SourceRef != nil &&
				node.Bundle.SourceRef.Kind != "" &&
				node.Bundle.SourceRef.Name != "" {
				sr = kustv1.CrossNamespaceSourceReference{
					Kind:      node.Bundle.SourceRef.Kind,
					Name:      node.Bundle.SourceRef.Name,
					Namespace: node.Bundle.SourceRef.Namespace,
				}
			}

			// A new CR without spec.sourceRef is invalid — fail fast.
			// Children that already have CRs (e.g. umbrella bundle layouts) are
			// exempt from this check; they were placed by GenerateFromBundle which
			// handles absent SourceRef separately. Descendant children are validated
			// inside generateChildFluxCRs as it recurses.
			if len(newCRChildren) > 0 && sr.Kind == "" {
				return errors.ResourceValidationError(
					"Bundle", node.Bundle.Name, "sourceRef",
					"FluxIntegrated mode requires a SourceRef with Kind and Name on bundles "+
						"whose layout has eligible children without existing Kustomization CRs; "+
						"omitting it produces invalid Flux Kustomization CRs",
					nil,
				)
			}

			// Emit direct-child CRs and recurse into all eligible children.
			// Recursion is unconditional: even when sr is empty (all direct children
			// already have CRs), grandchildren may need new CRs — generateChildFluxCRs
			// will error if it encounters one that needs a CR but sr is invalid.
			for _, child := range eligibleChildren {
				if !li.hasKustomizationCR(layoutNode.Resources, child.Name) {
					// sr.Kind is guaranteed non-empty here (checked above).
					layoutNode.Resources = append(layoutNode.Resources,
						li.Generator.createKustomizationForLayout(child, sr))
				}
				if err := li.generateChildFluxCRs(child, sr); err != nil {
					return err
				}
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

// isStackNodeChild returns true when name matches a direct child stack.Node of
// node. Used to skip ManifestLayout.Children that correspond to child nodes
// already processed by the recursive processNodeForIntegratedFlux call.
func (li *LayoutIntegrator) isStackNodeChild(name string, node *stack.Node) bool {
	for _, child := range node.Children {
		if child.Name == name {
			return true
		}
	}
	return false
}

// hasKustomizationCR returns true when resources already contains a
// *kustv1.Kustomization with the given name.
func (li *LayoutIntegrator) hasKustomizationCR(resources []client.Object, name string) bool {
	for _, r := range resources {
		if k, ok := r.(*kustv1.Kustomization); ok && k.Name == name {
			return true
		}
	}
	return false
}

// generateChildFluxCRs places a Kustomization CR in parent.Resources for each
// eligible child of parent, then recurses. A child is eligible when:
//   - !child.UmbrellaChild
//   - child.ApplicationFileMode != layout.AppFileSingle
//
// These conditions match exactly what the writers use to emit
// flux-system-kustomization-{child.Name}.yaml from the parent kustomization.yaml,
// ensuring every reference the writers produce has a backing CR.
func (li *LayoutIntegrator) generateChildFluxCRs(
	parent *layout.ManifestLayout,
	sourceRef kustv1.CrossNamespaceSourceReference,
) error {
	for _, child := range parent.Children {
		if child.UmbrellaChild || child.ApplicationFileMode == layout.AppFileSingle {
			continue
		}
		if !li.hasKustomizationCR(parent.Resources, child.Name) {
			if sourceRef.Kind == "" {
				return errors.ResourceValidationError(
					"ManifestLayout", child.Name, "sourceRef",
					"FluxIntegrated mode requires a SourceRef with Kind and Name; "+
						"this descendant layout needs a Kustomization CR but the "+
						"ancestor bundle has no valid SourceRef",
					nil,
				)
			}
			parent.Resources = append(parent.Resources,
				li.Generator.createKustomizationForLayout(child, sourceRef))
		}
		if err := li.generateChildFluxCRs(child, sourceRef); err != nil {
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
// When path-based search misses (because FlattenSingleTier collapsed the
// target's layout into an ancestor), it falls back to the flattenInfo alias
// recorded on the absorbing layout.
func (li *LayoutIntegrator) findLayoutNode(ml *layout.ManifestLayout, node *stack.Node) *layout.ManifestLayout {
	targetPath := node.GetPath()
	if found := li.findLayoutNodeByPath(ml, targetPath, ""); found != nil {
		return found
	}
	return layout.FindByNodeAlias(ml, targetPath)
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
