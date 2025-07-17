package fluxcd

import (
	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	meta "github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateResourceSetInputProvider returns a new ResourceSetInputProvider object.
func CreateResourceSetInputProvider(name, namespace string, spec fluxv1.ResourceSetInputProviderSpec) *fluxv1.ResourceSetInputProvider {
	obj := &fluxv1.ResourceSetInputProvider{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxv1.ResourceSetInputProviderKind,
			APIVersion: fluxv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}

// SetResourceSetInputProviderType sets the provider type.
func SetResourceSetInputProviderType(obj *fluxv1.ResourceSetInputProvider, typ string) {
	obj.Spec.Type = typ
}

// SetResourceSetInputProviderURL sets the provider URL.
func SetResourceSetInputProviderURL(obj *fluxv1.ResourceSetInputProvider, url string) {
	obj.Spec.URL = url
}

// SetResourceSetInputProviderServiceAccountName sets the service account name.
func SetResourceSetInputProviderServiceAccountName(obj *fluxv1.ResourceSetInputProvider, name string) {
	obj.Spec.ServiceAccountName = name
}

// SetResourceSetInputProviderSecretRef sets the secret reference.
func SetResourceSetInputProviderSecretRef(obj *fluxv1.ResourceSetInputProvider, ref *meta.LocalObjectReference) {
	obj.Spec.SecretRef = ref
}

// SetResourceSetInputProviderCertSecretRef sets the certificate secret reference.
func SetResourceSetInputProviderCertSecretRef(obj *fluxv1.ResourceSetInputProvider, ref *meta.LocalObjectReference) {
	obj.Spec.CertSecretRef = ref
}

// AddResourceSetInputProviderSchedule appends a schedule to the provider.
func AddResourceSetInputProviderSchedule(obj *fluxv1.ResourceSetInputProvider, s fluxv1.Schedule) {
	obj.Spec.Schedule = append(obj.Spec.Schedule, s)
}
