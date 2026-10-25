package built

import (
	"maps"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Annotations returns obj's annotations as written. For an unstructured
// object that is its metadata.annotations map as it is: decoded from YAML, it
// may hold a boolean, a number or null, and then Unstructured.GetAnnotations
// returns none at all, so that setting annotations from it drops every one.
// For a typed object it is GetAnnotations. The map returned is a copy.
func Annotations(obj client.Object) map[string]any {
	if meta, ok := unstructuredMetadata(obj); ok {
		a, _ := meta["annotations"].(map[string]any)
		return maps.Clone(a)
	}
	a := obj.GetAnnotations()
	if a == nil {
		return nil
	}
	out := make(map[string]any, len(a))
	for k, v := range a {
		out[k] = v
	}
	return out
}

// AddAnnotations gives obj each annotation of want whose key it does not
// carry, and keeps every annotation it carries as written (Annotations). It
// returns how to put obj's annotations back as they were, or nil when obj
// already carries every key of want.
func AddAnnotations(obj client.Object, want map[string]string) func() {
	have := Annotations(obj)
	changed := false
	for k := range want {
		if _, ok := have[k]; !ok {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	meta, ok := unstructuredMetadata(obj)
	if !ok {
		orig := obj.GetAnnotations()
		merged := make(map[string]string, len(orig)+len(want))
		maps.Copy(merged, want)
		maps.Copy(merged, orig)
		obj.SetAnnotations(merged)
		return func() { obj.SetAnnotations(orig) }
	}
	u := obj.(*unstructured.Unstructured)
	_, hadMeta := u.Object["metadata"]
	orig, hadAnnotations := meta["annotations"]
	merged := make(map[string]any, len(have)+len(want))
	for k, v := range want {
		merged[k] = v
	}
	maps.Copy(merged, have)
	meta["annotations"] = merged
	if !hadMeta {
		u.Object["metadata"] = meta
	}
	return func() {
		switch {
		case !hadMeta:
			delete(u.Object, "metadata")
		case hadAnnotations:
			meta["annotations"] = orig
		default:
			delete(meta, "annotations")
		}
	}
}

// unstructuredMetadata returns the metadata map of an unstructured obj, a new
// one when it has none, and false for a typed obj or one whose metadata is no
// map.
func unstructuredMetadata(obj client.Object) (map[string]any, bool) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok || u == nil || u.Object == nil {
		return nil, false
	}
	raw, present := u.Object["metadata"]
	if !present {
		return map[string]any{}, true
	}
	meta, ok := raw.(map[string]any)
	return meta, ok
}
