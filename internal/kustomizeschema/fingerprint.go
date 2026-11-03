package kustomizeschema

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"maps"
	"math"
	"reflect"
	"slices"
	"strconv"
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
// type is met it is defined once, by its kind and full name (for a named type
// its kind, package path and name, so it never reads as its underlying one:
// Strategy("merge") is no "merge", and two types of one name whose kinds
// differ are told apart), and for a struct by each field's name and type; every later
// use writes the number the definition got. A pointer, interface, struct, map,
// slice or array value is written with the number of its type wherever it sits,
// and an interface's dynamic value with the number of its dynamic type, so 1
// and 1.0 differ; a scalar, func or chan value is written without one, as the
// definition of the field, element, key or value it sits in names its type. A value moved to another field changes the stream, because a struct
// writes its fields in the order its definition lists them. A map is walked in
// key order, its nil and empty forms told apart. A pointer, map or non-empty
// slice met a second time (a slice is the same value by its address, type and
// length) is written as a reference to its first occurrence, so a schema that
// two parents share is read once and a slice shared many times over stays
// linear; its sharing is part of the stream, which makes a replacement by an
// equal copy a difference, not a silent equality. A pointer,
// map or slice that leads back to a value on its path is written as a cycle to
// that value's depth, also when the way back is through a map key. A function or
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
	seen     map[visit]int // pointers, maps and slices met so far, by the number of their first occurrence
	visiting map[visit]int // the values on the walk's path, by their depth on it
	entries  []entry
}

func newWalker(h hash.Hash) *walker {
	return &walker{h: h, buf: make([]byte, 0, 1<<16), types: map[reflect.Type]int{}, seen: map[visit]int{}, visiting: map[visit]int{}}
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

// count writes a length, a field count or a number, none of which is negative.
func (w *walker) count(n int) {
	w.uvarint(uint64(n)) //nolint:gosec // n is a length, count or index: never negative
}

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

// typeName is the full name of t, which says what t is made of: for a named
// type its kind, package path and name (two types of one name whose kinds
// differ are told apart, and neither reads as its underlying type); for a
// pointer, slice, array, map, chan, func, struct or interface the names of the
// types it holds (Go's own string for such a type names a package by its short
// name and never says the kind of a named type inside it). Each recurses only
// as far as a named type, so it ends.
func typeName(t reflect.Type) string {
	if t.Name() != "" {
		if t.PkgPath() == "" {
			return t.Name()
		}
		return t.Kind().String() + " " + t.PkgPath() + "." + t.Name()
	}
	switch t.Kind() {
	case reflect.Pointer:
		return "*" + typeName(t.Elem())
	case reflect.Slice:
		return "[]" + typeName(t.Elem())
	case reflect.Array:
		return "[" + strconv.Itoa(t.Len()) + "]" + typeName(t.Elem())
	case reflect.Map:
		return "map[" + typeName(t.Key()) + "]" + typeName(t.Elem())
	case reflect.Chan:
		return t.ChanDir().String() + " " + typeName(t.Elem())
	case reflect.Func:
		return funcName(t)
	case reflect.Struct:
		var b strings.Builder
		b.WriteString("struct{")
		for i := range t.NumField() {
			f := t.Field(i)
			if i > 0 {
				b.WriteString("; ")
			}
			if f.Anonymous {
				b.WriteString("embedded ")
			}
			// An unexported name belongs to the package that wrote it.
			b.WriteString(strconv.Quote(f.PkgPath + "." + f.Name))
			b.WriteString(" " + typeName(f.Type))
			if f.Tag != "" {
				b.WriteString(" " + strconv.Quote(string(f.Tag)))
			}
		}
		b.WriteString("}")
		return b.String()
	case reflect.Interface:
		var b strings.Builder
		b.WriteString("interface{")
		for i := range t.NumMethod() {
			m := t.Method(i)
			if i > 0 {
				b.WriteString("; ")
			}
			b.WriteString(strconv.Quote(m.PkgPath+"."+m.Name) + funcName(m.Type)[len("func"):])
		}
		b.WriteString("}")
		return b.String()
	default:
		return t.String()
	}
}

// funcName names a func type by what its parameters and results are.
func funcName(t reflect.Type) string {
	var b strings.Builder
	b.WriteString("func(")
	for i := range t.NumIn() {
		if i > 0 {
			b.WriteString(", ")
		}
		if t.IsVariadic() && i == t.NumIn()-1 {
			b.WriteString("..." + typeName(t.In(i).Elem()))
		} else {
			b.WriteString(typeName(t.In(i)))
		}
	}
	b.WriteString(")(")
	for i := range t.NumOut() {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(typeName(t.Out(i)))
	}
	b.WriteString(")")
	return b.String()
}

// typed writes t: its definition the first time, its number after.
func (w *walker) typed(t reflect.Type) {
	if n, ok := w.types[t]; ok {
		w.byte(tagType)
		w.count(n)
		return
	}
	n := len(w.types)
	w.types[t] = n
	w.byte(tagDefine)
	w.count(n)
	w.uvarint(uint64(t.Kind())) // a kind is below 128: one byte
	w.str(typeName(t))
	switch t.Kind() {
	case reflect.Struct:
		w.count(t.NumField())
		for i := range t.NumField() {
			f := t.Field(i)
			w.str(f.Name)
			w.str(typeName(f.Type))
		}
	case reflect.Pointer, reflect.Slice:
		w.str(typeName(t.Elem()))
	case reflect.Array:
		w.count(t.Len())
		w.str(typeName(t.Elem()))
	case reflect.Map:
		w.str(typeName(t.Key()))
		w.str(typeName(t.Elem()))
	default: // a scalar, func, chan or interface: its kind and name say all
	}
}

// enter notes a pointer-like value on the walk's path. When the value is
// already there it reports how to write it instead: as a cycle, with the depth
// on the path of the value it returns to, or, for a value that is shared (a
// pointer, a map or a non-empty slice), as a reference to the number of its
// first occurrence.
func (w *walker) enter(k visit, shared bool) (tag byte, n int, first bool) {
	if d, ok := w.visiting[k]; ok {
		return tagCycle, d, false
	}
	if shared {
		if n, ok := w.seen[k]; ok {
			return tagRef, n, false
		}
		w.seen[k] = len(w.seen)
	}
	w.visiting[k] = len(w.visiting)
	return 0, 0, true
}

// walk writes v, with its type when v holds other values.
func (w *walker) walk(v reflect.Value) {
	w.room()
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Struct, reflect.Map, reflect.Slice, reflect.Array:
		w.typed(v.Type())
	default: // a scalar, func or chan holds no other value: no type
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
			w.count(n)
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
			w.count(n)
			return
		}
		w.byte(tagPresent)
		w.count(v.Len())
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
			tag, n, first := w.enter(k, true)
			if !first {
				w.byte(tag)
				w.count(n)
				return
			}
			defer delete(w.visiting, k)
		}
		w.byte(tagPresent)
		w.count(v.Len())
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
			e.enc = w.encode(e.key)
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
// its own, so that it does not depend on the order the map is iterated in. It
// starts on the path of the walk that calls it, so a key that leads back to a
// value on that path is written as a cycle to its depth, not walked again.
// Keys of equal encoding but different identity, such as two pointers to equal
// values, sort in no fixed order: the fingerprint of such a map may differ
// between calls, which only refuses.
func (w *walker) encode(v reflect.Value) string {
	e := newWalker(nil)
	e.buf = make([]byte, 0, 256)
	maps.Copy(e.visiting, w.visiting)
	e.root(v)
	return string(e.buf)
}
