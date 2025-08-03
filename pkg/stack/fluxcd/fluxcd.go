package fluxcd

import (
	"path/filepath"
	"time"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	metaapi "github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	intfluxcd "github.com/go-kure/kure/internal/fluxcd"
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

// ClusterWithLayout converts the cluster definition into a ManifestLayout with integrated Flux Kustomizations.
// The behavior depends on the FluxPlacement setting in rules:
// - FluxSeparate: Creates traditional separate Flux directory (for backward compatibility)
// - FluxIntegrated: Places Flux Kustomizations alongside their target manifests
func (w Workflow) ClusterWithLayout(c *stack.Cluster, rules layout.LayoutRules) (*layout.ManifestLayout, error) {
	if c == nil || c.Node == nil {
		return nil, nil
	}

	// Generate the base manifest layout
	ml, err := layout.WalkCluster(c, rules)
	if err != nil {
		return nil, err
	}

	// Handle Flux placement based on rules
	switch rules.FluxPlacement {
	case layout.FluxIntegrated:
		// Add Flux Kustomizations to each node in the layout
		err = w.addIntegratedFluxToLayout(ml, c, rules)
		if err != nil {
			return nil, err
		}
	case layout.FluxSeparate:
		// Traditional behavior - return manifests only, Flux handled separately
		// This maintains backward compatibility
	default:
		// Default to separate for backward compatibility
	}

	return ml, nil
}

// addIntegratedFluxToLayout adds Flux Kustomizations in an integrated manner:
// - Node-level Kustomizations are placed in the parent node
// - App-level Kustomizations are placed alongside their manifests
func (w Workflow) addIntegratedFluxToLayout(ml *layout.ManifestLayout, c *stack.Cluster, rules layout.LayoutRules) error {
	if ml == nil || c == nil || c.Node == nil {
		return nil
	}

	// Find the flux-system node in the layout (it manages other nodes)
	var fluxSystemLayout *layout.ManifestLayout
	for _, child := range ml.Children {
		if child.Name == c.Node.Name { // Root node name from cluster
			fluxSystemLayout = child
			break
		}
	}

	if fluxSystemLayout != nil {
		// Add node-management Kustomizations to flux-system
		for _, child := range c.Node.Children {
			nodeKust, err := w.createNodeManagementKustomization(child, rules.ClusterName)
			if err != nil {
				return err
			}
			if nodeKust != nil {
				fluxSystemLayout.Resources = append(fluxSystemLayout.Resources, nodeKust)
			}
		}
	}

	// Add app-level Kustomizations to their respective nodes
	for _, child := range c.Node.Children {
		err := w.addAppLevelFluxToNode(ml, child, rules.ClusterName)
		if err != nil {
			return err
		}
	}

	return nil
}

// createNodeManagementKustomization creates a Flux Kustomization that manages an entire node
func (w Workflow) createNodeManagementKustomization(node *stack.Node, clusterName string) (client.Object, error) {
	if node == nil || node.Bundle == nil {
		return nil, nil
	}

	// Use the Bundle method to create the Kustomization for this node
	objs, err := w.Bundle(node.Bundle)
	if err != nil {
		return nil, err
	}

	// Return the first Kustomization (should be exactly one)
	for _, obj := range objs {
		if kust, ok := obj.(*kustv1.Kustomization); ok {
			// Update path to point to sibling node
			if clusterName != "" {
				kust.Spec.Path = filepath.Join(clusterName, node.Name)
			} else {
				kust.Spec.Path = node.Name
			}
			return kust, nil
		}
	}

	return nil, nil
}

// addAppLevelFluxToNode adds application-level Flux Kustomizations to a node's layout
func (w Workflow) addAppLevelFluxToNode(ml *layout.ManifestLayout, node *stack.Node, clusterName string) error {
	// Find the target node layout
	var targetLayout *layout.ManifestLayout
	for _, child := range ml.Children {
		if child.Name == node.Name {
			targetLayout = child
			break
		}
	}

	if targetLayout == nil {
		return nil // Node not found in layout
	}

	// Add Kustomizations for each application/service in this node
	if node.Bundle != nil {
		for _, app := range node.Bundle.Applications {
			if app != nil {
				appKust, err := w.createApplicationKustomization(app, node, clusterName)
				if err != nil {
					return err
				}
				if appKust != nil {
					targetLayout.Resources = append(targetLayout.Resources, appKust)
				}
			}
		}
	}

	// Recursively add Flux Kustomizations for child bundles (like logging, metrics, networking)
	for _, child := range node.Children {
		if child.Bundle != nil {
			for _, app := range child.Bundle.Applications {
				if app != nil {
					serviceKust, err := w.createServiceKustomization(app, child, node, clusterName)
					if err != nil {
						return err
					}
					if serviceKust != nil {
						targetLayout.Resources = append(targetLayout.Resources, serviceKust)
					}
				}
			}
		}
	}

	return nil
}

// createApplicationKustomization creates a Flux Kustomization for a specific application
func (w Workflow) createApplicationKustomization(app *stack.Application, node *stack.Node, clusterName string) (client.Object, error) {
	if app == nil {
		return nil, nil
	}

	// Create a Kustomization for this specific application
	spec := kustv1.KustomizationSpec{
		Interval: metav1.Duration{Duration: 10 * time.Minute},
		Prune:    true,
		SourceRef: kustv1.CrossNamespaceSourceReference{
			Kind:      "OCIRepository",
			Name:      node.Name, // Use node name as source
			Namespace: "flux-system",
		},
	}
	
	// Set the path to point to the application directory within the node
	if clusterName != "" {
		spec.Path = filepath.Join(clusterName, node.Name, app.Name)
	} else {
		spec.Path = filepath.Join(node.Name, app.Name)
	}
	
	kust := intfluxcd.CreateKustomization(app.Name, "flux-system", spec)

	return kust, nil
}

// createServiceKustomization creates a Flux Kustomization for a service (like logging, metrics)
func (w Workflow) createServiceKustomization(app *stack.Application, serviceNode *stack.Node, parentNode *stack.Node, clusterName string) (client.Object, error) {
	if app == nil || serviceNode == nil {
		return nil, nil
	}

	// Create a Kustomization for this specific service
	spec := kustv1.KustomizationSpec{
		Interval: metav1.Duration{Duration: 10 * time.Minute},
		Prune:    true,
		SourceRef: kustv1.CrossNamespaceSourceReference{
			Kind:      "OCIRepository",
			Name:      parentNode.Name, // Use parent node name as source
			Namespace: "flux-system",
		},
	}
	
	// Set the path to point to the service directory within the parent node
	if clusterName != "" {
		spec.Path = filepath.Join(clusterName, parentNode.Name, serviceNode.Name)
	} else {
		spec.Path = filepath.Join(parentNode.Name, serviceNode.Name)
	}
	
	kust := intfluxcd.CreateKustomization(serviceNode.Name, "flux-system", spec)

	return kust, nil
}

// addFluxToLayout recursively adds Flux Kustomizations to each node in the ManifestLayout.
func (w Workflow) addFluxToLayout(ml *layout.ManifestLayout, node *stack.Node, ancestors []string) error {
	if ml == nil || node == nil {
		return nil
	}

	// Generate Flux Kustomization for this node if it has a bundle
	if node.Bundle != nil {
		fluxObjs, err := w.Bundle(node.Bundle)
		if err != nil {
			return err
		}
		
		// Update Kustomization paths to be relative to the current node
		for _, obj := range fluxObjs {
			if kust, ok := obj.(*kustv1.Kustomization); ok {
				// Update the path to point to the current node location
				currentPath := append(ancestors, node.Name)
				kust.Spec.Path = filepath.Join(currentPath...)
			}
		}
		
		// Add Flux objects to this layout node
		ml.Resources = append(ml.Resources, fluxObjs...)
	}

	// Recursively process children
	currentPath := append(ancestors, node.Name)
	for i, child := range node.Children {
		if i < len(ml.Children) {
			err := w.addFluxToLayout(ml.Children[i], child, currentPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
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
	parsedInterval, err := time.ParseDuration(interval)
	if err != nil {
		parsedInterval = 10 * time.Minute // Default fallback
	}
	spec := kustv1.KustomizationSpec{
		Path:      path,
		Interval:  metav1.Duration{Duration: parsedInterval},
		Prune:     true,
		SourceRef: sourceRef,
	}
	k := intfluxcd.CreateKustomization(b.Name, "flux-system", spec)
	for _, dep := range b.DependsOn {
		intfluxcd.AddKustomizationDependsOn(k, metaapi.NamespacedObjectReference{Name: dep.Name})
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
	
	parsedInterval, err := time.ParseDuration(interval)
	if err != nil {
		parsedInterval = 10 * time.Minute // Default fallback
	}
	spec := kustv1.KustomizationSpec{
		Path:      path,
		Interval:  metav1.Duration{Duration: parsedInterval},
		Prune:     true,
		SourceRef: sourceRef,
	}
	k := intfluxcd.CreateKustomization(b.Name, "flux-system", spec)
	for _, dep := range b.DependsOn {
		intfluxcd.AddKustomizationDependsOn(k, metaapi.NamespacedObjectReference{Name: dep.Name})
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
