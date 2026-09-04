package kubernetes

import (
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// SetPDBMinAvailable sets MinAvailable. MinAvailable and MaxUnavailable are
// mutually exclusive upstream; this helper writes only the field it names, so
// a caller that sets both produces a PodDisruptionBudget the API server
// rejects. Clear the other field explicitly when switching:
//
//	pdb.Spec.MaxUnavailable = nil
//	kubernetes.SetPDBMinAvailable(pdb, intstr.FromInt32(2))
func SetPDBMinAvailable(pdb *policyv1.PodDisruptionBudget, val intstr.IntOrString) {
	if pdb == nil {
		panic("SetPDBMinAvailable: pdb must not be nil")
	}
	pdb.Spec.MinAvailable = &val
}

// SetPDBMaxUnavailable sets MaxUnavailable. See SetPDBMinAvailable for the
// mutual exclusion between the two fields.
func SetPDBMaxUnavailable(pdb *policyv1.PodDisruptionBudget, val intstr.IntOrString) {
	if pdb == nil {
		panic("SetPDBMaxUnavailable: pdb must not be nil")
	}
	pdb.Spec.MaxUnavailable = &val
}

// SetPDBSelector sets the label selector for the PDB.
func SetPDBSelector(pdb *policyv1.PodDisruptionBudget, selector *metav1.LabelSelector) {
	if pdb == nil {
		panic("SetPDBSelector: pdb must not be nil")
	}
	pdb.Spec.Selector = selector
}
