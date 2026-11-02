package fluxcd

import (
	"encoding"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"slices"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/internal/built"
	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack/layout"
)

const (
	// maxJSONDepth is how deep a value is followed, as encoding/json counts
	// a nesting before it takes it for a cycle.
	maxJSONDepth = 1000

	// maxJSONNodes is how many values are visited in one object. An object
	// is far smaller (the API server stores 1.5 MiB at most); a larger count
	// is a value that shares its parts so often that its tree is huge.
	maxJSONNodes = 1 << 22
)

// normalizeCallerObjects makes every unstructured object a caller put in the
// layouts of roots, and of the layouts below them, one kure can read
// (go-kure/kure#1201). apimachinery requires such an object to hold JSON
// typed values only, and panics in every Nested* read and in DeepCopy on any
// other, a plain int say. An object that holds only the types
// runtime.DeepCopyJSONValue accepts is left as it is. Another is replaced by
// a copy in which each value of a shape listed at jsonContentOf is its JSON
// typed counterpart. Its Object field is set to the new map, and the caller's
// map is not changed.
//
// A value of any other shape refuses the call, with the layout, the object
// and the field named, and no object is replaced then. So does a value that
// holds itself, one nested deeper than maxJSONDepth and one with more than
// maxJSONNodes values. Nothing the caller built is ever marshalled or
// asked to marshal itself: a marshaler could not be stopped on a value that
// holds itself.
//
// The objects are those in each layout's Resources, and the unstructured
// objects a typed List in them holds, directly or through the typed Lists it
// holds (built.HeldItems). An unstructured List's items are in its object,
// so they are covered with it. Nil roots are skipped.
//
// It returns the function that puts every replaced Object map back, for a
// call that a later check refuses: a refused call leaves the tree as the
// caller gave it. The caller does not change the tree during the call: an
// object whose Object the caller has set to another map since is left with
// that one, but a change inside the replacement is put back with it.
func normalizeCallerObjects(roots ...*layout.ManifestLayout) (undo func(), err error) {
	type replacement struct {
		obj          *unstructured.Unstructured
		held, parsed map[string]any
	}
	var replace []replacement
	seenLayout := map[*layout.ManifestLayout]bool{}
	seenObject := map[*unstructured.Unstructured]bool{}
	refuse := func(l *layout.ManifestLayout, u *unstructured.Unstructured, cause error) error {
		return errors.Wrapf(cause, "layout %q: %s %q", l.FullRepoPath(), u.GetKind(), crKey(u.GetNamespace(), u.GetName()))
	}
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
					if _, convErr := jsonContentOf(u.Object); convErr != nil {
						return refuse(l, u, convErr)
					}
				}
				return errors.Wrapf(err, "layout %q: read list items", l.FullRepoPath())
			}
			for _, u := range objs {
				if seenObject[u] {
					continue
				}
				seenObject[u] = true
				content, err := jsonContentOf(u.Object)
				if err != nil {
					return refuse(l, u, err)
				}
				if content != nil {
					replace = append(replace, replacement{u, u.Object, content})
				}
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
// with no more than maxJSONNodes values. A value that is not is not typed
// here, and is converted or refused (jsonContentOf). It must not accept more:
// a value it calls typed is read as it is, and apimachinery panics on any
// other.
func jsonTyped(v any, depth int) bool {
	nodes := 0
	return jsonTypedCount(v, depth, &nodes)
}

func jsonTypedCount(v any, depth int, nodes *int) bool {
	if *nodes++; depth > maxJSONDepth || *nodes > maxJSONNodes {
		return false
	}
	switch v := v.(type) {
	case string, int64, float64, bool, nil, json.Number:
		return true
	case map[string]any:
		for _, e := range v {
			if !jsonTypedCount(e, depth+1, nodes) {
				return false
			}
		}
		return true
	case []any:
		return !slices.ContainsFunc(v, func(e any) bool { return !jsonTypedCount(e, depth+1, nodes) })
	}
	return false
}

// jsonContentOf returns a copy of the object m, with each value of an accepted
// shape as its JSON typed counterpart, or nil for an object that needs no
// change (jsonTyped). The accepted shapes are closed:
//   - the types runtime.DeepCopyJSONValue accepts, which are kept as they are
//     (a json.Number included);
//   - a Go integer kind (int, int8 to int64, uint, uint8 to uint64, uintptr) as
//     an int64, so an integer above 2^53 keeps its exact value; a uint64 above
//     the largest int64 is refused, as it has no int64;
//   - a float32 as a float64;
//   - a named string, bool, integer or float kind as its base type;
//   - a map with string keys and a slice of any accepted element, element by
//     element, a nil map or slice staying nil; an interface holds an element;
//   - nothing else.
//
// Everything else is refused with the field named: a struct, a pointer, an
// array, a channel, a function, a map with other keys, and any type with a
// marshaler (json.RawMessage and net.IP among them). The conversion is made
// from the Go values and never marshals one, so it ends on every value.
func jsonContentOf(m map[string]any) (map[string]any, error) {
	if jsonTyped(m, 0) {
		return nil, nil
	}
	c := &converter{open: map[openKey]bool{}}
	v, err := c.convert(reflect.ValueOf(m), "", 0)
	if err != nil {
		return nil, err
	}
	content, _ := v.(map[string]any)
	return content, nil
}

// openKey names a map or a slice being converted: a slice by its first
// element and its length, as two slices of one array are two values.
type openKey struct {
	ptr uintptr
	len int
}

type converter struct {
	nodes int
	open  map[openKey]bool
}

var (
	jsonNumberType    = reflect.TypeFor[json.Number]()
	jsonMarshalerType = reflect.TypeFor[json.Marshaler]()
	textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()
)

// hasMarshaler reports whether a value of type t, or one addressed, marshals
// itself, which the converter refuses and never calls.
func hasMarshaler(t reflect.Type) bool {
	for _, t := range []reflect.Type{t, reflect.PointerTo(t)} {
		if t.Implements(jsonMarshalerType) || t.Implements(textMarshalerType) {
			return true
		}
	}
	return false
}

func (c *converter) convert(v reflect.Value, path string, depth int) (any, error) {
	if c.nodes++; c.nodes > maxJSONNodes {
		return nil, errors.Errorf("%s holds more than %d values", where(path), maxJSONNodes)
	}
	if v.Kind() == reflect.Invalid {
		return nil, nil
	}
	if v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil, nil
		}
		return c.convert(v.Elem(), path, depth)
	}
	if v.Type() == jsonNumberType {
		return json.Number(v.String()), nil
	}
	if hasMarshaler(v.Type()) {
		return nil, unsupported(path, v, "it marshals itself")
	}
	switch v.Kind() {
	case reflect.Bool:
		return v.Bool(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		u := v.Uint()
		if u > math.MaxInt64 {
			return nil, errors.Errorf("%s holds %d, which is above the largest int64", where(path), u)
		}
		return int64(u), nil
	case reflect.Float32, reflect.Float64:
		return v.Float(), nil
	case reflect.String:
		return v.String(), nil
	case reflect.Map:
		return c.convertMap(v, path, depth)
	case reflect.Slice:
		return c.convertSlice(v, path, depth)
	default:
		return nil, unsupported(path, v, "")
	}
}

func (c *converter) convertMap(v reflect.Value, path string, depth int) (any, error) {
	if v.Type().Key().Kind() != reflect.String || hasMarshaler(v.Type().Key()) {
		return nil, unsupported(path, v, "its keys are no plain strings")
	}
	if v.IsNil() {
		return nil, nil
	}
	if depth > maxJSONDepth {
		return nil, errors.Errorf("%s holds a value nested deeper than %d levels", where(path), maxJSONDepth)
	}
	key := openKey{ptr: v.Pointer()}
	if c.open[key] {
		return nil, errors.Errorf("%s holds itself", where(path))
	}
	c.open[key] = true
	defer delete(c.open, key)
	keys := v.MapKeys()
	slices.SortFunc(keys, func(a, b reflect.Value) int {
		switch x, y := a.String(), b.String(); {
		case x < y:
			return -1
		case x > y:
			return 1
		}
		return 0
	})
	out := make(map[string]any, len(keys))
	for _, k := range keys {
		p := k.String()
		if path != "" {
			p = path + "." + p
		}
		e, err := c.convert(v.MapIndex(k), p, depth+1)
		if err != nil {
			return nil, err
		}
		out[k.String()] = e
	}
	return out, nil
}

func (c *converter) convertSlice(v reflect.Value, path string, depth int) (any, error) {
	if v.IsNil() {
		return nil, nil
	}
	if depth > maxJSONDepth {
		return nil, errors.Errorf("%s holds a value nested deeper than %d levels", where(path), maxJSONDepth)
	}
	if v.Len() > 0 {
		key := openKey{ptr: v.Pointer(), len: v.Len()}
		if c.open[key] {
			return nil, errors.Errorf("%s holds itself", where(path))
		}
		c.open[key] = true
		defer delete(c.open, key)
	}
	out := make([]any, v.Len())
	for i := range out {
		e, err := c.convert(v.Index(i), fmt.Sprintf("%s[%d]", path, i), depth+1)
		if err != nil {
			return nil, err
		}
		out[i] = e
	}
	return out, nil
}

// unsupported refuses a value of a shape jsonContentOf does not accept.
func unsupported(path string, v reflect.Value, why string) error {
	if why != "" {
		why = ": " + why
	}
	return errors.Errorf("%s holds a %s, which kure does not convert to JSON%s", where(path), v.Type(), why)
}

func where(path string) string {
	if path == "" {
		return "the object"
	}
	return path
}
