package certmanager

import (
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateCertificate returns a new Certificate object with the provided name, namespace and spec.
func CreateCertificate(name, namespace string, spec certv1.CertificateSpec) *certv1.Certificate {
	obj := &certv1.Certificate{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Certificate",
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

// AddCertificateLabel adds or updates a label on the Certificate metadata.
func AddCertificateLabel(obj *certv1.Certificate, key, value string) {
	if obj.Labels == nil {
		obj.Labels = make(map[string]string)
	}
	obj.Labels[key] = value
}

// AddCertificateAnnotation adds or updates an annotation on the Certificate metadata.
func AddCertificateAnnotation(obj *certv1.Certificate, key, value string) {
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}
	obj.Annotations[key] = value
}

// AddCertificateDNSName appends a DNS name to the Certificate spec.
func AddCertificateDNSName(obj *certv1.Certificate, dns string) {
	obj.Spec.DNSNames = append(obj.Spec.DNSNames, dns)
}

// SetCertificateIssuerRef sets the issuer reference for the certificate.
func SetCertificateIssuerRef(obj *certv1.Certificate, ref cmmeta.ObjectReference) {
	obj.Spec.IssuerRef = ref
}

// SetCertificateDuration sets the desired certificate duration.
func SetCertificateDuration(obj *certv1.Certificate, dur *metav1.Duration) {
	obj.Spec.Duration = dur
}

// SetCertificateRenewBefore sets the renewBefore field of the certificate spec.
func SetCertificateRenewBefore(obj *certv1.Certificate, dur *metav1.Duration) {
	obj.Spec.RenewBefore = dur
}
