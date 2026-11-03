package fluxcd

import (
	"encoding"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"

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

	// maxHeldDepth is how deep walkHeld follows a typed object, counted in the
	// pointers, interfaces, fields, elements and values it steps through, which
	// are several for each level of a value.
	maxHeldDepth = 4 * maxJSONDepth
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
// maxJSONNodes values. No method of a value the caller built is ever called,
// also not to read a typed List that holds it: a marshaler could not be
// stopped on a value that holds itself.
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
		kind, namespace, name := identityOf(u)
		return errors.Wrapf(cause, "layout %q: %s %q", l.FullRepoPath(), kind, crKey(namespace, name))
	}
	var walk func(l *layout.ManifestLayout) error
	walk = func(l *layout.ManifestLayout) error {
		if l == nil || seenLayout[l] {
			return nil
		}
		seenLayout[l] = true
		for _, r := range l.Resources {
			objs, bad, err := callerObjects(r)
			if err != nil {
				if bad != nil {
					return refuse(l, bad, err)
				}
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
//
// A typed List is read from what it writes, and writing it marshals what it
// holds. So before that every value reachable from it is walked (walkHeld),
// and a typed List the walk cannot read, one that holds itself or one that
// holds an unstructured object with a value that marshalling would call a
// method of, is refused. The object that fails is returned as bad.
func callerObjects(r client.Object) (objs []*unstructured.Unstructured, bad *unstructured.Unstructured, err error) {
	switch o := r.(type) {
	case nil:
		return nil, nil, nil
	case *unstructured.Unstructured:
		if o == nil {
			return nil, nil, nil
		}
		return []*unstructured.Unstructured{o}, nil, nil
	}
	// The walk is bounded, so it goes first: the cycle check below revisits a
	// List held twice each time it is held.
	if bad, err := walkHeld(r); err != nil {
		return nil, bad, err
	}
	if err := built.RefuseItemCycle(r); err != nil {
		return nil, nil, err
	}
	objs, err = built.HeldItems(r)
	return objs, nil, err
}

// identityOf returns the kind, namespace and name of u for a refusal, read
// from the fields as they are. The accessors of Unstructured format a value
// that is no string, and a value that holds itself has no end when formatted.
func identityOf(u *unstructured.Unstructured) (kind, namespace, name string) {
	kind, _ = u.Object["kind"].(string)
	meta, _ := u.Object["metadata"].(map[string]any)
	namespace, _ = meta["namespace"].(string)
	name, _ = meta["name"].(string)
	return kind, namespace, name
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
//   - a float32 as the float64 of its shortest 32-bit decimal form, as
//     encoding/json writes it (float32(0.1) is 0.1);
//   - a byte slice as its base64 string, as encoding/json writes it, which a
//     list of more than maxJSONNodes bytes is not;
//   - a named string, bool, integer or float kind as its base type;
//   - a map with string keys and a slice of any accepted element, element by
//     element, a nil map or slice staying nil; an interface holds an element;
//   - nothing else.
//
// Everything else is refused with the field named: a struct, a pointer, an
// array, a channel, a function, a map with other keys, and any type with a
// marshaler (json.RawMessage and net.IP among them). The conversion is made
// from the Go values and never marshals one, so it ends on every value. A slice
// longer than the values left of maxJSONNodes is refused before it is copied.
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

var (
	unstructuredType     = reflect.TypeFor[unstructured.Unstructured]()
	unstructuredListType = reflect.TypeFor[unstructured.UnstructuredList]()
)

// walkHeld walks every value reachable from the typed object r, before
// anything marshals it, and fails closed. It looks for the unstructured objects
// r holds, wherever it holds them, and checks each for what marshalling it
// would call a method of or would not end on (reachable). Interfaces and
// non-nil pointers are followed. The unexported fields marshalling leaves out
// are not, unless the type marshals itself. Each pointer, map and slice is
// followed once on a path: one that holds itself is refused, as is a value nested deeper than maxHeldDepth, a
// walk of more than maxJSONNodes values and a value that cannot be read. When
// an unstructured object fails, bad is a wrapper of its Object map, to name it
// in the refusal.
func walkHeld(r client.Object) (bad *unstructured.Unstructured, err error) {
	w := &heldWalk{open: map[heldKey]bool{}}
	err = w.walk(reflect.ValueOf(r), "", 0)
	return w.bad, err
}

// heldKey names a pointer, a map or a slice being walked.
type heldKey struct {
	typ reflect.Type
	ptr uintptr
	len int
}

type heldWalk struct {
	nodes int
	open  map[heldKey]bool
	bad   *unstructured.Unstructured
}

// scalarKind reports whether a value of kind k holds nothing to walk into.
func scalarKind(k reflect.Kind) bool {
	switch k {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.String,
		reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return true
	default:
		return false
	}
}

func (w *heldWalk) walk(v reflect.Value, path string, depth int) error {
	if w.nodes++; w.nodes > maxJSONNodes {
		return errors.Errorf("%s holds more than %d values", whereTyped(path), maxJSONNodes)
	}
	if depth > maxHeldDepth {
		return errors.Errorf("%s holds a value nested deeper than %d steps", whereTyped(path), maxHeldDepth)
	}
	switch v.Kind() {
	case reflect.Invalid:
		return nil
	case reflect.Interface:
		if v.IsNil() {
			return nil
		}
		return w.walk(v.Elem(), path, depth)
	case reflect.Pointer:
		if v.IsNil() {
			return nil
		}
		if done, err := w.enter(v, heldKey{typ: v.Type(), ptr: v.Pointer()}, path); done || err != nil {
			return err
		}
		defer delete(w.open, heldKey{typ: v.Type(), ptr: v.Pointer()})
		return w.walk(v.Elem(), path, depth+1)
	case reflect.Map:
		if v.IsNil() || scalarKind(v.Type().Elem().Kind()) {
			return nil
		}
		key := heldKey{typ: v.Type(), ptr: v.Pointer()}
		if _, err := w.enter(v, key, path); err != nil {
			return err
		}
		defer delete(w.open, key)
		for it := v.MapRange(); it.Next(); {
			// The key is not formatted: a key may hold itself.
			if err := w.walk(it.Value(), path+"[key]", depth+1); err != nil {
				return err
			}
		}
		return nil
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.IsNil() || scalarKind(v.Type().Elem().Kind()) {
			return nil
		}
		if v.Len() > maxJSONNodes-w.nodes {
			return errors.Errorf("%s holds more than %d values", whereTyped(path), maxJSONNodes)
		}
		if v.Kind() == reflect.Slice && v.Len() > 0 {
			key := heldKey{typ: v.Type(), ptr: v.Pointer(), len: v.Len()}
			if _, err := w.enter(v, key, path); err != nil {
				return err
			}
			defer delete(w.open, key)
		}
		for i := 0; i < v.Len(); i++ {
			if err := w.walk(v.Index(i), fmt.Sprintf("%s[%d]", path, i), depth+1); err != nil {
				return err
			}
		}
		return nil
	case reflect.Struct:
		return w.walkStruct(v, path, depth)
	default:
		// A scalar, a channel, a function: nothing in it for marshalling to call.
		return nil
	}
}

// enter marks the pointer, map or slice v, with its key, as open on the path.
// A key already open is a value that holds itself: it is refused.
func (w *heldWalk) enter(v reflect.Value, key heldKey, path string) (done bool, err error) {
	if w.open[key] {
		return true, errors.Errorf("%s holds itself", whereTyped(path))
	}
	w.open[key] = true
	return false, nil
}

func (w *heldWalk) walkStruct(v reflect.Value, path string, depth int) error {
	switch v.Type() {
	case unstructuredType, unstructuredListType:
		if !v.CanInterface() {
			return errors.Errorf("%s holds an unstructured object that cannot be read", whereTyped(path))
		}
		m, _ := v.FieldByName("Object").Interface().(map[string]any)
		if err := reachable(m); err != nil {
			w.bad = &unstructured.Unstructured{Object: m}
			return err
		}
		if v.Type() == unstructuredListType {
			return w.walk(v.FieldByName("Items"), path+".Items", depth+1)
		}
		return nil
	}
	// A type that marshals itself may write any of its fields, a private one
	// included. Marshalling leaves out the private fields of any other.
	writesItself := hasMarshaler(v.Type())
	for i := 0; i < v.NumField(); i++ {
		f := v.Type().Field(i)
		if !f.IsExported() && !f.Anonymous && !writesItself {
			continue
		}
		if err := w.walk(v.Field(i), path+"."+f.Name, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func whereTyped(path string) string {
	if path == "" {
		return "the typed object"
	}
	return "the typed object" + path
}

// reachable reports a value in m that marshalling it would call a method of,
// or would not end on: one with a marshaler, one that holds itself, one nested
// too deep or with too many values, and a struct, a pointer or an array, which
// marshalling follows into fields that may hold any of those. It leaves the
// shapes that marshalling only fails on, a channel say, to the conversion.
func reachable(m map[string]any) error {
	if jsonTyped(m, 0) {
		return nil
	}
	c := &converter{open: map[openKey]bool{}, reach: true}
	_, err := c.convert(reflect.ValueOf(m), "", 0)
	return err
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

	// reach makes the walk the check of reachable: it refuses what marshalling
	// would call or not end on, and converts nothing else.
	reach bool
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
	// An interface and the value it holds are one value, as jsonTyped counts them.
	if v.Kind() == reflect.Interface && !v.IsNil() {
		v = v.Elem()
	}
	if c.nodes++; c.nodes > maxJSONNodes {
		return nil, errors.Errorf("%s holds more than %d values", where(path), maxJSONNodes)
	}
	if v.Kind() == reflect.Invalid || v.Kind() == reflect.Interface {
		return nil, nil
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
			if c.reach {
				return nil, nil
			}
			return nil, errors.Errorf("%s holds %d, which is above the largest int64", where(path), u)
		}
		return int64(u), nil
	case reflect.Float32:
		// As encoding/json writes it, in 32 bits, and reads it back: float32(0.1)
		// is 0.1, not 0.10000000149011612.
		f, err := strconv.ParseFloat(strconv.FormatFloat(v.Float(), 'g', -1, 32), 64)
		if err != nil {
			return nil, errors.Wrapf(err, "%s holds a float32 that is no number", where(path))
		}
		return f, nil
	case reflect.Float64:
		return v.Float(), nil
	case reflect.String:
		return v.String(), nil
	case reflect.Map:
		return c.convertMap(v, path, depth)
	case reflect.Slice:
		return c.convertSlice(v, path, depth)
	case reflect.Chan, reflect.Func, reflect.Complex64, reflect.Complex128, reflect.UnsafePointer:
		if c.reach {
			return nil, nil
		}
		return nil, unsupported(path, v, "")
	default:
		return nil, unsupported(path, v, "")
	}
}

func (c *converter) convertMap(v reflect.Value, path string, depth int) (any, error) {
	if hasMarshaler(v.Type().Key()) || (v.Type().Key().Kind() != reflect.String && !c.reach) {
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
	if c.isByteSlice(v.Type()) {
		// As encoding/json writes it: a base64 string.
		if v.Len() > maxJSONNodes {
			return nil, errors.Errorf("%s holds more than %d bytes", where(path), maxJSONNodes)
		}
		if c.reach {
			return nil, nil
		}
		return base64.StdEncoding.EncodeToString(v.Bytes()), nil
	}
	// The count is checked before the allocation: a slice of a type that needs no
	// storage can be as long as an int holds.
	if v.Len() > maxJSONNodes-c.nodes {
		return nil, errors.Errorf("%s holds more than %d values", where(path), maxJSONNodes)
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

// isByteSlice reports whether encoding/json writes a slice of type t as a
// base64 string: its elements are bytes, and a byte has no marshaler.
func (c *converter) isByteSlice(t reflect.Type) bool {
	return t.Elem().Kind() == reflect.Uint8 && !hasMarshaler(t.Elem())
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
