package fluxcd

import (
	"errors"

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
func SetResourceSetInputProviderType(obj *fluxv1.ResourceSetInputProvider, typ string) error {
	if obj == nil {
		return errors.New("nil ResourceSetInputProvider")
	}
	obj.Spec.Type = typ
	return nil
}

// SetResourceSetInputProviderURL sets the provider URL.
func SetResourceSetInputProviderURL(obj *fluxv1.ResourceSetInputProvider, url string) error {
	if obj == nil {
		return errors.New("nil ResourceSetInputProvider")
	}
	obj.Spec.URL = url
	return nil
}

// SetResourceSetInputProviderServiceAccountName sets the service account name.
func SetResourceSetInputProviderServiceAccountName(obj *fluxv1.ResourceSetInputProvider, name string) error {
	if obj == nil {
		return errors.New("nil ResourceSetInputProvider")
	}
	obj.Spec.ServiceAccountName = name
	return nil
}

// SetResourceSetInputProviderSecretRef sets the secret reference.
func SetResourceSetInputProviderSecretRef(obj *fluxv1.ResourceSetInputProvider, ref *meta.LocalObjectReference) error {
	if obj == nil {
		return errors.New("nil ResourceSetInputProvider")
	}
	obj.Spec.SecretRef = ref
	return nil
}

// SetResourceSetInputProviderCertSecretRef sets the certificate secret reference.
func SetResourceSetInputProviderCertSecretRef(obj *fluxv1.ResourceSetInputProvider, ref *meta.LocalObjectReference) error {
	if obj == nil {
		return errors.New("nil ResourceSetInputProvider")
	}
	obj.Spec.CertSecretRef = ref
	return nil
}

// AddResourceSetInputProviderSchedule appends a schedule to the provider.
func AddResourceSetInputProviderSchedule(obj *fluxv1.ResourceSetInputProvider, s fluxv1.Schedule) error {
	if obj == nil {
		return errors.New("nil ResourceSetInputProvider")
	}
	obj.Spec.Schedule = append(obj.Spec.Schedule, s)
	return nil
}
