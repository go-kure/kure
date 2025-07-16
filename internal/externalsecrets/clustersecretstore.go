package externalsecrets

import (
	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateClusterSecretStore returns a ClusterSecretStore object with the given name and spec.
func CreateClusterSecretStore(name string, spec esv1.SecretStoreSpec) *esv1.ClusterSecretStore {
	obj := &esv1.ClusterSecretStore{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterSecretStore",
			APIVersion: esv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: spec,
	}
	return obj
}

// AddClusterSecretStoreLabel adds or updates a label on the ClusterSecretStore.
func AddClusterSecretStoreLabel(obj *esv1.ClusterSecretStore, key, value string) {
	if obj.Labels == nil {
		obj.Labels = make(map[string]string)
	}
	obj.Labels[key] = value
}

// AddClusterSecretStoreAnnotation adds or updates an annotation on the ClusterSecretStore.
func AddClusterSecretStoreAnnotation(obj *esv1.ClusterSecretStore, key, value string) {
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}
	obj.Annotations[key] = value
}

// SetClusterSecretStoreProvider sets the provider field on the ClusterSecretStore spec.
func SetClusterSecretStoreProvider(obj *esv1.ClusterSecretStore, provider *esv1.SecretStoreProvider) {
	obj.Spec.Provider = provider
}

// SetClusterSecretStoreController sets the controller name on the ClusterSecretStore spec.
func SetClusterSecretStoreController(obj *esv1.ClusterSecretStore, controller string) {
	obj.Spec.Controller = controller
}
