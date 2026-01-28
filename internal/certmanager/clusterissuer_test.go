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

	if err := AddClusterIssuerLabel(ci, "app", "demo"); err != nil {
		t.Errorf("AddClusterIssuerLabel failed: %v", err)
	}
	if ci.Labels["app"] != "demo" {
		t.Errorf("label not set")
	}

	if err := AddClusterIssuerAnnotation(ci, "team", "dev"); err != nil {
		t.Errorf("AddClusterIssuerAnnotation failed: %v", err)
	}
	if ci.Annotations["team"] != "dev" {
		t.Errorf("annotation not set")
	}

	acme := &cmacme.ACMEIssuer{Server: "https://acme.example.com"}
	if err := SetClusterIssuerACME(ci, acme); err != nil {
		t.Errorf("SetClusterIssuerACME failed: %v", err)
	}
	if ci.Spec.IssuerConfig.ACME == nil || ci.Spec.IssuerConfig.ACME.Server != "https://acme.example.com" {
		t.Errorf("acme config not set")
	}
}

func TestClusterIssuerFunctionsWithNil(t *testing.T) {
	// Test that functions return errors when given nil ClusterIssuer
	if err := AddClusterIssuerLabel(nil, "key", "value"); err == nil {
		t.Error("expected error for nil ClusterIssuer")
	}
	if err := AddClusterIssuerAnnotation(nil, "key", "value"); err == nil {
		t.Error("expected error for nil ClusterIssuer")
	}
	if err := SetClusterIssuerACME(nil, nil); err == nil {
		t.Error("expected error for nil ClusterIssuer")
	}
}
