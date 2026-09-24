package layout

import (
	"fmt"
	"slices"
	"strings"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
)

// origin records the stack objects a walked layout renders. Only the walkers
// and flattenSingleTier set it; a hand-built layout has none, which is how
// IndexOrigins tells a walked tree from a hand-built one.
type origin struct {
	// nodes whose directory this is (several after a nodeFlat merge or a
	// FlattenSingleTier collapse).
	nodes []*stack.Node
	// bundles whose resources live in this directory, in emission order.
	bundles []*stack.Bundle
	// app is the application of a per-app layout.
	app *stack.Application
}

// OriginNodes returns the stack nodes whose directory this layout is. Nil for
// a hand-built layout and for bundle, application and augmenter layouts.
func (ml *ManifestLayout) OriginNodes() []*stack.Node { return ml.origin.nodes }

// OriginBundles returns the bundles whose resources this layout's directory
// holds. Nil for a hand-built layout.
func (ml *ManifestLayout) OriginBundles() []*stack.Bundle { return ml.origin.bundles }

// OriginApplication returns the application a per-app layout renders, or nil.
func (ml *ManifestLayout) OriginApplication() *stack.Application { return ml.origin.app }

// rendersBundle reports whether a bundle's resources live in this layout's
// directory. A Flux Kustomization or ArgoCD Application names that directory,
// so the writers give it a kustomization.yaml even when it holds nothing
// else: an empty directory does not survive a Git tree. (The same holds for
// every child of a FluxIntegratedPerLayout layout; see writeToDisk.)
func (ml *ManifestLayout) rendersBundle() bool { return len(ml.origin.bundles) > 0 }

func (ml *ManifestLayout) hasNodeOrBundleOrigin() bool {
	return len(ml.origin.nodes) > 0 || len(ml.origin.bundles) > 0
}

// OriginIndex resolves the stack objects of one cluster to the layouts that
// render them. Build it with IndexOrigins.
type OriginIndex struct {
	bundleLayout map[*stack.Bundle]*ManifestLayout
	nodeLayout   map[*stack.Node]*ManifestLayout
	parent       map[*ManifestLayout]*ManifestLayout
	bundles      []*stack.Bundle
	nodes        []*stack.Node
	units        []*ManifestLayout
	byName       map[string]*stack.Bundle
}

// IndexOrigins indexes a layout tree walked from cluster c (WalkCluster) by
// the origins its layouts record. It refuses:
//   - a bundle or node rendered by two layouts;
//   - a tree whose rendered set differs from the bundles and nodes reachable
//     from c (node bundles plus their umbrella descendants), naming what is
//     missing or foreign — so a hand-built, partial or other-cluster tree is
//     refused;
//   - two bundles with one Name: a bundle's Flux Kustomization and ArgoCD
//     Application are named after it, so the name is its identity;
//   - a layout rendering a node or bundle in AppFileSingle mode: it is
//     written into its Namespace, not into its own directory;
//   - a dependency cycle between reconciliation units (see Units).
func IndexOrigins(root *ManifestLayout, c *stack.Cluster) (*OriginIndex, error) {
	if root == nil || c == nil || c.Node == nil {
		return nil, errors.New("IndexOrigins: nil layout or cluster")
	}
	ix := &OriginIndex{
		bundleLayout: map[*stack.Bundle]*ManifestLayout{},
		nodeLayout:   map[*stack.Node]*ManifestLayout{},
		parent:       map[*ManifestLayout]*ManifestLayout{},
	}
	byName := map[string]*stack.Bundle{}
	var walk func(l, parent *ManifestLayout) error
	walk = func(l, parent *ManifestLayout) error {
		if parent != nil {
			ix.parent[l] = parent
		}
		if l.hasNodeOrBundleOrigin() && l.ApplicationFileMode == AppFileSingle {
			return errors.Errorf("layout %q renders a node or bundle but is AppFileSingle: its files are written into %q, not into its own directory", l.FullRepoPath(), l.Namespace)
		}
		for _, n := range l.origin.nodes {
			if other, dup := ix.nodeLayout[n]; dup {
				return errors.Errorf("node %q is rendered by two layouts: %q and %q", n.Name, other.FullRepoPath(), l.FullRepoPath())
			}
			ix.nodeLayout[n] = l
			ix.nodes = append(ix.nodes, n)
		}
		for _, b := range l.origin.bundles {
			if other, dup := ix.bundleLayout[b]; dup {
				return errors.Errorf("bundle %q is rendered by two layouts: %q and %q", b.Name, other.FullRepoPath(), l.FullRepoPath())
			}
			if other, dup := byName[b.Name]; dup && other != b {
				return errors.Errorf("two bundles are named %q (at %q and %q): the name is the Flux Kustomization and ArgoCD Application identity, so it must be unique", b.Name, ix.bundleLayout[other].FullRepoPath(), l.FullRepoPath())
			}
			byName[b.Name] = b
			ix.bundleLayout[b] = l
			ix.bundles = append(ix.bundles, b)
		}
		if len(l.origin.bundles) > 0 {
			ix.units = append(ix.units, l)
		}
		for _, child := range l.Children {
			if child == nil {
				continue
			}
			if err := walk(child, l); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root, nil); err != nil {
		return nil, err
	}
	ix.byName = byName
	if err := ix.checkCoverage(c); err != nil {
		return nil, err
	}
	if err := ix.checkUnitCycles(); err != nil {
		return nil, err
	}
	return ix, nil
}

// Units returns the layouts that render bundles, in layout pre-order. Each is
// one reconciliation unit: the directory is what a Flux Kustomization or an
// ArgoCD Application applies, so the bundles a grouping axis merged into one
// directory share one, named after the first of them (UnitName).
func (ix *OriginIndex) Units() []*ManifestLayout { return ix.units }

// UnitName returns the name of the reconciliation unit that applies bundle b:
// the first bundle the layout rendering b renders. b is resolved by name,
// which IndexOrigins proves unique, so a copy of a rendered bundle (the
// fluent builder copies bundles) resolves like the original. A bundle outside
// the index keeps its own name.
func (ix *OriginIndex) UnitName(b *stack.Bundle) string {
	return ix.UnitOfName(b.Name)
}

// UnitOfName maps a bundle name to the name of the unit that applies it; a
// name no rendered bundle has (an external Kustomization, say) is returned
// unchanged. References to a bundle's Kustomization by name — health checks,
// dependencies — resolve through it.
func (ix *OriginIndex) UnitOfName(name string) string {
	if b := ix.byName[name]; b != nil {
		if l := ix.bundleLayout[b]; l != nil && len(l.origin.bundles) > 0 {
			return l.origin.bundles[0].Name
		}
	}
	return name
}

// UnitDependencies returns the units the unit of layout l depends on through
// the DependsOn of the bundles it renders: each dependency mapped to the unit
// that applies it, in order and without repeats. A dependency applied by l's
// own unit is dropped: the bundles are applied together.
func (ix *OriginIndex) UnitDependencies(l *ManifestLayout) []string {
	var names []string
	for _, b := range l.origin.bundles {
		for _, d := range b.DependsOn {
			if d != nil {
				names = append(names, d.Name)
			}
		}
	}
	return ix.mapToUnits(l, names)
}

// UnitNamedDependencies is UnitDependencies for the bundles' NamedDependsOn:
// a name that is a rendered bundle's is mapped to its unit (and dropped when
// that is l's own), any other name is kept as the external dependency it is.
func (ix *OriginIndex) UnitNamedDependencies(l *ManifestLayout) []string {
	var names []string
	for _, b := range l.origin.bundles {
		names = append(names, b.NamedDependsOn...)
	}
	return ix.mapToUnits(l, names)
}

func (ix *OriginIndex) mapToUnits(l *ManifestLayout, names []string) []string {
	self := ix.UnitOfName(l.origin.bundles[0].Name)
	seen := map[string]bool{self: true}
	var out []string
	for _, name := range names {
		if unit := ix.UnitOfName(name); !seen[unit] {
			seen[unit] = true
			out = append(out, unit)
		}
	}
	return out
}

// checkUnitCycles refuses a cycle in what units wait for: Flux and ArgoCD
// would wait on each other forever. A unit waits for its DependsOn and
// NamedDependsOn units and, through health checks, for the units of its
// bundles' umbrella children. Merging bundles into one unit can close a cycle
// their own dependencies did not have (b1 -> u -> b2 becomes rb -> u -> rb
// when b1 and b2 merge into rb's directory; an umbrella child that depends on
// a bundle merged into its parent's unit waits for a unit that waits for it).
func (ix *OriginIndex) checkUnitCycles() error {
	deps := map[string][]string{}
	for _, l := range ix.units {
		name := ix.UnitName(l.origin.bundles[0])
		waits := append(ix.UnitDependencies(l), ix.UnitNamedDependencies(l)...)
		for _, b := range l.origin.bundles {
			for _, child := range b.Children {
				if child != nil {
					if unit := ix.UnitName(child); unit != name {
						waits = append(waits, unit)
					}
				}
			}
		}
		deps[name] = waits
	}
	const (
		visiting = 1
		done     = 2
	)
	state := map[string]int{}
	var path []string
	var visit func(n string) error
	visit = func(n string) error {
		switch state[n] {
		case done:
			return nil
		case visiting:
			i := slices.Index(path, n)
			return errors.Errorf("dependency cycle between reconciliation units: %s", strings.Join(append(path[i:], n), " -> "))
		}
		state[n] = visiting
		path = append(path, n)
		for _, d := range deps[n] {
			if _, unit := deps[d]; !unit {
				continue // a dependency outside the tree
			}
			if err := visit(d); err != nil {
				return err
			}
		}
		path = path[:len(path)-1]
		state[n] = done
		return nil
	}
	for _, l := range ix.units {
		if err := visit(ix.UnitName(l.origin.bundles[0])); err != nil {
			return err
		}
	}
	return nil
}

// checkCoverage compares the rendered set with what is reachable from c.
func (ix *OriginIndex) checkCoverage(c *stack.Cluster) error {
	reachNodes := map[*stack.Node]bool{}
	reachBundles := map[*stack.Bundle]bool{}
	var missing []string
	var umbrella func(b *stack.Bundle)
	umbrella = func(b *stack.Bundle) {
		if b == nil || reachBundles[b] {
			return
		}
		reachBundles[b] = true
		if ix.bundleLayout[b] == nil {
			missing = append(missing, fmt.Sprintf("bundle %q", b.Name))
		}
		for _, ch := range b.Children {
			umbrella(ch)
		}
	}
	var nodes func(n *stack.Node)
	nodes = func(n *stack.Node) {
		if n == nil || reachNodes[n] {
			return
		}
		reachNodes[n] = true
		if ix.nodeLayout[n] == nil {
			missing = append(missing, fmt.Sprintf("node %q", n.Name))
		}
		umbrella(n.Bundle)
		for _, ch := range n.Children {
			nodes(ch)
		}
	}
	nodes(c.Node)
	// Foreign objects first: a tree walked from another cluster is missing
	// everything, and saying so would hide the actual mistake.
	for _, n := range ix.nodes {
		if !reachNodes[n] {
			return errors.Errorf("layout %q renders node %q, which is not reachable from cluster %q", ix.nodeLayout[n].FullRepoPath(), n.Name, c.Name)
		}
	}
	for _, b := range ix.bundles {
		if !reachBundles[b] {
			return errors.Errorf("layout %q renders bundle %q, which is not reachable from cluster %q", ix.bundleLayout[b].FullRepoPath(), b.Name, c.Name)
		}
	}
	if len(missing) > 0 {
		return errors.Errorf("layout does not render cluster %q: missing %s; build it with layout.WalkCluster", c.Name, strings.Join(missing, ", "))
	}
	return nil
}

// BundleLayout returns the layout whose directory holds b's resources, or nil.
func (ix *OriginIndex) BundleLayout(b *stack.Bundle) *ManifestLayout { return ix.bundleLayout[b] }

// NodeLayout returns the layout whose directory is n's, or nil.
func (ix *OriginIndex) NodeLayout(n *stack.Node) *ManifestLayout { return ix.nodeLayout[n] }

// Parent returns ml's parent layout in the indexed tree, or nil for the root.
func (ix *OriginIndex) Parent(ml *ManifestLayout) *ManifestLayout { return ix.parent[ml] }

// Bundles returns every rendered bundle in layout pre-order (a layout's own
// bundles before its children's).
func (ix *OriginIndex) Bundles() []*stack.Bundle { return ix.bundles }

// KustomizationPath returns the directory of the layout that renders b: the
// one path rule for a bundle's Flux Kustomization spec.path and ArgoCD
// Application source.path, relative to the writer's output root.
func (ix *OriginIndex) KustomizationPath(b *stack.Bundle) (string, error) {
	l := ix.bundleLayout[b]
	if l == nil {
		name := "<nil>"
		if b != nil {
			name = b.Name
		}
		return "", errors.Errorf("bundle %q is not rendered by the indexed layout", name)
	}
	return l.FullRepoPath(), nil
}
