package certmanager

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
)

// CertificateConfig describes a cert-manager Certificate resource.
type CertificateConfig struct {
	Name        string                 `yaml:"name"`
	Namespace   string                 `yaml:"namespace"`
	SecretName  string                 `yaml:"secretName"`
	IssuerRef   cmmeta.IssuerReference `yaml:"issuerRef"`
	DNSNames    []string               `yaml:"dnsNames,omitempty"`
	Duration    *metav1.Duration       `yaml:"duration,omitempty"`
	RenewBefore *metav1.Duration       `yaml:"renewBefore,omitempty"`
}

// IssuerConfig describes a cert-manager Issuer resource.
type IssuerConfig struct {
	Name      string      `yaml:"name"`
	Namespace string      `yaml:"namespace"`
	ACME      *ACMEConfig `yaml:"acme,omitempty"`
	CA        *CAConfig   `yaml:"ca,omitempty"`
}

// ClusterIssuerConfig describes a cert-manager ClusterIssuer resource.
type ClusterIssuerConfig struct {
	Name string      `yaml:"name"`
	ACME *ACMEConfig `yaml:"acme,omitempty"`
	CA   *CAConfig   `yaml:"ca,omitempty"`
}

// ACMEConfig describes the ACME issuer settings.
type ACMEConfig struct {
	Server     string                   `yaml:"server"`
	Email      string                   `yaml:"email"`
	PrivateKey cmmeta.SecretKeySelector `yaml:"privateKey"`
	Solvers    []ACMESolverConfig       `yaml:"solvers,omitempty"`
}

// ACMESolverConfig describes a single ACME challenge solver.
type ACMESolverConfig struct {
	HTTP01 *HTTP01SolverConfig `yaml:"http01,omitempty"`
	DNS01  *DNS01SolverConfig  `yaml:"dns01,omitempty"`
}

// HTTP01SolverConfig describes an HTTP-01 challenge solver.
type HTTP01SolverConfig struct {
	ServiceType  corev1.ServiceType `yaml:"serviceType,omitempty"`
	IngressClass string             `yaml:"ingressClass,omitempty"`
}

// DNS01SolverConfig describes a DNS-01 challenge solver.
type DNS01SolverConfig struct {
	Provider        string                    `yaml:"provider"`
	Email           string                    `yaml:"email,omitempty"`
	Region          string                    `yaml:"region,omitempty"`
	Project         string                    `yaml:"project,omitempty"`
	ServiceAccount  *cmmeta.SecretKeySelector `yaml:"serviceAccount,omitempty"`
	APIToken        *cmmeta.SecretKeySelector `yaml:"apiToken,omitempty"`
	SecretAccessKey *cmmeta.SecretKeySelector `yaml:"secretAccessKey,omitempty"`
}

// CAConfig describes a CA issuer configuration.
type CAConfig struct {
	SecretName string `yaml:"secretName"`
}
