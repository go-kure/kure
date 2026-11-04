package fluxcd_test

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
)

// A struct that promotes a trusted marshaler through a chain of embedded
// structs, however long, promotes nothing that is not trusted: the check
// follows the type graph, which is finite, so it needs no depth limit.
type (
	promoteC0  struct{ *unstructured.Unstructured }
	promoteC1  struct{ promoteC0 }
	promoteC2  struct{ promoteC1 }
	promoteC3  struct{ promoteC2 }
	promoteC4  struct{ promoteC3 }
	promoteC5  struct{ promoteC4 }
	promoteC6  struct{ promoteC5 }
	promoteC7  struct{ promoteC6 }
	promoteC8  struct{ promoteC7 }
	promoteC9  struct{ promoteC8 }
	promoteC10 struct{ promoteC9 }
	promoteC11 struct{ promoteC10 }
	promoteC12 struct{ promoteC11 }
	promoteC13 struct{ promoteC12 }
	promoteC14 struct{ promoteC13 }
	promoteC15 struct{ promoteC14 }
	promoteC16 struct{ promoteC15 }
	promoteC17 struct{ promoteC16 }
	promoteC18 struct{ promoteC17 }
	promoteC19 struct{ promoteC18 }
	promoteC20 struct{ promoteC19 }
)

// Each level of the ladder holds the level below it twice, through two types,
// so the paths to the bottom double at every level while the types do not.
type (
	ladderP0 struct{ *unstructured.Unstructured }
	ladderQ0 struct{ ladderP0 }
	ladderR0 struct{ ladderP0 }
	ladderP1 struct {
		*unstructured.Unstructured
		ladderQ0
		ladderR0
	}
	ladderQ1 struct{ ladderP1 }
	ladderR1 struct{ ladderP1 }
	ladderP2 struct {
		*unstructured.Unstructured
		ladderQ1
		ladderR1
	}
	ladderQ2 struct{ ladderP2 }
	ladderR2 struct{ ladderP2 }
	ladderP3 struct {
		*unstructured.Unstructured
		ladderQ2
		ladderR2
	}
	ladderQ3 struct{ ladderP3 }
	ladderR3 struct{ ladderP3 }
	ladderP4 struct {
		*unstructured.Unstructured
		ladderQ3
		ladderR3
	}
	ladderQ4 struct{ ladderP4 }
	ladderR4 struct{ ladderP4 }
	ladderP5 struct {
		*unstructured.Unstructured
		ladderQ4
		ladderR4
	}
	ladderQ5 struct{ ladderP5 }
	ladderR5 struct{ ladderP5 }
	ladderP6 struct {
		*unstructured.Unstructured
		ladderQ5
		ladderR5
	}
	ladderQ6 struct{ ladderP6 }
	ladderR6 struct{ ladderP6 }
	ladderP7 struct {
		*unstructured.Unstructured
		ladderQ6
		ladderR6
	}
	ladderQ7 struct{ ladderP7 }
	ladderR7 struct{ ladderP7 }
	ladderP8 struct {
		*unstructured.Unstructured
		ladderQ7
		ladderR7
	}
	ladderQ8 struct{ ladderP8 }
	ladderR8 struct{ ladderP8 }
	ladderP9 struct {
		*unstructured.Unstructured
		ladderQ8
		ladderR8
	}
)

func TestPromotedFromTrusted_HasNoDepthLimit(t *testing.T) {
	for name, typ := range map[string]reflect.Type{
		"a chain of 21 embedded structs": reflect.TypeFor[promoteC20](),
		"a ladder of 10 levels":          reflect.TypeFor[ladderP9](),
	} {
		if !fluxstack.PromotedFromTrusted(typ) {
			t.Errorf("%s: not trusted, but it only promotes a trusted marshaler", name)
		}
	}
}
