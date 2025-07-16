package metallb

import (
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
func SetBFDProfileDetectMultiplier(obj *metallbv1beta1.BFDProfile, mult uint32) {
	obj.Spec.DetectMultiplier = &mult
}

// SetBFDProfileEchoInterval sets the echo interval on the BFDProfile spec.
func SetBFDProfileEchoInterval(obj *metallbv1beta1.BFDProfile, interval uint32) {
	obj.Spec.EchoInterval = &interval
}

// SetBFDProfileEchoMode sets the echo mode on the BFDProfile spec.
func SetBFDProfileEchoMode(obj *metallbv1beta1.BFDProfile, mode bool) {
	obj.Spec.EchoMode = &mode
}

// SetBFDProfilePassiveMode sets the passive mode on the BFDProfile spec.
func SetBFDProfilePassiveMode(obj *metallbv1beta1.BFDProfile, mode bool) {
	obj.Spec.PassiveMode = &mode
}
