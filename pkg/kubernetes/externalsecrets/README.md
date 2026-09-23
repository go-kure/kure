# External Secrets Builders - External Secrets Resource Constructors

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/externalsecrets.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/externalsecrets)

The `externalsecrets` package provides the generated constructors and the admissible sugar for External Secrets Operator Kubernetes resources. These are the low-level building blocks used by Kure's higher-level stack and workflow packages.

## Overview

Every kind is built the same way: the generated `Create<Kind>` wrapper gives you an object carrying identity only, and the upstream external-secrets struct is the construction API — set `Spec` fields directly, or through the few `Set*`/`Add*` helpers the builder contract admits.

## Constructors

Every kind this package registers has a generated `Create<Kind>` wrapper in `zz_generated_create.go`, produced from the scheme by `pkg/kubernetes/internal/gen` (`make gen-builders`, checked by `make check-builders` in CI). A wrapper delegates to `kubernetes.Create[T]` and emits **TypeMeta and identity only**: no default, no label, no spec value. Namespaced kinds take `(name, namespace)`, cluster-scoped kinds take `(name)`.

```go
obj := externalsecrets.CreateExternalSecret("my-secret", "default")
cl := externalsecrets.CreateClusterSecretStore("global-vault")
```

There is no second construction path. The config-struct layer this package used to carry (`externalsecrets.ExternalSecret(&externalsecrets.ExternalSecretConfig{...})`, `SecretStore`, `ClusterSecretStore`) was retired by release 2 of the builder contract: it reached 5 of an `ExternalSecret`'s 7 spec fields and was otherwise a copy of the assignments below. The [release 2 migration notes](/concepts/builder-contract-release-2/) map every removed field to the upstream field that replaces it. No hand-written `Create*` helper for a spec fragment remains either — a sub-type that is not a `client.Object` takes a struct literal, which is shorter and shows every field being set.

The kinds this package registers, their scope, and what stated that scope are rows in the generated [Supported kinds and field maturity](/api-reference/api-tables/) tables. The sections below are worked examples, not the coverage list.

See the [Kubernetes Builders](/api-reference/kubernetes-builders/) page for the full builder contract: construction, sugar admission classes, purity and the migration ledgers.

## Supported Resources

### External Secrets

```go
import (
    esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"

    "github.com/go-kure/kure/pkg/kubernetes/externalsecrets"
)

es := externalsecrets.CreateExternalSecret("my-secret", "default")
es.Spec.SecretStoreRef = esv1.SecretStoreRef{Name: "vault", Kind: "ClusterSecretStore"}
es.Spec.Target = esv1.ExternalSecretTarget{Name: "my-secret"}
externalsecrets.AddExternalSecretData(es, esv1.ExternalSecretData{
    SecretKey: "password",
    RemoteRef: esv1.ExternalSecretDataRemoteRef{Key: "secret/data/myapp"},
})
externalsecrets.SetRefreshInterval(es, metav1.Duration{Duration: time.Hour})
```

### Secret Stores

```go
ss := externalsecrets.CreateSecretStore("aws-store", "default")
externalsecrets.SetSecretStoreProvider(ss, &esv1.SecretStoreProvider{
    AWS: &esv1.AWSProvider{Service: esv1.AWSServiceSecretsManager, Region: "us-east-1"},
})
ss.Spec.Controller = "my-controller"
```

### Cluster Secret Stores

```go
css := externalsecrets.CreateClusterSecretStore("global-vault")
externalsecrets.SetClusterSecretStoreProvider(css, &esv1.SecretStoreProvider{
    AWS: &esv1.AWSProvider{Service: esv1.AWSServiceSecretsManager, Region: "us-east-1"},
})
```

## Modifier Functions

Update existing resources:

```go
// Replace full spec
es.Spec = newSpec
ss.Spec = newSpec
css.Spec = newSpec

// Granular updates
externalsecrets.AddExternalSecretData(es, data)
externalsecrets.AddDataFrom(es, source)
es.Spec.SecretStoreRef = ref

externalsecrets.SetSecretStoreProvider(ss, provider)
ss.Spec.Controller = "my-controller"

externalsecrets.SetClusterSecretStoreProvider(css, provider)
css.Spec.Controller = "global"

// Labels and annotations use the generic helpers, which work over any object
// with ObjectMeta -- this package carries no per-kind metadata helpers
kubernetes.AddLabel(es, "app", "myapp")
kubernetes.AddAnnotation(ss, "desc", "value")
kubernetes.AddLabel(css, "team", "platform")
```

## Related Packages

- [kubernetes](/api-reference/kubernetes-builders/) - Core Kubernetes resource builders
- [fluxcd](/api-reference/fluxcd-builders/) - FluxCD resource builders
- [metallb](/api-reference/metallb-builders/) - MetalLB resource builders
- [Builder contract: release 2 migration notes](/concepts/builder-contract-release-2/) - the retired config-struct layer, field by field
