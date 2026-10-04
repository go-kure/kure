package fluxcd

import (
	"fmt"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// validateSourceRefsForFluxIntegrated checks that every bundle reachable from
// the cluster node tree has a complete SourceRef before layout walking begins.
// CreateLayoutWithResources calls it for both integrated placements,
// FluxIntegratedPerLayout and FluxIntegratedPerBundle; placement is the one in
// use, which the refusal names.
//
// Coverage:
//   - node.Bundle for every node in the tree
//   - umbrella child bundles (bundle.Children) recursively
//
// Augmenter-added ManifestLayout children inherit the ancestor bundle's
// SourceRef; checking node bundles covers them.
func validateSourceRefsForFluxIntegrated(c *stack.Cluster, placement layout.FluxPlacement) error {
	if c == nil {
		return nil
	}
	return validateNodeSourceRefs(c.Node, placement)
}

func validateNodeSourceRefs(node *stack.Node, placement layout.FluxPlacement) error {
	if node == nil {
		return nil
	}
	if node.Bundle != nil {
		if err := validateBundleSourceRefs(node.Bundle, placement); err != nil {
			return err
		}
	}
	for _, child := range node.Children {
		if err := validateNodeSourceRefs(child, placement); err != nil {
			return err
		}
	}
	return nil
}

// validateBundleSourceRefs checks the bundle itself and all umbrella children
// recursively.
func validateBundleSourceRefs(b *stack.Bundle, placement layout.FluxPlacement) error {
	if b == nil {
		return nil
	}
	if b.SourceRef == nil || b.SourceRef.Kind == "" || b.SourceRef.Name == "" {
		return errors.ResourceValidationError(
			"Bundle", b.Name, "sourceRef",
			fmt.Sprintf("%s mode requires a SourceRef with Kind and Name; ", placementName(placement))+
				"omitting it produces a Kustomization CR without spec.sourceRef, which Flux rejects",
			nil,
		)
	}
	for _, child := range b.Children {
		if child == nil {
			continue
		}
		if err := validateBundleSourceRefs(child, placement); err != nil {
			return err
		}
	}
	return nil
}

// placementName is the name a consumer writes for an integrated placement:
// the constant's, not its string value.
func placementName(placement layout.FluxPlacement) string {
	switch placement {
	case layout.FluxIntegratedPerLayout:
		return "FluxIntegratedPerLayout"
	case layout.FluxIntegratedPerBundle:
		return "FluxIntegratedPerBundle"
	default:
		return fmt.Sprintf("FluxPlacement %q", string(placement))
	}
}
