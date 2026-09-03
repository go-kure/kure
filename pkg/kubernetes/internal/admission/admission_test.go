package admission

import (
	"os"
	"path/filepath"
	"testing"
)

const fixtureSource = `package fixture

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

// class b: a nil-valued scalar local is not a clear (only nillable fields count)
func SetNameFromZeroLocal(o *Obj) {
	var n string
	o.Spec.Ref = &Ref{Name: n}
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
		"SetNameFromZeroLocal":   Pointer,
		"SetRefViaInitNil":       Inadmissible,
		"SetSpecWithNilRef":      Inadmissible,
		"SetSpecNestedNil":       Inadmissible,
		"AddItemViaLocal":        Append,
		"AddLabelParenLocal":     Append,
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
