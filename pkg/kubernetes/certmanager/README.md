# Cert-Manager Builders - Certificate Resource Constructors

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/certmanager.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/certmanager)

The `certmanager` package provides strongly-typed constructor functions for creating cert-manager Kubernetes resources. These are the low-level building blocks used by Kure's higher-level stack and workflow packages.

## Overview

Each config-struct builder takes a configuration struct and returns a populated cert-manager custom resource. The builders handle API version and kind metadata, letting you focus on the resource specification.

## Constructors

Every kind this package registers has a generated `Create<Kind>` wrapper in `zz_generated_create.go`, produced from the scheme by `pkg/kubernetes/internal/gen` (`make gen-builders`, checked by `make check-builders` in CI). A wrapper delegates to `kubernetes.Create[T]` and emits **TypeMeta and identity only**: no default, no label, no spec value. Namespaced kinds take `(name, namespace)`, cluster-scoped kinds take `(name)`. The upstream struct is the construction API; set spec fields directly or through the admissible `Set*`/`Add*` sugar below.

```go
obj := certmanager.CreateCertificate("my-cert", "default")
cl := certmanager.CreateClusterIssuer("letsencrypt-prod")
```

The config-struct builders (`certmanager.Certificate(&certmanager.CertificateConfig{...})`) are a separate, opinionated layer on top of the same upstream types; they are unchanged by the generated constructors. No hand-written `Create*` helper for a spec fragment remains — a sub-type that is not a `client.Object` takes a struct literal, which is shorter and shows every field being set.

The kinds this package registers, their scope, and what stated that scope are rows in the generated [Supported kinds and field maturity](/api-reference/api-tables/) tables. The sections below are worked examples, not the coverage list.

See the [Kubernetes Builders](/api-reference/kubernetes-builders/) page for the full builder contract: construction, sugar admission classes, purity and the release-1 migration ledger.

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
cert.Spec = newSpec

// Labels and annotations use the generic helpers, which work over any object
// with ObjectMeta -- this package carries no per-kind metadata helpers
kubernetes.AddLabel(cert, "app", "my-app")
kubernetes.AddAnnotation(issuer, "note", "production")

// Update issuer configuration
certmanager.SetIssuerACME(issuer, acmeConfig)
certmanager.SetIssuerCA(issuer, caConfig)
certmanager.SetClusterIssuerCA(clusterIssuer, caConfig)
```

## Related Packages

- [stack](/api-reference/stack/) - Domain model that produces Kubernetes resources
