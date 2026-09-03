package admission

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureSource = `package fixture

import "errors"

var errNil = errors.New("nil")

type Ref struct{ Name, Kind string }

type Inner struct {
	A   string
	Ref Ref
}

type Spec struct {
	Replicas *int32
	Items    []string
	Name     string
	Ref      *Ref
	Nested   Inner
}

type Obj struct {
	Labels map[string]string
	Spec   Spec
}

// class a
func AddItem(o *Obj, s string) { o.Spec.Items = append(o.Spec.Items, s) }

// class a, map insert behind a nil-init
func AddLabel(o *Obj, k, v string) {
	if o.Labels == nil {
		o.Labels = map[string]string{}
	}
	o.Labels[k] = v
}

// class a through a local map variable
func AddLabelViaLocal(o *Obj, k, v string) {
	labels := o.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	labels[k] = v
	o.Labels = labels
}

// class b, pointer field
func SetReplicas(o *Obj, n int32) { o.Spec.Replicas = &n }

// class b, nil-init of an intermediate then write through it
func SetRefName(o *Obj, n string) {
	if o.Spec.Ref == nil {
		o.Spec.Ref = &Ref{}
	}
	o.Spec.Ref.Name = n
}

// class c, literal with two fields
func SetNestedRef(o *Obj, name, kind string) { o.Spec.Nested.Ref = Ref{Name: name, Kind: kind} }

// class c, nested literal
func SetNested(o *Obj, a string) { o.Spec.Nested = Inner{Ref: Ref{Name: a}} }

// inadmissible: two bare field writes are two forwarders, not a composite (no literal)
func SetNameAndA(o *Obj, n string) {
	o.Spec.Name = n
	o.Spec.Nested.A = n
}

// inadmissible: bare forwarder
func SetName(o *Obj, n string) { o.Spec.Name = n }

// inadmissible: same field written twice is still one field
func SetNameTwice(o *Obj, n string) {
	o.Spec.Name = n
	o.Spec.Name = n + "x"
}

// inadmissible: one-field literal
func SetNestedOneField(o *Obj, a string) { o.Spec.Nested = Inner{A: a} }

// inadmissible: delegation only
func SetViaDelegate(o *Obj, n string) { SetName(o, n) }

// inadmissible: no write at all
func SetNothing(o *Obj) { _ = o }

// inadmissible: a receiver nil guard does not make a bare forwarder class b
func SetGuardedName(o *Obj, n string) {
	if o == nil {
		panic("nil")
	}
	o.Spec.Name = n
}

// inadmissible: clears a field the caller did not name, despite the pointer assignment
func SetReplicasClearingRef(o *Obj, n int32) {
	o.Spec.Replicas = &n
	o.Spec.Ref = nil
}

// inadmissible: an append plus a bare write of a value the caller did not pass
func AddItemAndName(o *Obj, s string) {
	o.Spec.Items = append(o.Spec.Items, s)
	o.Spec.Name = "item"
}

// inadmissible: a pointer assignment plus a default into an unrelated field
func SetReplicasAndDefault(o *Obj, n int32) {
	o.Spec.Replicas = &n
	o.Spec.Name = "web"
}

// inadmissible: a computed value is not one the caller passed either
func SetReplicasAndDerived(o *Obj, n int32, name string) {
	o.Spec.Replicas = &n
	o.Spec.Name = name + "-x"
}

// class b: a bare write of a parameter next to the admitted operation forwards
// a caller-supplied value and does not change the class
func SetReplicasAndName(o *Obj, n int32, name string) {
	o.Spec.Replicas = &n
	o.Spec.Name = name
}

// class b, nil-init with new then a write through the star
func SetReplicasThroughStar(o *Obj, n int32) {
	if o.Spec.Replicas == nil {
		o.Spec.Replicas = new(int32)
	}
	*o.Spec.Replicas = n
}

// class a, make-init of the map then insert
func AddLabelMake(o *Obj, k, v string) {
	if o.Labels == nil {
		o.Labels = make(map[string]string)
	}
	o.Labels[k] = v
}

// inadmissible: the appended element is a constant the caller never supplied
func AddItemConstant(o *Obj) { o.Spec.Items = append(o.Spec.Items, "fixed") }

// inadmissible: the inserted value is a default
func AddLabelConstant(o *Obj, k string) {
	if o.Labels == nil {
		o.Labels = map[string]string{}
	}
	o.Labels[k] = "fixed"
}

// inadmissible: allocating a pointer nobody writes through leaves a zero value
func SetReplicasZero(o *Obj) { o.Spec.Replicas = new(int32) }

// inadmissible: a struct literal built entirely from constants is a default
func SetNestedRefConstant(o *Obj) { o.Spec.Nested.Ref = Ref{Name: "a", Kind: "b"} }

// inadmissible: a slice literal replaces the collection; class c is a struct literal
func SetItemsLiteral(o *Obj, a, b string) { o.Spec.Items = []string{a, b} }

// inadmissible: a by-value struct parameter is a copy, so the write reaches no caller
func SetReplicasByValue(o Obj, n int32) { o.Spec.Replicas = &n }

// inadmissible: silently skips the write instead of panicking on a nil receiver
func SetReplicasNilReturn(o *Obj, n int32) {
	if o == nil {
		return
	}
	o.Spec.Replicas = &n
}

// inadmissible: the optional-argument idiom, as an early return
func SetNameIfSet(o *Obj, n string) {
	if n == "" {
		return
	}
	o.Spec.Ref = &Ref{Name: n}
}

// inadmissible: a typed nil conversion is still a clear
func SetRefTypedNil(o *Obj) { o.Spec.Ref = (*Ref)(nil) }

// inadmissible: a local declared without a value is nil, and clears the field
func SetRefViaNilLocal(o *Obj) {
	var r *Ref
	o.Spec.Ref = r
}

// inadmissible: a local assigned nil later clears the field
func SetRefViaAssignedNil(o *Obj) {
	r := &Ref{}
	r = nil
	o.Spec.Ref = r
}

// inadmissible: a local declared with an explicit nil clears the field
func SetRefViaInitNil(o *Obj) {
	var r *Ref = nil
	o.Spec.Ref = r
}

// inadmissible: an explicit nil inside the literal clears that field
func SetSpecWithNilRef(o *Obj, n string) { o.Spec = Spec{Name: n, Ref: nil} }

// inadmissible: a typed nil two literals down
func SetSpecNestedNil(o *Obj, n string) {
	o.Spec = Spec{Nested: Inner{A: n, Ref: Ref{Name: n}}, Ref: (*Ref)(nil)}
}

// class a through a local slice variable
func AddItemViaLocal(o *Obj, s string) {
	items := o.Spec.Items
	items = append(items, s)
	o.Spec.Items = items
}

// class a through a parenthesised local map index, written back
func AddLabelParenLocal(o *Obj, k, v string) {
	labels := map[string]string{}
	(labels)[k] = v
	o.Labels = labels
}

// class a through a pointer alias of a field
func AddItemViaAlias(o *Obj, s string) {
	spec := &o.Spec
	spec.Items = append(spec.Items, s)
}

// class b through a declared alias of the object
func SetReplicasViaAlias(o *Obj, n int32) {
	var obj = o
	obj.Spec.Replicas = &n
}

// inadmissible: the write-back precedes the append, so the appended slice is lost
func AddItemWriteBackFirst(o *Obj, s string) {
	items := o.Spec.Items
	o.Spec.Items = items
	items = append(items, s)
	_ = items
}

// inadmissible: only a temporary is appended to
func AddItemToTemp(o *Obj, s string) {
	tmp := Obj{}
	tmp.Spec.Items = append(tmp.Spec.Items, s)
	_ = o
}

// inadmissible: only a temporary gets the pointer assignment
func SetRefOnTemp(o *Obj, n string) {
	tmp := &Obj{}
	tmp.Spec.Ref = &Ref{Name: n}
	_ = o
}

// inadmissible: only a temporary gets the composite
func SetNestedRefOnTemp(o *Obj, n string) {
	var tmp Obj
	tmp.Spec.Nested.Ref = Ref{Name: n, Kind: n}
	_ = o
}

// inadmissible: a temporary shadowing the parameter is still a temporary
func SetRefOnShadow(o *Obj, n string) {
	{
		o := &Obj{}
		o.Spec.Ref = &Ref{Name: n}
	}
	_ = o
}

// inadmissible: mutated before it is reassigned from the parameter
func SetRefBeforeAlias(o *Obj, n string) {
	tmp := &Obj{}
	tmp.Spec.Ref = &Ref{Name: n}
	tmp = o
	_ = tmp
}

// class b: an alias reassigned from the parameter counts from that point on
func SetRefAfterAlias(o *Obj, n string) {
	tmp := &Obj{}
	tmp = o
	tmp.Spec.Ref = &Ref{Name: n}
}

// inadmissible: the appended local and the written-back local are different objects
func AddItemShadowedLocal(o *Obj, s string) {
	items := o.Spec.Items
	{
		items := []string{}
		items = append(items, s)
		_ = items
	}
	o.Spec.Items = items
}

// inadmissible: append to a local that is never written back
func AddItemLocalOnly(o *Obj, s string) {
	tmp := o.Spec.Items
	tmp = append(tmp, s)
	_ = tmp
}

// inadmissible: map insert into a local that is never written back
func AddLabelLocalOnly(o *Obj, k, v string) {
	labels := map[string]string{}
	labels[k] = v
	_ = labels
}

// inadmissible: a local map op plus an unrelated bare write is still a forwarder
func AddLabelLocalThenName(o *Obj, k, v string) {
	labels := map[string]string{}
	labels[k] = v
	o.Spec.Name = k
}

// inadmissible as a bare forwarder, but not as a nil clear: a zero-valued
// scalar local is not nil (only nillable fields count)
func SetNameViaZeroLocal(o *Obj, n string) {
	var name string
	name = n
	o.Spec.Name = name
}

// inadmissible: an error return is out of contract however admissible the body
func AddItemWithError(o *Obj, s string) error {
	if o == nil {
		return errNil
	}
	o.Spec.Items = append(o.Spec.Items, s)
	return nil
}

// inadmissible: any result is out of contract, not only error
func SetReplicasReturning(o *Obj, n int32) *Obj {
	o.Spec.Replicas = &n
	return o
}

// exempt by name
func SetExempted(o *Obj, n string) { o.Spec.Name = n }

// not sugar helpers: no upper-case letter after the prefix, unexported, method
func Settle(o *Obj)                  { o.Spec.Name = "" }
func Address(o *Obj)                 { o.Spec.Name = "" }
func setName(o *Obj, n string)       { o.Spec.Name = n }
func (o *Obj) SetMethod(n string)    { o.Spec.Name = n }
`

func writeFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixture\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fixture.go"), []byte(fixtureSource), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestClassify_Fixture(t *testing.T) {
	dir := writeFixture(t)
	findings, err := Classify(Options{
		Dir:      dir,
		Patterns: []string{"."},
		Exempt:   map[string]bool{"fixture.SetExempted": true},
		Env:      append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod"),
	})
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]Class{
		"AddItem":                Append,
		"AddItemAndName":         Inadmissible,
		"SetReplicasAndDefault":  Inadmissible,
		"SetReplicasAndDerived":  Inadmissible,
		"SetReplicasAndName":     Pointer,
		"SetReplicasThroughStar": Pointer,
		"AddLabelMake":           Append,
		"AddItemConstant":        Inadmissible,
		"AddLabelConstant":       Inadmissible,
		"SetReplicasZero":        Inadmissible,
		"SetNestedRefConstant":   Inadmissible,
		"SetItemsLiteral":        Inadmissible,
		"SetReplicasByValue":     Inadmissible,
		"SetReplicasNilReturn":   Inadmissible,
		"SetNameIfSet":           Inadmissible,
		"AddLabel":               Append,
		"AddLabelViaLocal":       Append,
		"SetReplicas":            Pointer,
		"SetRefName":             Pointer,
		"SetNestedRef":           Composite,
		"SetNested":              Composite,
		"SetNameAndA":            Inadmissible,
		"SetName":                Inadmissible,
		"SetNameTwice":           Inadmissible,
		"SetNestedOneField":      Inadmissible,
		"SetViaDelegate":         Inadmissible,
		"SetNothing":             Inadmissible,
		"SetGuardedName":         Inadmissible,
		"SetReplicasClearingRef": Inadmissible,
		"SetRefTypedNil":         Inadmissible,
		"SetRefViaNilLocal":      Inadmissible,
		"SetRefViaAssignedNil":   Inadmissible,
		"SetNameViaZeroLocal":    Inadmissible,
		"AddItemWithError":       Inadmissible,
		"SetReplicasReturning":   Inadmissible,
		"SetRefViaInitNil":       Inadmissible,
		"SetSpecWithNilRef":      Inadmissible,
		"SetSpecNestedNil":       Inadmissible,
		"AddItemViaLocal":        Append,
		"AddLabelParenLocal":     Append,
		"AddItemViaAlias":        Append,
		"SetReplicasViaAlias":    Pointer,
		"AddItemWriteBackFirst":  Inadmissible,
		"SetRefOnShadow":         Inadmissible,
		"SetRefBeforeAlias":      Inadmissible,
		"SetRefAfterAlias":       Pointer,
		"AddItemShadowedLocal":   Inadmissible,
		"AddItemToTemp":          Inadmissible,
		"SetRefOnTemp":           Inadmissible,
		"SetNestedRefOnTemp":     Inadmissible,
		"AddItemLocalOnly":       Inadmissible,
		"AddLabelLocalOnly":      Inadmissible,
		"AddLabelLocalThenName":  Inadmissible,
		"SetExempted":            Exempt,
	}
	got := map[string]Finding{}
	for _, f := range findings {
		got[f.Name] = f
	}
	for name, class := range want {
		f, ok := got[name]
		if !ok {
			t.Errorf("%s: not classified at all", name)
			continue
		}
		if f.Class != class {
			t.Errorf("%s: class %s, want %s (%s)", name, f.Class, class, f.Reason)
		}
		if f.Package != "fixture" || f.Key() != "fixture."+name || f.Pos.Line == 0 {
			t.Errorf("%s: bad finding metadata %+v", name, f)
		}
	}
	// The reasons distinguish the rules a fixture exists for.
	for name, want := range map[string]string{
		"SetNameViaZeroLocal":   "single bare field assignment",
		"AddItemWithError":      "returns a value",
		"SetReplicasReturning":  "returns a value",
		"SetRefViaNilLocal":     "assigns nil",
		"SetNameAndA":           "bare field writes",
		"AddItemAndName":        "bare write to o.Spec.Name alongside",
		"SetReplicasAndDefault": "bare write to o.Spec.Name alongside",
		"SetReplicasAndDerived": "bare write to o.Spec.Name alongside",
		"AddItemConstant":       "value the caller did not supply to o.Spec.Items",
		"AddLabelConstant":      "value the caller did not supply to o.Labels[k]",
		"SetReplicasZero":       "value the caller did not supply to o.Spec.Replicas",
		"SetNestedRefConstant":  "value the caller did not supply to o.Spec.Nested.Ref",
		"SetItemsLiteral":       "single bare field assignment",
		"SetReplicasByValue":    "no field write",
		"SetReplicasNilReturn":  "returns early instead of writing",
		"SetNameIfSet":          "returns early instead of writing",
	} {
		if reason := got[name].Reason; !strings.Contains(reason, want) {
			t.Errorf("%s: reason %q, want it to contain %q", name, reason, want)
		}
	}
	for name := range got {
		if _, expected := want[name]; !expected {
			t.Errorf("%s: classified but is not a sugar helper", name)
		}
	}
	if len(findings) != len(want) {
		t.Errorf("got %d findings, want %d", len(findings), len(want))
	}
}

func TestClassify_FindingsAreSorted(t *testing.T) {
	dir := writeFixture(t)
	findings, err := Classify(Options{Dir: dir, Patterns: []string{"."}, Env: append(os.Environ(), "GOWORK=off")})
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(findings); i++ {
		if findings[i-1].Name > findings[i].Name {
			t.Fatalf("findings not sorted: %s before %s", findings[i-1].Name, findings[i].Name)
		}
	}
}

func TestClassify_LoadErrorIsReported(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module broken\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.go"), []byte("package broken\n\nfunc SetX(o *Missing) {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Classify(Options{Dir: dir, Patterns: []string{"."}, Env: append(os.Environ(), "GOWORK=off")})
	if err == nil {
		t.Fatal("expected a load error for an undefined type")
	}
}

func TestClassString(t *testing.T) {
	for c, s := range map[Class]string{Inadmissible: "inadmissible", Append: "append", Pointer: "pointer", Composite: "composite", Exempt: "exempt", Class(9): "Class(9)"} {
		if c.String() != s {
			t.Errorf("%d.String() = %q, want %q", int(c), c.String(), s)
		}
	}
}

func TestReadExclusions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ex.txt")
	if err := os.WriteFile(path, []byte("# comment\n\n  a.SetX  \nb.AddY\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	keys, err := ReadExclusions(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0] != "a.SetX" || keys[1] != "b.AddY" {
		t.Errorf("keys = %v", keys)
	}
	if _, err := ReadExclusions(filepath.Join(t.TempDir(), "missing.txt")); err == nil {
		t.Error("expected an error for a missing file")
	}
}
