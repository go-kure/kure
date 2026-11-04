package built

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

// IsList tells a typed List, which is read from the items it writes, from a
// typed object that is no List and from an unstructured object.
func TestIsList(t *testing.T) {
	for name, tc := range map[string]struct {
		r    runtime.Object
		want bool
	}{
		"a typed List":                   {listHolding(configMap("a")), true},
		"an empty typed List":            {listHolding(), true},
		"a typed object that is no List": {typedConfigMap("a"), false},
		"an unstructured object":         {configMap("a"), false},
	} {
		got, err := IsList(tc.r)
		if err != nil || got != tc.want {
			t.Errorf("%s: IsList = %v, %v; want %v", name, got, err, tc.want)
		}
	}

	self := listHolding()
	self.Items = []runtime.Object{self}
	if _, err := IsList(self); err == nil {
		t.Error("IsList of a List that holds itself gave no error")
	}
}
