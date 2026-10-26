package stacktest

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
)

var (
	timeType   = reflect.TypeFor[time.Time]()
	objectType = reflect.TypeFor[client.Object]()
)

// path is a walk path as its segments ("[0]", ".Spec", ".(*v1.Pod)"), so
// whether one path lies inside another does not depend on what a map key
// spells.
type path []string

// child returns p with seg appended, never sharing p's storage.
func (p path) child(seg string) path {
	out := make(path, len(p), len(p)+1)
	copy(out, p)
	return append(out, seg)
}

func (p path) String() string { return strings.Join(p, "") }

// extends reports whether p is q or lies inside the value at q.
func (p path) extends(q path) bool {
	if len(p) < len(q) {
		return false
	}
	for i := range q {
		if p[i] != q[i] {
			return false
		}
	}
	return true
}

// record is one encounter of storage during a walk: a pointer's pointee or a
// slice's backing array as the half-open range [start, end), or a map by
// identity (start is the map's pointer, end is unused).
type record struct {
	start, end uintptr
	isMap      bool
	kind       string
	path       path
	// first is false for a second encounter of the same storage, which is
	// not walked again.
	first bool
}

// visitKey identifies a value for traversal. A slice view adds its length
// and capacity: a longer view must still be walked, and a wider one still
// recorded as a first encounter.
type visitKey struct {
	kind     reflect.Kind
	addr     uintptr
	typ      reflect.Type
	len, cap int
}

// walk collects the records of one result in walk order, which is depth
// first, so a value is recorded before anything inside it.
type walk struct {
	// result keeps the walked result reachable: the records hold bare
	// addresses, and storage freed before the comparison could be reused
	// by the other call and reported as shared.
	result  []*client.Object
	records []record
	visited map[visitKey]bool
	// active holds the keys being walked, so a value that contains itself
	// is not recorded as a second path to itself.
	active map[visitKey]bool
}

func walkResult(objs []*client.Object) (*walk, error) {
	w := &walk{result: objs, visited: map[visitKey]bool{}, active: map[visitKey]bool{}}
	if err := w.value(reflect.ValueOf(objs), path{}); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *walk) value(v reflect.Value, p path) error {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return nil
		}
		start := v.Pointer()
		elem := v.Type().Elem()
		key := visitKey{kind: reflect.Pointer, addr: start, typ: elem}
		return w.storage(key, record{start: start, end: start + elem.Size(), kind: pointerKind(v.Type()), path: p}, func() error {
			return w.value(v.Elem(), p)
		})
	case reflect.Slice:
		if v.IsNil() || v.Cap() == 0 {
			return nil
		}
		start := v.Pointer()
		key := visitKey{kind: reflect.Slice, addr: start, typ: v.Type(), len: v.Len(), cap: v.Cap()}
		end := start + uintptr(v.Cap())*v.Type().Elem().Size()
		return w.storage(key, record{start: start, end: end, kind: "slice", path: p}, func() error {
			for i := 0; i < v.Len(); i++ {
				if err := w.value(v.Index(i), p.child("["+strconv.Itoa(i)+"]")); err != nil {
					return err
				}
			}
			return nil
		})
	case reflect.Map:
		if v.IsNil() {
			return nil
		}
		start := v.Pointer()
		key := visitKey{kind: reflect.Map, addr: start, typ: v.Type()}
		return w.storage(key, record{start: start, isMap: true, kind: "map", path: p}, func() error {
			// Two keys that print alike (two NaNs, or 1 and "1" in a
			// map[any]any) would give two values one path, and which is
			// walked first is up to MapRange: refused, like a func, and
			// before any entry is walked, so the error does not depend on
			// that order either.
			entries := sortedEntries(v)
			texts := make([]string, len(entries))
			seen := map[string]bool{}
			for i, e := range entries {
				texts[i] = "[" + fmt.Sprint(e.key) + "]"
				if seen[texts[i]] {
					return errors.Errorf("unsupported map at %q: two keys print as %s", p.String(), texts[i])
				}
				seen[texts[i]] = true
			}
			for i, e := range entries {
				kp := p.child(texts[i])
				if err := w.value(e.key, kp); err != nil {
					return err
				}
				if err := w.value(e.value, kp); err != nil {
					return err
				}
			}
			return nil
		})
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if err := w.value(v.Index(i), p.child("["+strconv.Itoa(i)+"]")); err != nil {
				return err
			}
		}
	case reflect.Struct:
		if v.Type() == timeType {
			return nil
		}
		for i := 0; i < v.NumField(); i++ {
			if err := w.value(v.Field(i), p.child("."+v.Type().Field(i).Name)); err != nil {
				return err
			}
		}
	case reflect.Interface:
		if v.IsNil() {
			return nil
		}
		elem := v.Elem()
		return w.value(elem, p.child(".("+elem.Type().String()+")"))
	case reflect.Func, reflect.Chan, reflect.UnsafePointer:
		if !v.IsNil() {
			return errors.Errorf("unsupported %s value at %q", v.Kind(), p.String())
		}
	default:
		// Strings, numbers and bools: nothing a write could reach through.
	}
	return nil
}

// storage records r and walks inside it on its first encounter only. A key
// already being walked is a cycle: neither recorded nor walked again.
func (w *walk) storage(key visitKey, r record, inside func() error) error {
	if w.active[key] {
		return nil
	}
	if !r.isMap && r.start == r.end {
		// A zero-size pointee: Go may place separate allocations at one
		// address, and there is nothing inside to share.
		return nil
	}
	r.first = !w.visited[key]
	w.records = append(w.records, r)
	if !r.first {
		return nil
	}
	w.visited[key] = true
	w.active[key] = true
	defer delete(w.active, key)
	return inside()
}

func pointerKind(t reflect.Type) string {
	if t.Elem() == objectType || t.Implements(objectType) {
		return "object"
	}
	return "pointer"
}

type entry struct{ key, value reflect.Value }

// sortedEntries returns a map's entries ordered by key: by value for string,
// integer and bool kinds and by formatted value otherwise, so the path kept
// for a value is stable. Entries are read with MapRange, which also returns
// those under a NaN key that MapIndex cannot look up.
func sortedEntries(v reflect.Value) []entry {
	var entries []entry
	for it := v.MapRange(); it.Next(); {
		entries = append(entries, entry{it.Key(), it.Value()})
	}
	var less func(a, b reflect.Value) bool
	switch v.Type().Key().Kind() {
	case reflect.String:
		less = func(a, b reflect.Value) bool { return a.String() < b.String() }
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		less = func(a, b reflect.Value) bool { return a.Int() < b.Int() }
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		less = func(a, b reflect.Value) bool { return a.Uint() < b.Uint() }
	case reflect.Bool:
		less = func(a, b reflect.Value) bool { return !a.Bool() && b.Bool() }
	default:
		less = func(a, b reflect.Value) bool { return fmt.Sprint(a) < fmt.Sprint(b) }
	}
	sort.SliceStable(entries, func(i, j int) bool { return less(entries[i].key, entries[j].key) })
	return entries
}

// overlaps reports whether a and b are the same map, or two storage ranges
// with at least one byte in common.
func overlaps(a, b record) bool {
	if a.isMap || b.isMap {
		return a.isMap && b.isMap && a.start == b.start
	}
	return a.start < b.end && b.start < a.end
}

// match is a pair of overlapping records, by index into their walks.
type match struct{ i, j int }

// matches returns every pair (i in a, j in b) of overlapping records, sorted
// by i then j. Ranges are matched by a scan of b sorted by start, cut short
// by the largest end among the records before each point; maps by identity.
func matches(a, b []record) []match {
	var ranges []int
	maps := map[uintptr][]int{}
	for j, r := range b {
		if r.isMap {
			maps[r.start] = append(maps[r.start], j)
		} else {
			ranges = append(ranges, j)
		}
	}
	sort.SliceStable(ranges, func(x, y int) bool { return b[ranges[x]].start < b[ranges[y]].start })
	maxEnd := make([]uintptr, len(ranges))
	for x, j := range ranges {
		maxEnd[x] = b[j].end
		if x > 0 && maxEnd[x-1] > maxEnd[x] {
			maxEnd[x] = maxEnd[x-1]
		}
	}

	var out []match
	for i, r := range a {
		if r.isMap {
			for _, j := range maps[r.start] {
				out = append(out, match{i, j})
			}
			continue
		}
		// The last range starting before r ends; scan back while some
		// range at or before x can still reach past r's start.
		x := sort.Search(len(ranges), func(x int) bool { return b[ranges[x]].start >= r.end }) - 1
		for ; x >= 0 && maxEnd[x] > r.start; x-- {
			if j := ranges[x]; overlaps(r, b[j]) {
				out = append(out, match{i, j})
			}
		}
	}
	sort.Slice(out, func(x, y int) bool {
		if out[x].i != out[y].i {
			return out[x].i < out[y].i
		}
		return out[x].j < out[y].j
	})
	return out
}

// report turns matches into Sharings, in walk order of the first side so a
// value is considered before anything inside it. A match whose two paths
// both extend the two paths of a reported one is inside that one and is
// skipped. With oncePerSecond, a record of the second side is considered
// once, against the first record it overlaps: if that match is inside a
// reported one, the record is covered and not reported against another.
func report(a, b []record, ms []match, oncePerSecond bool) []Sharing {
	var out []Sharing
	var done []match
	considered := map[int]bool{}
	for _, m := range ms {
		if oncePerSecond {
			if considered[m.j] {
				continue
			}
			considered[m.j] = true
		}
		first, second := a[m.i].path, b[m.j].path
		inside := false
		for _, d := range done {
			if first.extends(a[d.i].path) && second.extends(b[d.j].path) {
				inside = true
				break
			}
		}
		if inside {
			continue
		}
		done = append(done, m)
		out = append(out, Sharing{Kind: a[m.i].kind, First: first.String(), Second: second.String()})
	}
	return out
}

func sortSharings(s []Sharing) {
	sort.SliceStable(s, func(x, y int) bool {
		if s[x].First != s[y].First {
			return s[x].First < s[y].First
		}
		return s[x].Second < s[y].Second
	})
}

// shared reports the storage the two results share, comparing the first
// encounter of each value on each side.
func shared(a, b *walk) []Sharing {
	fa, fb := firsts(a.records), firsts(b.records)
	out := report(fa, fb, matches(fa, fb), false)
	sortSharings(out)
	return out
}

func firsts(rs []record) []record {
	var out []record
	for _, r := range rs {
		if r.first {
			out = append(out, r)
		}
	}
	return out
}

// internal reports the storage each result reaches at two paths, each later
// path against the earliest it overlaps, listing a value found in both
// results once.
func internal(walks ...*walk) []Sharing {
	var out []Sharing
	seen := map[Sharing]bool{}
	for _, w := range walks {
		var ms []match
		for _, m := range matches(w.records, w.records) {
			if m.i < m.j {
				ms = append(ms, m)
			}
		}
		for _, s := range report(w.records, w.records, ms, true) {
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	sortSharings(out)
	return out
}
