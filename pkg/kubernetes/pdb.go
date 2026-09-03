package kubernetes

import (
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// SetPDBMinAvailable sets MinAvailable and clears MaxUnavailable (mutually exclusive).
func SetPDBMinAvailable(pdb *policyv1.PodDisruptionBudget, val intstr.IntOrString) {
	if pdb == nil {
		panic("SetPDBMinAvailable: pdb must not be nil")
	}
	pdb.Spec.MinAvailable = &val
	pdb.Spec.MaxUnavailable = nil
}

// SetPDBMaxUnavailable sets MaxUnavailable and clears MinAvailable (mutually exclusive).
func SetPDBMaxUnavailable(pdb *policyv1.PodDisruptionBudget, val intstr.IntOrString) {
	if pdb == nil {
		panic("SetPDBMaxUnavailable: pdb must not be nil")
	}
	pdb.Spec.MaxUnavailable = &val
	pdb.Spec.MinAvailable = nil
}

// SetPDBSelector sets the label selector for the PDB.
func SetPDBSelector(pdb *policyv1.PodDisruptionBudget, selector *metav1.LabelSelector) {
	if pdb == nil {
		panic("SetPDBSelector: pdb must not be nil")
	}
	pdb.Spec.Selector = selector
}
