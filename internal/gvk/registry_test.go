package gvk

import (
	"testing"
)

// testType is a test type for registry testing
type testType struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

func TestRegistry(t *testing.T) {
	t.Run("NewRegistry", func(t *testing.T) {
		registry := NewRegistry[testType]()
		if registry == nil {
			t.Fatal("NewRegistry returned nil")
		}

		if count := registry.Count(); count != 0 {
			t.Errorf("Expected count 0, got %d", count)
		}
	})

	t.Run("Register and Create", func(t *testing.T) {
		registry := NewRegistry[testType]()
		gvk := GVK{
			Group:   "test.example.com",
			Version: "v1",
			Kind:    "TestType",
		}

		// Register a factory
		called := false
		registry.Register(gvk, func() testType {
			called = true
			return testType{
				Name:  "test",
				Value: "created",
			}
		})

		// Verify registration
		if !registry.HasGVK(gvk) {
			t.Error("GVK should be registered")
		}

		if count := registry.Count(); count != 1 {
			t.Errorf("Expected count 1, got %d", count)
		}

		// Create instance
		instance, err := registry.Create(gvk)
		if err != nil {
			t.Fatalf("Failed to create instance: %v", err)
		}

		if !called {
			t.Error("Factory function was not called")
		}

		if instance.Name != "test" || instance.Value != "created" {
			t.Errorf("Instance not created correctly: %+v", instance)
		}
	})

	t.Run("CreateFromAPIVersion", func(t *testing.T) {
		registry := NewRegistry[testType]()
		gvk := GVK{
			Group:   "test.example.com",
			Version: "v1",
			Kind:    "TestType",
		}

		registry.Register(gvk, func() testType {
			return testType{Name: "api-version-test"}
		})

		instance, err := registry.CreateFromAPIVersion("test.example.com/v1", "TestType")
		if err != nil {
			t.Fatalf("Failed to create from API version: %v", err)
		}

		if instance.Name != "api-version-test" {
			t.Errorf("Wrong instance created: %+v", instance)
		}
	})

	t.Run("Create unknown type", func(t *testing.T) {
		registry := NewRegistry[testType]()
		unknownGVK := GVK{
			Group:   "unknown.example.com",
			Version: "v1",
			Kind:    "Unknown",
		}

		_, err := registry.Create(unknownGVK)
		if err == nil {
			t.Error("Expected error for unknown type")
		}
	})

	t.Run("ListGVKs", func(t *testing.T) {
		registry := NewRegistry[testType]()

		gvks := []GVK{
			{Group: "test1.example.com", Version: "v1", Kind: "Type1"},
			{Group: "test2.example.com", Version: "v1", Kind: "Type2"},
		}

		for _, gvk := range gvks {
			registry.Register(gvk, func() testType { return testType{} })
		}

		listedGVKs := registry.ListGVKs()
		if len(listedGVKs) != 2 {
			t.Errorf("Expected 2 GVKs, got %d", len(listedGVKs))
		}

		// Check that all registered GVKs are in the list
		found := make(map[GVK]bool)
		for _, listed := range listedGVKs {
			found[listed] = true
		}

		for _, expected := range gvks {
			if !found[expected] {
				t.Errorf("GVK %v not found in listed GVKs", expected)
			}
		}
	})

	t.Run("HasAPIVersion", func(t *testing.T) {
		registry := NewRegistry[testType]()
		gvk := GVK{
			Group:   "test.example.com",
			Version: "v1",
			Kind:    "TestType",
		}

		registry.Register(gvk, func() testType { return testType{} })

		if !registry.HasAPIVersion("test.example.com/v1", "TestType") {
			t.Error("HasAPIVersion should return true for registered type")
		}

		if registry.HasAPIVersion("unknown.example.com/v1", "Unknown") {
			t.Error("HasAPIVersion should return false for unknown type")
		}
	})
}
