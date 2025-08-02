package layout

import (
	"path/filepath"

	"github.com/go-kure/kure/pkg/stack"
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

	return walkNode(c.Node, nil, nodeOnly, filePer)
}

// walkNode recursively processes a stack.Node and its children.
func walkNode(n *stack.Node, ancestors []string, nodeOnly bool, filePer FileExportMode) (*ManifestLayout, error) {
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
			cl, err := walkNode(child, currentPath, nodeOnly, filePer)
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
			cl, err := walkNode(child, currentPath, nodeOnly, filePer)
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
