package k8s

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateSecret(name, namespace string) *corev1.Secret {
	obj := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
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
		Data:       map[string][]byte{},
		StringData: map[string]string{},
		Type:       corev1.SecretTypeOpaque,
		Immutable:  new(bool),
	}
	return obj
}

func AddSecretData(secret *corev1.Secret, key string, value []byte) error {
	if secret == nil {
		return errors.New("nil secret")
	}
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[key] = value
	return nil
}

func AddSecretStringData(secret *corev1.Secret, key, value string) error {
	if secret == nil {
		return errors.New("nil secret")
	}
	if secret.StringData == nil {
		secret.StringData = make(map[string]string)
	}
	secret.StringData[key] = value
	return nil
}

func SetSecretType(secret *corev1.Secret, type_ corev1.SecretType) error {
	if secret == nil {
		return errors.New("nil secret")
	}
	secret.Type = type_
	return nil
}

func SetSecretImmutable(secret *corev1.Secret, immutable bool) error {
	if secret == nil {
		return errors.New("nil secret")
	}
	if secret.Immutable == nil {
		secret.Immutable = new(bool)
	}
	*secret.Immutable = immutable
	return nil
}

func AddSecretLabel(secret *corev1.Secret, key, value string) error {
	if secret == nil {
		return errors.New("nil secret")
	}
	if secret.Labels == nil {
		secret.Labels = make(map[string]string)
	}
	secret.Labels[key] = value
	return nil
}

func AddSecretAnnotation(secret *corev1.Secret, key, value string) error {
	if secret == nil {
		return errors.New("nil secret")
	}
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[key] = value
	return nil
}

func SetSecretLabels(secret *corev1.Secret, labels map[string]string) error {
	if secret == nil {
		return errors.New("nil secret")
	}
	secret.Labels = labels
	return nil
}

func SetSecretAnnotations(secret *corev1.Secret, anns map[string]string) error {
	if secret == nil {
		return errors.New("nil secret")
	}
	secret.Annotations = anns
	return nil
}
