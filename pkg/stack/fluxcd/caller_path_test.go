package fluxcd

import (
	goruntime "runtime"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// allocatedBy returns the bytes f allocates in all. No test here is
// t.Parallel, so nothing else of the package runs meanwhile.
func allocatedBy(f func()) uint64 {
	var before, after goruntime.MemStats
	goruntime.ReadMemStats(&before)
	f()
	goruntime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// The path of a refusal is made when there is one to report, not at every level
// on the way down: copying the whole path at each level costs the square of the
// depth. The bound is on bytes allocated, not time, so it holds on any host. The
// copying code allocates about 128 MiB here, and this one under 2 MiB.
func TestJSONContentOf_PathIsNotCopiedAtEveryLevel(t *testing.T) {
	const (
		levels = 1000
		bound  = 16 << 20
	)
	key := strings.Repeat("k", 256)
	m := map[string]any{key: 53} // an int is not JSON typed: the converter walks it.
	for range levels {
		m = map[string]any{key: m}
	}
	var err error
	got := allocatedBy(func() { _, err = jsonContentOf(m, &convBudget{}) })
	if err != nil {
		t.Fatalf("got %v, want the nesting accepted", err)
	}
	if got > bound {
		t.Errorf("converting %d levels with %d byte keys allocated %d bytes, want under %d", levels, len(key), got, bound)
	}
}

// longLink holds a chain of its own kind under a field whose name is long.
type longLink struct {
	metav1.TypeMeta
	*metav1.ObjectMeta
	AFieldWhoseNameIsLongEnoughThatCopyingItsPathAtEveryLevelCostsTheSquareOfTheDepth *longLink
}

func (l *longLink) DeepCopyObject() runtime.Object { c := *l; return &c }

func TestHeldWalk_PathIsNotCopiedAtEveryLevel(t *testing.T) {
	const (
		links = 2000
		bound = 32 << 20
	)
	head := &longLink{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Link"}}
	for tail, i := head, 0; i < links; i++ {
		tail.AFieldWhoseNameIsLongEnoughThatCopyingItsPathAtEveryLevelCostsTheSquareOfTheDepth = &longLink{TypeMeta: head.TypeMeta}
		tail = tail.AFieldWhoseNameIsLongEnoughThatCopyingItsPathAtEveryLevelCostsTheSquareOfTheDepth
	}
	var err error
	got := allocatedBy(func() { _, err = walkHeld(head, &convBudget{}) })
	if err != nil {
		t.Fatalf("got %v, want the chain accepted", err)
	}
	if got > bound {
		t.Errorf("walking %d links allocated %d bytes, want under %d", links, got, bound)
	}
}

// pinNode holds itself through a slice.
type pinNode struct {
	metav1.TypeMeta
	*metav1.ObjectMeta
	Items []*pinNode
}

func (n *pinNode) DeepCopyObject() runtime.Object { c := *n; return &c }

// The text of a refusal at depth is the path from the root, as it was when each
// level copied it.
func TestPaths_RefusalsNameTheExactPath(t *testing.T) {
	t.Run("converted value", func(t *testing.T) {
		m := map[string]any{"a": map[string]any{"b": []any{map[string]any{"c": make(chan int)}}}}
		_, err := jsonContentOf(m, &convBudget{})
		want := "a.b[0].c holds a chan int, which kure does not convert to JSON"
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("got %v, want one containing %q", err, want)
		}
	})
	t.Run("the object itself", func(t *testing.T) {
		_, err := jsonContentOf(map[string]any{"c": make(chan int)}, &convBudget{})
		want := "c holds a chan int, which kure does not convert to JSON"
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("got %v, want one containing %q", err, want)
		}
	})
	t.Run("typed walk", func(t *testing.T) {
		n := &pinNode{}
		n.Items = []*pinNode{n}
		_, err := walkHeld(n, &convBudget{})
		want := "the typed object.Items[0] holds itself"
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("got %v, want one containing %q", err, want)
		}
	})
}
