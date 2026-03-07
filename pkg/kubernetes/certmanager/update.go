package certmanager

import (
	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	intcm "github.com/go-kure/kure/internal/certmanager"
)

// SetCertificateSpec replaces the spec on the Certificate object.
func SetCertificateSpec(obj *certv1.Certificate, spec certv1.CertificateSpec) {
	obj.Spec = spec
}

// SetIssuerSpec replaces the spec on the Issuer object.
func SetIssuerSpec(obj *certv1.Issuer, spec certv1.IssuerSpec) {
	obj.Spec = spec
}

// SetClusterIssuerSpec replaces the spec on the ClusterIssuer object.
func SetClusterIssuerSpec(obj *certv1.ClusterIssuer, spec certv1.IssuerSpec) {
	obj.Spec = spec
}

// AddCertificateLabel delegates to the internal helper.
func AddCertificateLabel(obj *certv1.Certificate, key, value string) error {
	return intcm.AddCertificateLabel(obj, key, value)
}

// AddCertificateAnnotation delegates to the internal helper.
func AddCertificateAnnotation(obj *certv1.Certificate, key, value string) error {
	return intcm.AddCertificateAnnotation(obj, key, value)
}

// AddCertificateDNSName delegates to the internal helper.
func AddCertificateDNSName(obj *certv1.Certificate, dns string) error {
	return intcm.AddCertificateDNSName(obj, dns)
}

// SetCertificateIssuerRef delegates to the internal helper.
func SetCertificateIssuerRef(obj *certv1.Certificate, ref cmmeta.ObjectReference) error {
	return intcm.SetCertificateIssuerRef(obj, ref)
}

// SetCertificateDuration delegates to the internal helper.
func SetCertificateDuration(obj *certv1.Certificate, dur *metav1.Duration) error {
	return intcm.SetCertificateDuration(obj, dur)
}

// SetCertificateRenewBefore delegates to the internal helper.
func SetCertificateRenewBefore(obj *certv1.Certificate, dur *metav1.Duration) error {
	return intcm.SetCertificateRenewBefore(obj, dur)
}

// AddIssuerLabel delegates to the internal helper.
func AddIssuerLabel(obj *certv1.Issuer, key, value string) error {
	return intcm.AddIssuerLabel(obj, key, value)
}

// AddIssuerAnnotation delegates to the internal helper.
func AddIssuerAnnotation(obj *certv1.Issuer, key, value string) error {
	return intcm.AddIssuerAnnotation(obj, key, value)
}

// SetIssuerACME delegates to the internal helper.
func SetIssuerACME(obj *certv1.Issuer, acme *cmacme.ACMEIssuer) error {
	return intcm.SetIssuerACME(obj, acme)
}

// SetIssuerCA delegates to the internal helper.
func SetIssuerCA(obj *certv1.Issuer, ca *certv1.CAIssuer) error {
	return intcm.SetIssuerCA(obj, ca)
}

// AddClusterIssuerLabel delegates to the internal helper.
func AddClusterIssuerLabel(obj *certv1.ClusterIssuer, key, value string) error {
	return intcm.AddClusterIssuerLabel(obj, key, value)
}

// AddClusterIssuerAnnotation delegates to the internal helper.
func AddClusterIssuerAnnotation(obj *certv1.ClusterIssuer, key, value string) error {
	return intcm.AddClusterIssuerAnnotation(obj, key, value)
}

// SetClusterIssuerACME delegates to the internal helper.
func SetClusterIssuerACME(obj *certv1.ClusterIssuer, acme *cmacme.ACMEIssuer) error {
	return intcm.SetClusterIssuerACME(obj, acme)
}
