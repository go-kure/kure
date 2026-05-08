package externalsecrets

import (
	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// SetRefreshInterval sets the polling interval on an ExternalSecret.
func SetRefreshInterval(obj *esv1.ExternalSecret, d metav1.Duration) {
	obj.Spec.RefreshInterval = &d
}

// SetTarget sets the target secret configuration on an ExternalSecret.
func SetTarget(obj *esv1.ExternalSecret, target esv1.ExternalSecretTarget) {
	obj.Spec.Target = target
}

// AddDataFrom appends a dataFrom source to an ExternalSecret.
func AddDataFrom(obj *esv1.ExternalSecret, source esv1.ExternalSecretDataFromRemoteRef) {
	obj.Spec.DataFrom = append(obj.Spec.DataFrom, source)
}
