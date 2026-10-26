// Package nilobj tells whether a client.Object holds no object. Bundle.Generate
// and the layout walk ask it of every entry an application's Generate returns,
// so they cannot disagree on which entries hold nothing.
package nilobj

import (
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Is reports whether o holds no object: o is nil, or holds a nil pointer of an
// object type, such as client.Object((*corev1.ConfigMap)(nil)). Such a pointer
// is no nil interface, so o == nil misses it, and every method that reads the
// object (GetName, GetLabels, ...) dereferences it.
func Is(o client.Object) bool {
	if o == nil {
		return true
	}
	v := reflect.ValueOf(o)
	return v.Kind() == reflect.Pointer && v.IsNil()
}
