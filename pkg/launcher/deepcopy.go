package launcher

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

// deepCopyResources creates a deep copy of a resource slice
func deepCopyResources(resources []Resource) []Resource {
	if resources == nil {
		return nil
	}

	result := make([]Resource, len(resources))
	for i, r := range resources {
		result[i] = r.DeepCopy()
	}
	return result
}

// deepCopyPatches creates a deep copy of a patch slice
func deepCopyPatches(patches []Patch) []Patch {
	if patches == nil {
		return nil
	}

	result := make([]Patch, len(patches))
	for i, p := range patches {
		result[i] = Patch{
			Name:     p.Name,
			Path:     p.Path,
			Content:  p.Content,
			Metadata: deepCopyPatchMetadata(p.Metadata),
		}
	}
	return result
}

// deepCopyParameterMapWithSource creates a deep copy of a parameter map with sources
func deepCopyParameterMapWithSource(m ParameterMapWithSource) ParameterMapWithSource {
	if m == nil {
		return nil
	}

	result := make(ParameterMapWithSource)
	for k, v := range m {
		result[k] = ParameterSource{
			Value:    deepCopyValue(v.Value),
			Location: v.Location,
			File:     v.File,
			Line:     v.Line,
		}
	}
	return result
}

// deepCopyUnstructured creates a deep copy of an unstructured object
func deepCopyUnstructured(u *unstructured.Unstructured) *unstructured.Unstructured {
	if u == nil {
		return nil
	}
	return u.DeepCopy()
}

// deepCopyStringSlice creates a deep copy of a string slice
func deepCopyStringSlice(s []string) []string {
	if s == nil {
		return nil
	}

	result := make([]string, len(s))
	copy(result, s)
	return result
}

// deepCopyInterfaceSlice creates a deep copy of an interface slice
func deepCopyInterfaceSlice(s []interface{}) []interface{} {
	if s == nil {
		return nil
	}

	result := make([]interface{}, len(s))
	for i, v := range s {
		result[i] = deepCopyValue(v)
	}
	return result
}

// deepCopyMap creates a deep copy of a map[string]interface{}
func deepCopyMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}

	result := make(map[string]interface{})
	for k, v := range m {
		result[k] = deepCopyValue(v)
	}
	return result
}
