# Flux Engine - FluxCD Workflow Implementation

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/stack/fluxcd.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/stack/fluxcd)

The `fluxcd` package implements the `stack.Workflow` interface for FluxCD, providing complete Flux resource generation from domain model definitions.

## Overview

The Flux engine transforms Kure's hierarchical domain model (Cluster, Node, Bundle, Application) into FluxCD resources (Kustomizations, source references) organized in a GitOps-ready directory structure.

The engine is composed of three specialized components:

| Component | Responsibility |
|-----------|---------------|
| **ResourceGenerator** | Generates Flux resources from domain objects |
| **LayoutIntegrator** | Integrates resources into directory structures |
| **BootstrapGenerator** | Creates Flux bootstrap manifests |

## Quick Start

```go
import "github.com/go-kure/kure/pkg/stack/fluxcd"

// Create engine with defaults
engine := fluxcd.Engine()

// Generate all Flux resources for a cluster
objects, err := engine.GenerateFromCluster(cluster)

// Or with custom configuration
engine = fluxcd.EngineWithConfig(
    layout.KustomizationExplicit,
    layout.FluxSeparate,
)
```

## Engine Construction

```go
// Default engine
engine := fluxcd.Engine()

// Engine with specific kustomization mode
engine := fluxcd.EngineWithMode(layout.KustomizationExplicit)

// Engine with full configuration
engine := fluxcd.EngineWithConfig(mode, placement)

// Engine with custom components
engine := fluxcd.NewWorkflowEngine()
```

## Resource Generation

Generate Flux resources at different hierarchy levels:

```go
// From entire cluster
objects, err := engine.GenerateFromCluster(cluster)

// From a single node
objects, err := engine.GenerateFromNode(node)

// From a single bundle
objects, err := engine.GenerateFromBundle(bundle)
```

Each bundle produces a Flux Kustomization resource with:
- Path matching the layout directory structure
- Source reference from the node's package ref
- Dependency ordering from `Bundle.DependsOn`
- Interval and pruning configuration

## Layout Integration

Combine resource generation with directory structure:

```go
// Create layout with Flux resources integrated
ml, err := engine.CreateLayoutWithResources(cluster, rules)

// Write to disk
err = layout.WriteManifest(ml, "./clusters")
```

## Bootstrap Generation

Generate Flux system bootstrap manifests. Two modes are supported:

| Mode | Description |
|------|-------------|
| `"flux-operator"` | **Default.** Emits a full Flux Operator install bundle (CRDs, Deployment, RBAC). Recommended for new clusters. |
| `"gotk"` | Legacy mode. Emits the GitOps Toolkit component manifests directly. |

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

## Configuration

### Kustomization Mode

Controls how kustomization.yaml files reference resources:

- `KustomizationExplicit` - Lists all manifest files explicitly
- `KustomizationRecursive` - References subdirectories only

### Flux Placement

Controls where Flux Kustomization resources are placed:

- `FluxSeparate` - Flux resources in a separate directory tree
- `FluxIntegrated` - Flux resources alongside application manifests

## Umbrella Bundles

A `Bundle` with a non-empty `Children` slice becomes an **umbrella**: a parent
Flux Kustomization that aggregates the readiness of its children via
`spec.wait: true` and auto-generated `spec.healthChecks`. This gives downstream
consumers a single stable anchor regardless of how many internal tiers the
umbrella contains.

### Resource generation

`ResourceGenerator.createKustomization` detects umbrella bundles and:
- forces `spec.wait = true`
- prepends one `HealthChecks` entry per direct child (referencing the child's
  own Kustomization by name/namespace)
- leaves user-supplied `HealthChecks` appended after the auto entries

`GenerateFromBundle(b)` is strictly self-only — it never recurses into
`b.Children`. Callers that want the entire umbrella closure as a flat list use
`GenerateFromNode` or `GenerateFromCluster`, which walk umbrella children via
`generateUmbrellaClosure` internally.

### Placement in layouts

`LayoutIntegrator` places umbrella child Flux CRs at the **parent** layout
node:

- **FluxIntegrated, non-nodeOnly**: the walker creates a bundle sub-layout
  under the node layout. Umbrella child Kustomization CRs (and their Source
  CRs, if the child has a `SourceRef.URL`) are appended to the bundle
  sub-layout's `Resources`. Nested umbrella children are placed at their
  enclosing umbrella child's layout node.
- **FluxIntegrated, nodeOnly (GroupFlat)**: there is no intermediate bundle
  layer, so umbrella children become direct sub-layouts of the node layout,
  and their Flux CRs sit at the node layout alongside the umbrella self CR.
- **FluxSeparate**: `GenerateFromCluster` walks the full umbrella closure, so
  the `flux-system` layout directory receives every descendant's Kustomization
  CR as a flat list.

### On-disk shape

When a parent layout has an umbrella child, the parent's `kustomization.yaml`
references the child via `flux-system-kustomization-{child}.yaml` (the
Kustomization CR file sitting in the parent directory) instead of the child
subdirectory. The child subdirectory still exists and still contains its own
`kustomization.yaml` plus workload YAML files — but **no** Flux CR files, so
Flux does not double-apply the child's resources.

## Validation

All cluster-level entry points (`GenerateFromCluster`, `CreateLayoutWithResources`)
call `stack.ValidateCluster` before walking the tree. Invalid umbrella
configurations — such as a bundle referenced both by a `Node` and by another
bundle's `Children`, shared umbrella ownership, or multi-package umbrellas —
fail fast with a validation error rather than producing malformed output.

## Related Packages

- [stack](../) - Core domain model
- [stack/layout](../layout/) - Manifest directory organization
- [kubernetes/fluxcd](/api-reference/fluxcd-builders/) - Low-level Flux resource builders
