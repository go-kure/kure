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
		IssuerRef: cmmeta.IssuerReference{
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
		Variant: &ACMEConfig{
			Server: "https://acme-v02.api.letsencrypt.org/directory",
			Email:  "admin@example.com",
			Solvers: []ACMESolverConfig{
				{Solver: &HTTP01SolverConfig{IngressClass: "nginx"}},
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
		Variant:   &CAConfig{SecretName: "ca-key-pair"},
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

func TestIssuer_NilVariantLeavesSpecEmpty(t *testing.T) {
	cfg := &IssuerConfig{Name: "no-variant", Namespace: "default"}
	issuer := Issuer(cfg)
	if issuer == nil {
		t.Fatal("expected non-nil Issuer")
	}
	if issuer.Spec.IssuerConfig.ACME != nil || issuer.Spec.IssuerConfig.CA != nil {
		t.Errorf("expected empty IssuerConfig, got: %+v", issuer.Spec.IssuerConfig)
	}
}

func TestIssuer_TypedNilVariantDoesNotPanic(t *testing.T) {
	var nilACME *ACMEConfig
	cfg := &IssuerConfig{Name: "n", Namespace: "ns", Variant: nilACME}
	issuer := Issuer(cfg)
	if issuer == nil {
		t.Fatal("nil result")
	}
	if issuer.Spec.IssuerConfig.ACME != nil {
		t.Errorf("typed-nil ACME should leave spec empty, got: %+v", issuer.Spec.IssuerConfig)
	}

	var nilCA *CAConfig
	cfg = &IssuerConfig{Name: "n", Namespace: "ns", Variant: nilCA}
	issuer = Issuer(cfg)
	if issuer.Spec.IssuerConfig.CA != nil {
		t.Errorf("typed-nil CA should leave spec empty, got: %+v", issuer.Spec.IssuerConfig)
	}
}

func TestClusterIssuer_CA(t *testing.T) {
	cfg := &ClusterIssuerConfig{
		Name:    "ca-cluster-issuer",
		Variant: &CAConfig{SecretName: "ca-key-pair"},
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
		Variant: &ACMEConfig{
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

func TestClusterIssuer_TypedNilVariantDoesNotPanic(t *testing.T) {
	var nilACME *ACMEConfig
	cfg := &ClusterIssuerConfig{Name: "n", Variant: nilACME}
	ci := ClusterIssuer(cfg)
	if ci == nil {
		t.Fatal("nil result")
	}
	if ci.Spec.IssuerConfig.ACME != nil {
		t.Errorf("typed-nil ACME should leave spec empty, got: %+v", ci.Spec.IssuerConfig)
	}
}

func TestBuildACMEIssuer_SkipsEmptySolver(t *testing.T) {
	cfg := &ACMEConfig{
		Server: "https://acme-v02.api.letsencrypt.org/directory",
		Email:  "admin@example.com",
		Solvers: []ACMESolverConfig{
			{}, // no Solver — should be skipped
		},
	}

	acme := buildACMEIssuer(cfg)

	if len(acme.Solvers) != 0 {
		t.Errorf("expected 0 solvers for invalid config, got %d", len(acme.Solvers))
	}
}

func TestBuildDNS01Solver_Cloudflare(t *testing.T) {
	token := cmmeta.SecretKeySelector{
		LocalObjectReference: cmmeta.LocalObjectReference{Name: "cloudflare-token"},
		Key:                  "api-token",
	}
	cfg := &DNS01SolverConfig{
		Provider: &CloudflareProviderConfig{
			Email:    "admin@example.com",
			APIToken: &token,
		},
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
		Provider: &Route53ProviderConfig{
			Region:          "us-east-1",
			SecretAccessKey: &key,
		},
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
		Provider: &GoogleProviderConfig{Project: "my-project"},
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

func TestBuildDNS01Solver_NilProvider(t *testing.T) {
	cfg := &DNS01SolverConfig{}
	solver := buildDNS01Solver(cfg)
	if solver.DNS01 != nil || solver.HTTP01 != nil {
		t.Error("expected empty solver when no provider is set")
	}
}

func TestBuildDNS01Solver_TypedNilProviderDoesNotPanic(t *testing.T) {
	var nilCloudflare *CloudflareProviderConfig
	cfg := &DNS01SolverConfig{Provider: nilCloudflare}
	solver := buildDNS01Solver(cfg)
	if solver.DNS01 != nil {
		t.Error("typed-nil Cloudflare should leave solver empty")
	}

	var nilRoute53 *Route53ProviderConfig
	cfg = &DNS01SolverConfig{Provider: nilRoute53}
	solver = buildDNS01Solver(cfg)
	if solver.DNS01 != nil {
		t.Error("typed-nil Route53 should leave solver empty")
	}

	var nilGoogle *GoogleProviderConfig
	cfg = &DNS01SolverConfig{Provider: nilGoogle}
	solver = buildDNS01Solver(cfg)
	if solver.DNS01 != nil {
		t.Error("typed-nil Google should leave solver empty")
	}
}

func TestBuildACMESolver_TypedNilSolverDoesNotPanic(t *testing.T) {
	var nilHTTP *HTTP01SolverConfig
	cfg := &ACMESolverConfig{Solver: nilHTTP}
	s := buildACMESolver(cfg)
	if s.HTTP01 != nil || s.DNS01 != nil {
		t.Error("typed-nil HTTP01 should leave solver empty")
	}

	var nilDNS *DNS01SolverConfig
	cfg = &ACMESolverConfig{Solver: nilDNS}
	s = buildACMESolver(cfg)
	if s.HTTP01 != nil || s.DNS01 != nil {
		t.Error("typed-nil DNS01 should leave solver empty")
	}
}

// Compile-time interface conformance checks.
var (
	_ IssuerVariant = (*ACMEConfig)(nil)
	_ IssuerVariant = (*CAConfig)(nil)
	_ ACMESolver    = (*HTTP01SolverConfig)(nil)
	_ ACMESolver    = (*DNS01SolverConfig)(nil)
	_ DNS01Provider = (*CloudflareProviderConfig)(nil)
	_ DNS01Provider = (*Route53ProviderConfig)(nil)
	_ DNS01Provider = (*GoogleProviderConfig)(nil)
)
