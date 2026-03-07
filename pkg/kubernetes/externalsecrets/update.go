package externalsecrets

import (
	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"

	intes "github.com/go-kure/kure/internal/externalsecrets"
)

// SetExternalSecretSpec replaces the spec on the ExternalSecret object.
func SetExternalSecretSpec(obj *esv1.ExternalSecret, spec esv1.ExternalSecretSpec) {
	obj.Spec = spec
}

// SetSecretStoreSpec replaces the spec on the SecretStore object.
func SetSecretStoreSpec(obj *esv1.SecretStore, spec esv1.SecretStoreSpec) {
	obj.Spec = spec
}

// SetClusterSecretStoreSpec replaces the spec on the ClusterSecretStore object.
func SetClusterSecretStoreSpec(obj *esv1.ClusterSecretStore, spec esv1.SecretStoreSpec) {
	obj.Spec = spec
}

// AddExternalSecretLabel delegates to the internal helper.
func AddExternalSecretLabel(obj *esv1.ExternalSecret, key, value string) {
	intes.AddExternalSecretLabel(obj, key, value)
}

// AddExternalSecretAnnotation delegates to the internal helper.
func AddExternalSecretAnnotation(obj *esv1.ExternalSecret, key, value string) {
	intes.AddExternalSecretAnnotation(obj, key, value)
}

// AddExternalSecretData delegates to the internal helper.
func AddExternalSecretData(obj *esv1.ExternalSecret, data esv1.ExternalSecretData) {
	intes.AddExternalSecretData(obj, data)
}

// SetExternalSecretSecretStoreRef delegates to the internal helper.
func SetExternalSecretSecretStoreRef(obj *esv1.ExternalSecret, ref esv1.SecretStoreRef) {
	intes.SetExternalSecretSecretStoreRef(obj, ref)
}

// AddSecretStoreLabel delegates to the internal helper.
func AddSecretStoreLabel(obj *esv1.SecretStore, key, value string) {
	intes.AddSecretStoreLabel(obj, key, value)
}

// AddSecretStoreAnnotation delegates to the internal helper.
func AddSecretStoreAnnotation(obj *esv1.SecretStore, key, value string) {
	intes.AddSecretStoreAnnotation(obj, key, value)
}

// SetSecretStoreProvider delegates to the internal helper.
func SetSecretStoreProvider(obj *esv1.SecretStore, provider *esv1.SecretStoreProvider) {
	intes.SetSecretStoreProvider(obj, provider)
}

// SetSecretStoreController delegates to the internal helper.
func SetSecretStoreController(obj *esv1.SecretStore, controller string) {
	intes.SetSecretStoreController(obj, controller)
}

// AddClusterSecretStoreLabel delegates to the internal helper.
func AddClusterSecretStoreLabel(obj *esv1.ClusterSecretStore, key, value string) {
	intes.AddClusterSecretStoreLabel(obj, key, value)
}

// AddClusterSecretStoreAnnotation delegates to the internal helper.
func AddClusterSecretStoreAnnotation(obj *esv1.ClusterSecretStore, key, value string) {
	intes.AddClusterSecretStoreAnnotation(obj, key, value)
}

// SetClusterSecretStoreProvider delegates to the internal helper.
func SetClusterSecretStoreProvider(obj *esv1.ClusterSecretStore, provider *esv1.SecretStoreProvider) {
	intes.SetClusterSecretStoreProvider(obj, provider)
}

// SetClusterSecretStoreController delegates to the internal helper.
func SetClusterSecretStoreController(obj *esv1.ClusterSecretStore, controller string) {
	intes.SetClusterSecretStoreController(obj, controller)
}
