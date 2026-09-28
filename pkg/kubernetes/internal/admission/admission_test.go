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
	Rows     [][]string
	Name     string
	Ref      *Ref
	Nested   Inner
	Payload  any
	Box      *Box
	Count    int
	Groups   map[string][]string
	Holders  []Holder
	Nests    []Nest
	Hold     *Holder
}

type Holder struct {
	Items  []string
	Labels map[string]string
}

type Nest struct{ H Holder }

type Items []string

type Box struct {
	Payload any
	Ch      chan string
	Name    string
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

// inadmissible: class a inserts into the field itself, not into a local map
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

// inadmissible: a make with a non-zero length fills the slice with an empty
// element the caller never supplied
func AddItemMakeLen(o *Obj, s string) {
	o.Spec.Items = make([]string, 1)
	o.Spec.Items = append(o.Spec.Items, s)
}

// inadmissible: a guard around a make that fills the slice is not the
// nil-init guard, so the write under it is conditional
func AddItemGuardedMakeLen(o *Obj, s string) {
	if o.Spec.Items == nil {
		o.Spec.Items = make([]string, 1)
	}
	o.Spec.Items = append(o.Spec.Items, s)
}

// inadmissible: a length that is not a constant 0 may allocate elements, so
// the guard around it is not the nil-init guard either
func AddItemMakeParamLen(o *Obj, s string, n int) {
	if o.Spec.Items == nil {
		o.Spec.Items = make([]string, n)
	}
	o.Spec.Items = append(o.Spec.Items, s)
}

// class a, make-init of the slice at length 0 then append
func AddItemMakeZero(o *Obj, s string) {
	if o.Spec.Items == nil {
		o.Spec.Items = make([]string, 0)
	}
	o.Spec.Items = append(o.Spec.Items, s)
}

// class a, a capacity allocates no elements
func AddItemMakeCap(o *Obj, s string) {
	if o.Spec.Items == nil {
		o.Spec.Items = make([]string, 0, 4)
	}
	o.Spec.Items = append(o.Spec.Items, s)
}

// class a, a map size hint allocates no entries
func AddLabelMakeHint(o *Obj, k, v string) {
	if o.Labels == nil {
		o.Labels = make(map[string]string, 1)
	}
	o.Labels[k] = v
}

// inadmissible: the appended row is a non-empty make, whatever its capacity mentions
func AddRowMakeLen(o *Obj, n int) { o.Spec.Rows = append(o.Spec.Rows, make([]string, 1, n)) }

// inadmissible: an empty literal is the nil-init only of a map, slice or
// pointer field; written into an interface field it is a default
func SetRefPayloadDefault(o *Obj, r *Ref) {
	o.Spec.Payload = &Ref{}
	o.Spec.Ref = r
}

// inadmissible: guarding an interface field for nil and filling it is a
// set-if-unset, not the nil-init guard
func SetRefPayloadGuarded(o *Obj, r *Ref) {
	if o.Spec.Payload == nil {
		o.Spec.Payload = &Ref{}
	}
	o.Spec.Ref = r
}

// inadmissible: an empty literal written into a struct field wipes it
func SetRefNestedReset(o *Obj, r *Ref) {
	o.Spec.Nested = Inner{}
	o.Spec.Ref = r
}

// inadmissible: an empty make into an interface field initialises no
// collection the body inserts into; it is a default like an empty literal
func SetRefPayloadMake(o *Obj, r *Ref) {
	o.Spec.Payload = make(map[string]string)
	o.Spec.Ref = r
}

// class b, the control for the three below: a write of a caller value through
// a pointer the body initialised
func SetBoxName(o *Obj, name string) {
	if o.Spec.Box == nil {
		o.Spec.Box = &Box{}
	}
	o.Spec.Box.Name = name
}

// inadmissible: an empty make written through a pointer the body initialised
// is still an empty value into an interface field, a default
func SetBoxPayloadMake(o *Obj, name string) {
	if o.Spec.Box == nil {
		o.Spec.Box = &Box{}
	}
	o.Spec.Box.Payload = make(map[string]string)
	o.Spec.Box.Name = name
}

// inadmissible: new(T) through a pointer the body initialised, into an
// interface field
func SetBoxPayloadNew(o *Obj, name string) {
	if o.Spec.Box == nil {
		o.Spec.Box = &Box{}
	}
	o.Spec.Box.Payload = new(Ref)
	o.Spec.Box.Name = name
}

// inadmissible: an empty channel make through a pointer the body initialised;
// a channel is not a field the zero-value init initialises
func SetBoxChanMake(o *Obj, name string) {
	if o.Spec.Box == nil {
		o.Spec.Box = &Box{}
	}
	o.Spec.Box.Ch = make(chan string)
	o.Spec.Box.Name = name
}

// class a, an empty slice literal is the nil-init of a slice field
func AddItemEmptyLit(o *Obj, s string) {
	if o.Spec.Items == nil {
		o.Spec.Items = []string{}
	}
	o.Spec.Items = append(o.Spec.Items, s)
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

// inadmissible: class a appends to the field itself, not to a local slice
func AddItemViaLocal(o *Obj, s string) {
	items := o.Spec.Items
	items = append(items, s)
	o.Spec.Items = items
}

// inadmissible: the same through a parenthesised local map index
func AddLabelParenLocal(o *Obj, k, v string) {
	labels := o.Labels
	(labels)[k] = v
	o.Labels = labels
}

// inadmissible: an insert into the helper's own map, whose write-back replaces
// the caller's labels instead of adding to them
func AddLabelFreshMap(o *Obj, k, v string) {
	labels := map[string]string{}
	labels[k] = v
	o.Labels = labels
}

// inadmissible: a function literal's body is not checked, so a helper holding
// one is refused whether or not it calls it
func AddItemInClosure(o *Obj, s string) {
	f := func() { o.Spec.Items = append(o.Spec.Items, s) }
	_ = f
}

// inadmissible: class a extends a field path of the parameter itself, not of
// an alias
func AddItemViaAlias(o *Obj, s string) {
	spec := &o.Spec
	spec.Items = append(spec.Items, s)
}

// class b through a declared alias of the object
func SetReplicasViaAlias(o *Obj, n int32) {
	var obj = o
	obj.Spec.Replicas = &n
}

// inadmissible: an append into a local, here after its write-back
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

// inadmissible: an append into a local, here not the local written back
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

// inadmissible: an insert into a local map, beside a forwarder
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

// inadmissible: the optional-argument idiom as a guard around the write (§4)
func SetRefIfSet(o *Obj, ref *Ref) {
	if ref != nil {
		o.Spec.Ref = ref
	}
}

// inadmissible: a guarded composite is still conditional
func SetNestedRefIfSet(o *Obj, n string) {
	if n != "" {
		o.Spec.Nested.Ref = Ref{Name: n, Kind: n}
	}
}

// inadmissible: a guarded append is still conditional
func AddItemIfSet(o *Obj, s string) {
	if s != "" {
		o.Spec.Items = append(o.Spec.Items, s)
	}
}

// inadmissible: an insert into a local, here also only on some paths
func AddLabelIfSetViaLocal(o *Obj, k, v string) {
	labels := o.Labels
	if v != "" {
		labels[k] = v
	}
	o.Labels = labels
}

// inadmissible: a set-if-unset drops the caller's value when the field is set
func SetRefUnlessSet(o *Obj, ref *Ref) {
	if o.Spec.Ref == nil {
		o.Spec.Ref = ref
	}
}

// inadmissible: a nil-init guard with an else is not the admitted shape
func SetRefNameElse(o *Obj, n string) {
	if o.Spec.Ref == nil {
		o.Spec.Ref = &Ref{}
	} else {
		_ = n
	}
	o.Spec.Ref.Name = n
}

// inadmissible: a nil-init nested under an optional-value guard
func SetRefNameNestedInit(o *Obj, n string) {
	if n != "" {
		if o.Spec.Ref == nil {
			o.Spec.Ref = &Ref{}
		}
	}
	o.Spec.Ref.Name = n
}

// inadmissible: the guard tests one path and initialises another
func SetRefGuardedByOther(o *Obj, n string) {
	if o.Spec.Replicas == nil {
		o.Spec.Ref = &Ref{}
	}
	o.Spec.Ref.Name = n
}

// inadmissible: a nil-init guard with an init statement is not the admitted shape
func SetRefNameIfInit(o *Obj, n string) {
	if _ = n; o.Spec.Ref == nil {
		o.Spec.Ref = &Ref{}
	}
	o.Spec.Ref.Name = n
}

// inadmissible: a write under a switch case
func SetReplicasSwitch(o *Obj, n int32) {
	switch {
	case n > 0:
		o.Spec.Replicas = &n
	}
}

// inadmissible: a write inside a range loop
func AddItemsLoop(o *Obj, items []string) {
	for i := range items {
		o.Spec.Items = append(o.Spec.Items, items[i])
	}
}

// inadmissible: goto jumps over the write
func SetRefGoto(o *Obj, n string) {
	if n == "" {
		goto done
	}
	o.Spec.Ref = &Ref{Name: n}
done:
}

// class b: a nil-init guard with nil on the left is the admitted shape
func SetRefNameNilFirst(o *Obj, n string) {
	if nil == o.Spec.Ref {
		o.Spec.Ref = &Ref{}
	}
	o.Spec.Ref.Name = n
}

// inadmissible: new of a value is a set-if-unset, not a zero-init
func SetRefIfUnsetNewValue(o *Obj, ref Ref, n string) {
	if o.Spec.Ref == nil {
		o.Spec.Ref = new(ref)
	}
	o.Spec.Ref.Name = n
}

// inadmissible: a nil-init guard with a second statement is not the admitted shape
func SetRefNameGuardExtra(o *Obj, n string) {
	if o.Spec.Ref == nil {
		o.Spec.Ref = &Ref{}
		_ = n
	}
	o.Spec.Ref.Name = n
}

// inadmissible: a write inside a three-clause for loop
func AddItemsForLoop(o *Obj, items []string) {
	for i := 0; i < len(items); i++ {
		o.Spec.Items = append(o.Spec.Items, items[i])
	}
}

// inadmissible: a write under a type switch case
func SetRefTypeSwitch(o *Obj, v any) {
	switch v.(type) {
	case *Ref:
		o.Spec.Ref = v.(*Ref)
	}
}

// inadmissible: a write under a select case
func SetRefSelect(o *Obj, ch chan *Ref) {
	select {
	case r := <-ch:
		o.Spec.Ref = r
	}
}

// inadmissible: a same-path guard that is not a nil test
func SetRefNameIfNotNil(o *Obj, n string) {
	if o.Spec.Ref != nil {
		o.Spec.Ref = &Ref{}
	}
	o.Spec.Ref.Name = n
}

// inadmissible: the write is a for loop's post statement
func SetReplicasForPost(o *Obj, r int32) {
	for i := 0; i < 1; o.Spec.Replicas = &r {
		i++
	}
}

// inadmissible: a nil-init guard assigning two paths is not the admitted shape
func SetRefAndReplicasInGuard(o *Obj, n string, r int32) {
	if o.Spec.Ref == nil {
		o.Spec.Ref, o.Spec.Replicas = &Ref{}, &r
	}
	o.Spec.Ref.Name = n
}

// inadmissible: the write is only in the else branch
func SetRefElse(o *Obj, ref *Ref, skip bool) {
	if skip {
	} else {
		o.Spec.Ref = ref
	}
}

// inadmissible: the write is a select case's communication
func SetRefSelectComm(o *Obj, ch chan *Ref) {
	select {
	case o.Spec.Ref = <-ch:
	default:
	}
}

// inadmissible: the write is the init of an if nested under a guard
func SetRefNestedInit(o *Obj, ref *Ref, ok bool) {
	if ok {
		if o.Spec.Ref = ref; false {
		}
	}
}

// inadmissible: a guard inside a top-level bare block
func SetRefBlockIf(o *Obj, ref *Ref) {
	{
		if ref != nil {
			o.Spec.Ref = ref
		}
	}
}

// inadmissible: a write inside a labeled loop
func SetRefLabeledLoop(o *Obj, ref *Ref) {
L:
	for {
		o.Spec.Ref = ref
		break L
	}
}

// class b: a top-level bare block runs on every path
func SetRefBlock(o *Obj, ref *Ref) {
	{
		o.Spec.Ref = ref
	}
}

// inadmissible: the guard compares against a parameter named nil, not the
// predeclared nil, so the write runs only when the field equals a caller value
func SetRefNameShadowedNil(o *Obj, nil *Ref, n string) {
	if o.Spec.Ref == nil {
		o.Spec.Ref = &Ref{}
	}
	o.Spec.Ref.Name = n
}

// inadmissible: a range clause assigns the field once per element, or not at all
func SetRefRangeValue(o *Obj, refs []*Ref, n int32) {
	for _, o.Spec.Ref = range refs {
	}
	o.Spec.Replicas = &n
}

// inadmissible: a range clause's key is written the same way as its value
func SetCountRangeKey(o *Obj, xs []string, n int32) {
	for o.Spec.Count = range xs {
	}
	o.Spec.Replicas = &n
}

// inadmissible: an increment writes a value computed from what the field held
func SetReplicasIncCount(o *Obj, n int32) {
	o.Spec.Count++
	o.Spec.Replicas = &n
}

// inadmissible: incrementing through the pointer just assigned changes the caller's value
func SetReplicasIncOnly(o *Obj, n int32) {
	o.Spec.Replicas = &n
	*o.Spec.Replicas++
}

// inadmissible: a decrement is the same read-modify-write
func SetReplicasDecCount(o *Obj, n int32) {
	o.Spec.Count--
	o.Spec.Replicas = &n
}

// inadmissible: a compound assignment writes old + c, not the caller's c
func SetReplicasAddCount(o *Obj, n int32, c int) {
	o.Spec.Replicas = &n
	o.Spec.Count += c
}

// inadmissible: a compound assignment through a rooted local map writes old + v
func AddLabelConcatViaLocal(o *Obj, k, v string) {
	labels := o.Labels
	labels[k] += v
	o.Labels = labels
}

// inadmissible: a compound assignment to an element of a rooted local slice writes old + s
func SetReplicasItemConcat(o *Obj, s string, n int32) {
	items := o.Spec.Items
	items[0] += s
	o.Spec.Replicas = &n
}

// inadmissible: a range clause assigns an element of a rooted local slice once per element
func SetReplicasItemRange(o *Obj, xs []string, n int32) {
	items := o.Spec.Items
	for _, items[0] = range xs {
	}
	o.Spec.Replicas = &n
}

// inadmissible: parentheses around the target change nothing
func SetReplicasParenLabelConcat(o *Obj, k, v string, n int32) {
	labels := o.Labels
	(labels[k]) += v
	o.Spec.Replicas = &n
}

// inadmissible: the nil-init guard leaves labels the caller's map when it was not nil
func AddLabelConcatNilInit(o *Obj, k, v string) {
	labels := o.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	labels[k] += v
	o.Labels = labels
}

// inadmissible: the nil-init guard leaves p the caller's pointer when it was not nil
func SetReplicasIncNilInit(o *Obj, n int32) {
	p := o.Spec.Replicas
	if p == nil {
		p = new(int32)
	}
	*p++
	o.Spec.Replicas = &n
}

// class b: incrementing a temporary reaches no caller
func SetReplicasTempCount(o *Obj, n int32) {
	tmp := Obj{}
	tmp.Spec.Count++
	_ = tmp
	o.Spec.Replicas = &n
}

// inadmissible: slicing the caller's slice shares its backing array
func SetReplicasSliceExprConcat(o *Obj, s string, n int32) {
	o.Spec.Items[:][0] += s
	o.Spec.Replicas = &n
}

// inadmissible: a range clause assigns an element of the caller's resliced slice
func SetReplicasSliceExprRange(o *Obj, xs []string, n int32) {
	for _, o.Spec.Items[:][0] = range xs {
	}
	o.Spec.Replicas = &n
}

// inadmissible: q copies p, which is still the caller's pointer when it was not nil
func SetReplicasIncAliasNilInit(o *Obj, n int32) {
	p := o.Spec.Replicas
	if p == nil {
		p = new(int32)
	}
	q := p
	(*q)++
	o.Spec.Replicas = &n
}

// inadmissible: a local sliced from the caller's slice shares its backing array
func SetReplicasSlicedAliasConcat(o *Obj, s string, n int32) {
	items := o.Spec.Items[:]
	items[0] += s
	o.Spec.Replicas = &n
}

// inadmissible: parentheses around the alias's destination change nothing
func SetReplicasIncParenAliasNilInit(o *Obj, n int32) {
	p := o.Spec.Replicas
	if p == nil {
		p = new(int32)
	}
	q := new(int32)
	(q) = p
	(*q)++
	o.Spec.Replicas = &n
}

// inadmissible: a comma-ok map read copies the caller's slice header
func SetReplicasCommaOkConcat(o *Obj, k, s string, n int32) {
	items, _ := o.Spec.Groups[k]
	items[0] += s
	o.Spec.Replicas = &n
}

// inadmissible: the same read in a var declaration
func SetReplicasCommaOkVarConcat(o *Obj, k, s string, n int32) {
	var items, _ = o.Spec.Groups[k]
	items[0] += s
	o.Spec.Replicas = &n
}

// inadmissible: a range clause writing through a comma-ok alias
func SetReplicasCommaOkRange(o *Obj, k string, xs []string, n int32) {
	items, ok := o.Spec.Groups[k]
	_ = ok
	for _, items[0] = range xs {
	}
	o.Spec.Replicas = &n
}

// inadmissible: an append into a local, here read comma-ok from another field
func AddItemCommaOkCrossField(o *Obj, k, s string) {
	items, _ := o.Spec.Groups[k]
	items = append(items, s)
	o.Spec.Items = items
}

// inadmissible: an expanded read-modify-write is the same default as +=
func AddLabelExpandedConcat(o *Obj, k, v string) {
	if o.Labels == nil {
		o.Labels = map[string]string{}
	}
	o.Labels[k] = o.Labels[k] + v
}

// inadmissible: the same through a local alias of the caller's map
func AddLabelExpandedViaLocal(o *Obj, k, v string) {
	labels := o.Labels
	labels[k] = labels[k] + v
	o.Labels = labels
}

// inadmissible: parentheses around the target change nothing
func AddLabelExpandedParen(o *Obj, k, v string) {
	if o.Labels == nil {
		o.Labels = map[string]string{}
	}
	(o.Labels[k]) = o.Labels[k] + v
}

// inadmissible: an insert into a temporary map reaches no caller, but is not
// class a's statement either
func SetReplicasTempExpandedConcat(o *Obj, k, v string, n int32) {
	m := map[string]string{}
	m[k] = m[k] + v
	_ = m
	o.Spec.Replicas = &n
}

// inadmissible: the target read through the caller's path, written through an alias
func AddLabelExpandedCrossAlias(o *Obj, k, v string) {
	labels := o.Labels
	labels[k] = o.Labels[k] + v
	o.Labels = labels
}

// inadmissible: the target read through an alias, written through the caller's path
func AddLabelExpandedFromAlias(o *Obj, k, v string) {
	if o.Labels == nil {
		o.Labels = map[string]string{}
	}
	labels := o.Labels
	o.Labels[k] = labels[k] + v
}

// class a: a local that never reached the caller is not the target
func AddLabelFromTemp(o *Obj, k, v string) {
	m := map[string]string{}
	if o.Labels == nil {
		o.Labels = map[string]string{}
	}
	o.Labels[k] = m[k] + v
}

// inadmissible: an append result shares the caller's backing array
func SetReplicasAppendAliasConcat(o *Obj, s string, n int32) {
	items := append(o.Spec.Items, s)
	items[0] += s
	o.Spec.Replicas = &n
}

// inadmissible: the same for an append to a temporary
func SetReplicasAppendTempConcat(o *Obj, s string, n int32) {
	items := append([]string{}, s)
	items[0] += s
	o.Spec.Replicas = &n
}

// inadmissible: a range variable holds each of the caller's slices in turn
func SetReplicasRangeAliasConcat(o *Obj, v string, n int32) {
	for _, items := range o.Spec.Groups {
		items[0] += v
	}
	o.Spec.Replicas = &n
}

// class a: another parameter is the caller's own argument, not the target
func AddLabelFromOther(o, src *Obj, k string) {
	if o.Labels == nil {
		o.Labels = map[string]string{}
	}
	o.Labels[k] = src.Labels[k] + "x"
}

// class b: a range over a temporary reaches no caller
func SetReplicasRangeTempConcat(o *Obj, v string, n int32) {
	for _, items := range map[string][]string{} {
		items[0] += v
	}
	o.Spec.Replicas = &n
}

// class b: a range variable holding a struct is a copy
func SetReplicasRangeCopyConcat(o *Obj, refs []Ref, n int32) {
	for _, r := range refs {
		r.Name += "x"
		_ = r
	}
	o.Spec.Replicas = &n
}

// class a: the slice an append extends is not a read of the target
func AddGroupItem(o *Obj, k, s string) {
	if o.Spec.Groups == nil {
		o.Spec.Groups = map[string][]string{}
	}
	o.Spec.Groups[k] = append(o.Spec.Groups[k], s)
}

// class b: a comma-ok read of a temporary map reaches no caller
func SetReplicasCommaOkTemp(o *Obj, k, s string, n int32) {
	m := map[string][]string{}
	items, _ := m[k]
	items[0] += s
	o.Spec.Replicas = &n
}

// clonePtr is not a sugar helper: a call the classifier does not follow.
func clonePtr(p *int32) *int32 { c := *p; return &c }

// inadmissible: an append into a local, here read from another field
func AddItemCrossField(o *Obj, k, s string) {
	items := o.Spec.Groups[k]
	items = append(items, s)
	o.Spec.Items = items
}

// inadmissible: the same from an element of another slice field
func AddItemRowCrossField(o *Obj, s string) {
	items := o.Spec.Rows[0]
	items = append(items, s)
	o.Spec.Items = items
}

// inadmissible: an append into a slice parameter, which replaces the field
func AddItemFromParam(o *Obj, items []string, s string) {
	items = append(items, s)
	o.Spec.Items = items
}

// inadmissible: an append into a local, one of whose sources is another field
func AddItemReassignedCrossField(o *Obj, s string) {
	items := o.Spec.Rows[0]
	items = o.Spec.Items
	items = append(items, s)
	o.Spec.Items = items
}

// inadmissible: an append into a local, written back to two fields
func AddItemTwoWriteBacks(o *Obj, s string) {
	items := o.Spec.Items
	items = append(items, s)
	o.Spec.Rows[0] = items
	o.Spec.Items = items
}

// inadmissible: the same with the matching source first
func AddItemReassignedCrossFieldLast(o *Obj, s string) {
	items := o.Spec.Items
	items = o.Spec.Rows[0]
	items = append(items, s)
	o.Spec.Items = items
}

// inadmissible: the same with the matching write-back first
func AddItemTwoWriteBacksLast(o *Obj, s string) {
	items := o.Spec.Items
	items = append(items, s)
	o.Spec.Items = items
	o.Spec.Rows[0] = items
}

// inadmissible: the same with a parenthesised reassignment
func AddItemParenReassignedCrossField(o *Obj, s string) {
	items := o.Spec.Items
	(items) = o.Spec.Rows[0]
	items = append(items, s)
	o.Spec.Items = items
}

// inadmissible: the same with one of several names assigned together
func AddItemParenMultiReassigned(o *Obj, s string) {
	items := o.Spec.Items
	(items), _ = o.Spec.Rows[0], 0
	items = append(items, s)
	o.Spec.Items = items
}

// inadmissible: a declared local from another field, parenthesised
func AddItemVarParenCrossField(o *Obj, s string) {
	var items = (o.Spec.Rows[0])
	items = append(items, s)
	o.Spec.Items = items
}

// inadmissible: an append into a local, extending another field
func AddItemAppendCrossField(o *Obj, s string) {
	items := o.Spec.Items
	items = append(o.Spec.Rows[0], s)
	o.Spec.Items = items
}

// inadmissible: an append into a local, even one extending the field it is
// written back to
func AddItemAppendWrittenBackField(o *Obj, s string) {
	items := o.Spec.Items
	items = append(o.Spec.Items, s)
	o.Spec.Items = items
}

// inadmissible: a nested append into a local
func AddItemsNestedAppend(o *Obj, a, b string) {
	items := o.Spec.Items
	items = append(append(items, a), b)
	o.Spec.Items = items
}

// inadmissible: an append into a local, extending a collection the helper built
func AddItemAppendFresh(o *Obj, s string) {
	items := o.Spec.Items
	items = append([]string{}, s)
	o.Spec.Items = items
}

// inadmissible: an append into a local reassigned a slice of another field
func AddItemSliceReassignedCrossField(o *Obj, s string) {
	items := o.Spec.Items
	items = o.Spec.Rows[0][:]
	items = append(items, s)
	o.Spec.Items = items
}

// inadmissible: the same with a conversion of another field
func AddItemConvertedReassignedCrossField(o *Obj, s string) {
	items := o.Spec.Items
	items = []string(o.Spec.Rows[0])
	items = append(items, s)
	o.Spec.Items = items
}

// inadmissible: the same with a local holding a slice of another field
func AddItemCopiedSliceCrossField(o *Obj, s string) {
	items := o.Spec.Items
	rows := o.Spec.Rows[0][:]
	items = rows
	items = append(items, s)
	o.Spec.Items = items
}

// inadmissible: the same with a comma-ok read of another field
func AddItemCommaOkReassignedCrossField(o *Obj, s string) {
	items := o.Spec.Items
	items, _ = o.Spec.Groups["a"]
	items = append(items, s)
	o.Spec.Items = items
}

// inadmissible: the same with a range clause over another field
func AddItemRangeReassignedCrossField(o *Obj, s string) {
	items := o.Spec.Items
	for _, items = range o.Spec.Rows {
	}
	items = append(items, s)
	o.Spec.Items = items
}

// inadmissible: the same with a multi-assignment, which reads every value
// before assigning any name
func AddItemMultiAssignedCrossField(o *Obj, s string) {
	items := o.Spec.Items
	rows := o.Spec.Rows[0][:]
	items, rows = rows, o.Spec.Items
	items = append(items, s)
	o.Spec.Items = items
	_ = rows
}

// inadmissible: the same with the local's name last
func AddItemMultiAssignedCrossFieldLast(o *Obj, s string) {
	items := o.Spec.Items
	rows := o.Spec.Rows[0][:]
	rows, items = o.Spec.Items, rows
	items = append(items, s)
	o.Spec.Items = items
	_ = rows
}

// inadmissible: the same with two locals read from the field and swapped
func AddItemSwappedSameField(o *Obj, s string) {
	items := o.Spec.Items
	other := o.Spec.Items
	items, other = other, items
	items = append(items, s)
	o.Spec.Items = items
	_ = other
}

// inadmissible: the swap leaves tmp holding the caller's spec
func AddItemSwapHidesWrite(o *Obj, s string) {
	o.Spec.Items = append(o.Spec.Items, s)
	spec := &o.Spec
	tmp := &Spec{}
	spec, tmp = tmp, spec
	tmp.Name = "x"
	_ = spec
}

// inadmissible: an append into a local is refused beside a pointer write too
// (comma-ok read of another field)
func SetReplicasCommaOkCrossField(o *Obj, k, s string, n int32) {
	items, _ := o.Spec.Groups[k]
	items = append(items, s)
	o.Spec.Items = items
	o.Spec.Replicas = &n
}

// inadmissible: the same with a conversion of another field
func SetReplicasConvertedCrossField(o *Obj, s string, n int32) {
	items := []string(o.Spec.Rows[0])
	items = append(items, s)
	o.Spec.Items = items
	o.Spec.Replicas = &n
}

// inadmissible: the same with a slice of another field
func SetReplicasSlicedCrossField(o *Obj, s string, n int32) {
	items := o.Spec.Rows[0][:]
	items = append(items, s)
	o.Spec.Items = items
	o.Spec.Replicas = &n
}

// inadmissible: the same with a collection the helper built
func SetReplicasFreshWriteBack(o *Obj, s string, n int32) {
	items := []string{}
	items = append(items, s)
	o.Spec.Items = items
	o.Spec.Replicas = &n
}

// inadmissible: the same with an insert into a map the helper built
func SetReplicasFreshMapWriteBack(o *Obj, k, v string, n int32) {
	labels := map[string]string{}
	labels[k] = v
	o.Labels = labels
	o.Spec.Replicas = &n
}

// inadmissible: the same with two locals written back to one field
func SetReplicasOverwrittenWriteBack(o *Obj, s string, n int32) {
	items := []string{}
	items = append(items, s)
	o.Spec.Items = items
	next := o.Spec.Items
	next = append(next, s)
	o.Spec.Items = next
	o.Spec.Replicas = &n
}

// inadmissible: the same written back through a pointer the body initialised
func SetHoldCommaOkCrossField(o *Obj, k, s string) {
	if o.Spec.Hold == nil {
		o.Spec.Hold = &Holder{}
	}
	items, _ := o.Spec.Groups[k]
	items = append(items, s)
	o.Spec.Hold.Items = items
}

// inadmissible: the same with a map the helper built
func SetHoldFreshMapWriteBack(o *Obj, k, v string) {
	if o.Spec.Hold == nil {
		o.Spec.Hold = &Holder{}
	}
	labels := map[string]string{}
	labels[k] = v
	o.Spec.Hold.Labels = labels
}

// class b: a computed value written through a pointer the body initialised
func SetRefNameComputed(o *Obj, n string) {
	if o.Spec.Ref == nil {
		o.Spec.Ref = &Ref{}
	}
	o.Spec.Ref.Name = n + n
}

// inadmissible: an append into a local read through a pointer the body
// initialised, and written back to the same field
func AddHoldItemViaLocal(o *Obj, s string) {
	if o.Spec.Hold == nil {
		o.Spec.Hold = &Holder{}
	}
	items := o.Spec.Hold.Items
	items = append(items, s)
	o.Spec.Hold.Items = items
}

// inadmissible: an append into a local, beside a pointer write
func AddItemViaLocalAndReplicas(o *Obj, s string, n int32) {
	items := o.Spec.Items
	items = append(items, s)
	o.Spec.Items = items
	o.Spec.Replicas = &n
}

// inadmissible: an append into a local read from a map element, written back
// to that element
func AddGroupItemViaLocal(o *Obj, k, s string) {
	if o.Spec.Groups == nil {
		o.Spec.Groups = map[string][]string{}
	}
	items := o.Spec.Groups[k]
	items = append(items, s)
	o.Spec.Groups[k] = items
}

// inadmissible: an append into a declared local, written back to the field it
// came from
func AddItemViaVarLocal(o *Obj, s string) {
	var items = o.Spec.Items
	items = append(items, s)
	o.Spec.Items = items
}

// inadmissible: a conversion of the caller's pointer is still the caller's pointer
func SetReplicasConvertedInc(o *Obj, n int32) {
	p := (*int32)(o.Spec.Replicas)
	(*p)++
	o.Spec.Replicas = &n
}

// class b: a call other than a conversion is not followed
func SetReplicasClonedInc(o *Obj, n int32) {
	p := clonePtr(o.Spec.Replicas)
	(*p)++
	o.Spec.Replicas = &n
}

// inadmissible: a parenthesised index reads the same element
func AddLabelExpandedParenIndex(o *Obj, k, v string) {
	if o.Labels == nil {
		o.Labels = map[string]string{}
	}
	o.Labels[k] = o.Labels[(k)] + v
}

// class a: another key is not the target
func AddLabelFromOtherKey(o *Obj, k, j, v string) {
	if o.Labels == nil {
		o.Labels = map[string]string{}
	}
	o.Labels[k] = o.Labels[(j)] + v
}

// inadmissible: a struct copy's slice field shares the caller's backing array
func SetReplicasRangeHolderConcat(o *Obj, v string, n int32) {
	for _, h := range o.Spec.Holders {
		h.Items[0] += v
	}
	o.Spec.Replicas = &n
}

// inadmissible: the same, one struct deeper
func SetReplicasRangeNestConcat(o *Obj, v string, n int32) {
	for _, x := range o.Spec.Nests {
		x.H.Items[0] += v
	}
	o.Spec.Replicas = &n
}

// inadmissible: a by-value struct parameter's slice field is the caller's
func SetReplicasHolderParamConcat(o *Obj, h Holder, v string, n int32) {
	h.Items[0] += v
	o.Spec.Replicas = &n
}

// inadmissible: so is a by-value array parameter's slice element
func SetReplicasArrayParamConcat(o *Obj, a [1][]string, v string, n int32) {
	a[0][0] += v
	o.Spec.Replicas = &n
}

// inadmissible: an append into a local, written back under another key
func AddGroupItemCrossKey(o *Obj, k, j, s string) {
	if o.Spec.Groups == nil {
		o.Spec.Groups = map[string][]string{}
	}
	items := o.Spec.Groups[k]
	items = append(items, s)
	o.Spec.Groups[j] = items
}

// inadmissible: the same with two keys that print alike
func AddGroupItemLiteralKeys(o *Obj, s string) {
	if o.Spec.Groups == nil {
		o.Spec.Groups = map[string][]string{}
	}
	items := o.Spec.Groups[string([]byte{'a'})]
	items = append(items, s)
	o.Spec.Groups[string([]byte{'b'})] = items
}

// inadmissible: the same with one constant key
func AddGroupItemConstKey(o *Obj, s string) {
	if o.Spec.Groups == nil {
		o.Spec.Groups = map[string][]string{}
	}
	items := o.Spec.Groups["a"]
	items = append(items, s)
	o.Spec.Groups["a"] = items
}

// inadmissible: the append extends another field, with no local between
func AddItemFromRowDirect(o *Obj, s string) { o.Spec.Items = append(o.Spec.Rows[0], s) }

// inadmissible: the append extends a local read from another field
func AddItemLocalBase(o *Obj, s string) {
	items := o.Spec.Rows[0]
	o.Spec.Items = append(items, s)
}

// inadmissible: the append extends a local, even one read from the same field
func AddItemSameFieldLocalBase(o *Obj, s string) {
	items := o.Spec.Items
	o.Spec.Items = append(items, s)
}

// inadmissible: an append into a local read comma-ok, written back under
// another key
func AddGroupItemCommaOkCrossKey(o *Obj, k, j, s string) {
	if o.Spec.Groups == nil {
		o.Spec.Groups = map[string][]string{}
	}
	items, _ := o.Spec.Groups[k]
	items = append(items, s)
	o.Spec.Groups[j] = items
}

// inadmissible: the conversion written back through a pointer the body
// initialised reads a local that may hold the target, a value the caller did
// not supply (refused before the append into the local is)
func SetHoldItemsConverted(o *Obj, s string) {
	if o.Spec.Hold == nil {
		o.Spec.Hold = &Holder{}
	}
	items := o.Spec.Rows[0]
	items = append(items, s)
	o.Spec.Hold.Items = Items(items)
}

// inadmissible: an append into the helper's own slice, converted on its way
// back through a pointer the body initialised
func SetHoldItemsFreshConverted(o *Obj, s string) {
	if o.Spec.Hold == nil {
		o.Spec.Hold = &Holder{}
	}
	items := []string{}
	items = append(items, s)
	o.Spec.Hold.Items = Items(items)
}

// inadmissible: an append into a local the helper reassigned its own slice
func AddItemFreshReassigned(o *Obj, s string) {
	items := o.Spec.Items
	items = []string{}
	items = append(items, s)
	o.Spec.Items = items
}

// inadmissible: an append into a local, written back after the key changed
func AddGroupItemIndexReassigned(o *Obj, k, j, s string) {
	if o.Spec.Groups == nil {
		o.Spec.Groups = map[string][]string{}
	}
	items := o.Spec.Groups[k]
	items = append(items, s)
	k = j
	o.Spec.Groups[k] = items
}

// inadmissible: an append into a local reassigned by a comma-ok type assertion
func AddItemTypeAssertSource(o *Obj, s string) {
	items := o.Spec.Items
	items, _ = o.Spec.Payload.([]string)
	items = append(items, s)
	o.Spec.Items = items
}

// inadmissible: the same with a comma-ok receive
func AddItemReceiveSource(o *Obj, ch chan []string, s string) {
	items := o.Spec.Items
	items, _ = <-ch
	items = append(items, s)
	o.Spec.Items = items
}

// inadmissible: an append into a local sliced from the field
func AddItemFullSliceSource(o *Obj, s string) {
	items := o.Spec.Items[:]
	items = append(items, s)
	o.Spec.Items = items
}

// inadmissible: the same with a conversion of the field
func AddItemConvertedSource(o *Obj, s string) {
	items := []string(o.Spec.Items)
	items = append(items, s)
	o.Spec.Items = items
}

// inadmissible: the same with an append to the field
func AddItemsAppendSource(o *Obj, a, s string) {
	items := append(o.Spec.Items, a)
	items = append(items, s)
	o.Spec.Items = items
}

// inadmissible: the same with a partial slice, which drops the first item
func AddItemPartialSliceSource(o *Obj, s string) {
	items := o.Spec.Items[1:]
	items = append(items, s)
	o.Spec.Items = items
}

// inadmissible: a nested append extends the field by a constant first
func AddItemNestedConstant(o *Obj, s string) {
	o.Spec.Items = append(append(o.Spec.Items, "fixed"), s)
}

// inadmissible: an append of two values, one of them a constant
func AddItemTwoValues(o *Obj, s string) { o.Spec.Items = append(o.Spec.Items, s, "fixed") }

// inadmissible: a spread append splices a slice, not one value
func AddItemsSpread(o *Obj, xs []string) { o.Spec.Items = append(o.Spec.Items, xs...) }

// inadmissible: the append extends another key's element
func AddGroupItemCrossKeyDirect(o *Obj, k, j, s string) {
	if o.Spec.Groups == nil {
		o.Spec.Groups = map[string][]string{}
	}
	o.Spec.Groups[j] = append(o.Spec.Groups[k], s)
}

// inadmissible: the append is written through an alias of the field's parent
func AddItemAliasMismatch(o *Obj, s string) {
	spec := &o.Spec
	spec.Items = append(o.Spec.Items, s)
}

// inadmissible: an append in a var declaration, inserted under another key
func AddGroupItemVarAppend(o *Obj, k, j, s string) {
	if o.Spec.Groups == nil {
		o.Spec.Groups = map[string][]string{}
	}
	var items = append(o.Spec.Groups[k], s)
	o.Spec.Groups[j] = items
}

// inadmissible: an append sliced before it is inserted under another key
func AddGroupItemSlicedAppend(o *Obj, k, j, s string) {
	if o.Spec.Groups == nil {
		o.Spec.Groups = map[string][]string{}
	}
	o.Spec.Groups[j] = append(o.Spec.Groups[k], s)[:]
}

// inadmissible: an append discarded into the blank identifier, beside a
// pointer write
func SetReplicasBlankAppend(o *Obj, s string, n int32) {
	_ = append(o.Spec.Items, s)
	o.Spec.Replicas = &n
}

// inadmissible: an append to a temporary's field, beside a pointer write
func SetReplicasAppendTempField(o *Obj, s string, n int32) {
	tmp := &Obj{}
	tmp.Spec.Items = append(tmp.Spec.Items, s)
	o.Spec.Replicas = &n
}

// inadmissible: a compound assignment into a temporary map, beside a pointer write
func SetReplicasTempMapConcat(o *Obj, k, v string, n int32) {
	m := map[string]string{}
	m[k] += v
	o.Spec.Replicas = &n
}

// inadmissible: a parenthesised append builtin still appends to a local
func SetReplicasParenAppendLocal(o *Obj, s string, n int32) {
	var items []string
	items = (append)(items, s)
	o.Spec.Replicas = &n
}

// inadmissible: a parenthesised map-index target still inserts into a local
func SetReplicasParenMapTarget(o *Obj, k, v string, n int32) {
	labels := map[string]string{}
	(labels[k]) = v
	o.Spec.Replicas = &n
}

// inadmissible: a parenthesised make builtin still fills the appended row
func AddRowParenMakeLen(o *Obj, n int) {
	o.Spec.Rows = append(o.Spec.Rows, (make)([]string, 1, n))
}

// inadmissible: a parenthesised local assigned nil still clears the field
func SetRefViaParenAssignedNil(o *Obj, ref *Ref) {
	r := ref
	(r) = nil
	o.Spec.Ref = r
}

// class a: a parenthesised append builtin is still the append
func AddItemParenBuiltin(o *Obj, s string) { o.Spec.Items = (append)(o.Spec.Items, s) }

// class a: a parenthesised map-index target is still the insert
func AddLabelParenTarget(o *Obj, k, v string) {
	if o.Labels == nil {
		o.Labels = map[string]string{}
	}
	(o.Labels[k]) = v
}

// class b: a parenthesised local assigned a parameter carries the caller's value
func SetRefNameViaParenLocal(o *Obj, n string) {
	var name string
	(name) = n
	o.Spec.Ref = &Ref{Name: name}
}

// class a: the same constant key on both sides
func AddGroupItemConstKeyDirect(o *Obj, s string) {
	if o.Spec.Groups == nil {
		o.Spec.Groups = map[string][]string{}
	}
	o.Spec.Groups["a"] = append(o.Spec.Groups["a"], s)
}

// class a: a parenthesised append base is the same field
func AddItemParenBase(o *Obj, s string) { o.Spec.Items = append((o.Spec.Items), s) }

// class a: the append beside a pointer write
func AddItemAndReplicas(o *Obj, s string, n int32) {
	o.Spec.Items = append(o.Spec.Items, s)
	o.Spec.Replicas = &n
}

// class b: an element of a temporary slice is not a map element
func SetReplicasTempSliceExpanded(o *Obj, v string, n int32) {
	xs := []string{"a"}
	xs[0] = xs[0] + v
	o.Spec.Replicas = &n
}

// inadmissible: an increment of a temporary map's element, beside a pointer write
func SetReplicasTempMapInc(o *Obj, k string, n int32) {
	m := map[string]int{}
	m[k]++
	_ = m
	o.Spec.Replicas = &n
}

// inadmissible: a range clause assigning a temporary map's element
func SetReplicasTempMapRange(o *Obj, k string, xs []string, n int32) {
	m := map[string]string{}
	for _, m[k] = range xs {
	}
	_ = m
	o.Spec.Replicas = &n
}

// inadmissible: a type parameter; the classifier reads concrete types only
func SetReplicasGenericMap[M ~map[string]string](o *Obj, m M, k, v string, n int32) {
	m[k] = v
	o.Spec.Replicas = &n
}

// inadmissible: the same for an append of a converted type-parameter value
func AddItemGeneric[S ~string](o *Obj, s S) { o.Spec.Items = append(o.Spec.Items, string(s)) }

// inadmissible: a function literal the helper calls, whose body is not checked
func SetReplicasViaClosure(o *Obj, s string, n int32) {
	func() {
		items := o.Spec.Rows[0]
		items = append(items, s)
		o.Spec.Items = items
	}()
	o.Spec.Replicas = &n
}

// inadmissible: the tuple re-roots the alias the append goes through
func AddItemTupleReroot(o *Obj, s string) {
	spec := &Spec{}
	spec, spec.Items = &o.Spec, append(spec.Items, s)
}

// inadmissible: a tuple initialises the pointer the append goes through
func AddItemTupleThroughInit(o *Obj, s string) {
	o.Spec.Hold, o.Spec.Hold.Items = &Holder{}, append(o.Spec.Hold.Items, s)
}

// inadmissible: an alias that reaches the caller only on some paths
func AddItemConditionalAlias(o *Obj, s string, ok bool) {
	spec := &Spec{}
	if ok {
		spec = &o.Spec
	}
	spec.Items = append(spec.Items, s)
}

// inadmissible: the parameter is reassigned before the append
func AddItemParamReassigned(o, other *Obj, s string, ok bool) {
	if ok {
		o = &Obj{}
	} else {
		o = other
	}
	o.Spec.Items = append(o.Spec.Items, s)
}

// inadmissible: a range clause reassigns the parameter
func AddItemParamRanged(o *Obj, objs []*Obj, s string) {
	for _, o = range objs {
	}
	o.Spec.Items = append(o.Spec.Items, s)
}

// inadmissible: the parameter's address is taken, and it is reassigned through it
func AddItemParamAddressTaken(o *Obj, s string) {
	p := &o
	*p = &Obj{Spec: Spec{Name: s}}
	o.Spec.Items = append(o.Spec.Items, s)
}

// inadmissible: a struct copy's field is the helper's own
func AddItemStructCopy(o *Obj, s string) {
	spec := o.Spec
	spec.Items = append(spec.Items, s)
}

// inadmissible: an element of a slice parameter is not a pointer parameter
func AddItemSliceParam(objs []*Obj, s string) {
	objs[0].Spec.Items = append(objs[0].Spec.Items, s)
}

// inadmissible: class a is a single append
func AddItemTwice(o *Obj, s string) {
	o.Spec.Items = append(o.Spec.Items, s)
	o.Spec.Items = append(o.Spec.Items, s)
}

// inadmissible: class a is a single insert
func AddLabelTwice(o *Obj, k, j, v string) {
	if o.Labels == nil {
		o.Labels = map[string]string{}
	}
	o.Labels[k] = v
	o.Labels[j] = v
}

// inadmissible: an insert into a map parameter, beside a pointer write
func SetReplicasMapParam(o *Obj, m map[string]string, k, v string, n int32) {
	m[k] = v
	o.Spec.Replicas = &n
}

// class a: an explicit dereference of the parameter
func AddItemExplicitDeref(o *Obj, s string) {
	(*o).Spec.Items = append((*o).Spec.Items, s)
}

// inadmissible: two keys that print alike are not the same key, with no local
func AddGroupItemLiteralKeysDirect(o *Obj, s string) {
	if o.Spec.Groups == nil {
		o.Spec.Groups = map[string][]string{}
	}
	o.Spec.Groups[string([]byte{'b'})] = append(o.Spec.Groups[string([]byte{'a'})], s)
}

// inadmissible: an append into a local, beside an append extending another
// field (pins the off-target reason before the indirect one)
func AddItemLocalThenIndirect(o *Obj, s string) {
	items := o.Spec.Rows[0]
	items = append(items, s)
	o.Spec.Items = append(o.Spec.Rows[1], s)
}

// inadmissible: an append extending another field, beside an append passed as
// an argument (indirect before stray)
func AddItemIndirectBesideStray(o *Obj, s string) {
	o.Spec.Items = append(o.Spec.Rows[0], s)
	_ = len(append(o.Spec.Rows[1], s))
}

// inadmissible: an append passed as an argument, beside two appends (stray
// before repeated)
func AddItemStrayBesideTwice(o *Obj, s string) {
	o.Spec.Items = append(o.Spec.Items, s)
	o.Spec.Rows = append(o.Spec.Rows, []string{s})
	_ = len(append(o.Spec.Items, s))
}

// inadmissible: two appends, beside a bare write of a computed value
// (repeated before the bare write)
func AddItemTwiceBesideBare(o *Obj, s string) {
	o.Spec.Items = append(o.Spec.Items, s)
	o.Spec.Items = append(o.Spec.Items, s)
	o.Spec.Name = s + s
}

// inadmissible: the map is reached through an address-of, not a field path
func AddLabelAddrDeref(o *Obj, k, v string) {
	if o.Labels == nil {
		o.Labels = map[string]string{}
	}
	(*&o.Labels)[k] = v
}

// inadmissible: a key the comparison cannot equate, even spelled alike
func AddGroupItemConvKeyDirect(o *Obj, k, s string) {
	if o.Spec.Groups == nil {
		o.Spec.Groups = map[string][]string{}
	}
	o.Spec.Groups[string(k)] = append(o.Spec.Groups[string(k)], s)
}

// inadmissible: a parenthesised reassignment of the parameter
func AddItemParenParamReassigned(o, other *Obj, s string) {
	(o) = other
	o.Spec.Items = append(o.Spec.Items, s)
}

// inadmissible: a parenthesised address-of the parameter
func AddItemParenAddressTaken(o *Obj, s string) {
	p := &(o)
	*p = &Obj{Spec: Spec{Name: s}}
	o.Spec.Items = append(o.Spec.Items, s)
}

// inadmissible: a short variable declaration that redeclares the parameter
func AddItemParamRedeclared(o, other *Obj, s string) {
	o, n := other, 0
	_ = n
	o.Spec.Items = append(o.Spec.Items, s)
}

// class a: a parenthesised append value is still the append
func AddItemParenAppendValue(o *Obj, s string) { o.Spec.Items = (append(o.Spec.Items, s)) }

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
		"AddItemMakeLen":         Inadmissible,
		"AddItemGuardedMakeLen":  Inadmissible,
		"AddItemMakeParamLen":    Inadmissible,
		"AddItemMakeZero":        Append,
		"AddItemMakeCap":         Append,
		"AddLabelMakeHint":       Append,
		"AddRowMakeLen":          Inadmissible,
		"SetRefPayloadDefault":   Inadmissible,
		"SetRefPayloadGuarded":   Inadmissible,
		"SetRefNestedReset":      Inadmissible,
		"SetRefPayloadMake":      Inadmissible,
		"SetBoxName":             Pointer,
		"SetBoxPayloadMake":      Inadmissible,
		"SetBoxPayloadNew":       Inadmissible,
		"SetBoxChanMake":         Inadmissible,
		"AddItemEmptyLit":        Append,
		"AddItemConstant":        Inadmissible,
		"AddLabelConstant":       Inadmissible,
		"SetReplicasZero":        Inadmissible,
		"SetNestedRefConstant":   Inadmissible,
		"SetItemsLiteral":        Inadmissible,
		"SetReplicasByValue":     Inadmissible,
		"SetReplicasNilReturn":   Inadmissible,
		"SetNameIfSet":           Inadmissible,
		"AddLabel":               Append,
		"AddLabelViaLocal":       Inadmissible,
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
		"AddItemViaLocal":        Inadmissible,
		"AddLabelParenLocal":     Inadmissible,
		"AddLabelFreshMap":       Inadmissible,
		"AddItemInClosure":       Inadmissible,
		"AddItemViaAlias":        Inadmissible,
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
		// A field write that runs only on some paths (#751).
		"SetRefIfSet":              Inadmissible,
		"SetNestedRefIfSet":        Inadmissible,
		"AddItemIfSet":             Inadmissible,
		"AddLabelIfSetViaLocal":    Inadmissible,
		"SetRefUnlessSet":          Inadmissible,
		"SetRefNameElse":           Inadmissible,
		"SetRefNameNestedInit":     Inadmissible,
		"SetRefGuardedByOther":     Inadmissible,
		"SetRefNameIfInit":         Inadmissible,
		"SetReplicasSwitch":        Inadmissible,
		"AddItemsLoop":             Inadmissible,
		"SetRefGoto":               Inadmissible,
		"SetRefNameNilFirst":       Pointer,
		"SetRefIfUnsetNewValue":    Inadmissible,
		"SetRefNameGuardExtra":     Inadmissible,
		"AddItemsForLoop":          Inadmissible,
		"SetRefTypeSwitch":         Inadmissible,
		"SetRefSelect":             Inadmissible,
		"SetRefNameIfNotNil":       Inadmissible,
		"SetReplicasForPost":       Inadmissible,
		"SetRefAndReplicasInGuard": Inadmissible,
		"SetRefElse":               Inadmissible,
		"SetRefSelectComm":         Inadmissible,
		"SetRefNestedInit":         Inadmissible,
		"SetRefBlockIf":            Inadmissible,
		"SetRefLabeledLoop":        Inadmissible,
		"SetRefBlock":              Pointer,
		"SetRefNameShadowedNil":    Inadmissible,
		// A range-clause target and a read-modify-write (#913).
		"SetRefRangeValue":       Inadmissible,
		"SetCountRangeKey":       Inadmissible,
		"SetReplicasIncCount":    Inadmissible,
		"SetReplicasIncOnly":     Inadmissible,
		"SetReplicasDecCount":    Inadmissible,
		"SetReplicasAddCount":    Inadmissible,
		"AddLabelConcatViaLocal": Inadmissible,
		// An indexed, parenthesised or nil-initialised caller-reaching target.
		"SetReplicasItemConcat":       Inadmissible,
		"SetReplicasItemRange":        Inadmissible,
		"SetReplicasParenLabelConcat": Inadmissible,
		"AddLabelConcatNilInit":       Inadmissible,
		"SetReplicasIncNilInit":       Inadmissible,
		"SetReplicasTempCount":        Pointer,
		// A target reached through a slice expression or a chain of aliases.
		"SetReplicasSliceExprConcat":      Inadmissible,
		"SetReplicasSliceExprRange":       Inadmissible,
		"SetReplicasIncAliasNilInit":      Inadmissible,
		"SetReplicasSlicedAliasConcat":    Inadmissible,
		"SetReplicasIncParenAliasNilInit": Inadmissible,
		// A comma-ok map read is a copy like any other.
		"SetReplicasCommaOkConcat":    Inadmissible,
		"SetReplicasCommaOkVarConcat": Inadmissible,
		"SetReplicasCommaOkRange":     Inadmissible,
		"SetReplicasCommaOkTemp":      Pointer,
		"AddItemCommaOkCrossField":    Inadmissible,
		// An assignment whose value reads the target it overwrites.
		"AddLabelExpandedConcat":        Inadmissible,
		"AddLabelExpandedViaLocal":      Inadmissible,
		"AddLabelExpandedParen":         Inadmissible,
		"AddGroupItem":                  Append,
		"SetReplicasTempExpandedConcat": Inadmissible,
		// Aliases: a copy of the caller's path, an append result, a range variable.
		"AddLabelExpandedCrossAlias":   Inadmissible,
		"AddLabelExpandedFromAlias":    Inadmissible,
		"AddLabelFromTemp":             Append,
		"SetReplicasAppendAliasConcat": Inadmissible,
		"SetReplicasAppendTempConcat":  Inadmissible,
		"SetReplicasRangeAliasConcat":  Inadmissible,
		"SetReplicasRangeCopyConcat":   Pointer,
		"AddLabelFromOther":            Append,
		"SetReplicasRangeTempConcat":   Pointer,
		// An append into or insert into a local, whatever it is written back to.
		"AddItemCrossField":           Inadmissible,
		"AddItemRowCrossField":        Inadmissible,
		"AddItemFromParam":            Inadmissible,
		"AddItemReassignedCrossField": Inadmissible,
		"AddItemTwoWriteBacks":        Inadmissible,
		// Every spelling of such a local's sources and write-backs.
		"AddItemReassignedCrossFieldLast":      Inadmissible,
		"AddItemTwoWriteBacksLast":             Inadmissible,
		"AddItemParenReassignedCrossField":     Inadmissible,
		"AddItemParenMultiReassigned":          Inadmissible,
		"AddItemVarParenCrossField":            Inadmissible,
		"AddItemAppendCrossField":              Inadmissible,
		"AddItemAppendWrittenBackField":        Inadmissible,
		"AddItemsNestedAppend":                 Inadmissible,
		"AddItemAppendFresh":                   Inadmissible,
		"AddItemSliceReassignedCrossField":     Inadmissible,
		"AddItemConvertedReassignedCrossField": Inadmissible,
		"AddItemCopiedSliceCrossField":         Inadmissible,
		"AddItemCommaOkReassignedCrossField":   Inadmissible,
		"AddItemRangeReassignedCrossField":     Inadmissible,
		"AddItemMultiAssignedCrossField":       Inadmissible,
		"AddItemMultiAssignedCrossFieldLast":   Inadmissible,
		"AddItemSwappedSameField":              Inadmissible,
		"AddItemSwapHidesWrite":                Inadmissible,
		"SetReplicasCommaOkCrossField":         Inadmissible,
		"SetReplicasConvertedCrossField":       Inadmissible,
		"SetReplicasSlicedCrossField":          Inadmissible,
		"SetReplicasFreshWriteBack":            Inadmissible,
		"SetReplicasFreshMapWriteBack":         Inadmissible,
		"SetReplicasOverwrittenWriteBack":      Inadmissible,
		"SetHoldCommaOkCrossField":             Inadmissible,
		"SetHoldFreshMapWriteBack":             Inadmissible,
		"SetRefNameComputed":                   Pointer,
		"AddHoldItemViaLocal":                  Inadmissible,
		"AddItemViaLocalAndReplicas":           Inadmissible,
		"AddGroupItemViaLocal":                 Inadmissible,
		"AddItemViaVarLocal":                   Inadmissible,
		"AddGroupItemCrossKey":                 Inadmissible,
		"AddGroupItemLiteralKeys":              Inadmissible,
		"AddGroupItemConstKey":                 Inadmissible,
		// A conversion, a parenthesised index, a struct or array copy (#918).
		"SetReplicasConvertedInc":      Inadmissible,
		"SetReplicasClonedInc":         Pointer,
		"AddLabelExpandedParenIndex":   Inadmissible,
		"AddLabelFromOtherKey":         Append,
		"SetReplicasRangeHolderConcat": Inadmissible,
		"SetReplicasRangeNestConcat":   Inadmissible,
		"SetReplicasHolderParamConcat": Inadmissible,
		"SetReplicasArrayParamConcat":  Inadmissible,
		// Class a is one direct statement on a stable pointer parameter
		// (#919, #921, #922, #923, #924).
		"AddItemFromRowDirect":          Inadmissible,
		"AddItemLocalBase":              Inadmissible,
		"AddItemSameFieldLocalBase":     Inadmissible,
		"AddGroupItemCommaOkCrossKey":   Inadmissible,
		"SetHoldItemsConverted":         Inadmissible,
		"SetHoldItemsFreshConverted":    Inadmissible,
		"AddItemFreshReassigned":        Inadmissible,
		"AddGroupItemIndexReassigned":   Inadmissible,
		"AddItemTypeAssertSource":       Inadmissible,
		"AddItemReceiveSource":          Inadmissible,
		"AddItemFullSliceSource":        Inadmissible,
		"AddItemConvertedSource":        Inadmissible,
		"AddItemsAppendSource":          Inadmissible,
		"AddItemPartialSliceSource":     Inadmissible,
		"AddItemNestedConstant":         Inadmissible,
		"AddItemTwoValues":              Inadmissible,
		"AddItemsSpread":                Inadmissible,
		"AddGroupItemCrossKeyDirect":    Inadmissible,
		"AddItemAliasMismatch":          Inadmissible,
		"AddGroupItemVarAppend":         Inadmissible,
		"AddGroupItemSlicedAppend":      Inadmissible,
		"SetReplicasBlankAppend":        Inadmissible,
		"SetReplicasAppendTempField":    Inadmissible,
		"SetReplicasTempMapConcat":      Inadmissible,
		"SetReplicasParenAppendLocal":   Inadmissible,
		"SetReplicasParenMapTarget":     Inadmissible,
		"AddRowParenMakeLen":            Inadmissible,
		"SetRefViaParenAssignedNil":     Inadmissible,
		"AddItemParenBuiltin":           Append,
		"AddLabelParenTarget":           Append,
		"SetRefNameViaParenLocal":       Pointer,
		"AddGroupItemConstKeyDirect":    Append,
		"AddItemParenBase":              Append,
		"AddItemAndReplicas":            Append,
		"SetReplicasTempSliceExpanded":  Pointer,
		"SetReplicasTempMapInc":         Inadmissible,
		"SetReplicasTempMapRange":       Inadmissible,
		"SetReplicasGenericMap":         Inadmissible,
		"AddItemGeneric":                Inadmissible,
		"SetReplicasViaClosure":         Inadmissible,
		"AddItemTupleReroot":            Inadmissible,
		"AddItemTupleThroughInit":       Inadmissible,
		"AddItemConditionalAlias":       Inadmissible,
		"AddItemParamReassigned":        Inadmissible,
		"AddItemParamRanged":            Inadmissible,
		"AddItemParamAddressTaken":      Inadmissible,
		"AddItemStructCopy":             Inadmissible,
		"AddItemSliceParam":             Inadmissible,
		"AddItemTwice":                  Inadmissible,
		"AddLabelTwice":                 Inadmissible,
		"SetReplicasMapParam":           Inadmissible,
		"AddItemExplicitDeref":          Append,
		"AddGroupItemLiteralKeysDirect": Inadmissible,
		"AddItemLocalThenIndirect":      Inadmissible,
		"AddItemIndirectBesideStray":    Inadmissible,
		"AddItemStrayBesideTwice":       Inadmissible,
		"AddItemTwiceBesideBare":        Inadmissible,
		"AddLabelAddrDeref":             Inadmissible,
		"AddGroupItemConvKeyDirect":     Inadmissible,
		"AddItemParenParamReassigned":   Inadmissible,
		"AddItemParenAddressTaken":      Inadmissible,
		"AddItemParamRedeclared":        Inadmissible,
		"AddItemParenAppendValue":       Append,
		"SetExempted":                   Exempt,
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
		"AddItemMakeLen":        "value the caller did not supply to o.Spec.Items",
		"AddItemGuardedMakeLen": "writes o.Spec.Items (under if o.Spec.Items == nil) only on some paths",
		"AddItemMakeParamLen":   "writes o.Spec.Items (under if o.Spec.Items == nil) only on some paths",
		"AddRowMakeLen":         "value the caller did not supply to o.Spec.Rows",
		"SetRefPayloadDefault":  "value the caller did not supply to o.Spec.Payload",
		"SetRefPayloadGuarded":  "writes o.Spec.Payload (under if o.Spec.Payload == nil) only on some paths",
		"SetRefNestedReset":     "value the caller did not supply to o.Spec.Nested",
		"SetRefPayloadMake":     "value the caller did not supply to o.Spec.Payload",
		"SetBoxPayloadMake":     "value the caller did not supply to o.Spec.Box.Payload",
		"SetBoxPayloadNew":      "value the caller did not supply to o.Spec.Box.Payload",
		"SetBoxChanMake":        "value the caller did not supply to o.Spec.Box.Ch",
		"AddLabelConstant":      "value the caller did not supply to o.Labels[k]",
		"SetReplicasZero":       "value the caller did not supply to o.Spec.Replicas",
		"SetNestedRefConstant":  "value the caller did not supply to o.Spec.Nested.Ref",
		"SetItemsLiteral":       "single bare field assignment",
		"SetReplicasByValue":    "no field write",
		"SetReplicasNilReturn":  "returns early instead of writing",
		"SetNameIfSet":          "returns early instead of writing",
		"AddLabelFreshMap":      "appends to or inserts into labels[k], not a field",
		"AddItemInClosure":      "contains a function literal",
		"SetRefIfSet":           "writes o.Spec.Ref (under if ref != nil) only on some paths",
		"SetRefNameNestedInit":  `(under if n != "")`,
		"SetRefGoto":            "goto",
		"SetRefNameShadowedNil": "writes o.Spec.Ref (under if o.Spec.Ref == nil) only on some paths",
		// A range clause is conditional; a read-modify-write is a default.
		"SetRefRangeValue":                "writes o.Spec.Ref (under range refs) only on some paths",
		"SetCountRangeKey":                "o.Spec.Count (under range xs)",
		"SetReplicasIncCount":             "value the caller did not supply to o.Spec.Count",
		"SetReplicasIncOnly":              "value the caller did not supply to *o.Spec.Replicas",
		"SetReplicasAddCount":             "value the caller did not supply to o.Spec.Count",
		"AddLabelConcatViaLocal":          "value the caller did not supply to labels[k]",
		"SetReplicasItemConcat":           "value the caller did not supply to items[0]",
		"SetReplicasItemRange":            "items[0] (under range xs)",
		"SetReplicasParenLabelConcat":     "value the caller did not supply to",
		"AddLabelConcatNilInit":           "value the caller did not supply to labels[k]",
		"SetReplicasIncNilInit":           "value the caller did not supply to *p",
		"SetReplicasSliceExprConcat":      "value the caller did not supply to o.Spec.Items[:][0]",
		"SetReplicasSliceExprRange":       "o.Spec.Items[:][0] (under range xs)",
		"SetReplicasIncAliasNilInit":      "value the caller did not supply to (*q)",
		"SetReplicasSlicedAliasConcat":    "value the caller did not supply to items[0]",
		"SetReplicasIncParenAliasNilInit": "value the caller did not supply to (*q)",
		"SetReplicasCommaOkConcat":        "value the caller did not supply to items[0]",
		"SetReplicasCommaOkVarConcat":     "value the caller did not supply to items[0]",
		"SetReplicasCommaOkRange":         "items[0] (under range xs)",
		"AddLabelExpandedConcat":          "value the caller did not supply to o.Labels[k]",
		"AddLabelExpandedViaLocal":        "value the caller did not supply to labels[k]",
		"AddLabelExpandedParen":           "value the caller did not supply to",
		"AddLabelExpandedCrossAlias":      "value the caller did not supply to labels[k]",
		"AddLabelExpandedFromAlias":       "value the caller did not supply to o.Labels[k]",
		"SetReplicasAppendAliasConcat":    "value the caller did not supply to items[0]",
		"SetReplicasRangeAliasConcat":     "value the caller did not supply to items[0]",
		// An append into or insert into a local, and the #918 spellings.
		"AddItemCrossField":            "appends to or inserts into items, not a field",
		"AddItemFromParam":             "appends to or inserts into items, not a field",
		"AddGroupItemCrossKey":         "appends to or inserts into items, not a field",
		"SetReplicasConvertedInc":      "value the caller did not supply to (*p)",
		"AddLabelExpandedParenIndex":   "value the caller did not supply to o.Labels[k]",
		"SetReplicasRangeHolderConcat": "value the caller did not supply to h.Items[0]",
		// Every spelling of such a local's sources and write-backs.
		"AddItemReassignedCrossFieldLast":      "appends to or inserts into items, not a field",
		"AddItemTwoWriteBacksLast":             "appends to or inserts into items, not a field",
		"AddItemParenReassignedCrossField":     "appends to or inserts into items, not a field",
		"AddItemParenMultiReassigned":          "appends to or inserts into items, not a field",
		"AddItemVarParenCrossField":            "appends to or inserts into items, not a field",
		"AddItemAppendCrossField":              "appends to or inserts into items, not a field",
		"AddItemAppendFresh":                   "appends to or inserts into items, not a field",
		"AddItemSliceReassignedCrossField":     "appends to or inserts into items, not a field",
		"AddItemConvertedReassignedCrossField": "appends to or inserts into items, not a field",
		"AddItemCopiedSliceCrossField":         "appends to or inserts into items, not a field",
		"AddItemCommaOkReassignedCrossField":   "appends to or inserts into items, not a field",
		"AddItemRangeReassignedCrossField":     "appends to or inserts into items, not a field",
		"AddItemMultiAssignedCrossField":       "appends to or inserts into items, not a field",
		"AddItemMultiAssignedCrossFieldLast":   "appends to or inserts into items, not a field",
		"AddItemSwappedSameField":              "appends to or inserts into items, not a field",
		"AddItemSwapHidesWrite":                "bare write to tmp.Name",
		// An append into or insert into a local is refused beside any other write.
		"SetReplicasCommaOkCrossField":    "appends to or inserts into items, not a field",
		"SetReplicasConvertedCrossField":  "appends to or inserts into items, not a field",
		"SetReplicasSlicedCrossField":     "appends to or inserts into items, not a field",
		"SetReplicasFreshWriteBack":       "appends to or inserts into items, not a field",
		"SetReplicasFreshMapWriteBack":    "appends to or inserts into labels[k], not a field",
		"SetReplicasOverwrittenWriteBack": "appends to or inserts into items, next, not a field",
		"SetHoldCommaOkCrossField":        "appends to or inserts into items, not a field",
		"SetHoldFreshMapWriteBack":        "appends to or inserts into labels[k], not a field",
		"AddLabelIfSetViaLocal":           "appends to or inserts into labels[k], not a field",
		// The fixtures class a no longer admits.
		"AddLabelViaLocal":              "appends to or inserts into labels[k], not a field",
		"AddItemViaLocal":               "appends to or inserts into items, not a field",
		"AddLabelParenLocal":            "appends to or inserts into (labels)[k], not a field",
		"AddItemAppendWrittenBackField": "appends to or inserts into items, not a field",
		"AddItemsNestedAppend":          "appends to or inserts into items, not a field",
		"AddHoldItemViaLocal":           "appends to or inserts into items, not a field",
		"AddItemViaLocalAndReplicas":    "appends to or inserts into items, not a field",
		"AddGroupItemViaLocal":          "appends to or inserts into items, not a field",
		"AddItemViaVarLocal":            "appends to or inserts into items, not a field",
		"AddGroupItemConstKey":          "appends to or inserts into items, not a field",
		"AddItemViaAlias":               "appends to or inserts into spec.Items, not a field",
		"SetReplicasTempExpandedConcat": "appends to or inserts into m[k], not a field",
		"SetReplicasAppendTempConcat":   "appends to or inserts into items, not a field",
		// Class a is one direct statement on a stable pointer parameter.
		"AddItemFromRowDirect":          "writes o.Spec.Items other than as the one target",
		"AddItemLocalBase":              "writes o.Spec.Items other than as the one target",
		"AddItemSameFieldLocalBase":     "writes o.Spec.Items other than as the one target",
		"AddGroupItemCommaOkCrossKey":   "appends to or inserts into items, not a field",
		"SetHoldItemsConverted":         "value the caller did not supply to o.Spec.Hold.Items",
		"SetHoldItemsFreshConverted":    "appends to or inserts into items, not a field",
		"AddItemFreshReassigned":        "appends to or inserts into items, not a field",
		"AddGroupItemIndexReassigned":   "appends to or inserts into items, not a field",
		"AddItemTypeAssertSource":       "appends to or inserts into items, not a field",
		"AddItemReceiveSource":          "appends to or inserts into items, not a field",
		"AddItemFullSliceSource":        "appends to or inserts into items, not a field",
		"AddItemConvertedSource":        "appends to or inserts into items, not a field",
		"AddItemsAppendSource":          "appends to or inserts into items, not a field",
		"AddItemPartialSliceSource":     "appends to or inserts into items, not a field",
		"AddItemNestedConstant":         "writes o.Spec.Items other than as the one target",
		"AddItemTwoValues":              "writes o.Spec.Items other than as the one target",
		"AddItemsSpread":                "writes o.Spec.Items other than as the one target",
		"AddGroupItemCrossKeyDirect":    "writes o.Spec.Groups[j] other than as the one target",
		"AddItemAliasMismatch":          "appends to or inserts into spec.Items, not a field",
		"AddGroupItemVarAppend":         "uses append on o.Spec.Groups[k] other than",
		"AddGroupItemSlicedAppend":      "uses append on o.Spec.Groups[k] other than",
		"SetReplicasBlankAppend":        "appends to or inserts into _, not a field",
		"SetReplicasAppendTempField":    "appends to or inserts into tmp.Spec.Items, not a field",
		"SetReplicasTempMapConcat":      "appends to or inserts into m[k], not a field",
		"SetReplicasParenAppendLocal":   "appends to or inserts into items, not a field",
		"SetReplicasParenMapTarget":     "appends to or inserts into (labels[k]), not a field",
		"AddRowParenMakeLen":            "value the caller did not supply to o.Spec.Rows",
		"SetRefViaParenAssignedNil":     "assigns nil to o.Spec.Ref",
		"AddItemParenBuiltin":           "slice append or map insert (class a)",
		"AddLabelParenTarget":           "slice append or map insert (class a)",
		"SetRefNameViaParenLocal":       "pointer-typed field assignment (class b)",
		"AddGroupItemConstKeyDirect":    "slice append or map insert (class a)",
		"AddItemParenBase":              "slice append or map insert (class a)",
		"AddItemAndReplicas":            "slice append or map insert (class a)",
		"SetReplicasTempSliceExpanded":  "pointer-typed field assignment (class b)",
		"SetReplicasTempMapInc":         "appends to or inserts into m[k], not a field",
		"SetReplicasTempMapRange":       "appends to or inserts into m[k], not a field",
		"SetReplicasGenericMap":         "declares type parameters",
		"AddItemGeneric":                "declares type parameters",
		"SetReplicasViaClosure":         "contains a function literal",
		"AddItemTupleReroot":            "appends to or inserts into spec.Items, not a field",
		"AddItemTupleThroughInit":       "writes o.Spec.Hold.Items other than as the one target",
		"AddItemConditionalAlias":       "appends to or inserts into spec.Items, not a field",
		"AddItemParamReassigned":        "appends to or inserts into o.Spec.Items, not a field",
		"AddItemParamRanged":            "appends to or inserts into o.Spec.Items, not a field",
		"AddItemParamAddressTaken":      "appends to or inserts into o.Spec.Items, not a field",
		"AddItemStructCopy":             "appends to or inserts into spec.Items, not a field",
		"AddItemSliceParam":             "appends to or inserts into objs[0].Spec.Items, not a field",
		"AddItemTwice":                  "makes 2 appends or inserts",
		"AddLabelTwice":                 "makes 2 appends or inserts",
		"SetReplicasMapParam":           "appends to or inserts into m[k], not a field",
		"AddItemExplicitDeref":          "slice append or map insert (class a)",
		"AddGroupItemLiteralKeysDirect": "writes o.Spec.Groups[string([]byte{…})] other than as the one target",
		// Each adjacent pair of the class-a reasons, in order.
		"AddItemLocalThenIndirect":   "appends to or inserts into items, not a field",
		"AddItemIndirectBesideStray": "writes o.Spec.Items other than as the one target",
		"AddItemStrayBesideTwice":    "uses append on o.Spec.Items other than",
		"AddItemTwiceBesideBare":     "makes 2 appends or inserts",
		// A path through &, a key it cannot equate, a parenthesised parameter.
		"AddLabelAddrDeref":           "appends to or inserts into (*&o.Labels)[k], not a field",
		"AddGroupItemConvKeyDirect":   "writes o.Spec.Groups[string(k)] other than as the one target",
		"AddItemParenParamReassigned": "appends to or inserts into o.Spec.Items, not a field",
		"AddItemParenAddressTaken":    "appends to or inserts into o.Spec.Items, not a field",
		"AddItemParamRedeclared":      "appends to or inserts into o.Spec.Items, not a field",
		"AddItemParenAppendValue":     "slice append or map insert (class a)",
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
