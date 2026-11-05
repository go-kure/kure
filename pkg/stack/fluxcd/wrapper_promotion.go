package fluxcd

import (
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"

	"github.com/go-kure/kure/pkg/errors"
)

// embedStep is one struct type reached through embedded fields: the type, the
// index of the field path that reaches it from the wrapper, and that path by
// field name.
type embedStep struct {
	typ   reflect.Type
	index []int
	path  string
}

// embeddedUnstructured returns the unstructured objects that r embeds, for a
// named struct that does: an unstructured.Unstructured or an
// unstructured.UnstructuredList, by pointer or by value, directly or through
// other embedded structs, so that its methods are promoted from it. For an
// Unstructured it returns that one object; for a List the List's own map as an
// object, then each of its items. None are returned, with no error, when r is no
// such wrapper. st is the type of the wrapper's struct.
//
// The object is the one Go promotes the methods of: the shallowest. Two at the
// shallowest depth promote neither, and the wrapper is refused naming both
// paths. A wrapper is also refused when the DeepCopyObject it has is caller
// code the layout would call: one it declares itself, or one that a struct
// embedded on the way to the object, or beside it at or above its depth,
// declares (isPromotion tells the compiler's wrapper from a method of the
// source). It is refused as well when it embeds a nil pointer, when the object
// cannot be reached. Where this cannot be told, the wrapper is refused.
func embeddedUnstructured(r apiruntime.Object) (objs []*unstructured.Unstructured, st reflect.Type, err error) {
	if r == nil {
		return nil, nil, nil
	}
	rv := reflect.ValueOf(r)
	// A typed nil is refused by callerObjects, with the type named.
	if rv.Kind() == reflect.Pointer && rv.IsNil() {
		return nil, nil, nil
	}
	t := rv.Type()
	st = t
	for st.Kind() == reflect.Pointer {
		st = st.Elem()
	}
	if st.Kind() != reflect.Struct || st == unstructuredType || st == unstructuredListType {
		return nil, nil, nil
	}
	found, above := shallowestEmbeds(st)
	if len(found) == 0 {
		return nil, nil, nil
	}
	if len(found) > 1 {
		paths := make([]string, len(found))
		for i, f := range found {
			paths[i] = f.path
		}
		return nil, st, errors.Errorf("a %s embeds an unstructured object at the same depth more than once (%s), so its methods are promoted from none of them", st, strings.Join(paths, " and "))
	}
	if declaresDeepCopyObject(st) {
		return nil, st, errors.Errorf("a %s embeds an unstructured object but has a DeepCopyObject of its own", st)
	}
	for _, a := range above {
		if declaresDeepCopyObject(a.typ) {
			return nil, st, errors.Errorf("a %s embeds an unstructured object, and %s, which it embeds, has a DeepCopyObject of its own", st, a.path)
		}
	}
	v := rv
	for v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	p, reached := reachEmbedded(v, found[0].index)
	if !reached {
		return nil, st, errors.Errorf("a %s embeds a nil *unstructured.Unstructured or an unstructured object that cannot be reached", st)
	}
	switch e := p.(type) {
	case *unstructured.Unstructured:
		return []*unstructured.Unstructured{e}, st, nil
	case *unstructured.UnstructuredList:
		// The list's own map, then its items, which it holds by value.
		objs = []*unstructured.Unstructured{{Object: e.Object}}
		for i := range e.Items {
			objs = append(objs, &e.Items[i])
		}
		return objs, st, nil
	}
	return nil, st, errors.Errorf("a %s embeds an unstructured object that cannot be reached", st)
}

// shallowestEmbeds looks through the embedded fields of the struct type st,
// level by level as Go does for a promoted method, and returns the embedded
// unstructured.Unstructured (by value or by pointer) at the shallowest depth
// that has any, and the embedded struct types at the depths above it and at its
// own. More than one is returned when several share that depth. None is
// returned when st embeds none.
//
// The search has no limit of its own: the number of steps is linear in the
// types st reaches. A type is followed at the depth it is first reached, and at
// most twice there, as a third path to it adds nothing to the two that already
// make it ambiguous. So a level holds at most two steps per type, and there are
// at most as many levels as types. Each step holds the path to it, so the
// memory is quadratic in the depth of embedding. That depth is the one of a Go
// type the caller wrote, and no value of the caller's data makes it larger.
func shallowestEmbeds(st reflect.Type) (found, above []embedStep) {
	level := []embedStep{{typ: st}}
	seen := map[reflect.Type]int{st: 0}
	for depth := 0; len(level) > 0; depth++ {
		var next []embedStep
		paths := map[reflect.Type]int{}
		for _, step := range level {
			for i := 0; i < step.typ.NumField(); i++ {
				f := step.typ.Field(i)
				if !f.Anonymous {
					continue
				}
				at := embedStep{index: append(append([]int(nil), step.index...), i), path: joinPath(step.path, f.Name)}
				ft := f.Type
				if ft.Kind() == reflect.Pointer {
					ft = ft.Elem()
				}
				switch {
				case ft == unstructuredType || ft == unstructuredListType:
					at.typ = ft
					found = append(found, at)
				case ft.Kind() == reflect.Struct:
					// A type already reached at a shallower depth, or the one being
					// searched, adds nothing deeper: Go ranks its methods by the
					// shallower embed. Following it would never end for a type that
					// embeds itself. At the same depth it is followed, since two
					// paths to one type are an ambiguity.
					if d, ok := seen[ft]; ok && d <= depth {
						continue
					} else if !ok {
						seen[ft] = depth + 1
					}
					if paths[ft]++; paths[ft] > 2 {
						continue
					}
					at.typ = ft
					next = append(next, at)
				}
			}
		}
		if len(found) > 0 {
			// A struct that is embedded at this depth declares methods Go ranks
			// with the object's, so it counts with the ones above.
			return found, append(above, next...)
		}
		above = append(above, next...)
		level = next
	}
	return nil, nil
}

// declaresDeepCopyObject reports whether the struct type st declares a
// DeepCopyObject in its source, with a value or a pointer receiver. The
// compiler writes a pointer-receiver wrapper for a value-receiver method, so
// the method set of the pointer type alone would hide the value one.
func declaresDeepCopyObject(st reflect.Type) bool {
	for _, t := range []reflect.Type{st, reflect.PointerTo(st)} {
		if m, ok := t.MethodByName("DeepCopyObject"); ok && !isPromotion(m.Func.Pointer()) {
			return true
		}
	}
	return false
}

// joinPath appends the field name to a path of field names.
func joinPath(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

// reachEmbedded returns the unstructured object the struct v holds at the field
// index path. It reports false when a pointer on the way is nil, when the
// object cannot be reached or when v is not addressable where the object is held
// by value.
//
// Only the object itself has to be interfaced. A field on the way may be
// unexported, as the embedded field of an unexported struct type is, and reflect
// still follows it.
func reachEmbedded(v reflect.Value, index []int) (any, bool) {
	for n, i := range index {
		fv := v.Field(i)
		last := n == len(index)-1
		if fv.Kind() == reflect.Pointer {
			if fv.IsNil() {
				return nil, false
			}
			if last {
				if !fv.CanInterface() {
					return nil, false
				}
				return fv.Interface(), true
			}
			fv = fv.Elem()
		} else if last {
			if !fv.CanAddr() || !fv.Addr().CanInterface() {
				return nil, false
			}
			return fv.Addr().Interface(), true
		}
		v = fv
	}
	return nil, false
}
