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

## Bootstrap

Generate Flux system bootstrap manifests:

```go
bootstrapConfig := &stack.BootstrapConfig{
    Enabled:     true,
    FluxMode:    "install",
    FluxVersion: "v2.6.4",
    SourceRef:   sourceRef,
}

objects, err := engine.GenerateBootstrap(bootstrapConfig, rootNode)
```

## Further Reading

- [Stack](/api-reference/stack) - Domain model reference
- [Flux Engine](/api-reference/flux-engine) - Workflow engine reference
- [Layout Engine](/api-reference/layout) - Directory organization reference
- [Generators](generators) - Application generator guide
