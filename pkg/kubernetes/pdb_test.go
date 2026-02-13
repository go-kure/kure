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
	if err := SetPDBMinAvailable(nil, val); err == nil {
		t.Error("expected error for nil PDB on SetPDBMinAvailable")
	}
	if err := SetPDBMaxUnavailable(nil, val); err == nil {
		t.Error("expected error for nil PDB on SetPDBMaxUnavailable")
	}
	if err := SetPDBSelector(nil, &metav1.LabelSelector{}); err == nil {
		t.Error("expected error for nil PDB on SetPDBSelector")
	}
	if err := SetPDBLabels(nil, map[string]string{}); err == nil {
		t.Error("expected error for nil PDB on SetPDBLabels")
	}
	if err := SetPDBAnnotations(nil, map[string]string{}); err == nil {
		t.Error("expected error for nil PDB on SetPDBAnnotations")
	}
}

func TestPDBMutualExclusivity(t *testing.T) {
	pdb := CreatePodDisruptionBudget("test", "default")

	minVal := intstr.FromInt32(2)
	if err := SetPDBMinAvailable(pdb, minVal); err != nil {
		t.Fatalf("SetPDBMinAvailable: %v", err)
	}
	if pdb.Spec.MinAvailable == nil {
		t.Fatal("expected MinAvailable to be set")
	}

	maxVal := intstr.FromInt32(1)
	if err := SetPDBMaxUnavailable(pdb, maxVal); err != nil {
		t.Fatalf("SetPDBMaxUnavailable: %v", err)
	}
	if pdb.Spec.MaxUnavailable == nil {
		t.Fatal("expected MaxUnavailable to be set")
	}
	if pdb.Spec.MinAvailable != nil {
		t.Error("expected MinAvailable to be cleared after setting MaxUnavailable")
	}

	// Set MinAvailable again — should clear MaxUnavailable
	if err := SetPDBMinAvailable(pdb, minVal); err != nil {
		t.Fatalf("SetPDBMinAvailable: %v", err)
	}
	if pdb.Spec.MaxUnavailable != nil {
		t.Error("expected MaxUnavailable to be cleared after setting MinAvailable")
	}
}

func TestPDBSelector(t *testing.T) {
	pdb := CreatePodDisruptionBudget("test", "default")
	selector := &metav1.LabelSelector{
		MatchLabels: map[string]string{"app": "web"},
	}
	if err := SetPDBSelector(pdb, selector); err != nil {
		t.Fatalf("SetPDBSelector: %v", err)
	}
	if pdb.Spec.Selector == nil || pdb.Spec.Selector.MatchLabels["app"] != "web" {
		t.Errorf("selector not set correctly")
	}
}

func TestPDBLabelsAndAnnotations(t *testing.T) {
	pdb := CreatePodDisruptionBudget("test", "default")

	labels := map[string]string{"env": "prod"}
	if err := SetPDBLabels(pdb, labels); err != nil {
		t.Fatalf("SetPDBLabels: %v", err)
	}
	if pdb.Labels["env"] != "prod" {
		t.Errorf("labels not set correctly")
	}

	annotations := map[string]string{"note": "test"}
	if err := SetPDBAnnotations(pdb, annotations); err != nil {
		t.Fatalf("SetPDBAnnotations: %v", err)
	}
	if pdb.Annotations["note"] != "test" {
		t.Errorf("annotations not set correctly")
	}
}
