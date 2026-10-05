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
// be opened. A typed object that writes its items from a field apimachinery
// does not take for a list's items, so that its Go value gives no access to
// them or gives other ones, has them read from the written form (writtenAs).
//
// The items of an unstructured List are the List's own: a change to one is a
// change to the List. They are read from its map and not from a written form,
// so one that holds itself, directly or through the Lists it holds, would be
// opened without end: it is refused. So is a typed List that holds itself as
// the object of an item, which cannot be marshalled (refuseItemCycle).
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
	if err := refuseItemCycle(obj, map[listRef]bool{}); err != nil {
		return nil, false, err
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
		return readItems(written)
	}
	extracted, err := typedListItems(obj)
	if err != nil {
		return nil, false, err
	}
	// What is written under items decides, as it does for an unstructured
	// List: the Go value's items are the List's only where they are what is
	// written there.
	if !writtenAs(extracted, held) {
		return readItems(written)
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

// readItems is listItems on the written form of a typed object whose kind
// ends in "List": the items are read from it, so they are copies.
func readItems(written map[string]any) (items []Object, isList bool, err error) {
	items, isList, err = unstructuredItems(&unstructured.Unstructured{Object: written})
	for i := range items {
		items[i].Read = true
	}
	return items, isList, err
}

// writtenAs reports whether held, what a typed List writes under items, is
// an array of exactly the items its Go value gives: as many, and each written
// as the List writes it. It is not for a List that writes null, something
// that is no array, or an array it takes from another field.
//
// The same content written from another field cannot be told from the
// field's own. A change to such an item does not reach what is written, which
// the delivery intent finds when it verifies the written form afterwards.
func writtenAs(items []runtime.Object, held any) bool {
	written, ok := held.([]any)
	if !ok || len(written) != len(items) {
		return false
	}
	for i, item := range items {
		data := []byte("null")
		if raw, ok := item.(*runtime.Unknown); ok {
			data = raw.Raw
		} else if !empty(item) {
			var err error
			if data, err = json.Marshal(item); err != nil {
				return false
			}
		}
		var own any
		if err := json.Unmarshal(data, &own); err != nil || !reflect.DeepEqual(own, written[i]) {
			return false
		}
	}
	return true
}

// listRef names a typed List by its type and address.
type listRef struct {
	typ reflect.Type
	ptr uintptr
}

// refuseItemCycle fails when a typed List holds itself among its items,
// directly or through the typed Lists it holds. Such a List has no written
// form: an item held as an object is marshalled by a call of its own
// (RawExtension.MarshalJSON), so the encoder never sees the whole path and
// the marshalling does not end. open has the Lists being read around obj.
//
// Only what a List holds as its items is followed, read as the writers
// serialize it, so an object beside an item's raw JSON is not. A value that
// reaches itself another way is not looked for, here as in the writers.
func refuseItemCycle(obj runtime.Object, open map[listRef]bool) error {
	if _, ok := obj.(*unstructured.Unstructured); ok || !meta.IsListType(obj) {
		return nil
	}
	v := reflect.ValueOf(obj)
	if v.Kind() != reflect.Pointer {
		return nil
	}
	ref := listRef{typ: v.Type(), ptr: v.Pointer()}
	if open[ref] {
		kind := obj.GetObjectKind().GroupVersionKind().Kind
		if kind == "" {
			kind = "List"
		}
		return errors.Errorf("a %s holds itself among its items", kind)
	}
	items, err := typedListItems(obj)
	if err != nil {
		// Refused where the List is read.
		return nil
	}
	open[ref] = true
	defer delete(open, ref)
	for _, item := range items {
		if empty(item) {
			continue
		}
		if err := refuseItemCycle(item, open); err != nil {
			return err
		}
	}
	return nil
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
