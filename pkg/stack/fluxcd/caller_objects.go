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
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utiljson "k8s.io/apimachinery/pkg/util/json"
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
// the object its JSON form holds (jsonContentOf): an int64 or a float64 for a
// number, a map[string]any for a map[string]string. Its Object field is set
// to a new map, and the caller's map is not changed.
//
// A value that has no JSON form here refuses the call, with the layout, the
// object and the field named, and no object is replaced then: a channel, a
// function, a struct (including a pointer to one), a map whose keys are no
// strings, a value that holds itself, one nested deeper than maxJSONDepth or
// with more than maxJSONNodes values. The encoder is never run on a value of
// the caller's: it could not be stopped on one that holds itself through a
// struct.
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

// jsonContentOf returns the object m is, as the JSON it marshals to would
// decode, or nil for an object that needs no change (jsonTyped). It is built
// from the Go values directly, and not through encoding/json, so that it
// ends on every value: a number is an int64 when it is a whole number that
// fits one, else a float64, and an integer above 2^53 keeps its exact value;
// a []byte is its base64 string; a nil map or slice is nil; a string is valid
// UTF-8; a value of a non-struct type that marshals itself (json.RawMessage,
// net.IP, json.Number) is what the encoder writes for it, read back. It refuses a
// value with no JSON form here (normalizeCallerObjects), naming the field.
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
	jsonNumberType   = reflect.TypeFor[json.Number]()
	jsonMarshaler    = reflect.TypeFor[json.Marshaler]()
	textMarshalerTyp = reflect.TypeFor[encoding.TextMarshaler]()
)

// marshalsItself reports whether the encoder writes a value of type t by a
// method or a rule of its own that its Go kind does not give: a
// json.RawMessage, a net.IP, a json.Number.
func marshalsItself(t reflect.Type) bool {
	return t == jsonNumberType || t.Implements(jsonMarshaler) || t.Implements(textMarshalerTyp)
}

// viaJSON is x as the encoder writes it and the decoder reads it back. It is
// for a leaf that is no struct, which holds no value of the caller's to
// follow: the encoder runs on that one value only.
func viaJSON(x any, path string) (any, error) {
	data, err := json.Marshal(x)
	if err != nil {
		return nil, errors.Wrapf(err, "%s", where(path))
	}
	var out any
	if err := utiljson.Unmarshal(data, &out); err != nil {
		return nil, errors.Wrapf(err, "%s", where(path))
	}
	return out, nil
}

// validUTF8 is s with each byte that is no UTF-8 replaced by U+FFFD, as the
// encoder writes a string.
func validUTF8(s string) string {
	return strings.Map(func(r rune) rune { return r }, s)
}

func (c *converter) convert(v reflect.Value, path string, depth int) (any, error) {
	if c.nodes++; c.nodes > maxJSONNodes {
		return nil, errors.Errorf("%s holds more than %d values", where(path), maxJSONNodes)
	}
	if k := v.Kind(); k != reflect.Struct && k != reflect.Interface && k != reflect.Pointer && v.CanInterface() && marshalsItself(v.Type()) {
		if k == reflect.Map || k == reflect.Slice {
			if v.IsNil() {
				return nil, nil
			}
		}
		return viaJSON(v.Interface(), path)
	}
	switch v.Kind() {
	case reflect.Invalid:
		return nil, nil
	case reflect.Interface:
		if v.IsNil() {
			return nil, nil
		}
		return c.convert(v.Elem(), path, depth)
	case reflect.Pointer:
		if v.IsNil() {
			return nil, nil
		}
		if v.Elem().Kind() == reflect.Struct {
			return nil, unsupported(path, v)
		}
		key := openKey{ptr: v.Pointer()}
		if c.open[key] {
			return nil, errors.Errorf("%s holds itself", where(path))
		}
		c.open[key] = true
		defer delete(c.open, key)
		return c.convert(v.Elem(), path, depth)
	case reflect.Bool:
		return v.Bool(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if u := v.Uint(); u <= math.MaxInt64 {
			return int64(u), nil
		}
		return float64(v.Uint()), nil
	case reflect.Float32:
		return number(path, v.Float(), 32)
	case reflect.Float64:
		return number(path, v.Float(), 64)
	case reflect.String:
		return validUTF8(v.String()), nil
	case reflect.Map:
		return c.convertMap(v, path, depth)
	case reflect.Slice, reflect.Array:
		return c.convertList(v, path, depth)
	}
	return nil, unsupported(path, v)
}

func (c *converter) convertMap(v reflect.Value, path string, depth int) (any, error) {
	if v.Type().Key().Kind() != reflect.String {
		return nil, unsupported(path, v)
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
		out[validUTF8(k.String())] = e
	}
	return out, nil
}

func (c *converter) convertList(v reflect.Value, path string, depth int) (any, error) {
	if v.Kind() == reflect.Slice {
		if v.IsNil() {
			return nil, nil
		}
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return base64.StdEncoding.EncodeToString(v.Bytes()), nil
		}
		if v.Len() > 0 {
			key := openKey{ptr: v.Pointer(), len: v.Len()}
			if c.open[key] {
				return nil, errors.Errorf("%s holds itself", where(path))
			}
			c.open[key] = true
			defer delete(c.open, key)
		}
	}
	if depth > maxJSONDepth {
		return nil, errors.Errorf("%s holds a value nested deeper than %d levels", where(path), maxJSONDepth)
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

// number is f, a float of the given size, as the decoder reads what the
// encoder writes for it: the shortest decimal that gives f back, an int64 when
// that is a whole number that fits one, else a float64.
func number(path string, f float64, bits int) (any, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return nil, errors.Errorf("%s holds %v, which JSON cannot hold", where(path), f)
	}
	format := byte('f')
	if abs := math.Abs(f); abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		format = 'e'
	}
	text := strconv.FormatFloat(f, format, -1, bits)
	if i, err := strconv.ParseInt(text, 10, 64); err == nil {
		return i, nil
	}
	return strconv.ParseFloat(text, 64)
}

func unsupported(path string, v reflect.Value) error {
	return errors.Errorf("%s holds a %s, which kure cannot read as JSON", where(path), v.Type())
}

func where(path string) string {
	if path == "" {
		return "the object"
	}
	return path
}
