package fluxcd

import (
	"encoding/json"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

// The limit is counted in levels, the root map being the first: a value nested
// exactly maxJSONDepth levels deep is kept, and one level more is refused, as
// encoding/json takes the one and refuses the other. The checks are the same for
// a value kept as it is (jsonTyped) and one that is converted.
func TestNestingLimit_IsExactlyWhatEncodingJSONTakes(t *testing.T) {
	// nested returns a root map of the given number of levels, maps or slices
	// below it, with the leaf at the bottom.
	nested := func(levels int, useSlices bool, leaf any) map[string]any {
		v := leaf
		for range levels - 1 {
			if useSlices {
				v = []any{v}
			} else {
				v = map[string]any{"n": v}
			}
		}
		return map[string]any{"n": v}
	}
	for _, useSlices := range []bool{false, true} {
		// A byte slice is written as a string: it adds no level.
		for _, leaf := range []any{"x", 53, []byte("x")} {
			for _, levels := range []int{maxJSONDepth, maxJSONDepth + 1} {
				m := nested(levels, useSlices, leaf)
				_, marshalErr := json.Marshal(m)
				want := levels <= maxJSONDepth
				if (marshalErr == nil) != want {
					t.Fatalf("encoding/json: %d levels, error %v: the limit moved", levels, marshalErr)
				}
				var got bool
				if _, typed := leaf.(string); typed {
					got = jsonTyped(m, 0)
				} else {
					_, err := jsonContentOf(m, &convBudget{})
					got = err == nil
				}
				if got != want {
					t.Errorf("slices=%v leaf=%v levels=%d: accepted = %v, want %v", useSlices, leaf, levels, got, want)
				}
			}
		}
	}
}

// origin/main reads and writes an object nested up to the depth encoding/json
// takes (10,000 nested maps) and refuses a deeper one with an error. This
// change refuses an object only where main does, so a nesting main handles is
// kept, and converted where it needs it.
func TestNormalizeCallerObjects_NestingMainHandlesIsNotRefused(t *testing.T) {
	t.Parallel()
	nest := func(levels int, leaf any) map[string]any {
		m := map[string]any{"leaf": leaf}
		for range levels {
			m = map[string]any{"n": m}
		}
		return m
	}
	const levels = 9000

	t.Run("JSON typed, kept as it is", func(t *testing.T) {
		u := cmWith("cm", nest(levels, "x"))
		held := u.Object
		if err := normalizeWithin(t, u); err != nil {
			t.Fatalf("refused: %v", err)
		}
		if !sameMap(u.Object, held) {
			t.Error("a JSON typed object was replaced")
		}
		if _, err := yaml.Marshal(u.Object); err != nil {
			t.Errorf("main cannot write it either: %v", err)
		}
	})

	t.Run("not JSON typed, converted", func(t *testing.T) {
		u := cmWith("cm", nest(levels, 53))
		if err := normalizeWithin(t, u); err != nil {
			t.Fatalf("refused: %v", err)
		}
		if !jsonTyped(u.Object, 0) {
			t.Error("the object still holds a value that is not JSON typed")
		}
	})

	t.Run("deeper than main takes, refused", func(t *testing.T) {
		u := cmWith("cm", nest(maxJSONDepth+1, "x"))
		err := normalizeWithin(t, u)
		if err == nil || !strings.Contains(err.Error(), "nested deeper than") {
			t.Errorf("got %v, want a refusal for the nesting", err)
		}
	})
}
