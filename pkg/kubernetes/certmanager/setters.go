package certmanager

import (
	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Labels and annotations use the generic kubernetes.AddLabel /
// kubernetes.AddAnnotation over metav1.Object; this package carries no per-kind
// metadata helpers.

// Certificate setters

// AddCertificateDNSName appends a DNS name to the Certificate spec.
func AddCertificateDNSName(obj *certv1.Certificate, dns string) {
	obj.Spec.DNSNames = append(obj.Spec.DNSNames, dns)
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

// SetIssuerACME sets the ACME configuration on the issuer spec.
func SetIssuerACME(obj *certv1.Issuer, acme *cmacme.ACMEIssuer) {
	obj.Spec.IssuerConfig.ACME = acme
}

// SetIssuerCA sets the CA configuration on the issuer spec.
func SetIssuerCA(obj *certv1.Issuer, ca *certv1.CAIssuer) {
	obj.Spec.IssuerConfig.CA = ca
}

// ClusterIssuer setters

// SetClusterIssuerACME sets the ACME config on the ClusterIssuer.
func SetClusterIssuerACME(obj *certv1.ClusterIssuer, acme *cmacme.ACMEIssuer) {
	obj.Spec.IssuerConfig.ACME = acme
}

// SetClusterIssuerCA sets the CA configuration on the ClusterIssuer spec.
func SetClusterIssuerCA(obj *certv1.ClusterIssuer, ca *certv1.CAIssuer) {
	obj.Spec.IssuerConfig.CA = ca
}
