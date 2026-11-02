package fluxcd

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

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
// objects a typed List in them holds, directly or through the typed Lists it
// holds (built.HeldItems). An unstructured List's items are in its object,
// so they are covered with it. Nil roots are skipped.
//
// It returns the function that puts every replaced Object map back, for a
// call that a later check refuses: a refused call leaves the tree as the
// caller gave it. An object whose Object the caller has set to another map
// since is left with that one.
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
			objs, err := callerObjects(r)
			if err != nil {
				// Name the object or field that is the cause, when an item is.
				for _, u := range objs {
					if jsonTyped(u.Object, 0) {
						continue
					}
					if _, convErr := jsonContentOf(u.Object); convErr != nil {
						return errors.Wrapf(convErr, "layout %q: %s %q: %s", l.FullRepoPath(), u.GetKind(), crKey(u.GetNamespace(), u.GetName()), unmarshalableField(u.Object))
					}
				}
				return errors.Wrapf(err, "layout %q: read list items", l.FullRepoPath())
			}
			for _, u := range objs {
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
			// Only the replacement is put back: a caller who has set another
			// map since keeps it.
			if sameMap(r.obj.Object, r.parsed) {
				r.obj.Object = r.held
			}
		}
	}, nil
}

// sameMap reports whether a and b are the same map, not two with equal content.
func sameMap(a, b map[string]any) bool {
	return a != nil && b != nil && reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

// callerObjects returns the unstructured objects r holds as a caller's own: r
// itself, or the unstructured items of a typed List r, which a read of the
// List returns as they are, not as copies.
func callerObjects(r client.Object) ([]*unstructured.Unstructured, error) {
	switch o := r.(type) {
	case nil:
		return nil, nil
	case *unstructured.Unstructured:
		if o == nil {
			return nil, nil
		}
		return []*unstructured.Unstructured{o}, nil
	}
	return built.HeldItems(r)
}

// jsonTyped reports whether v holds only the types runtime.DeepCopyJSONValue
// accepts: a string, an int64, a float64, a bool, nil, a json.Number, and a
// map[string]any or a []any of them, nested no deeper than maxJSONDepth and
// not holding themselves. A value that is not is not typed here, and is
// converted or refused (shapeProblem). It must not accept more: a value it
// calls typed is read as it is, and apimachinery panics on any other.
func jsonTyped(v any, depth int) bool {
	return jsonTypedOpen(v, depth, map[uintptr]bool{})
}

// jsonTypedOpen is jsonTyped with the containers being walked around v, each
// by the address of its content: one met again holds itself.
func jsonTypedOpen(v any, depth int, open map[uintptr]bool) bool {
	if depth > maxJSONDepth {
		return false
	}
	switch v := v.(type) {
	case string, int64, float64, bool, nil, json.Number:
		return true
	case map[string]any:
		if len(v) == 0 {
			return true
		}
		id := reflect.ValueOf(v).Pointer()
		if open[id] {
			return false
		}
		open[id] = true
		defer delete(open, id)
		for _, e := range v {
			if !jsonTypedOpen(e, depth+1, open) {
				return false
			}
		}
		return true
	case []any:
		if len(v) == 0 {
			return true
		}
		id := reflect.ValueOf(v).Pointer()
		if open[id] {
			return false
		}
		open[id] = true
		defer delete(open, id)
		return !slices.ContainsFunc(v, func(e any) bool { return !jsonTypedOpen(e, depth+1, open) })
	}
	return false
}

// jsonContentOf returns m as JSON decodes it: a number is an int64 when it is
// a whole number that fits one, else a float64. An integer above 2^53 keeps
// its exact value. It refuses what shapeProblem finds, before the encoder
// could follow it, and what decodes nested deeper than maxJSONDepth.
func jsonContentOf(m map[string]any) (map[string]any, error) {
	if _, problem := shapeProblem(m); problem != "" {
		return nil, errors.New(problem)
	}
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var content map[string]any
	if err := utiljson.Unmarshal(data, &content); err != nil {
		return nil, err
	}
	if nestedTooDeep(content, 0) {
		return nil, errors.Errorf("decodes nested deeper than %d levels", maxJSONDepth)
	}
	return content, nil
}

// nestedTooDeep reports whether v holds a map[string]any or a []any nested
// deeper than maxJSONDepth. It is for a decoded value, which holds no other
// type and no cycle.
func nestedTooDeep(v any, depth int) bool {
	if depth > maxJSONDepth {
		return true
	}
	switch v := v.(type) {
	case map[string]any:
		for _, e := range v {
			if nestedTooDeep(e, depth+1) {
				return true
			}
		}
	case []any:
		return slices.ContainsFunc(v, func(e any) bool { return nestedTooDeep(e, depth+1) })
	}
	return false
}

// shapeProblem looks for what the JSON encoder must not be given: a value
// nested deeper than maxJSONDepth or one that holds itself. It follows every
// map, slice, array, pointer and struct field, named types included, and the
// Object of an *unstructured.Unstructured, whose MarshalJSON starts an
// encoder of its own. It returns the path of the container where the problem
// is and the problem, or "" for none.
func shapeProblem(m map[string]any) (path, problem string) {
	return walkShape(reflect.ValueOf(m), "", 0, map[uintptr]bool{})
}

func walkShape(v reflect.Value, path string, depth int, open map[uintptr]bool) (string, string) {
	deeper := fmt.Sprintf("holds a value nested deeper than %d levels", maxJSONDepth)
	switch v.Kind() {
	case reflect.Interface:
		if v.IsNil() {
			return "", ""
		}
		return walkShape(v.Elem(), path, depth, open)
	case reflect.Pointer:
		if v.IsNil() {
			return "", ""
		}
		id := v.Pointer()
		if open[id] {
			return path, "holds itself"
		}
		open[id] = true
		defer delete(open, id)
		if u, ok := v.Interface().(*unstructured.Unstructured); ok {
			return walkShape(reflect.ValueOf(u.Object), path, depth, open)
		}
		return walkShape(v.Elem(), path, depth, open)
	case reflect.Map, reflect.Slice:
		if v.Len() == 0 {
			return "", ""
		}
		if depth > maxJSONDepth {
			return path, deeper
		}
		id := v.Pointer()
		if open[id] {
			return path, "holds itself"
		}
		open[id] = true
		defer delete(open, id)
		if v.Kind() == reflect.Slice {
			return walkElems(v, path, depth, open)
		}
		keys := v.MapKeys()
		slices.SortFunc(keys, func(a, b reflect.Value) int { return strings.Compare(fmt.Sprint(a), fmt.Sprint(b)) })
		for _, k := range keys {
			p := fmt.Sprint(k)
			if path != "" {
				p = path + "." + p
			}
			if at, problem := walkShape(v.MapIndex(k), p, depth+1, open); problem != "" {
				return at, problem
			}
		}
	case reflect.Array:
		return walkElems(v, path, depth, open)
	case reflect.Struct:
		for i := range v.NumField() {
			if !v.Type().Field(i).IsExported() {
				continue
			}
			if at, problem := walkShape(v.Field(i), path, depth, open); problem != "" {
				return at, problem
			}
		}
	}
	return "", ""
}

func walkElems(v reflect.Value, path string, depth int, open map[uintptr]bool) (string, string) {
	if depth > maxJSONDepth {
		return path, fmt.Sprintf("holds a value nested deeper than %d levels", maxJSONDepth)
	}
	for i := range v.Len() {
		if at, problem := walkShape(v.Index(i), fmt.Sprintf("%s[%d]", path, i), depth+1, open); problem != "" {
			return at, problem
		}
	}
	return "", ""
}

// unmarshalableField describes why m cannot be converted: the container that
// holds itself or is nested too deep, else the first field of m, in key order,
// whose value json cannot marshal, as "spec.ref.bad holds a chan int, which
// JSON cannot hold". Another cause, one that names no field, gives "holds a
// value that JSON cannot hold".
func unmarshalableField(m map[string]any) string {
	if path, problem := shapeProblem(m); problem != "" {
		if path == "" {
			path = "the object"
		}
		return fmt.Sprintf("%s %s", path, problem)
	}
	if path, v, ok := firstUnmarshalable(m, ""); ok {
		return fmt.Sprintf("%s holds a %T, which JSON cannot hold", path, v)
	}
	return "holds a value that JSON cannot hold"
}

// firstUnmarshalable is for a value shapeProblem found none in.
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
