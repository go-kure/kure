package k8s

import (
	"errors"

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

func AddServiceAccountSecret(sa *corev1.ServiceAccount, secret corev1.ObjectReference) error {
	if sa == nil {
		return errors.New("nil serviceaccount")
	}
	sa.Secrets = append(sa.Secrets, secret)
	return nil
}

func AddServiceAccountImagePullSecret(sa *corev1.ServiceAccount, secret corev1.LocalObjectReference) error {
	if sa == nil {
		return errors.New("nil serviceaccount")
	}
	sa.ImagePullSecrets = append(sa.ImagePullSecrets, secret)
	return nil
}

func SetServiceAccountSecrets(sa *corev1.ServiceAccount, secrets []corev1.ObjectReference) error {
	if sa == nil {
		return errors.New("nil serviceaccount")
	}
	sa.Secrets = secrets
	return nil
}

func SetServiceAccountImagePullSecrets(sa *corev1.ServiceAccount, secrets []corev1.LocalObjectReference) error {
	if sa == nil {
		return errors.New("nil serviceaccount")
	}
	sa.ImagePullSecrets = secrets
	return nil
}

func SetServiceAccountAutomountToken(sa *corev1.ServiceAccount, automount bool) error {
	if sa == nil {
		return errors.New("nil serviceaccount")
	}
	if sa.AutomountServiceAccountToken == nil {
		sa.AutomountServiceAccountToken = new(bool)
	}
	*sa.AutomountServiceAccountToken = automount
	return nil
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
