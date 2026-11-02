package kustomizeschema

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"reflect"
	"sort"
	"strconv"
)

// fingerprint returns the SHA-256 of a canonical walk over v, in hex.
//
// It exists because kyaml reads fields a serializer does not tell apart:
// kube-openapi's Schema marshals Extensions and ExtraProps into one JSON
// object (spec/schema.go:521-540), but kyaml reads only Extensions, so moving
// "x-kubernetes-patch-strategy" from the one to the other keeps the JSON and
// changes how a patch merges. The walk reads every field, exported or not,
// through reflect.Value and never Interface(): a struct is tagged with its
// type and each field with its name, so a value moved to another field
// changes the stream; a map is walked in key order, its nil and empty forms
// told apart; a pointer is dereferenced, with a guard on a cycle; an
// interface value is tagged with its dynamic type, so 1 and 1.0 differ, and
// every value with its full concrete type (package path and name), so a named
// type never reads as its underlying one: Strategy("merge") is no "merge". A
// function or channel contributes its type and whether it is nil, as its
// identity is not a definition.
//
// What the fingerprint depends on is therefore the layout of the types it
// walks (their field names and order, which a library bump can change), not a
// marshaler. TestDefinitions_BuiltInMatchesThePin catches such a bump.
func fingerprint(v any) string {
	h := sha256.New()
	w := &walker{out: bufio.NewWriterSize(h, 1<<16), visiting: map[visit]bool{}, ids: map[reflect.Type]string{}}
	w.walk(reflect.ValueOf(v))
	_ = w.out.Flush()
	return hex.EncodeToString(h.Sum(nil))
}

// visit identifies a pointer-like value on the walk's path.
type visit struct {
	p uintptr
	t reflect.Type
}

type walker struct {
	out      *bufio.Writer
	visiting map[visit]bool
	ids      map[reflect.Type]string
	scratch  []byte
}

// typed writes what the value is, kind and full concrete type, so a named type
// never collides with its underlying type or with a same-named type of another
// package. A kind alone would let Strategy("merge") pass for "merge", which
// kyaml's own type assertion then rejects.
func (w *walker) typed(kind string, t reflect.Type) {
	id, ok := w.ids[t]
	if !ok {
		id = t.String()
		if t.Name() != "" {
			id = t.PkgPath() + "." + t.Name()
		}
		w.ids[t] = id
	}
	// The same bytes as tag(kind + " " + id), without building the string.
	w.num(int64(len(kind) + 1 + len(id)))
	_, _ = w.out.WriteString(kind)
	_ = w.out.WriteByte(' ')
	_, _ = w.out.WriteString(id)
}

// tag writes s, length-prefixed, so one tag cannot run into the next.
func (w *walker) tag(s string) {
	w.num(int64(len(s)))
	_, _ = w.out.WriteString(s)
}

func (w *walker) num(n int64) {
	w.scratch = strconv.AppendInt(w.scratch[:0], n, 10)
	w.scratch = append(w.scratch, ';')
	_, _ = w.out.Write(w.scratch)
}

func (w *walker) unum(n uint64) {
	w.scratch = strconv.AppendUint(w.scratch[:0], n, 10)
	w.scratch = append(w.scratch, ';')
	_, _ = w.out.Write(w.scratch)
}

// enter reports false when v is already on the path (a cycle); leave undoes it.
func (w *walker) enter(v reflect.Value) (visit, bool) {
	k := visit{v.Pointer(), v.Type()}
	if w.visiting[k] {
		return k, false
	}
	w.visiting[k] = true
	return k, true
}

func (w *walker) walk(v reflect.Value) {
	if !v.IsValid() {
		w.tag("invalid")
		return
	}
	switch v.Kind() {
	case reflect.Pointer:
		w.typed("ptr", v.Type())
		if v.IsNil() {
			w.tag("nil")
			return
		}
		k, ok := w.enter(v)
		if !ok {
			w.tag("cycle")
			return
		}
		w.walk(v.Elem())
		delete(w.visiting, k)
	case reflect.Interface:
		w.typed("iface", v.Type())
		if v.IsNil() {
			w.tag("nil")
			return
		}
		w.walk(v.Elem())
	case reflect.Struct:
		t := v.Type()
		w.typed("struct", t)
		for i := range t.NumField() {
			w.tag(t.Field(i).Name)
			w.walk(v.Field(i))
		}
	case reflect.Map:
		w.typed("map", v.Type())
		if v.IsNil() {
			w.tag("nil")
			return
		}
		k, ok := w.enter(v)
		if !ok {
			w.tag("cycle")
			return
		}
		w.num(int64(v.Len()))
		for _, e := range w.sortedEntries(v) {
			_, _ = w.out.WriteString(e.key)
			w.walk(e.val)
		}
		delete(w.visiting, k)
	case reflect.Slice:
		w.typed("slice", v.Type())
		if v.IsNil() {
			w.tag("nil")
			return
		}
		if v.Len() > 0 {
			k, ok := w.enter(v)
			if !ok {
				w.tag("cycle")
				return
			}
			defer delete(w.visiting, k)
		}
		w.num(int64(v.Len()))
		for i := range v.Len() {
			w.walk(v.Index(i))
		}
	case reflect.Array:
		w.typed("array", v.Type())
		for i := range v.Len() {
			w.walk(v.Index(i))
		}
	case reflect.String:
		w.typed("string", v.Type())
		w.tag(v.String())
	case reflect.Bool:
		w.typed("bool", v.Type())
		if v.Bool() {
			w.num(1)
		} else {
			w.num(0)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		w.typed("int", v.Type())
		w.num(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		w.typed("uint", v.Type())
		w.unum(v.Uint())
	case reflect.Float32, reflect.Float64:
		w.typed("float", v.Type())
		w.unum(math.Float64bits(v.Float()))
	case reflect.Complex64, reflect.Complex128:
		w.typed("complex", v.Type())
		c := v.Complex()
		w.unum(math.Float64bits(real(c)))
		w.unum(math.Float64bits(imag(c)))
	default: // Func, Chan, UnsafePointer
		w.typed(v.Kind().String(), v.Type())
		if v.IsNil() {
			w.tag("nil")
		} else {
			w.tag("set")
		}
	}
}

type entry struct {
	key string
	val reflect.Value
}

// sortedEntries returns the entries of the map v by the canonical encoding of
// their keys, so the order of iteration never shows in the stream.
func (w *walker) sortedEntries(v reflect.Value) []entry {
	out := make([]entry, 0, v.Len())
	for it := v.MapRange(); it.Next(); {
		k := it.Key()
		var key string
		if k.Kind() == reflect.String { // its type is the map's, tagged by walk; an interface key takes encode
			key = strconv.Itoa(k.Len()) + ":" + k.String()
		} else {
			key = w.encode(k)
		}
		out = append(out, entry{key, it.Value()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key < out[j].key })
	return out
}

// encode returns the canonical encoding of v alone, for a key that is no
// string. The encoding shares the path's cycle guard.
func (w *walker) encode(v reflect.Value) string {
	var buf bytes.Buffer
	sub := &walker{out: bufio.NewWriter(&buf), visiting: w.visiting, ids: w.ids}
	sub.walk(v)
	_ = sub.out.Flush()
	return buf.String()
}
