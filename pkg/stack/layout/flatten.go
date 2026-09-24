package layout

import (
	"strings"
)

// flattenSingleTier collapses one vestigial intermediate layout layer when
// safe. The absorbing layout takes over the collapsed child's origins, so the
// Flux and ArgoCD paths derived from it are the surviving directory; nothing
// is rewritten afterwards. Returns root unchanged when preconditions fail.
//
// Preconditions for collapse — ALL must hold:
//   - rules.FlattenSingleTier is true.
//   - The parent layout's Namespace has no path separator (top-level layer).
//   - The parent has exactly one Children entry.
//   - The parent has no own Resources.
//   - The single child is not an UmbrellaChild.
//   - The single child has no Children of its own.
//
// On collapse:
//   - parent.Resources = child.Resources, parent.Children = nil.
//   - parent.ExtraFiles += child.ExtraFiles, parent.ConfigMapGenerators += child.ConfigMapGenerators.
//   - Inherit child's Mode / FilePer / ApplicationFileMode / FileNaming if
//     the parent has them as their unset sentinel value.
//   - Append the child's origin nodes and bundles to the parent's, and take
//     its application.
func flattenSingleTier(root *ManifestLayout, rules LayoutRules) *ManifestLayout {
	if root == nil || !rules.FlattenSingleTier {
		return root
	}
	if !canFlatten(root) {
		return root
	}

	child := root.Children[0]

	// Mutate parent.
	root.Resources = child.Resources
	root.ExtraFiles = append(root.ExtraFiles, child.ExtraFiles...)
	root.ConfigMapGenerators = append(root.ConfigMapGenerators, child.ConfigMapGenerators...)
	root.Children = nil
	if root.Mode == KustomizationUnset && child.Mode != KustomizationUnset {
		root.Mode = child.Mode
	}
	if root.FilePer == FilePerUnset && child.FilePer != FilePerUnset {
		root.FilePer = child.FilePer
	}
	if root.ApplicationFileMode == AppFileUnset && child.ApplicationFileMode != AppFileUnset {
		root.ApplicationFileMode = child.ApplicationFileMode
	}
	if root.FileNaming == FileNamingUnset && child.FileNaming != FileNamingUnset {
		root.FileNaming = child.FileNaming
	}
	// The absorbed layout's resources now live in root's directory, so root
	// renders what it rendered: when both carry bundles, both bundles' paths
	// are this surviving directory.
	root.origin.nodes = append(root.origin.nodes, child.origin.nodes...)
	root.origin.bundles = append(root.origin.bundles, child.origin.bundles...)
	if child.origin.app != nil {
		root.origin.app = child.origin.app
	}

	return root
}

// canFlatten checks the collapse preconditions on a candidate parent layout.
func canFlatten(parent *ManifestLayout) bool {
	if parent == nil {
		return false
	}
	// Parent must be top-level (single-segment or empty Namespace).
	if strings.Contains(parent.Namespace, "/") {
		return false
	}
	if len(parent.Children) != 1 {
		return false
	}
	if len(parent.Resources) > 0 {
		return false
	}
	child := parent.Children[0]
	if child == nil {
		return false
	}
	if child.UmbrellaChild {
		return false
	}
	if len(child.Children) > 0 {
		return false
	}
	return true
}
