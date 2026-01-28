package launcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDeepCopyResources(t *testing.T) {
	t.Run("nil slice", func(t *testing.T) {
		result := deepCopyResources(nil)
		assert.Nil(t, result)
	})

	t.Run("empty slice", func(t *testing.T) {
		result := deepCopyResources([]Resource{})
		assert.NotNil(t, result)
		assert.Len(t, result, 0)
	})

	t.Run("with resources", func(t *testing.T) {
		original := []Resource{
			{APIVersion: "v1", Kind: "Pod"},
			{APIVersion: "apps/v1", Kind: "Deployment"},
		}

		result := deepCopyResources(original)
		assert.Len(t, result, 2)
		assert.Equal(t, "v1", result[0].APIVersion)
		assert.Equal(t, "apps/v1", result[1].APIVersion)

		// Verify independence
		result[0].APIVersion = "v2"
		assert.Equal(t, "v1", original[0].APIVersion)
	})
}

func TestDeepCopyPatches(t *testing.T) {
	t.Run("nil slice", func(t *testing.T) {
		result := deepCopyPatches(nil)
		assert.Nil(t, result)
	})

	t.Run("empty slice", func(t *testing.T) {
		result := deepCopyPatches([]Patch{})
		assert.NotNil(t, result)
		assert.Len(t, result, 0)
	})

	t.Run("with patches", func(t *testing.T) {
		original := []Patch{
			{
				Name:    "patch1",
				Path:    "/path/to/patch1",
				Content: "content1",
				Metadata: &PatchMetadata{
					Description: "test patch",
					Enabled:     "true",
					Requires:    []string{"base"},
				},
			},
		}

		result := deepCopyPatches(original)
		assert.Len(t, result, 1)
		assert.Equal(t, "patch1", result[0].Name)
		assert.Equal(t, "content1", result[0].Content)
		assert.NotNil(t, result[0].Metadata)
		assert.Equal(t, "test patch", result[0].Metadata.Description)

		// Verify independence
		result[0].Name = "modified"
		result[0].Metadata.Description = "modified"
		assert.Equal(t, "patch1", original[0].Name)
		assert.Equal(t, "test patch", original[0].Metadata.Description)
	})

	t.Run("with nil metadata", func(t *testing.T) {
		original := []Patch{
			{Name: "patch1", Path: "/path", Content: "content", Metadata: nil},
		}

		result := deepCopyPatches(original)
		assert.Len(t, result, 1)
		assert.Nil(t, result[0].Metadata)
	})
}

func TestDeepCopyParameterMapWithSource(t *testing.T) {
	t.Run("nil map", func(t *testing.T) {
		result := deepCopyParameterMapWithSource(nil)
		assert.Nil(t, result)
	})

	t.Run("empty map", func(t *testing.T) {
		original := make(ParameterMapWithSource)
		result := deepCopyParameterMapWithSource(original)
		assert.NotNil(t, result)
		assert.Len(t, result, 0)
	})

	t.Run("with values", func(t *testing.T) {
		original := ParameterMapWithSource{
			"key1": {Value: "value1", Location: "loc1", File: "file1", Line: 1},
			"key2": {Value: map[string]interface{}{"nested": "value"}, Location: "loc2", File: "file2", Line: 2},
		}

		result := deepCopyParameterMapWithSource(original)
		assert.Len(t, result, 2)
		assert.Equal(t, "value1", result["key1"].Value)
		assert.Equal(t, "loc1", result["key1"].Location)

		// Verify independence
		result["key1"] = ParameterSource{Value: "modified"}
		assert.Equal(t, "value1", original["key1"].Value)
	})
}

func TestDeepCopyUnstructured(t *testing.T) {
	t.Run("nil unstructured", func(t *testing.T) {
		result := deepCopyUnstructured(nil)
		assert.Nil(t, result)
	})

	t.Run("with unstructured", func(t *testing.T) {
		original := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"name": "test-pod",
				},
			},
		}

		result := deepCopyUnstructured(original)
		assert.NotNil(t, result)
		assert.Equal(t, "v1", result.Object["apiVersion"])

		// Verify independence
		result.Object["apiVersion"] = "v2"
		assert.Equal(t, "v1", original.Object["apiVersion"])
	})
}

func TestDeepCopyStringSlice(t *testing.T) {
	t.Run("nil slice", func(t *testing.T) {
		result := deepCopyStringSlice(nil)
		assert.Nil(t, result)
	})

	t.Run("empty slice", func(t *testing.T) {
		result := deepCopyStringSlice([]string{})
		assert.NotNil(t, result)
		assert.Len(t, result, 0)
	})

	t.Run("with strings", func(t *testing.T) {
		original := []string{"a", "b", "c"}
		result := deepCopyStringSlice(original)
		assert.Equal(t, original, result)

		// Verify independence
		result[0] = "modified"
		assert.Equal(t, "a", original[0])
	})
}

func TestDeepCopyInterfaceSlice(t *testing.T) {
	t.Run("nil slice", func(t *testing.T) {
		result := deepCopyInterfaceSlice(nil)
		assert.Nil(t, result)
	})

	t.Run("empty slice", func(t *testing.T) {
		result := deepCopyInterfaceSlice([]interface{}{})
		assert.NotNil(t, result)
		assert.Len(t, result, 0)
	})

	t.Run("with values", func(t *testing.T) {
		original := []interface{}{
			"string",
			42,
			true,
			map[string]interface{}{"nested": "value"},
			[]interface{}{"nested", "slice"},
		}

		result := deepCopyInterfaceSlice(original)
		assert.Len(t, result, 5)
		assert.Equal(t, "string", result[0])
		assert.Equal(t, 42, result[1])
		assert.Equal(t, true, result[2])

		// Verify nested map independence
		nestedMap := result[3].(map[string]interface{})
		nestedMap["nested"] = "modified"
		originalMap := original[3].(map[string]interface{})
		assert.Equal(t, "value", originalMap["nested"])
	})
}

func TestDeepCopyMap(t *testing.T) {
	t.Run("nil map", func(t *testing.T) {
		result := deepCopyMap(nil)
		assert.Nil(t, result)
	})

	t.Run("empty map", func(t *testing.T) {
		result := deepCopyMap(map[string]interface{}{})
		assert.NotNil(t, result)
		assert.Len(t, result, 0)
	})

	t.Run("with values", func(t *testing.T) {
		original := map[string]interface{}{
			"string": "value",
			"int":    42,
			"bool":   true,
			"nested": map[string]interface{}{
				"deep": "value",
			},
			"slice": []interface{}{"a", "b"},
		}

		result := deepCopyMap(original)
		assert.Len(t, result, 5)
		assert.Equal(t, "value", result["string"])
		assert.Equal(t, 42, result["int"])

		// Verify nested map independence
		nestedResult := result["nested"].(map[string]interface{})
		nestedResult["deep"] = "modified"
		nestedOriginal := original["nested"].(map[string]interface{})
		assert.Equal(t, "value", nestedOriginal["deep"])
	})
}
