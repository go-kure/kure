package fluxcd

import (
	"encoding/json"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utiljson "k8s.io/apimachinery/pkg/util/json"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// normalizeCallerObjects makes every unstructured object a caller put in the
// layouts of ml, and of the layouts below it, one kure can read
// (go-kure/kure#1201). apimachinery requires such an object to hold JSON
// typed values only, and panics in every Nested* read and in DeepCopy on any
// other, a plain int say. An object that holds only the types
// runtime.DeepCopyJSONValue accepts is left as it is. Another is replaced
// by what its JSON decodes to, an int64 or a float64 for a number: its Object
// field is set to a new map, and the caller's map is not changed. A value that
// does not marshal refuses the call, with the layout, the object and the field
// named, and no object is replaced then: the tree is left as the caller gave it.
//
// The objects are those in each layout's Resources. A List's items are in its
// object, so they are covered with it. Nil roots are skipped, and the refusal
// of one root replaces no object of any.
func normalizeCallerObjects(roots ...*layout.ManifestLayout) error {
	type replacement struct {
		obj     *unstructured.Unstructured
		content map[string]any
	}
	var replace []replacement
	seen := map[*layout.ManifestLayout]bool{}
	var walk func(l *layout.ManifestLayout) error
	walk = func(l *layout.ManifestLayout) error {
		if l == nil || seen[l] {
			return nil
		}
		seen[l] = true
		for _, r := range l.Resources {
			u, ok := r.(*unstructured.Unstructured)
			if !ok || u == nil || jsonTyped(u.Object) {
				continue
			}
			content, err := jsonContentOf(u.Object)
			if err != nil {
				return errors.Wrapf(err, "layout %q: %s %q: %s", l.FullRepoPath(), u.GetKind(), crKey(u.GetNamespace(), u.GetName()), unmarshalableField(u.Object))
			}
			replace = append(replace, replacement{u, content})
		}
		for _, c := range l.Children {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	for _, root := range roots {
		if err := walk(root); err != nil {
			return err
		}
	}
	for _, r := range replace {
		r.obj.Object = r.content
	}
	return nil
}

// jsonTyped reports whether v holds only the types runtime.DeepCopyJSONValue
// accepts: a string, an int64, a float64, a bool, nil, a json.Number, and a
// map[string]any or a []any of them. It must not accept more: a value it
// calls typed is read as it is, and apimachinery panics on any other.
func jsonTyped(v any) bool {
	switch v := v.(type) {
	case string, int64, float64, bool, nil, json.Number:
		return true
	case map[string]any:
		for _, e := range v {
			if !jsonTyped(e) {
				return false
			}
		}
		return true
	case []any:
		return !slices.ContainsFunc(v, func(e any) bool { return !jsonTyped(e) })
	}
	return false
}

// jsonContentOf returns m as JSON decodes it: a number is an int64 when it is
// a whole number that fits one, else a float64. An integer above 2^53 keeps
// its exact value.
func jsonContentOf(m map[string]any) (map[string]any, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var content map[string]any
	if err := utiljson.Unmarshal(data, &content); err != nil {
		return nil, err
	}
	return content, nil
}

// unmarshalableField describes the first field of m, in key order, whose value
// json cannot marshal, as "spec.ref.bad holds a chan int". Another cause, one
// that names no field, gives "holds a value json cannot marshal".
func unmarshalableField(m map[string]any) string {
	if path, v, ok := firstUnmarshalable(m, ""); ok {
		return fmt.Sprintf("%s holds a %T, which JSON cannot hold", path, v)
	}
	return "holds a value that JSON cannot hold"
}

func firstUnmarshalable(v any, path string) (string, any, bool) {
	switch x := v.(type) {
	case map[string]any:
		for _, k := range slices.Sorted(mapKeys(x)) {
			p := k
			if path != "" {
				p = path + "." + k
			}
			if at, bad, ok := firstUnmarshalable(x[k], p); ok {
				return at, bad, true
			}
		}
		return "", nil, false
	case []any:
		for i, e := range x {
			if at, bad, ok := firstUnmarshalable(e, fmt.Sprintf("%s[%d]", path, i)); ok {
				return at, bad, true
			}
		}
		return "", nil, false
	}
	if _, err := json.Marshal(v); err != nil {
		return path, v, true
	}
	return "", nil, false
}

func mapKeys(m map[string]any) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}
