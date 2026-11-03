package fluxcd

import (
	"crypto/x509"
	"encoding/json"
	"math"
	"net"
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
		if err := normalizeForTest(nil, layoutOf(nil)); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("a typed nil object is refused, naming it", func(t *testing.T) {
		var nilObj *unstructured.Unstructured
		err := normalizeForTest(layoutOf(nilObj))
		if err == nil || !strings.Contains(err.Error(), "nil *unstructured.Unstructured") {
			t.Fatalf("got %v, want a refusal naming a nil *unstructured.Unstructured", err)
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

	t.Run("the accepted shapes become their JSON typed counterparts", func(t *testing.T) {
		type name string
		type flag bool
		type count int
		type labels map[string]string
		var anyValue any = int8(-3)
		u := object(map[string]any{
			"int": 1, "int8": int8(2), "uint16": uint16(3), "uintptr": uintptr(4), "exact": int(1<<53 + 1),
			"maxUint64": uint64(math.MaxInt64), "float32": float32(2.5), "float64": 1.5, "whole": 2.0,
			"name": name("x"), "flag": flag(true), "count": count(5), "labels": labels{"a": "b"},
			"strings": []string{"a"}, "bytes": []byte{1, 2}, "nested": []map[string][]int{{"a": {1, 2}}},
			"number": json.Number("7"), "iface": []any{anyValue, nil},
			"nilMap": map[string]string(nil), "nilSlice": []string(nil), "empty": []string{},
		})
		if err := normalizeForTest(layoutOf(u)); err != nil {
			t.Fatal(err)
		}
		want := map[string]any{
			"int": int64(1), "int8": int64(2), "uint16": int64(3), "uintptr": int64(4), "exact": int64(1<<53 + 1),
			"maxUint64": int64(math.MaxInt64), "float32": 2.5, "float64": 1.5, "whole": 2.0,
			"name": "x", "flag": true, "count": int64(5), "labels": map[string]any{"a": "b"},
			"strings": []any{"a"}, "bytes": "AQI=", "nested": []any{map[string]any{"a": []any{int64(1), int64(2)}}},
			"number": json.Number("7"), "iface": []any{int64(-3), nil},
			"nilMap": nil, "nilSlice": nil, "empty": []any{},
		}
		if got := u.Object["spec"]; !reflect.DeepEqual(got, want) {
			t.Errorf("spec = %#v, want %#v", got, want)
		}
		if _, err := runtimeSafe(u); err != nil {
			t.Error(err)
		}
	})

	t.Run("everything else is refused with the field named", func(t *testing.T) {
		values := map[string]any{
			"json.RawMessage":                 json.RawMessage(`{"a":1}`),
			"net.IP":                          net.ParseIP("10.0.0.1"),
			"a named string with a marshaler": marshalingString("x"),
			"a pointer":                       new(int),
			"a struct":                        struct{ A int }{1},
			"an array":                        [2]int{1, 2},
			"a channel":                       make(chan int),
			"a function":                      func() {},
			"a map with int keys":             map[int]string{1: "a"},
			"a uint64 above int64":            uint64(math.MaxUint64),
			"a complex":                       complex(1, 2),
			"a slice of structs":              []struct{ A int }{{1}},
		}
		for label, v := range values {
			t.Run(label, func(t *testing.T) {
				u := object(map[string]any{"v": v, "n": 0})
				err := normalizeForTest(layoutOf(u))
				if err == nil || !strings.Contains(err.Error(), "spec.v") {
					t.Errorf("got %v, want a refusal naming spec.v", err)
				}
				if u.Object["spec"].(map[string]any)["n"] != 0 {
					t.Error("the refused object was changed")
				}
			})
		}
	})

	t.Run("a map that marshals itself and holds itself is refused, not recursed", func(t *testing.T) {
		m := selfMarshalingMap{}
		m["self"] = m
		u := object(map[string]any{"v": m, "n": 0})
		err := normalizeForTest(layoutOf(u))
		if err == nil || !strings.Contains(err.Error(), "spec.v") || !strings.Contains(err.Error(), "marshals itself") {
			t.Errorf("got %v, want a refusal naming spec.v and the marshaler", err)
		}
		if selfMarshalCalls != 0 {
			t.Errorf("MarshalJSON was called %d times", selfMarshalCalls)
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

// marshalingString is a named string kind that marshals itself.
type marshalingString string

func (marshalingString) MarshalJSON() ([]byte, error) { return []byte(`"m"`), nil }

// selfMarshalCalls counts the calls of selfMarshalingMap.MarshalJSON.
var selfMarshalCalls int

// selfMarshalingMap marshals itself by marshalling a plain copy of itself: held
// by itself it would recurse without end, as each call starts its own encoder.
type selfMarshalingMap map[string]any

func (m selfMarshalingMap) MarshalJSON() ([]byte, error) {
	selfMarshalCalls++
	return json.Marshal(map[string]any(m))
}

// The refusal names the object by its kind, namespace and name. A value that
// holds itself in one of them must not be formatted: apimachinery's accessors
// format the value of a field that is no string, and recurse on one that holds
// itself.
func TestNormalizeCallerObjects_RefusalDoesNotFormatIdentityValues(t *testing.T) {
	self := map[string]any{}
	self["self"] = self
	for _, field := range []string{"kind", "namespace", "name"} {
		t.Run(field, func(t *testing.T) {
			u := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "v1", "kind": "ConfigMap",
				"metadata": map[string]any{"name": "cm", "namespace": "default"},
			}}
			if field == "kind" {
				u.Object["kind"] = self
			} else {
				u.Object["metadata"].(map[string]any)[field] = self
			}
			err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{u}})
			if err == nil || !strings.Contains(err.Error(), "holds itself") {
				t.Errorf("got %v, want a refusal naming the cycle", err)
			}
		})
	}
}

func TestNormalizeCallerObjects_HugeSliceIsRefusedNotAllocated(t *testing.T) {
	// A slice of a zero-size type needs no storage, so it can be as long as
	// an int holds.
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "ConfigMap",
		"spec": map[string]any{"v": make([]struct{}, int(^uint(0)>>1))},
	}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("the conversion panicked: %v", r)
		}
	}()
	err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{u}})
	if err == nil || !strings.Contains(err.Error(), "spec.v") || !strings.Contains(err.Error(), "more than") {
		t.Errorf("got %v, want a refusal naming spec.v and the limit", err)
	}
}

// What the old read made of a value is what the conversion makes of it: the
// value as encoding/json writes it, read back.
type namedBytes []byte
type namedFloat32 float32

func TestNormalizeCallerObjects_MatchesEncodingJSON(t *testing.T) {
	values := map[string]any{
		"float32 0.1":        float32(0.1),
		"float32 small":      float32(1e-7),
		"float32 large":      float32(3.4e38),
		"float32 integer":    float32(16777216),
		"float32 negative":   float32(-2.5),
		"named float32":      namedFloat32(0.1),
		"float32 in a slice": []float32{0.1, 0.2},
		"bytes":              []byte{1, 2},
		"bytes empty":        []byte{},
		"bytes nil":          []byte(nil),
		"named bytes":        namedBytes("hello"),
		"bytes in a map":     map[string][]byte{"k": []byte("secret")},
		"bytes all values":   []byte{0, 255, 128},
	}
	for name, v := range values {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			var want any
			if err := json.Unmarshal(data, &want); err != nil {
				t.Fatal(err)
			}
			u := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "spec": map[string]any{"v": v}}}
			if err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{u}}); err != nil {
				t.Fatal(err)
			}
			got := u.Object["spec"].(map[string]any)["v"]
			if !reflect.DeepEqual(got, want) {
				t.Errorf("got %#v, want %#v (encoding/json: %s)", got, want, data)
			}
			if _, err := runtimeSafe(u); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestNormalizeCallerObjects_OversizedByteSliceIsRefused(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Secret",
		"data": map[string]any{"v": make([]byte, maxJSONNodes+1)},
	}}
	err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{u}})
	if err == nil || !strings.Contains(err.Error(), "data.v holds more than") || !strings.Contains(err.Error(), "bytes") {
		t.Errorf("got %v, want a refusal naming data.v and the byte limit", err)
	}
}

// An object that is under the value limit as JSON typed values is under it
// after one value needs converting: an interface and the value it holds are
// one value, as jsonTyped counts them.
func TestNormalizeCallerObjects_InterfaceValuesCountOnce(t *testing.T) {
	const n = 3_000_000
	list := make([]any, n, n+1)
	for i := range list {
		list[i] = int64(i)
	}
	if !jsonTyped(list, 0) {
		t.Fatal("the list is not JSON typed")
	}
	list = append(list, 1) // an int: the object now needs the conversion
	u := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "spec": map[string]any{"v": list}}}
	if err := normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: []client.Object{u}}); err != nil {
		t.Fatalf("refused an object of %d values: %v", n+1, err)
	}
	if got := u.Object["spec"].(map[string]any)["v"].([]any); len(got) != n+1 || got[n] != int64(1) {
		t.Errorf("got %d values, last %#v", len(got), got[len(got)-1])
	}
}

// A package path with no dot is not the standard library: package main and a
// caller's module of a short name have none.
func TestTrustedPath(t *testing.T) {
	for path, want := range map[string]bool{
		"time":                                true,
		"encoding/json":                       true,
		"k8s.io/apimachinery/pkg/util/intstr": true,
		"sigs.k8s.io/yaml":                    true,
		"github.com/go-kure/kure/pkg/x":       true,
		"main":                                false,
		"myapp/model":                         false,
		"os":                                  false,
		"example.com/x":                       false,
		"github.com/go-kure/kure/pkg/x_test":  false,
		"":                                    false,
	} {
		if got := trustedPath(path); got != want {
			t.Errorf("trustedPath(%q) = %v, want %v", path, got, want)
		}
	}
}

// promotesTrusted embeds a trusted type and so promotes its marshaler.
type promotesTrusted struct{ *unstructured.Unstructured }

// declaresOwn embeds a trusted type but declares a MarshalJSON of its own.
type declaresOwn struct{ *unstructured.Unstructured }

func (declaresOwn) MarshalJSON() ([]byte, error) { return []byte(`{}`), nil }

// ownMarshaler is a marshaler of this package, which is not trusted.
type ownMarshaler struct{}

func (ownMarshaler) MarshalJSON() ([]byte, error) { return []byte(`{}`), nil }

// promotesUntrusted embeds a marshaler of a package that is not trusted.
type promotesUntrusted struct{ x509.OID }

// A named struct that only promotes a marshaler from an embedded trusted type
// is trusted; one that declares its own, or promotes one from a type that is
// not trusted, is not.
func TestPromotedFromTrusted(t *testing.T) {
	for _, tc := range []struct {
		name string
		typ  reflect.Type
		want bool
	}{
		{"promotes a trusted marshaler", reflect.TypeFor[promotesTrusted](), true},
		{"promotes a trusted marshaler, addressed", reflect.TypeFor[*promotesTrusted](), true},
		{"declares its own", reflect.TypeFor[declaresOwn](), false},
		{"promotes an untrusted marshaler", reflect.TypeFor[promotesUntrusted](), false},
		{"is the marshaler", reflect.TypeFor[ownMarshaler](), false},
		{"no marshaler", reflect.TypeFor[struct{ A int }](), false},
	} {
		if got := promotedFromTrusted(tc.typ); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

// Where it cannot be told whether a method is a promotion wrapper, the answer
// is no, so the type is not trusted.
func TestIsPromotion_CannotTellIsNo(t *testing.T) {
	if isPromotion(0) {
		t.Error("a function that cannot be found is called a promotion")
	}
	own, _ := reflect.TypeFor[declaresOwn]().MethodByName("MarshalJSON")
	if isPromotion(own.Func.Pointer()) {
		t.Error("a method with a source file is called a promotion")
	}
	promoted, _ := reflect.TypeFor[promotesTrusted]().MethodByName("MarshalJSON")
	if !isPromotion(promoted.Func.Pointer()) {
		t.Error("the wrapper of a promoted method is not called a promotion")
	}
}
