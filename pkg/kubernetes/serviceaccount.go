package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
)

func AddServiceAccountSecret(sa *corev1.ServiceAccount, secret corev1.ObjectReference) {
	if sa == nil {
		panic("AddServiceAccountSecret: sa must not be nil")
	}
	sa.Secrets = append(sa.Secrets, secret)
}

func AddServiceAccountImagePullSecret(sa *corev1.ServiceAccount, secret corev1.LocalObjectReference) {
	if sa == nil {
		panic("AddServiceAccountImagePullSecret: sa must not be nil")
	}
	sa.ImagePullSecrets = append(sa.ImagePullSecrets, secret)
}

func SetServiceAccountAutomountToken(sa *corev1.ServiceAccount, automount bool) {
	if sa == nil {
		panic("SetServiceAccountAutomountToken: sa must not be nil")
	}
	if sa.AutomountServiceAccountToken == nil {
		sa.AutomountServiceAccountToken = new(bool)
	}
	*sa.AutomountServiceAccountToken = automount
}

// Labels and annotations use the generic AddLabel / AddAnnotation over
// metav1.Object (metadata.go); there is no per-kind ServiceAccount helper.
