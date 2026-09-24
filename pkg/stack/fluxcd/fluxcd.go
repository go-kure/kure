package fluxcd

import (
	"github.com/go-kure/kure/pkg/stack"
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
