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

	AddIssuerLabel(issuer, "env", "prod")
	if issuer.Labels["env"] != "prod" {
		t.Errorf("label not set")
	}

	AddIssuerAnnotation(issuer, "team", "dev")
	if issuer.Annotations["team"] != "dev" {
		t.Errorf("annotation not set")
	}

	acme := &cmacme.ACMEIssuer{Server: "https://acme.example.com"}
	SetIssuerACME(issuer, acme)
	if issuer.Spec.IssuerConfig.ACME == nil || issuer.Spec.IssuerConfig.ACME.Server != "https://acme.example.com" {
		t.Errorf("acme config not set")
	}

	ca := &certv1.CAIssuer{SecretName: "ca"}
	SetIssuerCA(issuer, ca)
	if issuer.Spec.IssuerConfig.CA == nil || issuer.Spec.IssuerConfig.CA.SecretName != "ca" {
		t.Errorf("ca config not set")
	}
}
