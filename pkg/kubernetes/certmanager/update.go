package certmanager

import (
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
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
