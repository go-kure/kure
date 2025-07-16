package externalsecrets

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	esv1beta1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1beta1"
)

// CreateClusterSecretStore returns a new ClusterSecretStore object.
func CreateClusterSecretStore(name string, spec esv1beta1.SecretStoreSpec) *esv1beta1.ClusterSecretStore {
	obj := &esv1beta1.ClusterSecretStore{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterSecretStore",
			APIVersion: esv1beta1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: spec,
	}
	return obj
}

// AddClusterSecretStoreLabel adds a label to the ClusterSecretStore metadata.
func AddClusterSecretStoreLabel(obj *esv1beta1.ClusterSecretStore, key, value string) {
	if obj.Labels == nil {
		obj.Labels = map[string]string{}
	}
	obj.Labels[key] = value
}

// AddClusterSecretStoreAnnotation adds an annotation to the ClusterSecretStore metadata.
func AddClusterSecretStoreAnnotation(obj *esv1beta1.ClusterSecretStore, key, value string) {
	if obj.Annotations == nil {
		obj.Annotations = map[string]string{}
	}
	obj.Annotations[key] = value
}

// SetClusterSecretStoreProvider sets the provider configuration on the ClusterSecretStore.
func SetClusterSecretStoreProvider(obj *esv1beta1.ClusterSecretStore, provider *esv1beta1.SecretStoreProvider) {
	obj.Spec.Provider = provider
}
