package workflow

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// ResourceGenerator defines the core interface for generating GitOps resources
// from stack components. This is the main interface that workflow implementations
// should focus on.
type ResourceGenerator interface {
	// GenerateFromCluster creates GitOps resources from a cluster definition
	GenerateFromCluster(*stack.Cluster) ([]client.Object, error)
	// GenerateFromNode creates GitOps resources from a node definition
	GenerateFromNode(*stack.Node) ([]client.Object, error)
	// GenerateFromBundle creates GitOps resources from a bundle definition
	GenerateFromBundle(*stack.Bundle) ([]client.Object, error)
}

// LayoutIntegrator handles the integration of GitOps resources with manifest layouts.
// This separates layout concerns from pure resource generation.
type LayoutIntegrator interface {
	// IntegrateWithLayout adds GitOps resources to an existing manifest layout
	IntegrateWithLayout(*layout.ManifestLayout, *stack.Cluster, layout.LayoutRules) error
	// CreateLayoutWithResources creates a new layout that includes GitOps resources
	CreateLayoutWithResources(*stack.Cluster, layout.LayoutRules) (*layout.ManifestLayout, error)
}

// BootstrapGenerator handles the generation of bootstrap resources for
// initializing GitOps systems.
type BootstrapGenerator interface {
	// GenerateBootstrap creates bootstrap resources for setting up the GitOps system
	GenerateBootstrap(*stack.BootstrapConfig, *stack.Node) ([]client.Object, error)
	// SupportedBootstrapModes returns the bootstrap modes supported by this generator
	SupportedBootstrapModes() []string
}

// PackageAwareGenerator extends ResourceGenerator with package-aware capabilities
// for workflows that support multiple packaging modes.
type PackageAwareGenerator interface {
	ResourceGenerator
	// GenerateByPackage groups generated resources by package type
	GenerateByPackage(*stack.Cluster) (map[string][]client.Object, error)
	// SupportedPackageTypes returns the package types supported by this generator
	SupportedPackageTypes() []schema.GroupVersionKind
}

// WorkflowEngine combines multiple workflow capabilities into a complete
// GitOps workflow implementation. Implementations can compose different
// generators to provide full workflow functionality.
type WorkflowEngine interface {
	ResourceGenerator
	LayoutIntegrator
	BootstrapGenerator
	
	// GetName returns a human-readable name for this workflow engine
	GetName() string
	// GetVersion returns the version of this workflow engine
	GetVersion() string
}

