package fluxcd_test

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// anyHolder is a typed object with one field that holds any value.
type anyHolder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	V                 any `json:"v,omitempty"`
}

func (o *anyHolder) DeepCopyObject() runtime.Object { c := *o; return &c }

// dashHolder is a typed object whose fields are left out of the JSON it is
// written as: encoding/json skips a field tagged json:"-" and never reads it.
type dashHolder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Skipped           []struct{}     `json:"-"`
	SkippedMap        map[string]any `json:"-"`
}

func (o *dashHolder) DeepCopyObject() runtime.Object { c := *o; return &c }

// namedDash is the same field written as the name "-": json:"-," names the field
// "-", so it is written and is walked.
type namedDash struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Named             []struct{} `json:"-,"`
}

func (o *namedDash) DeepCopyObject() runtime.Object { c := *o; return &c }

func umbrellaOutcome(t *testing.T, r client.Object) error {
	t.Helper()
	ml := &layout.ManifestLayout{Name: "root", Resources: []client.Object{r}}
	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("UmbrellaTerminals panicked: %v", p)
		}
	}()
	_, err := fluxstack.UmbrellaTerminals(ml)
	return err
}

// A chain of pointers to an interface is one JSON value: encoding/json writes
// it with no limit on the pointers, so the depth limit of the walk counts the
// levels of the JSON, not the pointer and interface steps between them
// (go-kure/kure#1201).
func TestCallerObject_PointerChainIsNotDepth(t *testing.T) {
	chain := func(n int) any {
		var v any = "leaf"
		for range n {
			next := v
			v = &next
		}
		return v
	}
	for _, n := range []int{100, 50000} {
		o := &anyHolder{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Thing"}, V: chain(n)}
		o.Name, o.Namespace = "holder", "default"
		if err := umbrellaOutcome(t, o); err != nil {
			t.Errorf("a chain of %d pointers was refused: %v", n, err)
		}
	}

	// A chain of millions ends origin/main's own write in a stack overflow that
	// nothing recovers: it is refused, by a walk that does not recurse for it.
	o := &anyHolder{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Thing"}, V: chain(2_090_000)}
	o.Name, o.Namespace = "holder", "default"
	if err := umbrellaOutcome(t, o); err == nil || !strings.Contains(err.Error(), "in a row") {
		t.Errorf("a chain of 2.09 million pointers: got %v, want a refusal naming the chain", err)
	}
}

// A field tagged json:"-" is left out of what is written, so it is not walked:
// a value in it that holds more than the walk allows, or that holds itself, is
// no reason to refuse the object.
func TestCallerObject_FieldLeftOutOfJSONIsNotWalked(t *testing.T) {
	self := map[string]any{}
	self["self"] = self
	o := &dashHolder{
		TypeMeta:   metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Thing"},
		Skipped:    make([]struct{}, 1<<22+1),
		SkippedMap: self,
	}
	o.Name, o.Namespace = "holder", "default"
	if err := umbrellaOutcome(t, o); err != nil {
		t.Errorf("a field tagged json:\"-\" was walked: %v", err)
	}
}

// A field named "-" by json:"-," is written, so the walk reads it: the control
// that the test above is a refusal removed, not an object never refused.
func TestCallerObject_FieldNamedDashIsWalked(t *testing.T) {
	o := &namedDash{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Thing"}, Named: make([]struct{}, 1<<22+1)}
	o.Name, o.Namespace = "holder", "default"
	err := umbrellaOutcome(t, o)
	if err == nil || !strings.Contains(err.Error(), "holds more than") {
		t.Errorf("got %v, want a refusal naming the size", err)
	}
}
