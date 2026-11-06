package built

import (
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const probePanicText = "text-of-a-probed-panic"

// probeObj is a caller's object whose methods panic on demand. setCalls counts
// the SetAnnotations calls; the setter panics on call number panicOnSet (1 is
// the first), and never when panicOnSet is 0.
type probeObj struct {
	*corev1.ConfigMap
	panicOn    string
	panicOnSet int
	setCalls   *int
}

func (p probeObj) hit(name string) {
	if p.panicOn == name {
		panic(probePanicText)
	}
}

func (p probeObj) GetName() string              { p.hit("GetName"); return p.ConfigMap.GetName() }
func (p probeObj) GetNamespace() string         { p.hit("GetNamespace"); return p.ConfigMap.GetNamespace() }
func (p probeObj) GetLabels() map[string]string { p.hit("GetLabels"); return p.ConfigMap.GetLabels() }
func (p probeObj) GetAnnotations() map[string]string {
	p.hit("GetAnnotations")
	return p.ConfigMap.GetAnnotations()
}

func (p probeObj) SetAnnotations(a map[string]string) {
	*p.setCalls++
	if p.panicOnSet != 0 && *p.setCalls == p.panicOnSet {
		panic(probePanicText)
	}
	p.ConfigMap.SetAnnotations(a)
}

// probeItems is a typed List whose Items are runtime.Objects.
type probeItems struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             []runtime.Object `json:"items"`
}

func (l *probeItems) DeepCopyObject() runtime.Object { c := *l; return &c }

func newProbeObj(panicOn string, panicOnSet int) (probeObj, *int) {
	calls := 0
	return probeObj{ConfigMap: typedConfigMap("p"), panicOn: panicOn, panicOnSet: panicOnSet, setCalls: &calls}, &calls
}

func wantRefusalNamingType(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("no error")
	}
	if !isRefusal(err) {
		t.Errorf("error %T is not the refusal of an object whose method panicked", err)
	}
	msg := err.Error()
	if strings.Contains(msg, probePanicText) {
		t.Errorf("the refusal carries the panic's text: %q", msg)
	}
	if !strings.Contains(msg, "built.probeObj") {
		t.Errorf("the refusal %q does not name the Go type", msg)
	}
}

// TestProbeHeld_EachMethodPanicIsARefusal: a panic in any of the five probed
// methods is the refusal that names the Go type and not the panic's text.
func TestProbeHeld_EachMethodPanicIsARefusal(t *testing.T) {
	for _, m := range []string{"GetName", "GetNamespace", "GetLabels", "GetAnnotations"} {
		t.Run(m, func(t *testing.T) {
			obj, _ := newProbeObj(m, 0)
			wantRefusalNamingType(t, ProbeHeld(obj))
		})
	}
}

// TestProbeHeld_ProbesTheItemsOfAList: an object held in a typed List is
// probed too, whatever else the List holds.
func TestProbeHeld_ProbesTheItemsOfAList(t *testing.T) {
	bad, _ := newProbeObj("GetLabels", 0)
	inner := &probeItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}, Items: []runtime.Object{bad}}
	list := &probeItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}, Items: []runtime.Object{typedConfigMap("fine"), inner}}
	wantRefusalNamingType(t, ProbeHeld(list))
}

// TestProbeHeld_AcceptsWhatItCannotProbe: a nil, an unstructured and a sound
// typed object are no refusal, and nothing is changed.
func TestProbeHeld_AcceptsWhatItCannotProbe(t *testing.T) {
	var nilObj *corev1.ConfigMap
	good, calls := newProbeObj("", 0)
	if err := ProbeHeld(nil, nilObj, configMap("u"), typedConfigMap("t"), good); err != nil {
		t.Fatalf("ProbeHeld: %v", err)
	}
	if *calls != 0 {
		t.Errorf("the probe called SetAnnotations %d times", *calls)
	}
}

// TestAddAnnotationsHeld_UnchangedReturnsNoUndo: annotations already present
// change nothing, so there is nothing to take back.
func TestAddAnnotationsHeld_UnchangedReturnsNoUndo(t *testing.T) {
	obj, calls := newProbeObj("", 0)
	obj.SetAnnotations(map[string]string{"a": "1"})
	*calls = 0
	undo, err := AddAnnotationsHeld(obj, map[string]string{"a": "1"})
	if err != nil || undo != nil {
		t.Fatalf("got undo %v, err %v; want neither", undo != nil, err)
	}
	if *calls != 0 {
		t.Errorf("SetAnnotations called %d times for nothing to change", *calls)
	}
}

// TestAddAnnotationsHeld_SetAndUndo: the annotations are set, and the undo
// takes them back and returns no error.
func TestAddAnnotationsHeld_SetAndUndo(t *testing.T) {
	obj, _ := newProbeObj("", 0)
	undo, err := AddAnnotationsHeld(obj, map[string]string{"k": "v"})
	if err != nil || undo == nil {
		t.Fatalf("got undo %v, err %v", undo != nil, err)
	}
	if got := obj.ConfigMap.GetAnnotations()["k"]; got != "v" {
		t.Fatalf("annotation not set: %q", got)
	}
	if err := undo(); err != nil {
		t.Fatalf("undo: %v", err)
	}
	if got, ok := obj.ConfigMap.GetAnnotations()["k"]; ok {
		t.Errorf("undo left the annotation %q", got)
	}
}

// TestAddAnnotationsHeld_PanickingSetterIsARefusal: a setter that panics is
// the refusal naming the Go type, with no undo to run.
func TestAddAnnotationsHeld_PanickingSetterIsARefusal(t *testing.T) {
	obj, _ := newProbeObj("", 1)
	undo, err := AddAnnotationsHeld(obj, map[string]string{"k": "v"})
	wantRefusalNamingType(t, err)
	if undo != nil {
		t.Error("an undo was returned with the refusal")
	}
	if !strings.Contains(err.Error(), "cannot be annotated") {
		t.Errorf("the refusal %q does not say the object cannot be annotated", err)
	}
	if _, ok := obj.ConfigMap.GetAnnotations()["k"]; ok {
		t.Error("the object was annotated by a call that panicked")
	}
}

// TestAddAnnotationsHeld_PanickingUndoIsARefusalNotAPanic: when the setter
// panics the second time, the undo returns the refusal and never panics.
func TestAddAnnotationsHeld_PanickingUndoIsARefusalNotAPanic(t *testing.T) {
	obj, _ := newProbeObj("", 2)
	undo, err := AddAnnotationsHeld(obj, map[string]string{"k": "v"})
	if err != nil || undo == nil {
		t.Fatalf("got undo %v, err %v", undo != nil, err)
	}
	var uerr error
	func() {
		defer func() {
			if p := recover(); p != nil {
				t.Fatalf("the undo panicked: %v", p)
			}
		}()
		uerr = undo()
	}()
	wantRefusalNamingType(t, uerr)
	if !strings.Contains(uerr.Error(), "could not be restored") {
		t.Errorf("the refusal %q does not say the annotations could not be restored", uerr)
	}
	joined := errors.Join(errors.New("original"), uerr)
	if !errors.Is(joined, uerr) {
		t.Error("the undo's refusal is lost when joined")
	}
}

// accessorMeta is the metadata an accessorItem hands out; its GetName panics.
type accessorMeta struct{ metav1.ObjectMeta }

func (*accessorMeta) GetName() string { panic(probePanicText) }

// accessorItem is a List item that is no client.Object: the writers read its
// identity through meta.Accessor, which takes the metadata it exposes through
// ObjectMetaAccessor.
type accessorItem struct {
	metav1.TypeMeta `json:",inline"`
	Meta            *accessorMeta `json:"metadata,omitempty"`
}

func (i *accessorItem) GetObjectMeta() metav1.Object {
	if i.Meta == nil {
		return nil
	}
	return i.Meta
}
func (i *accessorItem) DeepCopyObject() runtime.Object {
	c := *i
	return &c
}

// TestProbeHeld_ProbesTheMetadataAnItemExposesThroughTheAccessor: an item that
// is no client.Object, whose metadata the writers read through meta.Accessor,
// is probed through the same accessor.
func TestProbeHeld_ProbesTheMetadataAnItemExposesThroughTheAccessor(t *testing.T) {
	item := &accessorItem{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Thing"},
		Meta:     &accessorMeta{ObjectMeta: metav1.ObjectMeta{Name: "n"}},
	}
	list := &probeItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}, Items: []runtime.Object{item}}
	err := ProbeHeld(list)
	if err == nil || !isRefusal(err) {
		t.Fatalf("want the refusal of an item whose metadata panics, got %v", err)
	}
	msg := err.Error()
	if strings.Contains(msg, probePanicText) || !strings.Contains(msg, "built.accessorItem") {
		t.Errorf("the refusal %q carries the panic text or does not name built.accessorItem", msg)
	}
}

// TestProbeHeld_AcceptsAnItemWithoutMetadata: an item meta.Accessor cannot read
// is no refusal of the probe's: the reader reports it, in its own words.
func TestProbeHeld_AcceptsAnItemWithoutMetadata(t *testing.T) {
	item := &accessorItem{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Thing"}}
	list := &probeItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}, Items: []runtime.Object{item}}
	if err := ProbeHeld(list); err != nil {
		t.Fatalf("ProbeHeld: %v", err)
	}
}
