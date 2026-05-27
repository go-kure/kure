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

// Generate from a specific node subtree
objects, err := engine.GenerateFromNode(node)

// Generate from a single bundle
objects, err := engine.GenerateFromBundle(bundle)
```

Each `Bundle` in the cluster produces one ArgoCD `Application` (`argoproj.io/v1alpha1`). The Application's `spec.source.path` is derived from the bundle's ancestry in the node tree. `spec.destination.server` defaults to `https://kubernetes.default.svc`.

## Layout Integration

```go
// Create layout with Applications placed in an argocd/ subdirectory
ml, err := engine.CreateLayoutWithResources(cluster, layout.LayoutRules{})

// Integrate Applications into an existing layout
err = engine.IntegrateWithLayout(ml, cluster, layout.LayoutRules{})
```

`CreateLayoutWithResources` generates the base manifest layout via `layout.WalkCluster`, then appends an `argocd/` child layout containing the generated Applications.

## Known Limitations

- **Bootstrap not implemented**: `GenerateBootstrap` returns `nil, nil` when `config` is nil or disabled; returns an error when bootstrap is enabled. `SupportedBootstrapModes()` returns nil.
- Applications are generated as `unstructured.Unstructured` objects; ArgoCD CRD types are not imported.
- `IntegrateWithLayout` is a no-op (ArgoCD Applications reference external repos and do not require layout integration).

## Related Packages

- [stack/fluxcd](../fluxcd/) — full-featured FluxCD engine including bootstrap
- [stack](../) — domain model and Workflow interface
- [stack/layout](../layout/) — manifest layout generation
