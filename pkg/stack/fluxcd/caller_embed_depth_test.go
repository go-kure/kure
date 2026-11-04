package fluxcd

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack/layout"
)

// A typed resource that embeds a ConfigMap through a chain of structs holds no
// unstructured object, so it is no wrapper and is kept, however deep the chain
// is: origin/main accepts it. The search for an embedded Unstructured follows
// the type graph, which is finite, so it needs no depth limit.
type (
	embedD0  struct{ corev1.ConfigMap }
	embedD1  struct{ embedD0 }
	embedD2  struct{ embedD1 }
	embedD3  struct{ embedD2 }
	embedD4  struct{ embedD3 }
	embedD5  struct{ embedD4 }
	embedD6  struct{ embedD5 }
	embedD7  struct{ embedD6 }
	embedD8  struct{ embedD7 }
	embedD9  struct{ embedD8 }
	embedD10 struct{ embedD9 }
	embedD11 struct{ embedD10 }
	embedD12 struct{ embedD11 }
)

// The ladder embeds, at each level, both types of the level below: 2^levels
// paths to the bottom through a few types. A search that keeps one step per
// path grows without end on it.
type (
	ladderBottom struct{ N int }
	ladderA6     struct{ ladderBottom }
	ladderB6     struct{ ladderBottom }
	ladderA5     struct {
		ladderA6
		ladderB6
	}
	ladderB5 struct {
		ladderA6
		ladderB6
	}
	ladderA4 struct {
		ladderA5
		ladderB5
	}
	ladderB4 struct {
		ladderA5
		ladderB5
	}
	ladderA3 struct {
		ladderA4
		ladderB4
	}
	ladderB3 struct {
		ladderA4
		ladderB4
	}
	ladderA2 struct {
		ladderA3
		ladderB3
	}
	ladderB2 struct {
		ladderA3
		ladderB3
	}
	ladderTop struct {
		ladderA2
		ladderB2
	}
)

func TestNormalizeCallerObjects_DeepTypedEmbedIsNotRefused(t *testing.T) {
	deep := &embedD12{}
	deep.APIVersion, deep.Kind, deep.Name, deep.Namespace = "v1", "ConfigMap", "cm", "default"
	resources := map[string]client.Object{"a chain of 13 embedded structs": deep}
	for name, r := range resources {
		l := &layout.ManifestLayout{Name: "l", Resources: []client.Object{r}}
		if err := normalizeForTest(l); err != nil {
			t.Errorf("%s: refused: %v", name, err)
		}
		if l.Resources[0] != r {
			t.Errorf("%s: the layout holds a %T, want the caller's resource", name, l.Resources[0])
		}
		objs, _, err := embeddedUnstructured(r)
		if err != nil || len(objs) != 0 {
			t.Errorf("%s: got %d objects and %v, want none and no error", name, len(objs), err)
		}
	}
}

// The search ends at once on the ladder, which holds no unstructured object, and
// refuses nothing.
func TestShallowestEmbeds_LadderOfEmbedsEndsAtOnce(t *testing.T) {
	found, _ := shallowestEmbeds(reflect.TypeFor[ladderTop]())
	if len(found) != 0 {
		t.Errorf("found %d unstructured objects, want none", len(found))
	}
}
