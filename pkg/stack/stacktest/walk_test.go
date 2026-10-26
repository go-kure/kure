package stacktest

import (
	"math/rand"
	"reflect"
	"testing"
)

// earliestByPairs is what internal did before earliest: every pair of the
// walk with i < j, then the first pair of each j in (i, j) order.
func earliestByPairs(rs []record) []match {
	var out []match
	seen := map[int]bool{}
	for _, m := range matches(rs, rs) {
		if m.i < m.j && !seen[m.j] {
			seen[m.j] = true
			out = append(out, m)
		}
	}
	return out
}

func TestEarliest_SameAsAllPairs(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for range 500 {
		size := rng.Intn(40)
		rs := make([]record, size)
		for k := range rs {
			start := uintptr(rng.Intn(64))
			if rng.Intn(4) == 0 {
				rs[k] = record{start: start, isMap: true}
				continue
			}
			// Zero-length ranges included: they overlap nothing.
			rs[k] = record{start: start, end: start + uintptr(rng.Intn(16))}
		}
		if got, want := earliest(rs), earliestByPairs(rs); !reflect.DeepEqual(got, want) {
			t.Fatalf("records %+v:\n got %v\nwant %v", rs, got, want)
		}
	}
}
