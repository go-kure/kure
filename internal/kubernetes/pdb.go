package kubernetes

import (
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/go-kure/kure/internal/validation"
)

// CreatePodDisruptionBudget creates a new PodDisruptionBudget with the given name and namespace.
func CreatePodDisruptionBudget(name, namespace string) *policyv1.PodDisruptionBudget {
	return &policyv1.PodDisruptionBudget{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PodDisruptionBudget",
			APIVersion: policyv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": name,
			},
			Annotations: map[string]string{
				"app": name,
			},
		},
	}
}

// SetPDBMinAvailable sets MinAvailable and clears MaxUnavailable (mutually exclusive).
func SetPDBMinAvailable(pdb *policyv1.PodDisruptionBudget, val intstr.IntOrString) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodDisruptionBudget(pdb); err != nil {
		return err
	}
	pdb.Spec.MinAvailable = &val
	pdb.Spec.MaxUnavailable = nil
	return nil
}

// SetPDBMaxUnavailable sets MaxUnavailable and clears MinAvailable (mutually exclusive).
func SetPDBMaxUnavailable(pdb *policyv1.PodDisruptionBudget, val intstr.IntOrString) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodDisruptionBudget(pdb); err != nil {
		return err
	}
	pdb.Spec.MaxUnavailable = &val
	pdb.Spec.MinAvailable = nil
	return nil
}

// SetPDBSelector sets the label selector for the PDB.
func SetPDBSelector(pdb *policyv1.PodDisruptionBudget, selector *metav1.LabelSelector) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodDisruptionBudget(pdb); err != nil {
		return err
	}
	pdb.Spec.Selector = selector
	return nil
}

// SetPDBLabels sets the labels on the PDB.
func SetPDBLabels(pdb *policyv1.PodDisruptionBudget, labels map[string]string) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodDisruptionBudget(pdb); err != nil {
		return err
	}
	pdb.Labels = labels
	return nil
}

// SetPDBAnnotations sets the annotations on the PDB.
func SetPDBAnnotations(pdb *policyv1.PodDisruptionBudget, annotations map[string]string) error {
	validator := validation.NewValidator()
	if err := validator.ValidatePodDisruptionBudget(pdb); err != nil {
		return err
	}
	pdb.Annotations = annotations
	return nil
}
