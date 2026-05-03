package certmanager

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
)

// CertificateConfig describes a cert-manager Certificate resource.
type CertificateConfig struct {
	Name        string
	Namespace   string
	SecretName  string
	IssuerRef   cmmeta.IssuerReference
	DNSNames    []string
	Duration    *metav1.Duration
	RenewBefore *metav1.Duration
}

// IssuerVariant is a sealed interface implemented by exactly the per-variant
// config types valid for an Issuer or ClusterIssuer (ACMEConfig, CAConfig).
// The marker method is unexported so external packages cannot satisfy it.
type IssuerVariant interface {
	isIssuerVariant()
}

// IssuerConfig describes a cert-manager Issuer resource. Variant must hold
// exactly one of the implementing types; setting two variants is a compile
// error (single field).
type IssuerConfig struct {
	Name      string
	Namespace string
	Variant   IssuerVariant
}

// ClusterIssuerConfig describes a cert-manager ClusterIssuer resource. Variant
// must hold exactly one of the implementing types.
type ClusterIssuerConfig struct {
	Name    string
	Variant IssuerVariant
}

// ACMEConfig describes the ACME issuer settings.
type ACMEConfig struct {
	Server     string
	Email      string
	PrivateKey cmmeta.SecretKeySelector
	Solvers    []ACMESolverConfig
}

func (*ACMEConfig) isIssuerVariant() {}

// CAConfig describes a CA issuer configuration.
type CAConfig struct {
	SecretName string
}

func (*CAConfig) isIssuerVariant() {}

// ACMESolver is a sealed interface implemented by exactly the per-challenge
// solver config types valid for an ACME challenge (HTTP01SolverConfig,
// DNS01SolverConfig).
type ACMESolver interface {
	isACMESolver()
}

// ACMESolverConfig describes a single ACME challenge solver. Solver must hold
// exactly one of the implementing types.
type ACMESolverConfig struct {
	Solver ACMESolver
}

// HTTP01SolverConfig describes an HTTP-01 challenge solver.
type HTTP01SolverConfig struct {
	ServiceType  corev1.ServiceType
	IngressClass string
}

func (*HTTP01SolverConfig) isACMESolver() {}

// DNS01SolverConfig describes a DNS-01 challenge solver. Provider must hold
// exactly one provider implementation.
type DNS01SolverConfig struct {
	Provider DNS01Provider
}

func (*DNS01SolverConfig) isACMESolver() {}

// DNS01Provider is a sealed interface implemented by per-provider DNS-01
// solver configs (Cloudflare, Route53, Google CloudDNS).
type DNS01Provider interface {
	isDNS01Provider()
}

// CloudflareProviderConfig configures a Cloudflare DNS-01 solver.
type CloudflareProviderConfig struct {
	Email    string
	APIToken *cmmeta.SecretKeySelector
}

func (*CloudflareProviderConfig) isDNS01Provider() {}

// Route53ProviderConfig configures an AWS Route 53 DNS-01 solver.
type Route53ProviderConfig struct {
	Region          string
	SecretAccessKey *cmmeta.SecretKeySelector
}

func (*Route53ProviderConfig) isDNS01Provider() {}

// GoogleProviderConfig configures a Google Cloud DNS DNS-01 solver.
type GoogleProviderConfig struct {
	Project        string
	ServiceAccount *cmmeta.SecretKeySelector
}

func (*GoogleProviderConfig) isDNS01Provider() {}
