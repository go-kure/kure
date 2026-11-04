package fluxcd

import (
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

// origin/main reads and writes an object nested up to the depth encoding/json
// takes (10,000 nested maps) and refuses a deeper one with an error. This
// change refuses an object only where main does, so a nesting main handles is
// kept, and converted where it needs it.
func TestNormalizeCallerObjects_NestingMainHandlesIsNotRefused(t *testing.T) {
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
