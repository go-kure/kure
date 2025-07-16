package certmanager

import (
	"testing"

	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
)

func TestClusterIssuerFunctions(t *testing.T) {
	spec := certv1.IssuerSpec{}
	ci := CreateClusterIssuer("demo", spec)

	if ci.Name != "demo" {
		t.Fatalf("name mismatch: %s", ci.Name)
	}
	if ci.Kind != "ClusterIssuer" {
		t.Errorf("unexpected kind %q", ci.Kind)
	}

	AddClusterIssuerLabel(ci, "app", "demo")
	if ci.Labels["app"] != "demo" {
		t.Errorf("label not set")
	}

	AddClusterIssuerAnnotation(ci, "team", "dev")
	if ci.Annotations["team"] != "dev" {
		t.Errorf("annotation not set")
	}

	acme := &cmacme.ACMEIssuer{Server: "https://acme.example.com"}
	SetClusterIssuerACME(ci, acme)
	if ci.Spec.IssuerConfig.ACME == nil || ci.Spec.IssuerConfig.ACME.Server != "https://acme.example.com" {
		t.Errorf("acme config not set")
	}
}
