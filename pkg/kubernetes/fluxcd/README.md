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

// Helm repository (HTTP/HTTPS)
helmRepo := fluxcd.HelmRepository(&fluxcd.HelmRepositoryConfig{
    Name:      "bitnami",
    Namespace: "flux-system",
    URL:       "https://charts.bitnami.com/bitnami",
})

// Helm repository (OCI registry)
ociHelmRepo := fluxcd.HelmRepository(&fluxcd.HelmRepositoryConfig{
    Name:      "ghcr",
    Namespace: "flux-system",
    URL:       "oci://ghcr.io/example/charts",
    Type:      sourcev1.HelmRepositoryTypeOCI, // "oci"
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

// HelmRelease — chart template mode (chart + version + source reference)
hr := fluxcd.HelmRelease(&fluxcd.HelmReleaseConfig{
    Name:        "redis",
    Namespace:   "apps",
    Chart:       "redis",
    Version:     "17.0.0",
    SourceRef:   helmv2.CrossNamespaceObjectReference{Kind: "HelmRepository", Name: "bitnami"},
    Interval:    "10m",
    ReleaseName: "redis",
    TargetNamespace:       "databases",
    DriftDetectionMode:    "enabled",
    InstallCRDs:           "CreateReplace",
    InstallRetries:        ptr(3),
    UpgradeCRDs:           "Skip",
    UpgradeCleanupOnFail:  true,
    UpgradeRetries:        ptr(3),
    RollbackCleanupOnFail: true,
    ValuesFrom: []fluxcd.ValuesFromConfig{
        {Kind: "ConfigMap", Name: "redis-values"},
        {Kind: "Secret", Name: "redis-secret", Optional: true},
    },
})

// HelmRelease — chartRef mode (references an existing OCIRepository or HelmChart)
hr = fluxcd.HelmRelease(&fluxcd.HelmReleaseConfig{
    Name:      "my-app",
    Namespace: "apps",
    Interval:  "10m",
    ChartRef: &fluxcd.ChartRefConfig{
        Kind:      "OCIRepository",
        Name:      "my-oci-source",
        Namespace: "flux-system",
    },
})
```

### Notification Controllers

> **Note:** Provider and Alert use `notification.toolkit.fluxcd.io/v1beta3` — the highest
> API version available upstream. Receiver is on v1. See [compatibility](/api-reference/compatibility/#notification-controller-provider-and-alert-on-v1beta3)
> for details and tracking issue [#250](https://github.com/go-kure/kure/issues/250).

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
