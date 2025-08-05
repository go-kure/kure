package fluxcd

import (
	"fmt"
	"path/filepath"
	"time"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	metaapi "github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// ResourceGenerator implements the workflow.ResourceGenerator interface for Flux.
// It focuses purely on generating Flux CRDs from stack components.
type ResourceGenerator struct {
	// Mode controls how spec.path is generated in Kustomizations
	Mode layout.KustomizationMode
	// DefaultInterval is the default reconciliation interval for generated resources
	DefaultInterval time.Duration
	// DefaultNamespace is the default namespace for generated Flux resources
	DefaultNamespace string
}

// NewResourceGenerator creates a FluxCD resource generator with sensible defaults.
func NewResourceGenerator() *ResourceGenerator {
	return &ResourceGenerator{
		Mode:             layout.KustomizationExplicit,
		DefaultInterval:  5 * time.Minute,
		DefaultNamespace: "flux-system",
	}
}

// GenerateFromCluster creates Flux Kustomizations and Sources from a cluster definition.
func (g *ResourceGenerator) GenerateFromCluster(c *stack.Cluster) ([]client.Object, error) {
	if c == nil || c.Node == nil {
		return nil, nil
	}
	return g.GenerateFromNode(c.Node)
}

// GenerateFromNode creates Flux resources from a node and its children.
func (g *ResourceGenerator) GenerateFromNode(n *stack.Node) ([]client.Object, error) {
	if n == nil {
		return nil, nil
	}
	
	var resources []client.Object
	
	// Generate resources for this node's bundle
	if n.Bundle != nil {
		bundleResources, err := g.GenerateFromBundle(n.Bundle)
		if err != nil {
			return nil, errors.ResourceValidationError("Node", n.Name, "bundle", 
				fmt.Sprintf("failed to generate bundle resources: %v", err), err)
		}
		resources = append(resources, bundleResources...)
	}
	
	// Generate resources for child nodes
	for _, child := range n.Children {
		childResources, err := g.GenerateFromNode(child)
		if err != nil {
			return nil, errors.ResourceValidationError("Node", n.Name, "children",
				fmt.Sprintf("failed to generate child node resources: %v", err), err)
		}
		resources = append(resources, childResources...)
	}
	
	return resources, nil
}

// GenerateFromBundle creates a Flux Kustomization from a bundle definition.
func (g *ResourceGenerator) GenerateFromBundle(b *stack.Bundle) ([]client.Object, error) {
	if b == nil {
		return nil, nil
	}
	
	// Create the main Kustomization for this bundle
	kustomization := g.createKustomization(b)
	resources := []client.Object{kustomization}
	
	// Create source if specified
	if b.SourceRef != nil {
		source, err := g.createSource(b.SourceRef, b.Name)
		if err != nil {
			return nil, errors.ResourceValidationError("Bundle", b.Name, "source",
				fmt.Sprintf("failed to create source: %v", err), err)
		}
		if source != nil {
			resources = append(resources, source)
		}
	}
	
	return resources, nil
}

// createKustomization creates a Flux Kustomization resource from a bundle.
func (g *ResourceGenerator) createKustomization(b *stack.Bundle) client.Object {
	interval := g.DefaultInterval
	if b.Interval != "" {
		if d, err := time.ParseDuration(b.Interval); err == nil {
			interval = d
		}
	}
	
	kust := &kustv1.Kustomization{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kustv1.GroupVersion.String(),
			Kind:       "Kustomization",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.Name,
			Namespace: g.DefaultNamespace,
			Labels:    b.Labels,
		},
		Spec: kustv1.KustomizationSpec{
			Interval: metav1.Duration{Duration: interval},
			Path:     g.generatePath(b),
			Prune:    true,
		},
	}
	
	// Set source reference
	if b.SourceRef != nil {
		kust.Spec.SourceRef = kustv1.CrossNamespaceSourceReference{
			Kind: b.SourceRef.Kind,
			Name: b.SourceRef.Name,
		}
		if b.SourceRef.Namespace != "" {
			kust.Spec.SourceRef.Namespace = b.SourceRef.Namespace
		}
	}
	
	// Add dependencies
	for _, dep := range b.DependsOn {
		kust.Spec.DependsOn = append(kust.Spec.DependsOn, metaapi.NamespacedObjectReference{
			Name: dep.Name,
		})
	}
	
	return kust
}

// createSource creates a Flux source resource based on the source reference.
func (g *ResourceGenerator) createSource(ref *stack.SourceRef, name string) (client.Object, error) {
	switch ref.Kind {
	case "GitRepository":
		// For now, return nil - GitRepository creation would need additional parameters
		// This could be extended to create actual GitRepository resources
		return nil, nil
	case "OCIRepository":
		// For now, return nil - OCIRepository creation would need additional parameters
		return nil, nil
	default:
		return nil, errors.NewValidationError("kind", ref.Kind, "SourceRef", 
			[]string{"GitRepository", "OCIRepository"})
	}
}

// generatePath generates the path for a Kustomization based on the bundle hierarchy.
// This replicates the logic from the original bundlePath function to maintain compatibility.
func (g *ResourceGenerator) generatePath(b *stack.Bundle) string {
	path := g.bundlePath(b)
	if g.Mode == layout.KustomizationRecursive && b.GetParent() != nil {
		path = g.bundlePath(b.GetParent())
	}
	return path
}

// bundlePath builds a repository path for the bundle based on its ancestry.
// This is copied from the original implementation to maintain compatibility.
func (g *ResourceGenerator) bundlePath(b *stack.Bundle) string {
	var parts []string
	for p := b; p != nil; p = p.GetParent() {
		if p.Name != "" {
			parts = append([]string{p.Name}, parts...)
		}
	}
	return filepath.ToSlash(filepath.Join(parts...))
}

// GetName returns the name of this resource generator.
func (g *ResourceGenerator) GetName() string {
	return "FluxCD Resource Generator"
}

// GetVersion returns the version of this resource generator.
func (g *ResourceGenerator) GetVersion() string {
	return "v1.0.0"
}