package fluxcd

import (
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestCheckReconcileOrder_ReadyExprTrue: the members' check alone, a
// depending on b with "true" (go-kure/kure#1161). With existenceOnly, a waits
// for b to exist, so for the apply of b's creator: refused when that is a,
// admitted when b's creator is outside the members even though b depends on
// a. Without it, a waits for b's Ready, and b depending on a is refused.
func TestCheckReconcileOrder_ReadyExprTrue(t *testing.T) {
	member := func(name string, deps ...kustv1.DependencyReference) *kustv1.Kustomization {
		return &kustv1.Kustomization{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "flux-system"},
			Spec:       kustv1.KustomizationSpec{DependsOn: deps},
		}
	}
	for name, c := range map[string]struct {
		bDependsOnA   bool
		aCreatesB     bool
		existenceOnly bool
		refused       string
	}{
		"gate off, a creates b":                {aCreatesB: true, existenceOnly: true, refused: "apply flux-system/a -> exist flux-system/b -> apply flux-system/a"},
		"gate off, b depends on a":             {bDependsOnA: true, existenceOnly: true},
		"gate unknown, b depends on a":         {bDependsOnA: true, refused: "apply flux-system/a -> ready flux-system/b -> apply flux-system/b -> ready flux-system/a"},
		"gate unknown, a creates b, no b -> a": {aCreatesB: true, refused: "apply flux-system/a -> ready flux-system/b -> apply flux-system/b -> apply flux-system/a"},
	} {
		t.Run(name, func(t *testing.T) {
			a := member("a", kustv1.DependencyReference{Name: "b", ReadyExpr: "true"})
			b := member("b")
			if c.bDependsOnA {
				b.Spec.DependsOn = []kustv1.DependencyReference{{Name: "a"}}
			}
			creator := func(key string) string {
				if c.aCreatesB && key == crKey("flux-system", "b") {
					return crKey("flux-system", "a")
				}
				return ""
			}
			err := checkReconcileOrder([]*kustv1.Kustomization{a, b}, creator, nil, c.existenceOnly)
			if c.refused == "" {
				if err != nil {
					t.Fatalf("got %v, want the members admitted", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.refused) {
				t.Fatalf("got %v, want the chain %s", err, c.refused)
			}
		})
	}
}
