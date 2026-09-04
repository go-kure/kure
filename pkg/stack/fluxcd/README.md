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

## Defaults are declared, named and exported

This package is a workflow layer above `pkg/kubernetes`, so unlike the builders it may hold
opinions — but only as declared inputs with names a consumer can read, compare against and
override. Every fallback this package applies is one of the exported identifiers in `defaults.go`,
or comes from `layout.DefaultLayoutRules()` where the value belongs to `pkg/stack/layout`; grep
for the identifier to find every place its value can reach emitted YAML.

| Identifier | Value | Applies when | Override by |
|---|---|---|---|
| `DefaultInterval` | `60m` | the caller names no interval, or names one that does not parse | assigning `.DefaultInterval` on either generator |
| `DefaultNamespace` | `flux-system` | generated resources need a namespace | assigning `.DefaultNamespace` on either generator |
| `DefaultMode` | `layout.KustomizationExplicit` | `ResourceGenerator` is constructed — its only site | assigning `ResourceGenerator.Mode` |
| `DefaultBootstrapName` | `flux-system` | naming the bootstrap Kustomization and the `FluxInstance` | assigning `BootstrapGenerator.BootstrapName` |
| `DefaultSourceName` | `flux-system` | the root node has no name | naming the root `stack.Node` |
| `DefaultFluxMode` | `flux-operator` | `BootstrapConfig.FluxMode` is empty | setting `BootstrapConfig.FluxMode` |
| `DefaultSourceKind` | `OCIRepository` | `BootstrapConfig.SourceKind` does not name `GitRepository`, the empty string included | setting `BootstrapConfig.SourceKind` |
| `DefaultBootstrapPathRoot` | `manifests` | building the bootstrap Kustomization's `spec.path` | not overrideable; the root node's name is joined onto it |
| `DefaultFluxDirName` | `flux-system` | a separate Flux layout needs a directory | not overrideable |
| `DefaultSourceRef` | `latest` | an OCI source has no `SourceRef` | setting `BootstrapConfig.SourceRef` |
| `DefaultSyncPath` | `./` | the root node has no name | not overrideable; it is the prefix a sync path is built from |

Four of these — `DefaultInterval`, `DefaultNamespace`, `DefaultMode` and `DefaultBootstrapName` —
are copied into exported generator fields by `NewResourceGenerator` / `NewBootstrapGenerator`, and
a field assigned afterwards is never overridden. The rest are applied where they are used and are
overridden by naming the corresponding input, as the last column says. Three have no override at
all and say so, rather than being listed as though they had one.

An empty `BootstrapGenerator.BootstrapName` resolves back to `DefaultBootstrapName` at emission.
A generator built as a struct literal rather than through `NewBootstrapGenerator` leaves the field
zero, and a Kustomization or `FluxInstance` with no `metadata.name` is invalid — the field is an
override, not a way to remove the name.

`defaults.go` also declares `ModeGotk = "gotk"`, which is not a default: nothing falls back to it.
It is named so that the bootstrap mode set has one authority. `DefaultFluxMode` is both the
fallback for an empty `BootstrapConfig.FluxMode` and the mode `GenerateBootstrap` dispatches on,
and `SupportedBootstrapModes()` reports exactly those two — the validation error for an
unrecognised mode reads its list from that method rather than restating the names.

`DefaultFluxDirName` shares `DefaultNamespace`'s value and is deliberately a separate identifier:
one is a path segment and the other a Kubernetes namespace, and renaming the namespace must not
silently rename the directory.

Three defaults are not declared here because this package does not own them. The separate Flux
layout's file granularity and the fallback for an unset `FluxPlacement` both come from
`layout.DefaultLayoutRules()`, which is where `pkg/stack/layout` declares them and where its own
walker reads them from. A constant here would be a second copy of a value that can change
independently.

The third is that layout's own `Mode`, left unset rather than seeded from `DefaultMode`. The
layout writer resolves an unset mode to `KustomizationExplicit`, and the separate Flux layout
never has children, so the mode selects nothing there — seeding it would have implied an override
via `ResourceGenerator.Mode` at a site that never reads that field.

### One resolved source kind, not three

`resolvedSourceKind` is the only place the source kind is decided. Three sites need it — the
source object itself, the bootstrap Kustomization's `spec.sourceRef.Kind`, and the `FluxInstance`
sync block — and they used to decide it separately, with the `sourceRef` testing for
`OCIRepository` while the other two tested for `GitRepository`. The three agreed only when
`SourceKind` named a kind exactly; an empty or unrecognised `SourceKind` emitted an
`OCIRepository` under a `sourceRef` naming a `GitRepository` that was never created.

### `prune` and `wait` are inputs, not policy

`Bundle.Prune` and `Bundle.Wait` are `*bool` and are passed through untouched.
`BootstrapConfig.Prune` does the same for the bootstrap Kustomization, and
`ResourceGenerator.Prune` for Kustomizations generated from a `layout.ManifestLayout`, which
carries no prune setting of its own.

The two fields resolve differently because their upstream tags differ:

- `KustomizationSpec.Prune` is `+required` with no `omitempty`, so an unset input cannot leave
  the key out. **Unset emits `prune: false`** — garbage collection off. An unset tri-state
  previously collapsed onto `prune: true`, enabling destructive garbage collection for a caller
  who never asked for it.
- `KustomizationSpec.Wait` is `+optional` with `omitempty`, so unset and `false` produce the same
  YAML: the key is absent and Flux's own default, `false`, applies.

Sources (`GitRepository`, `OCIRepository`) and the `FluxInstance` are built the way the
builder contract prescribes: the `pkg/kubernetes/fluxcd` `Create<Kind>` constructor returns a
typed object, and the generator assigns the plain fields directly.

```go
gr := pubfluxcd.CreateGitRepository(ref.Name, namespace)
gr.Spec.URL = ref.URL
gr.Spec.Interval = metav1.Duration{Duration: g.DefaultInterval}
```

Only writes that the contract admits as sugar keep a helper — appending to a slice field,
or assigning a pointer field through a constructed value, as with
`SetGitRepositoryReference`. See
[Kubernetes Builders](/api-reference/kubernetes-builders/) for the admission
rules.

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

External augmenters may add child layouts that are not represented in the bundle model; integrated placement discovers those layouts and emits the required Flux resources.

## Umbrella Bundles

A `Bundle` with a non-empty `Children` slice becomes an **umbrella**: a parent
Flux Kustomization that aggregates the readiness of its children via
auto-generated `spec.healthChecks`. This gives downstream
consumers a single stable anchor regardless of how many internal tiers the
umbrella contains.

### Resource generation

`ResourceGenerator.createKustomization` detects umbrella bundles and:
- prepends one `HealthChecks` entry per direct child (referencing the child's
  own Kustomization by name/namespace)
- leaves user-supplied `HealthChecks` appended after the auto entries

`spec.wait` is **not** forced to `true` here; it is the caller's `Bundle.Wait` input like any
other field. Forcing it was self-defeating: upstream documents that when `wait` is enabled
"the HealthChecks are ignored", so the auto entries the generator had just built were inert.
Leaving `wait` unset is what makes them take effect. A caller who does set `Wait=true` gets
upstream's whole-of-resources health assessment instead, which also gates on the child
Kustomizations.

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
