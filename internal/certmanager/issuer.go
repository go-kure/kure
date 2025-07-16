package certmanager

import (
	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateIssuer returns a new Issuer object with the provided name, namespace and spec.
func CreateIssuer(name, namespace string, spec certv1.IssuerSpec) *certv1.Issuer {
	obj := &certv1.Issuer{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Issuer",
			APIVersion: certv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}

// AddIssuerLabel adds or updates a label on the Issuer metadata.
func AddIssuerLabel(obj *certv1.Issuer, key, value string) {
	if obj.Labels == nil {
		obj.Labels = make(map[string]string)
	}
	obj.Labels[key] = value
}

// AddIssuerAnnotation adds or updates an annotation on the Issuer metadata.
func AddIssuerAnnotation(obj *certv1.Issuer, key, value string) {
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}
	obj.Annotations[key] = value
}

// SetIssuerACME sets the ACME configuration on the issuer spec.
func SetIssuerACME(obj *certv1.Issuer, acme *cmacme.ACMEIssuer) {
	if obj == nil {
		return
	}
	obj.Spec.IssuerConfig.ACME = acme
}

// SetIssuerCA sets the CA configuration on the issuer spec.
func SetIssuerCA(obj *certv1.Issuer, ca *certv1.CAIssuer) {
	obj.Spec.IssuerConfig.CA = ca
}
