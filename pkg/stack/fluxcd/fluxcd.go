package fluxcd

import (
	"github.com/go-kure/kure/pkg/stack/layout"
)

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

// EngineWithConfig returns a WorkflowEngine with custom configuration.
func EngineWithConfig(mode layout.KustomizationMode, placement layout.FluxPlacement) *WorkflowEngine {
	return NewWorkflowEngineWithConfig(mode, placement)
}