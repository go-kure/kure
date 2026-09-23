# Builder contract: release 2 migration notes

This page is the ledger for the second release of the builder contract (ADR-038,
the "thin core + admissible sugar" decision). Release 1 made the upstream Go struct
the construction API and removed the bare forwarders and injected defaults; its
ledger is the [release 1 migration notes](/concepts/builder-contract-release-1/).
Release 2 removes the one construction path that survived it: the `Kind(&Config)`
layer — a function per kind taking a kure-invented `Config` struct and translating
it into the upstream spec — in the six packages that still carried one, together
with the sealed-interface sum types and the values that layer used to inject. The
normative contract text is the [Kubernetes Builders](/api-reference/kubernetes-builders/)
page.

Every removed function, type and field is listed here once, with the upstream
expression that replaces it.

## Why the layer went

In four of the six packages the `Config` was lossy by construction: the vocabulary
reached 5 of a cert-manager `Certificate`'s 24 spec fields, 2 of an issuer's 5 arms
(no Vault, SelfSigned or Venafi), 15 of a CNPG `Cluster`'s 55, 6 of a
`ServiceMonitor`'s 19 — and every upstream release widened the gap. In the other two
(`cilium`, `volsync`) it was a verbatim copy of `obj.Spec = spec`. cert-manager's
`IssuerVariant`, `ACMESolver` and `DNS01Provider` and volsync's `SourceMover` and
`DestinationMover` were interfaces closed by an unexported marker method — the only
compile-time wall in kure: a caller could not add an Azure DNS solver or a Vault
issuer even by hand. The layer also held the last injected values the release 1
purge deferred to it (CNPG `enablePDB`, `primaryUpdateStrategy`, S3 key names, the
barman-cloud plugin entry, the pooler type coercion).

One idiom remains: the generated `Create<Kind>` constructor plus the upstream struct.

## The rule that replaces it

> No exported function under `pkg/kubernetes/...` takes a kure-defined type where
> an upstream spec type exists.

`TestAdmission_NoOwnParameterTypes` (`pkg/kubernetes/admission_params_test.go`)
walks every exported top-level function in the non-test, non-generated files and
fails on any parameter whose type is a struct or interface declared under
`pkg/kubernetes` — reached directly or through pointer, slice, array or map layers.
There is no exclusion list. A named scalar such as `kubernetes.PSALevel` (a string
enum with no upstream spec type) is not a spec type and passes; a defined type over
an upstream struct (volsync's former `SourceResticConfig`) does not.

## How a call site migrates

Every `Kind(&KindConfig{Name: n, Namespace: ns, ...})` becomes the generated
constructor plus assignments on the upstream struct:

```go
// before
cert := certmanager.Certificate(&certmanager.CertificateConfig{
    Name: "api-tls", Namespace: "default",
    SecretName: "api-tls-secret",
    DNSNames:   []string{"api.example.com"},
})

// after
cert := certmanager.CreateCertificate("api-tls", "default")
cert.Spec = certv1.CertificateSpec{
    SecretName: "api-tls-secret",
    DNSNames:   []string{"api.example.com"},
}
```

Cluster-scoped kinds take `(name)` only. Two behaviours of the layer have no
equivalent and need no replacement: a `nil` `*Config` returned a `nil` object
(constructors never return nil), and a typed-nil variant stored in a sealed
interface was treated as "no variant" (there is no interface to store it in — an
unset pointer arm is unset).

The golden fixtures under each package's `testdata/` were written by the layer as it
stood before this release; the tests that now produce them use `Create<Kind>` and
the upstream struct and reproduce that output byte for byte, including every value
the layer used to inject. Those values are the caller's own lines now, and each
package section below lists them.

## `pkg/kubernetes/certmanager`

### Removed functions (3)

| Removed | Replacement |
|---|---|
| `Certificate(&CertificateConfig{...})` | `CreateCertificate(name, namespace)` + `cert.Spec = certv1.CertificateSpec{...}` |
| `Issuer(&IssuerConfig{...})` | `CreateIssuer(name, namespace)` + `SetIssuerACME(issuer, acme)` / `SetIssuerCA(issuer, ca)` |
| `ClusterIssuer(&ClusterIssuerConfig{...})` | `CreateClusterIssuer(name)` + `SetClusterIssuerACME(ci, acme)` / `SetClusterIssuerCA(ci, ca)` |

### Removed types (14) and their fields

The sealed interfaces `IssuerVariant`, `ACMESolver` and `DNS01Provider` are gone
with the eleven structs that implemented or carried them. The upstream one-of is
`certv1.IssuerConfig`, embedded in `IssuerSpec` (the `Spec` type of both `Issuer` and `ClusterIssuer`), with a
pointer per arm: `ACME`, `CA`, `Vault`, `SelfSigned`, `Venafi`.

| Removed field | Upstream field |
|---|---|
| `CertificateConfig.Name`, `.Namespace` | `CreateCertificate(name, namespace)` |
| `CertificateConfig.SecretName` | `cert.Spec.SecretName` |
| `CertificateConfig.IssuerRef` | `cert.Spec.IssuerRef` |
| `CertificateConfig.DNSNames` | `cert.Spec.DNSNames`, or `AddCertificateDNSName(cert, dns)` per name |
| `CertificateConfig.Duration` | `cert.Spec.Duration`, or `SetCertificateDuration(cert, d)` |
| `CertificateConfig.RenewBefore` | `cert.Spec.RenewBefore`, or `SetCertificateRenewBefore(cert, d)` |
| `IssuerConfig.Name`, `.Namespace` | `CreateIssuer(name, namespace)` |
| `IssuerConfig.Variant` = `*ACMEConfig` | `SetIssuerACME(issuer, &cmacme.ACMEIssuer{...})` |
| `IssuerConfig.Variant` = `*CAConfig` | `SetIssuerCA(issuer, &certv1.CAIssuer{...})` |
| `ClusterIssuerConfig.Name` | `CreateClusterIssuer(name)` |
| `ClusterIssuerConfig.Variant` | `SetClusterIssuerACME(ci, acme)` / `SetClusterIssuerCA(ci, ca)` |
| `ACMEConfig.Server` | `cmacme.ACMEIssuer.Server` |
| `ACMEConfig.Email` | `cmacme.ACMEIssuer.Email` |
| `ACMEConfig.PrivateKey` | `cmacme.ACMEIssuer.PrivateKey` |
| `ACMEConfig.Solvers []ACMESolverConfig` | `cmacme.ACMEIssuer.Solvers []cmacme.ACMEChallengeSolver`, or `AddACMEIssuerSolver(acme, solver)` per solver |
| `CAConfig.SecretName` | `certv1.CAIssuer.SecretName` |
| `ACMESolverConfig.Solver` = `*HTTP01SolverConfig` | `cmacme.ACMEChallengeSolver.HTTP01 = &cmacme.ACMEChallengeSolverHTTP01{Ingress: &cmacme.ACMEChallengeSolverHTTP01Ingress{...}}` |
| `ACMESolverConfig.Solver` = `*DNS01SolverConfig` | `cmacme.ACMEChallengeSolver.DNS01 = &cmacme.ACMEChallengeSolverDNS01{...}` |
| `HTTP01SolverConfig.ServiceType` | `cmacme.ACMEChallengeSolverHTTP01Ingress.ServiceType` |
| `HTTP01SolverConfig.IngressClass string` | `cmacme.ACMEChallengeSolverHTTP01Ingress.IngressClassName *string` — `ptr.To(class)`; leave nil to omit |
| `DNS01SolverConfig.Provider` = `*CloudflareProviderConfig` | `cmacme.ACMEChallengeSolverDNS01.Cloudflare = &cmacme.ACMEIssuerDNS01ProviderCloudflare{...}` |
| `DNS01SolverConfig.Provider` = `*Route53ProviderConfig` | `cmacme.ACMEChallengeSolverDNS01.Route53 = &cmacme.ACMEIssuerDNS01ProviderRoute53{...}` |
| `DNS01SolverConfig.Provider` = `*GoogleProviderConfig` | `cmacme.ACMEChallengeSolverDNS01.CloudDNS = &cmacme.ACMEIssuerDNS01ProviderCloudDNS{...}` |
| `CloudflareProviderConfig.Email` | `cmacme.ACMEIssuerDNS01ProviderCloudflare.Email` |
| `CloudflareProviderConfig.APIToken` | `cmacme.ACMEIssuerDNS01ProviderCloudflare.APIToken` |
| `Route53ProviderConfig.Region` | `cmacme.ACMEIssuerDNS01ProviderRoute53.Region` |
| `Route53ProviderConfig.SecretAccessKey *cmmeta.SecretKeySelector` | `cmacme.ACMEIssuerDNS01ProviderRoute53.SecretAccessKey cmmeta.SecretKeySelector` (a value; dereference) |
| `GoogleProviderConfig.Project` | `cmacme.ACMEIssuerDNS01ProviderCloudDNS.Project` |
| `GoogleProviderConfig.ServiceAccount` | `cmacme.ACMEIssuerDNS01ProviderCloudDNS.ServiceAccount` |

### Behaviour of the layer that a struct literal does not have

None of these was documented as a feature; each is listed so a caller relying on
it can see what to write instead.

| The layer used to | Now |
|---|---|
| Leave `ingressClassName` out when `IngressClass` was `""` | `IngressClassName` is a `*string`; nil omits it, `ptr.To("")` writes an empty class |
| Drop a solver with neither `HTTP01` nor `DNS01` set | Nothing is dropped; an empty `ACMEChallengeSolver{}` serialises as `{}` |
| Drop a Cloudflare provider whose `APIToken` was nil, and a Route 53 provider whose `SecretAccessKey` was nil | Nothing is dropped; cert-manager validates the reference at apply time |
| Emit `privateKeySecretRef: {name: ""}` for an ACME issuer with no `PrivateKey` | Unchanged — `cmacme.ACMEIssuer.PrivateKey` is a value field with no `omitempty` upstream, so the literal emits the same |

Golden deltas: none. Every fixture in `pkg/kubernetes/certmanager/testdata` is
reproduced byte for byte.
