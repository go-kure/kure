package k8s

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateServiceAccount(name, namespace string) *corev1.ServiceAccount {
	obj := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: corev1.SchemeGroupVersion.String(),
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
		Secrets:                      []corev1.ObjectReference{},
		ImagePullSecrets:             []corev1.LocalObjectReference{},
		AutomountServiceAccountToken: new(bool),
	}
	return obj
}

func AddServiceAccountSecret(sa *corev1.ServiceAccount, secret corev1.ObjectReference) {
	sa.Secrets = append(sa.Secrets, secret)
}

func AddServiceAccountImagePullSecret(sa *corev1.ServiceAccount, secret corev1.LocalObjectReference) {
	sa.ImagePullSecrets = append(sa.ImagePullSecrets, secret)
}

func SetServiceAccountAutomountToken(sa *corev1.ServiceAccount, automount bool) {
	if sa.AutomountServiceAccountToken == nil {
		sa.AutomountServiceAccountToken = new(bool)
	}
	*sa.AutomountServiceAccountToken = automount
}
