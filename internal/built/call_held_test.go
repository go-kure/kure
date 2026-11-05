package built

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

// callHeldProbe is a caller's object whose GetLabels panics with a text the
// refusal must not carry.
type callHeldProbe struct{ *corev1.ConfigMap }

const callHeldPanicText = "text-of-a-callers-panic"

func (callHeldProbe) GetLabels() map[string]string { panic(callHeldPanicText) }

// TestCallHeld_PanicIsARefusalNamingTheTypes: CallHeld, the entry for a reader
// outside the package, turns a panic of the object's method into the refusal
// callHeld returns, with the Go type of the object and of the panic value and
// not the panic's text (go-kure/kure#1267).
func TestCallHeld_PanicIsARefusalNamingTheTypes(t *testing.T) {
	obj := callHeldProbe{ConfigMap: &corev1.ConfigMap{}}
	err := CallHeld(obj, func() { _ = obj.GetLabels() })
	if err == nil {
		t.Fatal("CallHeld returned no error for a method that panics")
	}
	if !isRefusal(err) {
		t.Errorf("error %T is not the refusal of an object whose method panicked", err)
	}
	msg := err.Error()
	if strings.Contains(msg, callHeldPanicText) {
		t.Errorf("the refusal carries the panic's text: %q", msg)
	}
	for _, want := range []string{"built.callHeldProbe", "string"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal %q does not name %q", msg, want)
		}
	}
}

// TestCallHeld_ReturnsNilWhenTheMethodsReturn: f runs once and CallHeld
// returns no error when no method panics.
func TestCallHeld_ReturnsNilWhenTheMethodsReturn(t *testing.T) {
	obj := &corev1.ConfigMap{}
	calls := 0
	if err := CallHeld(obj, func() { calls++; _ = obj.GetLabels() }); err != nil {
		t.Fatalf("CallHeld: %v", err)
	}
	if calls != 1 {
		t.Errorf("f ran %d times, want 1", calls)
	}
}
