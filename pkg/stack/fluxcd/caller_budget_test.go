package fluxcd

import (
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack/layout"
)

// sharedMap returns a map of 2^levels values that is a few allocations: each
// level holds the one below twice. It holds an int, so it is not plain JSON and
// needs the converter, which expands it once per occurrence.
func sharedMap(levels int) map[string]any {
	m := map[string]any{"n": 1}
	for i := 0; i < levels; i++ {
		m = map[string]any{"a": m, "b": m}
	}
	return m
}

func cmWith(name string, spec any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]any{"name": name, "namespace": "default"},
		"spec":     spec,
	}}
}

// typedList is a typed List that holds unstructured items.
type typedList struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             []runtime.Object `json:"items"`
}

func (l *typedList) DeepCopyObject() runtime.Object { c := *l; return &c }

// normalizeWithin fails the test if the call does not end soon: a bound that
// holds ends it in milliseconds, one that does not walks for minutes.
func normalizeWithin(t *testing.T, resources ...client.Object) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- normalizeForTest(&layout.ManifestLayout{Name: "l", Resources: resources}) }()
	select {
	case err := <-done:
		return err
	case <-time.After(20 * time.Second):
		t.Fatal("the call did not end")
		return nil
	}
}

// The conversion and reachability work is bounded across the whole call, not
// per object: each of these objects is under the bound alone, and together
// they are far over it. No test here is t.Parallel: each lowers the bound.
func TestNormalizeCallerObjects_ConversionBudgetIsPerCall(t *testing.T) {
	defer SetMaxJSONNodesForTest(1 << 12)()
	const items = 64
	shared := sharedMap(8) // 2^9 values: a fraction of the bound for one object.

	t.Run("separate resources that share one map", func(t *testing.T) {
		var rs []client.Object
		for i := 0; i < items; i++ {
			rs = append(rs, cmWith("cm", shared))
		}
		err := normalizeWithin(t, rs...)
		if err == nil || !strings.Contains(err.Error(), "more than") {
			t.Errorf("got %v, want a refusal naming the value limit", err)
		}
	})

	t.Run("the items of one typed list that share one map", func(t *testing.T) {
		l := &typedList{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}}
		l.Name, l.Namespace = "holder", "default"
		for i := 0; i < items; i++ {
			l.Items = append(l.Items, cmWith("cm", shared))
		}
		err := normalizeWithin(t, l)
		if err == nil || !strings.Contains(err.Error(), "more than") {
			t.Errorf("got %v, want a refusal naming the value limit", err)
		}
	})

	t.Run("separate resources that share one byte slice", func(t *testing.T) {
		raw := make([]byte, 1<<11) // half the bound, and base64 text for each resource.
		var rs []client.Object
		for i := 0; i < 8; i++ {
			rs = append(rs, cmWith("cm", map[string]any{"raw": raw}))
		}
		err := normalizeWithin(t, rs...)
		if err == nil || !strings.Contains(err.Error(), "more than") {
			t.Errorf("got %v, want a refusal naming the value limit", err)
		}
	})

	t.Run("one resource alone, under the bound, is accepted", func(t *testing.T) {
		if err := normalizeWithin(t, cmWith("cm", shared)); err != nil {
			t.Errorf("got %v, want it accepted", err)
		}
	})
}

// A map with more entries than the budget has left is refused before its keys
// are collected and sorted, as a slice is refused before it is copied: the
// work is not in proportion to a map that must be refused. Collecting the keys
// of a map of strings allocates once for each.
func TestJSONContentOf_MapOverBudgetIsRefusedBeforeItsKeysAreCollected(t *testing.T) {
	defer SetMaxJSONNodesForTest(1 << 12)()
	big := make(map[string]int, 1<<14)
	for i := 0; i < 1<<14; i++ {
		big[strings.Repeat("k", 1+i%7)+string(rune('a'+i%26))+strings.Repeat("x", i/26)] = i
	}
	m := map[string]any{"big": big}
	var err error
	allocs := testing.AllocsPerRun(3, func() { _, err = jsonContentOf(m, &convBudget{}) })
	if err == nil || !strings.Contains(err.Error(), "holds more than") {
		t.Fatalf("got %v, want a refusal for a map over the bound", err)
	}
	if allocs > 256 {
		t.Errorf("the refusal made %.0f allocations, want it before the keys are collected", allocs)
	}
}

// A large layout of plain JSON objects holds nothing that needs the converter,
// so it is charged nothing: origin/main builds it, and so does this.
func TestNormalizeCallerObjects_LargePlainJSONLayoutIsNotRefused(t *testing.T) {
	defer SetMaxJSONNodesForTest(1 << 12)()
	var rs []client.Object
	for i := 0; i < 500; i++ {
		spec := map[string]any{}
		for j := 0; j < 100; j++ {
			spec[string(rune('a'+j%26))+strings.Repeat("x", j/26)] = []any{"v", int64(j), map[string]any{"k": true}}
		}
		rs = append(rs, cmWith("cm", spec))
	}
	if err := normalizeWithin(t, rs...); err != nil {
		t.Errorf("got %v, want a large plain JSON layout accepted", err)
	}
}
