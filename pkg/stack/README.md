# Stack - Core Domain Model

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/stack.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/stack)

The `stack` package defines the hierarchical domain model at the heart of Kure. It provides the Cluster, Node, Bundle, and Application abstractions used to describe a complete Kubernetes deployment topology.

## Overview

Kure models Kubernetes infrastructure as a four-level hierarchy:

```
Cluster
  └── Node (tree structure)
        └── Bundle (deployment unit)
              └── Application (workload)
```

Each level maps to a concept in GitOps deployment:

| Level | Purpose | GitOps Mapping |
|-------|---------|----------------|
| **Cluster** | Target cluster | Root directory |
| **Node** | Organizational grouping (e.g., `infrastructure`, `apps`) | Subdirectory tree |
| **Bundle** | Deployment unit with dependencies | Flux Kustomization / ArgoCD Application |
| **Application** | Individual workload or resource set | Kubernetes manifests |

## Key Types

Each Go block on this page, except the three marked as excerpts, is the body of an `Example`
function in `example_test.go`, which `go test` runs: it imports this package as `stack`,
`github.com/go-kure/kure/pkg/stack/fluxcd` for its side effect of registering the `"flux"`
workflow provider, `k8s.io/apimachinery/pkg/runtime/schema`, and `fmt` for the line that prints
what the example built. The `myConfig` type the examples use is the one the
[Optional Validation](#optional-validation) section declares.

### Cluster

The root of the hierarchy, representing a complete cluster configuration.
Exported fields remain available for compatibility, while getter and setter methods support consumers that prefer method-based access.

<!-- doc-example: pkg/stack ExampleNewCluster -->
```go
rootNode := &stack.Node{Name: "flux-system"}

cluster := stack.NewCluster("production", rootNode)
cluster.SetGitOps(&stack.GitOpsConfig{
    Type: "flux",
})
fmt.Println(cluster.GetName(), cluster.GetNode().Name, cluster.GetGitOps().Type)
```
<!-- doc-example:end -->

`GitOpsConfig.Bootstrap` takes a `BootstrapConfig`: the Flux mode (`flux-operator` by default, or
`gotk`), the Flux version (in `gotk` mode, empty or the vendored `fluxcd.GotkVersion` builds
offline; any other value downloads manifests — the named release for `vX.Y.Z`, the latest
release otherwise) and components, and the sync
source — `SourceKind`, `SourceURL`,
`SourceRef` and, in `flux-operator` mode, `SyncName`, the name the operator gives the sync source
and Kustomization it creates (empty leaves it to the operator, which uses the `FluxInstance`
namespace). See [Flux Engine](/api-reference/flux-engine/) for how each field is emitted.

### Node

A tree structure for organizing bundles into logical groups. Nodes can have children (sub-nodes) and a package reference for multi-source deployments.

<!-- doc-example: pkg/stack ExampleNode -->
```go
childNode := &stack.Node{Name: "cert-manager"}
monitoringBundle := &stack.Bundle{Name: "monitoring"}

node := &stack.Node{
    Name:     "infrastructure",
    Children: []*stack.Node{childNode},
    Bundle:   monitoringBundle,
}
fmt.Println(node.Name, node.Children[0].Name, node.Bundle.Name)
```
<!-- doc-example:end -->

### Bundle

A deployment unit corresponding to a single GitOps resource (e.g., a Flux Kustomization). Bundles support dependency ordering via `DependsOn` (pointer-based) or `NamedDependsOn` (name-based, for cross-scope references).

`NewBundle` validates the bundle it builds; fields set afterwards are checked by
`Bundle.Validate`:

<!-- doc-example: pkg/stack ExampleNewBundle -->
```go
apps := []*stack.Application{
    stack.NewApplication("prometheus", "monitoring", &myConfig{Port: 9090}),
}
labels := map[string]string{"team": "platform"}
certManagerBundle := &stack.Bundle{Name: "cert-manager"}

bundle, err := stack.NewBundle("monitoring", apps, labels)
if err != nil {
    panic(err)
}
// Pointer-based — when you hold the bundle object:
bundle.DependsOn = []*stack.Bundle{certManagerBundle}
// Name-based — when you only know the name (e.g. a hook-phase bundle):
bundle.NamedDependsOn = []string{"cert-manager-pre-install"}
bundle.Interval = "10m"

if err := bundle.Validate(); err != nil {
    panic(err)
}
fmt.Println(bundle.Name, bundle.DependsOn[0].Name, bundle.NamedDependsOn, bundle.Interval)
```
<!-- doc-example:end -->

`Interval`, `Timeout` and `RetryInterval` take Go duration strings (`"10m"`,
`"1h30m"`). Empty means the generator's default interval, or, for
`Timeout`/`RetryInterval`, that the field is omitted so Flux's own defaults
apply; any other value that does not parse (`"5 minutes"`, `"5min"`, `"5"`)
is rejected by `Bundle.Validate` and by the Flux generator, never replaced by
the default.

Bundles also support an **umbrella pattern** via `Bundle.Children`. When a
bundle has non-empty `Children`, its generated Flux Kustomization automatically
gets an entry in `spec.healthChecks` for each child,
giving external consumers a single readiness anchor for a group of bundles.
`spec.wait` is left to `Bundle.Wait`: the generator no longer forces it, because
upstream ignores `healthChecks` when `wait` is enabled.
The child bundles' Flux Kustomization CRs are rendered into the parent
bundle's directory. Children must be standalone bundles — they cannot
simultaneously be attached as the `Bundle` of any `stack.Node`.

`Bundle.HealthChecks` can also be set explicitly to monitor specific resources
during reconciliation:

<!-- doc-example: pkg/stack ExampleHealthCheck -->
```go
bundle := &stack.Bundle{Name: "web"}

bundle.HealthChecks = []stack.HealthCheck{
    {APIVersion: "apps/v1", Kind: "Deployment", Name: "web", Namespace: "default"},
}
fmt.Println(bundle.HealthChecks[0].Kind, bundle.HealthChecks[0].Name)
```
<!-- doc-example:end -->

When Children is non-empty, health checks for each child Kustomization are
auto-generated and merged with any user-supplied entries.

**Validation:** `ValidateCluster()` runs automatically in all layout entry
points (`WalkCluster`, `WalkClusterByPackage`) and rejects invalid umbrella
configurations (e.g., shared ownership, children that are also node bundles).
It also rejects a `Node` tree containing a cycle, naming the node where the
cycle closes.

### Application

An individual Kubernetes workload. Applications use the `ApplicationConfig` interface to generate their resources.

<!-- doc-example: pkg/stack ExampleApplication_Generate -->
```go
prometheusConfig := &myConfig{Port: 9090}

app := stack.NewApplication("prometheus", "monitoring", prometheusConfig)
resources, err := app.Generate()
if err != nil {
    panic(err)
}
fmt.Println(len(resources), (*resources[0]).GetName())
```
<!-- doc-example:end -->

### ApplicationConfig Interface

Implement this interface to define how an application generates its Kubernetes resources:

<!-- doc-example:excerpt the interface declaration from application.go, not a call -->
```go
type ApplicationConfig interface {
    Generate(*Application) ([]*client.Object, error)
}
```

### Optional Validation

`ApplicationConfig` implementations can optionally implement the `Validator` interface to validate configuration before resource generation:

<!-- doc-example:excerpt the interface declaration from application.go, not a call -->
```go
type Validator interface {
    Validate() error
}
```

When present, `Application.Generate()` calls `Validate()` automatically before `Generate()`. If validation fails, generation stops and the error is returned with application context:

<!-- doc-example:excerpt a caller's own type and methods, declared as in example_test.go -->
```go
type myConfig struct{ Port int }

func (c *myConfig) Validate() error {
    if c.Port <= 0 {
        return errors.New("port must be positive")
    }
    return nil
}

func (c *myConfig) Generate(app *stack.Application) ([]*client.Object, error) {
    // Only called if Validate() passes (or is not implemented)
    cm := kubernetes.CreateConfigMap(app.Name, app.Namespace)
    cm.Data = map[string]string{"port": strconv.Itoa(c.Port)}
    var obj client.Object = cm
    return []*client.Object{&obj}, nil
}
```

`errors` is `github.com/go-kure/kure/pkg/errors`, `kubernetes` is
`github.com/go-kure/kure/pkg/kubernetes`, and `client` is
`sigs.k8s.io/controller-runtime/pkg/client`.

Validation errors are wrapped with application name and namespace:

```
validation failed for application "web" in namespace "prod": port must be positive
```

Configs that do not implement `Validator` continue to work without changes.

## Fluent Builder API

For ergonomic cluster construction, use the fluent builder. Builder methods
use **copy-on-write semantics** — each `With*` call returns a new builder
instance, leaving the original unchanged. `Build` returns the cluster, or every error the chain
collected, joined:

<!-- doc-example: pkg/stack ExampleNewClusterBuilder -->
```go
appConfig := &myConfig{Port: 9090}

cluster, err := stack.NewClusterBuilder("production").
    WithNode("infrastructure").
    WithBundle("monitoring").
    WithApplication("prometheus", appConfig).
    End().
    End().
    Build()
if err != nil {
    panic(err)
}
fmt.Println(cluster.Name, cluster.Node.Name, cluster.Node.Bundle.Name)
```
<!-- doc-example:end -->

## Workflow System

The package provides a pluggable workflow abstraction for GitOps tool integration:

<!-- doc-example: pkg/stack ExampleNewWorkflow -->
```go
cluster, err := stack.NewClusterBuilder("production").
    WithNode("infrastructure").
    WithBundle("monitoring").
    WithApplication("prometheus", &myConfig{Port: 9090}).
    End().
    End().
    Build()
if err != nil {
    panic(err)
}

// Create a workflow for your GitOps tool
wf, err := stack.NewWorkflow("flux")
if err != nil {
    panic(err)
}

// Generate GitOps resources from the cluster definition
objects, err := wf.GenerateFromCluster(cluster)
if err != nil {
    panic(err)
}
for _, obj := range objects {
    fmt.Println(obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
}
```
<!-- doc-example:end -->

`GenerateFromCluster` walks the cluster with `layout.DefaultLayoutRules()`: every Flux
Kustomization `spec.path` and ArgoCD Application `source.path` is a directory that walk writes.
To write the layout with other rules, use `CreateLayoutWithResources`, which generates from the
layout it walks.

Supported workflow providers: `"flux"` / `"fluxcd"` and `"argo"` / `"argocd"`. Each is registered by
importing its package, and `NewWorkflow` returns that package's `*fluxcd.WorkflowEngine` or
`*argocd.WorkflowEngine`; without the import it fails with an error naming the package to import.

## Source References

Bundles and nodes can reference different source types for multi-source deployments: a bundle
through its `SourceRef` field, a node through its `PackageRef`, a `schema.GroupVersionKind`:

<!-- doc-example: pkg/stack ExampleSourceRef -->
```go
// A bundle's SourceRef names the Flux source its Kustomization reads from;
// with URL set, the generator creates that source as well.
bundle := &stack.Bundle{Name: "apps"}
bundle.SourceRef = &stack.SourceRef{
    Kind:      "OCIRepository",
    Name:      "my-registry",
    Namespace: "flux-system",
    URL:       "oci://registry.example.com/manifests",
    Tag:       "v1.0.0",
}

// A node's PackageRef names the package its subtree is bundled into;
// child nodes without one inherit it.
node := &stack.Node{Name: "apps", Bundle: bundle}
node.SetPackageRef(&schema.GroupVersionKind{
    Group:   "source.toolkit.fluxcd.io",
    Version: "v1",
    Kind:    "OCIRepository",
})
fmt.Println(bundle.SourceRef.URL, node.GetPackageRef().Kind)
```
<!-- doc-example:end -->

## Related Packages

- [stack/fluxcd](/api-reference/flux-engine/) - FluxCD workflow engine implementation
- [stack/layout](/api-reference/layout/) - Manifest directory organization
