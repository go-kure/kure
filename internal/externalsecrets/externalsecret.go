package externalsecrets

import (
	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateExternalSecret returns a new ExternalSecret object with the provided name, namespace and spec.
func CreateExternalSecret(name, namespace string, spec esv1.ExternalSecretSpec) *esv1.ExternalSecret {
	obj := &esv1.ExternalSecret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ExternalSecret",
			APIVersion: esv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}

// AddExternalSecretLabel adds or updates a label on the ExternalSecret.
func AddExternalSecretLabel(obj *esv1.ExternalSecret, key, value string) {
	if obj.Labels == nil {
		obj.Labels = make(map[string]string)
	}
	obj.Labels[key] = value
}

// AddExternalSecretAnnotation adds or updates an annotation on the ExternalSecret.
func AddExternalSecretAnnotation(obj *esv1.ExternalSecret, key, value string) {
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}
	obj.Annotations[key] = value
}

// AddExternalSecretData appends a data entry to the ExternalSecret spec.
func AddExternalSecretData(obj *esv1.ExternalSecret, data esv1.ExternalSecretData) {
	obj.Spec.Data = append(obj.Spec.Data, data)
}

// SetExternalSecretSecretStoreRef sets the secret store reference on the ExternalSecret spec.
func SetExternalSecretSecretStoreRef(obj *esv1.ExternalSecret, ref esv1.SecretStoreRef) {
	obj.Spec.SecretStoreRef = ref
}
