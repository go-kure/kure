# Cert-Manager Builders - Certificate Resource Constructors

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/certmanager.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/certmanager)

The `certmanager` package provides the generated constructors and the admissible sugar for cert-manager Kubernetes resources. These are the low-level building blocks used by Kure's higher-level stack and workflow packages.

## Overview

Every kind is built the same way: the generated `Create<Kind>` wrapper gives you an object carrying identity only, and the upstream cert-manager struct is the construction API — set `Spec` fields directly, or through the few `Set*`/`Add*` helpers the builder contract admits.

## Constructors

Every kind this package registers has a generated `Create<Kind>` wrapper in `zz_generated_create.go`, produced from the scheme by `pkg/kubernetes/internal/gen` (`make gen-builders`, checked by `make check-builders` in CI). A wrapper delegates to `kubernetes.Create[T]` and emits **TypeMeta and identity only**: no default, no label, no spec value. Namespaced kinds take `(name, namespace)`, cluster-scoped kinds take `(name)`.

```go
obj := certmanager.CreateCertificate("my-cert", "default")
cl := certmanager.CreateClusterIssuer("letsencrypt-prod")
```

There is no second construction path. <!-- doc-api-refs:ignore names the retired config-struct layer --> The config-struct layer this package used to carry (`certmanager.Certificate(&certmanager.CertificateConfig{...})`, `Issuer`, `ClusterIssuer`, and the sealed `IssuerVariant` / `ACMESolver` / `DNS01Provider` sums behind them) was retired by release 2 of the builder contract: it reached 5 of a `Certificate`'s 24 spec fields and 2 of an issuer's 5 arms, and a consumer could not add a Vault issuer or an Azure DNS solver even by hand. The [release 2 migration notes](/concepts/builder-contract-release-2/) map every removed field to the upstream field that replaces it. No hand-written `Create*` helper for a spec fragment remains either — a sub-type that is not a `client.Object` takes a struct literal, which is shorter and shows every field being set.

The kinds this package registers, their scope, and what stated that scope are rows in the generated [Supported kinds and field maturity](/api-reference/api-tables/) tables. The sections below are worked examples, not the coverage list.

See the [Kubernetes Builders](/api-reference/kubernetes-builders/) page for the full builder contract: construction, sugar admission classes, purity and the migration ledgers.

## Supported Resources

### Certificate

```go
import (
    certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
    cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"

    "github.com/go-kure/kure/pkg/kubernetes/certmanager"
)

cert := certmanager.CreateCertificate("my-cert", "default")
cert.Spec = certv1.CertificateSpec{
    SecretName: "my-cert-tls",
    IssuerRef:  cmmeta.IssuerReference{Name: "letsencrypt", Kind: "ClusterIssuer"},
    DNSNames:   []string{"example.com", "www.example.com"},
    Duration:   &metav1.Duration{Duration: 2160 * time.Hour},
}
```

Every field of `certv1.CertificateSpec` is reachable this way — `PrivateKey`, `Usages`, `Subject`, `Keystores`, the lot — because the spec is the upstream struct, not a copy of it.

### Issuer

`certv1.IssuerSpec` embeds the upstream one-of, `IssuerConfig`, with a pointer per arm (`ACME`, `CA`, `Vault`, `SelfSigned`, `Venafi`). Set exactly one; cert-manager rejects an issuer with two at apply time. The two most common arms have a pointer setter:

```go
import cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"

issuer := certmanager.CreateIssuer("letsencrypt", "default")
certmanager.SetIssuerACME(issuer, &cmacme.ACMEIssuer{
    Server: "https://acme-v02.api.letsencrypt.org/directory",
    Email:  "admin@example.com",
    PrivateKey: cmmeta.SecretKeySelector{
        LocalObjectReference: cmmeta.LocalObjectReference{Name: "letsencrypt-account-key"},
    },
    Solvers: []cmacme.ACMEChallengeSolver{{
        HTTP01: &cmacme.ACMEChallengeSolverHTTP01{
            Ingress: &cmacme.ACMEChallengeSolverHTTP01Ingress{IngressClassName: ptr.To("nginx")},
        },
    }},
})
```

A DNS-01 solver is the same shape with a provider set on `ACMEChallengeSolverDNS01` — Cloudflare, Route 53, Google CloudDNS, and every other provider cert-manager knows (Azure DNS, DigitalOcean, RFC 2136, AcmeDNS, a webhook). `AddACMEIssuerSolver` appends one solver at a time when the list is assembled incrementally:

```go
acme := &cmacme.ACMEIssuer{Server: "https://acme-v02.api.letsencrypt.org/directory", Email: "admin@example.com"}
certmanager.AddACMEIssuerSolver(acme, cmacme.ACMEChallengeSolver{
    DNS01: &cmacme.ACMEChallengeSolverDNS01{
        Cloudflare: &cmacme.ACMEIssuerDNS01ProviderCloudflare{
            Email: "admin@example.com",
            APIToken: &cmmeta.SecretKeySelector{
                LocalObjectReference: cmmeta.LocalObjectReference{Name: "cf-api-token"},
                Key:                  "api-token",
            },
        },
    },
})
certmanager.SetIssuerACME(issuer, acme)
```

The other arms are plain pointer assignments on the embedded struct:

```go
issuer.Spec.Vault = &certv1.VaultIssuer{Server: "https://vault.example.com", Path: "pki/sign/example"}
issuer.Spec.SelfSigned = &certv1.SelfSignedIssuer{}
```

### ClusterIssuer

```go
clusterIssuer := certmanager.CreateClusterIssuer("letsencrypt-prod")
certmanager.SetClusterIssuerCA(clusterIssuer, &certv1.CAIssuer{SecretName: "ca-key-pair"})
```

## Modifier Functions

Update existing resources:

```go
// Replace the whole Certificate spec
cert.Spec = newSpec

// Append to the DNS names, set the pointer-typed durations
certmanager.AddCertificateDNSName(cert, "alt.example.com")
certmanager.SetCertificateDuration(cert, &metav1.Duration{Duration: 2160 * time.Hour})
certmanager.SetCertificateRenewBefore(cert, &metav1.Duration{Duration: 360 * time.Hour})

// Labels and annotations use the generic helpers, which work over any object
// with ObjectMeta -- this package carries no per-kind metadata helpers
kubernetes.AddLabel(cert, "app", "my-app")
kubernetes.AddAnnotation(issuer, "note", "production")

// Update issuer configuration
certmanager.SetIssuerACME(issuer, acmeConfig)
certmanager.SetIssuerCA(issuer, caConfig)
certmanager.SetClusterIssuerACME(clusterIssuer, acmeConfig)
certmanager.SetClusterIssuerCA(clusterIssuer, caConfig)
```

## Related Packages

- [stack](/api-reference/stack/) - Domain model that produces Kubernetes resources
- [Builder contract: release 2 migration notes](/concepts/builder-contract-release-2/) - the retired config-struct layer, field by field
