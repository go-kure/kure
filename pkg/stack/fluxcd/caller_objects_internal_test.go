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
			if got := jsonTyped(v, 0); got != accepted {
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
		if err := normalizeForTest(layoutOf(u)); err != nil {
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
		if err := normalizeForTest(layoutOf(u)); err != nil {
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
		if err := normalizeForTest(layoutOf(u)); err != nil {
			t.Fatal(err)
		}
		if reflect.ValueOf(u.Object).Pointer() != reflect.ValueOf(before).Pointer() || reflect.ValueOf(u.Object["spec"]).Pointer() != reflect.ValueOf(spec).Pointer() {
			t.Error("a clean object's maps were replaced")
		}
	})

	t.Run("the caller's map is not changed", func(t *testing.T) {
		u := object(map[string]any{"n": 53})
		held := u.Object["spec"].(map[string]any)
		if err := normalizeForTest(layoutOf(u)); err != nil {
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
		err := normalizeForTest(root)
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
		if err := normalizeForTest(nil, layoutOf(nilObj, nil)); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("an object that holds itself is refused, not followed without end", func(t *testing.T) {
		m := map[string]any{"apiVersion": "v1", "kind": "List"}
		m["items"] = []any{m}
		err := normalizeForTest(layoutOf(&unstructured.Unstructured{Object: m}))
		if err == nil || !strings.Contains(err.Error(), "holds itself") {
			t.Errorf("got %v, want a refusal naming the cycle", err)
		}
	})

	t.Run("a value nested deeper than the limit is refused, cycle or not", func(t *testing.T) {
		nest := func(levels int, leaf any) map[string]any {
			m := map[string]any{"leaf": leaf}
			for range levels {
				m = map[string]any{"n": m}
			}
			return m
		}
		for name, leaf := range map[string]any{"JSON typed": "x", "not JSON typed": 53} {
			u := object(nest(maxJSONDepth+1, leaf))
			err := normalizeForTest(layoutOf(u))
			if err == nil || !strings.Contains(err.Error(), "nested deeper than") {
				t.Errorf("%s: got %v, want a refusal for the nesting", name, err)
			}
		}
		u := object(nest(100, 53))
		if err := normalizeForTest(layoutOf(u)); err != nil {
			t.Errorf("a nesting of 100 levels was refused: %v", err)
		}
	})

	t.Run("a value nested deeper than the limit in a named container is refused", func(t *testing.T) {
		type named map[string]any
		m := named{"leaf": 53}
		for range maxJSONDepth + 1 {
			m = named{"n": m}
		}
		u := object(map[string]any{"deep": m})
		for call := range 2 {
			err := normalizeForTest(layoutOf(u))
			if err == nil || !strings.Contains(err.Error(), "nested deeper than") {
				t.Errorf("call %d: got %v, want a refusal for the nesting", call, err)
			}
		}
	})

	t.Run("a struct is refused with its field named, and never encoded", func(t *testing.T) {
		u := object(nil)
		u.Object["spec"] = map[string]any{"self": u}
		err := normalizeForTest(layoutOf(u))
		if err == nil || !strings.Contains(err.Error(), "spec.self holds a *unstructured.Unstructured") {
			t.Errorf("got %v, want a refusal naming spec.self", err)
		}
	})

	t.Run("a slice that holds a shorter slice of itself is no cycle", func(t *testing.T) {
		s := make([]any, 3)
		s[1], s[2] = s[:1], 53
		u := object(map[string]any{"s": s})
		if err := normalizeForTest(layoutOf(u)); err != nil {
			t.Fatalf("got %v, want none", err)
		}
		want := []any{nil, []any{nil}, int64(53)}
		if got := u.Object["spec"].(map[string]any)["s"]; !reflect.DeepEqual(got, want) {
			t.Errorf("spec.s = %#v, want %#v", got, want)
		}
	})

	t.Run("a slice that holds itself is refused", func(t *testing.T) {
		s := make([]any, 2)
		s[0], s[1] = 53, s
		err := normalizeForTest(layoutOf(object(map[string]any{"s": s})))
		if err == nil || !strings.Contains(err.Error(), "spec.s[1] holds itself") {
			t.Errorf("got %v, want a refusal naming spec.s[1]", err)
		}
	})

	t.Run("values shared 60 levels deep are refused, not walked", func(t *testing.T) {
		for name, leaf := range map[string]any{"JSON typed": "x", "not JSON typed": 53} {
			m := map[string]any{"leaf": leaf}
			for range 60 {
				m = map[string]any{"a": m, "b": m}
			}
			err := normalizeForTest(layoutOf(object(m)))
			if err == nil || !strings.Contains(err.Error(), "more than") {
				t.Errorf("%s: got %v, want a refusal for the count", name, err)
			}
		}
	})

	t.Run("the JSON form of the common Go kinds", func(t *testing.T) {
		type name string
		u := object(map[string]any{
			"name": name("x"), "ptr": new(int), "bytes": []byte("hi"), "whole": 2.0, "frac": 0.1,
			"f32": float32(0.1), "nilMap": map[string]string(nil), "nilSlice": []string(nil),
			"empty": []string{}, "num": json.Number("7"), "big": uint64(math.MaxUint64),
		})
		if err := normalizeForTest(layoutOf(u)); err != nil {
			t.Fatal(err)
		}
		want := map[string]any{
			"name": "x", "ptr": int64(0), "bytes": "aGk=", "whole": int64(2), "frac": 0.1,
			"f32": 0.1, "nilMap": nil, "nilSlice": nil, "empty": []any{}, "num": int64(7), "big": float64(math.MaxUint64),
		}
		if got := u.Object["spec"]; !reflect.DeepEqual(got, want) {
			t.Errorf("spec = %#v, want %#v", got, want)
		}
	})

	t.Run("maps that hold themselves twice are refused at once", func(t *testing.T) {
		m := map[string]any{"n": 53}
		m["a"], m["b"] = m, m
		err := normalizeForTest(layoutOf(&unstructured.Unstructured{Object: m}))
		if err == nil || !strings.Contains(err.Error(), "holds itself") {
			t.Errorf("got %v, want a refusal naming the cycle", err)
		}
	})

	t.Run("undo keeps what the caller set after the normalization", func(t *testing.T) {
		u := object(map[string]any{"n": 53})
		undo, err := normalizeCallerObjects(layoutOf(u))
		if err != nil {
			t.Fatal(err)
		}
		later := map[string]any{"apiVersion": "v1", "kind": "ConfigMap"}
		u.Object = later
		undo()
		if reflect.ValueOf(u.Object).Pointer() != reflect.ValueOf(later).Pointer() {
			t.Error("undo overwrote a map the caller set after the call")
		}
	})

	t.Run("undo puts the caller's Object maps back", func(t *testing.T) {
		u := object(map[string]any{"n": 53})
		held := u.Object
		undo, err := normalizeCallerObjects(layoutOf(u))
		if err != nil {
			t.Fatal(err)
		}
		if reflect.ValueOf(u.Object).Pointer() == reflect.ValueOf(held).Pointer() {
			t.Fatal("the object was not replaced")
		}
		undo()
		if reflect.ValueOf(u.Object).Pointer() != reflect.ValueOf(held).Pointer() {
			t.Error("undo did not put the caller's map back")
		}
	})
}

// normalizeForTest is normalizeCallerObjects without the undo.
func normalizeForTest(roots ...*layout.ManifestLayout) error {
	_, err := normalizeCallerObjects(roots...)
	return err
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
