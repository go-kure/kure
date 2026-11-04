package fluxcd

import (
	"encoding"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/internal/built"
	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack/layout"
)

const (
	// maxJSONDepth is how deep a value is followed, as encoding/json counts
	// a nesting before it takes it for a cycle.
	maxJSONDepth = 1000

	// maxHeldDepth is how deep walkHeld follows a typed object, counted in the
	// pointers, interfaces, fields, elements and values it steps through, which
	// are several for each level of a value.
	maxHeldDepth = 4 * maxJSONDepth
)

// maxJSONNodes is how many values are visited in one object. An object
// is far smaller (the API server stores 1.5 MiB at most); a larger count
// is a value that shares its parts so often that its tree is huge.
// It is a var only so that a test can lower it through the setter in
// export_test.go; no in-process test that does so may be t.Parallel.
var maxJSONNodes = 1 << 22

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
// holds (built.HeldObjects). An unstructured List's items are in its object,
// so they are covered with it. Nil roots are skipped.
//
// A type that embeds an unstructured object, as a resource or as an item of a
// typed List, is not normalized: the layout and the walker's records of an
// object's origin hold the wrapper, so a copy in its place would lose what is
// done to the wrapper. It is refused, naming its type, when its map needs
// conversion (refuseWrapper), and left as it is when it does not.
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
		for i, r := range l.Resources {
			if err := refuseWrapper(l, i, r); err != nil {
				return err
			}
			objs, others, bad, err := callerObjects(r)
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
				return errors.Wrapf(err, "layout %q: read list items of resource %d", l.FullRepoPath(), i)
			}
			for _, o := range others {
				if err := refuseWrapper(l, i, o); err != nil {
					return err
				}
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

// refuseWrapper refuses r, resource i of layout l or an item of it, when it is
// a wrapper that embeds an unstructured object (embeddedUnstructured) whose map
// holds a value that is not JSON typed, or one that embeds an unstructured List
// with such a value in its own map or in an item. Such a wrapper cannot be normalized:
// the layout and the records the walker made of its origin hold the wrapper,
// and a copy put in its place would no longer carry what is done to the
// wrapper. So it is refused, naming its type, and a wrapper that holds only
// JSON typed values is left as it is. An object that is no wrapper is no
// concern of this check.
func refuseWrapper(l *layout.ManifestLayout, i int, r apiruntime.Object) error {
	embedded, st, err := embeddedUnstructured(r)
	if err != nil {
		return errors.Wrapf(err, "layout %q: resource %d", l.FullRepoPath(), i)
	}
	for _, obj := range embedded {
		content, err := jsonContentOf(obj.Object)
		if err == nil && content == nil {
			continue
		}
		if err == nil {
			err = errors.New("holds a value that is not JSON typed")
		}
		kind, namespace, name := identityOf(obj)
		return errors.Wrapf(err, "layout %q: %s %q is held by a %s that embeds it, and is not normalized; hold it as a *unstructured.Unstructured to have it normalized", l.FullRepoPath(), kind, crKey(namespace, name), st)
	}
	return nil
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
//
// others are the objects a typed List holds that are not unstructured, Lists
// included, which the caller looks at for a wrapper that embeds an unstructured
// object or List.
func callerObjects(r client.Object) (objs []*unstructured.Unstructured, others []apiruntime.Object, bad *unstructured.Unstructured, err error) {
	if r == nil {
		return nil, nil, nil, nil
	}
	// A typed nil holds no object, and a method called on it may panic, here and
	// in the phases after this one: it is refused, not passed on unread. An
	// unstructured one is no exception: its accessors dereference it.
	if v := reflect.ValueOf(r); v.Kind() == reflect.Pointer && v.IsNil() {
		return nil, nil, nil, errors.Errorf("a nil %T", r)
	}
	if o, ok := r.(*unstructured.Unstructured); ok {
		return []*unstructured.Unstructured{o}, nil, nil, nil
	}
	// The walk is bounded, so it goes first: the cycle check below revisits a
	// List held twice each time it is held.
	if bad, err := walkHeld(r); err != nil {
		return nil, nil, bad, err
	}
	if err := built.RefuseItemCycle(r); err != nil {
		return nil, nil, nil, err
	}
	objs, others, err = built.HeldObjects(r)
	return objs, others, nil, err
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
// non-nil pointers are followed; the unexported fields marshalling leaves out
// are not. A type of r that marshals itself, a map key included, is refused
// unless its package is trusted (checkType). Each pointer, map and slice is
// followed once on a path: one that holds itself is refused, as is a value
// nested deeper than maxHeldDepth, a walk of more than maxJSONNodes values and
// a value that cannot be read. When
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
	nodes   int
	open    map[heldKey]bool
	checked map[reflect.Type]bool
	bad     *unstructured.Unstructured
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
	default:
	}
	if err := w.checkType(v.Type(), path); err != nil {
		return err
	}
	switch v.Kind() {
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
		// A key is written by its marshaler, if it has one, and a type is checked
		// before any shortcut. A key is not formatted: it may hold itself.
		if err := w.checkType(v.Type().Key(), path+"[key]"); err != nil {
			return err
		}
		if err := w.checkType(v.Type().Elem(), path+"[key]"); err != nil {
			return err
		}
		// An interface key holds any type: the keys are walked, as the values are.
		keys := v.Type().Key().Kind() == reflect.Interface
		if v.IsNil() || scalarKind(v.Type().Elem().Kind()) && !keys {
			return nil
		}
		key := heldKey{typ: v.Type(), ptr: v.Pointer()}
		if _, err := w.enter(v, key, path); err != nil {
			return err
		}
		defer delete(w.open, key)
		for it := v.MapRange(); it.Next(); {
			if keys {
				if err := w.walk(it.Key(), path+"[key]", depth+1); err != nil {
					return err
				}
			}
			if err := w.walk(it.Value(), path+"[key]", depth+1); err != nil {
				return err
			}
		}
		return nil
	case reflect.Slice, reflect.Array:
		if err := w.checkType(v.Type().Elem(), path+"[]"); err != nil {
			return err
		}
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

// checkType refuses a type that marshals itself unless its package is trusted.
// Such a method may write what the walk cannot see, from a private field say,
// and may call anything. A package is trusted when it is one of
// trustedStdPackages, or in k8s.io, sigs.k8s.io, github.com/fluxcd or
// github.com/go-kure, and is not a test package. A type of no package, a struct that embeds a
// marshaler for one, is not trusted. A named struct of another package that
// declares no marshaler and only promotes it from an embedded trusted type
// (a type that embeds *unstructured.Unstructured) is trusted: the call is the
// embedded type's.
func (w *heldWalk) checkType(t reflect.Type, path string) error {
	if t.Kind() == reflect.Interface || w.checked[t] {
		return nil
	}
	if w.checked == nil {
		w.checked = map[reflect.Type]bool{}
	}
	if !hasMarshaler(t) || trustedPackage(t) || trustedType(t) || promotedFromTrusted(t) {
		w.checked[t] = true
		return nil
	}
	return errors.Errorf("%s holds a %s, which marshals itself and is not from a trusted package", whereTyped(path), t)
}

var (
	// trustedPackagePrefixes are the module paths whose marshalers are trusted.
	trustedPackagePrefixes = []string{"k8s.io/", "sigs.k8s.io/", "github.com/fluxcd/", "github.com/go-kure/"}

	// trustedTypes are single types of a package that is not trusted, whose
	// marshaler was read. XDSResource (a Cilium Envoy config's resource) writes
	// its embedded protobuf Any with protojson and nothing else of the Envoy
	// config's. protojson resolves the Any's type URL in the global protobuf
	// registry, so a message type a caller registered there is still run, as it
	// is for the same config on main. Add a type here only after reading its
	// marshaler; a whole package is never trusted for one type.
	trustedTypes = []string{"github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2.XDSResource"}

	// trustedStdPackages are the standard library packages that define a
	// marshaler. A path with no dot is not enough: package main and a caller's
	// module of a short name have none.
	trustedStdPackages = []string{"encoding/json", "encoding/json/jsontext", "encoding/json/v2", "log/slog", "math/big", "net", "net/netip", "net/url", "regexp", "time"}
)

// marshalerMethods are the methods that make a type marshal itself.
var marshalerMethods = []string{"MarshalJSON", "MarshalText", "MarshalYAML"}

// maxPromotionDepth bounds how many embedded types promotedFromTrusted follows.
// A type deeper than this is not trusted.
const maxPromotionDepth = 16

// promotedFromTrusted reports whether every marshaler method of the named
// struct t is promoted from an embedded field of a trusted type, and t declares
// none of its own.
//
// It depends on the Go runtime: a method t declares has a source file, and one
// the compiler writes to promote a method of an embedded field has the file
// "<autogenerated>" (isPromotion). Where this cannot be told, t is not trusted:
// the check fails closed, and a refusal follows. So does a type that embeds
// itself, directly or through other types, and one nested deeper than
// maxPromotionDepth.
func promotedFromTrusted(t reflect.Type) bool {
	return promotedFrom(t, map[reflect.Type]bool{}, 0)
}

func promotedFrom(t reflect.Type, onPath map[reflect.Type]bool, depth int) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || t.Name() == "" || onPath[t] || depth > maxPromotionDepth {
		return false
	}
	onPath[t] = true
	defer delete(onPath, t)
	found := false
	for _, pt := range []reflect.Type{t, reflect.PointerTo(t)} {
		for _, name := range marshalerMethods {
			m, ok := pt.MethodByName(name)
			if !ok {
				continue
			}
			found = true
			if !isPromotion(m.Func.Pointer()) {
				return false
			}
		}
	}
	if !found {
		return false
	}
	embedsMarshaler := false
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.Anonymous || !hasMarshaler(f.Type) {
			continue
		}
		embedsMarshaler = true
		if !trustedPackage(f.Type) && !promotedFrom(f.Type, onPath, depth+1) {
			return false
		}
	}
	return embedsMarshaler
}

// isPromotion reports whether the function at pc is a wrapper the compiler
// wrote to promote a method of an embedded field. A function it cannot find, or
// one with a source file, is not.
func isPromotion(pc uintptr) bool {
	f := runtime.FuncForPC(pc)
	if f == nil {
		return false
	}
	file, _ := f.FileLine(f.Entry())
	return file == "<autogenerated>"
}

func trustedPackage(t reflect.Type) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return trustedPath(t.PkgPath())
}

// trustedType reports whether t, or the type it points to, is one of trustedTypes.
func trustedType(t reflect.Type) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Name() != "" && slices.Contains(trustedTypes, t.PkgPath()+"."+t.Name())
}

func trustedPath(p string) bool {
	switch {
	case p == "" || strings.HasSuffix(p, "_test"):
		return false
	case slices.Contains(trustedStdPackages, p):
		return true
	}
	return slices.ContainsFunc(trustedPackagePrefixes, func(prefix string) bool { return strings.HasPrefix(p, prefix) })
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
	// Marshalling leaves out the private fields of a struct. A type that
	// marshals itself and could write one is refused by checkType.
	for i := 0; i < v.NumField(); i++ {
		f := v.Type().Field(i)
		if !f.IsExported() && !f.Anonymous {
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
	yamlMarshalerType = reflect.TypeFor[interface{ MarshalYAML() (any, error) }]()
)

// hasMarshaler reports whether a value of type t, or one addressed, marshals
// itself, which the converter refuses and never calls.
func hasMarshaler(t reflect.Type) bool {
	for _, t := range []reflect.Type{t, reflect.PointerTo(t)} {
		if t.Implements(jsonMarshalerType) || t.Implements(textMarshalerType) || t.Implements(yamlMarshalerType) {
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
