+++
title = "Generating Flux Manifests"
weight = 20
+++

# Generating Flux Manifests

This guide walks through the complete workflow for generating a GitOps repository structure with Flux resources using Kure.

## Overview

The workflow has four stages:

1. **Define** the cluster topology using the domain model
2. **Select** the Flux workflow engine
3. **Generate** Flux resources and directory layout
4. **Write** manifests to disk

## Step 1: Define the Cluster

Use the fluent builder to define your cluster's structure:

```go
import "github.com/go-kure/kure/pkg/stack"

cluster := stack.NewClusterBuilder("production").
    WithNode("infrastructure").
        WithBundle("cert-manager").
            WithApplication("cert-manager", certManagerConfig).
        End().
    End().
    WithNode("applications").
        WithBundle("web-tier").
            WithApplication("frontend", frontendConfig).
            WithApplication("api-gateway", apiConfig).
        End().
    End().
    Build()
```

Each bundle becomes a Flux Kustomization, and each application generates its Kubernetes manifests.

## Step 2: Create the Flux Engine

```go
import (
    "github.com/go-kure/kure/pkg/stack/fluxcd"
    "github.com/go-kure/kure/pkg/stack/layout"
)

engine := fluxcd.EngineWithConfig(
    layout.KustomizationExplicit,  // List files in kustomization.yaml
    layout.FluxSeparate,           // Flux resources in separate tree
)
```

See the [Flux Engine reference](/api-reference/flux-engine) for configuration options.

## Step 3: Generate Resources with Layout

```go
// Define layout rules
rules := layout.LayoutRules{
    NodeGrouping:        layout.GroupByName,
    BundleGrouping:      layout.GroupByName,
    ApplicationGrouping: layout.GroupByName,
    FilePer:             layout.FilePerResource,
}

// Generate layout with Flux resources integrated
ml, err := engine.CreateLayoutWithResources(cluster, rules)
if err != nil {
    return errors.Wrap(err, "failed to create layout")
}
```

## Step 4: Write to Disk

```go
err := layout.WriteManifest(ml, "./clusters")
```

This produces a directory structure like:

```
clusters/
  production/
    infrastructure/
      cert-manager/
        cert-manager/
          deployment.yaml
          service.yaml
          kustomization.yaml
        kustomization.yaml        # Flux Kustomization
    applications/
      web-tier/
        frontend/
          deployment.yaml
          service.yaml
        api-gateway/
          deployment.yaml
          service.yaml
        kustomization.yaml        # Flux Kustomization
```

## Layout Configuration

The [Layout Engine](/api-reference/layout) supports multiple grouping and file organization strategies:

| Option | Values | Effect |
|--------|--------|--------|
| NodeGrouping | `GroupByName`, `GroupFlat` | Create subdirectories per node or flatten |
| BundleGrouping | `GroupByName`, `GroupFlat` | Create subdirectories per bundle or flatten |
| ApplicationGrouping | `GroupByName`, `GroupFlat` | Create subdirectories per app or flatten |
| FilePer | `FilePerResource`, `FilePerKind` | One file per resource or group by kind |
| FluxPlacement | `FluxSeparate`, `FluxIntegrated` | Separate or inline Flux resources |

## Umbrella Bundles — Readiness Aggregation

A bundle with non-empty `Children` becomes an **umbrella**: Flux will only mark
its Kustomization `Ready` once every child Kustomization is `Ready`. The Flux
engine enforces this by:

- Forcing `spec.wait: true` on the umbrella's Kustomization
- Prepending an auto `spec.healthChecks` entry for each direct child

The resulting umbrella Kustomization aggregates child readiness regardless of
how many children there are, giving external consumers a single stable anchor:

```go
umbrella := &stack.Bundle{
    Name: "platform",
    Children: []*stack.Bundle{
        {Name: "platform-infra"},
        {Name: "platform-services"},
        {Name: "platform-apps"},
    },
}
```

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: platform
  namespace: flux-system
spec:
  wait: true
  healthChecks:
  - apiVersion: kustomize.toolkit.fluxcd.io/v1
    kind: Kustomization
    name: platform-infra
    namespace: flux-system
  - apiVersion: kustomize.toolkit.fluxcd.io/v1
    kind: Kustomization
    name: platform-services
    namespace: flux-system
  - apiVersion: kustomize.toolkit.fluxcd.io/v1
    kind: Kustomization
    name: platform-apps
    namespace: flux-system
  # ...rest of spec
```

User-supplied `HealthChecks` on the umbrella bundle are appended AFTER the
auto entries. Setting `Wait: false` on a bundle that has `Children` is
rejected at validation time.

Umbrella children must be **standalone** — a bundle cannot simultaneously be
the `Bundle` of a `stack.Node` and appear in another bundle's `Children`.
`stack.ValidateCluster` rejects any such overlap before resource generation.

### Disk layout

In `FluxIntegrated` placement, the umbrella children's Flux Kustomization CRs
live alongside the parent's, and the parent's `kustomization.yaml` references
each child via the CR file (not the child subdirectory):

```
clusters/production/apps/
  platform/                                       # umbrella bundle directory
    flux-system-kustomization-platform.yaml       # umbrella self CR (wait+HC)
    flux-system-kustomization-platform-infra.yaml # child CR (placed at parent)
    flux-system-kustomization-platform-apps.yaml  # child CR (placed at parent)
    kustomization.yaml                            # references the CR files
    platform-infra/                               # umbrella child subdirectory
      workload-*.yaml
      kustomization.yaml                          # workloads only, no Flux CRs
    platform-apps/
      workload-*.yaml
      kustomization.yaml
```

The child subdirectories contain **only** their workload manifests and a
per-directory `kustomization.yaml` listing those workloads. They do **not**
contain any `flux-system-kustomization-*.yaml` files — those live in the
parent directory, so Flux applies them once via the parent's Kustomization.

In `FluxSeparate` placement, all Kustomization CRs (the umbrella's own plus
every descendant) are written to the shared `flux-system/` directory as a
flat list.

## Bootstrap

Generate Flux system bootstrap manifests. Two modes are available:

- **`"flux-operator"`** (default) — emits a full Flux Operator install bundle (CRDs, Deployment, RBAC). Recommended for new clusters.
- **`"gotk"`** — emits the legacy GitOps Toolkit component manifests directly.

When `FluxMode` is empty, it defaults to `"flux-operator"`.

```go
bootstrapConfig := &stack.BootstrapConfig{
    Enabled:     true,
    FluxMode:    "flux-operator", // or "gotk"; empty defaults to "flux-operator"
    FluxVersion: "v2.8.2",
    SourceRef:   sourceRef,
}

objects, err := engine.GenerateBootstrap(bootstrapConfig, rootNode)
```

## Further Reading

- [Stack](/api-reference/stack) - Domain model reference
- [Flux Engine](/api-reference/flux-engine) - Workflow engine reference
- [Layout Engine](/api-reference/layout) - Directory organization reference
- [Generators](/guides/generators/) - Application generator guide
