package externalsecrets

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	esv1beta1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1beta1"
)

// CreateSecretStore returns a new SecretStore object with the provided name, namespace and spec.
func CreateSecretStore(name, namespace string, spec esv1beta1.SecretStoreSpec) *esv1beta1.SecretStore {
	obj := &esv1beta1.SecretStore{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SecretStore",
			APIVersion: esv1beta1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}

// AddSecretStoreLabel adds a label to the SecretStore metadata.
func AddSecretStoreLabel(obj *esv1beta1.SecretStore, key, value string) {
	if obj.Labels == nil {
		obj.Labels = map[string]string{}
	}
	obj.Labels[key] = value
}

// AddSecretStoreAnnotation adds an annotation to the SecretStore metadata.
func AddSecretStoreAnnotation(obj *esv1beta1.SecretStore, key, value string) {
	if obj.Annotations == nil {
		obj.Annotations = map[string]string{}
	}
	obj.Annotations[key] = value
}

// SetSecretStoreProvider sets the provider configuration on the SecretStore.
func SetSecretStoreProvider(obj *esv1beta1.SecretStore, provider *esv1beta1.SecretStoreProvider) {
	obj.Spec.Provider = provider
}
