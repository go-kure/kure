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

	if err := AddCertificateLabel(crt, "app", "demo"); err != nil {
		t.Errorf("AddCertificateLabel failed: %v", err)
	}
	if crt.Labels["app"] != "demo" {
		t.Errorf("label not set")
	}

	if err := AddCertificateAnnotation(crt, "team", "dev"); err != nil {
		t.Errorf("AddCertificateAnnotation failed: %v", err)
	}
	if crt.Annotations["team"] != "dev" {
		t.Errorf("annotation not set")
	}

	if err := AddCertificateDNSName(crt, "example.com"); err != nil {
		t.Errorf("AddCertificateDNSName failed: %v", err)
	}
	if len(crt.Spec.DNSNames) != 1 || crt.Spec.DNSNames[0] != "example.com" {
		t.Errorf("dns name not added")
	}

	ref := cmmeta.ObjectReference{Name: "issuer"}
	if err := SetCertificateIssuerRef(crt, ref); err != nil {
		t.Errorf("SetCertificateIssuerRef failed: %v", err)
	}
	if crt.Spec.IssuerRef.Name != "issuer" {
		t.Errorf("issuerRef not set")
	}

	dur := metav1.Duration{Duration: 0}
	if err := SetCertificateDuration(crt, &dur); err != nil {
		t.Errorf("SetCertificateDuration failed: %v", err)
	}
	if crt.Spec.Duration == nil {
		t.Errorf("duration not set")
	}

	if err := SetCertificateRenewBefore(crt, &dur); err != nil {
		t.Errorf("SetCertificateRenewBefore failed: %v", err)
	}
	if crt.Spec.RenewBefore == nil {
		t.Errorf("renewBefore not set")
	}
}

func TestCertificateFunctionsWithNil(t *testing.T) {
	// Test that functions return errors when given nil Certificate
	if err := AddCertificateLabel(nil, "key", "value"); err == nil {
		t.Error("expected error for nil Certificate")
	}
	if err := AddCertificateAnnotation(nil, "key", "value"); err == nil {
		t.Error("expected error for nil Certificate")
	}
	if err := AddCertificateDNSName(nil, "example.com"); err == nil {
		t.Error("expected error for nil Certificate")
	}
	if err := SetCertificateIssuerRef(nil, cmmeta.ObjectReference{}); err == nil {
		t.Error("expected error for nil Certificate")
	}
	if err := SetCertificateDuration(nil, nil); err == nil {
		t.Error("expected error for nil Certificate")
	}
	if err := SetCertificateRenewBefore(nil, nil); err == nil {
		t.Error("expected error for nil Certificate")
	}
}
