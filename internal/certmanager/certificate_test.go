package certmanager

import (
	"testing"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCertificateFunctions(t *testing.T) {
	spec := certv1.CertificateSpec{}
	crt := CreateCertificate("demo", "ns", spec)

	if crt.Name != "demo" || crt.Namespace != "ns" {
		t.Fatalf("metadata mismatch: %s/%s", crt.Namespace, crt.Name)
	}
	if crt.Kind != "Certificate" {
		t.Errorf("unexpected kind %q", crt.Kind)
	}

	AddCertificateLabel(crt, "app", "demo")
	if crt.Labels["app"] != "demo" {
		t.Errorf("label not set")
	}

	AddCertificateAnnotation(crt, "team", "dev")
	if crt.Annotations["team"] != "dev" {
		t.Errorf("annotation not set")
	}

	AddCertificateDNSName(crt, "example.com")
	if len(crt.Spec.DNSNames) != 1 || crt.Spec.DNSNames[0] != "example.com" {
		t.Errorf("dns name not added")
	}

	ref := cmmeta.ObjectReference{Name: "issuer"}
	SetCertificateIssuerRef(crt, ref)
	if crt.Spec.IssuerRef.Name != "issuer" {
		t.Errorf("issuerRef not set")
	}

	dur := metav1.Duration{Duration: 0}
	SetCertificateDuration(crt, &dur)
	if crt.Spec.Duration == nil {
		t.Errorf("duration not set")
	}

	SetCertificateRenewBefore(crt, &dur)
	if crt.Spec.RenewBefore == nil {
		t.Errorf("renewBefore not set")
	}
}
