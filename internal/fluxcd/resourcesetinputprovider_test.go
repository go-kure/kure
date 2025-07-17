package fluxcd

import (
	"testing"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	meta "github.com/fluxcd/pkg/apis/meta"
)

func TestCreateResourceSetInputProvider(t *testing.T) {
	rsip := CreateResourceSetInputProvider("prov", "ns", fluxv1.ResourceSetInputProviderSpec{Type: fluxv1.InputProviderStatic})
	if rsip.Name != "prov" || rsip.Namespace != "ns" {
		t.Fatalf("unexpected metadata")
	}
	if rsip.TypeMeta.Kind != fluxv1.ResourceSetInputProviderKind {
		t.Errorf("unexpected kind %s", rsip.TypeMeta.Kind)
	}
}

func TestResourceSetInputProviderHelpers(t *testing.T) {
	rsip := CreateResourceSetInputProvider("prov", "ns", fluxv1.ResourceSetInputProviderSpec{})
	SetResourceSetInputProviderType(rsip, fluxv1.InputProviderGitHubBranch)
	SetResourceSetInputProviderURL(rsip, "https://example.com/repo")
	SetResourceSetInputProviderServiceAccountName(rsip, "sa")
	SetResourceSetInputProviderSecretRef(rsip, &meta.LocalObjectReference{Name: "secret"})
	SetResourceSetInputProviderCertSecretRef(rsip, &meta.LocalObjectReference{Name: "cert"})
	AddResourceSetInputProviderSchedule(rsip, CreateSchedule("@daily"))
	if rsip.Spec.Type != fluxv1.InputProviderGitHubBranch {
		t.Errorf("type not set")
	}
	if rsip.Spec.URL != "https://example.com/repo" {
		t.Errorf("url not set")
	}
	if rsip.Spec.ServiceAccountName != "sa" {
		t.Errorf("sa not set")
	}
	if rsip.Spec.SecretRef == nil || rsip.Spec.SecretRef.Name != "secret" {
		t.Errorf("secret ref not set")
	}
	if rsip.Spec.CertSecretRef == nil || rsip.Spec.CertSecretRef.Name != "cert" {
		t.Errorf("cert ref not set")
	}
	if len(rsip.Spec.Schedule) != 1 {
		t.Errorf("schedule not added")
	}
}
