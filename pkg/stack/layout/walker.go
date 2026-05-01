package layout

import (
	"path/filepath"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
)

// WalkCluster traverses a stack.Cluster and builds a ManifestLayout tree that
// mirrors the node and bundle hierarchy. Behaviour is controlled via
// LayoutRules. When BundleGrouping and ApplicationGrouping are set to
// GroupFlat, all application resources are written directly to their parent
// node's directory.
func WalkCluster(c *stack.Cluster, rules LayoutRules) (*ManifestLayout, error) {
	if c == nil || c.Node == nil {
		return nil, nil
	}

	// Fail fast on umbrella / disjointness / multi-package violations.
	if err := stack.ValidateCluster(c); err != nil {
		return nil, err
	}

	// Apply documented defaults for unset options.
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
	if rules.FluxPlacement == FluxUnset {
		rules.FluxPlacement = def.FluxPlacement
	}

	nodeOnly := rules.BundleGrouping == GroupFlat && rules.ApplicationGrouping == GroupFlat
	nodeFlat := rules.NodeGrouping == GroupFlat
	filePer := rules.FilePer
	if nodeOnly {
		filePer = FilePerResource
	}

	// For cluster-aware layout, we need to restructure the hierarchy
	if rules.ClusterName != "" {
		return walkClusterWithClusterName(c, rules, nodeOnly, filePer)
	}

	// Traditional layout without cluster name
	ml, err := walkNode(c.Node, nil, nodeOnly, nodeFlat, filePer, nil, rules.FluxPlacement, rules.FileNaming)
	if err != nil {
		return nil, err
	}

	return ml, nil
}

// walkClusterWithClusterName creates a cluster-aware layout where the cluster
// name is the root directory and the root node (plus any child-node subtrees)
// are nested underneath it. Child-node sub-layouts are placed under the root
// node layout (not as cluster-level siblings) so their accumulated layout
// path matches stack.Node.GetPath() — the Flux integrator's path-based lookup
// relies on this correspondence.
func walkClusterWithClusterName(c *stack.Cluster, rules LayoutRules, nodeOnly bool, filePer FileExportMode) (*ManifestLayout, error) {
	// Create a cluster-level layout with the cluster name as the root
	clusterLayout := &ManifestLayout{
		Name:       "",
		Namespace:  rules.ClusterName,
		FilePer:    filePer,
		FileNaming: rules.FileNaming,
		Children:   []*ManifestLayout{},
	}

	nodeFlat := rules.NodeGrouping == GroupFlat

	// Unnamed root node: resources go directly at the cluster root with no
	// intermediate subdirectory. The clusterLayout itself holds the bundle's
	// resources so WriteToDisk writes a single directory (no path collision).
	if c.Node.Name == "" {
		if c.Node.Bundle != nil {
			for _, app := range c.Node.Bundle.Applications {
				if app == nil {
					continue
				}
				objsPtr, err := app.Generate()
				if err != nil {
					return nil, err
				}
				for _, o := range objsPtr {
					if o == nil {
						continue
					}
					clusterLayout.Resources = append(clusterLayout.Resources, *o)
				}
			}
			if len(c.Node.Bundle.Children) > 0 {
				c.Node.Bundle.InitializeUmbrella()
				umbrellaChildren, err := walkUmbrellaChildLayouts(
					c.Node.Bundle.Children,
					[]string{rules.ClusterName},
					filePer,
					rules.FluxPlacement,
					rules.FileNaming,
				)
				if err != nil {
					return nil, err
				}
				clusterLayout.Children = append(clusterLayout.Children, umbrellaChildren...)
			}
		}
		for _, child := range c.Node.Children {
			childLayout, err := walkNode(child, []string{rules.ClusterName}, nodeOnly, nodeFlat, filePer, nil, rules.FluxPlacement, rules.FileNaming)
			if err != nil {
				return nil, err
			}
			if childLayout != nil {
				clusterLayout.Children = append(clusterLayout.Children, childLayout)
			}
		}
		return clusterLayout, nil
	}

	// Build the root node layout. Done unconditionally (even when the root
	// node has no Bundle) so child-node subtrees can be nested underneath it.
	rootLayout := &ManifestLayout{
		Name:       c.Node.Name,
		Namespace:  filepath.Join(rules.ClusterName, c.Node.Name),
		FilePer:    filePer,
		FileNaming: rules.FileNaming,
		Children:   []*ManifestLayout{},
	}

	if c.Node.Bundle != nil {
		// Add only the root node's bundle resources (not child resources)
		for _, app := range c.Node.Bundle.Applications {
			if app == nil {
				continue
			}
			objsPtr, err := app.Generate()
			if err != nil {
				return nil, err
			}
			for _, o := range objsPtr {
				if o == nil {
					continue
				}
				rootLayout.Resources = append(rootLayout.Resources, *o)
			}
		}

		// Umbrella children of the root node's bundle become sub-layouts of
		// the root node layout (cluster-name root dir → rootNode → children).
		if len(c.Node.Bundle.Children) > 0 {
			c.Node.Bundle.InitializeUmbrella()
			umbrellaChildren, err := walkUmbrellaChildLayouts(
				c.Node.Bundle.Children,
				[]string{rules.ClusterName, c.Node.Name},
				filePer,
				rules.FluxPlacement,
				rules.FileNaming,
			)
			if err != nil {
				return nil, err
			}
			rootLayout.Children = append(rootLayout.Children, umbrellaChildren...)
		}
	}

	// Nest child-node sub-layouts under the root node layout so their
	// accumulated path (clusterName/rootName/childName/...) matches
	// stack.Node.GetPath() (rootName/childName/...) when the Flux integrator
	// searches for the corresponding layout node.
	for _, child := range c.Node.Children {
		childLayout, err := walkNode(child, []string{rules.ClusterName, c.Node.Name}, nodeOnly, nodeFlat, filePer, nil, rules.FluxPlacement, rules.FileNaming)
		if err != nil {
			return nil, err
		}
		if childLayout != nil {
			rootLayout.Children = append(rootLayout.Children, childLayout)
		}
	}

	clusterLayout.Children = append(clusterLayout.Children, rootLayout)

	return clusterLayout, nil
}

// WalkClusterByPackage traverses a stack.Cluster and builds separate ManifestLayout trees
// for each unique PackageRef (OCI artifact). Returns a map where keys are PackageRef GVKs
// and values are the corresponding ManifestLayout trees. Nodes without PackageRef inherit
// from their parent, with nil representing the default package.
func WalkClusterByPackage(c *stack.Cluster, rules LayoutRules) (map[string]*ManifestLayout, error) {
	if c == nil || c.Node == nil {
		return nil, nil
	}

	// Fail fast on umbrella / disjointness / multi-package violations.
	if err := stack.ValidateCluster(c); err != nil {
		return nil, err
	}

	// Apply documented defaults for unset options.
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

	nodeOnly := rules.BundleGrouping == GroupFlat && rules.ApplicationGrouping == GroupFlat
	filePer := rules.FilePer
	if nodeOnly {
		filePer = FilePerResource
	}

	// First pass: collect all unique package references
	packages := make(map[string]*schema.GroupVersionKind)
	collectPackageRefs(c.Node, nil, packages)

	// Second pass: build layouts for each package
	layouts := make(map[string]*ManifestLayout)
	for pkgKey, pkgRef := range packages {
		layout, err := walkNodeForPackage(c.Node, nil, nodeOnly, filePer, pkgRef, pkgKey, rules.FileNaming)
		if err != nil {
			return nil, err
		}
		if layout != nil {
			layouts[pkgKey] = layout
		}
	}

	return layouts, nil
}

// walkNode recursively processes a stack.Node and its children.
// When nodeFlat is true, child nodes do not create subdirectories; their
// resources are merged into the parent ManifestLayout.
func walkNode(n *stack.Node, ancestors []string, nodeOnly bool, nodeFlat bool, filePer FileExportMode, inheritedPackageRef *schema.GroupVersionKind, fluxPlacement FluxPlacement, fileNaming FileNamingMode) (*ManifestLayout, error) {
	if n == nil {
		return nil, nil
	}

	currentPath := append([]string{}, ancestors...)
	if n.Name != "" {
		currentPath = append(currentPath, n.Name)
	}

	ml := &ManifestLayout{
		Name:          n.Name,
		Namespace:     filepath.Join(ancestors...),
		FilePer:       filePer,
		FluxPlacement: fluxPlacement,
		FileNaming:    fileNaming,
	}

	if nodeOnly {
		if b := n.Bundle; b != nil {
			for _, app := range b.Applications {
				if app == nil {
					continue
				}
				objsPtr, err := app.Generate()
				if err != nil {
					return nil, err
				}
				for _, o := range objsPtr {
					if o == nil {
						continue
					}
					ml.Resources = append(ml.Resources, *o)
				}
			}
			// Umbrella: umbrella child sub-layouts live directly under the
			// node layout in nodeOnly mode (no intermediate bundle layer).
			if len(b.Children) > 0 {
				b.InitializeUmbrella()
				umbrellaChildren, err := walkUmbrellaChildLayouts(b.Children, currentPath, filePer, fluxPlacement, fileNaming)
				if err != nil {
					return nil, err
				}
				ml.Children = append(ml.Children, umbrellaChildren...)
			}
		}
	} else {
		var children []*ManifestLayout
		if b := n.Bundle; b != nil {
			var bundleChildren []*ManifestLayout
			for _, app := range b.Applications {
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
				appLayout := &ManifestLayout{
					Name:          app.Name,
					Namespace:     filepath.Join(append(currentPath, b.Name)...),
					Resources:     objs,
					Mode:          KustomizationExplicit,
					FluxPlacement: fluxPlacement,
					FileNaming:    fileNaming,
				}
				bundleChildren = append(bundleChildren, appLayout)
			}
			// Umbrella: umbrella child sub-layouts are siblings of application
			// sub-layouts within the bundle's layout directory.
			if len(b.Children) > 0 {
				b.InitializeUmbrella()
				umbrellaChildren, err := walkUmbrellaChildLayouts(b.Children, append(currentPath, b.Name), filePer, fluxPlacement, fileNaming)
				if err != nil {
					return nil, err
				}
				bundleChildren = append(bundleChildren, umbrellaChildren...)
			}
			bundleLayout := &ManifestLayout{
				Name:          b.Name,
				Namespace:     filepath.Join(currentPath...),
				Children:      bundleChildren,
				Mode:          KustomizationRecursive,
				FluxPlacement: fluxPlacement,
				FileNaming:    fileNaming,
			}
			children = append(children, bundleLayout)
		}

		for _, child := range n.Children {
			cl, err := walkNode(child, currentPath, nodeOnly, nodeFlat, filePer, resolvePackageRef(n, inheritedPackageRef), fluxPlacement, fileNaming)
			if err != nil {
				return nil, err
			}
			if cl != nil {
				children = append(children, cl)
			}
		}

		ml.Children = children
	}

	if nodeOnly {
		for _, child := range n.Children {
			if nodeFlat {
				// Merge child node resources directly into this node
				cl, err := walkNode(child, ancestors, nodeOnly, nodeFlat, filePer, resolvePackageRef(n, inheritedPackageRef), fluxPlacement, fileNaming)
				if err != nil {
					return nil, err
				}
				if cl != nil {
					ml.Resources = append(ml.Resources, cl.Resources...)
					// Recursively collect from grandchildren too
					for _, gc := range cl.Children {
						ml.Resources = append(ml.Resources, gc.Resources...)
					}
				}
			} else {
				cl, err := walkNode(child, currentPath, nodeOnly, nodeFlat, filePer, resolvePackageRef(n, inheritedPackageRef), fluxPlacement, fileNaming)
				if err != nil {
					return nil, err
				}
				if cl != nil {
					ml.Children = append(ml.Children, cl)
				}
			}
		}
	}

	return ml, nil
}

// walkUmbrellaChildLayouts renders a slice of umbrella Bundle.Children into a
// flat ManifestLayout list. Each returned layout carries UmbrellaChild=true so
// downstream writers emit a flux-system-kustomization-{Name}.yaml reference
// in the parent directory instead of descending into a subdirectory for the
// Flux CR. Child application resources are flattened into the child layout's
// Resources (single-directory-per-child on disk). Nested umbrellas recurse so
// grandchildren become sub-layouts of their immediate parent umbrella child.
func walkUmbrellaChildLayouts(children []*stack.Bundle, currentPath []string, filePer FileExportMode, fluxPlacement FluxPlacement, fileNaming FileNamingMode) ([]*ManifestLayout, error) {
	var out []*ManifestLayout
	for _, cb := range children {
		if cb == nil {
			continue
		}
		ml := &ManifestLayout{
			Name:          cb.Name,
			Namespace:     filepath.Join(currentPath...),
			FilePer:       filePer,
			FluxPlacement: fluxPlacement,
			FileNaming:    fileNaming,
			Mode:          KustomizationExplicit,
			UmbrellaChild: true,
		}
		for _, app := range cb.Applications {
			if app == nil {
				continue
			}
			objsPtr, err := app.Generate()
			if err != nil {
				return nil, err
			}
			for _, o := range objsPtr {
				if o == nil {
					continue
				}
				ml.Resources = append(ml.Resources, *o)
			}
		}
		if len(cb.Children) > 0 {
			cb.InitializeUmbrella()
			nested, err := walkUmbrellaChildLayouts(cb.Children, append(currentPath, cb.Name), filePer, fluxPlacement, fileNaming)
			if err != nil {
				return nil, err
			}
			ml.Children = append(ml.Children, nested...)
		}
		out = append(out, ml)
	}
	return out, nil
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

// walkNodeForPackage walks the tree but only includes nodes that belong to the specified package
func walkNodeForPackage(n *stack.Node, ancestors []string, nodeOnly bool, filePer FileExportMode, targetPackageRef *schema.GroupVersionKind, targetKey string, fileNaming FileNamingMode) (*ManifestLayout, error) {
	return walkNodeForPackageInternal(n, ancestors, nodeOnly, filePer, nil, targetPackageRef, targetKey, fileNaming)
}

// walkNodeForPackageInternal is the internal implementation with inheritance tracking
func walkNodeForPackageInternal(n *stack.Node, ancestors []string, nodeOnly bool, filePer FileExportMode, inheritedPackageRef *schema.GroupVersionKind, targetPackageRef *schema.GroupVersionKind, targetKey string, fileNaming FileNamingMode) (*ManifestLayout, error) {
	if n == nil {
		return nil, nil
	}

	currentPackageRef := resolvePackageRef(n, inheritedPackageRef)

	// Check if this node belongs to the target package
	belongsToPackage := packageRefKey(currentPackageRef) == targetKey

	currentPath := append([]string{}, ancestors...)
	if n.Name != "" && belongsToPackage {
		currentPath = append(currentPath, n.Name)
	}

	var ml *ManifestLayout
	if belongsToPackage {
		ml = &ManifestLayout{
			Name:       n.Name,
			Namespace:  filepath.Join(ancestors...),
			FilePer:    filePer,
			FileNaming: fileNaming,
		}

		if nodeOnly {
			if b := n.Bundle; b != nil {
				for _, app := range b.Applications {
					if app == nil {
						continue
					}
					objsPtr, err := app.Generate()
					if err != nil {
						return nil, err
					}
					for _, o := range objsPtr {
						if o == nil {
							continue
						}
						ml.Resources = append(ml.Resources, *o)
					}
				}
			}
		} else {
			var children []*ManifestLayout
			if b := n.Bundle; b != nil {
				var bundleChildren []*ManifestLayout
				for _, app := range b.Applications {
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
					appLayout := &ManifestLayout{
						Name:       app.Name,
						Namespace:  filepath.Join(append(currentPath, b.Name)...),
						Resources:  objs,
						FileNaming: fileNaming,
					}
					bundleChildren = append(bundleChildren, appLayout)
				}
				if len(bundleChildren) > 0 {
					bundleLayout := &ManifestLayout{
						Name:       b.Name,
						Namespace:  filepath.Join(currentPath...),
						Children:   bundleChildren,
						FileNaming: fileNaming,
					}
					children = append(children, bundleLayout)
				}
			}

			for _, child := range n.Children {
				cl, err := walkNodeForPackageInternal(child, currentPath, nodeOnly, filePer, currentPackageRef, targetPackageRef, targetKey, fileNaming)
				if err != nil {
					return nil, err
				}
				if cl != nil {
					children = append(children, cl)
				}
			}

			ml.Children = children
		}

		if nodeOnly {
			for _, child := range n.Children {
				cl, err := walkNodeForPackageInternal(child, currentPath, nodeOnly, filePer, currentPackageRef, targetPackageRef, targetKey, fileNaming)
				if err != nil {
					return nil, err
				}
				if cl != nil {
					ml.Children = append(ml.Children, cl)
				}
			}
		}
	} else {
		// Node doesn't belong to target package, but continue traversing children
		// in case they have different PackageRef values
		for _, child := range n.Children {
			cl, err := walkNodeForPackageInternal(child, ancestors, nodeOnly, filePer, currentPackageRef, targetPackageRef, targetKey, fileNaming)
			if err != nil {
				return nil, err
			}
			if cl != nil {
				// If we get a valid layout from a child but this node doesn't belong to the package,
				// we need to create a minimal parent structure
				if ml == nil {
					ml = &ManifestLayout{
						Name:       "",
						Namespace:  filepath.Join(ancestors...),
						FilePer:    filePer,
						FileNaming: fileNaming,
						Children:   []*ManifestLayout{cl},
					}
				} else {
					ml.Children = append(ml.Children, cl)
				}
			}
		}
	}

	return ml, nil
}
