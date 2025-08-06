package launcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestResourceDeepCopy(t *testing.T) {
	// Create a resource
	original := Resource{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Metadata: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
		},
		Raw: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "test-app",
					"namespace": "default",
				},
			},
		},
	}
	
	// Create a deep copy
	copied := original.DeepCopy()
	
	// Verify the copy is equal but not the same object
	assert.Equal(t, original.APIVersion, copied.APIVersion)
	assert.Equal(t, original.Kind, copied.Kind)
	assert.Equal(t, original.Metadata.Name, copied.Metadata.Name)
	assert.Equal(t, original.Metadata.Namespace, copied.Metadata.Namespace)
	
	// Modify the copy
	copied.Metadata.Name = "modified"
	copied.Metadata.Labels["new"] = "label"
	
	// Verify original is unchanged
	assert.Equal(t, "test-app", original.Metadata.Name)
	assert.NotContains(t, original.Metadata.Labels, "new")
}

func TestPackageDefinitionDeepCopy(t *testing.T) {
	// Create a package definition
	original := &PackageDefinition{
		Path: "/test/package",
		Metadata: KurelMetadata{
			Name:       "test-package",
			Version:    "1.0.0",
			AppVersion: "2.0.0",
		},
		Parameters: ParameterMap{
			"app": "test",
			"nested": map[string]interface{}{
				"key": "value",
			},
		},
		Resources: []Resource{
			{
				APIVersion: "v1",
				Kind:       "Service",
				Metadata: metav1.ObjectMeta{
					Name: "test-service",
				},
			},
		},
		Patches: []Patch{
			{
				Name:    "test-patch",
				Path:    "patches/test.kpatch",
				Content: "[deployment.test]\nspec.replicas: 3",
				Metadata: &PatchMetadata{
					Enabled:     "${feature.enabled}",
					Description: "Test patch",
					Requires:    []string{"base-patch"},
				},
			},
		},
	}
	
	// Create a deep copy
	copied := original.DeepCopy()
	
	// Verify the copy is equal but independent
	assert.Equal(t, original.Path, copied.Path)
	assert.Equal(t, original.Metadata.Name, copied.Metadata.Name)
	assert.Equal(t, len(original.Resources), len(copied.Resources))
	assert.Equal(t, len(original.Patches), len(copied.Patches))
	
	// Modify the copy
	copied.Metadata.Name = "modified"
	copied.Parameters["new"] = "param"
	copied.Resources[0].Metadata.Name = "modified-service"
	copied.Patches[0].Metadata.Requires = append(copied.Patches[0].Metadata.Requires, "new-req")
	
	// Verify original is unchanged
	assert.Equal(t, "test-package", original.Metadata.Name)
	assert.NotContains(t, original.Parameters, "new")
	assert.Equal(t, "test-service", original.Resources[0].Metadata.Name)
	assert.Len(t, original.Patches[0].Metadata.Requires, 1)
}

func TestParameterMapDeepCopy(t *testing.T) {
	// Create a complex parameter map
	original := ParameterMap{
		"string": "value",
		"number": 42,
		"bool":   true,
		"array":  []interface{}{"a", "b", "c"},
		"nested": map[string]interface{}{
			"key1": "value1",
			"key2": map[string]interface{}{
				"deep": "value",
			},
		},
	}
	
	// Deep copy
	copied := deepCopyParameterMap(original)
	
	// Verify equality
	assert.Equal(t, original, copied)
	
	// Modify nested values in copy
	copied["string"] = "modified"
	copied["array"].([]interface{})[0] = "modified"
	copied["nested"].(map[string]interface{})["key1"] = "modified"
	
	// Verify original is unchanged
	assert.Equal(t, "value", original["string"])
	assert.Equal(t, "a", original["array"].([]interface{})[0])
	assert.Equal(t, "value1", original["nested"].(map[string]interface{})["key1"])
}

func TestValidationResult(t *testing.T) {
	// Test empty result
	result := ValidationResult{}
	assert.True(t, result.IsValid())
	assert.False(t, result.HasErrors())
	assert.False(t, result.HasWarnings())
	
	// Test with errors
	result.Errors = []ValidationError{
		{Message: "test error"},
	}
	assert.False(t, result.IsValid())
	assert.True(t, result.HasErrors())
	
	// Test with warnings
	result.Warnings = []ValidationWarning{
		{Message: "test warning"},
	}
	assert.True(t, result.HasWarnings())
}

func TestValidationError(t *testing.T) {
	tests := []struct {
		name     string
		err      ValidationError
		expected string
	}{
		{
			name:     "message only",
			err:      ValidationError{Message: "test error"},
			expected: "test error",
		},
		{
			name:     "with resource",
			err:      ValidationError{Resource: "deployment", Message: "invalid"},
			expected: "deployment: invalid",
		},
		{
			name:     "with field",
			err:      ValidationError{Field: "spec.replicas", Message: "must be positive"},
			expected: "spec.replicas: must be positive",
		},
		{
			name:     "with resource and field",
			err:      ValidationError{Resource: "deployment", Field: "spec.replicas", Message: "must be positive"},
			expected: "deployment.spec.replicas: must be positive",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestResourceGetters(t *testing.T) {
	r := Resource{
		Metadata: metav1.ObjectMeta{
			Name:      "test-resource",
			Namespace: "test-namespace",
		},
	}
	
	assert.Equal(t, "test-resource", r.GetName())
	assert.Equal(t, "test-namespace", r.GetNamespace())
}

func TestResourceToUnstructured(t *testing.T) {
	// Test with nil Raw
	r := Resource{}
	u, err := r.ToUnstructured()
	require.NoError(t, err)
	assert.Nil(t, u)
	
	// Test with Raw
	r.Raw = &unstructured.Unstructured{
		Object: map[string]interface{}{
			"test": "value",
		},
	}
	
	u, err = r.ToUnstructured()
	require.NoError(t, err)
	require.NotNil(t, u)
	
	// Verify it's a copy
	u.Object["modified"] = true
	assert.NotContains(t, r.Raw.Object, "modified")
}