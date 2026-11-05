package fluxcd_test

import (
	"strings"
	"testing"

	ciliumapi "github.com/cilium/cilium/pkg/policy/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// encoding/json calls the IsZero method of a field tagged omitzero, as it calls
// a marshaler. A method of a type that is not from a trusted package is not
// called: the object that holds it is refused, with the type named
// (go-kure/kure#1260). The child-process case of
// TestHostileResources_NoEntryPointPanics holds the shape that overflows the
// stack.

// zeroCalls counts the calls of the IsZero methods below.
var zeroCalls int

// zeroCounted is a type of this test package, which is not trusted, with an
// IsZero method that returns at once.
type zeroCounted struct{ N int }

func (z zeroCounted) IsZero() bool { zeroCalls++; return z.N == 0 }

// zeroPtrCounted has IsZero on its pointer.
type zeroPtrCounted struct{ N int }

func (z *zeroPtrCounted) IsZero() bool { zeroCalls++; return z.N == 0 }

// isZeroer is an interface that declares IsZero.
type isZeroer interface{ IsZero() bool }

// zeroHolder is an object with one field F, tagged as the test needs.
type zeroHolder[T any] struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	F                 T `json:"f,omitzero"`
}

func (o *zeroHolder[T]) DeepCopyObject() runtime.Object { c := *o; return &c }

func newZeroHolder[T any](f T) *zeroHolder[T] {
	o := &zeroHolder[T]{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Thing"}, F: f}
	o.Name, o.Namespace = "holder", "default"
	return o
}

// zeroUntagged holds fields on which json does not call IsZero.
type zeroUntagged struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	NoOption          zeroCounted `json:"noOption,omitempty"`
	Left              zeroCounted `json:"-"`
	NilIface          isZeroer    `json:"nilIface,omitzero"`
}

func (o *zeroUntagged) DeepCopyObject() runtime.Object { c := *o; return &c }

// zeroTrusted holds fields of a trusted package, whose IsZero json calls.
type zeroTrusted struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	When              metav1.Time  `json:"when,omitzero"`
	WhenPointer       *metav1.Time `json:"whenPointer,omitzero"`
	Plain             int          `json:"plain,omitzero"`
}

func (o *zeroTrusted) DeepCopyObject() runtime.Object { c := *o; return &c }

// ifaceHolder is one struct type with an interface field that declares IsZero.
type ifaceHolder = zeroHolder[isZeroer]

func zeroRefusedNaming(t *testing.T, r client.Object, want string) {
	t.Helper()
	zeroCalls = 0
	refusedWithoutPanic(t, r, want)
	if zeroCalls != 0 {
		t.Errorf("IsZero was called %d times, want none", zeroCalls)
	}
}

// zeroNotRefused fails if the entry point refuses r for an IsZero method.
func zeroNotRefused(t *testing.T, r client.Object) {
	t.Helper()
	zeroCalls = 0
	_, err := fluxstack.UmbrellaTerminals(&layout.ManifestLayout{Name: "root", Resources: []client.Object{r}})
	if err != nil && strings.Contains(err.Error(), "IsZero") {
		t.Errorf("got %v, want no refusal for an IsZero", err)
	}
}

func TestCallerObject_UntrustedIsZeroOfOmitzeroFieldIsRefused(t *testing.T) {
	const why = ", whose IsZero method is called for its omitzero tag and is not from a trusted package"
	t.Run("value", func(t *testing.T) {
		zeroRefusedNaming(t, newZeroHolder(zeroCounted{N: 1}), ".F holds a fluxcd_test.zeroCounted"+why)
	})
	t.Run("pointer", func(t *testing.T) {
		zeroRefusedNaming(t, newZeroHolder(&zeroCounted{N: 1}), ".F holds a *fluxcd_test.zeroCounted"+why)
	})
	t.Run("method on the pointer, field by value", func(t *testing.T) {
		zeroRefusedNaming(t, newZeroHolder(zeroPtrCounted{}), ".F holds a fluxcd_test.zeroPtrCounted"+why)
	})
	t.Run("interface holding a value", func(t *testing.T) {
		zeroRefusedNaming(t, newZeroHolder[isZeroer](zeroCounted{N: 1}), ".F holds a fluxcd_test.zeroCounted"+why)
	})
	t.Run("interface holding a pointer", func(t *testing.T) {
		zeroRefusedNaming(t, newZeroHolder[isZeroer](&zeroPtrCounted{N: 1}), ".F holds a *fluxcd_test.zeroPtrCounted"+why)
	})
	t.Run("held by a List item", func(t *testing.T) {
		l := &objectItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}, Items: []runtime.Object{newZeroHolder(zeroCounted{})}}
		l.Name, l.Namespace = "list", "default"
		zeroRefusedNaming(t, l, "fluxcd_test.zeroCounted"+why)
	})
}

// A field the check must leave alone is accepted: the same type without the
// tag option, a field that json leaves out, a nil interface, a type of a
// trusted package, and a field whose type has no IsZero.
func TestCallerObject_IsZeroThatIsNotCalledIsAccepted(t *testing.T) {
	u := &zeroUntagged{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Thing"}}
	u.Name, u.Namespace = "holder", "default"
	u.NoOption.N, u.Left.N = 1, 1
	zeroNotRefused(t, u)
	tr := &zeroTrusted{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Thing"}, WhenPointer: &metav1.Time{}}
	tr.Name, tr.Namespace = "holder", "default"
	zeroNotRefused(t, tr)
}

// The judgment of an interface field follows the value it holds, not the first
// value the walk saw in a field of that struct type: a List with a trusted
// value in one item and an untrusted one in the next refuses the second.
func TestCallerObject_IsZeroInterfaceFieldIsJudgedByEachValue(t *testing.T) {
	item := func(z isZeroer) *ifaceHolder { return newZeroHolder(z) }
	l := &objectItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"},
		Items: []runtime.Object{item(&metav1.Time{}), item(zeroCounted{N: 1})}}
	l.Name, l.Namespace = "list", "default"
	zeroRefusedNaming(t, l, "fluxcd_test.zeroCounted, whose IsZero method is called for its omitzero tag")
	// The trusted value alone is accepted.
	ok := &objectItems{TypeMeta: l.TypeMeta, Items: []runtime.Object{item(&metav1.Time{})}}
	ok.Name, ok.Namespace = "list", "default"
	zeroNotRefused(t, ok)
}

// ZeroEmbedded is exported, so a struct can embed it under an exported name.
type ZeroEmbedded struct{ N int }

func (z ZeroEmbedded) IsZero() bool { zeroCalls++; return z.N == 0 }

// ZeroEmbeddedIface is an exported interface type that declares IsZero.
type ZeroEmbeddedIface interface{ IsZero() bool }

// zeroFlat is an unexported struct type with an IsZero method.
type zeroFlat struct{ N int }

func (z zeroFlat) IsZero() bool { zeroCalls++; return z.N == 0 }

// zeroScalar is an unexported type that is not a struct, with an IsZero method.
type zeroScalar int

func (z zeroScalar) IsZero() bool { zeroCalls++; return z == 0 }

// zeroInner holds a field on which json calls IsZero when it writes the struct.
type zeroInner struct {
	F zeroCounted `json:"f,omitzero"`
}

// The zeroEmbed objects below each embed one type under the tag its test needs.
type zeroEmbedFlat struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	ZeroEmbedded      `json:",omitzero"`
}

func (o *zeroEmbedFlat) DeepCopyObject() runtime.Object { c := *o; return &c }

type zeroEmbedFlatPointer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	*ZeroEmbedded     `json:",omitzero"`
}

func (o *zeroEmbedFlatPointer) DeepCopyObject() runtime.Object { c := *o; return &c }

type zeroEmbedUnexportedFlat struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	zeroFlat          `json:",omitzero"`
}

func (o *zeroEmbedUnexportedFlat) DeepCopyObject() runtime.Object { c := *o; return &c }

type zeroEmbedUnexportedFlatPointer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	*zeroFlat         `json:",omitzero"`
}

func (o *zeroEmbedUnexportedFlatPointer) DeepCopyObject() runtime.Object { c := *o; return &c }

type zeroEmbedScalar struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	zeroScalar        `json:"z,omitzero"`
}

func (o *zeroEmbedScalar) DeepCopyObject() runtime.Object { c := *o; return &c }

type zeroEmbedNamed struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	ZeroEmbedded      `json:"z,omitzero"`
}

func (o *zeroEmbedNamed) DeepCopyObject() runtime.Object { c := *o; return &c }

type zeroEmbedInterface struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	ZeroEmbeddedIface `json:",omitzero"`
}

func (o *zeroEmbedInterface) DeepCopyObject() runtime.Object { c := *o; return &c }

type zeroHidden struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Hidden            zeroInner `json:"-"`
}

func (o *zeroHidden) DeepCopyObject() runtime.Object { c := *o; return &c }

func zeroMeta[T interface {
	GetName() string
	SetName(string)
	SetNamespace(string)
}](o T) T {
	o.SetName("holder")
	o.SetNamespace("default")
	return o
}

// encoding/json flattens an embedded struct that has no name in its tag, and
// ignores an unexported embedded field that is not a struct: it calls no
// IsZero method for either. Such a field is accepted. A struct embedded under a
// name, and an embedded interface (a field named after its type), are fields
// like any other, so their IsZero is judged.
func TestCallerObject_IsZeroOfEmbeddedFieldFollowsEncodingJSON(t *testing.T) {
	const why = " whose IsZero method is called for its omitzero tag and is not from a trusted package"
	tm := metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Thing"}
	t.Run("embedded struct without a name is flattened", func(t *testing.T) {
		zeroNotRefused(t, zeroMeta(&zeroEmbedFlat{TypeMeta: tm, ZeroEmbedded: ZeroEmbedded{N: 1}}))
	})
	t.Run("embedded pointer to a struct without a name is flattened", func(t *testing.T) {
		zeroNotRefused(t, zeroMeta(&zeroEmbedFlatPointer{TypeMeta: tm, ZeroEmbedded: &ZeroEmbedded{N: 1}}))
	})
	t.Run("unexported embedded struct without a name is flattened", func(t *testing.T) {
		zeroNotRefused(t, zeroMeta(&zeroEmbedUnexportedFlat{TypeMeta: tm, zeroFlat: zeroFlat{N: 1}}))
	})
	t.Run("unexported embedded pointer to a struct without a name is flattened", func(t *testing.T) {
		zeroNotRefused(t, zeroMeta(&zeroEmbedUnexportedFlatPointer{TypeMeta: tm, zeroFlat: &zeroFlat{N: 1}}))
	})
	t.Run("unexported embedded type that is not a struct is ignored", func(t *testing.T) {
		zeroNotRefused(t, zeroMeta(&zeroEmbedScalar{TypeMeta: tm, zeroScalar: 1}))
	})
	t.Run("embedded struct under a name is a field", func(t *testing.T) {
		zeroRefusedNaming(t, zeroMeta(&zeroEmbedNamed{TypeMeta: tm, ZeroEmbedded: ZeroEmbedded{N: 1}}),
			".ZeroEmbedded holds a fluxcd_test.ZeroEmbedded,"+why)
	})
	t.Run("embedded interface is a field", func(t *testing.T) {
		zeroRefusedNaming(t, zeroMeta(&zeroEmbedInterface{TypeMeta: tm, ZeroEmbeddedIface: zeroCounted{N: 1}}),
			".ZeroEmbeddedIface holds a fluxcd_test.zeroCounted,"+why)
	})
	t.Run("nil embedded interface is not called", func(t *testing.T) {
		zeroNotRefused(t, zeroMeta(&zeroEmbedInterface{TypeMeta: tm}))
	})
}

// A field tagged json:"-" is judged like any other. The default encoder leaves
// it out, but a marshaler or kure's own code can write it, and each exemption
// found so far was a hole. So an untrusted IsZero inside it is refused.
func TestCallerObject_IsZeroInsideFieldTaggedDashIsRefused(t *testing.T) {
	const why = " whose IsZero method is called for its omitzero tag and is not from a trusted package"
	tm := metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Thing"}
	zeroRefusedNaming(t, zeroMeta(&zeroHidden{TypeMeta: tm, Hidden: zeroInner{F: zeroCounted{N: 1}}}),
		".Hidden.F holds a fluxcd_test.zeroCounted,"+why)
	zeroRefusedNaming(t, newZeroHolder(zeroInner{F: zeroCounted{N: 1}}),
		".F.F holds a fluxcd_test.zeroCounted,"+why)
}

// json calls no IsZero on a nil pointer, nor on an interface that holds one, so
// an optional field that is not set is not judged. The same types with a value
// are refused.
func TestCallerObject_IsZeroOfNilPointerIsNotJudged(t *testing.T) {
	const why = " whose IsZero method is called for its omitzero tag and is not from a trusted package"
	zeroNotRefused(t, newZeroHolder((*zeroPtrCounted)(nil)))
	zeroNotRefused(t, newZeroHolder((*zeroCounted)(nil)))
	zeroNotRefused(t, newZeroHolder[isZeroer]((*zeroPtrCounted)(nil)))
	zeroNotRefused(t, newZeroHolder[isZeroer](nil))
	zeroRefusedNaming(t, newZeroHolder(&zeroPtrCounted{N: 1}), ".F holds a *fluxcd_test.zeroPtrCounted,"+why)
	zeroRefusedNaming(t, newZeroHolder[isZeroer](&zeroPtrCounted{N: 1}), ".F holds a *fluxcd_test.zeroPtrCounted,"+why)
}

// EndpointSelector has the name of the Cilium type below, in another package.
type EndpointSelector struct{ N int }

func (e EndpointSelector) IsZero() bool { zeroCalls++; return e.N == 0 }

// Cilium's policy EndpointSelector is trusted by its exact name: kure's own
// CiliumNetworkPolicy builders emit it, and its IsZero tests one field for nil
// (go-kure/kure#1260). The name alone is not enough: a type of another package
// with the same name is refused. The Cilium policy package declares no other
// type with an IsZero method, so the same-name type stands for "another type".
func TestCallerObject_IsZeroOfTrustedCiliumEndpointSelectorIsAccepted(t *testing.T) {
	const why = " whose IsZero method is called for its omitzero tag and is not from a trusted package"
	zeroNotRefused(t, newZeroHolder(ciliumapi.EndpointSelector{}))
	zeroNotRefused(t, newZeroHolder(&ciliumapi.EndpointSelector{}))
	zeroNotRefused(t, newZeroHolder[isZeroer](&ciliumapi.EndpointSelector{}))
	zeroRefusedNaming(t, newZeroHolder(EndpointSelector{N: 1}), ".F holds a fluxcd_test.EndpointSelector,"+why)
	zeroRefusedNaming(t, newZeroHolder(&EndpointSelector{N: 1}), ".F holds a *fluxcd_test.EndpointSelector,"+why)
}

// zeroShadowed embeds a struct whose field F is written under the JSON name
// "f", and declares its own field under that name: json writes only the outer
// one and never calls the inner IsZero.
type zeroShadowInner struct {
	F zeroCounted `json:"f,omitzero"`
}

type zeroShadowed struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	zeroShadowInner
	F int `json:"f"`
}

func (o *zeroShadowed) DeepCopyObject() runtime.Object { c := *o; return &c }

// zeroBadName embeds a struct under a tag whose name is not valid for json.
type zeroBadName struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	ZeroEmbedded      `json:"\t,omitzero"`
}

func (o *zeroBadName) DeepCopyObject() runtime.Object { c := *o; return &c }

// Two shapes where json's own field selection would not call the IsZero, and
// the check refuses anyway. That is the documented contract: the check follows
// json for the plain cases and refuses where the choice turns on name dominance
// or name validity, because a refusal is the safe side and a copy of json's
// rules is a place for a hole.
func TestCallerObject_IsZeroWhereJSONFieldSelectionIsSubtleIsRefused(t *testing.T) {
	const why = " whose IsZero method is called for its omitzero tag and is not from a trusted package"
	tm := metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Thing"}
	t.Run("field shadowed by a shallower field of the same JSON name", func(t *testing.T) {
		zeroRefusedNaming(t, zeroMeta(&zeroShadowed{TypeMeta: tm, zeroShadowInner: zeroShadowInner{F: zeroCounted{N: 1}}}),
			".zeroShadowInner.F holds a fluxcd_test.zeroCounted,"+why)
	})
	t.Run("embedded struct under a tag name that is not valid", func(t *testing.T) {
		zeroRefusedNaming(t, zeroMeta(&zeroBadName{TypeMeta: tm, ZeroEmbedded: ZeroEmbedded{N: 1}}),
			".ZeroEmbedded holds a fluxcd_test.ZeroEmbedded,"+why)
	})
}

// A struct that marshals itself may write a field the default encoder leaves
// out: RawExtension marshals its Object, which is tagged "-", when Raw is nil.
// So an untrusted IsZero inside that Object is judged and refused.
func TestCallerObject_IsZeroInsideRawExtensionObjectIsRefused(t *testing.T) {
	inner := newZeroHolder(zeroCounted{N: 1})
	zeroRefusedNaming(t, newZeroHolder(runtime.RawExtension{Object: inner}),
		".F.Object")
	zeroRefusedNaming(t, newZeroHolder(runtime.RawExtension{Object: inner}),
		"fluxcd_test.zeroCounted, whose IsZero method is called for its omitzero tag and is not from a trusted package")
}
