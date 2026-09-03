# External Secrets Builders - External Secrets Resource Constructors

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/externalsecrets.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/externalsecrets)

The `externalsecrets` package provides strongly-typed constructor functions for creating External Secrets Operator Kubernetes resources. These are the low-level building blocks used by Kure's higher-level stack and workflow packages.

## Overview

Each config-struct builder takes a configuration struct and returns a populated External Secrets custom resource. The builders handle API version and kind metadata, letting you focus on the resource specification.

## Constructors

Every kind this package registers has a generated `Create<Kind>` wrapper in `zz_generated_create.go`, produced from the scheme by `pkg/kubernetes/internal/gen` (`make gen-builders`, checked by `make check-builders` in CI). A wrapper delegates to `kubernetes.Create[T]` and emits **TypeMeta and identity only**: no default, no label, no spec value. Namespaced kinds take `(name, namespace)`, cluster-scoped kinds take `(name)`. The upstream struct is the construction API; set spec fields directly or through the admissible `Set*`/`Add*` sugar below.

```go
obj := externalsecrets.CreateExternalSecret("my-secret", "default")
cl := externalsecrets.CreateClusterSecretStore("global-vault")
```

The config-struct builders (`externalsecrets.ExternalSecret(&externalsecrets.ExternalSecretConfig{...})`) are a separate, opinionated layer on top of the same upstream types; they are unchanged by the generated constructors. The hand-written `Create*` helpers for spec fragments that remain in this package are legacy and are removed by the prune work item of the builder-contract epic.

See the [Kubernetes Builders](/api-reference/kubernetes-builders/) page for the full builder contract: construction, sugar admission classes, purity and the release-1 migration ledger.

## Supported Resources

### External Secrets

```go
import "github.com/go-kure/kure/pkg/kubernetes/externalsecrets"

es := externalsecrets.ExternalSecret(&externalsecrets.ExternalSecretConfig{
    Name:      "my-secret",
    Namespace: "default",
    SecretStoreRef: esv1.SecretStoreRef{
        Name: "vault",
        Kind: "ClusterSecretStore",
    },
    Data: []esv1.ExternalSecretData{
        {
            SecretKey: "password",
            RemoteRef: esv1.ExternalSecretDataRemoteRef{
                Key: "secret/data/myapp",
            },
        },
    },
})
```

### Secret Stores

```go
ss := externalsecrets.SecretStore(&externalsecrets.SecretStoreConfig{
    Name:      "aws-store",
    Namespace: "default",
    Provider: &esv1.SecretStoreProvider{
        AWS: &esv1.AWSProvider{
            Region: "us-east-1",
        },
    },
})
```

### Cluster Secret Stores

```go
css := externalsecrets.ClusterSecretStore(&externalsecrets.ClusterSecretStoreConfig{
    Name: "global-vault",
    Provider: &esv1.SecretStoreProvider{
        AWS: &esv1.AWSProvider{
            Region: "us-east-1",
        },
    },
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
es.Spec.SecretStoreRef = ref
externalsecrets.AddExternalSecretLabel(es, "app", "myapp")
externalsecrets.AddExternalSecretAnnotation(es, "note", "value")

externalsecrets.SetSecretStoreProvider(ss, provider)
ss.Spec.Controller = "my-controller"
externalsecrets.AddSecretStoreLabel(ss, "env", "prod")
externalsecrets.AddSecretStoreAnnotation(ss, "desc", "value")

externalsecrets.SetClusterSecretStoreProvider(css, provider)
css.Spec.Controller = "global"
externalsecrets.AddClusterSecretStoreLabel(css, "team", "platform")
externalsecrets.AddClusterSecretStoreAnnotation(css, "owner", "ops")
```

## Related Packages

- [kubernetes](/api-reference/kubernetes-builders/) - Core Kubernetes resource builders
- [fluxcd](/api-reference/fluxcd-builders/) - FluxCD resource builders
- [metallb](/api-reference/metallb-builders/) - MetalLB resource builders
