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
// For a typed object it is GetAnnotations. The map returned is a copy. Text
// reads its values as a kustomize build does.
func Annotations(obj client.Object) map[string]any {
	return writtenMetadataMap(obj, "annotations", obj.GetAnnotations)
}

// Labels returns obj's labels as written, as Annotations returns its
// annotations: for an unstructured object its metadata.labels map as it is,
// for which Unstructured.GetLabels returns none at all once it holds a value
// that is no string, and for a typed object GetLabels. The map returned is a
// copy. Text reads its values as a kustomize build does.
func Labels(obj client.Object) map[string]any {
	return writtenMetadataMap(obj, "labels", obj.GetLabels)
}

// writtenMetadataMap returns the map field of obj's metadata as written: of an
// unstructured obj the map as it is, of a typed one typed's.
func writtenMetadataMap(obj client.Object, field string, typed func() map[string]string) map[string]any {
	if meta, ok := unstructuredMetadata(obj); ok {
		m, _ := meta[field].(map[string]any)
		return maps.Clone(m)
	}
	m := typed()
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// AddAnnotations gives obj each annotation of want whose key it does not
// carry, and keeps every annotation it carries as written (Annotations). It
// returns how to put obj back exactly as it was, or nil when it changed
// nothing: when obj already carries every key of want, or when obj is
// unstructured and its metadata is present but no map, which holds no
// annotations and takes none.
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
	u, ok := obj.(*unstructured.Unstructured)
	if !ok || u == nil {
		orig := obj.GetAnnotations()
		merged := make(map[string]string, len(orig)+len(want))
		maps.Copy(merged, want)
		maps.Copy(merged, orig)
		obj.SetAnnotations(merged)
		return func() { obj.SetAnnotations(orig) }
	}
	hadObject := u.Object != nil
	raw, hadMeta := u.Object["metadata"]
	meta, isMap := raw.(map[string]any)
	if hadMeta && !isMap {
		return nil
	}
	if !hadObject {
		u.Object = map[string]any{}
	}
	if !hadMeta {
		meta = map[string]any{}
		u.Object["metadata"] = meta
	}
	orig, hadAnnotations := meta["annotations"]
	merged := make(map[string]any, len(have)+len(want))
	for k, v := range want {
		merged[k] = v
	}
	maps.Copy(merged, have)
	meta["annotations"] = merged
	return func() {
		switch {
		case !hadObject:
			u.Object = nil
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
