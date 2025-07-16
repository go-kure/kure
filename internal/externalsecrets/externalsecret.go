package externalsecrets

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	esv1beta1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1beta1"
)

// CreateExternalSecret returns a new ExternalSecret object with the provided name, namespace and spec.
func CreateExternalSecret(name, namespace string, spec esv1beta1.ExternalSecretSpec) *esv1beta1.ExternalSecret {
	obj := &esv1beta1.ExternalSecret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ExternalSecret",
			APIVersion: esv1beta1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}

// AddExternalSecretLabel adds a label to the ExternalSecret metadata.
func AddExternalSecretLabel(obj *esv1beta1.ExternalSecret, key, value string) {
	if obj.Labels == nil {
		obj.Labels = map[string]string{}
	}
	obj.Labels[key] = value
}

// AddExternalSecretAnnotation adds an annotation to the ExternalSecret metadata.
func AddExternalSecretAnnotation(obj *esv1beta1.ExternalSecret, key, value string) {
	if obj.Annotations == nil {
		obj.Annotations = map[string]string{}
	}
	obj.Annotations[key] = value
}

// AddExternalSecretData appends a data mapping to the ExternalSecret spec.
func AddExternalSecretData(obj *esv1beta1.ExternalSecret, data esv1beta1.ExternalSecretData) {
	obj.Spec.Data = append(obj.Spec.Data, data)
}

// AddExternalSecretDataFrom appends a dataFrom entry to the ExternalSecret spec.
func AddExternalSecretDataFrom(obj *esv1beta1.ExternalSecret, ref esv1beta1.ExternalSecretDataFromRemoteRef) {
	obj.Spec.DataFrom = append(obj.Spec.DataFrom, ref)
}

// SetExternalSecretRefreshInterval sets the refresh interval on the ExternalSecret spec.
func SetExternalSecretRefreshInterval(obj *esv1beta1.ExternalSecret, interval metav1.Duration) {
	obj.Spec.RefreshInterval = &interval
}
