package certmanager

import (
	"testing"

	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
)

func TestIssuerFunctions(t *testing.T) {
	spec := certv1.IssuerSpec{}
	issuer := CreateIssuer("demo", "ns", spec)

	if issuer.Name != "demo" || issuer.Namespace != "ns" {
		t.Fatalf("metadata mismatch: %s/%s", issuer.Namespace, issuer.Name)
	}
	if issuer.Kind != "Issuer" {
		t.Errorf("unexpected kind %q", issuer.Kind)
	}

	if err := AddIssuerLabel(issuer, "env", "prod"); err != nil {
		t.Errorf("AddIssuerLabel failed: %v", err)
	}
	if issuer.Labels["env"] != "prod" {
		t.Errorf("label not set")
	}

	if err := AddIssuerAnnotation(issuer, "team", "dev"); err != nil {
		t.Errorf("AddIssuerAnnotation failed: %v", err)
	}
	if issuer.Annotations["team"] != "dev" {
		t.Errorf("annotation not set")
	}

	acme := &cmacme.ACMEIssuer{Server: "https://acme.example.com"}
	if err := SetIssuerACME(issuer, acme); err != nil {
		t.Errorf("SetIssuerACME failed: %v", err)
	}
	if issuer.Spec.IssuerConfig.ACME == nil || issuer.Spec.IssuerConfig.ACME.Server != "https://acme.example.com" {
		t.Errorf("acme config not set")
	}

	ca := &certv1.CAIssuer{SecretName: "ca"}
	if err := SetIssuerCA(issuer, ca); err != nil {
		t.Errorf("SetIssuerCA failed: %v", err)
	}
	if issuer.Spec.IssuerConfig.CA == nil || issuer.Spec.IssuerConfig.CA.SecretName != "ca" {
		t.Errorf("ca config not set")
	}
}

func TestIssuerFunctionsWithNil(t *testing.T) {
	// Test that functions return errors when given nil Issuer
	if err := AddIssuerLabel(nil, "key", "value"); err == nil {
		t.Error("expected error for nil Issuer")
	}
	if err := AddIssuerAnnotation(nil, "key", "value"); err == nil {
		t.Error("expected error for nil Issuer")
	}
	if err := SetIssuerACME(nil, nil); err == nil {
		t.Error("expected error for nil Issuer")
	}
	if err := SetIssuerCA(nil, nil); err == nil {
		t.Error("expected error for nil Issuer")
	}
}
