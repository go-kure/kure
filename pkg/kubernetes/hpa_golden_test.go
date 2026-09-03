package kubernetes_test

import (
	"testing"

	. "github.com/go-kure/kure/pkg/kubernetes"
)

// SetHPAScaleTargetRef is the one class (c) composite the contract keeps at this
// revision; the contract's purity rule (README §4) requires every composite to
// carry a golden of its complete output, so an injected value shows up in the
// diff that adds it.
func TestSetHPAScaleTargetRef_Golden(t *testing.T) {
	hpa := CreateHorizontalPodAutoscaler("web", "default")
	SetHPAScaleTargetRef(hpa, "apps/v1", "Deployment", "web")
	goldenTest(t, "hpa-scale-target-ref.yaml", hpa)
}
