package fluxcd

import (
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

func init() {
	// Register the Flux workflow factory with the stack package
	stack.RegisterFluxWorkflow(func() stack.Workflow {
		return Engine()
	})
}

// Engine returns a WorkflowEngine initialized with defaults.
// This is the primary entry point for FluxCD workflow functionality.
func Engine() *WorkflowEngine {
	return NewWorkflowEngine()
}

// EngineWithMode returns a WorkflowEngine with a specific kustomization mode.
func EngineWithMode(mode layout.KustomizationMode) *WorkflowEngine {
	engine := NewWorkflowEngine()
	engine.SetKustomizationMode(mode)
	return engine
}

// EngineWithConfig returns a WorkflowEngine with a specific kustomization
// mode. Placement is no longer a constructor argument; set it on
// layout.LayoutRules.FluxPlacement at call time.
func EngineWithConfig(mode layout.KustomizationMode) *WorkflowEngine {
	return NewWorkflowEngineWithConfig(mode)
}
