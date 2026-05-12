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
    Ref:       "main",
    Interval:  "5m",
})

// OCI repository source — tag reference
ociRepo := fluxcd.OCIRepository(&fluxcd.OCIRepositoryConfig{
    Name:      "my-oci",
    Namespace: "flux-system",
    URL:       "oci://registry.example.com/manifests",
    Ref:       "latest",
    Interval:  "10m",
})

// OCI repository source — digest (content-addressable) reference
ociRepoDigest := fluxcd.OCIRepository(&fluxcd.OCIRepositoryConfig{
    Name:      "my-oci-pinned",
    Namespace: "flux-system",
    URL:       "oci://registry.example.com/manifests",
    Digest:    "sha256:abc123...",
    Interval:  "10m",
})

// Helm repository (HTTP/HTTPS)
helmRepo := fluxcd.HelmRepository(&fluxcd.HelmRepositoryConfig{
    Name:      "bitnami",
    Namespace: "flux-system",
    URL:       "https://charts.bitnami.com/bitnami",
    Interval:  "10m",
})

// Helm repository (OCI registry)
ociHelmRepo := fluxcd.HelmRepository(&fluxcd.HelmRepositoryConfig{
    Name:      "ghcr",
    Namespace: "flux-system",
    URL:       "oci://ghcr.io/example/charts",
    Type:      sourcev1.HelmRepositoryTypeOCI, // "oci"
    Interval:  "10m",
})

// Bucket source
bucket := fluxcd.Bucket(&fluxcd.BucketConfig{
    Name:       "my-bucket",
    Namespace:  "flux-system",
    Endpoint:   "minio.example.com",
    BucketName: "manifests",
})
```

### Deployment Controllers

```go
// Kustomization (reconciles manifests from a source)
ks := fluxcd.Kustomization(&fluxcd.KustomizationConfig{
    Name:            "my-app",
    Namespace:       "flux-system",
    Path:            "./clusters/production/apps",
    Interval:        "10m",
    Prune:           true,
    TargetNamespace: "production",
    Wait:            true,
    SourceRef: kustv1.CrossNamespaceSourceReference{
        Kind: "GitRepository",
        Name: "my-repo",
    },
})

// HelmRelease — chart template mode (chart + version + source reference)
installRetries := 3
upgradeRetries := 3
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
    InstallRetries:        &installRetries,
    UpgradeCRDs:           "Skip",
    UpgradeCleanupOnFail:  true,
    UpgradeRetries:        &upgradeRetries,
    RollbackCleanupOnFail: true,
    ValuesFrom: []fluxcd.ValuesFromConfig{
        {Kind: "ConfigMap", Name: "redis-values"},
        {Kind: "Secret", Name: "redis-secret", Optional: true},
    },
    Values: map[string]any{
        "replicaCount": 1,
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
    Name:          "slack-alert",
    Namespace:     "flux-system",
    ProviderRef:   "slack",
    EventSeverity: "error",
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

### ExternalArtifact (source.toolkit.fluxcd.io/v1)

`ExternalArtifact` allows a Flux source artifact produced outside the cluster to be referenced by other Flux resources.

```go
ea := fluxcd.CreateExternalArtifact("my-artifact", "flux-system")
fluxcd.SetExternalArtifactSourceRef(ea, &meta.NamespacedObjectKindReference{
    APIVersion: "source.toolkit.fluxcd.io/v1",
    Kind:       "OCIRepository",
    Name:       "my-oci-source",
    Namespace:  "flux-system",
})
// Replace the full spec when needed:
// fluxcd.SetExternalArtifactSpec(ea, newSpec)
```

Available functions: `CreateExternalArtifact`, `SetExternalArtifactSourceRef`, `SetExternalArtifactSpec`.

### ArtifactGenerator (source.extensions.fluxcd.io/v1beta1)

`ArtifactGenerator` is provided by the optional **source-watcher** component. It assembles a new artifact by copying files from one or more source artifacts.

```go
ag := fluxcd.CreateArtifactGenerator("my-gen", "flux-system")

// Declare source references
src := fluxcd.CreateSourceReference("app", "my-oci-source", "OCIRepository")
fluxcd.SetSourceReferenceNamespace(&src, "flux-system")
fluxcd.AddArtifactGeneratorSource(ag, src)

// Build an output artifact with a copy operation
out := fluxcd.CreateOutputArtifact("combined")
fluxcd.SetOutputArtifactRevision(&out, "@app")

cp := fluxcd.CreateCopyOperation("@app/manifests/**", "@artifact/manifests")
fluxcd.AddCopyOperationExclude(&cp, "**/*.secret.yaml")
fluxcd.SetCopyOperationStrategy(&cp, "Overwrite")
fluxcd.AddOutputArtifactCopyOperation(&out, cp)

fluxcd.AddArtifactGeneratorOutputArtifact(ag, out)
```

Available functions:
- Resource: `CreateArtifactGenerator`, `AddArtifactGeneratorSource`, `AddArtifactGeneratorOutputArtifact`
- Source references: `CreateSourceReference`, `SetSourceReferenceNamespace`
- Output artifacts: `CreateOutputArtifact`, `SetOutputArtifactRevision`, `SetOutputArtifactOriginRevision`
- Copy operations: `CreateCopyOperation`, `AddCopyOperationExclude`, `SetCopyOperationStrategy`, `AddOutputArtifactCopyOperation`

## New Setters on Existing Types

### GitRepository

- `SetGitRepositorySparseCheckout(gr, []string)` — restrict checkout to specific directories
- `AddGitRepositorySparseCheckoutPath(gr, path)` — append a single sparse-checkout path
- `SetGitRepositoryServiceAccountName(gr, name)` — workload identity via service account

### Kustomization

- `AddKustomizationHealthCheckExpr(k, check)` — add a CEL-based custom health check expression
- `SetKustomizationIgnoreMissingComponents(k, bool)` — silently skip missing component paths
- Helper: `CreateCustomHealthCheck(apiVersion, kind, current)` constructs a `CustomHealthCheck`; optionally set `SetCustomHealthCheckInProgress` and `SetCustomHealthCheckFailed` for the remaining CEL expressions.

### HelmRelease Install control flags

Fine-grained setters for `spec.install` — each auto-initialises the `Install` struct if nil:

- `SetHelmReleaseInstallTimeout` — per-action timeout
- `SetHelmReleaseInstallCRDs` — CRD policy (`Skip`, `Create`, `CreateReplace`)
- `SetHelmReleaseInstallCreateNamespace` — create target namespace
- `SetHelmReleaseInstallDisableSchemaValidation`
- `SetHelmReleaseInstallDisableOpenAPIValidation`
- `SetHelmReleaseInstallDisableHooks`
- `SetHelmReleaseInstallDisableWait`
- `SetHelmReleaseInstallDisableWaitForJobs`
- `SetHelmReleaseInstallDisableTakeOwnership`
- `SetHelmReleaseInstallReplace`

### HelmRelease Upgrade control flags

Fine-grained setters for `spec.upgrade` — each auto-initialises the `Upgrade` struct if nil:

- `SetHelmReleaseUpgradeTimeout` — per-action timeout
- `SetHelmReleaseUpgradeCRDs` — CRD policy
- `SetHelmReleaseUpgradeDisableSchemaValidation`
- `SetHelmReleaseUpgradeDisableOpenAPIValidation`
- `SetHelmReleaseUpgradeDisableHooks`
- `SetHelmReleaseUpgradeDisableWait`
- `SetHelmReleaseUpgradeDisableWaitForJobs`
- `SetHelmReleaseUpgradeDisableTakeOwnership`
- `SetHelmReleaseUpgradeForce`
- `SetHelmReleaseUpgradePreserveValues`
- `SetHelmReleaseUpgradeCleanupOnFail`

### FluxInstance

- `SetFluxInstanceDistributionVariant(obj, variant)` — set the image variant (`upstream-alpine`, `enterprise-alpine`, `enterprise-distroless`, `enterprise-distroless-fips`)

## Related Packages

- [stack/fluxcd](/api-reference/flux-engine/) - High-level Flux workflow engine
- [stack](/api-reference/stack/) - Domain model that produces Flux resources
