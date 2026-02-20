# FluxCD Builders - Flux Resource Constructors

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/fluxcd.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/fluxcd)

The `fluxcd` package provides strongly-typed constructor functions for creating FluxCD Kubernetes resources. These are the low-level building blocks used by Kure's higher-level stack and workflow packages.

## Overview

Each function takes a configuration struct and returns a fully initialized Flux custom resource. The builders handle API version and kind metadata, letting you focus on the resource specification.

## Supported Resources

### Source Controllers

```go
import "github.com/go-kure/kure/pkg/kubernetes/fluxcd"

// Git repository source
gitRepo := fluxcd.GitRepository(&fluxcd.GitRepositoryConfig{
    Name:      "my-repo",
    Namespace: "flux-system",
    URL:       "https://github.com/org/repo",
    Branch:    "main",
    Interval:  "5m",
})

// OCI repository source
ociRepo := fluxcd.OCIRepository(&fluxcd.OCIRepositoryConfig{
    Name:      "my-oci",
    Namespace: "flux-system",
    URL:       "oci://registry.example.com/manifests",
    Tag:       "latest",
    Interval:  "10m",
})

// Helm repository
helmRepo := fluxcd.HelmRepository(&fluxcd.HelmRepositoryConfig{
    Name:      "bitnami",
    Namespace: "flux-system",
    URL:       "https://charts.bitnami.com/bitnami",
    Interval:  "1h",
})

// Bucket source
bucket := fluxcd.Bucket(&fluxcd.BucketConfig{
    Name:      "my-bucket",
    Namespace: "flux-system",
    Endpoint:  "minio.example.com",
    BucketName: "manifests",
})
```

### Deployment Controllers

```go
// Kustomization (reconciles manifests from a source)
ks := fluxcd.Kustomization(&fluxcd.KustomizationConfig{
    Name:      "my-app",
    Namespace: "flux-system",
    Path:      "./clusters/production/apps",
    Interval:  "10m",
    Prune:     true,
    SourceRef: kustv1.CrossNamespaceSourceReference{
        Kind: "GitRepository",
        Name: "my-repo",
    },
})

// HelmRelease (reconciles a Helm chart)
hr := fluxcd.HelmRelease(&fluxcd.HelmReleaseConfig{
    Name:       "redis",
    Namespace:  "default",
    Chart:      "redis",
    Version:    "17.0.0",
    RepoName:   "bitnami",
    RepoNamespace: "flux-system",
    Values: map[string]interface{}{
        "auth": map[string]interface{}{
            "enabled": false,
        },
    },
})
```

### Notification Controllers

```go
// Alert
alert := fluxcd.Alert(&fluxcd.AlertConfig{
    Name:      "slack-alert",
    Namespace: "flux-system",
    Provider:  "slack",
    Severity:  "error",
})

// Provider
provider := fluxcd.Provider(&fluxcd.ProviderConfig{
    Name:      "slack",
    Namespace: "flux-system",
    Type:      "slack",
    Channel:   "#alerts",
})

// Receiver (for webhooks)
receiver := fluxcd.Receiver(&fluxcd.ReceiverConfig{
    Name:      "github-receiver",
    Namespace: "flux-system",
    Type:      "github",
})
```

### Flux Operator

```go
// FluxInstance (for flux-operator deployments)
instance := fluxcd.FluxInstance(&fluxcd.FluxInstanceConfig{
    Name:      "flux",
    Namespace: "flux-system",
})
```

## Modifier Functions

Update existing resources:

```go
// Update Kustomization spec
err := fluxcd.SetKustomizationSpec(ks, newSpec)

// Update HelmRelease spec
err := fluxcd.SetHelmReleaseSpec(hr, newSpec)

// Add dependency to Kustomization
err := fluxcd.AddKustomizationDependency(ks, kustv1.Dependency{
    Name: "cert-manager",
})
```

## Related Packages

- [stack/fluxcd](/api-reference/flux-engine/) - High-level Flux workflow engine
- [stack](/api-reference/stack/) - Domain model that produces Flux resources
