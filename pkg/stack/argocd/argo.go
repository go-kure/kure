package argocd

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Ensure WorkflowEngine implements the stack.Workflow interface
var _ stack.Workflow = (*WorkflowEngine)(nil)

// argoDirName is the directory CreateLayoutWithResources writes the
// Applications to, inside the directory of the layout the walk returns.
const argoDirName = "argocd"

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
// it walks the cluster with the rules and generates from that layout (see
// generateFromLayout), so each source.path is the directory a walk with those
// rules writes the bundle to. The rules must be the ones the caller writes
// the tree with, and are held to what CreateLayoutWithResources accepts (see
// layoutRules, then the walk's layout.LayoutRules.Validate). An absent or
// empty cluster is walked too, so invalid rules are an error whatever the
// cluster is; with valid rules it yields nothing.
func (w *WorkflowEngine) GenerateFromCluster(c *stack.Cluster, rulesInterface stack.LayoutRulesProvider) ([]client.Object, error) {
	rules, err := layoutRules(rulesInterface)
	if err != nil {
		return nil, err
	}
	ml, err := layout.WalkCluster(c, rules)
	if err != nil {
		return nil, err
	}
	return w.generateFromLayout(ml, c)
}

// layoutRules returns the layout.LayoutRules behind a cluster-level entry
// point's rules, and refuses anything else (nil included) and an integrated
// Flux placement: that asks the writer to reference child layouts through
// Flux CRs, and an Argo layout has none, so the argocd/ directory and the
// child layouts of a tree written with such rules would never be applied.
func layoutRules(rulesInterface stack.LayoutRulesProvider) (layout.LayoutRules, error) {
	rules, ok := rulesInterface.(layout.LayoutRules)
	if !ok {
		return layout.LayoutRules{}, errors.New("rules must be of type layout.LayoutRules")
	}
	if rules.FluxPlacement == layout.FluxIntegratedPerLayout || rules.FluxPlacement == layout.FluxIntegratedPerBundle {
		return layout.LayoutRules{}, errors.Errorf("ArgoCD layouts do not support FluxPlacement %q: no Flux Kustomization exists to apply the argocd directory; use FluxSeparate or leave it unset", rules.FluxPlacement)
	}
	return rules, nil
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
	if err := refuseDeliveryIntent(root); err != nil {
		return nil, err
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
// spec.source.path is path, verbatim. The Application is named by b's
// KustomizationName, or its Name without one (Bundle.UnitName), and
// spec.dependencies names each DependsOn bundle the same way.
func (w *WorkflowEngine) applicationForBundle(b *stack.Bundle, path string) (client.Object, error) {
	app := &unstructured.Unstructured{}
	app.SetAPIVersion("argoproj.io/v1alpha1")
	app.SetKind("Application")
	app.SetName(b.UnitName())
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
			deps = append(deps, d.UnitName())
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
// It adds nothing, and refuses a delivery intent set by an application the
// layout records (see refuseDeliveryIntent) or by one of c's applications: a
// layout the caller built records none, so c is read as well.
func (w *WorkflowEngine) IntegrateWithLayout(ml *layout.ManifestLayout, c *stack.Cluster, rules layout.LayoutRules) error {
	// ArgoCD Applications typically don't need layout integration
	// as they reference external repositories
	if err := refuseDeliveryIntent(ml); err != nil {
		return err
	}
	return refuseClusterDeliveryIntent(c)
}

// refuseClusterDeliveryIntent fails when an application of c — in a node's
// bundle or in an umbrella child below it — sets a delivery intent. Each node
// and bundle is read once, so a tree that loops back ends.
func refuseClusterDeliveryIntent(c *stack.Cluster) error {
	if c == nil {
		return nil
	}
	nodes := map[*stack.Node]bool{}
	bundles := map[*stack.Bundle]bool{}
	var bundle func(b *stack.Bundle) error
	bundle = func(b *stack.Bundle) error {
		if b == nil || bundles[b] {
			return nil
		}
		bundles[b] = true
		for _, app := range b.Applications {
			if app != nil && !app.Delivery.IsZero() {
				return errors.Errorf("application %q (bundle %q) sets a delivery intent, which the ArgoCD workflow cannot express yet; leave Application.Delivery unset or use the Flux workflow",
					app.Name, b.Name)
			}
		}
		for _, child := range b.Children {
			if err := bundle(child); err != nil {
				return err
			}
		}
		return nil
	}
	var node func(n *stack.Node) error
	node = func(n *stack.Node) error {
		if n == nil || nodes[n] {
			return nil
		}
		nodes[n] = true
		if err := bundle(n.Bundle); err != nil {
			return err
		}
		for _, child := range n.Children {
			if err := node(child); err != nil {
				return err
			}
		}
		return nil
	}
	return node(c.Node)
}

// refuseDeliveryIntent fails when an application rendered in the walked tree
// under root sets a delivery intent. This workflow has no mapping for one yet
// (ArgoCD states pruning and replacement through its own sync options), and
// rendering the application without it would silently drop what it asked for.
func refuseDeliveryIntent(root *layout.ManifestLayout) error {
	if root == nil {
		return nil
	}
	for _, rec := range root.OriginApplicationObjects() {
		if rec.Application != nil && !rec.Application.Delivery.IsZero() {
			return errors.Errorf("application %q (rendered in %q) sets a delivery intent, which the ArgoCD workflow cannot express yet; leave Application.Delivery unset or use the Flux workflow",
				rec.Application.Name, root.FullRepoPath())
		}
	}
	for _, child := range root.Children {
		if err := refuseDeliveryIntent(child); err != nil {
			return err
		}
	}
	return nil
}

// CreateLayoutWithResources creates a new layout that includes ArgoCD Applications.
func (w *WorkflowEngine) CreateLayoutWithResources(c *stack.Cluster, rulesInterface stack.LayoutRulesProvider) (stack.ManifestLayoutResult, error) {
	rules, err := layoutRules(rulesInterface)
	if err != nil {
		return nil, err
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
		// The directory must be free. The root node's bundle is rendered in
		// a directory named after it (go-kure/kure#979), and a child node in
		// one named after the node: either of them named like the
		// Applications' directory would share it with them, which the
		// writers refuse only when the tree is written. Names are compared
		// as the writers compare directories, case-insensitively.
		for _, child := range ml.Children {
			if child == nil || !strings.EqualFold(child.Name, argoDirName) {
				continue
			}
			// A node's directory is named after the node, whatever it renders.
			switch {
			case len(child.OriginNodes()) > 0:
				return nil, errors.Errorf("node %q is rendered to %q, the directory the ArgoCD Applications are written to: rename the node",
					child.OriginNodes()[0].Name, child.FullRepoPath())
			case len(child.OriginBundles()) > 0:
				return nil, errors.Errorf("bundle %q is rendered to %q, the directory the ArgoCD Applications are written to: rename the bundle",
					child.OriginBundles()[0].Name, child.FullRepoPath())
			}
		}
		// Namespace is ml's own directory, since ml references this layout as
		// a child (go-kure/kure#771).
		argoCDLayout := &layout.ManifestLayout{
			Name:       argoDirName,
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
