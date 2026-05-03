# Cert-Manager Builders - Certificate Resource Constructors

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/certmanager.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/certmanager)

The `certmanager` package provides strongly-typed constructor functions for creating cert-manager Kubernetes resources. These are the low-level building blocks used by Kure's higher-level stack and workflow packages.

## Overview

Each function takes a configuration struct and returns a fully initialized cert-manager custom resource. The builders handle API version and kind metadata, letting you focus on the resource specification.

## Supported Resources

### Certificate

```go
import "github.com/go-kure/kure/pkg/kubernetes/certmanager"

cert := certmanager.Certificate(&certmanager.CertificateConfig{
    Name:       "my-cert",
    Namespace:  "default",
    SecretName: "my-cert-tls",
    IssuerRef:  cmmeta.IssuerReference{Name: "letsencrypt", Kind: "ClusterIssuer"},
    DNSNames:   []string{"example.com", "www.example.com"},
})
```

### Issuer

`IssuerConfig.Variant` is a [sealed-interface sum type](/concepts/architecture/#one-of-constraints-sealed-interfaces): exactly one of `*ACMEConfig` or `*CAConfig` is permitted, enforced at compile time.

```go
issuer := certmanager.Issuer(&certmanager.IssuerConfig{
    Name:      "letsencrypt",
    Namespace: "default",
    Variant: &certmanager.ACMEConfig{
        Server: "https://acme-v02.api.letsencrypt.org/directory",
        Email:  "admin@example.com",
        Solvers: []certmanager.ACMESolverConfig{
            {Solver: &certmanager.HTTP01SolverConfig{IngressClass: "nginx"}},
        },
    },
})
```

`ACMESolverConfig.Solver` is also a sealed sum (`*HTTP01SolverConfig` or `*DNS01SolverConfig`). DNS-01 providers are likewise sealed:

```go
issuer = certmanager.Issuer(&certmanager.IssuerConfig{
    Name:      "letsencrypt-dns",
    Namespace: "default",
    Variant: &certmanager.ACMEConfig{
        Server: "https://acme-v02.api.letsencrypt.org/directory",
        Email:  "admin@example.com",
        Solvers: []certmanager.ACMESolverConfig{{
            Solver: &certmanager.DNS01SolverConfig{
                Provider: &certmanager.CloudflareProviderConfig{
                    Email:    "admin@example.com",
                    APIToken: &cmmeta.SecretKeySelector{
                        LocalObjectReference: cmmeta.LocalObjectReference{Name: "cf-api-token"},
                        Key:                  "api-token",
                    },
                },
            },
        }},
    },
})
```

### ClusterIssuer

```go
clusterIssuer := certmanager.ClusterIssuer(&certmanager.ClusterIssuerConfig{
    Name:    "letsencrypt-prod",
    Variant: &certmanager.CAConfig{SecretName: "ca-key-pair"},
})
```

## Modifier Functions

Update existing resources:

```go
// Update Certificate spec
certmanager.SetCertificateSpec(cert, newSpec)

// Add labels and annotations
certmanager.AddCertificateLabel(cert, "app", "my-app")
certmanager.AddIssuerAnnotation(issuer, "note", "production")

// Update issuer configuration
certmanager.SetIssuerACME(issuer, acmeConfig)
certmanager.SetIssuerCA(issuer, caConfig)
certmanager.SetClusterIssuerCA(clusterIssuer, caConfig)
```

## Related Packages

- [stack](/api-reference/stack/) - Domain model that produces Kubernetes resources
