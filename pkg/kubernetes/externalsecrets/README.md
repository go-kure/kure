# External Secrets Builders - External Secrets Resource Constructors

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/externalsecrets.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/externalsecrets)

The `externalsecrets` package provides strongly-typed constructor functions for creating External Secrets Operator Kubernetes resources. These are the low-level building blocks used by Kure's higher-level stack and workflow packages.

## Overview

Each function takes a configuration struct and returns a fully initialized External Secrets custom resource. The builders handle API version and kind metadata, letting you focus on the resource specification.

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
externalsecrets.SetExternalSecretSpec(es, newSpec)
externalsecrets.SetSecretStoreSpec(ss, newSpec)
externalsecrets.SetClusterSecretStoreSpec(css, newSpec)

// Granular updates
externalsecrets.AddExternalSecretData(es, data)
externalsecrets.SetExternalSecretSecretStoreRef(es, ref)
externalsecrets.AddExternalSecretLabel(es, "app", "myapp")
externalsecrets.AddExternalSecretAnnotation(es, "note", "value")

externalsecrets.SetSecretStoreProvider(ss, provider)
externalsecrets.SetSecretStoreController(ss, "my-controller")
externalsecrets.AddSecretStoreLabel(ss, "env", "prod")
externalsecrets.AddSecretStoreAnnotation(ss, "desc", "value")

externalsecrets.SetClusterSecretStoreProvider(css, provider)
externalsecrets.SetClusterSecretStoreController(css, "global")
externalsecrets.AddClusterSecretStoreLabel(css, "team", "platform")
externalsecrets.AddClusterSecretStoreAnnotation(css, "owner", "ops")
```

## Related Packages

- [kubernetes](/api-reference/kubernetes-builders/) - Core Kubernetes resource builders
- [fluxcd](/api-reference/fluxcd-builders/) - FluxCD resource builders
- [metallb](/api-reference/metallb-builders/) - MetalLB resource builders
