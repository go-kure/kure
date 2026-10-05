package stack

import (
	"fmt"
	"slices"

	"github.com/go-kure/kure/pkg/errors"
)

// ValidateCluster performs cluster-level structural validation that cannot be
// expressed on a single Bundle alone. It is the single validation entry point
// shared by the resource generator, layout walker, layout integrator, and the
// v1alpha1 converter round-trip.
//
// It enforces:
//  0. The Node tree has no cycle: a Node reached again from one of its own
//     descendants is rejected, naming the node where the cycle closes.
//  1. Every Node bundle passes Bundle.Validate (which recursively validates
//     umbrella Children subtrees including cycle detection and the name of
//     each bundle), and every node name passes ValidateDirectoryName, under
//     every layout grouping: the name is a segment of the node's path. A root
//     node without a name is the one exception: it adds no segment. A
//     node's NamedDependsOn holds no empty and no repeated entry, as a
//     bundle's (validateNodeNamedDependsOn).
//  2. Disjointness: a bundle pointer appearing inside any umbrella Children
//     subtree must NOT also be attached as the Bundle of any stack.Node.
//  3. No umbrella child pointer is shared by two distinct umbrella parents.
//  4. Multi-package rejection: if any Node has a PackageRef set and any
//     bundle in the cluster has umbrella Children, the cluster is rejected.
//     Cross-package umbrella semantics are follow-up work.
//
// Before the bundles are validated (1), it checks the DependsOn bundles that
// are copies of the cluster's bundles, which Bundle.Validate cannot tell from
// any other bundle (see validateDependencyCopies): a copy that sets another
// KustomizationName than the bundle it stands for is refused, and so is one
// that stands for a bundle whose Kustomization is also in NamedDependsOn.
//
// ValidateCluster is safe to call with a nil cluster or a cluster with no
// root node (it returns nil in both cases).
func ValidateCluster(c *Cluster) error {
	if c == nil || c.Node == nil {
		return nil
	}

	// Collect every bundle pointer that is attached to a Node. onPath holds
	// the nodes on the current descent, so a Node reached again from below
	// itself is a cycle; done holds the nodes already walked, so a node
	// reachable from two parents is visited once rather than misreported as a
	// cycle. Every later walk of the Node tree may then iterate nodeOrder
	// instead of recursing again.
	//
	// The same walk checks every node name. A node name is a segment of the
	// node's path in the model (Node.GetPath and the path map join the names
	// with "/"), and of its directory when the layout gives it one. The check
	// therefore does not depend on the layout rules: a child that flat node
	// grouping absorbs into its parent's directory still has a path, and an
	// empty name or one holding "/" can give two nodes the same one. The
	// path the error reports is the one walked from the root, joined here
	// rather than read from Node.ParentPath, which a hand-built tree may not
	// have set. The root is exempt when it is unnamed: it adds no segment.
	nodeBundles := make(map[*Bundle]*Node)
	nodePaths := make(map[*Node]string)
	onPath := make(map[*Node]bool)
	done := make(map[*Node]bool)
	var nodeOrder []*Node
	var walkNodes func(n *Node, path string, root bool) error
	walkNodes = func(n *Node, path string, root bool) error {
		if n == nil || done[n] {
			return nil
		}
		if onPath[n] {
			return errors.ResourceValidationError("Cluster", c.Name, "nodes",
				fmt.Sprintf("node cycle detected at %q", n.Name), nil)
		}
		onPath[n] = true
		if !root || n.Name != "" {
			if err := ValidateDirectoryName(n.Name); err != nil {
				return errors.ResourceValidationError("Cluster", c.Name, "nodes",
					fmt.Sprintf("node %q: %v", path, err), nil)
			}
		}
		if err := validateNodeNamedDependsOn(n.NamedDependsOn); err != nil {
			return errors.ResourceValidationError("Cluster", c.Name, "nodes",
				fmt.Sprintf("node %q: %v", path, err), nil)
		}
		nodePaths[n] = path
		if n.Bundle != nil {
			nodeBundles[n.Bundle] = n
		}
		for _, ch := range n.Children {
			if ch == nil {
				continue
			}
			chPath := ch.Name
			if path != "" {
				chPath = path + "/" + ch.Name
			}
			if err := walkNodes(ch, chPath, false); err != nil {
				return err
			}
		}
		onPath[n] = false
		done[n] = true
		nodeOrder = append(nodeOrder, n)
		return nil
	}
	if err := walkNodes(c.Node, c.Node.Name, true); err != nil {
		return err
	}

	if err := validateDependencyCopies(c, nodeOrder); err != nil {
		return err
	}

	// 1. Validate every Node bundle. Bundle.Validate recursively walks the
	//    umbrella Children subtree, and checks the name of each bundle in it.
	for b, n := range nodeBundles {
		if err := b.Validate(); err != nil {
			return errors.Wrapf(err, "bundle %q at node %q failed validation", b.Name, nodePaths[n])
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
		hasPackageRef := slices.ContainsFunc(nodeOrder, func(n *Node) bool {
			return n.PackageRef != nil
		})
		if hasPackageRef {
			return errors.ResourceValidationError("Cluster", c.Name, "bundles",
				"umbrella bundles (Bundle.Children) are not supported with multi-package PackageRef in this release",
				nil)
		}
	}

	return nil
}

// validateNodeNamedDependsOn refuses an empty or a repeated entry in a node's
// NamedDependsOn, as Bundle.Validate does in a bundle's. The value of an entry
// is not checked: it is a caller-supplied reference to a Kustomization, which
// need not be one kure builds.
func validateNodeNamedDependsOn(names []string) error {
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if name == "" {
			return errors.New("NamedDependsOn holds an empty name")
		}
		if seen[name] {
			return errors.Errorf("NamedDependsOn lists %q twice", name)
		}
		seen[name] = true
	}
	return nil
}

// validateDependencyCopies checks the DependsOn bundles that are copies of
// the cluster's bundles: not one of them, but with the Name of one. A layout
// resolves a DependsOn bundle by its Name (layout.OriginIndex.UnitName), so a
// copy stands for the cluster's bundle of that name and the dependency is on
// that bundle's Kustomization, whatever the copy says. Two inputs are refused:
//
//   - a copy that sets a KustomizationName other than the name in effect of
//     the bundle (Bundle.UnitName). A copy that leaves it empty says nothing,
//     and is the bundle;
//   - a copy standing for a bundle whose Kustomization name (Bundle.UnitName)
//     is also in the dependant's NamedDependsOn: one dependency in both lists.
//
// Bundle.Validate compares a DependsOn bundle with NamedDependsOn on the name
// that bundle carries itself, so for a copy it reports the wrong name or
// misses the pair. This runs before it for that reason, and layout.IndexOrigins
// refuses the same inputs in the same words for a tree walked earlier.
//
// The cluster's bundles are each node's bundle and its umbrella descendants,
// nodes in the order given. Nothing is validated yet, so the walk ends on an
// umbrella cycle by itself and skips what Bundle.Validate refuses later (a nil
// child, a bundle without a name). Where two of the cluster's bundles have one
// Name a copy of that name stands for neither: that pair is the layout's to
// refuse.
func validateDependencyCopies(c *Cluster, nodeOrder []*Node) error {
	var bundles []*Bundle
	inCluster := make(map[*Bundle]bool)
	var collect func(b *Bundle)
	collect = func(b *Bundle) {
		if b == nil || inCluster[b] {
			return
		}
		inCluster[b] = true
		bundles = append(bundles, b)
		for _, child := range b.Children {
			collect(child)
		}
	}
	for _, n := range nodeOrder {
		collect(n.Bundle)
	}
	byName := make(map[string]*Bundle, len(bundles))
	shared := make(map[string]bool)
	for _, b := range bundles {
		if _, dup := byName[b.Name]; dup {
			shared[b.Name] = true
		}
		byName[b.Name] = b
	}
	for _, b := range bundles {
		for _, dep := range b.DependsOn {
			if dep == nil || dep.Name == "" || inCluster[dep] || shared[dep.Name] {
				continue
			}
			of := byName[dep.Name]
			if of == nil {
				continue
			}
			if reason := dependencyCopyRefusal(b, dep, of); reason != "" {
				return errors.ResourceValidationError("Cluster", c.Name, "bundles", reason, nil)
			}
		}
	}
	return nil
}

// dependencyCopyRefusal returns why dep, a copy of the cluster's bundle of in
// the DependsOn of b, is refused, or "" when it is not. layout.IndexOrigins
// reports the same two reasons in the same words.
//
// The names in effect are compared, not the fields: a copy that sets as its
// KustomizationName the Name of a bundle that sets none names the same
// Kustomization as that bundle.
func dependencyCopyRefusal(b, dep, of *Bundle) string {
	if dep.KustomizationName != "" && dep.UnitName() != of.UnitName() {
		return fmt.Sprintf("bundle %q depends on a copy of bundle %q with KustomizationName %q, but that bundle has KustomizationName %q: a DependsOn bundle is resolved by its Name, so a copy leaves KustomizationName empty or sets the bundle's",
			b.GetPath(), of.GetPath(), dep.KustomizationName, of.KustomizationName)
	}
	if slices.Contains(b.NamedDependsOn, of.UnitName()) {
		return fmt.Sprintf("bundle %q: dependency %q appears in both DependsOn and NamedDependsOn: its DependsOn bundle %q is resolved by its Name to the bundle whose Kustomization has that name",
			b.GetPath(), of.UnitName(), dep.Name)
	}
	return ""
}
