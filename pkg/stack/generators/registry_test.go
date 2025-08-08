package generators_test

import (
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/generators"
	_ "github.com/go-kure/kure/pkg/stack/generators/appworkload" // Register AppWorkload
	_ "github.com/go-kure/kure/pkg/stack/generators/fluxhelm"   // Register FluxHelm
)

func TestRegistry(t *testing.T) {
	t.Run("ParseAPIVersion", func(t *testing.T) {
		tests := []struct {
			name       string
			apiVersion string
			kind       string
			expected   generators.GVK
		}{
			{
				name:       "full GVK",
				apiVersion: "generators.gokure.dev/v1alpha1",
				kind:       "AppWorkload",
				expected: generators.GVK{
					Group:   "generators.gokure.dev",
					Version: "v1alpha1",
					Kind:    "AppWorkload",
				},
			},
			{
				name:       "core version",
				apiVersion: "v1",
				kind:       "ConfigMap",
				expected: generators.GVK{
					Group:   "",
					Version: "v1",
					Kind:    "ConfigMap",
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				gvk := generators.ParseAPIVersion(tt.apiVersion, tt.kind)
				if gvk != tt.expected {
					t.Errorf("ParseAPIVersion() = %v, want %v", gvk, tt.expected)
				}
			})
		}
	})

	t.Run("Register and Create", func(t *testing.T) {
		// Test using global registry functions
		testGVK := generators.GVK{
			Group:   "test.gokure.dev",
			Version: "v1",
			Kind:    "TestGeneratorRC", // Make unique
		}
		
		called := false
		generators.Register(testGVK, func() stack.ApplicationConfig {
			called = true
			return &mockConfig{}
		})

		// Create an instance
		config, err := generators.CreateFromGVK(testGVK)
		if err != nil {
			t.Fatalf("Failed to create config: %v", err)
		}

		if !called {
			t.Error("Factory function was not called")
		}

		if config == nil {
			t.Error("Created config is nil")
		}
	})

	t.Run("Create Unknown Type", func(t *testing.T) {
		unknownGVK := generators.GVK{
			Group:   "unknown.gokure.dev",
			Version: "v1",
			Kind:    "UnknownType",
		}

		_, err := generators.CreateFromGVK(unknownGVK)
		if err == nil {
			t.Error("Expected error for unknown type, got nil")
		}
	})

	t.Run("Global Registry", func(t *testing.T) {
		// Test that built-in types are registered
		if !generators.HasKind("generators.gokure.dev/v1alpha1", "AppWorkload") {
			t.Error("AppWorkload should be registered")
		}

		if !generators.HasKind("generators.gokure.dev/v1alpha1", "FluxHelm") {
			t.Error("FluxHelm should be registered")
		}

		// List all registered kinds
		kinds := generators.ListKinds()
		if len(kinds) < 2 {
			t.Errorf("Expected at least 2 registered kinds, got %d", len(kinds))
		}
	})
}

// mockConfig is a test implementation of ApplicationConfig
type mockConfig struct{}

func (m *mockConfig) Generate(app *stack.Application) ([]*client.Object, error) {
	return nil, nil
}