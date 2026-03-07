package certmanager

import (
	"testing"

	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetCertificateSpec(t *testing.T) {
	cfg := &CertificateConfig{
		Name:       "test-cert",
		Namespace:  "default",
		SecretName: "test-tls",
		IssuerRef:  cmmeta.ObjectReference{Name: "issuer"},
	}

	cert := Certificate(cfg)
	if cert == nil {
		t.Fatal("failed to create Certificate")
	}

	newSpec := certv1.CertificateSpec{
		SecretName: "new-secret",
	}

	SetCertificateSpec(cert, newSpec)

	if cert.Spec.SecretName != "new-secret" {
		t.Errorf("expected SecretName 'new-secret', got %s", cert.Spec.SecretName)
	}
}

func TestSetIssuerSpec(t *testing.T) {
	cfg := &IssuerConfig{
		Name:      "test-issuer",
		Namespace: "default",
	}

	issuer := Issuer(cfg)
	if issuer == nil {
		t.Fatal("failed to create Issuer")
	}

	newSpec := certv1.IssuerSpec{}
	newSpec.CA = &certv1.CAIssuer{SecretName: "new-ca"}

	SetIssuerSpec(issuer, newSpec)

	if issuer.Spec.CA == nil || issuer.Spec.CA.SecretName != "new-ca" {
		t.Error("expected CA SecretName 'new-ca'")
	}
}

func TestSetClusterIssuerSpec(t *testing.T) {
	cfg := &ClusterIssuerConfig{
		Name: "test-cluster-issuer",
	}

	ci := ClusterIssuer(cfg)
	if ci == nil {
		t.Fatal("failed to create ClusterIssuer")
	}

	newSpec := certv1.IssuerSpec{}
	newSpec.CA = &certv1.CAIssuer{SecretName: "cluster-ca"}

	SetClusterIssuerSpec(ci, newSpec)

	if ci.Spec.CA == nil || ci.Spec.CA.SecretName != "cluster-ca" {
		t.Error("expected CA SecretName 'cluster-ca'")
	}
}

func TestAddCertificateLabel(t *testing.T) {
	cfg := &CertificateConfig{
		Name:       "test-cert",
		Namespace:  "default",
		SecretName: "tls",
		IssuerRef:  cmmeta.ObjectReference{Name: "issuer"},
	}
	cert := Certificate(cfg)

	if err := AddCertificateLabel(cert, "app", "test"); err != nil {
		t.Errorf("AddCertificateLabel failed: %v", err)
	}
	if cert.Labels["app"] != "test" {
		t.Error("expected label 'app' to be 'test'")
	}
}

func TestAddCertificateAnnotation(t *testing.T) {
	cfg := &CertificateConfig{
		Name:       "test-cert",
		Namespace:  "default",
		SecretName: "tls",
		IssuerRef:  cmmeta.ObjectReference{Name: "issuer"},
	}
	cert := Certificate(cfg)

	if err := AddCertificateAnnotation(cert, "note", "value"); err != nil {
		t.Errorf("AddCertificateAnnotation failed: %v", err)
	}
	if cert.Annotations["note"] != "value" {
		t.Error("expected annotation 'note' to be 'value'")
	}
}

func TestSetCertificateDuration(t *testing.T) {
	cfg := &CertificateConfig{
		Name:       "test-cert",
		Namespace:  "default",
		SecretName: "tls",
		IssuerRef:  cmmeta.ObjectReference{Name: "issuer"},
	}
	cert := Certificate(cfg)

	dur := &metav1.Duration{Duration: 720 * 3600_000_000_000} // 720h
	if err := SetCertificateDuration(cert, dur); err != nil {
		t.Errorf("SetCertificateDuration failed: %v", err)
	}
}

func TestSetIssuerACME(t *testing.T) {
	cfg := &IssuerConfig{
		Name:      "test-issuer",
		Namespace: "default",
	}
	issuer := Issuer(cfg)

	acme := &cmacme.ACMEIssuer{Server: "https://acme.example.com"}
	if err := SetIssuerACME(issuer, acme); err != nil {
		t.Errorf("SetIssuerACME failed: %v", err)
	}
	if issuer.Spec.IssuerConfig.ACME == nil || issuer.Spec.IssuerConfig.ACME.Server != "https://acme.example.com" {
		t.Error("expected ACME config to be set")
	}
}

func TestSetIssuerCA(t *testing.T) {
	cfg := &IssuerConfig{
		Name:      "test-issuer",
		Namespace: "default",
	}
	issuer := Issuer(cfg)

	ca := &certv1.CAIssuer{SecretName: "ca-secret"}
	if err := SetIssuerCA(issuer, ca); err != nil {
		t.Errorf("SetIssuerCA failed: %v", err)
	}
	if issuer.Spec.IssuerConfig.CA == nil || issuer.Spec.IssuerConfig.CA.SecretName != "ca-secret" {
		t.Error("expected CA config to be set")
	}
}

func TestSetClusterIssuerACME(t *testing.T) {
	cfg := &ClusterIssuerConfig{
		Name: "test-cluster-issuer",
	}
	ci := ClusterIssuer(cfg)

	acme := &cmacme.ACMEIssuer{Server: "https://acme.example.com"}
	if err := SetClusterIssuerACME(ci, acme); err != nil {
		t.Errorf("SetClusterIssuerACME failed: %v", err)
	}
	if ci.Spec.IssuerConfig.ACME == nil || ci.Spec.IssuerConfig.ACME.Server != "https://acme.example.com" {
		t.Error("expected ACME config to be set")
	}
}

func TestSetClusterIssuerCA(t *testing.T) {
	cfg := &ClusterIssuerConfig{
		Name: "test-cluster-issuer",
	}
	ci := ClusterIssuer(cfg)

	ca := &certv1.CAIssuer{SecretName: "ca-secret"}
	if err := SetClusterIssuerCA(ci, ca); err != nil {
		t.Errorf("SetClusterIssuerCA failed: %v", err)
	}
	if ci.Spec.IssuerConfig.CA == nil || ci.Spec.IssuerConfig.CA.SecretName != "ca-secret" {
		t.Error("expected CA config to be set")
	}
}

func TestAddIssuerLabel(t *testing.T) {
	cfg := &IssuerConfig{
		Name:      "test-issuer",
		Namespace: "default",
	}
	issuer := Issuer(cfg)

	if err := AddIssuerLabel(issuer, "env", "prod"); err != nil {
		t.Errorf("AddIssuerLabel failed: %v", err)
	}
	if issuer.Labels["env"] != "prod" {
		t.Error("expected label 'env' to be 'prod'")
	}
}

func TestAddClusterIssuerLabel(t *testing.T) {
	cfg := &ClusterIssuerConfig{
		Name: "test-cluster-issuer",
	}
	ci := ClusterIssuer(cfg)

	if err := AddClusterIssuerLabel(ci, "env", "prod"); err != nil {
		t.Errorf("AddClusterIssuerLabel failed: %v", err)
	}
	if ci.Labels["env"] != "prod" {
		t.Error("expected label 'env' to be 'prod'")
	}
}
