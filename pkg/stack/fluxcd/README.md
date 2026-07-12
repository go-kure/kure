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

// Or with a specific kustomization mode
engine = fluxcd.EngineWithConfig(layout.KustomizationExplicit)

// Placement is set on the LayoutRules passed to the layout call,
// not on the engine — see Layout Integration below.
```

## Engine Construction

```go
// Default engine
engine := fluxcd.Engine()

// Engine with specific kustomization mode
engine := fluxcd.EngineWithMode(layout.KustomizationExplicit)

// Engine with a specific kustomization mode (alias)
engine := fluxcd.EngineWithConfig(mode)

// Engine with custom components
engine := fluxcd.NewWorkflowEngine()
```

Placement (FluxIntegratedPerLayout vs FluxSeparate) is configured per call on
`layout.LayoutRules.FluxPlacement`. `FluxUnset` is normalized to
`FluxSeparate` by `LayoutIntegrator.CreateLayoutWithResources` — matching
`layout.DefaultLayoutRules()` and the walker. See
[Layout Integration](#layout-integration).

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

The `"flux-operator"` bundle is vendored from the upstream flux-operator release and pinned
in lockstep with the `github.com/controlplaneio-fluxcd/flux-operator` Go module
(`FluxOperatorVersion`, currently **v0.53.0**). See `flux_operator_install.go` for the
refresh procedure.

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

- `FluxSeparate` - Flux resources collected in a separate directory tree; children referenced as directories
- `FluxIntegratedPerLayout` - a Flux Kustomization CR for **every** layout node (incl. augmenter-added child layouts), placed alongside its manifests; children referenced as `kustomization-<child>.yaml` CR files. Finest granularity.
- `FluxIntegratedPerBundle` - Flux Kustomization CRs at **bundle/node boundaries only**; a bundle's interior (incl. augmenter-added child layouts) is a single kustomize build, with children referenced as directories. Coarser: Flux reconciles per bundle, kustomize handles the interior.

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

- **FluxIntegratedPerLayout, non-nodeOnly**: the walker creates a bundle sub-layout
  under the node layout. Umbrella child Kustomization CRs (and their Source
  CRs, if the child has a `SourceRef.URL`) are appended to the bundle
  sub-layout's `Resources`. Nested umbrella children are placed at their
  enclosing umbrella child's layout node.
- **FluxIntegratedPerLayout, nodeOnly (GroupFlat)**: there is no intermediate bundle
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

## Non-Bundle Child Layout CRs

In `FluxIntegratedPerLayout` mode the layout integrator generates `Kustomization` CRs for **all eligible children** of each node layout, not only the node's own bundle. A child is eligible when `!UmbrellaChild && ApplicationFileMode != AppFileSingle`.

This covers two cases with the same code path:

- **Flat/nodeOnly layouts** — app layouts are direct children of the node layout. Each eligible app layout gets a `Kustomization` CR placed in the node layout's `Resources`, with `spec.path` set to `child.FullRepoPath()`.
- **Augmenter sub-layouts** — hook-group child layouts added by a `LayoutAugmenter` are children of an app layout. Each eligible child gets a CR placed in the app layout's `Resources`. `spec.dependsOn` is populated from `ManifestLayout.DependsOn`, enabling ordered reconciliation between hook groups.

The integrator applies this rule recursively: it covers children at any depth, always placing the CR in the immediate parent's `Resources`.

If the ancestor bundle has a nil, empty, or incomplete `SourceRef` (missing `Kind` or `Name`) and eligible children without existing CRs are present, `IntegrateWithLayout` returns a hard error. A `Kustomization` without a valid `spec.sourceRef` is rejected by Flux and must not be emitted silently.

## Validation

All cluster-level entry points (`GenerateFromCluster`, `CreateLayoutWithResources`)
call `stack.ValidateCluster` before walking the tree. Invalid umbrella
configurations — such as a bundle referenced both by a `Node` and by another
bundle's `Children`, shared umbrella ownership, or multi-package umbrellas —
fail fast with a validation error rather than producing malformed output.

`CreateLayoutWithResources` additionally calls `validateSourceRefsForFluxIntegrated`
for **both inline placements** (`FluxIntegratedPerLayout` and
`FluxIntegratedPerBundle`) — both emit bundle/node CRs that carry a `spec.sourceRef`.
(After normalization `FluxUnset` becomes `FluxSeparate`, which skips this gate.)
This checks that every bundle
reachable from the cluster node tree — node bundles and umbrella child bundles
recursively — has a complete `SourceRef` with both `Kind` and `Name` set. A nil,
zero-value, or partially-populated `SourceRef` is rejected before layout walking
begins. The integrator also enforces this at CR-creation time as defense in
depth. `FluxSeparate` and non-Flux paths are unaffected.

## Related Packages

- [stack](/api-reference/stack/) - Core domain model
- [stack/layout](/api-reference/layout/) - Manifest directory organization
- [kubernetes/fluxcd](/api-reference/fluxcd-builders/) - Low-level Flux resource builders
