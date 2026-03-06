package stack

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
)

// LayoutRulesProvider is the interface for layout configuration passed to
// CreateLayoutWithResources. The concrete implementation is layout.LayoutRules
// from pkg/stack/layout. Defined here to avoid an import cycle between
// pkg/stack and pkg/stack/layout.
type LayoutRulesProvider interface {
	Validate() error
}

// ManifestLayoutResult is the interface for layout results returned by
// CreateLayoutWithResources. The concrete implementation is *layout.ManifestLayout
// from pkg/stack/layout. Callers that need the full concrete type should
// type-assert: ml, ok := result.(*layout.ManifestLayout).
type ManifestLayoutResult interface {
	WriteToDisk(basePath string) error
	FullRepoPath() string
}

// Workflow defines the core interface for GitOps workflow implementations.
// This interface provides a minimal abstraction for converting stack definitions
// into GitOps-specific resources (Flux Kustomizations, ArgoCD Applications, etc.).
type Workflow interface {
	// GenerateFromCluster creates GitOps resources from a cluster definition.
	// This is the primary entry point for resource generation.
	GenerateFromCluster(*Cluster) ([]client.Object, error)

	// CreateLayoutWithResources creates a new manifest layout that includes
	// both the application manifests and the GitOps resources needed to
	// deploy them. This combines manifest generation with GitOps resource
	// generation in a single operation.
	// The rules parameter must be a layout.LayoutRules value.
	// The returned ManifestLayoutResult is a *layout.ManifestLayout.
	CreateLayoutWithResources(*Cluster, LayoutRulesProvider) (ManifestLayoutResult, error)

	// GenerateBootstrap creates bootstrap resources for initializing the
	// GitOps system itself. This is used to set up the GitOps controller
	// (Flux, ArgoCD, etc.) in the cluster.
	GenerateBootstrap(*BootstrapConfig, *Node) ([]client.Object, error)
}

// NewWorkflow creates a workflow implementation based on the provider type.
// Supported providers: "flux", "argocd"
func NewWorkflow(provider string) (Workflow, error) {
	switch provider {
	case "flux", "fluxcd":
		if newFluxWorkflow == nil {
			return nil, errors.New("workflow provider 'flux' not registered — import github.com/go-kure/kure/pkg/stack/fluxcd")
		}
		return newFluxWorkflow(), nil
	case "argo", "argocd":
		if newArgoWorkflow == nil {
			return nil, errors.New("workflow provider 'argocd' not registered — import github.com/go-kure/kure/pkg/stack/argocd")
		}
		return newArgoWorkflow(), nil
	default:
		return nil, errors.Errorf("unsupported GitOps provider: %s", provider)
	}
}

// These factory functions will be implemented by the respective packages
// to avoid import cycles.
var (
	newFluxWorkflow func() Workflow
	newArgoWorkflow func() Workflow
)

// RegisterFluxWorkflow registers the Flux workflow factory.
// This is called by the fluxcd package during init.
func RegisterFluxWorkflow(factory func() Workflow) {
	newFluxWorkflow = factory
}

// RegisterArgoWorkflow registers the ArgoCD workflow factory.
// This is called by the argocd package during init.
func RegisterArgoWorkflow(factory func() Workflow) {
	newArgoWorkflow = factory
}
