package fluxcd

import (
	"path/filepath"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	meta "github.com/fluxcd/pkg/apis/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdpkg "github.com/go-kure/kure/pkg/k8s/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
	"github.com/go-kure/kure/pkg/stack"
)

// Workflow implements the stack.Workflow interface for Flux.
type Workflow struct {
	// Mode controls how spec.path is generated.
	Mode layout.KustomizationMode
}

// NewWorkflow returns a Workflow initialized with defaults.
func NewWorkflow() Workflow {
	return Workflow{
		Mode: layout.KustomizationExplicit,
	}
}

// Cluster converts the cluster definition into Flux Kustomizations.
func (w Workflow) Cluster(c *stack.Cluster) ([]client.Object, error) {
	if c == nil || c.Node == nil {
		return nil, nil
	}
	return w.Node(c.Node)
}

// ClusterByPackage converts the cluster definition into Flux Kustomizations grouped by PackageRef.
// Returns a map where keys are package reference strings and values are the Flux objects for that package.
func (w Workflow) ClusterByPackage(c *stack.Cluster) (map[string][]client.Object, error) {
	if c == nil || c.Node == nil {
		return nil, nil
	}
	return w.NodeByPackage(c.Node, nil)
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

// NodeByPackage converts a Node and its children into Kustomizations grouped by PackageRef.
func (w Workflow) NodeByPackage(n *stack.Node, inheritedPackageRef *schema.GroupVersionKind) (map[string][]client.Object, error) {
	if n == nil {
		return nil, nil
	}
	
	result := make(map[string][]client.Object)
	currentPackageRef := resolveNodePackageRef(n, inheritedPackageRef)
	
	if n.Bundle != nil {
		bObjs, err := w.BundleWithPackageRef(n.Bundle, currentPackageRef)
		if err != nil {
			return nil, err
		}
		if len(bObjs) > 0 {
			key := packageRefToKey(currentPackageRef)
			result[key] = append(result[key], bObjs...)
		}
	}
	
	for _, child := range n.Children {
		childObjs, err := w.NodeByPackage(child, currentPackageRef)
		if err != nil {
			return nil, err
		}
		for key, objs := range childObjs {
			result[key] = append(result[key], objs...)
		}
	}
	
	return result, nil
}

// Bundle converts a Bundle into a Flux Kustomization.
func (w Workflow) Bundle(b *stack.Bundle) ([]client.Object, error) {
	if b == nil {
		return nil, nil
	}
	path := bundlePath(b)
	if w.Mode == layout.KustomizationRecursive && b.Parent != nil {
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

// BundleWithPackageRef converts a Bundle into a Flux Kustomization using the provided PackageRef for source reference.
func (w Workflow) BundleWithPackageRef(b *stack.Bundle, packageRef *schema.GroupVersionKind) ([]client.Object, error) {
	if b == nil {
		return nil, nil
	}
	path := bundlePath(b)
	if w.Mode == layout.KustomizationRecursive && b.Parent != nil {
		path = bundlePath(b.Parent)
	}
	interval := b.Interval
	if interval == "" {
		interval = "10m"
	}
	
	// Use PackageRef to determine source reference
	sourceRef := sourceRefFromPackageRef(packageRef)
	
	// Override with Bundle's SourceRef if provided
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

// resolveNodePackageRef returns the effective PackageRef for a node, using inheritance from parent
func resolveNodePackageRef(n *stack.Node, inheritedPackageRef *schema.GroupVersionKind) *schema.GroupVersionKind {
	if n.PackageRef != nil {
		return n.PackageRef
	}
	return inheritedPackageRef
}

// packageRefToKey converts a PackageRef to a string key for map indexing
func packageRefToKey(ref *schema.GroupVersionKind) string {
	if ref == nil {
		return "default"
	}
	return ref.String()
}

// sourceRefFromPackageRef creates a CrossNamespaceSourceReference from a PackageRef
func sourceRefFromPackageRef(packageRef *schema.GroupVersionKind) kustv1.CrossNamespaceSourceReference {
	if packageRef == nil {
		// Default to OCIRepository for backward compatibility
		return kustv1.CrossNamespaceSourceReference{
			Kind:      "OCIRepository",
			Name:      "flux-system",
			Namespace: "flux-system",
		}
	}
	
	// Use the PackageRef's Kind and generate default name/namespace
	return kustv1.CrossNamespaceSourceReference{
		Kind:      packageRef.Kind,
		Name:      "flux-system", // Could be enhanced to derive from PackageRef
		Namespace: "flux-system",
	}
}
