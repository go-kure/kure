package fluxcd

import (
	"encoding/json"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utiljson "k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/internal/built"
	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// maxJSONDepth is how deep a value is followed: encoding/json refuses a value
// nested deeper than 1000 levels as a cycle, and so does kure, before the
// recursion could overflow the stack.
const maxJSONDepth = 1000

// normalizeCallerObjects makes every unstructured object a caller put in the
// layouts of roots, and of the layouts below them, one kure can read
// (go-kure/kure#1201). apimachinery requires such an object to hold JSON
// typed values only, and panics in every Nested* read and in DeepCopy on any
// other, a plain int say. An object that holds only the types
// runtime.DeepCopyJSONValue accepts is left as it is. Another is replaced
// by what its JSON decodes to, an int64 or a float64 for a number: its Object
// field is set to a new map, and the caller's map is not changed. A value that
// does not marshal, or is nested deeper than maxJSONDepth, refuses the call,
// with the layout, the object and the field named, and no object is replaced
// then.
//
// The objects are those in each layout's Resources, and the unstructured
// items a typed List in them holds (built.HeldItems). An unstructured List's
// items are in its object, so they are covered with it. Nil roots are skipped.
//
// It returns the function that puts every replaced Object map back, for a
// call that a later check refuses: a refused call leaves the tree as the
// caller gave it.
func normalizeCallerObjects(roots ...*layout.ManifestLayout) (undo func(), err error) {
	type replacement struct {
		obj          *unstructured.Unstructured
		held, parsed map[string]any
	}
	var replace []replacement
	seenLayout := map[*layout.ManifestLayout]bool{}
	seenObject := map[*unstructured.Unstructured]bool{}
	var walk func(l *layout.ManifestLayout) error
	walk = func(l *layout.ManifestLayout) error {
		if l == nil || seenLayout[l] {
			return nil
		}
		seenLayout[l] = true
		for _, r := range l.Resources {
			for _, u := range callerObjects(r) {
				if seenObject[u] {
					continue
				}
				seenObject[u] = true
				if jsonTyped(u.Object, 0) {
					continue
				}
				content, err := jsonContentOf(u.Object)
				if err != nil {
					return errors.Wrapf(err, "layout %q: %s %q: %s", l.FullRepoPath(), u.GetKind(), crKey(u.GetNamespace(), u.GetName()), unmarshalableField(u.Object))
				}
				replace = append(replace, replacement{u, u.Object, content})
			}
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
			return func() {}, err
		}
	}
	for _, r := range replace {
		r.obj.Object = r.parsed
	}
	return func() {
		for _, r := range replace {
			r.obj.Object = r.held
		}
	}, nil
}

// callerObjects returns the unstructured objects r holds as a caller's own: r
// itself, or the unstructured items of a typed List r, which a read of the
// List returns as they are, not as copies.
func callerObjects(r client.Object) []*unstructured.Unstructured {
	switch o := r.(type) {
	case nil:
		return nil
	case *unstructured.Unstructured:
		if o == nil {
			return nil
		}
		return []*unstructured.Unstructured{o}
	}
	if !meta.IsListType(r) {
		return nil
	}
	return built.HeldItems(r)
}

// jsonTyped reports whether v holds only the types runtime.DeepCopyJSONValue
// accepts: a string, an int64, a float64, a bool, nil, a json.Number, and a
// map[string]any or a []any of them, nested no deeper than maxJSONDepth. It
// must not accept more: a value it calls typed is read as it is, and
// apimachinery panics on any other.
func jsonTyped(v any, depth int) bool {
	if depth > maxJSONDepth {
		return false
	}
	switch v := v.(type) {
	case string, int64, float64, bool, nil, json.Number:
		return true
	case map[string]any:
		for _, e := range v {
			if !jsonTyped(e, depth+1) {
				return false
			}
		}
		return true
	case []any:
		return !slices.ContainsFunc(v, func(e any) bool { return !jsonTyped(e, depth+1) })
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

// tooDeep is firstUnmarshalable's answer for a value nested deeper than
// maxJSONDepth, which is a cycle or as good as one.
type tooDeep struct{}

// unmarshalableField describes the first field of m, in key order, whose value
// json cannot marshal, as "spec.ref.bad holds a chan int, which JSON cannot
// hold". Another cause, one that names no field, gives "holds a value that
// JSON cannot hold".
func unmarshalableField(m map[string]any) string {
	path, v, ok := firstUnmarshalable(m, "", 0)
	switch {
	case !ok:
		return "holds a value that JSON cannot hold"
	case v == tooDeep{}:
		return fmt.Sprintf("holds a value nested deeper than %d levels, or one that contains itself", maxJSONDepth)
	}
	return fmt.Sprintf("%s holds a %T, which JSON cannot hold", path, v)
}

func firstUnmarshalable(v any, path string, depth int) (string, any, bool) {
	if depth > maxJSONDepth {
		return "", tooDeep{}, true
	}
	switch x := v.(type) {
	case map[string]any:
		for _, k := range slices.Sorted(mapKeys(x)) {
			p := k
			if path != "" {
				p = path + "." + k
			}
			if at, bad, ok := firstUnmarshalable(x[k], p, depth+1); ok {
				return at, bad, true
			}
		}
		return "", nil, false
	case []any:
		for i, e := range x {
			if at, bad, ok := firstUnmarshalable(e, fmt.Sprintf("%s[%d]", path, i), depth+1); ok {
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
