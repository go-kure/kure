package externalsecrets

import (
	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateSecretStore returns a SecretStore object with the given name, namespace and spec.
func CreateSecretStore(name, namespace string, spec esv1.SecretStoreSpec) *esv1.SecretStore {
	obj := &esv1.SecretStore{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SecretStore",
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

// AddSecretStoreLabel adds or updates a label on the SecretStore.
func AddSecretStoreLabel(obj *esv1.SecretStore, key, value string) {
	if obj.Labels == nil {
		obj.Labels = make(map[string]string)
	}
	obj.Labels[key] = value
}

// AddSecretStoreAnnotation adds or updates an annotation on the SecretStore.
func AddSecretStoreAnnotation(obj *esv1.SecretStore, key, value string) {
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}
	obj.Annotations[key] = value
}

// SetSecretStoreProvider sets the provider field on the SecretStore spec.
func SetSecretStoreProvider(obj *esv1.SecretStore, provider *esv1.SecretStoreProvider) {
	obj.Spec.Provider = provider
}

// SetSecretStoreController sets the controller name on the SecretStore spec.
func SetSecretStoreController(obj *esv1.SecretStore, controller string) {
	obj.Spec.Controller = controller
}
