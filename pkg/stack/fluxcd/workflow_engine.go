package fluxcd

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Ensure WorkflowEngine implements the stack.Workflow interface
var _ stack.Workflow = (*WorkflowEngine)(nil)

// WorkflowEngine implements the stack.Workflow interface by composing
// the specialized generator components. This provides a complete FluxCD workflow
// implementation with clear separation of concerns.
type WorkflowEngine struct {
	// ResourceGen handles core resource generation
	ResourceGen *ResourceGenerator
	// LayoutInteg handles layout integration
	LayoutInteg *LayoutIntegrator
	// BootstrapGen handles bootstrap resource generation
	BootstrapGen *BootstrapGenerator
}

// NewWorkflowEngine creates a FluxCD workflow engine with default components.
func NewWorkflowEngine() *WorkflowEngine {
	resourceGen := NewResourceGenerator()
	layoutInteg := NewLayoutIntegrator(resourceGen)
	bootstrapGen := NewBootstrapGenerator()

	return &WorkflowEngine{
		ResourceGen:  resourceGen,
		LayoutInteg:  layoutInteg,
		BootstrapGen: bootstrapGen,
	}
}

// NewWorkflowEngineWithConfig creates a workflow engine with custom configuration.
func NewWorkflowEngineWithConfig(mode layout.KustomizationMode, placement layout.FluxPlacement) *WorkflowEngine {
	resourceGen := NewResourceGenerator()
	resourceGen.Mode = mode

	layoutInteg := NewLayoutIntegrator(resourceGen)
	layoutInteg.FluxPlacement = placement

	bootstrapGen := NewBootstrapGenerator()

	return &WorkflowEngine{
		ResourceGen:  resourceGen,
		LayoutInteg:  layoutInteg,
		BootstrapGen: bootstrapGen,
	}
}

// ResourceGenerator interface implementation

// GenerateFromCluster creates Flux resources from a cluster definition.
func (we *WorkflowEngine) GenerateFromCluster(c *stack.Cluster) ([]client.Object, error) {
	return we.ResourceGen.GenerateFromCluster(c)
}

// GenerateFromNode creates Flux resources from a node definition.
func (we *WorkflowEngine) GenerateFromNode(n *stack.Node) ([]client.Object, error) {
	return we.ResourceGen.GenerateFromNode(n)
}

// GenerateFromBundle creates Flux resources from a bundle definition.
func (we *WorkflowEngine) GenerateFromBundle(b *stack.Bundle) ([]client.Object, error) {
	return we.ResourceGen.GenerateFromBundle(b)
}

// LayoutIntegrator interface implementation

// IntegrateWithLayout adds Flux resources to an existing manifest layout.
func (we *WorkflowEngine) IntegrateWithLayout(ml *layout.ManifestLayout, c *stack.Cluster, rules layout.LayoutRules) error {
	return we.LayoutInteg.IntegrateWithLayout(ml, c, rules)
}

// CreateLayoutWithResources creates a new layout that includes Flux resources.
func (we *WorkflowEngine) CreateLayoutWithResources(c *stack.Cluster, rules interface{}) (interface{}, error) {
	layoutRules, ok := rules.(layout.LayoutRules)
	if !ok {
		return nil, fmt.Errorf("rules must be of type layout.LayoutRules")
	}
	return we.LayoutInteg.CreateLayoutWithResources(c, layoutRules)
}

// BootstrapGenerator interface implementation

// GenerateBootstrap creates bootstrap resources for setting up Flux.
func (we *WorkflowEngine) GenerateBootstrap(config *stack.BootstrapConfig, rootNode *stack.Node) ([]client.Object, error) {
	return we.BootstrapGen.GenerateBootstrap(config, rootNode)
}

// SupportedBootstrapModes returns the bootstrap modes supported by this engine.
func (we *WorkflowEngine) SupportedBootstrapModes() []string {
	return we.BootstrapGen.SupportedBootstrapModes()
}

// WorkflowEngine interface implementation

// GetName returns a human-readable name for this workflow engine.
func (we *WorkflowEngine) GetName() string {
	return "FluxCD Workflow Engine"
}

// GetVersion returns the version of this workflow engine.
func (we *WorkflowEngine) GetVersion() string {
	return "v2.0.0"
}

// Configuration methods

// SetKustomizationMode configures how Kustomization paths are generated.
func (we *WorkflowEngine) SetKustomizationMode(mode layout.KustomizationMode) {
	we.ResourceGen.Mode = mode
}

// SetFluxPlacement configures where Flux resources are placed in layouts.
func (we *WorkflowEngine) SetFluxPlacement(placement layout.FluxPlacement) {
	we.LayoutInteg.SetFluxPlacement(placement)
}

// GetResourceGenerator returns the underlying resource generator for advanced configuration.
func (we *WorkflowEngine) GetResourceGenerator() *ResourceGenerator {
	return we.ResourceGen
}

// GetLayoutIntegrator returns the underlying layout integrator for advanced configuration.
func (we *WorkflowEngine) GetLayoutIntegrator() *LayoutIntegrator {
	return we.LayoutInteg
}

// GetBootstrapGenerator returns the underlying bootstrap generator for advanced configuration.
func (we *WorkflowEngine) GetBootstrapGenerator() *BootstrapGenerator {
	return we.BootstrapGen
}
