package kubernetes

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestCreatePodDisruptionBudget(t *testing.T) {
	pdb := CreatePodDisruptionBudget("my-pdb", "default")
	if pdb.Name != "my-pdb" || pdb.Namespace != "default" {
		t.Fatalf("metadata mismatch: %s/%s", pdb.Namespace, pdb.Name)
	}
	if pdb.Kind != "PodDisruptionBudget" {
		t.Errorf("unexpected kind %q", pdb.Kind)
	}
	if pdb.Labels["app"] != "my-pdb" {
		t.Errorf("expected label app=my-pdb, got %v", pdb.Labels)
	}
}

func TestPDBNilErrors(t *testing.T) {
	val := intstr.FromInt32(1)
	// All PDB functions now panic on nil receiver
	assertPanics(t, func() { SetPDBMinAvailable(nil, val) })
	assertPanics(t, func() { SetPDBMaxUnavailable(nil, val) })
	assertPanics(t, func() { SetPDBSelector(nil, &metav1.LabelSelector{}) })
	assertPanics(t, func() { SetPDBLabels(nil, map[string]string{}) })
	assertPanics(t, func() { SetPDBAnnotations(nil, map[string]string{}) })
}

func TestPDBMutualExclusivity(t *testing.T) {
	pdb := CreatePodDisruptionBudget("test", "default")

	minVal := intstr.FromInt32(2)
	SetPDBMinAvailable(pdb, minVal)
	if pdb.Spec.MinAvailable == nil {
		t.Fatal("expected MinAvailable to be set")
	}

	maxVal := intstr.FromInt32(1)
	SetPDBMaxUnavailable(pdb, maxVal)
	if pdb.Spec.MaxUnavailable == nil {
		t.Fatal("expected MaxUnavailable to be set")
	}
	if pdb.Spec.MinAvailable != nil {
		t.Error("expected MinAvailable to be cleared after setting MaxUnavailable")
	}

	// Set MinAvailable again — should clear MaxUnavailable
	SetPDBMinAvailable(pdb, minVal)
	if pdb.Spec.MaxUnavailable != nil {
		t.Error("expected MaxUnavailable to be cleared after setting MinAvailable")
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

func TestPDBLabelsAndAnnotations(t *testing.T) {
	pdb := CreatePodDisruptionBudget("test", "default")

	labels := map[string]string{"env": "prod"}
	SetPDBLabels(pdb, labels)
	if pdb.Labels["env"] != "prod" {
		t.Errorf("labels not set correctly")
	}

	annotations := map[string]string{"note": "test"}
	SetPDBAnnotations(pdb, annotations)
	if pdb.Annotations["note"] != "test" {
		t.Errorf("annotations not set correctly")
	}
}
