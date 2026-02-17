package stack

import (
	"strings"
	"testing"
)

func TestNewWorkflow_UnregisteredProvider(t *testing.T) {
	// Save original factories and restore after test
	origFlux := newFluxWorkflow
	origArgo := newArgoWorkflow
	defer func() {
		newFluxWorkflow = origFlux
		newArgoWorkflow = origArgo
	}()

	// Nil out the factories to simulate missing imports
	newFluxWorkflow = nil
	newArgoWorkflow = nil

	tests := []struct {
		name     string
		provider string
		wantMsg  string
	}{
		{
			name:     "flux not registered",
			provider: "flux",
			wantMsg:  "not registered",
		},
		{
			name:     "fluxcd not registered",
			provider: "fluxcd",
			wantMsg:  "not registered",
		},
		{
			name:     "argo not registered",
			provider: "argo",
			wantMsg:  "not registered",
		},
		{
			name:     "argocd not registered",
			provider: "argocd",
			wantMsg:  "not registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf, err := NewWorkflow(tt.provider)
			if err == nil {
				t.Fatal("expected error when factory is nil, got nil")
			}
			if wf != nil {
				t.Error("expected nil workflow when factory is nil")
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("error %q should contain %q", err.Error(), tt.wantMsg)
			}
		})
	}
}
