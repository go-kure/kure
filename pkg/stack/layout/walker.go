package layout

import (
	"fmt"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
// they carry their own Flux Kustomization or writer-owned files. So do the
// bundles a flat BundleGrouping would render into the root node's layout (see
// rootUnit).
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
	// root is the walk's root node and what it rendered; nil in a walk that
	// does not render the root node. One walk shares it.
	root *rootUnit
}

// rootUnit keeps the root node's layout from rendering a bundle
// (go-kure/kure#979). A layout that renders bundles is applied by its own Flux
// Kustomization, which its parent hosts; the root node's layout can be the top
// of the tree, which has no parent, so its Kustomization would be part of the
// build it applies. With BundleGrouping flat, the bundles that would be
// rendered into the root node's layout (the root's own, and under a flat
// NodeGrouping every absorbed node's) are therefore rendered into one directory
// inside it, named after the first of them. They stay one unit: one directory,
// one Kustomization.
type rootUnit struct {
	node   *stack.Node
	layout *ManifestLayout
	unit   *ManifestLayout
}

// checkRootUnitName refuses a child node of the root that is named like the
// directory of the root's bundles: both would be written to one directory.
// The two directories are compared as the writers compare them (SameDirectory),
// so a node name that differs only in case is refused here too, not when the
// tree is written. A node name that is not one path segment ("/web", "./web")
// does not get here: stack.ValidateCluster refuses it before the walk. A
// layout below a child node that is rendered to that directory is refused the
// same way: see checkBelow.
//
// The bundle's own directory needs no check against the root node's: its name
// is a bundle's, which stack.ValidateCluster has checked before the walk, so
// it is one plain path segment (never ".", "..", or a name with a separator),
// and the root node's directory joined with one is a directory inside it.
func (r *rootUnit) checkRootUnitName() error {
	if r == nil || r.unit == nil {
		return nil
	}
	for _, child := range r.layout.Children {
		if child != r.unit && child.SameDirectory(r.unit) {
			return errors.ResourceValidationError("Node", child.Name, "name",
				fmt.Sprintf("node %q and bundle %q are both rendered to directory %q: the root node's bundle has a directory named after it, so no child node of the root may carry that name",
					child.Name, r.unit.Name, r.unit.FullRepoPath()), nil)
		}
	}
	for _, child := range r.layout.Children {
		if child == nil || child == r.unit {
			continue
		}
		if err := r.checkBelow(child); err != nil {
			return err
		}
	}
	return nil
}

// checkBelow refuses a layout below l that is rendered to the directory of the
// root's bundles. A node or bundle name is one path segment
// (stack.ValidateCluster), so the directory of a nested node or bundle lies at
// least two levels below the root node's and is never that one. Two kinds of
// layout can be: an application's own directory, whose name is checked only
// after this (checkApplicationDirs) and may climb out of its bundle's
// ("../core"), and a layout an application's LayoutAugmenter adds, which
// carries whatever Name and Namespace the augmenter gives it. The unit's own
// layouts are not walked: they lie below its directory.
func (r *rootUnit) checkBelow(l *ManifestLayout) error {
	for _, child := range l.Children {
		if child == nil {
			continue
		}
		if child.SameDirectory(r.unit) {
			return errors.ResourceValidationError("Bundle", r.unit.Name, "name",
				fmt.Sprintf("bundle %q would be rendered to directory %q, which %s further down the tree already takes: the root node's bundle has a directory named after it inside the root node's directory, so its name must name a directory nothing else is rendered to",
					r.unit.Name, r.unit.FullRepoPath(), child.origin.describe(child)), nil)
		}
		if err := r.checkBelow(child); err != nil {
			return err
		}
	}
	return nil
}

// describe says, for an error, what the walked layout l with origin o is the
// directory of: its application, its first node or its first bundle. A layout
// without any of them (a directory an augmenter added) is named as a layout.
func (o origin) describe(l *ManifestLayout) string {
	switch {
	case o.app != nil:
		return fmt.Sprintf("application %q", o.app.Name)
	case len(o.nodes) > 0:
		return fmt.Sprintf("node %q", o.nodes[0].Name)
	case len(o.bundles) > 0:
		return fmt.Sprintf("bundle %q", o.bundles[0].Name)
	}
	return fmt.Sprintf("layout %q", l.Name)
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
// rules decides whether its level gets a directory (see grouping). The rules
// are validated first, as given (LayoutRules.Validate); unset values then take
// their defaults.
func WalkCluster(c *stack.Cluster, rules LayoutRules) (*ManifestLayout, error) {
	if err := rules.Validate(); err != nil {
		return nil, err
	}
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
	g.root = &rootUnit{node: c.Node}

	// For cluster-aware layout, we need to restructure the hierarchy
	if rules.ClusterName != "" {
		ml, err := walkClusterWithClusterName(c, rules, g)
		if err != nil {
			return nil, err
		}
		if err := g.root.checkRootUnitName(); err != nil {
			return nil, err
		}
		return checkedLayout(flattenSingleTier(ml, rules))
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
	if err := g.root.checkRootUnitName(); err != nil {
		return nil, err
	}

	return checkedLayout(flattenSingleTier(ml, rules))
}

// checkedLayout returns the finished tree ml, or the error of
// checkApplicationDirs on it.
func checkedLayout(ml *ManifestLayout) (*ManifestLayout, error) {
	if err := checkApplicationDirs(ml); err != nil {
		return nil, err
	}
	return ml, nil
}

// checkApplicationDirs checks, with stack.ValidateDirectoryName, the name of
// every application that has a directory of its own in the finished tree ml.
// stack.ValidateCluster takes no layout rules and cannot know which
// applications get one, so the check is the walker's. It runs on the tree the
// walk returns, and so on the directories that are written: a layout with an
// application origin is that application's own directory, named after it
// (renderApps), and FlattenSingleTier collapses none, since it leaves every
// directory that renders a bundle and the directories below it.
func checkApplicationDirs(ml *ManifestLayout) error {
	if ml == nil {
		return nil
	}
	if app := ml.origin.app; app != nil {
		if err := stack.ValidateDirectoryName(app.Name); err != nil {
			return errors.ResourceValidationError("Application", app.Name, "name",
				fmt.Sprintf("it names a directory in %q: %v", ml.Namespace, err), nil)
		}
	}
	for _, child := range ml.Children {
		if err := checkApplicationDirs(child); err != nil {
			return err
		}
	}
	return nil
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
// The rules are validated first, as in WalkCluster, including the options
// this walk does not use.
func WalkClusterByPackage(c *stack.Cluster, rules LayoutRules) (map[string]*ManifestLayout, error) {
	if err := rules.Validate(); err != nil {
		return nil, err
	}
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
			// As in WalkCluster: a named root's parent is the tree root ".",
			// and the root node's layout renders no bundle (rootUnit). (This
			// walk places the root node without the ClusterName, and a node
			// outside the package adds no directory.)
			g.root = &rootUnit{node: c.Node}
			var err error
			ml, err = walkNode(c.Node, rootAncestors(c.Node), g, nil)
			if err != nil {
				return nil, err
			}
			if err := g.root.checkRootUnitName(); err != nil {
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
		if err := checkApplicationDirs(ml); err != nil {
			return nil, err
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
	if g.root != nil && n == g.root.node {
		g.root.layout = into
	}
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
// which then renders b; otherwise b gets its own directory inside into's. The
// root node's layout is the exception to the flat case: it renders no bundle,
// so b goes into the one directory inside it that the bundles merged there
// share (see rootUnit).
func renderBundle(b *stack.Bundle, into *ManifestLayout, g grouping) error {
	target := into
	switch {
	case g.bundleFlat && g.root != nil && into == g.root.layout:
		if g.root.unit == nil {
			// Explicit, as a bundle directory below: it can host Flux CRs
			// whose targets are directories inside it.
			g.root.unit = g.newLayout(b.Name, into.FullRepoPath())
			g.root.unit.Mode = KustomizationExplicit
			into.Children = append(into.Children, g.root.unit)
			into.origin.unit = g.root.unit
		}
		target = g.root.unit
		target.origin.bundles = append(target.origin.bundles, b)
	case g.bundleFlat:
		into.origin.bundles = append(into.origin.bundles, b)
	default:
		// Explicit, not Recursive: a Flux CR hosted here (an umbrella
		// child's, a PerLayout application's) targets a directory below
		// this one, and the writers refuse a Recursive build that would
		// include that target. The layout has no own workloads, so the
		// listing is otherwise unchanged.
		target = g.newLayout(b.Name, into.FullRepoPath())
		target.Mode = KustomizationExplicit
		target.origin = origin{bundles: []*stack.Bundle{b}}
		into.Children = append(into.Children, target)
	}
	objs, err := renderApps(b.Applications, target, g)
	if err != nil {
		return err
	}
	target.origin.addObjects(b, objs)
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
		objs, err := renderApps(cb.Applications, ml, g)
		if err != nil {
			return err
		}
		ml.origin.addObjects(cb, objs)
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
// LayoutAugmenter is invoked on it. The name of an application that gets a
// directory is checked once the tree is complete (checkApplicationDirs). It
// returns every object the applications emitted, wherever they were written.
func renderApps(apps []*stack.Application, target *ManifestLayout, g grouping) ([]client.Object, error) {
	var all []client.Object
	for _, app := range apps {
		if app == nil {
			continue
		}
		objsPtr, err := app.Generate()
		if err != nil {
			return nil, err
		}
		var objs []client.Object
		for _, o := range objsPtr {
			if o == nil {
				continue
			}
			objs = append(objs, *o)
		}
		all = append(all, objs...)
		if g.appFlat && !isAugmenter(app) {
			target.Resources = append(target.Resources, objs...)
			target.origin.apps = append(target.origin.apps, ApplicationObjects{Application: app, Objects: objs})
			continue
		}
		// Only here does the application's name become a directory. It is
		// checked once the tree is complete (checkApplicationDirs).
		appLayout := g.newLayout(app.Name, target.FullRepoPath())
		appLayout.Resources = objs
		appLayout.Mode = KustomizationExplicit
		appLayout.origin = origin{app: app}
		if err := augmentAppLayout(app, appLayout); err != nil {
			return nil, err
		}
		inheritFileNaming(appLayout)
		// Taken after the augmenter ran: what it added to the layout, or in
		// a layout below it, is the application's too, the ConfigMaps its
		// generators build included.
		generated := generatedStandIns(appLayout)
		for _, standIn := range generated {
			all = append(all, standIn.object)
		}
		target.origin.apps = append(target.origin.apps, ApplicationObjects{Application: app, Objects: subtreeResources(appLayout), Layout: appLayout, generated: generated})
		target.Children = append(target.Children, appLayout)
	}
	return all, nil
}

// generatedConfigMap stands for the ConfigMap a configMapGenerator entry
// makes when kustomize builds the directory, for matching patch targets: its
// identity (kind, and the name before the content-hash suffix) and, once
// origin.syncGenerated has run, the annotations the entry sets.
func generatedConfigMap(name string) client.Object {
	cm := &unstructured.Unstructured{}
	cm.SetAPIVersion("v1")
	cm.SetKind("ConfigMap")
	cm.SetName(name)
	return cm
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

// inheritFileNaming gives every layout below ml that leaves FileNaming unset
// its parent's, so the layouts an augmenter added are named like the tree
// they sit in. A layout that sets its own keeps it, and passes it on to the
// layouts below it.
func inheritFileNaming(ml *ManifestLayout) {
	for _, child := range ml.Children {
		if child == nil {
			continue
		}
		if child.FileNaming == FileNamingUnset {
			child.FileNaming = ml.FileNaming
		}
		inheritFileNaming(child)
	}
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
