package layout

import (
	"strings"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"

	"github.com/go-kure/kure/pkg/stack"
)

// flattenSingleTier collapses one vestigial intermediate layout layer when
// safe, populating redirects on the absorbing layout for the Flux integrator
// to consume. Returns root unchanged when preconditions fail.
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
//   - Populate parent.flattenInfo (nodeAliases + pathRewrites).
func flattenSingleTier(root *ManifestLayout, c *stack.Cluster, rules LayoutRules) *ManifestLayout {
	if root == nil || c == nil || !rules.FlattenSingleTier {
		return root
	}
	if !canFlatten(root) {
		return root
	}

	child := root.Children[0]

	oldLayoutPath := child.FullRepoPath()
	newLayoutPath := root.FullRepoPath()

	// Resolve the node-path form for the alias. The collapsed child layout
	// corresponds to a stack.Node only when the layout's Name matches a Node
	// reachable from c.Node. We look up by name in the immediate Node tree;
	// for the typical case (cluster-name mode collapsing the root node into
	// the synthetic cluster root) the match is c.Node itself.
	var oldNodePath string
	if matched := findNodeByLayoutName(c.Node, child.Name); matched != nil {
		oldNodePath = matched.GetPath()
	}

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

	// Populate redirects.
	if root.flattenInfo == nil {
		root.flattenInfo = &flattenInfo{
			nodeAliases:  map[string]*ManifestLayout{},
			pathRewrites: map[string]string{},
		}
	}
	if oldNodePath != "" {
		root.flattenInfo.nodeAliases[oldNodePath] = root
	}
	if oldLayoutPath != newLayoutPath {
		root.flattenInfo.pathRewrites[oldLayoutPath] = newLayoutPath
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

// findNodeByLayoutName traverses the stack.Node tree depth-first looking for
// a Node whose Name matches the given layout-side name. Returns the first
// match or nil.
func findNodeByLayoutName(n *stack.Node, name string) *stack.Node {
	if n == nil {
		return nil
	}
	if n.Name == name {
		return n
	}
	for _, child := range n.Children {
		if found := findNodeByLayoutName(child, name); found != nil {
			return found
		}
	}
	return nil
}

// FindByNodeAlias walks the layout tree looking for a flattenInfo nodeAlias
// matching nodePath. Returns the absorbing layout, or nil if no alias
// matches. Used by the Flux integrator's findLayoutNode as a fallback when
// regular path-based search fails.
func FindByNodeAlias(ml *ManifestLayout, nodePath string) *ManifestLayout {
	if ml == nil {
		return nil
	}
	if redirect := ml.FlattenInfoNodeAlias(nodePath); redirect != nil {
		return redirect
	}
	for _, child := range ml.Children {
		if redirect := FindByNodeAlias(child, nodePath); redirect != nil {
			return redirect
		}
	}
	return nil
}

// ApplyFlattenPathRewrites walks the layout tree, gathers all path rewrites
// recorded by flattenSingleTier collapses, and rewrites Spec.Path on every
// Flux Kustomization CR found in the tree's Resources. Idempotent: if no
// rewrites were recorded, returns immediately; on repeated invocations
// already-rewritten paths no longer match the rewrite keys, so subsequent
// passes are no-ops.
//
// flattenInfo is intentionally left in place after a rewrite pass so that
// IntegrateWithLayout can be called multiple times on the same flattened
// layout — the integrator's findLayoutNode fallback depends on the
// nodeAliases remaining populated for the lifetime of the layout.
//
// Called by the Flux integrator's IntegrateWithLayout before returning.
// Lives in the layout package because the flattenInfo field is unexported
// here.
func ApplyFlattenPathRewrites(root *ManifestLayout) {
	rewrites := collectPathRewrites(root)
	if len(rewrites) == 0 {
		return
	}
	rewriteFluxPaths(root, rewrites)
}

func collectPathRewrites(ml *ManifestLayout) map[string]string {
	if ml == nil {
		return nil
	}
	var out map[string]string
	if r := ml.FlattenInfoPathRewrites(); len(r) > 0 {
		out = map[string]string{}
		for k, v := range r {
			out[k] = v
		}
	}
	for _, child := range ml.Children {
		childMap := collectPathRewrites(child)
		if len(childMap) == 0 {
			continue
		}
		if out == nil {
			out = map[string]string{}
		}
		for k, v := range childMap {
			out[k] = v
		}
	}
	return out
}

func rewriteFluxPaths(ml *ManifestLayout, rewrites map[string]string) {
	if ml == nil {
		return
	}
	for _, obj := range ml.Resources {
		k, ok := obj.(*kustomizev1.Kustomization)
		if !ok {
			continue
		}
		for old, neu := range rewrites {
			// Once a rewrite fires for this resource, stop testing the
			// remaining entries against the modified path. With Go's
			// non-deterministic map iteration, continuing could double-
			// rewrite if multiple rewrites' keys/values overlap.
			if k.Spec.Path == old {
				k.Spec.Path = neu
				break
			}
			if strings.HasPrefix(k.Spec.Path, old+"/") {
				k.Spec.Path = neu + strings.TrimPrefix(k.Spec.Path, old)
				break
			}
		}
	}
	for _, child := range ml.Children {
		rewriteFluxPaths(child, rewrites)
	}
}
