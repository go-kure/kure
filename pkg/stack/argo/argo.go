package argo

import (
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
)

// Workflow implements the stack.Workflow interface for Argo CD.
type Workflow struct {
	// RepoURL is used as the source repo for generated Applications.
	RepoURL string
}

// Cluster converts a Cluster into Argo CD Applications.
func (w Workflow) Cluster(c *stack.Cluster) ([]client.Object, error) {
	if c == nil || c.Node == nil {
		return nil, nil
	}
	return w.Node(c.Node)
}

// Node converts a Node and its children into Applications.
func (w Workflow) Node(n *stack.Node) ([]client.Object, error) {
	if n == nil {
		return nil, nil
	}
	var objs []client.Object
	if n.Bundle != nil {
		bObjs, err := w.Bundle(n.Bundle)
		if err != nil {
			return nil, err
		}
		objs = append(objs, bObjs...)
	}
	for _, child := range n.Children {
		cObjs, err := w.Node(child)
		if err != nil {
			return nil, err
		}
		objs = append(objs, cObjs...)
	}
	return objs, nil
}

// Bundle converts a Bundle into an Argo CD Application.
func (w Workflow) Bundle(b *stack.Bundle) ([]client.Object, error) {
	if b == nil {
		return nil, nil
	}
	app := &unstructured.Unstructured{}
	app.SetAPIVersion("argoproj.io/v1alpha1")
	app.SetKind("Application")
	app.SetName(b.Name)
	app.SetNamespace("argocd")

	source := map[string]interface{}{"repoURL": w.RepoURL, "path": bundlePath(b)}
	dest := map[string]interface{}{"server": "https://kubernetes.default.svc", "namespace": "default"}
	_ = unstructured.SetNestedField(app.Object, source, "spec", "source")
	_ = unstructured.SetNestedField(app.Object, dest, "spec", "destination")
	if len(b.DependsOn) > 0 {
		var deps []string
		for _, d := range b.DependsOn {
			deps = append(deps, d.Name)
		}
		_ = unstructured.SetNestedStringSlice(app.Object, deps, "spec", "dependencies")
	}

	var obj client.Object = app
	return []client.Object{obj}, nil
}

func bundlePath(b *stack.Bundle) string {
	var parts []string
	for p := b; p != nil; p = p.Parent {
		if p.Name != "" {
			parts = append([]string{p.Name}, parts...)
		}
	}
	return filepath.ToSlash(filepath.Join(parts...))
}
