package v1alpha1

import (
	"sync"
	"testing"

	"github.com/go-kure/kure/internal/gvk"
)

func TestStackRegistry(t *testing.T) {
	// Test initial registration
	t.Run("default registrations", func(t *testing.T) {
		// Check that default types are registered
		clusterGVK := gvk.GVK{Group: "stack.gokure.dev", Version: "v1alpha1", Kind: "Cluster"}
		nodeGVK := gvk.GVK{Group: "stack.gokure.dev", Version: "v1alpha1", Kind: "Node"}
		bundleGVK := gvk.GVK{Group: "stack.gokure.dev", Version: "v1alpha1", Kind: "Bundle"}

		if !IsStackConfigRegistered(clusterGVK) {
			t.Error("Cluster should be registered by default")
		}
		if !IsStackConfigRegistered(nodeGVK) {
			t.Error("Node should be registered by default")
		}
		if !IsStackConfigRegistered(bundleGVK) {
			t.Error("Bundle should be registered by default")
		}
	})

	t.Run("create stack config", func(t *testing.T) {
		gvk := gvk.GVK{Group: "stack.gokure.dev", Version: "v1alpha1", Kind: "Cluster"}
		config, err := CreateStackConfig(gvk)
		if err != nil {
			t.Fatalf("unexpected error creating cluster config: %v", err)
		}

		if config == nil {
			t.Fatal("expected non-nil config")
		}

		if _, ok := config.(*ClusterConfig); !ok {
			t.Errorf("expected *ClusterConfig, got %T", config)
		}
	})

	t.Run("create unregistered config", func(t *testing.T) {
		gvk := gvk.GVK{Group: "unknown", Version: "v1", Kind: "Unknown"}
		_, err := CreateStackConfig(gvk)
		if err == nil {
			t.Error("expected error for unregistered GVK")
		}
	})

	t.Run("get registered GVKs", func(t *testing.T) {
		gvks := GetRegisteredStackGVKs()
		if len(gvks) < 3 {
			t.Errorf("expected at least 3 registered GVKs, got %d", len(gvks))
		}

		// Check that our default types are present
		hasCluster, hasNode, hasBundle := false, false, false
		for _, g := range gvks {
			if g.Kind == "Cluster" && g.Group == "stack.gokure.dev" {
				hasCluster = true
			}
			if g.Kind == "Node" && g.Group == "stack.gokure.dev" {
				hasNode = true
			}
			if g.Kind == "Bundle" && g.Group == "stack.gokure.dev" {
				hasBundle = true
			}
		}

		if !hasCluster || !hasNode || !hasBundle {
			t.Error("missing expected default GVKs")
		}
	})

	t.Run("custom registration", func(t *testing.T) {
		// Register a custom type
		customGVK := gvk.GVK{Group: "custom.gokure.dev", Version: "v1", Kind: "Custom"}
		RegisterStackConfig(customGVK, func() StackConfig {
			return &ClusterConfig{} // Just reuse ClusterConfig for testing
		})

		if !IsStackConfigRegistered(customGVK) {
			t.Error("custom GVK should be registered")
		}

		config, err := CreateStackConfig(customGVK)
		if err != nil {
			t.Fatalf("unexpected error creating custom config: %v", err)
		}

		if config == nil {
			t.Fatal("expected non-nil config for custom GVK")
		}
	})
}

func TestStackRegistry_Concurrency(t *testing.T) {
	// Test concurrent registration and creation
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent creates
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			gvk := gvk.GVK{Group: "stack.gokure.dev", Version: "v1alpha1", Kind: "Cluster"}
			_, err := CreateStackConfig(gvk)
			if err != nil {
				errors <- err
			}
		}()
	}

	// Concurrent registrations
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			gvk := gvk.GVK{
				Group:   "concurrent.test",
				Version: "v1",
				Kind:    string(rune('A' + idx%26)), // A-Z kinds
			}
			RegisterStackConfig(gvk, func() StackConfig {
				return &ClusterConfig{}
			})
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("concurrent operation error: %v", err)
	}
}

func TestStackWrapper(t *testing.T) {
	t.Run("create wrapper", func(t *testing.T) {
		gvk := gvk.GVK{Group: "stack.gokure.dev", Version: "v1alpha1", Kind: "Cluster"}
		wrapper, err := CreateStackWrapper(gvk)
		if err != nil {
			t.Fatalf("unexpected error creating wrapper: %v", err)
		}

		if wrapper == nil {
			t.Fatal("expected non-nil wrapper")
		}

		if wrapper.GetGVK() != gvk {
			t.Errorf("expected GVK %v, got %v", gvk, wrapper.GetGVK())
		}

		if wrapper.GetType() != StackConfigTypeCluster {
			t.Errorf("expected type %s, got %s", StackConfigTypeCluster, wrapper.GetType())
		}
	})

	t.Run("new wrapper", func(t *testing.T) {
		config := NewClusterConfig("test-cluster")
		gvk := gvk.GVK{Group: "stack.gokure.dev", Version: "v1alpha1", Kind: "Cluster"}
		wrapper := NewStackWrapper(gvk, config)

		if wrapper.GetConfig() != config {
			t.Error("wrapper should return the same config instance")
		}

		if err := wrapper.Validate(); err != nil {
			t.Errorf("unexpected validation error: %v", err)
		}
	})

	t.Run("type assertions", func(t *testing.T) {
		// Test AsCluster
		clusterGVK := gvk.GVK{Group: "stack.gokure.dev", Version: "v1alpha1", Kind: "Cluster"}
		clusterWrapper, _ := CreateStackWrapper(clusterGVK)

		if cluster, ok := clusterWrapper.AsCluster(); !ok {
			t.Error("expected successful cluster type assertion")
		} else if cluster == nil {
			t.Error("expected non-nil cluster")
		}

		if _, ok := clusterWrapper.AsNode(); ok {
			t.Error("cluster should not assert as node")
		}

		if _, ok := clusterWrapper.AsBundle(); ok {
			t.Error("cluster should not assert as bundle")
		}

		// Test AsNode
		nodeGVK := gvk.GVK{Group: "stack.gokure.dev", Version: "v1alpha1", Kind: "Node"}
		nodeWrapper, _ := CreateStackWrapper(nodeGVK)

		if node, ok := nodeWrapper.AsNode(); !ok {
			t.Error("expected successful node type assertion")
		} else if node == nil {
			t.Error("expected non-nil node")
		}

		if _, ok := nodeWrapper.AsCluster(); ok {
			t.Error("node should not assert as cluster")
		}

		// Test AsBundle
		bundleGVK := gvk.GVK{Group: "stack.gokure.dev", Version: "v1alpha1", Kind: "Bundle"}
		bundleWrapper, _ := CreateStackWrapper(bundleGVK)

		if bundle, ok := bundleWrapper.AsBundle(); !ok {
			t.Error("expected successful bundle type assertion")
		} else if bundle == nil {
			t.Error("expected non-nil bundle")
		}

		if _, ok := bundleWrapper.AsCluster(); ok {
			t.Error("bundle should not assert as cluster")
		}
	})

	t.Run("validate wrapper", func(t *testing.T) {
		// Test with valid config
		config := NewClusterConfig("test")
		gvk := gvk.GVK{Group: "stack.gokure.dev", Version: "v1alpha1", Kind: "Cluster"}
		wrapper := NewStackWrapper(gvk, config)

		if err := wrapper.Validate(); err != nil {
			t.Errorf("unexpected validation error: %v", err)
		}

		// Test with nil config
		nilWrapper := &StackWrapper{gvk: gvk, config: nil}
		if err := nilWrapper.Validate(); err == nil {
			t.Error("expected validation error for nil config")
		}

		// Test with invalid config
		invalidConfig := &ClusterConfig{} // No name
		invalidWrapper := NewStackWrapper(gvk, invalidConfig)
		if err := invalidWrapper.Validate(); err == nil {
			t.Error("expected validation error for invalid config")
		}
	})

	t.Run("get type for unknown kind", func(t *testing.T) {
		config := NewClusterConfig("test")
		gvk := gvk.GVK{Group: "custom", Version: "v1", Kind: "CustomKind"}
		wrapper := NewStackWrapper(gvk, config)

		expectedType := StackConfigType("CustomKind")
		if wrapper.GetType() != expectedType {
			t.Errorf("expected type %s for unknown kind, got %s", expectedType, wrapper.GetType())
		}
	})
}

func TestStackConfigInterface(t *testing.T) {
	// Ensure all types implement StackConfig interface
	t.Run("interface compliance", func(t *testing.T) {
		var _ StackConfig = (*ClusterConfig)(nil)
		var _ StackConfig = (*NodeConfig)(nil)
		var _ StackConfig = (*BundleConfig)(nil)
	})

	t.Run("metadata operations", func(t *testing.T) {
		configs := []StackConfig{
			NewClusterConfig("cluster"),
			NewNodeConfig("node"),
			NewBundleConfig("bundle"),
		}

		for _, config := range configs {
			// Test name operations
			config.SetName("test-name")
			if config.GetName() != "test-name" {
				t.Errorf("expected name 'test-name', got %s", config.GetName())
			}

			// Test namespace operations
			config.SetNamespace("test-namespace")
			if config.GetNamespace() != "test-namespace" {
				t.Errorf("expected namespace 'test-namespace', got %s", config.GetNamespace())
			}
		}
	})

	t.Run("validation", func(t *testing.T) {
		// Test that all types validate correctly with valid data
		validConfigs := []StackConfig{
			NewClusterConfig("valid-cluster"),
			NewNodeConfig("valid-node"),
			NewBundleConfig("valid-bundle"),
		}

		for _, config := range validConfigs {
			if err := config.Validate(); err != nil {
				t.Errorf("unexpected validation error for %T: %v", config, err)
			}
		}

		// Test that all types fail validation without name
		invalidConfigs := []StackConfig{
			&ClusterConfig{},
			&NodeConfig{},
			&BundleConfig{},
		}

		for _, config := range invalidConfigs {
			if err := config.Validate(); err == nil {
				t.Errorf("expected validation error for %T without name", config)
			}
		}
	})
}

func BenchmarkCreateStackConfig(b *testing.B) {
	gvk := gvk.GVK{Group: "stack.gokure.dev", Version: "v1alpha1", Kind: "Cluster"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = CreateStackConfig(gvk)
	}
}

func BenchmarkStackWrapper(b *testing.B) {
	gvk := gvk.GVK{Group: "stack.gokure.dev", Version: "v1alpha1", Kind: "Node"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wrapper, _ := CreateStackWrapper(gvk)
		_ = wrapper.Validate()
	}
}
