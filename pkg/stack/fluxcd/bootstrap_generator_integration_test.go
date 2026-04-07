//go:build integration

package fluxcd_test

import (
	"testing"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
)

func TestGenerateGotkBootstrap(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()

	config := &stack.BootstrapConfig{
		Enabled:  true,
		FluxMode: "gotk",
	}

	rootNode := &stack.Node{Name: "test-cluster"}

	// gotk mode downloads manifests from GitHub — requires network access
	_, _ = bg.GenerateBootstrap(config, rootNode)
}

func TestGenerateGotkBootstrapWithOptions(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()

	config := &stack.BootstrapConfig{
		Enabled:         true,
		FluxMode:        "gotk",
		FluxVersion:     "v2.0.0",
		Registry:        "ghcr.io/fluxcd",
		ImagePullSecret: "my-secret",
		Components:      []string{"source-controller", "kustomize-controller"},
		SourceURL:       "oci://registry.example.com/flux-system",
		SourceRef:       "latest",
	}

	rootNode := &stack.Node{Name: "test-cluster"}

	// gotk mode downloads manifests from GitHub — requires network access
	_, _ = bg.GenerateBootstrap(config, rootNode)
}
