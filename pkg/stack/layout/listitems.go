package layout

import (
	"reflect"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

// typedListItems returns the items of a typed List as the writers serialize
// them, for listItems. An item held as a runtime.RawExtension is serialized
// from its raw JSON when it has any and from its object otherwise
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
