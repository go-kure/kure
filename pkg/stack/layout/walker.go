package layout

import (
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
)

// layoutPathSegments splits a layout's repo path into directory segments for
// use as the parent path of its children, so every child's Namespace is its
// parent's FullRepoPath() (go-kure/kure#771). A layout at the tree root "."
// yields []string{"."}, never nil: filepath.Join of no segments is "", which
// FullRepoPath would read as "cluster".
func layoutPathSegments(ml *ManifestLayout) []string {
	p := ml.FullRepoPath()
	if p == "." || p == "" {
		return []string{"."}
	}
	return strings.Split(p, "/")
}

// rootAncestors is the parent path of the root node when no ClusterName is
// set. A named root's parent is the tree root ".", so it sits at <root>. An
// unnamed root has no directory of its own; it keeps no parent and so resolves
// to "cluster", as before go-kure/kure#771. At "." it would look exactly like
// the ClusterName "." container, whose root kustomization.yaml WriteManifest
// skips, dropping the root's references to its children.
func rootAncestors(root *stack.Node) []string {
	if root.Name == "" {
		return nil
	}
	return []string{"."}
}

// grouping holds the settings one walk renders with. Each of the three
// grouping axes says whether its level gets a directory: a level whose axis is
// GroupFlat is rendered into the layout above it (its resources, its child
// layouts and its origins), so no setting is ever silently ignored. Umbrella
// child bundles and augmenter applications always get a directory, because
// they carry their own Flux Kustomization or writer-owned files.
type grouping struct {
	nodeFlat   bool
	bundleFlat bool
	appFlat    bool
	filePer    FileExportMode
	flux       FluxPlacement
	fileNaming FileNamingMode
	// pkgKey restricts the walk to the nodes of one package (WalkClusterByPackage);
	// "" walks every node. A node outside the package adds no directory: its
	// children are rendered where it would have been.
	pkgKey string
}

// newGrouping resolves rules, with unset options taking their documented
// defaults.
func newGrouping(rules LayoutRules) grouping {
	def := DefaultLayoutRules()
	if rules.NodeGrouping == GroupUnset {
		rules.NodeGrouping = def.NodeGrouping
	}
	if rules.BundleGrouping == GroupUnset {
		rules.BundleGrouping = def.BundleGrouping
	}
	if rules.ApplicationGrouping == GroupUnset {
		rules.ApplicationGrouping = def.ApplicationGrouping
	}
	if rules.FilePer == FilePerUnset {
		rules.FilePer = def.FilePer
	}
	return grouping{
		nodeFlat:   rules.NodeGrouping == GroupFlat,
		bundleFlat: rules.BundleGrouping == GroupFlat,
		appFlat:    rules.ApplicationGrouping == GroupFlat,
		filePer:    rules.FilePer,
		flux:       rules.FluxPlacement,
		fileNaming: rules.FileNaming,
	}
}

// includes reports whether a node whose package is pkg belongs to the walk.
func (g grouping) includes(pkg *schema.GroupVersionKind) bool {
	return g.pkgKey == "" || packageRefKey(pkg) == g.pkgKey
}

// newLayout returns a layout named name inside dir, carrying the walk's
// file, placement and naming settings.
func (g grouping) newLayout(name, dir string) *ManifestLayout {
	return &ManifestLayout{
		Name:          name,
		Namespace:     dir,
		FilePer:       g.filePer,
		FluxPlacement: g.flux,
		FileNaming:    g.fileNaming,
	}
}

// WalkCluster traverses a stack.Cluster and builds a ManifestLayout tree that
// mirrors the node, bundle and application hierarchy. Each grouping axis of
// rules decides whether its level gets a directory (see grouping).
func WalkCluster(c *stack.Cluster, rules LayoutRules) (*ManifestLayout, error) {
	if c == nil || c.Node == nil {
		return nil, nil
	}

	// Fail fast on umbrella / disjointness / multi-package violations.
	if err := stack.ValidateCluster(c); err != nil {
		return nil, err
	}

	if rules.FluxPlacement == FluxUnset {
		rules.FluxPlacement = DefaultLayoutRules().FluxPlacement
	}
	g := newGrouping(rules)

	// For cluster-aware layout, we need to restructure the hierarchy
	if rules.ClusterName != "" {
		ml, err := walkClusterWithClusterName(c, rules, g)
		if err != nil {
			return nil, err
		}
		return flattenSingleTier(ml, rules), nil
	}

	// Traditional layout without cluster name. The root node's parent is the
	// tree root ".", so the root sits at <root> and its children at
	// <root>/<child>, which is also where the Flux bootstrap sync path
	// ./<root> points. (With no parent at all, Namespace "" would put the root
	// alone at cluster/<root>, away from its children.) An unnamed root stays
	// at "cluster" (see rootAncestors).
	ml, err := walkNode(c.Node, rootAncestors(c.Node), g, nil)
	if err != nil {
		return nil, err
	}

	return flattenSingleTier(ml, rules), nil
}

// walkClusterWithClusterName creates a cluster-aware layout where the cluster
// name is the root directory and the root node (plus any child-node subtrees)
// are nested underneath it. The root node is rendered like every other node:
// its bundle, applications and child nodes follow the grouping axes.
func walkClusterWithClusterName(c *stack.Cluster, rules LayoutRules, g grouping) (*ManifestLayout, error) {
	// Create a cluster-level layout with the cluster name as the root.
	// It carries the placement like every other walked layout: under
	// FluxIntegratedPerLayout it hosts its children's Flux CRs (the root
	// bundle's, among others), so it must not also reference their
	// directories, which would apply them twice.
	clusterLayout := g.newLayout("", rules.ClusterName)

	// Unnamed root node: it has no directory of its own, so its content is
	// rendered straight into the cluster directory.
	if c.Node.Name == "" {
		clusterLayout.origin = origin{nodes: []*stack.Node{c.Node}}
		if err := renderNodeContent(c.Node, clusterLayout, g, nil); err != nil {
			return nil, err
		}
		return clusterLayout, nil
	}

	// Build the root node layout following the walker's path invariant
	// (Namespace = parent path, Name = this segment). The root's parent is the
	// cluster layer, so Namespace = ClusterName. When the cluster directory's
	// last segment is the node's name, the node IS that directory: its parent
	// is set explicitly to the directory above, so the root resolves to
	// ClusterName instead of <cluster>/<node> (and the wrapper is elided
	// below).
	rootNamespace := filepath.Clean(rules.ClusterName)
	if filepath.Base(rootNamespace) == c.Node.Name {
		rootNamespace = filepath.Dir(rootNamespace)
	}
	rootLayout := g.newLayout(c.Node.Name, rootNamespace)
	rootLayout.origin = origin{nodes: []*stack.Node{c.Node}}
	if err := renderNodeContent(c.Node, rootLayout, g, nil); err != nil {
		return nil, err
	}

	// Elide the synthetic cluster wrapper when it would occupy the same
	// directory as the root layout (e.g. ClusterName == Node.Name → both resolve
	// to "<name>"). Without this, clusterLayout and rootLayout share a
	// FullRepoPath() and WriteToDisk/WriteToTar emit a duplicate
	// kustomization.yaml. When the paths differ (e.g. ClusterName="." or a
	// distinct cluster name), keep the wrapper as before.
	if clusterLayout.FullRepoPath() == rootLayout.FullRepoPath() {
		return rootLayout, nil
	}

	clusterLayout.Children = append(clusterLayout.Children, rootLayout)

	return clusterLayout, nil
}

// WalkClusterByPackage traverses a stack.Cluster and builds separate ManifestLayout trees
// for each unique PackageRef (OCI artifact). Returns a map where keys are PackageRef GVKs
// and values are the corresponding ManifestLayout trees. Nodes without PackageRef inherit
// from their parent, with nil representing the default package. The grouping
// axes apply as in WalkCluster; the package trees carry no Flux placement.
func WalkClusterByPackage(c *stack.Cluster, rules LayoutRules) (map[string]*ManifestLayout, error) {
	if c == nil || c.Node == nil {
		return nil, nil
	}

	// Fail fast on umbrella / disjointness / multi-package violations.
	if err := stack.ValidateCluster(c); err != nil {
		return nil, err
	}

	rules.FluxPlacement = FluxUnset
	base := newGrouping(rules)

	// First pass: collect all unique package references
	packages := make(map[string]*schema.GroupVersionKind)
	collectPackageRefs(c.Node, nil, packages)

	// Second pass: build layouts for each package
	layouts := make(map[string]*ManifestLayout)
	for pkgKey := range packages {
		g := base
		g.pkgKey = pkgKey
		rootPkg := resolvePackageRef(c.Node, nil)
		var ml *ManifestLayout
		if g.includes(rootPkg) {
			// As in WalkCluster: a named root's parent is the tree root ".".
			var err error
			ml, err = walkNode(c.Node, rootAncestors(c.Node), g, nil)
			if err != nil {
				return nil, err
			}
		} else {
			// A root outside this package has no directory in it: like an
			// unnamed root, the package's nodes below it sit under a wrapper
			// at "cluster".
			ml = g.newLayout("", "")
			if err := renderChildren(c.Node.Children, ml, g, rootPkg); err != nil {
				return nil, err
			}
			// Nothing of this package lies below: no layout, resource or
			// merged node. A merged node with an empty bundle still counts.
			if len(ml.Children) == 0 && len(ml.Resources) == 0 && len(ml.origin.nodes) == 0 {
				ml = nil
			}
		}
		if ml != nil {
			layouts[pkgKey] = ml
		}
	}

	return layouts, nil
}

// walkNode returns the layout of node n, a directory inside ancestors, with
// n's bundle and child nodes rendered according to the grouping axes. pkg is
// the package n inherits from its parent.
func walkNode(n *stack.Node, ancestors []string, g grouping, pkg *schema.GroupVersionKind) (*ManifestLayout, error) {
	if n == nil {
		return nil, nil
	}
	ml := g.newLayout(n.Name, filepath.Join(ancestors...))
	ml.origin = origin{nodes: []*stack.Node{n}}
	if err := renderNodeContent(n, ml, g, pkg); err != nil {
		return nil, err
	}
	return ml, nil
}

// renderNodeContent renders node n's bundle and child nodes into into: n's own
// layout, or the layout that absorbs n when NodeGrouping is flat.
func renderNodeContent(n *stack.Node, into *ManifestLayout, g grouping, pkg *schema.GroupVersionKind) error {
	if n.Bundle != nil {
		if err := renderBundle(n.Bundle, into, g); err != nil {
			return err
		}
	}
	return renderChildren(n.Children, into, g, resolvePackageRef(n, pkg))
}

// renderChildren renders child nodes into into. With NodeGrouping flat a child
// is absorbed: its content goes into into and it becomes one of into's origin
// nodes. Otherwise it gets its own directory inside into's. A child outside the
// walked package adds no directory; its own children are rendered in its place.
func renderChildren(children []*stack.Node, into *ManifestLayout, g grouping, pkg *schema.GroupVersionKind) error {
	for _, child := range children {
		if child == nil {
			continue
		}
		childPkg := resolvePackageRef(child, pkg)
		if !g.includes(childPkg) {
			if err := renderChildren(child.Children, into, g, childPkg); err != nil {
				return err
			}
			continue
		}
		if g.nodeFlat {
			into.origin.nodes = append(into.origin.nodes, child)
			if err := renderNodeContent(child, into, g, pkg); err != nil {
				return err
			}
			continue
		}
		cl, err := walkNode(child, layoutPathSegments(into), g, pkg)
		if err != nil {
			return err
		}
		into.Children = append(into.Children, cl)
	}
	return nil
}

// renderBundle renders bundle b into into. With BundleGrouping flat the
// bundle's applications and umbrella children are rendered into into itself,
// which then renders b; otherwise b gets its own directory inside into's.
func renderBundle(b *stack.Bundle, into *ManifestLayout, g grouping) error {
	target := into
	if g.bundleFlat {
		into.origin.bundles = append(into.origin.bundles, b)
	} else {
		// Explicit, not Recursive: a Recursive layout with children lists
		// none of its own files, so a Flux CR hosted here (an umbrella
		// child's, a PerLayout application's) was never applied. The layout
		// has no own workloads, so the listing is otherwise unchanged.
		target = g.newLayout(b.Name, into.FullRepoPath())
		target.Mode = KustomizationExplicit
		target.origin = origin{bundles: []*stack.Bundle{b}}
		into.Children = append(into.Children, target)
	}
	if err := renderApps(b.Applications, target, g); err != nil {
		return err
	}
	if len(b.Children) > 0 {
		b.InitializeUmbrella()
		if err := renderUmbrellaChildren(b.Children, target, g); err != nil {
			return err
		}
	}
	return nil
}

// renderUmbrellaChildren renders umbrella child bundles as directories inside
// parent's. Each carries UmbrellaChild=true: the parent's kustomization.yaml
// does not list it, because the child is applied by its own Flux
// Kustomization (hosted in the parent under integrated placement, in
// flux-system under separate placement), whose spec.path is the child's
// directory. The child's applications follow ApplicationGrouping like any
// other bundle's; nested umbrellas recurse.
func renderUmbrellaChildren(children []*stack.Bundle, parent *ManifestLayout, g grouping) error {
	for _, cb := range children {
		if cb == nil {
			continue
		}
		ml := g.newLayout(cb.Name, parent.FullRepoPath())
		ml.Mode = KustomizationExplicit
		ml.UmbrellaChild = true
		ml.origin = origin{bundles: []*stack.Bundle{cb}}
		if err := renderApps(cb.Applications, ml, g); err != nil {
			return err
		}
		if len(cb.Children) > 0 {
			cb.InitializeUmbrella()
			if err := renderUmbrellaChildren(cb.Children, ml, g); err != nil {
				return err
			}
		}
		parent.Children = append(parent.Children, ml)
	}
	return nil
}

// renderApps renders applications into target, the layout of the directory
// that renders their bundle. With ApplicationGrouping flat an application's
// resources are written into target itself and it has no layout (or origin)
// of its own; an augmenter application that wants its own layout still gets
// one, so its extra files and generators do not collide with its siblings'.
// Otherwise every application gets its own directory inside target's, and a
// LayoutAugmenter is invoked on it.
func renderApps(apps []*stack.Application, target *ManifestLayout, g grouping) error {
	for _, app := range apps {
		if app == nil {
			continue
		}
		objsPtr, err := app.Generate()
		if err != nil {
			return err
		}
		var objs []client.Object
		for _, o := range objsPtr {
			if o == nil {
				continue
			}
			objs = append(objs, *o)
		}
		if g.appFlat && !isAugmenter(app) {
			target.Resources = append(target.Resources, objs...)
			continue
		}
		appLayout := g.newLayout(app.Name, target.FullRepoPath())
		appLayout.Resources = objs
		appLayout.Mode = KustomizationExplicit
		appLayout.origin = origin{app: app}
		if err := augmentAppLayout(app, appLayout); err != nil {
			return err
		}
		target.Children = append(target.Children, appLayout)
	}
	return nil
}

// augmentAppLayout invokes the LayoutAugmenter on app.Config when it satisfies
// the interface, attaching ExtraFiles and ConfigMapGenerators to the per-app
// ManifestLayout. It is a no-op when the config does not implement the
// interface.
func augmentAppLayout(app *stack.Application, ml *ManifestLayout) error {
	if app == nil || app.Config == nil {
		return nil
	}
	augmenter, ok := app.Config.(LayoutAugmenter)
	if !ok {
		return nil
	}
	if err := augmenter.AugmentLayout(ml); err != nil {
		return errors.Wrapf(err, "augment layout for application %q", app.Name)
	}
	return nil
}

// wantsOwnLayout reports whether an augmenting config wants its own layout.
// Absent companion interface ⇒ true (today's presence-only behaviour).
func wantsOwnLayout(cfg stack.ApplicationConfig) bool {
	if intent, ok := cfg.(LayoutIntentAugmenter); ok {
		return intent.WantsOwnLayout()
	}
	return true
}

// isAugmenter reports whether app.Config implements LayoutAugmenter and
// wants its own layout.
func isAugmenter(app *stack.Application) bool {
	if app == nil || app.Config == nil {
		return false
	}
	_, ok := app.Config.(LayoutAugmenter)
	return ok && wantsOwnLayout(app.Config)
}

// resolvePackageRef returns the effective PackageRef for a node, using inheritance from parent
func resolvePackageRef(n *stack.Node, inheritedPackageRef *schema.GroupVersionKind) *schema.GroupVersionKind {
	if n.PackageRef != nil {
		return n.PackageRef
	}
	return inheritedPackageRef
}

// packageRefKey converts a PackageRef to a string key for map indexing
func packageRefKey(ref *schema.GroupVersionKind) string {
	if ref == nil {
		return "default"
	}
	return ref.String()
}

// collectPackageRefs recursively traverses nodes to collect all unique PackageRef values
func collectPackageRefs(n *stack.Node, inheritedPackageRef *schema.GroupVersionKind, packages map[string]*schema.GroupVersionKind) {
	if n == nil {
		return
	}

	currentPackageRef := resolvePackageRef(n, inheritedPackageRef)
	key := packageRefKey(currentPackageRef)
	packages[key] = currentPackageRef

	for _, child := range n.Children {
		collectPackageRefs(child, currentPackageRef, packages)
	}
}
