// Package built reads a resource as kustomize builds it once it is written:
// the object itself, or the objects a List holds. The layout writers'
// pre-write checks and the Flux integration read a resource through it, so
// they cannot disagree on which objects a resource holds.
package built

import (
	"encoding/json"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/go-kure/kure/pkg/errors"
)

// Object is one object kustomize builds from a resource.
type Object struct {
	// Object is the resource itself, or an item of a List it is or holds. An
	// item of a typed List can be any runtime.Object, so it need not carry
	// object metadata.
	runtime.Object
	// Read is set when the object is not a Go value the resource holds but
	// was read from what the resource writes in its place: an item a typed
	// List holds as raw JSON, an item of a typed object that writes an items
	// field its Go value gives no access to, and whatever such an item holds.
	// It has the content that is written, and a change to it is not a change
	// to the resource.
	Read bool
}

// Objects returns the objects kustomize builds from r once it is written: r
// itself, or the objects a List holds, a List among them opened the same way.
//
// A List is what kustomize takes for one (its resource factory's
// inlineAnyEmbeddedLists): an object whose kind ends in "List" and that has
// an items field. Any other kind is one object, whatever fields it has, and
// so is a kind ending in "List" without items. A List whose items are null
// holds nothing; one whose items are not an array fails the kustomize build
// and is returned as the one object it is, since there is nothing in it to
// read.
//
// For a typed object the items field is read from its written form, the JSON
// the writers marshal it to, so a field left out when empty counts as absent,
// and one written as null or as something that is no array is read as that,
// whatever the Go value holds beside it. The items are the ones its Go value
// gives access to, read as the writers serialize them (typedListItems): an
// item held as raw JSON is the object that JSON encodes, and an empty item
// holds nothing, as does one that is a nil pointer, which is written as null.
// A List among them need not carry object metadata (metav1.List has none) to
// be opened. A typed object that writes items its Go value gives no access
// to, from a field apimachinery does not take for a list's items, has them
// read from the written form.
//
// The items of an unstructured List are the List's own: a change to one is a
// change to the List. They are read from its map and not from a written form,
// so one that holds itself, directly or through the Lists it holds, would be
// opened without end: it is refused.
func Objects(r runtime.Object) ([]Object, error) {
	return objects(Object{Object: r}, map[uintptr]bool{})
}

// objects is Objects for one object. open has the unstructured Lists that are
// being opened around it, each by the map it holds its content in.
func objects(obj Object, open map[uintptr]bool) ([]Object, error) {
	items, isList, err := listItems(obj.Object)
	if err != nil {
		return nil, err
	}
	if !isList {
		return []Object{obj}, nil
	}
	if u, ok := obj.Object.(*unstructured.Unstructured); ok {
		content := reflect.ValueOf(u.Object).Pointer()
		if open[content] {
			return nil, errors.Errorf("a %s holds itself among its items", u.GetKind())
		}
		open[content] = true
		defer delete(open, content)
	}
	var out []Object
	for _, item := range items {
		// What a copy holds is a copy.
		item.Read = item.Read || obj.Read
		held, err := objects(item, open)
		if err != nil {
			return nil, err
		}
		out = append(out, held...)
	}
	return out, nil
}

// listItems returns the items of obj when it is a List as Objects defines
// one, and whether it is.
func listItems(obj runtime.Object) (items []Object, isList bool, err error) {
	if !strings.HasSuffix(obj.GetObjectKind().GroupVersionKind().Kind, "List") {
		return nil, false, nil
	}
	if u, ok := obj.(*unstructured.Unstructured); ok {
		return unstructuredItems(u)
	}
	written, err := writtenForm(obj)
	if err != nil {
		return nil, false, err
	}
	held, has := written["items"]
	if !has {
		return nil, false, nil
	}
	if !meta.IsListType(obj) {
		items, isList, err = unstructuredItems(&unstructured.Unstructured{Object: written})
		for i := range items {
			items[i].Read = true
		}
		return items, isList, err
	}
	extracted, err := typedListItems(obj)
	if err != nil {
		return nil, false, err
	}
	// What is written under items decides, as it does for an unstructured
	// List: the Go value's items are the List's only where an array is
	// written.
	switch held.(type) {
	case nil:
		return nil, true, nil
	case []any:
	default:
		return nil, false, nil
	}
	for _, item := range extracted {
		if empty(item) {
			continue
		}
		switch o := item.(type) {
		case *runtime.Unknown:
			u := &unstructured.Unstructured{}
			if err := json.Unmarshal(o.Raw, &u.Object); err != nil {
				return nil, false, errors.Wrap(err, "read a raw JSON item")
			}
			if u.Object != nil {
				items = append(items, Object{Object: u, Read: true})
			}
		default:
			items = append(items, Object{Object: o})
		}
	}
	return items, true, nil
}

// empty reports whether an item of a typed List holds no object: no item, or
// a nil pointer, which the writers serialize as null like an item without
// content.
func empty(item runtime.Object) bool {
	if item == nil {
		return true
	}
	v := reflect.ValueOf(item)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

// unstructuredItems is listItems for an unstructured object whose kind ends
// in "List".
func unstructuredItems(u *unstructured.Unstructured) (items []Object, isList bool, err error) {
	held, has := u.Object["items"]
	switch {
	case !has:
		return nil, false, nil
	case held == nil:
		return nil, true, nil
	case !u.IsList():
		return nil, false, nil
	}
	list, err := u.ToList()
	if err != nil {
		return nil, false, err
	}
	for i := range list.Items {
		items = append(items, Object{Object: &list.Items[i]})
	}
	return items, true, nil
}

// writtenForm returns a typed object as it is written: the writers marshal an
// object to JSON first.
func writtenForm(obj runtime.Object) (map[string]any, error) {
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, errors.Wrapf(err, "marshal the %s", kind)
	}
	var written map[string]any
	if err := json.Unmarshal(data, &written); err != nil {
		return nil, errors.Wrapf(err, "read the %s as written", kind)
	}
	return written, nil
}

// typedListItems returns the items of a typed List as the writers serialize
// them. An item held as a runtime.RawExtension is serialized from its raw
// JSON when it has any and from its object otherwise
// (RawExtension.MarshalJSON), so it is read in that order: meta.ExtractList
// reads the object first, and would name an object the written file does not
// hold when the two differ. The raw JSON is returned as a runtime.Unknown.
//
// meta.ExtractList reads the list, so whatever it accepts as a list of items
// is accepted here and whatever it refuses (an Items pointer that is nil,
// among others) is refused with its error. It decides by the element type of
// the Items slice, and so does the correction: a named slice type and a
// pointer to the slice are read alike. An empty item is a nil entry.
func typedListItems(list runtime.Object) ([]runtime.Object, error) {
	items, err := meta.ExtractList(list)
	if err != nil || len(items) == 0 {
		return items, err
	}
	ptr, err := meta.GetItemsPtr(list)
	if err != nil {
		return nil, err
	}
	slice := reflect.ValueOf(ptr).Elem()
	if slice.Type().Elem() != reflect.TypeFor[runtime.RawExtension]() {
		return items, nil
	}
	for i := range items {
		if raw := slice.Index(i).Interface().(runtime.RawExtension).Raw; raw != nil {
			items[i] = &runtime.Unknown{Raw: raw}
		}
	}
	return items, nil
}
