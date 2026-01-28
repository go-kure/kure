package gvk_test

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/go-kure/kure/internal/gvk"
)

// TestSpec is a test type that implements named and namespaced interfaces
type TestSpec struct {
	Field1 string `yaml:"field1"`
	Field2 int    `yaml:"field2"`
}

// Named test type
type NamedTestSpec struct {
	TestSpec
	name string
}

func (n *NamedTestSpec) SetName(name string) {
	n.name = name
}

func (n *NamedTestSpec) GetName() string {
	return n.name
}

// Namespaced test type
type NamespacedTestSpec struct {
	NamedTestSpec
	namespace string
}

func (n *NamespacedTestSpec) SetNamespace(namespace string) {
	n.namespace = namespace
}

func (n *NamespacedTestSpec) GetNamespace() string {
	return n.namespace
}

func TestTypedWrapper_MarshalYAML(t *testing.T) {
	registry := gvk.NewRegistry[TestSpec]()
	registry.Register(gvk.GVK{Group: "test.io", Version: "v1", Kind: "TestSpec"}, func() TestSpec {
		return TestSpec{}
	})

	tests := []struct {
		name     string
		wrapper  *gvk.TypedWrapper[TestSpec]
		expected string
	}{
		{
			name: "basic marshaling",
			wrapper: &gvk.TypedWrapper[TestSpec]{
				APIVersion: "test.io/v1",
				Kind:       "TestSpec",
				Metadata: map[string]any{
					"name": "test-resource",
				},
				Spec: TestSpec{
					Field1: "value1",
					Field2: 42,
				},
			},
			expected: "test-resource",
		},
		{
			name: "with namespace",
			wrapper: &gvk.TypedWrapper[TestSpec]{
				APIVersion: "test.io/v1",
				Kind:       "TestSpec",
				Metadata: map[string]any{
					"name":      "test-resource",
					"namespace": "test-namespace",
				},
				Spec: TestSpec{
					Field1: "value1",
					Field2: 42,
				},
			},
			expected: "test-namespace",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := yaml.Marshal(test.wrapper)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			if len(data) == 0 {
				t.Error("expected non-empty marshaled data")
			}

			// Unmarshal to verify structure
			var result map[string]interface{}
			if err := yaml.Unmarshal(data, &result); err != nil {
				t.Fatalf("failed to unmarshal result: %v", err)
			}

			if result["apiVersion"] != test.wrapper.APIVersion {
				t.Errorf("expected apiVersion %s, got %v", test.wrapper.APIVersion, result["apiVersion"])
			}
			if result["kind"] != test.wrapper.Kind {
				t.Errorf("expected kind %s, got %v", test.wrapper.Kind, result["kind"])
			}

			if metadata, ok := result["metadata"].(map[string]interface{}); ok {
				if name, ok := metadata["name"].(string); ok && name != test.wrapper.GetName() {
					t.Errorf("expected name %s, got %s", test.wrapper.GetName(), name)
				}
				if test.expected == "test-namespace" {
					if namespace, ok := metadata["namespace"].(string); ok && namespace != "test-namespace" {
						t.Errorf("expected namespace test-namespace, got %s", namespace)
					}
				}
			}

			if _, ok := result["spec"]; !ok {
				t.Error("expected spec in marshaled output")
			}
		})
	}
}

func TestTypedWrapper_GetGVK(t *testing.T) {
	wrapper := &gvk.TypedWrapper[TestSpec]{
		APIVersion: "test.io/v1",
		Kind:       "TestSpec",
	}

	gvk := wrapper.GetGVK()
	if gvk.Group != "test.io" {
		t.Errorf("expected group test.io, got %s", gvk.Group)
	}
	if gvk.Version != "v1" {
		t.Errorf("expected version v1, got %s", gvk.Version)
	}
	if gvk.Kind != "TestSpec" {
		t.Errorf("expected kind TestSpec, got %s", gvk.Kind)
	}
}

func TestTypedWrapper_GetName(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
		expected string
	}{
		{
			name: "with name",
			metadata: map[string]any{
				"name": "test-name",
			},
			expected: "test-name",
		},
		{
			name:     "without name",
			metadata: map[string]any{},
			expected: "",
		},
		{
			name: "name is not a string",
			metadata: map[string]any{
				"name": 123,
			},
			expected: "",
		},
		{
			name:     "nil metadata",
			metadata: nil,
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wrapper := &gvk.TypedWrapper[TestSpec]{
				Metadata: test.metadata,
			}
			result := wrapper.GetName()
			if result != test.expected {
				t.Errorf("expected %q, got %q", test.expected, result)
			}
		})
	}
}

func TestTypedWrapper_GetNamespace(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
		expected string
	}{
		{
			name: "with namespace",
			metadata: map[string]any{
				"namespace": "test-namespace",
			},
			expected: "test-namespace",
		},
		{
			name:     "without namespace",
			metadata: map[string]any{},
			expected: "",
		},
		{
			name: "namespace is not a string",
			metadata: map[string]any{
				"namespace": 456,
			},
			expected: "",
		},
		{
			name:     "nil metadata",
			metadata: nil,
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wrapper := &gvk.TypedWrapper[TestSpec]{
				Metadata: test.metadata,
			}
			result := wrapper.GetNamespace()
			if result != test.expected {
				t.Errorf("expected %q, got %q", test.expected, result)
			}
		})
	}
}

func TestTypedWrapper_SetName(t *testing.T) {
	tests := []struct {
		name         string
		initialMeta  map[string]any
		nameToSet    string
		expectedName string
	}{
		{
			name:         "set name on nil metadata",
			initialMeta:  nil,
			nameToSet:    "new-name",
			expectedName: "new-name",
		},
		{
			name:         "set name on empty metadata",
			initialMeta:  map[string]any{},
			nameToSet:    "new-name",
			expectedName: "new-name",
		},
		{
			name: "override existing name",
			initialMeta: map[string]any{
				"name": "old-name",
			},
			nameToSet:    "new-name",
			expectedName: "new-name",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wrapper := &gvk.TypedWrapper[TestSpec]{
				Metadata: test.initialMeta,
			}
			wrapper.SetName(test.nameToSet)

			if wrapper.Metadata == nil {
				t.Fatal("expected non-nil metadata after SetName")
			}

			result := wrapper.GetName()
			if result != test.expectedName {
				t.Errorf("expected name %q, got %q", test.expectedName, result)
			}
		})
	}
}

func TestTypedWrapper_SetNamespace(t *testing.T) {
	tests := []struct {
		name              string
		initialMeta       map[string]any
		namespaceToSet    string
		expectedNamespace string
	}{
		{
			name:              "set namespace on nil metadata",
			initialMeta:       nil,
			namespaceToSet:    "new-namespace",
			expectedNamespace: "new-namespace",
		},
		{
			name:              "set namespace on empty metadata",
			initialMeta:       map[string]any{},
			namespaceToSet:    "new-namespace",
			expectedNamespace: "new-namespace",
		},
		{
			name: "override existing namespace",
			initialMeta: map[string]any{
				"namespace": "old-namespace",
			},
			namespaceToSet:    "new-namespace",
			expectedNamespace: "new-namespace",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wrapper := &gvk.TypedWrapper[TestSpec]{
				Metadata: test.initialMeta,
			}
			wrapper.SetNamespace(test.namespaceToSet)

			if wrapper.Metadata == nil {
				t.Fatal("expected non-nil metadata after SetNamespace")
			}

			result := wrapper.GetNamespace()
			if result != test.expectedNamespace {
				t.Errorf("expected namespace %q, got %q", test.expectedNamespace, result)
			}
		})
	}
}

func TestTypedWrappers_UnmarshalYAML(t *testing.T) {
	// Test that TypedWrappers.UnmarshalYAML returns an error
	// since it requires registry context
	var wrappers gvk.TypedWrappers[TestSpec]
	yamlData := `
- apiVersion: test.io/v1
  kind: TestSpec
  metadata:
    name: test1
`
	err := yaml.Unmarshal([]byte(yamlData), &wrappers)
	if err == nil {
		t.Error("expected error for TypedWrappers unmarshal without registry")
	}
}

func TestTypedWrapper_MarshalYAML_EmptySpec(t *testing.T) {
	wrapper := &gvk.TypedWrapper[TestSpec]{
		APIVersion: "test.io/v1",
		Kind:       "TestSpec",
		Metadata: map[string]any{
			"name": "test",
		},
		Spec: TestSpec{}, // Zero value
	}

	data, err := yaml.Marshal(wrapper)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var result map[string]interface{}
	if err := yaml.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	// spec should still be included even if it's a zero value
	// (isZero check should handle this)
	if result["apiVersion"] != "test.io/v1" {
		t.Errorf("expected apiVersion test.io/v1, got %v", result["apiVersion"])
	}
}

func TestTypedWrapper_FullRoundTrip(t *testing.T) {
	registry := gvk.NewRegistry[TestSpec]()
	registry.Register(gvk.GVK{Group: "test.io", Version: "v1", Kind: "TestSpec"}, func() TestSpec {
		return TestSpec{}
	})

	original := &gvk.TypedWrapper[TestSpec]{
		APIVersion: "test.io/v1",
		Kind:       "TestSpec",
		Metadata: map[string]any{
			"name":      "test-resource",
			"namespace": "test-ns",
		},
		Spec: TestSpec{
			Field1: "value1",
			Field2: 42,
		},
	}

	// Marshal to YAML
	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal back
	parsed, err := gvk.ParseSingle(data, registry, nil)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	// Verify fields
	if parsed.APIVersion != original.APIVersion {
		t.Errorf("apiVersion mismatch: expected %s, got %s", original.APIVersion, parsed.APIVersion)
	}
	if parsed.Kind != original.Kind {
		t.Errorf("kind mismatch: expected %s, got %s", original.Kind, parsed.Kind)
	}
	if parsed.GetName() != original.GetName() {
		t.Errorf("name mismatch: expected %s, got %s", original.GetName(), parsed.GetName())
	}
	if parsed.GetNamespace() != original.GetNamespace() {
		t.Errorf("namespace mismatch: expected %s, got %s", original.GetNamespace(), parsed.GetNamespace())
	}
	if parsed.Spec.Field1 != original.Spec.Field1 {
		t.Errorf("field1 mismatch: expected %s, got %s", original.Spec.Field1, parsed.Spec.Field1)
	}
	if parsed.Spec.Field2 != original.Spec.Field2 {
		t.Errorf("field2 mismatch: expected %d, got %d", original.Spec.Field2, parsed.Spec.Field2)
	}
}
