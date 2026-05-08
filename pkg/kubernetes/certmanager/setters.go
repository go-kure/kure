package certmanager

import (
	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Certificate setters

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
func SetCertificateIssuerRef(obj *certv1.Certificate, ref cmmeta.IssuerReference) {
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

// Issuer setters

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
	obj.Spec.IssuerConfig.ACME = acme
}

// SetIssuerCA sets the CA configuration on the issuer spec.
func SetIssuerCA(obj *certv1.Issuer, ca *certv1.CAIssuer) {
	obj.Spec.IssuerConfig.CA = ca
}

// ClusterIssuer setters

// AddClusterIssuerLabel adds or updates a label on the ClusterIssuer metadata.
func AddClusterIssuerLabel(obj *certv1.ClusterIssuer, key, value string) {
	if obj.Labels == nil {
		obj.Labels = make(map[string]string)
	}
	obj.Labels[key] = value
}

// AddClusterIssuerAnnotation adds or updates an annotation on the ClusterIssuer metadata.
func AddClusterIssuerAnnotation(obj *certv1.ClusterIssuer, key, value string) {
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string)
	}
	obj.Annotations[key] = value
}

// SetClusterIssuerACME sets the ACME config on the ClusterIssuer.
func SetClusterIssuerACME(obj *certv1.ClusterIssuer, acme *cmacme.ACMEIssuer) {
	obj.Spec.IssuerConfig.ACME = acme
}

// SetClusterIssuerCA sets the CA configuration on the ClusterIssuer spec.
func SetClusterIssuerCA(obj *certv1.ClusterIssuer, ca *certv1.CAIssuer) {
	obj.Spec.IssuerConfig.CA = ca
}
