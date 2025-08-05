package argocd

import (
	"fmt"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Ensure WorkflowEngine implements the stack.Workflow interface
var _ stack.Workflow = (*WorkflowEngine)(nil)

func init() {
	// Register the ArgoCD workflow factory with the stack package
	stack.RegisterArgoWorkflow(func() stack.Workflow {
		return Engine()
	})
}

// WorkflowEngine implements the stack.Workflow interface for ArgoCD.
type WorkflowEngine struct {
	// RepoURL is used as the source repo for generated Applications
	RepoURL string
	// DefaultNamespace is the default namespace for ArgoCD Applications
	DefaultNamespace string
}

// Engine creates an ArgoCD workflow engine.
func Engine() *WorkflowEngine {
	return &WorkflowEngine{
		RepoURL:          "https://github.com/example/manifests.git",
		DefaultNamespace: "argocd",
	}
}

// ResourceGenerator interface implementation

// GenerateFromCluster creates ArgoCD Applications from a cluster definition.
func (w *WorkflowEngine) GenerateFromCluster(c *stack.Cluster) ([]client.Object, error) {
	if c == nil || c.Node == nil {
		return nil, nil
	}
	return w.GenerateFromNode(c.Node)
}

// GenerateFromNode creates ArgoCD Applications from a node and its children.
func (w *WorkflowEngine) GenerateFromNode(n *stack.Node) ([]client.Object, error) {
	if n == nil {
		return nil, nil
	}
	
	var objs []client.Object
	
	// Generate application for this node's bundle
	if n.Bundle != nil {
		bundleApps, err := w.GenerateFromBundle(n.Bundle)
		if err != nil {
			return nil, err
		}
		objs = append(objs, bundleApps...)
	}
	
	// Generate applications for child nodes
	for _, child := range n.Children {
		childApps, err := w.GenerateFromNode(child)
		if err != nil {
			return nil, err
		}
		objs = append(objs, childApps...)
	}
	
	return objs, nil
}

// GenerateFromBundle creates an ArgoCD Application from a bundle definition.
func (w *WorkflowEngine) GenerateFromBundle(b *stack.Bundle) ([]client.Object, error) {
	if b == nil {
		return nil, nil
	}
	
	app := &unstructured.Unstructured{}
	app.SetAPIVersion("argoproj.io/v1alpha1")
	app.SetKind("Application")
	app.SetName(b.Name)
	app.SetNamespace(w.DefaultNamespace)
	
	// Set labels if provided
	if len(b.Labels) > 0 {
		app.SetLabels(b.Labels)
	}
	
	// Configure source
	source := map[string]interface{}{
		"repoURL": w.RepoURL,
		"path":    w.bundlePath(b),
	}
	
	// Configure destination
	dest := map[string]interface{}{
		"server":    "https://kubernetes.default.svc",
		"namespace": "default",
	}
	
	// Set spec fields
	_ = unstructured.SetNestedField(app.Object, source, "spec", "source")
	_ = unstructured.SetNestedField(app.Object, dest, "spec", "destination")
	
	// Add dependencies if present
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

// LayoutIntegrator interface implementation

// IntegrateWithLayout adds ArgoCD Applications to an existing manifest layout.
// For ArgoCD, this is typically not needed as Applications reference external repos.
func (w *WorkflowEngine) IntegrateWithLayout(ml *layout.ManifestLayout, c *stack.Cluster, rules layout.LayoutRules) error {
	// ArgoCD Applications typically don't need layout integration
	// as they reference external repositories
	return nil
}

// CreateLayoutWithResources creates a new layout that includes ArgoCD Applications.
func (w *WorkflowEngine) CreateLayoutWithResources(c *stack.Cluster, rulesInterface interface{}) (interface{}, error) {
	rules, ok := rulesInterface.(layout.LayoutRules)
	if !ok {
		return nil, fmt.Errorf("rules must be of type layout.LayoutRules")
	}
	// Generate the base manifest layout
	ml, err := layout.WalkCluster(c, rules)
	if err != nil {
		return nil, err
	}
	
	// For ArgoCD, we typically create a separate argocd directory for Applications
	apps, err := w.GenerateFromCluster(c)
	if err != nil {
		return nil, err
	}
	
	if len(apps) > 0 {
		argoCDLayout := &layout.ManifestLayout{
			Name:      "argocd",
			Namespace: filepath.Join(ml.Namespace, "argocd"),
			FilePer:   layout.FilePerResource,
			Resources: apps,
		}
		ml.Children = append(ml.Children, argoCDLayout)
	}
	
	return ml, nil
}

// BootstrapGenerator interface implementation

// GenerateBootstrap creates bootstrap resources for setting up ArgoCD.
func (w *WorkflowEngine) GenerateBootstrap(config *stack.BootstrapConfig, rootNode *stack.Node) ([]client.Object, error) {
	if config == nil || !config.Enabled {
		return nil, nil
	}
	
	// Mock implementation - returns empty for now
	// TODO: Implement ArgoCD bootstrap with:
	// - ArgoCD namespace
	// - ArgoCD CRDs and deployment manifests
	// - App-of-apps pattern setup
	// - Root application pointing to the cluster manifests
	return []client.Object{}, nil
}

// SupportedBootstrapModes returns the bootstrap modes supported by ArgoCD.
func (w *WorkflowEngine) SupportedBootstrapModes() []string {
	return []string{"argocd", "app-of-apps"}
}

// WorkflowEngine interface implementation

// GetName returns a human-readable name for this workflow engine.
func (w *WorkflowEngine) GetName() string {
	return "ArgoCD Workflow Engine"
}

// GetVersion returns the version of this workflow engine.
func (w *WorkflowEngine) GetVersion() string {
	return "v1.0.0"
}

// Configuration methods

// SetRepoURL configures the repository URL for generated Applications.
func (w *WorkflowEngine) SetRepoURL(repoURL string) {
	w.RepoURL = repoURL
}

// SetDefaultNamespace configures the default namespace for ArgoCD Applications.
func (w *WorkflowEngine) SetDefaultNamespace(namespace string) {
	w.DefaultNamespace = namespace
}

// bundlePath builds a repository path for the bundle based on its ancestry.
func (w *WorkflowEngine) bundlePath(b *stack.Bundle) string {
	var parts []string
	for p := b; p != nil; p = p.GetParent() {
		if p.Name != "" {
			parts = append([]string{p.Name}, parts...)
		}
	}
	return filepath.ToSlash(filepath.Join(parts...))
}