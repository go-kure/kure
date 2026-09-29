package admission

import (
	"go/ast"
	"go/parser"
	"go/token"
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
	"unsafe"
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

type Shell struct {
	Box   *Box
	Hold  *Holder
	Slots [3]*int32
}

type Doc struct {
	Shell   *Shell
	Meta
	Blob    *Blob
	Plan    Plan
	Plans   []Plan
	HoldMap map[string]*Holder
	Spare   *Blob
	Name    string
	Any     any
	Keyed   map[any]*int32
	RawP    *[]byte
	Ref     *Ref
	Holds   []Holder
	Held    map[string]Holder
	Errs    []error
	*Note
	Cells *[3]string
}

type Note struct{ Text string }

func grow(o *Obj) int { return len(o.Spec.Items) }

func glue(k *string, v string) string { return *k + v }

func marshal(v any) ([]byte, error) { return json.Marshal(v) }

// Marshal has the name of json.Marshal and is not it.
func Marshal(v any) ([]byte, error) { return json.Marshal(v) }

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

// class b: a nil-init guard and the write through it, on the element a
// constant index names; the index is a conversion, which is no literal and no
// identifier, and is the same index each time by its value
func SetDocPlanLimitGuardedConstIndex(d *Doc, limit int32) {
	if d.Plans[int(0)].Policy.Limit == nil {
		d.Plans[int(0)].Policy.Limit = new(int32)
	}
	*d.Plans[int(0)].Policy.Limit = limit
}

// class a: a struct literal is the inserted element, beside another write
func AddDocHeld(d *Doc, k string, items []string, s string) {
	d.Held[k] = Holder{Items: items}
	d.Name = s
}

// class b: the condition of a guard in parentheses
func SetReplicasParenGuard(o *Obj, n int32) {
	if (o == nil) {
		panic("SetReplicasParenGuard: o must not be nil")
	}
	o.Spec.Replicas = &n
}

// class a: a key in parentheses is a key
func AddLabelParenKey(o *Obj, k, v string) { o.Labels[(k)] = v }

// class b: the address of a literal into a pointer field, beside another write
func SetDocNameAndRef(d *Doc, s, name string) {
	d.Name = s
	d.Ref = &Ref{Name: name}
}

// class a: a guard on a parameter the body dereferences inside a literal
func AddDocPlanFromPointer(d *Doc, name *string) {
	if name == nil {
		panic("AddDocPlanFromPointer: name must not be nil")
	}
	d.Plans = append(d.Plans, Plan{Name: *name})
}

// class b: two locals, each with an error of its own
func SetDocBlobAndSpare(d *Doc, v, w map[string]any) {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("SetDocBlobAndSpare: %v", err))
	}
	spare, err2 := json.Marshal(w)
	if err2 != nil {
		panic(fmt.Sprintf("SetDocBlobAndSpare: %v", err2))
	}
	d.Blob = &Blob{Raw: raw}
	d.Spare = &Blob{Raw: spare}
}

// class b: a nil guard after a local and its error guard
func SetDocBlobGuardAfter(d *Doc, v map[string]any) {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("SetDocBlobGuardAfter: %v", err))
	}
	if d == nil {
		panic("SetDocBlobGuardAfter: d must not be nil")
	}
	d.Blob = &Blob{Raw: raw}
}

// class a: an append behind a nil-init needs no slice there
func AddHoldItemInit(o *Obj, s string) {
	if o.Spec.Hold == nil {
		o.Spec.Hold = &Holder{}
	}
	o.Spec.Hold.Items = append(o.Spec.Hold.Items, s)
}

// class b: an array behind a nil-init has its elements
func SetDocShellSlot(d *Doc, i int, n int32) {
	if d.Shell == nil {
		d.Shell = &Shell{}
	}
	d.Shell.Slots[i] = &n
}

// P4: the nil-init is of element 1 and the write through element 2, two
// constant indexes that print alike
func SetDocPlanLimitAbbreviatedIndex(d *Doc, limit int32) {
	if d.Plans[len([...]int{1})].Policy.Limit == nil {
		d.Plans[len([...]int{1})].Policy.Limit = new(int32)
	}
	*d.Plans[len([...]int{1, 2})].Policy.Limit = limit
}

// N1: the guard tests element 1, and the body initialises element 2 and
// writes through it, two constant indexes that print alike
func SetDocPlanLimitGuardOtherIndex(d *Doc, limit int32) {
	if d.Plans[len([...]int{1})].Policy.Limit == nil {
		d.Plans[len([...]int{1, 2})].Policy.Limit = new(int32)
	}
	*d.Plans[len([...]int{1, 2})].Policy.Limit = limit
}

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

// S3: the local marshals the object
func SetDocBlobFromSelf(d *Doc) {
	raw, err := json.Marshal(d)
	if err != nil {
		panic(fmt.Sprintf("SetDocBlobFromSelf: %v", err))
	}
	d.Blob = &Blob{Raw: raw}
}

// S3: the local is the result of a Marshal that is not encoding/json's
func SetDocBlobOtherMarshal(d *Doc, v map[string]any) {
	raw, err := Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("SetDocBlobOtherMarshal: %v", err))
	}
	d.Blob = &Blob{Raw: raw}
}

// S3: the second local declares one new name, its error is the first one's
func SetDocBlobErrReused(d *Doc, v, w map[string]any) {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("SetDocBlobErrReused: %v", err))
	}
	spare, err := json.Marshal(w)
	if err != nil {
		panic(fmt.Sprintf("SetDocBlobErrReused: %v", err))
	}
	d.Blob = &Blob{Raw: raw}
	d.Spare = &Blob{Raw: spare}
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

// S4: a second guard on the error, after the one that follows the local
func SetDocBlobGuardTwice(d *Doc, v map[string]any) {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("SetDocBlobGuardTwice: %v", err))
	}
	if err != nil {
		panic("SetDocBlobGuardTwice: again")
	}
	d.Blob = &Blob{Raw: raw}
}

// S5: the error guard formats another value than the error
func SetDocBlobPanicValue(d *Doc, v map[string]any) {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("SetDocBlobPanicValue: %v", v))
	}
	d.Blob = &Blob{Raw: raw}
}

// S5: a nil guard formats its message
func SetReplicasPanicFormatted(o *Obj, n int32) {
	if o == nil {
		panic(fmt.Sprintf("SetReplicasPanicFormatted: %v", o))
	}
	o.Spec.Replicas = &n
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

// S8: a guard that panics when the object is set
func SetReplicasGuardNotNil(o *Obj, n int32) {
	if o != nil {
		panic("SetReplicasGuardNotNil: o must be nil")
	}
	o.Spec.Replicas = &n
}

// S8: a guard with an init statement
func SetReplicasGuardInit(o *Obj, n int32) {
	if _ = n; o == nil {
		panic("SetReplicasGuardInit: o must not be nil")
	}
	o.Spec.Replicas = &n
}

// P1: two objects
func SetReplicasTwoObjects(a, b *Obj, n int32, s string) {
	a.Spec.Replicas = &n
	b.Spec.Name = s
}

// P1: a write through an argument the body also writes
func SetRefAndRename(o *Obj, ref *Ref, name string) {
	o.Spec.Ref = ref
	ref.Name = name
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

// P3: the key is the object
func AddDocKeyedSelf(d *Doc, n int32) { d.Keyed[d] = &n }

// P3: the key is a local; a local has no place but a written value, and one
// the body uses nowhere does not compile
func AddDocKeyedLocal(d *Doc, v map[string]any, n int32) {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("AddDocKeyedLocal: %v", err))
	}
	d.Keyed[raw] = &n
}

// S8: the local is tested, not written
func AddDocKeyedLocalGuarded(d *Doc, v map[string]any, n int32) {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("AddDocKeyedLocalGuarded: %v", err))
	}
	if raw == nil {
		panic("AddDocKeyedLocalGuarded: v must not be empty")
	}
	d.Keyed["k"] = &n
}

// P1: the local is assigned to nothing
func AddDocKeyedLocalDropped(d *Doc, v map[string]any, n int32) {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("AddDocKeyedLocalDropped: %v", err))
	}
	_ = raw
	d.Keyed["k"] = &n
}

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
	o.Spec.Ref.Name = n
	o.Spec.Ref = ref
}

// P4: the field a nil-init initialises is replaced, not written through
func SetLabelsAfterInit(o *Obj, m map[string]string, n int32) {
	if o.Labels == nil {
		o.Labels = map[string]string{}
	}
	o.Labels = m
	o.Spec.Replicas = &n
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

// P7: the slice the body initialised is empty, and the write indexes it
func SetItemAtInit(o *Obj, i int, v string, n int32) {
	if o.Spec.Items == nil {
		o.Spec.Items = []string{}
	}
	o.Spec.Items[i] = v
	o.Spec.Replicas = &n
}

// P7: the same slice, and the write appends to an element of it
func AddRowItemInit(o *Obj, i int, s string) {
	if o.Spec.Rows == nil {
		o.Spec.Rows = [][]string{}
	}
	o.Spec.Rows[i] = append(o.Spec.Rows[i], s)
}

// P7: the slice behind the pointer the body initialised is nil
func SetHoldItemAtInit(o *Obj, i int, v string, n int32) {
	if o.Spec.Hold == nil {
		o.Spec.Hold = &Holder{}
	}
	o.Spec.Hold.Items[i] = v
	o.Spec.Replicas = &n
}

// P7: the map behind the pointer the body initialised is nil
func AddHoldLabelOneInit(o *Obj, k, v string) {
	if o.Spec.Hold == nil {
		o.Spec.Hold = &Holder{}
	}
	o.Spec.Hold.Labels[k] = v
}

// P7: the pointer behind the pointer the body initialised is nil
func SetDocShellBoxName(d *Doc, s string, ref *Ref) {
	if d.Shell == nil {
		d.Shell = &Shell{}
	}
	d.Shell.Box.Name = s
	d.Ref = ref
}

// P7: the map the body initialised has no element to write through
func SetDocHoldMapLabelsInit(d *Doc, k string, m map[string]string, ref *Ref) {
	if d.HoldMap == nil {
		d.HoldMap = map[string]*Holder{}
	}
	d.HoldMap[k].Labels = m
	d.Ref = ref
}

// P7: the second nil-init is behind a pointer the first left nil
func AddDocShellHoldLabel(d *Doc, k, v string) {
	if d.Shell == nil {
		d.Shell = &Shell{}
	}
	if d.Shell.Hold.Labels == nil {
		d.Shell.Hold.Labels = map[string]string{}
	}
	d.Shell.Hold.Labels[k] = v
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

// V1: the value is the object
func SetDocRefAndAnySelf(d *Doc, ref *Ref) {
	d.Ref = ref
	d.Any = d
}

// V1: the value is the address of a local
func SetDocRawPointer(d *Doc, v map[string]any, s string) {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("SetDocRawPointer: %v", err))
	}
	d.RawP = &raw
	d.Name = s
}

// V2: a struct literal replaces a struct beside another write
func SetReplicasAndNestedA(o *Obj, n int32, a string) {
	o.Spec.Replicas = &n
	o.Spec.Nested = Inner{A: a}
}

// V2: the address of a literal into a field that is no pointer, beside
// another write
func SetDocRefAndAnyRef(d *Doc, ref *Ref, name string) {
	d.Ref = ref
	d.Any = &Ref{Name: name}
}

// V2: a struct literal replaces an element of a slice beside another write;
// the element of a slice is there already, that of a map is inserted
func SetDocRefAndHold(d *Doc, ref *Ref, i int, items []string) {
	d.Ref = ref
	d.Holds[i] = Holder{Items: items}
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

// V4: the argument a local marshals is written as it is as well
func SetDocBlobAndAny(d *Doc, v map[string]any) {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("SetDocBlobAndAny: %v", err))
	}
	d.Blob = &Blob{Raw: raw}
	d.Any = v
}

// V5: a parameter the body reads nowhere
func SetRefIgnoring(o *Obj, ref *Ref, ignored string) { o.Spec.Ref = ref }

// V5: a blank parameter has no place either
func SetRefBlank(o *Obj, ref *Ref, _ string) { o.Spec.Ref = ref }

// V1: the error of a local is never a value; here it is appended
func AddDocErr(d *Doc, v map[string]any) {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("AddDocErr: %v", err))
	}
	d.Blob = &Blob{Raw: raw}
	d.Errs = append(d.Errs, err)
}

// N1: the nil-init is new of a value, not of a type
func SetDocPlanLimitNewValue(d *Doc, limit int32) {
	if d.Plan.Policy.Limit == nil {
		d.Plan.Policy.Limit = new(int32(1))
	}
	*d.Plan.Policy.Limit = limit
}

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

// S9: a function literal inside a constant key
func AddDocKeyedFuncKey(d *Doc, n *int32) { d.Keyed[len([1]func(){func() {}})] = n }

// class a: the type of a function inside a constant key is no function literal
func AddDocKeyedFuncTypeKey(d *Doc, n *int32) { d.Keyed[len([1]func(){})] = n }

// S11: an append inside a constant key
func AddDocKeyedAppendKey(d *Doc, n *int32) { d.Keyed[unsafe.Sizeof(append([]int{}, 0))] = n }

// S10: two appends to two fields
func AddItemAndRow(o *Obj, s string, r []string) {
	o.Spec.Items = append(o.Spec.Items, s)
	o.Spec.Rows = append(o.Spec.Rows, r)
}

// S10: an append and an insert
func AddItemAndLabel(o *Obj, s, k, v string) {
	o.Spec.Items = append(o.Spec.Items, s)
	o.Labels[k] = v
}

// S10: two inserts into two maps
func AddDocLabelAndHeld(d *Doc, k, v, j string, h Holder) {
	d.Labels[k] = v
	d.Held[j] = h
}

// N1: the guard is spelled otherwise than the path it initialises
func SetRefNameGuardDerefObj(o *Obj, n string) {
	if (*o).Spec.Ref == nil {
		o.Spec.Ref = &Ref{}
	}
	o.Spec.Ref.Name = n
}

// class b: the operand of the guard in parentheses is spelled as the path
func SetRefNameParenGuardOperand(o *Obj, n string) {
	if (o.Spec.Ref) == nil {
		o.Spec.Ref = &Ref{}
	}
	o.Spec.Ref.Name = n
}

// class b: guard, nil-init and write all spelled from (*o)
func SetRefNameDerefObj(o *Obj, n string) {
	if (*o).Spec.Ref == nil {
		(*o).Spec.Ref = &Ref{}
	}
	(*o).Spec.Ref.Name = n
}

// V7: the nil-init's capacity reads the slice it initialises
func AddItemMakeCapReadsField(o *Obj, s string) {
	if o.Spec.Items == nil {
		o.Spec.Items = make([]string, 0, unsafe.Sizeof(o.Spec.Items))
	}
	o.Spec.Items = append(o.Spec.Items, s)
}

// V7: the type of the literal written reads the field it replaces
func SetDocAnyReadsAny(d *Doc, n string, m Mode) {
	d.Any = struct {
		A    [unsafe.Sizeof(d.Any)%1 + 1]int
		Name string
		Mode Mode
	}{Name: n, Mode: m}
}

// class c: V7 reads the target as written, parentheses aside; (*d).Any and
// d.Any are one field in two spellings, and the type of the literal reads the
// other spelling, which V7 does not equate with the target
func SetDocAnyReadsAnyDerefObj(d *Doc, n string, m Mode) {
	(*d).Any = struct {
		A    [unsafe.Sizeof(d.Any)%1 + 1]int
		Name string
		Mode Mode
	}{Name: n, Mode: m}
}

// class c: the type of the literal written reads another field
func SetDocAnyReadsName(d *Doc, n string, m Mode) {
	d.Any = struct {
		A    [unsafe.Sizeof(d.Name)%1 + 1]int
		Name string
		Mode Mode
	}{Name: n, Mode: m}
}

// P6: the write through the nil-init is spelled (*F).x, not F.x or *F
func SetRefNameStarWrite(o *Obj, n string) {
	if o.Spec.Ref == nil {
		o.Spec.Ref = &Ref{}
	}
	(*o.Spec.Ref).Name = n
}

// P6: the write through an embedded pointer names a promoted field
func SetDocText(d *Doc, s string) {
	if d.Note == nil {
		d.Note = &Note{}
	}
	d.Text = s
}

// P6: the write through a pointer to an array indexes it
func SetDocCell(d *Doc, s string) {
	if d.Cells == nil {
		d.Cells = &[3]string{}
	}
	d.Cells[0] = s
}

// P6: the nil-init is spelled in parentheses and the write through it is not;
// (o.Spec.Ref) and o.Spec.Ref are one path spelled two ways
func SetRefNameParenInitPlainWrite(o *Obj, n string) {
	if o.Spec.Ref == nil {
		(o.Spec.Ref) = &Ref{}
	}
	o.Spec.Ref.Name = n
}

// class b: the write through a parenthesised nil-init is spelled alike
func SetRefNameParenInit(o *Obj, n string) {
	if o.Spec.Ref == nil {
		(o.Spec.Ref) = &Ref{}
	}
	(o.Spec.Ref).Name = n
}

// class a: an append behind the nil-init of a pointer to a slice
func AddDocRawByte(d *Doc, c byte) {
	if d.RawP == nil {
		d.RawP = &[]byte{}
	}
	*d.RawP = append(*d.RawP, c)
}

// class b: the slice behind the nil-init of a pointer to it, replaced whole
func SetDocRawWhole(d *Doc, raw []byte) {
	if d.RawP == nil {
		d.RawP = &[]byte{}
	}
	*d.RawP = raw
}

// P6: an element of the slice behind the nil-init of a pointer to it; the
// index (*d.RawP)[0] is not spelled d.RawP.… or *d.RawP, so the pointer
// nil-init is not written through
func SetDocRawFirst(d *Doc, c byte) {
	if d.RawP == nil {
		d.RawP = &[]byte{}
	}
	(*d.RawP)[0] = c
}

// V1: a parameter named nil, refused by that name
func SetRefFromNilParam(o *Obj, nil *Ref) { o.Spec.Ref = nil }

// V1: a parameter named nil as a keyed element of the literal
func SetDocPolicyNilLimit(d *Doc, name string, nil *int32) {
	d.Plan.Policy = Policy{Name: name, Limit: nil}
}

// class c: a parameter named nil as a keyed element of a literal that is a
// positional element of the one written; a positional element is not read
// for the name nil
func SetDocPlanPositionalNilLimit(d *Doc, plan, policy string, nil *int32) {
	d.Plan = Plan{plan, Policy{Name: policy, Limit: nil}}
}

// class b: a parameter named nil into a field that cannot hold nil
func SetReplicasAndNilName(o *Obj, n int32, nil string) {
	o.Spec.Replicas = &n
	o.Spec.Name = nil
}

// V6: &P into a field that is not pointer-typed, beside a pointer write
func SetReplicasAndPayloadAddr(o *Obj, n int32, s string) {
	o.Spec.Replicas = &n
	o.Spec.Payload = &s
}

// V6: the same, *P
func SetReplicasAndCountDeref(o *Obj, n int32, c *int) {
	o.Spec.Replicas = &n
	o.Spec.Count = *c
}

// class b: *P written through the pointer the nil-init allocated
func SetRefNameFromPointer(o *Obj, n *string) {
	if o.Spec.Ref == nil {
		o.Spec.Ref = &Ref{}
	}
	o.Spec.Ref.Name = *n
}

// S1: a return at the top level, after the write
func SetReplicasThenReturn(o *Obj, n int32) {
	o.Spec.Replicas = &n
	return
}

// S1: a goto at the top level
func SetReplicasGotoAhead(o *Obj, n int32) {
	goto set
set:
	o.Spec.Replicas = &n
}

// class b: a path through a map element is no insert; the write that makes
// the class is the pointer-typed one
func SetDocRefAndHoldMapLabels(d *Doc, k string, m map[string]string, ref *Ref) {
	d.HoldMap[k].Labels = m
	d.Ref = ref
}

// class c: a path through a pointer past the object is no pointer-typed
// target
func SetDocRefThrough(d *Doc, n, k string) { *d.Ref = Ref{Name: n, Kind: k} }

// class b: a pointer-typed target whose value is a literal of two fields is
// class b before class c
func SetRefWhole(o *Obj, n, k string) { o.Spec.Ref = &Ref{Name: n, Kind: k} }

// class c: an address written to a field that is no pointer is no
// pointer-typed target
func SetDocAnyRef(d *Doc, n, k string) { d.Any = &Ref{Name: n, Kind: k} }

// class b: two nested pointer nil-inits, the outer written through by the
// inner and the inner by the write
func SetDocShellBoxNameTwoInits(d *Doc, n string) {
	if d.Shell == nil {
		d.Shell = &Shell{}
	}
	if d.Shell.Box == nil {
		d.Shell.Box = &Box{}
	}
	d.Shell.Box.Name = n
}

// inadmissible: two arguments forwarded to two fields are two forwarders
func SetNameAndATwoArgs(o *Obj, n, a string) {
	o.Spec.Name = n
	o.Spec.Nested.A = a
}

// inadmissible: a guard and no write
func SetNothingButGuard(o *Obj) {
	if o == nil {
		panic("SetNothingButGuard: o must not be nil")
	}
}
`

// grammarRules lists every fixture refused for its body, with the rule that
// refuses it first. A fixture refused off its signature, for returning a
// value or for declaring type parameters, is not read and is not here; the
// forwarders, which the grammar admits and the class of their writes refuses,
// are in grammarForwarders.
var grammarRules = map[string]string{
	"SetReplicasViaAlias":          "S1",
	"SetRefAfterAlias":             "S3",
	"SetRefBlock":                  "S1",
	"SetReplicasTempCount":         "S3",
	"AddLabelFromTemp":             "S3",
	"AddLabelFromOther":            "V1",
	"SetReplicasRangeTempConcat":   "S1",
	"SetReplicasRangeCopyConcat":   "S1",
	"SetReplicasCommaOkTemp":       "S3",
	"SetRefNameComputed":           "V1",
	"SetReplicasClonedInc":         "S3",
	"AddLabelFromOtherKey":         "V1",
	"SetRefNameViaParenLocal":      "S1",
	"SetReplicasTempSliceExpanded": "S3",

	"AddLabelDelete":                  "S1",
	"AddLabelMapsCopy":                "S1",
	"AddItemMethodCall":               "S1",
	"AddItemDeferred":                 "S1",
	"AddItemGo":                       "S1",
	"AddItemSend":                     "S1",
	"SetReplicasGuardLast":            "S2",
	"SetDocBlobBlankErr":              "S3",
	"SetDocBlobFromField":             "S3",
	"SetDocBlobFromCall":              "S3",
	"SetDocBlobFromSelf":              "S3",
	"SetDocBlobOtherMarshal":          "S3",
	"SetDocBlobErrReused":             "S3",
	"AddItemLaundered":                "S3",
	"SetRefLaundered":                 "S3",
	"SetNestedRefLaundered":           "S3",
	"SetReplicasConditionalAlias":     "S3",
	"SetReplicasViaAddr":              "S3",
	"SetDocBlobGuardApart":            "S4",
	"SetDocBlobGuardTwice":            "S4",
	"SetReplicasPanicParam":           "S5",
	"AddLabelPanicCall":               "S5",
	"SetDocBlobPanicValue":            "S5",
	"SetReplicasPanicFormatted":       "S5",
	"SetRefGuarded":                   "S6",
	"SetReplicasAndNameTuple":         "S7",
	"SetReplicasTupleReroot":          "S7",
	"SetReplicasParamBumped":          "S7",
	"SetReplicasValidated":            "S8",
	"SetReplicasGuardElse":            "S8",
	"SetReplicasParamReassigned":      "S8",
	"SetReplicasGuardNotNil":          "S8",
	"SetReplicasGuardInit":            "S8",
	"SetReplicasTwoObjects":           "P1",
	"SetRefAndRename":                 "P1",
	"SetReplicasParamOverwritten":     "P1",
	"SetRefDefaulted":                 "P1",
	"SetObjWhole":                     "P2",
	"AddLabelFieldKey":                "P3",
	"AddDocKeyedSelf":                 "P3",
	"AddDocKeyedLocal":                "P3",
	"AddDocKeyedLocalGuarded":         "S8",
	"AddDocKeyedLocalDropped":         "P1",
	"AddLabelAfterReplace":            "P4",
	"AddHoldItemAfterCopy":            "P4",
	"AddDocLabelAfterReplace":         "P4",
	"SetRefNameThenReplace":           "P4",
	"SetLabelsAfterInit":              "P4",
	"SetDocPlanLimitAbbreviatedIndex": "P4",
	"SetDocHoldLabels":                "P5",
	"AddGroupInit":                    "P5",
	"SetReplicasUnusedInit":           "P6",
	"SetItemAtInit":                   "P7",
	"AddRowItemInit":                  "P7",
	"SetHoldItemAtInit":               "P7",
	"AddHoldLabelOneInit":             "P7",
	"SetDocShellBoxName":              "P7",
	"SetDocHoldMapLabelsInit":         "P7",
	"AddDocShellHoldLabel":            "P7",
	"AddItemGrow":                     "N1",
	"SetDocPlanLimitGuardOtherIndex":  "N1",
	"AddItemFromName":                 "V1",
	"AddGroupFromGroup":               "V1",
	"AddLabelCallValue":               "V1",
	"SetBoxNameFixed":                 "V1",
	"AddItemAndLabels":                "V1",
	"SetDocRefAndAnySelf":             "V1",
	"SetDocRawPointer":                "V1",
	"SetReplicasAndNestedA":           "V2",
	"SetDocRefAndAnyRef":              "V2",
	"SetDocRefAndHold":                "V2",
	"SetNestedRefKind":                "V3",
	"SetDocPlanFixedPolicy":           "V3b",
	"AddHoldItemUnguardedInit":        "V3b",
	"SetBoxNameUnguardedInit":         "V3b",
	"SetReplicasNameTwice":            "V4",
	"SetNestedRefSameName":            "V4",
	"SetDocBlobAndAny":                "V4",
	"SetRefIgnoring":                  "V5",
	"SetRefBlank":                     "V5",

	"AddDocKeyedFuncKey":                   "S9",
	"AddDocKeyedAppendKey":                 "S11",
	"AddItemAndRow":                        "S10",
	"AddItemAndLabel":                      "S10",
	"AddDocLabelAndHeld":                   "S10",
	"AddItemGuardedMakeLen":                "N1",
	"SetRefNameGuardDerefObj":              "N1",
	"AddItemMakeCapReadsField":             "V7",
	"SetDocAnyReadsAny":                    "V7",
	"SetRefNameStarWrite":                  "P6",
	"SetDocText":                           "P6",
	"SetDocCell":                           "P6",
	"SetRefNameParenInitPlainWrite":        "P6",
	"SetRefFromNilParam":                   "V1",
	"SetDocPolicyNilLimit":                 "V1",
	"SetReplicasAndPayloadAddr":            "V6",
	"SetReplicasAndCountDeref":             "V6",
	"SetReplicasThenReturn":                "S1",
	"SetReplicasGotoAhead":                 "S1",
	"AddDocErr":                            "V1",
	"AddGroupItemCommaOkCrossKey":          "S3",
	"AddGroupItemConstKey":                 "S3",
	"AddGroupItemConvKeyDirect":            "P3",
	"AddGroupItemCrossKey":                 "S3",
	"AddGroupItemCrossKeyDirect":           "V1",
	"AddGroupItemIndexReassigned":          "S3",
	"AddGroupItemLiteralKeys":              "S3",
	"AddGroupItemLiteralKeysDirect":        "P3",
	"AddGroupItemSlicedAppend":             "S11",
	"AddGroupItemVarAppend":                "S11",
	"AddGroupItemViaLocal":                 "S3",
	"AddHoldItemViaLocal":                  "S3",
	"AddItemAliasMismatch":                 "S3",
	"AddItemAndName":                       "V1",
	"AddItemAppendCrossField":              "S3",
	"AddItemAppendFresh":                   "S3",
	"AddItemAppendWrittenBackField":        "S3",
	"AddItemCommaOkCrossField":             "S3",
	"AddItemCommaOkReassignedCrossField":   "S3",
	"AddItemConditionalAlias":              "S3",
	"AddItemConstant":                      "V1",
	"AddItemConvertedReassignedCrossField": "S3",
	"AddItemConvertedSource":               "S3",
	"AddItemCopiedSliceCrossField":         "S3",
	"AddItemCrossField":                    "S3",
	"AddItemFreshReassigned":               "S3",
	"AddItemFromParam":                     "P1",
	"AddItemFromRowDirect":                 "V1",
	"AddItemFullSliceSource":               "S3",
	"AddItemIfSet":                         "S8",
	"AddItemInClosure":                     "S9",
	"AddItemIndirectBesideStray":           "S11",
	"AddItemLocalBase":                     "S3",
	"AddItemLocalOnly":                     "S3",
	"AddItemLocalThenIndirect":             "S3",
	"AddItemMakeLen":                       "V1",
	"AddItemMakeParamLen":                  "N1",
	"AddItemMultiAssignedCrossField":       "S3",
	"AddItemMultiAssignedCrossFieldLast":   "S3",
	"AddItemNestedConstant":                "S11",
	"AddItemParamAddressTaken":             "S3",
	"AddItemParamRanged":                   "S1",
	"AddItemParamReassigned":               "S8",
	"AddItemParamRedeclared":               "S3",
	"AddItemParenAddressTaken":             "S3",
	"AddItemParenMultiReassigned":          "S3",
	"AddItemParenParamReassigned":          "P1",
	"AddItemParenReassignedCrossField":     "S3",
	"AddItemPartialSliceSource":            "S3",
	"AddItemRangeReassignedCrossField":     "S3",
	"AddItemReassignedCrossField":          "S3",
	"AddItemReassignedCrossFieldLast":      "S3",
	"AddItemReceiveSource":                 "S3",
	"AddItemRowCrossField":                 "S3",
	"AddItemSameFieldLocalBase":            "S3",
	"AddItemShadowedLocal":                 "S3",
	"AddItemSliceParam":                    "P1",
	"AddItemSliceReassignedCrossField":     "S3",
	"AddItemStrayBesideTwice":              "S11",
	"AddItemStructCopy":                    "S3",
	"AddItemSwapHidesWrite":                "S3",
	"AddItemSwappedSameField":              "S3",
	"AddItemToTemp":                        "S3",
	"AddItemTupleReroot":                   "S3",
	"AddItemTupleThroughInit":              "S7",
	"AddItemTwice":                         "S10",
	"AddItemTwiceBesideBare":               "S10",
	"AddItemTwoValues":                     "V1",
	"AddItemTwoWriteBacks":                 "S3",
	"AddItemTwoWriteBacksLast":             "S3",
	"AddItemTypeAssertSource":              "S3",
	"AddItemVarParenCrossField":            "S1",
	"AddItemViaAlias":                      "S3",
	"AddItemViaLocal":                      "S3",
	"AddItemViaLocalAndReplicas":           "S3",
	"AddItemViaVarLocal":                   "S1",
	"AddItemWriteBackFirst":                "S3",
	"AddItemsAppendSource":                 "S3",
	"AddItemsForLoop":                      "S1",
	"AddItemsLoop":                         "S1",
	"AddItemsNestedAppend":                 "S11",
	"AddItemsSpread":                       "V1",
	"AddLabelAddrDeref":                    "P1",
	"AddLabelConcatNilInit":                "S3",
	"AddLabelConcatViaLocal":               "S3",
	"AddLabelConstant":                     "V1",
	"AddLabelExpandedConcat":               "V1",
	"AddLabelExpandedCrossAlias":           "S3",
	"AddLabelExpandedFromAlias":            "S3",
	"AddLabelExpandedParen":                "V1",
	"AddLabelExpandedParenIndex":           "V1",
	"AddLabelExpandedViaLocal":             "S3",
	"AddLabelFreshMap":                     "S3",
	"AddLabelIfSetViaLocal":                "S3",
	"AddLabelLocalOnly":                    "S3",
	"AddLabelLocalThenName":                "S3",
	"AddLabelParenLocal":                   "S3",
	"AddLabelTwice":                        "S10",
	"AddLabelViaLocal":                     "S3",
	"AddRowMakeLen":                        "V1",
	"AddRowParenMakeLen":                   "V1",
	"SetBoxChanMake":                       "V1",
	"SetBoxPayloadMake":                    "V1",
	"SetBoxPayloadNew":                     "V1",
	"SetCountRangeKey":                     "S1",
	"SetDocPlanLimitNewValue":              "N1",
	"SetDocRawFirst":                       "P6",
	"SetHoldCommaOkCrossField":             "S3",
	"SetHoldFreshMapWriteBack":             "S3",
	"SetHoldItemsConverted":                "S3",
	"SetHoldItemsFreshConverted":           "S3",
	"SetItemsLiteral":                      "V1",
	"SetNameAndA":                          "V4",
	"SetNameIfSet":                         "S8",
	"SetNameTwice":                         "V1",
	"SetNameViaZeroLocal":                  "S1",
	"SetNestedRefConstant":                 "V3",
	"SetNestedRefIfSet":                    "S8",
	"SetNestedRefOnTemp":                   "S1",
	"SetNothing":                           "P1",
	"SetRefAndReplicasInGuard":             "S8",
	"SetRefBeforeAlias":                    "S3",
	"SetRefBlockIf":                        "S1",
	"SetRefElse":                           "S8",
	"SetRefGoto":                           "S8",
	"SetRefGuardedByOther":                 "N1",
	"SetRefIfSet":                          "S8",
	"SetRefIfUnsetNewValue":                "N1",
	"SetRefLabeledLoop":                    "S1",
	"SetRefNameElse":                       "S8",
	"SetRefNameGuardExtra":                 "S8",
	"SetRefNameIfInit":                     "S8",
	"SetRefNameIfNotNil":                   "S8",
	"SetRefNameNestedInit":                 "S8",
	"SetRefNameShadowedNil":                "S8",
	"SetRefNestedInit":                     "S8",
	"SetRefNestedReset":                    "V3b",
	"SetRefOnShadow":                       "S1",
	"SetRefOnTemp":                         "S3",
	"SetRefPayloadDefault":                 "V3b",
	"SetRefPayloadGuarded":                 "N1",
	"SetRefPayloadMake":                    "V1",
	"SetRefRangeValue":                     "S1",
	"SetRefSelect":                         "S1",
	"SetRefSelectComm":                     "S1",
	"SetRefTypeSwitch":                     "S1",
	"SetRefTypedNil":                       "V1",
	"SetRefUnlessSet":                      "N1",
	"SetRefViaAssignedNil":                 "S3",
	"SetRefViaInitNil":                     "S1",
	"SetRefViaNilLocal":                    "S1",
	"SetRefViaParenAssignedNil":            "S3",
	"SetReplicasAddCount":                  "S7",
	"SetReplicasAndDefault":                "V1",
	"SetReplicasAndDerived":                "V1",
	"SetReplicasAppendAliasConcat":         "S3",
	"SetReplicasAppendTempConcat":          "S3",
	"SetReplicasAppendTempField":           "S3",
	"SetReplicasArrayParamConcat":          "S7",
	"SetReplicasBlankAppend":               "P1",
	"SetReplicasByValue":                   "P1",
	"SetReplicasClearingRef":               "V1",
	"SetReplicasCommaOkConcat":             "S3",
	"SetReplicasCommaOkCrossField":         "S3",
	"SetReplicasCommaOkRange":              "S3",
	"SetReplicasCommaOkVarConcat":          "S1",
	"SetReplicasConvertedCrossField":       "S3",
	"SetReplicasConvertedInc":              "S3",
	"SetReplicasDecCount":                  "S1",
	"SetReplicasForPost":                   "S1",
	"SetReplicasFreshMapWriteBack":         "S3",
	"SetReplicasFreshWriteBack":            "S3",
	"SetReplicasHolderParamConcat":         "S7",
	"SetReplicasIncAliasNilInit":           "S3",
	"SetReplicasIncCount":                  "S1",
	"SetReplicasIncNilInit":                "S3",
	"SetReplicasIncOnly":                   "S1",
	"SetReplicasIncParenAliasNilInit":      "S3",
	"SetReplicasItemConcat":                "S3",
	"SetReplicasItemRange":                 "S3",
	"SetReplicasMapParam":                  "P1",
	"SetReplicasNilReturn":                 "S8",
	"SetReplicasOverwrittenWriteBack":      "S3",
	"SetReplicasParenAppendLocal":          "S1",
	"SetReplicasParenLabelConcat":          "S3",
	"SetReplicasParenMapTarget":            "S3",
	"SetReplicasRangeAliasConcat":          "S1",
	"SetReplicasRangeHolderConcat":         "S1",
	"SetReplicasRangeNestConcat":           "S1",
	"SetReplicasSliceExprConcat":           "S7",
	"SetReplicasSliceExprRange":            "S1",
	"SetReplicasSlicedAliasConcat":         "S3",
	"SetReplicasSlicedCrossField":          "S3",
	"SetReplicasSwitch":                    "S1",
	"SetReplicasTempExpandedConcat":        "S3",
	"SetReplicasTempMapConcat":             "S3",
	"SetReplicasTempMapInc":                "S3",
	"SetReplicasTempMapRange":              "S3",
	"SetReplicasViaClosure":                "S9",
	"SetReplicasZero":                      "V1",
	"SetSpecNestedNil":                     "V1",
	"SetSpecWithNilRef":                    "V1",
	"SetViaDelegate":                       "S1",
}

// grammarAdmitted lists the fixtures in grammarFixtureSource that stay
// admitted: the shapes the grammar must not refuse.
var grammarAdmitted = map[string]Class{
	"SetDocBlob":                       Pointer,
	"AddItemFromPointer":               Append,
	"AddDocPlan":                       Append,
	"AddHoldLabel":                     Append,
	"SetNestedRefPositional":           Composite,
	"SetDocPlanLimitConstIndex":        Pointer,
	"SetDocPlanLimitGuardedConstIndex": Pointer,
	"AddDocHeld":                       Append,
	"SetReplicasParenGuard":            Pointer,
	"AddLabelParenKey":                 Append,
	"SetDocNameAndRef":                 Pointer,
	"AddDocPlanFromPointer":            Append,
	"SetDocBlobAndSpare":               Pointer,
	"SetDocBlobGuardAfter":             Pointer,
	"AddHoldItemInit":                  Append,
	"SetDocShellSlot":                  Pointer,

	"AddDocKeyedFuncTypeKey":      Append,
	"SetRefNameParenGuardOperand": Pointer,
	"SetRefNameDerefObj":          Pointer,
	"SetDocAnyReadsName":          Composite,
	"AddDocRawByte":               Append,
	"SetDocRawWhole":              Pointer,
	"SetReplicasAndNilName":       Pointer,
	"SetRefNameFromPointer":       Pointer,

	"SetDocRefAndHoldMapLabels": Pointer,
	"SetDocRefThrough":          Composite,
	"SetRefWhole":               Pointer,
	"SetDocAnyRef":              Composite,

	"SetDocShellBoxNameTwoInits": Pointer,

	"SetRefNameParenInit":          Pointer,
	"SetDocPlanPositionalNilLimit": Composite,

	"SetDocAnyReadsAnyDerefObj": Composite,
}

// grammarForwarders lists the fixtures the grammar admits and the class of
// their writes refuses, with a fragment of the reason: one bare write, two,
// or none at all.
var grammarForwarders = map[string]string{
	"SetName":            "single bare field assignment",
	"SetNameAndATwoArgs": "2 bare field writes and no composite literal",
	"SetNothingButGuard": "no field write",
}

// fixtureVerdicts classifies the fixture and returns the verdict on each
// helper, by name.
func fixtureVerdicts(t *testing.T) map[string]Finding {
	t.Helper()
	findings, err := Classify(Options{
		Dir:      writeFixture(t),
		Patterns: []string{"."},
		Exempt:   map[string]bool{"fixture.SetExempted": true},
		Env:      append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod"),
	})
	if err != nil {
		t.Fatal(err)
	}
	verdicts := map[string]Finding{}
	for _, f := range findings {
		verdicts[f.Name] = f
	}
	return verdicts
}

// TestClassify_GrammarRules reads the verdict on every fixture grammarRules
// names: refused, by the rule the table names. For the fixtures
// grammarWording names it reads the words of the reason, and for those
// grammarForwarders names the forwarder reason the class of the writes gives.
func TestClassify_GrammarRules(t *testing.T) {
	verdicts := fixtureVerdicts(t)
	verdict := func(name string) (Finding, bool) {
		f, classified := verdicts[name]
		if !classified {
			t.Errorf("%s: not classified at all", name)
		}
		return f, classified
	}
	for name, rule := range grammarRules {
		f, classified := verdict(name)
		if classified && (f.Class != Inadmissible || !strings.Contains(f.Reason, "(grammar "+rule+",")) {
			t.Errorf("%s: %s (%s), want it refused by rule %s", name, f.Class, f.Reason, rule)
		}
	}
	for name, fragment := range grammarWording {
		f, classified := verdict(name)
		if classified && !strings.Contains(f.Reason, fragment) {
			t.Errorf("%s: %s (%s), want the reason to contain %q", name, f.Class, f.Reason, fragment)
		}
	}
	for name, fragment := range grammarForwarders {
		f, classified := verdict(name)
		if classified && (f.Class != Inadmissible || !strings.Contains(f.Reason, fragment)) {
			t.Errorf("%s: %s (%s), want it refused as a forwarder, %q", name, f.Class, f.Reason, fragment)
		}
	}
}

// grammarWording lists fixtures whose refusal names its cause, with a
// fragment of the reason: a return or a goto by name at the top level (S1)
// and inside an if (S8), and each of the four reasons a path is not the
// object's (P1).
var grammarWording = map[string]string{
	"SetReplicasThenReturn":       "a return is neither an if nor an assignment (grammar S1,",
	"SetReplicasGotoAhead":        "a goto is neither an if nor an assignment (grammar S1,",
	"SetReplicasNilReturn":        "if o == nil returns early instead of writing; a nil receiver panics and sugar has no conditional no-op (grammar S8,",
	"SetRefGoto":                  `if n == "" jumps over the write with goto; sugar has no conditional no-op (grammar S8,`,
	"SetNothing":                  "_ is not spelled from a parameter through selectors, indexes and dereferences (grammar P1,",
	"SetReplicasTwoObjects":       "b.Spec.Name is spelled from b, not from a, the object of the first write (grammar P1,",
	"SetReplicasByValue":          "o.Spec.Replicas is spelled from o, which is not a pointer (grammar P1,",
	"SetRefDefaulted":             "ref is spelled from ref, which the body reassigns or takes the address of (grammar P1,",
	"AddItemParenParamReassigned": "(o) is spelled from o, which the body reassigns or takes the address of (grammar P1,",
}

// TestSameIndex pins what makes two keys or indexes the same one: the same
// expression, or two constants of one type and one value however each is
// spelled. The fixture cannot isolate the type: two constants of two types
// never print alike, so a comparison by text would tell them apart too.
func TestSameIndex(t *testing.T) {
	const src = `package p

func f() int { return 0 }

func g(m map[any]int, k string) {
	_ = m[int32(0)]
	_ = m[int32(1-1)]
	_ = m[int64(0)]
	_ = m[int32(1)]
	_ = m[f()]
	_ = m[f()]
	_ = m[k]
	_ = m[(k)]
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Types: map[ast.Expr]types.TypeAndValue{},
		Defs:  map[*ast.Ident]types.Object{},
		Uses:  map[*ast.Ident]types.Object{},
	}
	if _, err := (&types.Config{}).Check("p", fset, []*ast.File{file}, info); err != nil {
		t.Fatal(err)
	}
	var keys []ast.Expr
	ast.Inspect(file, func(n ast.Node) bool {
		if index, ok := n.(*ast.IndexExpr); ok {
			keys = append(keys, index.Index)
		}
		return true
	})
	if len(keys) != 8 {
		t.Fatalf("read %d keys, want 8", len(keys))
	}

	b := &helperBody{info: info}
	for _, tc := range []struct {
		name string
		x, y int
		want bool
	}{
		{"one constant spelled two ways", 0, 1, true},
		{"one value of two types", 0, 2, false},
		{"two values of one type", 0, 3, false},
		{"two calls that print alike", 4, 5, false},
		{"one parameter, parenthesised or not", 6, 7, true},
		{"a constant and a parameter", 0, 6, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x, y := keys[tc.x], keys[tc.y]
			if got := b.sameIndex(x, y); got != tc.want {
				t.Errorf("sameIndex(%s, %s) = %t, want %t", types.ExprString(x), types.ExprString(y), got, tc.want)
			}
			if got := b.sameIndex(y, x); got != tc.want {
				t.Errorf("sameIndex(%s, %s) = %t, want %t", types.ExprString(y), types.ExprString(x), got, tc.want)
			}
		})
	}
}
