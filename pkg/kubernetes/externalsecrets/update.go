package externalsecrets

import (
	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Labels and annotations use the generic kubernetes.AddLabel /
// kubernetes.AddAnnotation over metav1.Object; this package carries no per-kind
// metadata helpers.

// AddExternalSecretData appends a data entry to the ExternalSecret spec.
func AddExternalSecretData(obj *esv1.ExternalSecret, data esv1.ExternalSecretData) {
	obj.Spec.Data = append(obj.Spec.Data, data)
}

// SetSecretStoreProvider sets the provider field on the SecretStore spec.
func SetSecretStoreProvider(obj *esv1.SecretStore, provider *esv1.SecretStoreProvider) {
	obj.Spec.Provider = provider
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
