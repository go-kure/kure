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
`k8s.io/apimachinery/pkg/apis/meta/v1/unstructured`, `os`, `path/filepath`, and `fmt`
for the lines that print what the example built. Two helpers are declared in the same file:
`exampleCluster()` builds a cluster whose one node `apps` holds one bundle `web` with one
application, which emits a ConfigMap through `github.com/go-kure/kure/pkg/kubernetes` and
`sigs.k8s.io/controller-runtime/pkg/client`; `printFiles(dir)` prints every file below `dir`.

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
printFiles(dir)
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

// Generate ArgoCD Applications from a cluster: each source.path is a
// directory a walk with the rules you pass writes
objects, err := engine.GenerateFromCluster(cluster, layout.DefaultLayoutRules())
if err != nil {
    panic(err)
}
for _, obj := range objects {
    path, _, _ := unstructured.NestedString(obj.(*unstructured.Unstructured).Object, "spec", "source", "path")
    fmt.Println(obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), path)
}
```
<!-- doc-example:end -->

`GenerateFromCluster` walks the cluster with the `layout.LayoutRules` you pass (the ones you write the tree with) and produces one ArgoCD `Application` (`argoproj.io/v1alpha1`) per directory that renders bundles, umbrella children included. Each Application's `spec.source.path` is that directory (`layout.OriginIndex.KustomizationPath`), not a path guessed from bundle names. `spec.destination.server` defaults to `https://kubernetes.default.svc`. The rules are held to what `CreateLayoutWithResources` accepts: a value that is not `layout.LayoutRules` (`nil` included) and an integrated `FluxPlacement` are refused, and so is every rule the walk refuses (`layout.LayoutRules.Validate`), also when the cluster is nil or has no root node: such a cluster yields no Application only with valid rules.

The root node's directory renders no bundle (go-kure/kure#979): with a flat `BundleGrouping` the root node's bundle has a directory inside it, named after the bundle (by its `DirName` when it sets one), and that is its Application's `spec.source.path` (`platform/platform-bundle` for root node `platform` with bundle `platform-bundle` and no `DirName`; before, `platform`). See "The root node's bundles" in the layout package README.

When a `GroupFlat` axis merges several bundles into one directory, they share one Application, as they share one Flux Kustomization (see the fluxcd package's "One Kustomization per directory"). It is named after the first bundle. An Application takes its bundle's `KustomizationName` when the bundle sets one, and `spec.dependencies` names a dependency the same way (`Bundle.UnitName`); `source.path` stays the bundle's directory. That directory is named by the bundle's `DirName` wherever the bundle's name becomes a directory (an umbrella child, and the root node's bundle, whose directory takes the first merged bundle's `DirName`): `source.path` follows it and the Application's name does not, and two bundles that would share a directory of their own are refused. Their labels are combined, and one label key with two values is an error. `spec.dependencies` names the Applications of the directories each bundle's `DependsOn` renders, and dependencies between the merged bundles are dropped. With the default rules every bundle has its own directory, so this is one Application per bundle.

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

`CreateLayoutWithResources` generates the base manifest layout via `layout.WalkCluster` with the caller's rules, generates the Applications from that same layout (so every `source.path` is a directory it writes), then appends an `argocd/` child layout containing them. The `argocd/` directory sits inside the root layout's own directory, where the root's `kustomization.yaml` references it. It must be free: a root node's bundle or a child node rendered to it (named `argocd` in whatever case, or a name that resolves to it such as `./argocd`; `ManifestLayout.SameDirectory` in the layout package) is refused, naming the bundle or node (since go-kure/kure#979 the root node's bundle has a directory named after it there, by its `DirName` when it sets one, which is what the check compares). An integrated `FluxPlacement` (`FluxIntegratedPerLayout`, `FluxIntegratedPerBundle`) is refused: the writer would then reference child layouts through Flux CRs, which an Argo layout does not have, so nothing would apply `argocd/`. Every other rule is validated by the walk (`layout.LayoutRules.Validate`): an unknown option value or a `ClusterName` with a `..` path segment is an error naming the field, and no layout is returned.

A `KustomizationRecursive` layout gets no `kustomization.yaml`, and the Applications do not set
`source.directory.recurse` (go-kure/kure#144), so Argo CD applies only the manifest files at the
top of such a directory.

## Known Limitations

- **Bootstrap not implemented**: `GenerateBootstrap` returns `nil, nil` when `config` is nil or disabled; returns an error when bootstrap is enabled. `SupportedBootstrapModes()` returns nil. It takes the layout rules because the `stack.Workflow` interface passes them (go-kure/kure#979) and reads nothing from them: rules that are nil, of another type than `layout.LayoutRules`, or invalid change nothing here, where the Flux engine refuses all three.
- Applications are generated as `unstructured.Unstructured` objects; ArgoCD CRD types are not imported.
- `IntegrateWithLayout` adds nothing to the layout (ArgoCD Applications reference external repos and do not require layout integration).
- **No delivery intent**: an application that sets `Application.Delivery` (prune protection, force replace) is refused by `GenerateFromCluster`, `CreateLayoutWithResources` and `IntegrateWithLayout`, with an error naming the application; `IntegrateWithLayout` also reads the cluster it is given, so a hand-built layout does not get past it. This workflow has no mapping for the intent yet, and rendering the application without it would drop what it asked for. Leave `Delivery` unset, or use the Flux workflow.
- **No Application for a node**: an Application is generated for a bundle only. A node that sets `KustomizationName`, `DependsOn` or `NamedDependsOn`, which name and order a node's own Flux Kustomization, is refused by `GenerateFromCluster`, `CreateLayoutWithResources` and `IntegrateWithLayout` rather than ignored; the error names the node and the fields. Name and order the node's bundles instead.

## Related Packages

- [stack/fluxcd](/api-reference/flux-engine/) — full-featured FluxCD engine including bootstrap
- [stack](/api-reference/stack/) — domain model and Workflow interface
- [stack/layout](/api-reference/layout/) — manifest layout generation
