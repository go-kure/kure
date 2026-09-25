# ArgoCD Engine - ArgoCD Workflow Implementation

The `argocd` package implements the `stack.Workflow` interface for ArgoCD, generating ArgoCD `Application` resources from Kure's domain model.

> **Bootstrap not implemented.** `GenerateBootstrap` returns an error when bootstrap is enabled. If bootstrap generation is required, use the FluxCD engine instead.

## Quick Start

<!-- doc-example:excerpt the import block alone; the blank import is what registers the provider -->
```go
import (
    _ "github.com/go-kure/kure/pkg/stack/argocd" // registers "argocd" provider
    "github.com/go-kure/kure/pkg/stack"
    "github.com/go-kure/kure/pkg/stack/layout"
)
```

Every other Go block on this page is the body of an `Example` function in `example_test.go`,
which `go test` runs: it imports this package as `argocd`, `stack` and `layout` as above,
`k8s.io/apimachinery/pkg/apis/meta/v1/unstructured`, `io/fs`, `os`, `path/filepath`, and `fmt`
for the lines that print what the example built. `exampleCluster()`, declared in the same file,
builds a cluster whose one node `apps` holds one bundle `web` with one application, which emits a
ConfigMap through `github.com/go-kure/kure/pkg/kubernetes` and
`sigs.k8s.io/controller-runtime/pkg/client`.

<!-- doc-example: pkg/stack/argocd Example -->
```go
cluster := exampleCluster()
dir, err := os.MkdirTemp("", "kure-argocd-example")
if err != nil {
    panic(err)
}
defer func() { _ = os.RemoveAll(dir) }()

// Use via the stack.Workflow registry
wf, err := stack.NewWorkflow("argocd")
if err != nil {
    panic(err)
}
ml, err := wf.CreateLayoutWithResources(cluster, layout.LayoutRules{})
if err != nil {
    panic(err)
}
if err := ml.WriteToDisk(filepath.Join(dir, "clusters/prod")); err != nil {
    panic(err)
}

err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
    if err == nil && !d.IsDir() {
        rel, _ := filepath.Rel(dir, path)
        fmt.Println(rel)
    }
    return err
})
if err != nil {
    panic(err)
}
```
<!-- doc-example:end -->

## Engine Construction

<!-- doc-example: pkg/stack/argocd ExampleEngine -->
```go
// Direct construction (bypasses registry)
engine := argocd.Engine()

// Configure source repository and namespace
engine.SetRepoURL("https://github.com/example/manifests.git")
engine.SetDefaultNamespace("argocd")
fmt.Println(engine.GetName(), engine.RepoURL, engine.DefaultNamespace)
```
<!-- doc-example:end -->

## Resource Generation

<!-- doc-example: pkg/stack/argocd ExampleWorkflowEngine_GenerateFromCluster -->
```go
cluster := exampleCluster()
engine := argocd.Engine()

// Generate ArgoCD Applications from a cluster
objects, err := engine.GenerateFromCluster(cluster)
if err != nil {
    panic(err)
}
for _, obj := range objects {
    path, _, _ := unstructured.NestedString(obj.(*unstructured.Unstructured).Object, "spec", "source", "path")
    fmt.Println(obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), path)
}
```
<!-- doc-example:end -->

`GenerateFromCluster` walks the cluster with `layout.DefaultLayoutRules()` and produces one ArgoCD `Application` (`argoproj.io/v1alpha1`) per directory that renders bundles, umbrella children included. Each Application's `spec.source.path` is that directory (`layout.OriginIndex.KustomizationPath`), not a path guessed from bundle names. `spec.destination.server` defaults to `https://kubernetes.default.svc`.

When a `GroupFlat` axis or `FlattenSingleTier` merges several bundles into one directory, they share one Application, as they share one Flux Kustomization (see the fluxcd package's "One Kustomization per directory"). It is named after the first bundle. Their labels are combined, and one label key with two values is an error. `spec.dependencies` names the Applications of the directories each bundle's `DependsOn` renders, and dependencies between the merged bundles are dropped. With the default rules every bundle has its own directory, so this is one Application per bundle.

## Layout Integration

`CreateLayoutWithResources` returns the `stack.ManifestLayoutResult` interface;
`IntegrateWithLayout` takes the concrete `*layout.ManifestLayout` behind it:

<!-- doc-example: pkg/stack/argocd ExampleWorkflowEngine_CreateLayoutWithResources -->
```go
cluster := exampleCluster()
engine := argocd.Engine()

// Create layout with Applications placed in an argocd/ subdirectory
result, err := engine.CreateLayoutWithResources(cluster, layout.LayoutRules{})
if err != nil {
    panic(err)
}
ml := result.(*layout.ManifestLayout)

// Integrate Applications into an existing layout (a no-op for ArgoCD)
if err := engine.IntegrateWithLayout(ml, cluster, layout.LayoutRules{}); err != nil {
    panic(err)
}
for _, child := range ml.Children {
    fmt.Println(child.FullRepoPath(), len(child.Resources))
}
```
<!-- doc-example:end -->

`CreateLayoutWithResources` generates the base manifest layout via `layout.WalkCluster` with the caller's rules, generates the Applications from that same layout (so every `source.path` is a directory it writes), then appends an `argocd/` child layout containing them. The `argocd/` directory sits inside the root layout's own directory, where the root's `kustomization.yaml` references it. An integrated `FluxPlacement` (`FluxIntegratedPerLayout`, `FluxIntegratedPerBundle`) is refused: the writer would then reference child layouts through Flux CRs, which an Argo layout does not have, so nothing would apply `argocd/`.

## Known Limitations

- **Bootstrap not implemented**: `GenerateBootstrap` returns `nil, nil` when `config` is nil or disabled; returns an error when bootstrap is enabled. `SupportedBootstrapModes()` returns nil.
- Applications are generated as `unstructured.Unstructured` objects; ArgoCD CRD types are not imported.
- `IntegrateWithLayout` is a no-op (ArgoCD Applications reference external repos and do not require layout integration).

## Related Packages

- [stack/fluxcd](/api-reference/flux-engine/) — full-featured FluxCD engine including bootstrap
- [stack](/api-reference/stack/) — domain model and Workflow interface
- [stack/layout](/api-reference/layout/) — manifest layout generation
