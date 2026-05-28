package fluxcd

import (
	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
)

// validateSourceRefsForFluxIntegrated checks that every bundle reachable from
// the cluster node tree has a complete SourceRef before layout walking begins.
//
// Coverage:
//   - node.Bundle for every node in the tree
//   - umbrella child bundles (bundle.Children) recursively
//
// Augmenter-added ManifestLayout children inherit the ancestor bundle's
// SourceRef; checking node bundles covers them.
func validateSourceRefsForFluxIntegrated(c *stack.Cluster) error {
	if c == nil {
		return nil
	}
	return validateNodeSourceRefs(c.Node)
}

func validateNodeSourceRefs(node *stack.Node) error {
	if node == nil {
		return nil
	}
	if node.Bundle != nil {
		if err := validateBundleSourceRefs(node.Bundle); err != nil {
			return err
		}
	}
	for _, child := range node.Children {
		if err := validateNodeSourceRefs(child); err != nil {
			return err
		}
	}
	return nil
}

// validateBundleSourceRefs checks the bundle itself and all umbrella children
// recursively.
func validateBundleSourceRefs(b *stack.Bundle) error {
	if b == nil {
		return nil
	}
	if b.SourceRef == nil || b.SourceRef.Kind == "" || b.SourceRef.Name == "" {
		return errors.ResourceValidationError(
			"Bundle", b.Name, "sourceRef",
			"FluxIntegratedPerLayout mode requires a SourceRef with Kind and Name; "+
				"omitting it produces a Kustomization CR without spec.sourceRef, which Flux rejects",
			nil,
		)
	}
	for _, child := range b.Children {
		if child == nil {
			continue
		}
		if err := validateBundleSourceRefs(child); err != nil {
			return err
		}
	}
	return nil
}
