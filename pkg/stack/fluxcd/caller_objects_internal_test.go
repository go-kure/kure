package fluxcd

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack/layout"
)

// jsonTyped accepts exactly what runtime.DeepCopyJSONValue accepts. A wider
// check calls an object clean that apimachinery then panics on; a narrower
// one rewrites objects it need not.
func TestJSONTyped_MirrorsDeepCopyJSONValue(t *testing.T) {
	values := map[string]any{
		"string":       "x",
		"int64":        int64(1),
		"float64":      1.5,
		"bool":         true,
		"nil":          nil,
		"json.Number":  json.Number("1"),
		"map":          map[string]any{"a": int64(1), "b": []any{"c", nil}},
		"nil map":      map[string]any(nil),
		"slice":        []any{int64(1), map[string]any{}},
		"int":          1,
		"int32":        int32(1),
		"uint":         uint(1),
		"float32":      float32(1),
		"[]string":     []string{"a"},
		"map[string]s": map[string]string{"app": "x"},
		"nested int":   map[string]any{"a": []any{map[string]any{"b": 1}}},
		"struct":       struct{ A int }{1},
	}
	for name, v := range values {
		t.Run(name, func(t *testing.T) {
			var accepted bool
			func() {
				defer func() { accepted = recover() == nil }()
				runtime.DeepCopyJSONValue(v)
			}()
			if got := jsonTyped(v); got != accepted {
				t.Errorf("jsonTyped(%#v) = %v, but DeepCopyJSONValue accepts it: %v", v, got, accepted)
			}
		})
	}
}

func TestNormalizeCallerObjects(t *testing.T) {
	object := func(spec map[string]any) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1", "kind": "ConfigMap",
			"metadata": map[string]any{"name": "cm", "namespace": "default"},
			"spec":     spec,
		}}
	}
	layoutOf := func(objs ...client.Object) *layout.ManifestLayout {
		return &layout.ManifestLayout{Name: "l", Resources: objs}
	}

	t.Run("integers become int64 and keep their exact value", func(t *testing.T) {
		const big = int64(1<<53 + 1)
		u := object(map[string]any{"n": 53, "big": int(big), "u": uint(7), "f32": float32(2.5), "i32": int32(-3), "huge": uint64(math.MaxInt64)})
		if err := normalizeCallerObjects(layoutOf(u)); err != nil {
			t.Fatal(err)
		}
		spec := u.Object["spec"].(map[string]any)
		for key, want := range map[string]any{"n": int64(53), "big": big, "u": int64(7), "f32": 2.5, "i32": int64(-3), "huge": int64(math.MaxInt64)} {
			if spec[key] != want {
				t.Errorf("spec.%s = %#v, want %#v", key, spec[key], want)
			}
		}
		if _, err := runtimeSafe(u); err != nil {
			t.Error(err)
		}
	})

	t.Run("the common caller shapes are normalized", func(t *testing.T) {
		u := object(map[string]any{"labels": map[string]string{"app": "x"}, "args": []string{"a", "b"}})
		if err := normalizeCallerObjects(layoutOf(u)); err != nil {
			t.Fatal(err)
		}
		want := map[string]any{"labels": map[string]any{"app": "x"}, "args": []any{"a", "b"}}
		if got := u.Object["spec"]; !reflect.DeepEqual(got, want) {
			t.Errorf("spec = %#v, want %#v", got, want)
		}
	})

	t.Run("a JSON typed object is left untouched", func(t *testing.T) {
		u := object(map[string]any{"n": int64(1), "f": 1.5, "s": "x", "b": true, "z": nil, "num": json.Number("2"), "l": []any{int64(1)}})
		before := u.Object
		spec := u.Object["spec"].(map[string]any)
		if err := normalizeCallerObjects(layoutOf(u)); err != nil {
			t.Fatal(err)
		}
		if reflect.ValueOf(u.Object).Pointer() != reflect.ValueOf(before).Pointer() || reflect.ValueOf(u.Object["spec"]).Pointer() != reflect.ValueOf(spec).Pointer() {
			t.Error("a clean object's maps were replaced")
		}
	})

	t.Run("the caller's map is not changed", func(t *testing.T) {
		u := object(map[string]any{"n": 53})
		held := u.Object["spec"].(map[string]any)
		if err := normalizeCallerObjects(layoutOf(u)); err != nil {
			t.Fatal(err)
		}
		if held["n"] != 53 {
			t.Errorf("the map the caller held now holds %#v", held["n"])
		}
	})

	t.Run("children are covered and a refusal replaces nothing", func(t *testing.T) {
		good := object(map[string]any{"n": 53})
		bad := object(map[string]any{"ok": []any{map[string]any{"ch": make(chan int)}}})
		root := layoutOf(good)
		root.Children = []*layout.ManifestLayout{layoutOf(bad)}
		err := normalizeCallerObjects(root)
		if err == nil {
			t.Fatal("got no error")
		}
		for _, want := range []string{"ConfigMap", "default/cm", "spec.ok[0].ch", "chan int"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not name %q", err, want)
			}
		}
		if good.Object["spec"].(map[string]any)["n"] != 53 {
			t.Error("an object was replaced although the call was refused")
		}
	})

	t.Run("nil roots, layouts and objects are skipped", func(t *testing.T) {
		var nilObj *unstructured.Unstructured
		if err := normalizeCallerObjects(nil, layoutOf(nilObj, nil)); err != nil {
			t.Fatal(err)
		}
	})
}

// runtimeSafe deep-copies u, which panics on a value that is not JSON typed.
func runtimeSafe(u *unstructured.Unstructured) (c *unstructured.Unstructured, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &panicError{r}
		}
	}()
	return u.DeepCopy(), nil
}

type panicError struct{ v any }

func (p *panicError) Error() string { return "DeepCopy panicked" }
