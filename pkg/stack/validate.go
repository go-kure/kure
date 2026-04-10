package stack

import (
	"fmt"

	"github.com/go-kure/kure/pkg/errors"
)

// ValidateCluster performs cluster-level structural validation that cannot be
// expressed on a single Bundle alone. It is the single validation entry point
// shared by the resource generator, layout walker, layout integrator, and the
// v1alpha1 converter round-trip.
//
// It enforces:
//  1. Every Node bundle passes Bundle.Validate (which recursively validates
//     umbrella Children subtrees including cycle detection).
//  2. Disjointness: a bundle pointer appearing inside any umbrella Children
//     subtree must NOT also be attached as the Bundle of any stack.Node.
//  3. No umbrella child pointer is shared by two distinct umbrella parents.
//  4. Multi-package rejection: if any Node has a PackageRef set and any
//     bundle in the cluster has umbrella Children, the cluster is rejected.
//     Cross-package umbrella semantics are follow-up work.
//
// ValidateCluster is safe to call with a nil cluster or a cluster with no
// root node (it returns nil in both cases).
func ValidateCluster(c *Cluster) error {
	if c == nil || c.Node == nil {
		return nil
	}

	// Collect every bundle pointer that is attached to a Node.
	nodeBundles := make(map[*Bundle]*Node)
	var walkNodes func(*Node)
	walkNodes = func(n *Node) {
		if n == nil {
			return
		}
		if n.Bundle != nil {
			nodeBundles[n.Bundle] = n
		}
		for _, ch := range n.Children {
			walkNodes(ch)
		}
	}
	walkNodes(c.Node)

	// 1. Validate every Node bundle. Bundle.Validate recursively walks the
	//    umbrella Children subtree.
	for b := range nodeBundles {
		if err := b.Validate(); err != nil {
			return errors.Wrapf(err, "bundle %q failed validation", b.Name)
		}
	}

	// 2 & 3. Walk every umbrella Children subtree to check disjointness with
	// the Node tree and no shared umbrella ownership.
	umbrellaOwnership := make(map[*Bundle]*Bundle)
	var collectUmbrella func(owner, b *Bundle) error
	collectUmbrella = func(owner, b *Bundle) error {
		for _, child := range b.Children {
			if child == nil {
				continue
			}
			if _, isNode := nodeBundles[child]; isNode {
				return errors.ResourceValidationError("Cluster", c.Name, "bundles",
					fmt.Sprintf("bundle %q is referenced by umbrella %q but also attached to a Node", child.Name, owner.Name),
					nil)
			}
			if prev, dup := umbrellaOwnership[child]; dup && prev != owner {
				return errors.ResourceValidationError("Cluster", c.Name, "bundles",
					fmt.Sprintf("bundle %q is child of two umbrellas: %q and %q", child.Name, prev.Name, owner.Name),
					nil)
			}
			umbrellaOwnership[child] = owner
			if err := collectUmbrella(owner, child); err != nil {
				return err
			}
		}
		return nil
	}
	for nb := range nodeBundles {
		if err := collectUmbrella(nb, nb); err != nil {
			return err
		}
	}

	// 4. Multi-package rejection: umbrella + PackageRef is out of scope for
	// this initial patch.
	if len(umbrellaOwnership) > 0 {
		hasPackageRef := false
		var scanPkg func(*Node)
		scanPkg = func(n *Node) {
			if n == nil || hasPackageRef {
				return
			}
			if n.PackageRef != nil {
				hasPackageRef = true
				return
			}
			for _, ch := range n.Children {
				scanPkg(ch)
			}
		}
		scanPkg(c.Node)
		if hasPackageRef {
			return errors.ResourceValidationError("Cluster", c.Name, "bundles",
				"umbrella bundles (Bundle.Children) are not supported with multi-package PackageRef in this release",
				nil)
		}
	}

	return nil
}
