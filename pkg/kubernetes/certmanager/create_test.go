package certmanager

import (
	"testing"

	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
)

func TestCertificate_Success(t *testing.T) {
	cfg := &CertificateConfig{
		Name:       "test-cert",
		Namespace:  "default",
		SecretName: "test-cert-tls",
		IssuerRef: cmmeta.ObjectReference{
			Name: "letsencrypt",
			Kind: "ClusterIssuer",
		},
		DNSNames: []string{"example.com", "www.example.com"},
	}

	cert := Certificate(cfg)

	if cert == nil {
		t.Fatal("expected non-nil Certificate")
	}
	if cert.Name != "test-cert" {
		t.Errorf("expected Name 'test-cert', got %s", cert.Name)
	}
	if cert.Namespace != "default" {
		t.Errorf("expected Namespace 'default', got %s", cert.Namespace)
	}
	if cert.Spec.SecretName != "test-cert-tls" {
		t.Errorf("expected SecretName 'test-cert-tls', got %s", cert.Spec.SecretName)
	}
	if cert.Spec.IssuerRef.Name != "letsencrypt" {
		t.Errorf("expected IssuerRef.Name 'letsencrypt', got %s", cert.Spec.IssuerRef.Name)
	}
	if len(cert.Spec.DNSNames) != 2 {
		t.Fatalf("expected 2 DNS names, got %d", len(cert.Spec.DNSNames))
	}
	if cert.Spec.DNSNames[0] != "example.com" {
		t.Errorf("expected first DNS name 'example.com', got %s", cert.Spec.DNSNames[0])
	}
}

func TestCertificate_NilConfig(t *testing.T) {
	cert := Certificate(nil)
	if cert != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestIssuer_ACME(t *testing.T) {
	cfg := &IssuerConfig{
		Name:      "letsencrypt",
		Namespace: "cert-manager",
		ACME: &ACMEConfig{
			Server: "https://acme-v02.api.letsencrypt.org/directory",
			Email:  "admin@example.com",
			Solvers: []ACMESolverConfig{
				{HTTP01: &HTTP01SolverConfig{IngressClass: "nginx"}},
			},
		},
	}

	issuer := Issuer(cfg)

	if issuer == nil {
		t.Fatal("expected non-nil Issuer")
	}
	if issuer.Name != "letsencrypt" {
		t.Errorf("expected Name 'letsencrypt', got %s", issuer.Name)
	}
	if issuer.Namespace != "cert-manager" {
		t.Errorf("expected Namespace 'cert-manager', got %s", issuer.Namespace)
	}
	if issuer.Spec.IssuerConfig.ACME == nil {
		t.Fatal("expected non-nil ACME config")
	}
	if issuer.Spec.IssuerConfig.ACME.Server != "https://acme-v02.api.letsencrypt.org/directory" {
		t.Errorf("expected ACME server URL, got %s", issuer.Spec.IssuerConfig.ACME.Server)
	}
	if len(issuer.Spec.IssuerConfig.ACME.Solvers) != 1 {
		t.Fatalf("expected 1 solver, got %d", len(issuer.Spec.IssuerConfig.ACME.Solvers))
	}
}

func TestIssuer_CA(t *testing.T) {
	cfg := &IssuerConfig{
		Name:      "ca-issuer",
		Namespace: "default",
		CA:        &CAConfig{SecretName: "ca-key-pair"},
	}

	issuer := Issuer(cfg)

	if issuer == nil {
		t.Fatal("expected non-nil Issuer")
	}
	if issuer.Spec.IssuerConfig.CA == nil {
		t.Fatal("expected non-nil CA config")
	}
	if issuer.Spec.IssuerConfig.CA.SecretName != "ca-key-pair" {
		t.Errorf("expected CA SecretName 'ca-key-pair', got %s", issuer.Spec.IssuerConfig.CA.SecretName)
	}
}

func TestIssuer_NilConfig(t *testing.T) {
	issuer := Issuer(nil)
	if issuer != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestClusterIssuer_CA(t *testing.T) {
	cfg := &ClusterIssuerConfig{
		Name: "ca-cluster-issuer",
		CA:   &CAConfig{SecretName: "ca-key-pair"},
	}

	ci := ClusterIssuer(cfg)

	if ci == nil {
		t.Fatal("expected non-nil ClusterIssuer")
	}
	if ci.Name != "ca-cluster-issuer" {
		t.Errorf("expected Name 'ca-cluster-issuer', got %s", ci.Name)
	}
	if ci.Spec.IssuerConfig.CA == nil {
		t.Fatal("expected non-nil CA config")
	}
	if ci.Spec.IssuerConfig.CA.SecretName != "ca-key-pair" {
		t.Errorf("expected CA SecretName 'ca-key-pair', got %s", ci.Spec.IssuerConfig.CA.SecretName)
	}
}

func TestClusterIssuer_ACME(t *testing.T) {
	cfg := &ClusterIssuerConfig{
		Name: "letsencrypt-prod",
		ACME: &ACMEConfig{
			Server: "https://acme-v02.api.letsencrypt.org/directory",
			Email:  "admin@example.com",
		},
	}

	ci := ClusterIssuer(cfg)

	if ci == nil {
		t.Fatal("expected non-nil ClusterIssuer")
	}
	if ci.Spec.IssuerConfig.ACME == nil {
		t.Fatal("expected non-nil ACME config")
	}
	if ci.Spec.IssuerConfig.ACME.Server != "https://acme-v02.api.letsencrypt.org/directory" {
		t.Errorf("expected ACME server URL, got %s", ci.Spec.IssuerConfig.ACME.Server)
	}
}

func TestClusterIssuer_NilConfig(t *testing.T) {
	ci := ClusterIssuer(nil)
	if ci != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestBuildDNS01Solver_Cloudflare(t *testing.T) {
	token := cmmeta.SecretKeySelector{
		LocalObjectReference: cmmeta.LocalObjectReference{Name: "cloudflare-token"},
		Key:                  "api-token",
	}
	cfg := &DNS01SolverConfig{
		Provider: "cloudflare",
		Email:    "admin@example.com",
		APIToken: &token,
	}

	solver := buildDNS01Solver(cfg)

	if solver.DNS01 == nil {
		t.Fatal("expected non-nil DNS01 solver")
	}
	if solver.DNS01.Cloudflare == nil {
		t.Fatal("expected non-nil Cloudflare config")
	}
	if solver.DNS01.Cloudflare.Email != "admin@example.com" {
		t.Errorf("expected Email 'admin@example.com', got %s", solver.DNS01.Cloudflare.Email)
	}
}

func TestBuildDNS01Solver_Route53(t *testing.T) {
	key := cmmeta.SecretKeySelector{
		LocalObjectReference: cmmeta.LocalObjectReference{Name: "aws-secret"},
		Key:                  "secret-access-key",
	}
	cfg := &DNS01SolverConfig{
		Provider:        "route53",
		Region:          "us-east-1",
		SecretAccessKey: &key,
	}

	solver := buildDNS01Solver(cfg)

	if solver.DNS01 == nil {
		t.Fatal("expected non-nil DNS01 solver")
	}
	if solver.DNS01.Route53 == nil {
		t.Fatal("expected non-nil Route53 config")
	}
	if solver.DNS01.Route53.Region != "us-east-1" {
		t.Errorf("expected Region 'us-east-1', got %s", solver.DNS01.Route53.Region)
	}
}

func TestBuildDNS01Solver_CloudDNS(t *testing.T) {
	cfg := &DNS01SolverConfig{
		Provider: "clouddns",
		Project:  "my-project",
	}

	solver := buildDNS01Solver(cfg)

	if solver.DNS01 == nil {
		t.Fatal("expected non-nil DNS01 solver")
	}
	if solver.DNS01.CloudDNS == nil {
		t.Fatal("expected non-nil CloudDNS config")
	}
	if solver.DNS01.CloudDNS.Project != "my-project" {
		t.Errorf("expected Project 'my-project', got %s", solver.DNS01.CloudDNS.Project)
	}
}

func TestBuildDNS01Solver_UnknownProvider(t *testing.T) {
	cfg := &DNS01SolverConfig{
		Provider: "unknown",
	}

	solver := buildDNS01Solver(cfg)

	if solver.DNS01 != nil || solver.HTTP01 != nil {
		t.Error("expected empty solver for unknown provider")
	}
}
