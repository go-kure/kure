# ArgoCD Engine - ArgoCD Workflow Implementation

The `argocd` package implements the `stack.Workflow` interface for ArgoCD, generating ArgoCD `Application` resources from Kure's domain model.

> **Bootstrap not implemented.** `GenerateBootstrap` returns an error when bootstrap is enabled. If bootstrap generation is required, use the FluxCD engine instead.

## Quick Start

```go
import (
    _ "github.com/go-kure/kure/pkg/stack/argocd" // registers "argocd" provider
    "github.com/go-kure/kure/pkg/stack"
    "github.com/go-kure/kure/pkg/stack/layout"
)

// Use via the stack.Workflow registry
wf, err := stack.NewWorkflow("argocd")
ml, err := wf.CreateLayoutWithResources(cluster, layout.LayoutRules{})
_ = ml.WriteToDisk("./clusters/prod")
```

## Engine Construction

```go
// Direct construction (bypasses registry)
engine := argocd.Engine()

// Configure source repository and namespace
engine.SetRepoURL("https://github.com/example/manifests.git")
engine.SetDefaultNamespace("argocd")
```

## Resource Generation

```go
// Generate ArgoCD Applications from a cluster
objects, err := engine.GenerateFromCluster(cluster)
```

`GenerateFromCluster` walks the cluster with `layout.DefaultLayoutRules()` and produces one ArgoCD `Application` (`argoproj.io/v1alpha1`) per directory that renders bundles, umbrella children included. Each Application's `spec.source.path` is that directory (`layout.OriginIndex.KustomizationPath`), not a path guessed from bundle names. `spec.destination.server` defaults to `https://kubernetes.default.svc`.

When a `GroupFlat` axis or `FlattenSingleTier` merges several bundles into one directory, they share one Application, as they share one Flux Kustomization (see the fluxcd package's "One Kustomization per directory"). It is named after the first bundle. Their labels are combined, and one label key with two values is an error. `spec.dependencies` names the Applications of the directories each bundle's `DependsOn` renders, and dependencies between the merged bundles are dropped. With the default rules every bundle has its own directory, so this is one Application per bundle.

## Layout Integration

```go
// Create layout with Applications placed in an argocd/ subdirectory
ml, err := engine.CreateLayoutWithResources(cluster, layout.LayoutRules{})

// Integrate Applications into an existing layout
err = engine.IntegrateWithLayout(ml, cluster, layout.LayoutRules{})
```

`CreateLayoutWithResources` generates the base manifest layout via `layout.WalkCluster` with the caller's rules, generates the Applications from that same layout (so every `source.path` is a directory it writes), then appends an `argocd/` child layout containing them. The `argocd/` directory sits inside the root layout's own directory, where the root's `kustomization.yaml` references it. An integrated `FluxPlacement` (`FluxIntegratedPerLayout`, `FluxIntegratedPerBundle`) is refused: the writer would then reference child layouts through Flux CRs, which an Argo layout does not have, so nothing would apply `argocd/`.

## Known Limitations

- **Bootstrap not implemented**: `GenerateBootstrap` returns `nil, nil` when `config` is nil or disabled; returns an error when bootstrap is enabled. `SupportedBootstrapModes()` returns nil.
- Applications are generated as `unstructured.Unstructured` objects; ArgoCD CRD types are not imported.
- `IntegrateWithLayout` is a no-op (ArgoCD Applications reference external repos and do not require layout integration).

## Related Packages

- [stack/fluxcd](/api-reference/flux-engine/) — full-featured FluxCD engine including bootstrap
- [stack](/api-reference/stack/) — domain model and Workflow interface
- [stack/layout](/api-reference/layout/) — manifest layout generation
