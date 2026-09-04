package kubernetes

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestPDBNilErrors(t *testing.T) {
	val := intstr.FromInt32(1)
	// All PDB functions now panic on nil receiver
	assertPanics(t, func() { SetPDBMinAvailable(nil, val) })
	assertPanics(t, func() { SetPDBMaxUnavailable(nil, val) })
	assertPanics(t, func() { SetPDBSelector(nil, &metav1.LabelSelector{}) })
}

// TestPDBAvailabilityTouchesOnlyItsOwnField pins the contract behaviour: each
// setter writes the field it names and leaves the mutually exclusive sibling
// alone. Enforcing the exclusion is upstream's job, not the builder's.
func TestPDBAvailabilityTouchesOnlyItsOwnField(t *testing.T) {
	pdb := CreatePodDisruptionBudget("test", "default")

	minVal := intstr.FromInt32(2)
	SetPDBMinAvailable(pdb, minVal)
	if pdb.Spec.MinAvailable == nil {
		t.Fatal("expected MinAvailable to be set")
	}
	if pdb.Spec.MaxUnavailable != nil {
		t.Error("SetPDBMinAvailable must not write MaxUnavailable")
	}

	maxVal := intstr.FromInt32(1)
	SetPDBMaxUnavailable(pdb, maxVal)
	if pdb.Spec.MaxUnavailable == nil {
		t.Fatal("expected MaxUnavailable to be set")
	}
	if pdb.Spec.MinAvailable == nil {
		t.Error("SetPDBMaxUnavailable must not clear MinAvailable")
	}

	// Switching back is the caller's explicit two-step.
	pdb.Spec.MaxUnavailable = nil
	SetPDBMinAvailable(pdb, minVal)
	if pdb.Spec.MaxUnavailable != nil {
		t.Error("expected MaxUnavailable to stay cleared")
	}
	if pdb.Spec.MinAvailable == nil || pdb.Spec.MinAvailable.IntValue() != 2 {
		t.Errorf("expected MinAvailable 2, got %v", pdb.Spec.MinAvailable)
	}
}

func TestPDBSelector(t *testing.T) {
	pdb := CreatePodDisruptionBudget("test", "default")
	selector := &metav1.LabelSelector{
		MatchLabels: map[string]string{"app": "web"},
	}
	SetPDBSelector(pdb, selector)
	if pdb.Spec.Selector == nil || pdb.Spec.Selector.MatchLabels["app"] != "web" {
		t.Errorf("selector not set correctly")
	}
}
