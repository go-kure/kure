package argocd

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
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

// GenerateFromCluster creates ArgoCD Applications from a cluster definition:
// it walks the cluster with layout.DefaultLayoutRules and generates from that
// layout (see generateFromLayout), so each source.path is the directory a
// default-rules walk writes the bundle to. Callers writing the layout with
// other rules use CreateLayoutWithResources, which generates from the layout
// it walks.
func (w *WorkflowEngine) GenerateFromCluster(c *stack.Cluster) ([]client.Object, error) {
	if c == nil || c.Node == nil {
		return nil, nil
	}
	ml, err := layout.WalkCluster(c, layout.DefaultLayoutRules())
	if err != nil {
		return nil, err
	}
	return w.generateFromLayout(ml, c)
}

// generateFromLayout creates one Application for every reconciliation unit of
// the layout tree root — every directory that renders bundles, umbrella
// children included — in layout pre-order. root must have been walked from c:
// layout.IndexOrigins refuses anything else. Each source.path is that
// directory; the bundles a grouping axis merged into it share the
// Application, named after the first of them, with their labels combined and
// spec.dependencies mapped to units (OriginIndex.UnitDependencies).
func (w *WorkflowEngine) generateFromLayout(root *layout.ManifestLayout, c *stack.Cluster) ([]client.Object, error) {
	if root == nil || c == nil || c.Node == nil {
		return nil, nil
	}
	ix, err := layout.IndexOrigins(root, c)
	if err != nil {
		return nil, err
	}
	var objs []client.Object
	for _, l := range ix.Units() {
		bundles := l.OriginBundles()
		labels := map[string]string{}
		for _, b := range bundles {
			for k, v := range b.Labels {
				if prev, ok := labels[k]; ok && prev != v {
					return nil, errors.Errorf("bundles merged into %q share one Application but set label %q to %q and %q", l.FullRepoPath(), k, prev, v)
				}
				labels[k] = v
			}
		}
		unit := *bundles[0]
		unit.Labels = labels
		unit.DependsOn = nil
		app, err := w.applicationForBundle(&unit, l.FullRepoPath())
		if err != nil {
			return nil, err
		}
		if deps := ix.UnitDependencies(l); len(deps) > 0 {
			if err := unstructured.SetNestedStringSlice(app.(*unstructured.Unstructured).Object, deps, "spec", "dependencies"); err != nil {
				return nil, errors.Wrap(err, "failed to set spec.dependencies")
			}
		}
		objs = append(objs, app)
	}
	return objs, nil
}

// applicationForBundle creates an ArgoCD Application for b whose
// spec.source.path is path, verbatim.
func (w *WorkflowEngine) applicationForBundle(b *stack.Bundle, path string) (client.Object, error) {
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
	source := map[string]any{
		"repoURL": w.RepoURL,
		"path":    path,
	}

	// Configure destination
	dest := map[string]any{
		"server":    "https://kubernetes.default.svc",
		"namespace": "default",
	}

	// Set spec fields
	if err := unstructured.SetNestedField(app.Object, source, "spec", "source"); err != nil {
		return nil, errors.Wrap(err, "failed to set spec.source")
	}
	if err := unstructured.SetNestedField(app.Object, dest, "spec", "destination"); err != nil {
		return nil, errors.Wrap(err, "failed to set spec.destination")
	}

	// Add dependencies if present
	if len(b.DependsOn) > 0 {
		var deps []string
		for _, d := range b.DependsOn {
			deps = append(deps, d.Name)
		}
		if err := unstructured.SetNestedStringSlice(app.Object, deps, "spec", "dependencies"); err != nil {
			return nil, errors.Wrap(err, "failed to set spec.dependencies")
		}
	}

	return app, nil
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
func (w *WorkflowEngine) CreateLayoutWithResources(c *stack.Cluster, rulesInterface stack.LayoutRulesProvider) (stack.ManifestLayoutResult, error) {
	rules, ok := rulesInterface.(layout.LayoutRules)
	if !ok {
		return nil, errors.New("rules must be of type layout.LayoutRules")
	}
	// An integrated Flux placement asks the writer to reference child layouts
	// through Flux CRs, and an Argo layout has none: the argocd/ directory
	// and the child layouts would never be applied.
	if rules.FluxPlacement == layout.FluxIntegratedPerLayout || rules.FluxPlacement == layout.FluxIntegratedPerBundle {
		return nil, errors.Errorf("ArgoCD layouts do not support FluxPlacement %q: no Flux Kustomization exists to apply the argocd directory; use FluxSeparate or leave it unset", rules.FluxPlacement)
	}
	// Generate the base manifest layout
	ml, err := layout.WalkCluster(c, rules)
	if err != nil {
		return nil, err
	}

	// For ArgoCD, we typically create a separate argocd directory for
	// Applications. They are generated from the layout just walked, so each
	// source.path is a directory this layout writes.
	apps, err := w.generateFromLayout(ml, c)
	if err != nil {
		return nil, err
	}

	if len(apps) > 0 {
		// Namespace is ml's own directory, since ml references this layout as
		// a child (go-kure/kure#771).
		argoCDLayout := &layout.ManifestLayout{
			Name:       "argocd",
			Namespace:  ml.FullRepoPath(),
			FilePer:    layout.FilePerResource,
			FileNaming: ml.FileNaming,
			Resources:  apps,
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

	return nil, errors.New("ArgoCD bootstrap is not yet implemented")
}

// SupportedBootstrapModes returns the bootstrap modes supported by ArgoCD.
func (w *WorkflowEngine) SupportedBootstrapModes() []string {
	return nil
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
