package kubernetes

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

func SetServiceAccountSecrets(sa *corev1.ServiceAccount, secrets []corev1.ObjectReference) {
	if sa == nil {
		panic("SetServiceAccountSecrets: sa must not be nil")
	}
	sa.Secrets = secrets
}

func SetServiceAccountImagePullSecrets(sa *corev1.ServiceAccount, secrets []corev1.LocalObjectReference) {
	if sa == nil {
		panic("SetServiceAccountImagePullSecrets: sa must not be nil")
	}
	sa.ImagePullSecrets = secrets
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

func AddServiceAccountLabel(sa *corev1.ServiceAccount, key, value string) {
	if sa.Labels == nil {
		sa.Labels = make(map[string]string)
	}
	sa.Labels[key] = value
}

func AddServiceAccountAnnotation(sa *corev1.ServiceAccount, key, value string) {
	if sa.Annotations == nil {
		sa.Annotations = make(map[string]string)
	}
	sa.Annotations[key] = value
}

func SetServiceAccountLabels(sa *corev1.ServiceAccount, labels map[string]string) {
	sa.Labels = labels
}

func SetServiceAccountAnnotations(sa *corev1.ServiceAccount, annotations map[string]string) {
	sa.Annotations = annotations
}
