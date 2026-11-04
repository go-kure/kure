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
	got := allocatedBy(func() { _, _, err = walkHeld(head, &convBudget{}) })
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
	const tail = " holds a chan int, which kure does not convert to JSON"
	// Each case is compared whole: a key that always took a leading dot, or a
	// path that lost its root, would pass a test that looks for a part.
	for name, tc := range map[string]struct {
		m    map[string]any
		want string
	}{
		"nested keys and a slice":     {map[string]any{"a": map[string]any{"b": []any{map[string]any{"c": make(chan int)}}}}, "a.b[0].c" + tail},
		"a key of the root":           {map[string]any{"c": make(chan int)}, "c" + tail},
		"a slice below a key":         {map[string]any{"a": []any{make(chan int)}}, "a[0]" + tail},
		"an empty key of the root":    {map[string]any{"": make(chan int)}, "the object" + tail},
		"a key below an empty key":    {map[string]any{"": map[string]any{"x": make(chan int)}}, "x" + tail},
		"an empty key below a key":    {map[string]any{"a": map[string]any{"": make(chan int)}}, "a." + tail},
		"an empty key below an index": {map[string]any{"a": []any{map[string]any{"": make(chan int)}}}, "a[0]." + tail},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := jsonContentOf(tc.m, &convBudget{})
			if err == nil || err.Error() != tc.want {
				t.Errorf("got %v, want %q", err, tc.want)
			}
		})
	}
	t.Run("the root, named as the object", func(t *testing.T) {
		defer SetMaxJSONNodesForTest(2)()
		_, err := jsonContentOf(map[string]any{"a": 1, "b": 2, "c": 3}, &convBudget{})
		if want := "the object holds more than 2 values"; err == nil || err.Error() != want {
			t.Errorf("got %v, want %q", err, want)
		}
	})
	t.Run("typed walk", func(t *testing.T) {
		n := &pinNode{}
		n.Items = []*pinNode{n}
		_, _, err := walkHeld(n, &convBudget{})
		want := "the typed object.Items[0] holds itself"
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("got %v, want one containing %q", err, want)
		}
	})
}
