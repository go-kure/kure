package kustomizeschema

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"math"
	"reflect"
	"slices"
	"strings"
)

// fingerprint returns the SHA-256 of a canonical walk over v, in hex.
//
// It exists because kyaml reads fields a serializer does not tell apart:
// kube-openapi's Schema marshals Extensions and ExtraProps into one JSON
// object (spec/schema.go:521-540), but kyaml reads only Extensions, so moving
// "x-kubernetes-patch-strategy" from the one to the other keeps the JSON and
// changes how a patch merges. The walk reads every field, exported or not,
// through reflect.Value and never Interface().
//
// The stream says what each value is without repeating it. The first time a
// type is met it is defined once, by its kind and full name (package path and
// name, so a named type never reads as its underlying one: Strategy("merge")
// is no "merge"), and for a struct by each field's name and type; every later
// use writes the number the definition got. The type of a value that sits in a
// struct field, a slice or array element, a map key or a map value is the one
// its container's definition names, so only the value is written there; an
// interface value is written with the number of its dynamic type, so 1 and 1.0
// differ. A value moved to another field changes the stream, because a struct
// writes its fields in the order its definition lists them. A map is walked in
// key order, its nil and empty forms told apart. A pointer or map met a second
// time is written as a reference to its first occurrence, so a schema that two
// parents share is read once; its sharing is part of the stream, which makes a
// replacement by an equal copy a difference, not a silent equality. A pointer,
// map or slice that leads back to itself is written as a cycle. A function or
// channel contributes its type and whether it is nil, as its identity is not a
// definition.
//
// What the fingerprint depends on is therefore the layout of the types it
// walks (their field names and types and their order, which a library bump can
// change), not a marshaler. TestDefinitions_BuiltInMatchesThePin catches such
// a bump.
func fingerprint(v any) string {
	h := sha256.New()
	w := newWalker(h)
	w.root(reflect.ValueOf(v))
	w.flush()
	return hex.EncodeToString(h.Sum(nil))
}

// The bytes that mark what follows. A type is written as tagDefine, its
// number and its definition, or as tagType and its number.
const (
	tagInvalid byte = iota
	tagNil
	tagPresent
	tagRef
	tagCycle
	tagDefine
	tagType
)

// visit identifies a pointer-like value: where it points, its type and, for a
// slice, its length.
type visit struct {
	p uintptr
	t reflect.Type
	n int
}

type walker struct {
	h        hash.Hash // nil for a walk that is kept in buf
	buf      []byte
	types    map[reflect.Type]int
	seen     map[visit]int // pointers and maps met so far, by the number of their first occurrence
	visiting map[visit]bool
	entries  []entry
}

func newWalker(h hash.Hash) *walker {
	return &walker{h: h, buf: make([]byte, 0, 1<<16), types: map[reflect.Type]int{}, seen: map[visit]int{}, visiting: map[visit]bool{}}
}

// root writes v with its type, whatever kind it is.
func (w *walker) root(v reflect.Value) {
	if !v.IsValid() {
		w.byte(tagInvalid)
		return
	}
	w.typed(v.Type())
	w.body(v)
}

func (w *walker) flush() {
	if w.h != nil {
		_, _ = w.h.Write(w.buf)
		w.buf = w.buf[:0]
	}
}

// room flushes the buffer once it holds enough for the next value.
func (w *walker) room() {
	if w.h != nil && len(w.buf) >= 1<<15 {
		w.flush()
	}
}

func (w *walker) byte(b byte) { w.buf = append(w.buf, b) }

func (w *walker) uvarint(n uint64) { w.buf = binary.AppendUvarint(w.buf, n) }

func (w *walker) varint(n int64) { w.buf = binary.AppendVarint(w.buf, n) }

// str writes s, length-prefixed, so one string cannot run into the next.
func (w *walker) str(s string) {
	w.uvarint(uint64(len(s)))
	if w.h == nil {
		w.buf = append(w.buf, s...)
		return
	}
	for len(s) > 0 {
		if len(w.buf) == cap(w.buf) {
			w.flush()
		}
		n := min(len(s), cap(w.buf)-len(w.buf))
		w.buf = append(w.buf, s[:n]...)
		s = s[n:]
	}
}

// typeName is the full name of t: package path and name for a named type,
// else the type's own string, which spells out the types it is made of.
func typeName(t reflect.Type) string {
	if t.Name() != "" {
		return t.PkgPath() + "." + t.Name()
	}
	return t.String()
}

// typed writes t: its definition the first time, its number after.
func (w *walker) typed(t reflect.Type) {
	if n, ok := w.types[t]; ok {
		w.byte(tagType)
		w.uvarint(uint64(n))
		return
	}
	n := len(w.types)
	w.types[t] = n
	w.byte(tagDefine)
	w.uvarint(uint64(n))
	w.byte(byte(t.Kind()))
	w.str(typeName(t))
	switch t.Kind() {
	case reflect.Struct:
		w.uvarint(uint64(t.NumField()))
		for i := range t.NumField() {
			f := t.Field(i)
			w.str(f.Name)
			w.str(typeName(f.Type))
		}
	case reflect.Pointer, reflect.Slice:
		w.str(typeName(t.Elem()))
	case reflect.Array:
		w.uvarint(uint64(t.Len()))
		w.str(typeName(t.Elem()))
	case reflect.Map:
		w.str(typeName(t.Key()))
		w.str(typeName(t.Elem()))
	}
}

// enter notes a pointer-like value on the walk's path. When the value is
// already there it reports how to write it instead: as a cycle, or, for a
// value that is shared, as a reference to the number of its first occurrence.
func (w *walker) enter(k visit, shared bool) (tag byte, n int, first bool) {
	if w.visiting[k] {
		return tagCycle, w.seen[k], false
	}
	if shared {
		if n, ok := w.seen[k]; ok {
			return tagRef, n, false
		}
		n := len(w.seen)
		w.seen[k] = n
		w.visiting[k] = true
		return 0, n, true
	}
	w.visiting[k] = true
	return 0, 0, true
}

// walk writes v, with its type when v holds other values.
func (w *walker) walk(v reflect.Value) {
	w.room()
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Struct, reflect.Map, reflect.Slice, reflect.Array:
		w.typed(v.Type())
	}
	w.body(v)
}

// body writes v without its type.
func (w *walker) body(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			w.byte(tagNil)
			return
		}
		k := visit{p: v.Pointer(), t: v.Type()}
		tag, n, first := w.enter(k, true)
		if !first {
			w.byte(tag)
			w.uvarint(uint64(n))
			return
		}
		w.byte(tagPresent)
		w.walk(v.Elem())
		delete(w.visiting, k)
	case reflect.Interface:
		if v.IsNil() {
			w.byte(tagNil)
			return
		}
		w.byte(tagPresent)
		e := v.Elem()
		w.typed(e.Type())
		w.body(e)
	case reflect.Struct:
		for i := range v.NumField() {
			w.walk(v.Field(i))
		}
	case reflect.Map:
		if v.IsNil() {
			w.byte(tagNil)
			return
		}
		k := visit{p: v.Pointer(), t: v.Type()}
		tag, n, first := w.enter(k, true)
		if !first {
			w.byte(tag)
			w.uvarint(uint64(n))
			return
		}
		w.byte(tagPresent)
		w.uvarint(uint64(v.Len()))
		start := len(w.entries)
		w.sortedEntries(v)
		for _, e := range w.entries[start:] {
			w.key(e)
			w.walk(e.val)
		}
		w.entries = w.entries[:start]
		delete(w.visiting, k)
	case reflect.Slice:
		if v.IsNil() {
			w.byte(tagNil)
			return
		}
		if v.Len() > 0 {
			k := visit{p: v.Pointer(), t: v.Type(), n: v.Len()}
			tag, n, first := w.enter(k, false)
			if !first {
				w.byte(tag)
				w.uvarint(uint64(n))
				return
			}
			defer delete(w.visiting, k)
		}
		w.byte(tagPresent)
		w.uvarint(uint64(v.Len()))
		for i := range v.Len() {
			w.walk(v.Index(i))
		}
	case reflect.Array:
		for i := range v.Len() {
			w.walk(v.Index(i))
		}
	case reflect.String:
		w.str(v.String())
	case reflect.Bool:
		if v.Bool() {
			w.byte(1)
		} else {
			w.byte(0)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		w.varint(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		w.uvarint(v.Uint())
	case reflect.Float32, reflect.Float64:
		w.uvarint(math.Float64bits(v.Float()))
	case reflect.Complex64, reflect.Complex128:
		c := v.Complex()
		w.uvarint(math.Float64bits(real(c)))
		w.uvarint(math.Float64bits(imag(c)))
	default: // Func, Chan, UnsafePointer
		if v.IsNil() {
			w.byte(tagNil)
		} else {
			w.byte(tagPresent)
		}
	}
}

// entry is one entry of a map being walked: the key and, for a key that is no
// string, its encoding.
type entry struct {
	key reflect.Value
	enc string
	val reflect.Value
}

// sortedEntries appends the entries of the map v to w.entries, by key, so the
// order of iteration never shows in the stream.
func (w *walker) sortedEntries(v reflect.Value) {
	start := len(w.entries)
	stringKeys := v.Type().Key().Kind() == reflect.String
	for it := v.MapRange(); it.Next(); {
		e := entry{key: it.Key(), val: it.Value()}
		if !stringKeys {
			e.enc = encode(e.key)
		}
		w.entries = append(w.entries, e)
	}
	sorted := w.entries[start:]
	if stringKeys {
		slices.SortFunc(sorted, func(a, b entry) int { return strings.Compare(a.key.String(), b.key.String()) })
	} else {
		slices.SortFunc(sorted, func(a, b entry) int { return strings.Compare(a.enc, b.enc) })
	}
}

// key writes the key of e: its string, or the encoding of a key that is none.
func (w *walker) key(e entry) {
	if e.key.Kind() == reflect.String {
		w.str(e.key.String())
		return
	}
	w.str(e.enc)
}

// encode returns the canonical encoding of v alone, for a key that is no
// string. It is self-contained, with the type definitions and references of
// its own, so that it does not depend on the order the map is iterated in.
func encode(v reflect.Value) string {
	w := newWalker(nil)
	w.root(v)
	return string(w.buf)
}
