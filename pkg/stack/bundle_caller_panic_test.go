package stack

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// secretPanic is the text of every panic below. A refusal names the Go type of
// the object and of the panic value, never the text, so the tests look for it.
const secretPanic = "secret-panic-text-of-a-callers-method"

// panickyObject is a caller's object whose method panics: panicOn names the
// method, and every other method is the embedded ConfigMap's.
type panickyObject struct {
	*corev1.ConfigMap
	panicOn string
}

func (p *panickyObject) GetLabels() map[string]string {
	if p.panicOn == "GetLabels" {
		panic(secretPanic)
	}
	return p.ConfigMap.GetLabels()
}

func (p *panickyObject) SetLabels(l map[string]string) {
	if p.panicOn == "SetLabels" {
		panic(secretPanic)
	}
	p.ConfigMap.SetLabels(l)
}

func (p *panickyObject) GetAnnotations() map[string]string {
	if p.panicOn == "GetAnnotations" {
		panic(secretPanic)
	}
	return p.ConfigMap.GetAnnotations()
}

func (p *panickyObject) SetAnnotations(a map[string]string) {
	if p.panicOn == "SetAnnotations" {
		panic(secretPanic)
	}
	p.ConfigMap.SetAnnotations(a)
}

// TestBundleGenerate_CallerMethodPanicIsARefusal: Bundle.Generate calls
// GetLabels and SetLabels, and GetAnnotations and SetAnnotations through
// built.AddAnnotations, on every object an application emits, before any
// other guard runs (go-kure/kure#1267). A method that panics is a refusal
// that returns no objects and names the Go type of the object and of the
// panic value, not the panic's text.
func TestBundleGenerate_CallerMethodPanicIsARefusal(t *testing.T) {
	for _, method := range []string{"GetLabels", "SetLabels", "GetAnnotations", "SetAnnotations"} {
		t.Run(method, func(t *testing.T) {
			hostile := &panickyObject{
				ConfigMap: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "hostile"}},
				panicOn:   method,
			}
			obj := client.Object(hostile)
			b := &Bundle{
				Name:         "web",
				Applications: []*Application{NewApplication("web", "default", &fakeConfig{objs: []*client.Object{&obj}})},
				Labels:       map[string]string{"team": "shop"},
				Annotations:  map[string]string{"owner": "shop"},
			}

			resources, err := b.Generate()
			if err == nil {
				t.Fatalf("Generate returned no error for an object whose %s panics", method)
			}
			if resources != nil {
				t.Errorf("Generate returned %d objects with its refusal, want none", len(resources))
			}
			msg := err.Error()
			if strings.Contains(msg, secretPanic) {
				t.Errorf("the refusal carries the panic's text: %q", msg)
			}
			if !strings.Contains(msg, "stack.panickyObject") {
				t.Errorf("the refusal does not name the Go type of the object: %q", msg)
			}
			if !strings.Contains(msg, "string") {
				t.Errorf("the refusal does not name the Go type of the panic value: %q", msg)
			}
		})
	}
}

// TestBundleGenerate_AnObjectThatDoesNotPanicIsUnchanged: the guard changes
// nothing for an object whose methods return.
func TestBundleGenerate_AnObjectThatDoesNotPanicIsUnchanged(t *testing.T) {
	obj := client.Object(&panickyObject{
		ConfigMap: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "fine"}},
	})
	b := &Bundle{
		Name:         "web",
		Applications: []*Application{NewApplication("web", "default", &fakeConfig{objs: []*client.Object{&obj}})},
		Labels:       map[string]string{"team": "shop"},
		Annotations:  map[string]string{"owner": "shop"},
	}
	resources, err := b.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("Generate returned %d objects, want 1", len(resources))
	}
	if got := (*resources[0]).GetLabels()["team"]; got != "shop" {
		t.Errorf("label team = %q, want shop", got)
	}
	if got := (*resources[0]).GetAnnotations()["owner"]; got != "shop" {
		t.Errorf("annotation owner = %q, want shop", got)
	}
}
