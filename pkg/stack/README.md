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
[Optional Validation](#optional-validation) section declares; its `Generate` method also needs
`strconv`.

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
release otherwise; in `flux-operator` mode it is required, as is `Registry`, and an empty value
is an error) and components, and the sync
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

A node whose directory renders no bundle (a group of nodes, say) is applied by a Flux Kustomization
of its own under the layout rule `FluxPlacement: FluxIntegratedPerLayout`. `KustomizationName` names
it, instead of the name derived from the node's path, and `DependsOn` (nodes) and `NamedDependsOn`
(Kustomization names) order it. On any other node, under another placement and under the ArgoCD
workflow the three fields are refused rather than ignored. See
[Flux Engine](/api-reference/flux-engine/#node-kustomizations).

### Bundle

A deployment unit corresponding to a single GitOps resource (e.g., a Flux Kustomization). Bundles support dependency ordering via `DependsOn` (pointer-based) or `NamedDependsOn` (name-based, for cross-scope references).

The generated resource is named after the bundle. `KustomizationName` gives it another name
without renaming the bundle or moving its directory; `UnitName()` returns the name in effect.
Under the ArgoCD workflow the same value names the Application. A `NamedDependsOn` entry names
such a resource, so it reaches a bundle that sets `KustomizationName` by that value.

A nil `DependsOn` entry is refused by `Bundle.Validate`, with the bundle's path (an umbrella
descendant's from the bundle that was validated: `platform/infra/db`) and the entry's index; the
Flux generator refuses it the same way for a bundle it is handed unvalidated (`GenerateForBundle`).

A `DependsOn` entry need not be the cluster's own bundle: another `Bundle` value with the same
`Name` is a copy of it, and a cluster resolves it by `Name` to that bundle, so the dependency is on
that bundle's Kustomization. Leave `KustomizationName` empty on a copy, or set the name of the
bundle's Kustomization (its `UnitName()`: the bundle's `KustomizationName`, or its `Name` when it
sets none). `ValidateCluster` refuses, before it validates the bundles one by one:

- a copy that sets a `KustomizationName` other than that name, naming both;
- a copy of a bundle whose Kustomization name is also in the dependant's `NamedDependsOn`: one
  dependency in both lists.

`Bundle.Validate` sees one bundle and no cluster, so it compares a `DependsOn` entry with
`NamedDependsOn` on the name that entry carries itself. One valid input is refused for that reason:
a name-only copy of a bundle that sets `KustomizationName`, beside a `NamedDependsOn` entry equal
to the bundle's `Name` (which then means another Kustomization), is reported as one dependency in
both lists. Set the bundle's `KustomizationName` on the copy to say which one is meant.

A bundle's directory is named after the bundle as well. `DirName` gives it another name
(`00-infra` for the bundle `shop-infra`, say), and the path the generated resource applies follows
it; the resource's name does not. The rule is one: wherever a bundle's name becomes a directory,
`DirName` names it, or else `Name`. That is the case for an umbrella child, for a node's bundle
under the layout rule `BundleGrouping: GroupByName`, and for the root node's bundle under
`GroupFlat`, which has a directory inside the root node's. Under `GroupFlat` below the root a
node's bundle is rendered in its node's directory and `DirName` has no effect on it. When a flat
`NodeGrouping` merges several bundles into the root node, the directory they share takes the first
merged bundle's `DirName`, or its `Name`; a `DirName` on a later one has no effect.

A `DirName` must be one path segment (`ValidateDirectoryName`): not `.` or `..`, and without `/`,
`\` or a NUL byte. `Bundle.Validate` checks it on the bundle and on every umbrella child, whether
or not the layout rules give the bundle a directory. It is no object name, so it need not be a
DNS-1123 subdomain: `00_Infra` is valid.

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
the default. The Flux generator also refuses a duration the Flux API does not
take, a negative one or a positive one under a millisecond (`"-1s"`, `"1us"`), where it
writes the bundle's Kustomization; `Bundle.Validate` accepts those, since the rule is
Flux's (see the Flux engine's "Durations").

Bundles also support an **umbrella pattern** via `Bundle.Children`. When a
bundle has non-empty `Children`, its generated Flux Kustomization automatically
gets an entry in `spec.healthChecks` for each child while `Bundle.Wait` is unset
or false,
giving external consumers a single readiness anchor for a group of bundles.
`spec.wait` is left to `Bundle.Wait`: the generator no longer forces it, because
upstream ignores `healthChecks` when `wait` is enabled, and with `Wait` true it
writes no entry for the children. Whether `wait` then covers the children
depends on the Flux placement (see the Flux engine's "Umbrella Bundles").
Under the integrated Flux placements the child bundles' Flux Kustomization CRs
are rendered into the parent bundle's directory; under `FluxSeparate` they go to
`flux-system/` with every other one. Children must be standalone bundles — they cannot
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

When Children is non-empty and `Wait` is not true, health checks for each child
Kustomization are auto-generated and merged with any user-supplied entries.
The user-supplied entries are handled as before; dropping the child entries
under `Wait` does not touch them.

**Validation:** `ValidateCluster()` runs automatically in all layout entry
points (`WalkCluster`, `WalkClusterByPackage`) and rejects invalid umbrella
configurations (e.g., shared ownership, children that are also node bundles).
It also rejects a `Node` tree containing a cycle, naming the node where the
cycle closes.

**Names:** a bundle name and a node name are checked when the model is
validated, and a limit that only one delivery engine has is checked by that
engine's workflow when it generates, so a name that could never be applied is
refused before anything is written. Nothing is shortened or rewritten: the
caller chooses a valid name.

| Name | Rule | Checked by |
|---|---|---|
| Bundle, including every umbrella child | a DNS-1123 subdomain and a directory name | `Bundle.Validate` |
| A bundle's `KustomizationName`, when set | a DNS-1123 subdomain | `Bundle.Validate` |
| The name a bundle's Kustomization gets in the Flux workflow (`UnitName()`) | a Flux Kustomization name | the Flux generator, where it builds the Kustomization |
| Node | a directory name, under every layout grouping; an unnamed root is allowed, it adds no path segment | `ValidateCluster` |
| The name a node's own Kustomization gets in the Flux workflow (its `KustomizationName`, or the derived `<path>-node`) | a Flux Kustomization name | the Flux layout integrator, where it creates the Kustomization |
| An entry of a node's `NamedDependsOn` | not empty and not repeated; its value is a caller-supplied reference and is not checked, as for a bundle's | `ValidateCluster` |
| Application | a directory name, only where the layout rules give the application a directory | `layout.WalkCluster` |

- **Bundle name**: a DNS-1123 subdomain, at most 253 characters: lower-case
  letters, digits, `-` and `.`, starting and ending with a letter or digit.
  `my.app` is valid; `My-App` is not. This is what holds for every delivery
  engine, because the engine applies the bundle with an object named after it
  (a Flux Kustomization, an ArgoCD Application). A `KustomizationName` names
  that object instead of the bundle's name, so it is a DNS-1123 subdomain too;
  it is no directory name. The bundle's `Name` is checked the same with or
  without it.
- **Kustomization name** (`ValidateKustomizationName`): a DNS-1123 subdomain
  of at most `KustomizationNameMaxLength` (63) characters. The limit is Flux's,
  so `Bundle.Validate` and `ValidateCluster` do not apply it: the Flux
  workflow refuses a longer name where it builds the bundle's
  Kustomization, before anything is written, and the same name validates and
  renders an Application in the ArgoCD workflow. The name checked is the one
  the Kustomization gets, `UnitName()`: the `KustomizationName`, or the `Name`
  without one, so a `Name` over the limit beside a `KustomizationName` within
  it generates. The limit is 63 and not the
  253 of an object name because
  Flux writes the Kustomization's name into a label value on every object it
  applies. Read in kustomize-controller v1.9.5, the version this module pins:
  the reconciler hands the name to the apply manager as the owner of the
  objects (`internal/controller/kustomization_controller.go:462`), and the
  manager sets it as the value of the `kustomize.toolkit.fluxcd.io/name` label
  (`github.com/fluxcd/pkg/ssa` v0.76.2, `manager.go:66-78`).
- **Directory name** (`ValidateDirectoryName`): one path segment. It is not
  empty, not `.` or `..`, and holds neither `/` nor `\` nor a NUL byte. The
  backslash is refused on every platform, so a name such as `..\outside` cannot
  leave its directory where the backslash is a separator. A NUL byte is refused
  because a directory with one in its name cannot be created; characters that
  only some file systems refuse are not checked.

A node name is checked whatever the layout rules are, also where flat node
grouping absorbs the node into its parent's directory. The name is a segment
of the node's path in the model before it is a directory: `Node.GetPath` and
the path map join node names with `/`, so two unnamed children of one node
have the same path, and so do a node named `a/b` and a node `b` below a
sibling `a`. A cluster with an unnamed node below the
root, or with a separator in a node name, is therefore refused under every
grouping.

An error from either rule names where the name sits: a bundle by its path from
the bundle that was validated (`platform/platform-infra`), a node by its path
from the root, and `ValidateCluster` adds the node a refused bundle is attached
to (`bundle "web" at node "apps/web" failed validation`). An umbrella child
without a name is refused earlier, by the check on `Children`, which names its
parent and the child's index. Both functions are exported so that code deriving
a name from these fields can check the result with the same rule. The Flux
generator's refusal of a Kustomization name over 63 characters names the
bundle by `Bundle.GetPath`, which for an umbrella child of a walked cluster
holds its umbrella's path (`platform/platform-infra`), and the field that
holds the name (`name` or `kustomizationName`). Its refusal of the name of a
node's own Kustomization names the node by its path from the root and the
field `kustomizationName`, also where the name was derived and the field is
the one to set. Two Flux
entry points take a bundle or a node that no cluster validation has seen and
apply a rule themselves: the generator checks the whole Kustomization name
rule on a bundle it is handed directly (`GenerateForBundle`), and the bootstrap
generator checks the root node's name as a directory name wherever it builds a
path from it.

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

#### Delivery intent

`Application.Delivery` states how the delivery engine should treat the application's objects,
in engine-neutral terms. Both fields default to off, and an application that sets neither renders
exactly as before.

| Field | Meaning |
|---|---|
| `PruneProtection` | Keep the application's objects in the cluster when they are removed from the source. |
| `ForceReplace` | Allow an object to be deleted and recreated when a change touches an immutable field. |

The intent covers every object the application emits, the objects a layout augmenter adds to the
application's own directory, and the ConfigMaps built from that directory's `configMapGenerator`
entries. Each workflow engine maps it to its own mechanism: the Flux workflow writes per-object
annotations (see [Flux Engine](/api-reference/flux-engine/)); the ArgoCD workflow has no mapping
yet and refuses an application that sets one. The Flux `GenerateFromCluster` refuses one too: it
returns Kustomizations and Sources and none of the application's objects, so use
`CreateLayoutWithResources`. `Bundle.Prune` and `Bundle.Force` stay the
bundle-wide switches on the generated Flux Kustomization and are independent of this field.

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

// Generate GitOps resources from the cluster definition, with the layout
// rules the tree is written with
objects, err := wf.GenerateFromCluster(cluster, layout.DefaultLayoutRules())
if err != nil {
    panic(err)
}
for _, obj := range objects {
    fmt.Println(obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
}
```
<!-- doc-example:end -->

`GenerateFromCluster` walks the cluster with the layout rules you pass: every Flux
Kustomization `spec.path` and ArgoCD Application `source.path` is a directory that walk writes,
so pass the rules you write the tree with (`layout.DefaultLayoutRules()` for the defaults). The
rules must be a `layout.LayoutRules` value; anything else, `nil` included, is refused. Neither
engine returns a list for rules with `FluxIntegratedPerLayout` (the Flux Kustomizations that
apply that tree's child directories exist only in a layout): use `CreateLayoutWithResources`
with the Flux engine for that placement.
`CreateLayoutWithResources` takes the same rules and also returns the layout it walked, with
the resources placed in it.
`GenerateBootstrap(config, rootNode, rules)` takes the same rules too, and points Flux at the top
directory of the tree a walk with them writes (see [Flux Engine](/api-reference/flux-engine/)).

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
