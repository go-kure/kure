package kustomizeschema

import (
	randv1 "math/rand"
	randv2 "math/rand/v2"
	"runtime/debug"
	"testing"
)

type pair struct {
	A map[string]any
	B map[string]any
}

type hidden struct {
	shown  string
	hidden string
}

type (
	namedString string
	namedBool   bool
	namedInt    int
	namedFloat  float64
	namedSlice  []string
	namedKey    string
)

type node struct {
	Name string
	Next *node
	Any  any
}

// TestFingerprint_TellsValuesApart: each pair differs in one way a
// serializer, or a walk that skipped it, would not see: a value moved to
// another field, an unexported field, a dynamic type, nil against empty, a
// pointer's pointee, and a split between two strings.
func TestFingerprint_TellsValuesApart(t *testing.T) {
	for name, tc := range map[string][2]any{
		"a value moved between fields": {pair{A: map[string]any{"k": 1}}, pair{B: map[string]any{"k": 1}}},
		"an unexported field":          {hidden{"a", "x"}, hidden{"a", "y"}},
		"an int against a float":       {[]any{1}, []any{1.0}},
		"an int against an int64":      {[]any{1}, []any{int64(1)}},
		"a nil map against an empty":   {pair{}, pair{A: map[string]any{}}},
		"a nil slice against an empty": {[]string(nil), []string{}},
		"a pointee":                    {&node{Name: "a"}, &node{Name: "b"}},
		"a nil pointer":                {&node{}, &node{Next: &node{}}},
		"a split between strings":      {[]string{"ab", "c"}, []string{"a", "bc"}},
		"a key against a value":        {map[string]string{"a": "b"}, map[string]string{"b": "a"}},
		"an interface value's type":    {node{Any: "1"}, node{Any: 1}},
		"a bool":                       {[]bool{true}, []bool{false}},
		"an array element":             {[2]string{"a", "b"}, [2]string{"b", "a"}},
		"a named string":               {[]any{"merge"}, []any{namedString("merge")}},
		"a named bool":                 {[]any{true}, []any{namedBool(true)}},
		"a named int":                  {[]any{1}, []any{namedInt(1)}},
		"a named float":                {[]any{1.5}, []any{namedFloat(1.5)}},
		"a named slice":                {[]any{[]string{"a"}}, []any{namedSlice{"a"}}},
		"a named key":                  {map[any]string{"k": "v"}, map[any]string{namedKey("k"): "v"}},
		"a named key type":             {map[string]string{"k": "v"}, map[namedKey]string{"k": "v"}},
	} {
		if fingerprint(tc[0]) == fingerprint(tc[1]) {
			t.Errorf("%s: the fingerprints are equal", name)
		}
	}
}

// TestFingerprint_ReadsEqualValuesAlike: values equal in every field have one
// fingerprint, whatever the pointers they hold, the order a map was filled in
// or Go's order of iterating it, and a map keyed by something other than a
// string.
func TestFingerprint_ReadsEqualValuesAlike(t *testing.T) {
	build := func(order []string) map[string]any {
		m := map[string]any{}
		for _, k := range order {
			m[k] = []any{k, 1.5, map[string]any{"in": k}}
		}
		return m
	}
	want := fingerprint(build([]string{"a", "b", "c", "d", "e", "f"}))
	for range 20 {
		if got := fingerprint(build([]string{"f", "e", "d", "c", "b", "a"})); got != want {
			t.Fatalf("a map filled in another order: %s, want %s", got, want)
		}
	}
	chain := func() *node { return &node{Name: "x", Next: &node{Name: "y"}} }
	if a, b := fingerprint(chain()), fingerprint(chain()); a != b {
		t.Error("equal values behind different pointers have different fingerprints")
	}
	keyed := func() map[[2]int]string {
		return map[[2]int]string{{1, 2}: "a", {3, 4}: "b", {0, 9}: "c", {5, 5}: "d"}
	}
	first := fingerprint(keyed())
	for range 20 {
		if got := fingerprint(keyed()); got != first {
			t.Fatal("a map with array keys has an unstable fingerprint")
		}
	}
}

// TestFingerprint_StopsAtACycle: a pointer, and a map reached through an
// interface, that lead back to themselves are walked once, and what they hold
// still counts.
func TestFingerprint_StopsAtACycle(t *testing.T) {
	self := func(name string) *node {
		n := &node{Name: name}
		n.Next = n
		return n
	}
	if a, b := fingerprint(self("a")), fingerprint(self("a")); a != b {
		t.Error("a self-referencing pointer has an unstable fingerprint")
	}
	if fingerprint(self("a")) == fingerprint(self("b")) {
		t.Error("two cycles holding different names have one fingerprint")
	}
	m := map[string]any{}
	m["self"] = m
	_ = fingerprint(m)
	s := make([]any, 1)
	s[0] = s
	_ = fingerprint(s)
}

// TestFingerprint_TellsEveryKindApart: a value of each kind the walk reads, in
// a container that is otherwise equal, moves the fingerprint, as do maps
// nested in maps, an array's length and a root that holds nothing.
func TestFingerprint_TellsEveryKindApart(t *testing.T) {
	ch := make(chan int)
	for name, tc := range map[string][2]any{
		"an int8":                {[]int8{1}, []int8{2}},
		"a negative int":         {[]int{-1}, []int{1}},
		"a uint":                 {[]uint{1}, []uint{2}},
		"a float32":              {[]float32{1.5}, []float32{2.5}},
		"a complex real part":    {[]complex128{1 + 2i}, []complex128{3 + 2i}},
		"a complex imaginary":    {[]complex128{1 + 2i}, []complex128{1 + 3i}},
		"a func against nil":     {[]any{func() {}}, []any{(func())(nil)}},
		"a chan against nil":     {[]any{ch}, []any{(chan int)(nil)}},
		"a map nested in a map":  {map[string]map[string]int{"a": {"b": 1}}, map[string]map[string]int{"a": {"b": 2}}},
		"a nil interface":        {[]any{nil}, []any{0}},
		"a root that holds none": {nil, 0},
		"an array's length":      {[1]string{"a"}, [2]string{"a", ""}},
		"a key's type":           {map[any]int{1: 1}, map[any]int{int64(1): 1}},
	} {
		if fingerprint(tc[0]) == fingerprint(tc[1]) {
			t.Errorf("%s: the fingerprints are equal", name)
		}
	}
}

func retypedField() any {
	type T struct{ X string }
	return T{"a"}
}

func retypedFieldType() any {
	type T struct{ X namedString }
	return T{"a"}
}

func renamedField() any {
	type T struct{ Y string }
	return T{"a"}
}

// TestFingerprint_ReadsTheLayoutOfATypeNotItsName: three struct types of one
// name and one value, whose field differs in its type or its name, have three
// fingerprints: a library bump that retypes or renames a field of the schema
// changes it, which the pin test then reports.
func TestFingerprint_ReadsTheLayoutOfATypeNotItsName(t *testing.T) {
	a, b, c := fingerprint(retypedField()), fingerprint(retypedFieldType()), fingerprint(renamedField())
	if a == b || a == c || b == c {
		t.Errorf("fingerprints %s, %s, %s: want three", a, b, c)
	}
}

// TestFingerprint_ASharedPointerOrMapIsNotAnEqualCopy: a pointer or map that
// two parents share is read once and written as a reference the second time,
// so the same content in two copies differs from it, the conservative way: a
// definition replaced by an equal copy that stops being shared is refused,
// never read as unchanged. Slices carry no such sharing.
func TestFingerprint_ASharedPointerOrMapIsNotAnEqualCopy(t *testing.T) {
	p := &node{Name: "x"}
	if fingerprint([]*node{p, p}) == fingerprint([]*node{{Name: "x"}, {Name: "x"}}) {
		t.Error("a shared pointer and two equal copies have one fingerprint")
	}
	if a, b := fingerprint([]*node{p, p}), fingerprint([]*node{p, p}); a != b {
		t.Error("a shared pointer has an unstable fingerprint")
	}
	m := map[string]int{"a": 1}
	if fingerprint([]map[string]int{m, m}) == fingerprint([]map[string]int{{"a": 1}, {"a": 1}}) {
		t.Error("a shared map and two equal copies have one fingerprint")
	}
	s := []string{"a"}
	if fingerprint([][]string{s, s}) != fingerprint([][]string{{"a"}, {"a"}}) {
		t.Error("a shared slice and two equal copies have different fingerprints")
	}
}

// TestFingerprint_AKeyThatLeadsBackToItsMapIsACycle: a map key that is no
// string and reaches the map that holds it is walked once and written as a
// cycle, not forever. The stack is capped so that a regression fails the test
// at once, not after a gigabyte.
func TestFingerprint_AKeyThatLeadsBackToItsMapIsACycle(t *testing.T) {
	defer debug.SetMaxStack(debug.SetMaxStack(16 << 20))
	keyed := func(v int) map[any]any {
		m := map[any]any{}
		m[&m] = v
		return m
	}
	if a, b := fingerprint(keyed(1)), fingerprint(keyed(1)); a != b {
		t.Error("a map keyed by a pointer to itself has an unstable fingerprint")
	}
	if fingerprint(keyed(1)) == fingerprint(keyed(2)) {
		t.Error("two such maps holding different values have one fingerprint")
	}
}

func localKindInt() any {
	type N int
	type T struct{ F N }
	return T{}
}

func localKindString() any {
	type N string
	type T struct{ F N }
	return T{}
}

func localKindIntSlice() any {
	type N int
	type T struct{ F []N }
	return T{}
}

func localKindStringSlice() any {
	type N string
	type T struct{ F []N }
	return T{}
}

// TestFingerprint_ReadsTheKindOfANamedType: two types of one package path and
// name whose underlying kinds differ, behind a field or an element of an
// unnamed type, have different fingerprints, though their zero values write
// the same bytes.
func TestFingerprint_ReadsTheKindOfANamedType(t *testing.T) {
	if fingerprint(localKindInt()) == fingerprint(localKindString()) {
		t.Error("a field of a named int and one of a named string have one fingerprint")
	}
	if fingerprint(localKindIntSlice()) == fingerprint(localKindStringSlice()) {
		t.Error("a slice of a named int and one of a named string have one fingerprint")
	}
}

func localInt() []any {
	type N int
	type S struct{ F []struct{ G N } }
	type P struct{ F *struct{ G N } }
	type C struct{ F chan N }
	type R struct{ F func(N) }
	type O struct{ F func() N }
	type I struct{ F interface{ M(N) } }
	return []any{S{}, P{}, C{}, R{}, O{}, I{}}
}

func localString() []any {
	type N string
	type S struct{ F []struct{ G N } }
	type P struct{ F *struct{ G N } }
	type C struct{ F chan N }
	type R struct{ F func(N) }
	type O struct{ F func() N }
	type I struct{ F interface{ M(N) } }
	return []any{S{}, P{}, C{}, R{}, O{}, I{}}
}

// TestFingerprint_ReadsTheKindOfANamedTypeInAnUnnamedOne: the same two types
// behind a field of an unnamed struct, func, chan or interface type, which a
// nil value or a func or chan leaves undefined beyond its type.
func TestFingerprint_ReadsTheKindOfANamedTypeInAnUnnamedOne(t *testing.T) {
	ints, strs := localInt(), localString()
	for i, shape := range []string{"slice of struct", "pointer to struct", "chan", "func parameter", "func result", "interface method"} {
		if fingerprint(ints[i]) == fingerprint(strs[i]) {
			t.Errorf("a %s of a named int and one of a named string have one fingerprint", shape)
		}
	}
}

func shortNameV1() any {
	type T struct{ F []struct{ G *randv1.Rand } }
	return T{}
}

func shortNameV2() any {
	type T struct{ F []struct{ G *randv2.Rand } }
	return T{}
}

// TestFingerprint_ReadsThePackagePathInAnUnnamedStruct: two packages of one
// short name behind a field of an unnamed struct.
func TestFingerprint_ReadsThePackagePathInAnUnnamedStruct(t *testing.T) {
	if fingerprint(shortNameV1()) == fingerprint(shortNameV2()) {
		t.Error("two packages of one short name in an unnamed struct have one fingerprint")
	}
}

// TestFingerprint_NumbersASliceCycleByDepth: two cycles of slices that return
// to different slices of the path have different fingerprints.
func TestFingerprint_NumbersASliceCycleByDepth(t *testing.T) {
	cycle := func(toRoot bool) []any {
		a, b := []any{nil}, []any{nil}
		a[0] = b
		if toRoot {
			b[0] = a
		} else {
			b[0] = b
		}
		return a
	}
	if fingerprint(cycle(true)) == fingerprint(cycle(false)) {
		t.Error("a cycle back to the root and one back to the inner slice have one fingerprint")
	}
	first, again := fingerprint(cycle(true)), fingerprint(cycle(true))
	if first != again {
		t.Error("a slice cycle has an unstable fingerprint")
	}
}

// BenchmarkDefinitions is the cost of one Definitions call on the parsed
// built-in schema, which treeBuilds pays twice per tree.
func BenchmarkDefinitions(b *testing.B) {
	Parse()
	b.ResetTimer()
	for range b.N {
		if _, ok := Definitions(); !ok {
			b.Fatal("not the built-in definitions")
		}
	}
}
