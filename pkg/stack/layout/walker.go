package layout

import (
	"path/filepath"

	"github.com/go-kure/kure/pkg/stack"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	return walkNode(c.Node, nil, nodeOnly, filePer, nil)
}

// WalkClusterByPackage traverses a stack.Cluster and builds separate ManifestLayout trees
// for each unique PackageRef (OCI artifact). Returns a map where keys are PackageRef GVKs
// and values are the corresponding ManifestLayout trees. Nodes without PackageRef inherit
// from their parent, with nil representing the default package.
func WalkClusterByPackage(c *stack.Cluster, rules LayoutRules) (map[string]*ManifestLayout, error) {
	if c == nil || c.Node == nil {
		return nil, nil
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
		layout, err := walkNodeForPackage(c.Node, nil, nodeOnly, filePer, pkgRef, pkgKey)
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
func walkNode(n *stack.Node, ancestors []string, nodeOnly bool, filePer FileExportMode, inheritedPackageRef *schema.GroupVersionKind) (*ManifestLayout, error) {
	if n == nil {
		return nil, nil
	}

	currentPath := append([]string{}, ancestors...)
	if n.Name != "" {
		currentPath = append(currentPath, n.Name)
	}

	ml := &ManifestLayout{
		Name:      n.Name,
		Namespace: filepath.Join(ancestors...),
		FilePer:   filePer,
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
					Name:      app.Name,
					Namespace: filepath.Join(append(currentPath, b.Name)...),
					Resources: objs,
				}
				bundleChildren = append(bundleChildren, appLayout)
			}
			bundleLayout := &ManifestLayout{
				Name:      b.Name,
				Namespace: filepath.Join(currentPath...),
				Children:  bundleChildren,
			}
			children = append(children, bundleLayout)
		}

		for _, child := range n.Children {
			cl, err := walkNode(child, currentPath, nodeOnly, filePer, resolvePackageRef(n, inheritedPackageRef))
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
			cl, err := walkNode(child, currentPath, nodeOnly, filePer, resolvePackageRef(n, inheritedPackageRef))
			if err != nil {
				return nil, err
			}
			if cl != nil {
				ml.Children = append(ml.Children, cl)
			}
		}
	}

	return ml, nil
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
func walkNodeForPackage(n *stack.Node, ancestors []string, nodeOnly bool, filePer FileExportMode, targetPackageRef *schema.GroupVersionKind, targetKey string) (*ManifestLayout, error) {
	return walkNodeForPackageInternal(n, ancestors, nodeOnly, filePer, nil, targetPackageRef, targetKey)
}

// walkNodeForPackageInternal is the internal implementation with inheritance tracking
func walkNodeForPackageInternal(n *stack.Node, ancestors []string, nodeOnly bool, filePer FileExportMode, inheritedPackageRef *schema.GroupVersionKind, targetPackageRef *schema.GroupVersionKind, targetKey string) (*ManifestLayout, error) {
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
			Name:      n.Name,
			Namespace: filepath.Join(ancestors...),
			FilePer:   filePer,
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
						Name:      app.Name,
						Namespace: filepath.Join(append(currentPath, b.Name)...),
						Resources: objs,
					}
					bundleChildren = append(bundleChildren, appLayout)
				}
				if len(bundleChildren) > 0 {
					bundleLayout := &ManifestLayout{
						Name:      b.Name,
						Namespace: filepath.Join(currentPath...),
						Children:  bundleChildren,
					}
					children = append(children, bundleLayout)
				}
			}

			for _, child := range n.Children {
				cl, err := walkNodeForPackageInternal(child, currentPath, nodeOnly, filePer, currentPackageRef, targetPackageRef, targetKey)
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
				cl, err := walkNodeForPackageInternal(child, currentPath, nodeOnly, filePer, currentPackageRef, targetPackageRef, targetKey)
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
			cl, err := walkNodeForPackageInternal(child, ancestors, nodeOnly, filePer, currentPackageRef, targetPackageRef, targetKey)
			if err != nil {
				return nil, err
			}
			if cl != nil {
				// If we get a valid layout from a child but this node doesn't belong to the package,
				// we need to create a minimal parent structure
				if ml == nil {
					ml = &ManifestLayout{
						Name:      "",
						Namespace: filepath.Join(ancestors...),
						FilePer:   filePer,
						Children:  []*ManifestLayout{cl},
					}
				} else {
					ml.Children = append(ml.Children, cl)
				}
			}
		}
	}

	return ml, nil
}
