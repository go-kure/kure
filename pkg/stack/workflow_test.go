package stack_test

import (
	"testing"

	"github.com/go-kure/kure/pkg/stack"
	_ "github.com/go-kure/kure/pkg/stack/argocd"
	_ "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

func TestNewWorkflow(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		wantErr  bool
	}{
		{
			name:     "flux provider",
			provider: "flux",
			wantErr:  false,
		},
		{
			name:     "fluxcd provider alias",
			provider: "fluxcd",
			wantErr:  false,
		},
		{
			name:     "argo provider",
			provider: "argo",
			wantErr:  false,
		},
		{
			name:     "argocd provider alias",
			provider: "argocd",
			wantErr:  false,
		},
		{
			name:     "unsupported provider",
			provider: "unknown",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf, err := stack.NewWorkflow(tt.provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewWorkflow() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && wf == nil {
				t.Error("NewWorkflow() returned nil workflow")
			}
		})
	}
}

func TestWorkflowInterface(t *testing.T) {
	// Test that both implementations satisfy the interface
	providers := []string{"flux", "argocd"}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			wf, err := stack.NewWorkflow(provider)
			if err != nil {
				t.Fatalf("Failed to create %s workflow: %v", provider, err)
			}

			// Create a minimal cluster for testing
			cluster := &stack.Cluster{
				Name: "test-cluster",
				Node: &stack.Node{
					Name: "root",
					Bundle: &stack.Bundle{
						Name: "test-bundle",
					},
				},
				GitOps: &stack.GitOpsConfig{
					Type: provider,
					Bootstrap: &stack.BootstrapConfig{
						Enabled:  true,
						FluxMode: "gotk",
					},
				},
			}

			// Test GenerateFromCluster
			_, err = wf.GenerateFromCluster(cluster)
			if err != nil {
				t.Errorf("GenerateFromCluster() error = %v", err)
			}
			// It's ok for resources to be nil for minimal test clusters

			// Test CreateLayoutWithResources
			rules := layout.LayoutRules{}
			result, err := wf.CreateLayoutWithResources(cluster, rules)
			if err != nil {
				t.Errorf("CreateLayoutWithResources() error = %v", err)
			}
			if result == nil {
				t.Error("CreateLayoutWithResources() returned nil layout")
			}
			// Verify it returns the expected type
			if _, ok := result.(*layout.ManifestLayout); !ok {
				t.Error("CreateLayoutWithResources() returned unexpected type")
			}

			// Test GenerateBootstrap
			_, err = wf.GenerateBootstrap(cluster.GitOps.Bootstrap, cluster.Node)
			if provider == "argocd" {
				// ArgoCD bootstrap is not yet implemented, expect error
				if err == nil {
					t.Error("GenerateBootstrap() expected error for unimplemented ArgoCD bootstrap")
				}
			} else if err != nil {
				t.Errorf("GenerateBootstrap() error = %v", err)
			}
		})
	}
}
