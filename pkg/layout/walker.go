package layout

import (
	"path/filepath"

	"github.com/go-kure/kure/pkg/stack"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WalkCluster traverses a stack.Cluster and builds a ManifestLayout tree that
// mirrors the node and bundle hierarchy. Applications are generated and their
// resources are assigned to the corresponding directories based on the
// parent-child relationships of nodes and bundle names.
func WalkCluster(c *stack.Cluster) (*ManifestLayout, error) {
	if c == nil || c.Node == nil {
		return nil, nil
	}
	return walkNode(c.Node, nil)
}

// walkNode recursively processes a stack.Node and its children.
func walkNode(n *stack.Node, ancestors []string) (*ManifestLayout, error) {
	if n == nil {
		return nil, nil
	}

	currentPath := append([]string{}, ancestors...)
	if n.Name != "" {
		currentPath = append(currentPath, n.Name)
	}

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
		cl, err := walkNode(child, currentPath)
		if err != nil {
			return nil, err
		}
		if cl != nil {
			children = append(children, cl)
		}
	}

	return &ManifestLayout{
		Name:      n.Name,
		Namespace: filepath.Join(ancestors...),
		Children:  children,
	}, nil
}
