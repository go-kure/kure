package kubernetes_test

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/kubernetes"
	"github.com/go-kure/kure/pkg/kubernetes/internal/kinds"
)

// TestIdentity_EveryRegisteredKindHasAWrapper walks the scheme and fails on any
// object kind that has no generated constructor, and on any registry entry
// that no longer matches a registered kind (a stale generated file).
func TestIdentity_EveryRegisteredKindHasAWrapper(t *testing.T) {
	registered, err := kinds.Registered()
	if err != nil {
		t.Fatal(err)
	}
	generated := map[schema.GroupVersionKind]generatedKind{}
	for _, g := range generatedKinds {
		generated[g.GVK] = g
	}
	seen := map[schema.GroupVersionKind]bool{}
	for _, k := range registered {
		g, ok := generated[k.GVK]
		if !ok {
			t.Errorf("%s is registered in the scheme but has no generated wrapper (run scripts/gen-builders.sh generate)", k.GVK)
			continue
		}
		if g.Namespaced != k.Namespaced {
			t.Errorf("%s: generated wrapper scope (namespaced=%v) disagrees with the kinds table (%v)", k.GVK, g.Namespaced, k.Namespaced)
		}
		seen[k.GVK] = true
	}
	for gvk := range generated {
		if !seen[gvk] {
			t.Errorf("generated wrapper for %s has no registered kind (stale generated file)", gvk)
		}
	}
	if len(registered) == 0 {
		t.Fatal("no registered kinds found")
	}
}

// TestIdentity_ConstructorsEmitIdentityOnly proves, for every registered kind,
// that the generated constructor sets exactly TypeMeta plus name (and
// namespace when namespaced) and nothing else: the result must DeepEqual a
// zero value of the type with only those fields written. Any injected label,
// annotation, selector or default turns this red.
func TestIdentity_ConstructorsEmitIdentityOnly(t *testing.T) {
	registered, err := kinds.Registered()
	if err != nil {
		t.Fatal(err)
	}
	byGVK := map[schema.GroupVersionKind]kinds.Kind{}
	for _, k := range registered {
		byGVK[k.GVK] = k
	}
	for _, g := range generatedKinds {
		k, ok := byGVK[g.GVK]
		if !ok {
			continue // reported by the coverage test above
		}
		t.Run(g.GVK.String(), func(t *testing.T) {
			got := g.Create("identity", "ns")

			want, ok := reflect.New(k.Type).Interface().(client.Object)
			if !ok {
				t.Fatalf("%s does not implement client.Object", k.Type)
			}
			want.GetObjectKind().SetGroupVersionKind(k.GVK)
			want.SetName("identity")
			if k.Namespaced {
				want.SetNamespace("ns")
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("constructor emitted more than identity:\n got %#v\nwant %#v", got, want)
			}

			gvks, _, err := kubernetes.Scheme.ObjectKinds(got)
			if err != nil {
				t.Fatalf("ObjectKinds: %v", err)
			}
			if len(gvks) != 1 || gvks[0] != k.GVK {
				t.Errorf("ObjectKinds = %v, want exactly [%s]", gvks, k.GVK)
			}
		})
	}
}
