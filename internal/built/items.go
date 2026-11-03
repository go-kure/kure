package built

import (
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/go-kure/kure/pkg/errors"
)

var (
	objectType       = reflect.TypeFor[runtime.Object]()
	rawExtensionType = reflect.TypeFor[runtime.RawExtension]()
)

// goItems returns the items of obj where apimachinery reads it as a typed List
// (a pointer to a struct with an Items field that is a slice, or a pointer to
// one, which meta.IsListType and meta.ExtractList decide), and whether it does.
// It does the same by reflection alone: it calls no method of obj and formats
// nothing it holds, as meta.ExtractList formats the whole object when an item
// is no object, which has no end for an object that holds itself in a private
// field (go-kure/kure#1201).
//
// An object that is no pointer to a struct, one whose Items is not reached
// (a nil embedded pointer on the way, one that is not exported, an interface
// that is nil or holds no pointer to a slice) is no List it can read, and is
// read from what it writes, as any typed object apimachinery takes for no
// List. A nil Items pointer, and an item that is no object, are refused.
//
// An item held as a runtime.RawExtension is read from its raw JSON when it has
// any and from its object otherwise, as RawExtension.MarshalJSON writes it,
// and the raw JSON is returned as a runtime.Unknown. A nil entry of an Items
// slice that is an interface and nil is a nil item, kept as one, and reported
// by nilEntry: apimachinery panics on it, so a List that holds one is no List
// it can read (listItems).
func goItems(obj runtime.Object) (items []runtime.Object, isList, nilEntry bool, err error) {
	slice, isList, err := itemsSlice(obj)
	if !isList || err != nil {
		return nil, isList, false, err
	}
	if !slice.IsValid() || slice.IsNil() || slice.Len() == 0 {
		return nil, true, false, nil
	}
	elemType := slice.Type().Elem()
	isRaw := elemType == rawExtensionType
	implementsObject := elemType.Implements(objectType)
	items = make([]runtime.Object, slice.Len())
	for i := range items {
		elem := slice.Index(i)
		switch {
		case isRaw:
			ext := elem.Interface().(runtime.RawExtension)
			switch {
			case ext.Raw != nil:
				items[i] = &runtime.Unknown{Raw: ext.Raw}
			default:
				items[i] = ext.Object
			}
		case implementsObject:
			if elem.Kind() == reflect.Interface && elem.IsNil() {
				nilEntry = true
				continue
			}
			items[i] = elem.Interface().(runtime.Object)
		default:
			o, ok := elem.Addr().Interface().(runtime.Object)
			if !ok {
				return nil, true, false, errors.Errorf("a %T is a List whose items are %s, not objects", obj, elemType)
			}
			items[i] = o
		}
	}
	return items, true, nilEntry, nil
}

// itemsSlice returns the Items slice of obj, as meta.GetItemsPtr finds it, and
// whether obj is a List. The slice is not valid for a List that holds no
// slice (a nil pointer to one is refused).
func itemsSlice(obj runtime.Object) (slice reflect.Value, isList bool, err error) {
	v := reflect.ValueOf(obj)
	if v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return reflect.Value{}, false, nil
	}
	e := v.Elem()
	field, found := e.Type().FieldByName("Items")
	if !found {
		return reflect.Value{}, false, nil
	}
	items, reached := reachField(e, field.Index)
	if !reached {
		return reflect.Value{}, false, nil
	}
	switch items.Kind() {
	case reflect.Slice:
		return items, true, nil
	case reflect.Interface:
		// A pointer to a slice, held as an interface.
		if items.IsNil() {
			return reflect.Value{}, false, nil
		}
		items = items.Elem()
		if items.Kind() != reflect.Pointer {
			return reflect.Value{}, false, nil
		}
		fallthrough
	case reflect.Pointer:
		if items.Type().Elem().Kind() != reflect.Slice {
			return reflect.Value{}, false, nil
		}
		if items.IsNil() {
			return reflect.Value{}, true, errors.Errorf("a %T is a List whose Items pointer is nil", obj)
		}
		return items.Elem(), true, nil
	default:
		return reflect.Value{}, false, nil
	}
}

// reachField returns the field of e at index, and whether it is reached: a nil
// embedded pointer on the way, or a field that cannot be read, is not.
func reachField(e reflect.Value, index []int) (reflect.Value, bool) {
	field, err := e.FieldByIndexErr(index)
	return field, err == nil && field.CanInterface()
}
