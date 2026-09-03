package externalsecrets

import (
	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

// SetRefreshInterval sets the polling interval on an ExternalSecret.
func SetRefreshInterval(obj *esv1.ExternalSecret, d metav1.Duration) {
	obj.Spec.RefreshInterval = &d
}

// AddDataFrom appends a dataFrom source to an ExternalSecret.
func AddDataFrom(obj *esv1.ExternalSecret, source esv1.ExternalSecretDataFromRemoteRef) {
	obj.Spec.DataFrom = append(obj.Spec.DataFrom, source)
}
