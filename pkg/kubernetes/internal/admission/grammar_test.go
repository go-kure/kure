package admission

import (
	"go/ast"
	"go/types"
	"os"
	"strings"
	"testing"
)

// grammarFixtureSource is the second file of the fixture package: the helpers
// that exist for the grammar, and the types only they need.
const grammarFixtureSource = `package fixture

import (
	"encoding/json"
	"fmt"
	"maps"
)

type Mode string

const ModeFixed Mode = "fixed"

type Blob struct{ Raw []byte }

type Policy struct {
	Mode  Mode
	Name  string
	Limit *int32
}

type Plan struct {
	Name   string
	Policy Policy
}

type Meta struct{ Labels map[string]string }

type Doc struct {
	Meta
	Blob    *Blob
	Plan    Plan
	Plans   []Plan
	HoldMap map[string]*Holder
}

func grow(o *Obj) int { return len(o.Spec.Items) }

func glue(k *string, v string) string { return *k + v }

func marshal(v any) ([]byte, error) { return json.Marshal(v) }

// class b: the marshalled local, its error guard and the literal carrying it
func SetDocBlob(d *Doc, v map[string]any) {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("SetDocBlob: %v", err))
	}
	d.Blob = &Blob{Raw: raw}
}

// class a: a guard on a parameter the body dereferences
func AddItemFromPointer(o *Obj, s *string) {
	if o == nil {
		panic("AddItemFromPointer: o must not be nil")
	}
	if s == nil {
		panic("AddItemFromPointer: s must not be nil")
	}
	o.Spec.Items = append(o.Spec.Items, *s)
}

// class a: named constants of a defined type beside an argument, two levels deep
func AddDocPlan(d *Doc, name string, limit int32) {
	d.Plans = append(d.Plans, Plan{Name: name, Policy: Policy{Mode: ModeFixed, Limit: &limit}})
}

// class a: a nil-init through a nil-init
func AddHoldLabel(o *Obj, k, v string) {
	if o.Spec.Hold == nil {
		o.Spec.Hold = &Holder{}
	}
	if o.Spec.Hold.Labels == nil {
		o.Spec.Hold.Labels = map[string]string{}
	}
	o.Spec.Hold.Labels[k] = v
}

// class c: positional elements
func SetNestedRefPositional(o *Obj, name, kind string) { o.Spec.Nested.Ref = Ref{name, kind} }

// class b: a constant index is a constant however it is spelled, a conversion
// or a builtin the compiler evaluates included
func SetDocPlanLimitConstIndex(d *Doc, limit int32) { d.Plans[int(len("a"))].Policy.Limit = &limit }

// S2: a receiver guard after the write
func SetReplicasGuardLast(o *Obj, n int32) {
	o.Spec.Replicas = &n
	if o == nil {
		panic("SetReplicasGuardLast: o must not be nil")
	}
}

// S3: the error of the marshalled local is discarded
func SetDocBlobBlankErr(d *Doc, v map[string]any) {
	raw, _ := json.Marshal(v)
	d.Blob = &Blob{Raw: raw}
}

// S3: the local marshals a field of the object, not an argument
func SetDocBlobFromField(d *Doc) {
	raw, err := json.Marshal(d.Plan.Name)
	if err != nil {
		panic(fmt.Sprintf("SetDocBlobFromField: %v", err))
	}
	d.Blob = &Blob{Raw: raw}
}

// S3: the local is the result of another call
func SetDocBlobFromCall(d *Doc, v map[string]any) {
	raw, err := marshal(v)
	if err != nil {
		panic(fmt.Sprintf("SetDocBlobFromCall: %v", err))
	}
	d.Blob = &Blob{Raw: raw}
}

// S4: the error guard is not the statement after its local
func SetDocBlobGuardApart(d *Doc, v map[string]any) {
	raw, err := json.Marshal(v)
	if d == nil {
		panic("SetDocBlobGuardApart: d must not be nil")
	}
	if err != nil {
		panic(fmt.Sprintf("SetDocBlobGuardApart: %v", err))
	}
	d.Blob = &Blob{Raw: raw}
}

// S5: the panic message is an argument
func SetReplicasPanicParam(o *Obj, n int32, msg string) {
	if o == nil {
		panic(msg)
	}
	o.Spec.Replicas = &n
}

// S5: the panic message is a call
func AddLabelPanicCall(o *Obj, k, v string) {
	if o == nil {
		panic(glue(&k, v))
	}
	o.Labels[k] = v
}

// S6: a guard on a parameter the body writes as it is makes nil unexpressible
func SetRefGuarded(o *Obj, ref *Ref) {
	if ref == nil {
		panic("SetRefGuarded: ref must not be nil")
	}
	o.Spec.Ref = ref
}

// S7: a tuple assignment
func SetReplicasAndNameTuple(o *Obj, n int32, s string) {
	o.Spec.Replicas, o.Spec.Name = &n, s
}

// S7: the tuple writes the field of the parameter it replaces
func SetReplicasTupleReroot(o, p *Obj, n int32) {
	p, p.Spec.Replicas = o, &n
}

// S7: a compound assignment of an argument before it is written
func SetReplicasParamBumped(o *Obj, n int32) {
	n += 1
	o.Spec.Replicas = &n
}

// S8: validation of an argument
func SetReplicasValidated(o *Obj, n int32) {
	if n < 0 {
		panic("SetReplicasValidated: n must not be negative")
	}
	o.Spec.Replicas = &n
}

// S8: a guard with an else branch
func SetReplicasGuardElse(o *Obj, n int32) {
	if o == nil {
		panic("SetReplicasGuardElse: o must not be nil")
	} else {
		_ = n
	}
	o.Spec.Replicas = &n
}

// S8: the object is replaced on one path
func SetReplicasParamReassigned(o, p *Obj, n int32) {
	if p != nil {
		o = p
	}
	o.Spec.Replicas = &n
}

// P1: two objects
func SetReplicasTwoObjects(a, b *Obj, n int32, s string) {
	a.Spec.Replicas = &n
	b.Spec.Name = s
}

// P1: the argument is replaced before it is written
func SetReplicasParamOverwritten(o *Obj, n int32) {
	n = 5
	o.Spec.Replicas = &n
}

// P1: the argument is defaulted before it is written
func SetRefDefaulted(o *Obj, ref *Ref) {
	if ref == nil {
		ref = &Ref{}
	}
	o.Spec.Ref = ref
}

// P2: the object itself is replaced
func SetObjWhole(o *Obj, m map[string]string, n string) {
	*o = Obj{Labels: m, Spec: Spec{Name: n}}
}

// P3: the key is read from the object
func AddLabelFieldKey(o *Obj, v string) { o.Labels[o.Spec.Name] = v }

// P4: an insert into the map the body just replaced
func AddLabelAfterReplace(o *Obj, m map[string]string, k, v string) {
	o.Labels = m
	o.Labels[k] = v
}

// P4: the struct behind the pointer is replaced, then appended through
func AddHoldItemAfterCopy(o *Obj, h Holder, s string) {
	if o.Spec.Hold == nil {
		o.Spec.Hold = &Holder{}
	}
	*o.Spec.Hold = h
	o.Spec.Hold.Items = append(o.Spec.Hold.Items, s)
}

// P4: the same field spelled through the embedded struct and promoted
func AddDocLabelAfterReplace(d *Doc, m map[string]string, k, v string) {
	d.Meta.Labels = m
	d.Labels[k] = v
}

// P4: the pointer written through is replaced afterwards
func SetRefNameThenReplace(o *Obj, n string, ref *Ref) {
	if o.Spec.Ref == nil {
		o.Spec.Ref = &Ref{}
	}
	o.Spec.Ref.Name = n
	o.Spec.Ref = ref
}

// P5: the nil-init of a map element is an insert of a value the body chose
func SetDocHoldLabels(d *Doc, k string, m map[string]string) {
	if d.HoldMap[k] == nil {
		d.HoldMap[k] = &Holder{}
	}
	d.HoldMap[k].Labels = m
}

// P5: the same, with nothing written through it
func AddGroupInit(o *Obj, k string) {
	if o.Spec.Groups[k] == nil {
		o.Spec.Groups[k] = []string{}
	}
}

// P6: a nil-init nothing is written through
func SetReplicasUnusedInit(o *Obj, n int32) {
	if o.Labels == nil {
		o.Labels = map[string]string{}
	}
	o.Spec.Replicas = &n
}

// N1: a call in the capacity of the nil-init
func AddItemGrow(o *Obj, s string) {
	if o.Spec.Items == nil {
		o.Spec.Items = make([]string, 0, grow(o))
	}
	o.Spec.Items = append(o.Spec.Items, s)
}

// V1: the appended value is read from the object
func AddItemFromName(o *Obj) { o.Spec.Items = append(o.Spec.Items, o.Spec.Name) }

// V1: the inserted value is read from the object
func AddGroupFromGroup(o *Obj, j, k string) { o.Spec.Groups[j] = o.Spec.Groups[k] }

// V1: the inserted value is the result of a call
func AddLabelCallValue(o *Obj, k, v string) { o.Labels[k] = glue(&k, v) }

// V1: a constant written through a pointer the body initialised
func SetBoxNameFixed(o *Obj) {
	if o.Spec.Box == nil {
		o.Spec.Box = &Box{}
	}
	o.Spec.Box.Name = "fixed"
}

// V1: a map literal replaces the field beside the append
func AddItemAndLabels(o *Obj, s, k, v string) {
	o.Spec.Items = append(o.Spec.Items, s)
	o.Labels = map[string]string{k: v}
}

// V2: a struct literal replaces a struct beside another write
func SetReplicasAndNestedA(o *Obj, n int32, a string) {
	o.Spec.Replicas = &n
	o.Spec.Nested = Inner{A: a}
}

// V3: an inline constant in the literal
func SetNestedRefKind(o *Obj, name string) {
	o.Spec.Nested.Ref = Ref{Name: name, Kind: "Deployment"}
}

// V3b: a nested literal built from constants only
func SetDocPlanFixedPolicy(d *Doc, name string) {
	d.Plan = Plan{Name: name, Policy: Policy{Mode: ModeFixed}}
}

// V3b: an unguarded empty literal, then an append through it
func AddHoldItemUnguardedInit(o *Obj, s string) {
	o.Spec.Hold = &Holder{}
	o.Spec.Hold.Items = append(o.Spec.Hold.Items, s)
}

// V3b: an unguarded empty literal, then a write through it
func SetBoxNameUnguardedInit(o *Obj, name string) {
	o.Spec.Box = &Box{}
	o.Spec.Box.Name = name
}

// V4: one argument written to two fields
func SetReplicasNameTwice(o *Obj, n int32, s string) {
	o.Spec.Replicas = &n
	o.Spec.Name = s
	o.Spec.Nested.A = s
}

// V4: one argument twice in one literal
func SetNestedRefSameName(o *Obj, n string) { o.Spec.Nested.Ref = Ref{Name: n, Kind: n} }

// S3: a local carrying an argument is overwritten before the append
func AddItemLaundered(o *Obj, s string) {
	x := s
	x = "fixed"
	o.Spec.Items = append(o.Spec.Items, x)
}

// S3: the same before a pointer write
func SetRefLaundered(o *Obj, ref *Ref) {
	x := ref
	x = &Ref{}
	o.Spec.Ref = x
}

// S3: the same inside a literal
func SetNestedRefLaundered(o *Obj, name, kind string) {
	x := name
	x = "fixed"
	o.Spec.Nested.Ref = Ref{Name: x, Kind: kind}
}

// S3: an alias replaced on one path
func SetReplicasConditionalAlias(o, p *Obj, n int32) {
	x := o
	if p != nil {
		x = p
	}
	x.Spec.Replicas = &n
}

// S3: an alias through the address of the parameter
func SetReplicasViaAddr(o *Obj, n int32) {
	pp := &o
	(*pp).Spec.Replicas = &n
}

// S1: a call statement beside the insert
func AddLabelDelete(o *Obj, k, v string) {
	delete(o.Labels, "x")
	o.Labels[k] = v
}

// S1: a call into another package beside the insert
func AddLabelMapsCopy(o *Obj, m map[string]string, k, v string) {
	maps.Copy(o.Labels, m)
	o.Labels[k] = v
}

// S1: a method call beside the append
func AddItemMethodCall(o *Obj, s string) {
	o.SetMethod("x")
	o.Spec.Items = append(o.Spec.Items, s)
}

// S1: a deferred call
func AddItemDeferred(o *Obj, s string) {
	defer o.SetMethod("x")
	o.Spec.Items = append(o.Spec.Items, s)
}

// S1: a go statement
func AddItemGo(o *Obj, s string) {
	go o.SetMethod("x")
	o.Spec.Items = append(o.Spec.Items, s)
}

// S1: a send
func AddItemSend(o *Obj, s string) {
	o.Spec.Box.Ch <- "x"
	o.Spec.Items = append(o.Spec.Items, s)
}
`

// refusal is one fixture that only the grammar refuses.
type refusal struct {
	// rule is the grammar rule that refuses the fixture first.
	rule string
	// was is the class the checks before the grammar give it.
	was Class
}

// grammarRefusals lists every fixture that only the grammar refuses. The
// first 14 are in fixtureSource: the checks before the grammar admit them.
var grammarRefusals = map[string]refusal{
	"SetReplicasViaAlias":          {"S1", Pointer},
	"SetRefAfterAlias":             {"S3", Pointer},
	"SetRefBlock":                  {"S1", Pointer},
	"SetReplicasTempCount":         {"S3", Pointer},
	"AddLabelFromTemp":             {"S3", Append},
	"AddLabelFromOther":            {"V1", Append},
	"SetReplicasRangeTempConcat":   {"S1", Pointer},
	"SetReplicasRangeCopyConcat":   {"S1", Pointer},
	"SetReplicasCommaOkTemp":       {"S3", Pointer},
	"SetRefNameComputed":           {"V1", Pointer},
	"SetReplicasClonedInc":         {"S3", Pointer},
	"AddLabelFromOtherKey":         {"V1", Append},
	"SetRefNameViaParenLocal":      {"S1", Pointer},
	"SetReplicasTempSliceExpanded": {"S3", Pointer},

	"AddLabelDelete":              {"S1", Append},
	"AddLabelMapsCopy":            {"S1", Append},
	"AddItemMethodCall":           {"S1", Append},
	"AddItemDeferred":             {"S1", Append},
	"AddItemGo":                   {"S1", Append},
	"AddItemSend":                 {"S1", Append},
	"SetReplicasGuardLast":        {"S2", Pointer},
	"SetDocBlobBlankErr":          {"S3", Pointer},
	"SetDocBlobFromField":         {"S3", Pointer},
	"SetDocBlobFromCall":          {"S3", Pointer},
	"AddItemLaundered":            {"S3", Append},
	"SetRefLaundered":             {"S3", Pointer},
	"SetNestedRefLaundered":       {"S3", Composite},
	"SetReplicasConditionalAlias": {"S3", Pointer},
	"SetReplicasViaAddr":          {"S3", Pointer},
	"SetDocBlobGuardApart":        {"S4", Pointer},
	"SetReplicasPanicParam":       {"S5", Pointer},
	"AddLabelPanicCall":           {"S5", Append},
	"SetRefGuarded":               {"S6", Pointer},
	"SetReplicasAndNameTuple":     {"S7", Pointer},
	"SetReplicasTupleReroot":      {"S7", Pointer},
	"SetReplicasParamBumped":      {"S7", Pointer},
	"SetReplicasValidated":        {"S8", Pointer},
	"SetReplicasGuardElse":        {"S8", Pointer},
	"SetReplicasParamReassigned":  {"S8", Pointer},
	"SetReplicasTwoObjects":       {"P1", Pointer},
	"SetReplicasParamOverwritten": {"P1", Pointer},
	"SetRefDefaulted":             {"P1", Pointer},
	"SetObjWhole":                 {"P2", Composite},
	"AddLabelFieldKey":            {"P3", Append},
	"AddLabelAfterReplace":        {"P4", Append},
	"AddHoldItemAfterCopy":        {"P4", Append},
	"AddDocLabelAfterReplace":     {"P4", Append},
	"SetRefNameThenReplace":       {"P4", Pointer},
	"SetDocHoldLabels":            {"P5", Append},
	"AddGroupInit":                {"P5", Append},
	"SetReplicasUnusedInit":       {"P6", Pointer},
	"AddItemGrow":                 {"N1", Append},
	"AddItemFromName":             {"V1", Append},
	"AddGroupFromGroup":           {"V1", Append},
	"AddLabelCallValue":           {"V1", Append},
	"SetBoxNameFixed":             {"V1", Pointer},
	"AddItemAndLabels":            {"V1", Append},
	"SetReplicasAndNestedA":       {"V2", Pointer},
	"SetNestedRefKind":            {"V3", Composite},
	"SetDocPlanFixedPolicy":       {"V3b", Composite},
	"AddHoldItemUnguardedInit":    {"V3b", Append},
	"SetBoxNameUnguardedInit":     {"V3b", Pointer},
	"SetReplicasNameTwice":        {"V4", Pointer},
	"SetNestedRefSameName":        {"V4", Composite},
}

// grammarAdmitted lists the fixtures in grammarFixtureSource that stay
// admitted: the shapes the grammar must not refuse.
var grammarAdmitted = map[string]Class{
	"SetDocBlob":                Pointer,
	"AddItemFromPointer":        Append,
	"AddDocPlan":                Append,
	"AddHoldLabel":              Append,
	"SetNestedRefPositional":    Composite,
	"SetDocPlanLimitConstIndex": Pointer,
}

// TestClassify_GrammarOnlyRefuses classifies the fixture with and without the
// grammar. The grammar may turn an admission into a refusal, for the fixtures
// grammarRefusals names and by the rule it names, and may change nothing else:
// not the class of an admitted helper, and not the reason of a refusal the
// checks before it already make.
func TestClassify_GrammarOnlyRefuses(t *testing.T) {
	dir := writeFixture(t)
	classifyFixture := func() []Finding {
		t.Helper()
		findings, err := Classify(Options{
			Dir:      dir,
			Patterns: []string{"."},
			Exempt:   map[string]bool{"fixture.SetExempted": true},
			Env:      append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod"),
		})
		if err != nil {
			t.Fatal(err)
		}
		return findings
	}
	with := classifyFixture()

	restore := checkGrammar
	t.Cleanup(func() { checkGrammar = restore })
	checkGrammar = func(*ast.FuncDecl, *types.Info) string { return "" }
	without := classifyFixture()

	if len(with) != len(without) {
		t.Fatalf("%d findings with the grammar, %d without", len(with), len(without))
	}
	seen := map[string]bool{}
	for i, before := range without {
		after := with[i]
		if before.Name != after.Name {
			t.Fatalf("finding %d is %s without the grammar and %s with it", i, before.Name, after.Name)
		}
		seen[before.Name] = true
		want, refused := grammarRefusals[before.Name]
		if !refused {
			if before.Class != after.Class || before.Reason != after.Reason {
				t.Errorf("%s: %s (%s) without the grammar, %s (%s) with it",
					before.Name, before.Class, before.Reason, after.Class, after.Reason)
			}
			continue
		}
		if before.Class != want.was {
			t.Errorf("%s: class %s without the grammar, want %s (%s)", before.Name, before.Class, want.was, before.Reason)
		}
		if after.Class != Inadmissible || !strings.Contains(after.Reason, "(grammar "+want.rule+",") {
			t.Errorf("%s: %s (%s) with the grammar, want it refused by rule %s", before.Name, after.Class, after.Reason, want.rule)
		}
	}
	for name := range grammarRefusals {
		if !seen[name] {
			t.Errorf("%s: not classified at all", name)
		}
	}
}
