package patch

import (
	"encoding/json"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/strategicpatch"

	"github.com/go-kure/kure/pkg/errors"
)

// StrategicPatch represents a partial YAML document for deep merge
// using Kubernetes strategic merge patch semantics.
type StrategicPatch struct {
	Patch map[string]interface{}
}

// ApplyStrategicMergePatch applies a strategic merge patch to a resource.
// For known Kubernetes kinds (registered in the lookup scheme), it uses
// StrategicMergeMapPatch with typed struct tags to enable list-merge-by-key
// semantics (e.g. containers merged by name).
// For unknown kinds (CRDs), it falls back to RFC 7386 JSON merge patch.
// If lookup is nil, fallback is always used.
func ApplyStrategicMergePatch(
	resource *unstructured.Unstructured,
	patch map[string]interface{},
	lookup KindLookup,
) error {
	gvk := resource.GroupVersionKind()
	patchCopy := deepCopyMap(patch)

	if lookup != nil {
		typedObj, ok := lookup.LookupKind(gvk)
		if ok {
			return applyTypedSMP(resource, patchCopy, typedObj)
		}
	}

	return applyJSONMergePatch(resource, patchCopy)
}

// applyTypedSMP applies a strategic merge patch using Go struct tags from the
// typed object. StrategicMergeMapPatch mutates its arguments, so we deep-copy
// the resource map before passing it in. On error the resource is left untouched.
func applyTypedSMP(resource *unstructured.Unstructured, patch map[string]interface{}, typedObj interface{}) error {
	original := deepCopyMap(resource.Object)

	merged, err := strategicpatch.StrategicMergeMapPatch(original, patch, typedObj)
	if err != nil {
		return errors.Wrap(err, "strategic merge patch failed")
	}

	resource.Object = merged
	return nil
}

// applyJSONMergePatch applies RFC 7386 JSON merge patch as a fallback for
// CRDs that lack Go struct tags.
func applyJSONMergePatch(resource *unstructured.Unstructured, patch map[string]interface{}) error {
	originalJSON, err := json.Marshal(resource.Object)
	if err != nil {
		return errors.Wrap(err, "failed to marshal resource for JSON merge patch")
	}

	patchJSON, err := json.Marshal(patch)
	if err != nil {
		return errors.Wrap(err, "failed to marshal patch for JSON merge patch")
	}

	mergedJSON, err := jsonpatch.MergePatch(originalJSON, patchJSON)
	if err != nil {
		return errors.Wrap(err, "JSON merge patch failed")
	}

	var merged map[string]interface{}
	if err := json.Unmarshal(mergedJSON, &merged); err != nil {
		return errors.Wrap(err, "failed to unmarshal merged result")
	}

	resource.Object = merged
	return nil
}

// deepCopyMap creates a deep copy of a map[string]interface{} to prevent
// StrategicMergeMapPatch from mutating the original patch data.
func deepCopyMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = deepCopyValue(v)
	}
	return result
}

func deepCopyValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		return deepCopyMap(val)
	case []interface{}:
		copied := make([]interface{}, len(val))
		for i, item := range val {
			copied[i] = deepCopyValue(item)
		}
		return copied
	default:
		return v
	}
}
