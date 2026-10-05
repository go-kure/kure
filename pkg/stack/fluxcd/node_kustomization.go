package fluxcd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// nodeIndex names the nodes of one cluster and finds the layout whose
// FluxIntegratedPerLayout Kustomization is a node's own.
type nodeIndex struct {
	ix    *layout.OriginIndex
	paths map[*stack.Node]string
	order []*stack.Node
}

// newNodeIndex walks c's nodes. A node is named by the names from the root
// down to it, joined with "/": Node.GetPath gives that only where ParentPath
// was set, which a tree built from literals does not do. An unnamed root adds
// no segment.
func newNodeIndex(ix *layout.OriginIndex, c *stack.Cluster) *nodeIndex {
	nx := &nodeIndex{ix: ix, paths: map[*stack.Node]string{}}
	var walk func(n *stack.Node, parent string)
	walk = func(n *stack.Node, parent string) {
		if n == nil {
			return
		}
		if _, seen := nx.paths[n]; seen {
			return
		}
		p := parent
		switch {
		case n.Name == "":
		case parent == "":
			p = n.Name
		default:
			p = parent + "/" + n.Name
		}
		nx.paths[n] = p
		nx.order = append(nx.order, n)
		for _, child := range n.Children {
			walk(child, p)
		}
	}
	walk(c.Node, "")
	return nx
}

// path names node n in a message: its path in the cluster, or, for a node
// outside it, its own name.
func (nx *nodeIndex) path(n *stack.Node) string {
	if p, ok := nx.paths[n]; ok {
		return p
	}
	return n.GetPath()
}

// kustomizationLayout returns the layout whose per-layout Kustomization is
// node n's own, or why n has none: only a layout that is n's own, has a
// parent to host the CR and renders no bundle gets one (see place).
func (nx *nodeIndex) kustomizationLayout(n *stack.Node) (*layout.ManifestLayout, string) {
	l := nx.ix.NodeLayout(n)
	switch {
	case l == nil:
		return nil, "it is not a node of this cluster"
	case l.OriginNodes()[0] != n:
		return nil, fmt.Sprintf("it is rendered into the directory of node %q (NodeGrouping GroupFlat, or a FlattenSingleTier collapse)", nx.path(l.OriginNodes()[0]))
	case len(l.OriginBundles()) > 0:
		return nil, fmt.Sprintf("its directory %q renders bundle %q, and that bundle's Kustomization applies it", l.FullRepoPath(), l.OriginBundles()[0].GetPath())
	case nx.ix.Parent(l) == nil:
		return nil, "it is the root of the tree, which the Flux bootstrap applies"
	case !hasLayoutCR(l):
		// Not a walker's doing: a caller marked the node's layout. (One
		// marked AppFileSingle is refused by IndexOrigins before this.)
		return nil, fmt.Sprintf("its layout %q is marked UmbrellaChild, and such a layout gets no Kustomization of its own", l.FullRepoPath())
	}
	return l, ""
}

// nodeKustomizationFields lists the node-level Kustomization fields n sets.
func nodeKustomizationFields(n *stack.Node) []string {
	var set []string
	if n.KustomizationName != "" {
		set = append(set, "KustomizationName")
	}
	if len(n.DependsOn) > 0 {
		set = append(set, "DependsOn")
	}
	if len(n.NamedDependsOn) > 0 {
		set = append(set, "NamedDependsOn")
	}
	return set
}

// checkFields refuses KustomizationName, DependsOn or NamedDependsOn on a
// node that gets no Kustomization of its own: the fields would be ignored.
// perLayout says whether this generation places per-layout Kustomizations at
// all; only LayoutIntegrator under FluxIntegratedPerLayout does. A node that
// has one must depend on nodes that have one too (dependencies).
func (nx *nodeIndex) checkFields(perLayout bool) error {
	for _, n := range nx.order {
		set := nodeKustomizationFields(n)
		if len(set) == 0 {
			continue
		}
		fields := strings.Join(set, ", ")
		if !perLayout {
			return errors.Errorf("node %q sets %s, but a node has a Flux Kustomization of its own only where LayoutIntegrator places them under FluxPlacement %q (FluxIntegratedPerLayout): the fields would be ignored here; name and order the node's bundles instead",
				nx.path(n), fields, string(layout.FluxIntegratedPerLayout))
		}
		l, why := nx.kustomizationLayout(n)
		if why != "" {
			return errors.Errorf("node %q sets %s, but has no Flux Kustomization of its own: %s", nx.path(n), fields, why)
		}
		if err := nx.checkCarried(n, l); err != nil {
			return err
		}
		// Checked here and not only where the Kustomization is generated:
		// one kept from an earlier integration is not generated again.
		if _, err := nx.dependencies(n); err != nil {
			return err
		}
	}
	return nil
}

// checkCarried refuses a node whose own layout l does not carry what the node
// sets. The walker copies a node's KustomizationName and NamedDependsOn onto
// its layout, and the Kustomization is built from the layout: a name or an
// entry the node got after the walk is not there, and would be ignored. A
// layout that carries another name than the node's is not refused: a caller
// may name a walked layout, and that name wins.
//
// An empty or a repeated NamedDependsOn entry is refused here as well. The
// walk refuses one (stack.ValidateCluster), but IntegrateWithLayout does not
// validate the cluster again, and a node changed after the walk would
// otherwise put it into spec.dependsOn.
func (nx *nodeIndex) checkCarried(n *stack.Node, l *layout.ManifestLayout) error {
	if n.KustomizationName != "" && l.KustomizationName == "" {
		return errors.Errorf("node %q sets KustomizationName %q, but its layout %q carries no name: the node was named after the layout was walked; walk the cluster again, or set KustomizationName on the layout",
			nx.path(n), n.KustomizationName, l.FullRepoPath())
	}
	for i, name := range n.NamedDependsOn {
		if name == "" {
			return errors.Errorf("node %q: NamedDependsOn holds an empty name", nx.path(n))
		}
		if slices.Contains(n.NamedDependsOn[:i], name) {
			return errors.Errorf("node %q: NamedDependsOn lists %q twice", nx.path(n), name)
		}
		if !slices.Contains(l.DependsOn, name) {
			return errors.Errorf("node %q lists %q in NamedDependsOn, but its layout %q does not list it in DependsOn: the entry was added after the layout was walked; walk the cluster again, or add it to the layout's DependsOn",
				nx.path(n), name, l.FullRepoPath())
		}
	}
	return nil
}

// dependencies returns the names node n's DependsOn puts into spec.dependsOn:
// each target's Kustomization name in effect (layoutCRName). A target with no
// Kustomization of its own is refused, naming both nodes, and so is one whose
// Kustomization n also lists in NamedDependsOn: one dependency in both lists,
// as for a bundle (Bundle.Validate).
func (nx *nodeIndex) dependencies(n *stack.Node) ([]string, error) {
	var names []string
	for _, d := range n.DependsOn {
		if d == nil {
			continue
		}
		target, why := nx.kustomizationLayout(d)
		if why != "" {
			return nil, errors.Errorf("node %q depends on node %q, which has no Flux Kustomization of its own: %s", nx.path(n), nx.path(d), why)
		}
		name := layoutCRName(target, "")
		if slices.Contains(n.NamedDependsOn, name) {
			return nil, errors.Errorf("node %q: dependency %q appears in both DependsOn and NamedDependsOn: it is the Flux Kustomization of node %q",
				nx.path(n), name, nx.path(d))
		}
		names = append(names, name)
	}
	return names, nil
}
