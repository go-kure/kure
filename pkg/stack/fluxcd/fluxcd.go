package fluxcd

import (
	"path/filepath"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	meta "github.com/fluxcd/pkg/apis/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdpkg "github.com/go-kure/kure/pkg/fluxcd"
	layoutpkg "github.com/go-kure/kure/pkg/layout"
	"github.com/go-kure/kure/pkg/stack"
)

// Workflow implements the stack.Workflow interface for Flux.
type Workflow struct {
	// Mode controls how spec.path is generated.
	Mode layoutpkg.KustomizationMode
}

// NewWorkflow returns a Workflow initialized with defaults.
func NewWorkflow() Workflow {
	return Workflow{
		Mode: layoutpkg.KustomizationExplicit,
	}
}

// Cluster converts the cluster definition into Flux Kustomizations.
func (w Workflow) Cluster(c *stack.Cluster) ([]client.Object, error) {
	if c == nil || c.Node == nil {
		return nil, nil
	}
	return w.Node(c.Node)
}

// Node converts a Node and its children into Kustomizations.
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

// Bundle converts a Bundle into a Flux Kustomization.
func (w Workflow) Bundle(b *stack.Bundle) ([]client.Object, error) {
	if b == nil {
		return nil, nil
	}
	path := bundlePath(b)
	if w.Mode == layoutpkg.KustomizationRecursive && b.Parent != nil {
		path = bundlePath(b.Parent)
	}
	interval := b.Interval
	if interval == "" {
		interval = "10m"
	}
	sourceRef := kustv1.CrossNamespaceSourceReference{
		Kind:      "OCIRepository",
		Name:      "flux-system",
		Namespace: "flux-system",
	}
	if b.SourceRef != nil {
		if b.SourceRef.Kind != "" {
			sourceRef.Kind = b.SourceRef.Kind
		}
		if b.SourceRef.Name != "" {
			sourceRef.Name = b.SourceRef.Name
		}
		if b.SourceRef.Namespace != "" {
			sourceRef.Namespace = b.SourceRef.Namespace
		}
	}
	cfg := fluxcdpkg.KustomizationConfig{
		Name:      b.Name,
		Namespace: "flux-system",
		Path:      path,
		Interval:  interval,
		Prune:     true,
		SourceRef: sourceRef,
	}
	k := fluxcdpkg.NewKustomization(&cfg)
	for _, dep := range b.DependsOn {
		k.Spec.DependsOn = append(k.Spec.DependsOn, meta.NamespacedObjectReference{Name: dep.Name})
	}
	var obj client.Object = k
	return []client.Object{obj}, nil
}

// bundlePath builds a repository path for the bundle based on its ancestry.
func bundlePath(b *stack.Bundle) string {
	var parts []string
	for p := b; p != nil; p = p.Parent {
		if p.Name != "" {
			parts = append([]string{p.Name}, parts...)
		}
	}
	return filepath.ToSlash(filepath.Join(parts...))
}
