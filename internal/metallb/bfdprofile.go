package metallb

import (
	"errors"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateBFDProfile returns a new BFDProfile object with the provided name, namespace and spec.
func CreateBFDProfile(name, namespace string, spec metallbv1beta1.BFDProfileSpec) *metallbv1beta1.BFDProfile {
	obj := &metallbv1beta1.BFDProfile{
		TypeMeta: metav1.TypeMeta{
			Kind:       "BFDProfile",
			APIVersion: metallbv1beta1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}

// SetBFDProfileDetectMultiplier sets the detect multiplier on the BFDProfile spec.
func SetBFDProfileDetectMultiplier(obj *metallbv1beta1.BFDProfile, mult uint32) error {
	if obj == nil {
		return errors.New("nil BFDProfile")
	}
	obj.Spec.DetectMultiplier = &mult
	return nil
}

// SetBFDProfileEchoInterval sets the echo interval on the BFDProfile spec.
func SetBFDProfileEchoInterval(obj *metallbv1beta1.BFDProfile, interval uint32) error {
	if obj == nil {
		return errors.New("nil BFDProfile")
	}
	obj.Spec.EchoInterval = &interval
	return nil
}

// SetBFDProfileEchoMode sets the echo mode on the BFDProfile spec.
func SetBFDProfileEchoMode(obj *metallbv1beta1.BFDProfile, mode bool) error {
	if obj == nil {
		return errors.New("nil BFDProfile")
	}
	obj.Spec.EchoMode = &mode
	return nil
}

// SetBFDProfilePassiveMode sets the passive mode on the BFDProfile spec.
func SetBFDProfilePassiveMode(obj *metallbv1beta1.BFDProfile, mode bool) error {
	if obj == nil {
		return errors.New("nil BFDProfile")
	}
	obj.Spec.PassiveMode = &mode
	return nil
}
