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

Start with Quick Start and Engine Construction below, then read the page for the task in front of
you:

| If you need | Read |
|---|---|
| The Flux objects a cluster produces | Resource Generation, below |
| How Kustomizations are named and where their paths point | [Flux Names and Paths](/api-reference/flux-names-and-paths/) |
| Where the files land and how Kustomizations point at them | [Flux Layout Integration](/api-reference/flux-layout-integration/), [Flux Umbrella Bundles](/api-reference/flux-umbrella-bundles/), [Flux Non-Bundle Child Layout CRs](/api-reference/flux-child-layout-crs/) |
| To install Flux itself | [Flux Bootstrap Generation](/api-reference/flux-bootstrap-generation/) |
| The settings and defaults the engine takes | [Flux Defaults](/api-reference/flux-defaults/), Configuration and Service account, below |
| What the engine refuses | Validation, below |

This page used to hold all of these. A link to a section that moved opens this page. The table
above names the page that holds it.

## Quick Start

<!-- doc-example:excerpt the import line alone, which every example on this page uses -->
```go
import "github.com/go-kure/kure/pkg/stack/fluxcd"
```

Every other Go block on this page, except the one marked as an excerpt of the generator's own
source, is the body of an `Example` function in `example_test.go`, which `go test` runs: it
imports this package as `fluxcd`, `github.com/go-kure/kure/pkg/stack` as `stack`,
`github.com/go-kure/kure/pkg/stack/layout` as `layout`, `kustv1`
(`github.com/fluxcd/kustomize-controller/api/v1`), `os`, and `fmt` for the lines that print what
the example built. Two helpers are declared in the same file: `exampleCluster()` builds cluster
`prod`, whose root node `apps` holds one bundle `web` with one application, which emits a
ConfigMap through `github.com/go-kure/kure/pkg/kubernetes` and
`sigs.k8s.io/controller-runtime/pkg/client`; `printFiles(dir)` prints every file below `dir`.

<!-- doc-example: pkg/stack/fluxcd ExampleEngine -->
```go
cluster := exampleCluster()

// Create engine with defaults. Placement is set on the LayoutRules passed
// to the layout call, not on the engine — see the Layout Integration page.
engine := fluxcd.Engine()

// Generate all Flux resources for a cluster: each spec.path is a directory
// a walk with the rules you pass writes
objects, err := engine.GenerateFromCluster(cluster, layout.DefaultLayoutRules())
if err != nil {
    panic(err)
}
for _, obj := range objects {
    fmt.Println(obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
}
```
<!-- doc-example:end -->

## Engine Construction

<!-- doc-example: pkg/stack/fluxcd ExampleNewWorkflowEngine -->
```go
// Default engine
engine := fluxcd.Engine()

// The same, built from its components
built := fluxcd.NewWorkflowEngine()
fmt.Println(engine.GetName() == built.GetName(), built.SupportedBootstrapModes())
```
<!-- doc-example:end -->

The engine has no path mode: every Kustomization `spec.path` is a layout directory (see
[Kustomization paths](/api-reference/flux-names-and-paths/#kustomization-paths)).

Placement (FluxIntegratedPerLayout vs FluxSeparate) is configured per call on
`layout.LayoutRules.FluxPlacement`. `FluxUnset` is normalized to
`FluxSeparate` by `LayoutIntegrator.CreateLayoutWithResources` — matching
`layout.DefaultLayoutRules()` and the walker. The call's placement is the
tree's: `IntegrateWithLayout` sets it on every layout it integrates,
whatever placement the tree was walked with. A tree that already holds Flux
Kustomizations an earlier integration or the caller placed keeps the placement
they were made for (another is refused; Kustomizations an application emits,
and ones kustomize drops as local-config, do not count), and a refused call
leaves the tree exactly as it was. See
[Layout Integration](/api-reference/flux-layout-integration/).

## Resource Generation

Generate Flux resources from a cluster, from a layout walked from it, or for one bundle:

<!-- doc-example: pkg/stack/fluxcd ExampleResourceGenerator_GenerateFromLayout -->
```go
cluster := exampleCluster()
engine := fluxcd.Engine()
rules := layout.DefaultLayoutRules()
bundle := cluster.Node.Bundle

// From an entire cluster: walks it with the rules you write the tree with
objects, err := engine.GenerateFromCluster(cluster, rules)
if err != nil {
    panic(err)
}
fmt.Println(objects[0].GetName(), objects[0].(*kustv1.Kustomization).Spec.Path)

// From a layout you walked (and will write) yourself
ml, err := layout.WalkCluster(cluster, rules)
if err != nil {
    panic(err)
}
objects, err = engine.ResourceGen.GenerateFromLayout(ml, cluster)
if err != nil {
    panic(err)
}
fmt.Println(objects[0].GetName(), objects[0].(*kustv1.Kustomization).Spec.Path)

// For one bundle, at a path you supply
objects, err = engine.ResourceGen.GenerateForBundle(bundle, "clusters/prod/apps/web")
if err != nil {
    panic(err)
}
fmt.Println(objects[0].GetName(), objects[0].(*kustv1.Kustomization).Spec.Path)
```
<!-- doc-example:end -->

Each directory that renders bundles produces one Flux Kustomization resource (see
[One Kustomization per directory](/api-reference/flux-names-and-paths/#one-kustomization-per-directory)) with:
- `metadata.name` from the bundle (see [Kustomization names](/api-reference/flux-names-and-paths/#kustomization-names))
- `spec.path` = that directory
- Source reference from `Bundle.SourceRef` (and a Source when it has a `URL`)
- Dependency ordering from `Bundle.DependsOn` and `Bundle.NamedDependsOn`
- Interval and pruning configuration
- `metadata.labels` and `metadata.annotations` from `Bundle.Labels` and `Bundle.Annotations`

A Kustomization holds copies of the bundle's label and annotation maps, never the maps
themselves, whichever entry point builds it (`GenerateFromCluster`, `GenerateFromLayout`,
`GenerateForBundle`, `CreateLayoutWithResources`, `IntegrateWithLayout`): editing a Kustomization
kure returns leaves the bundle, and so what later generations from it produce, as it was. An
application's own objects are passed through without copying (a delivery intent that adds
annotations gives the object a new annotations map, see [Delivery intent](/api-reference/flux-layout-integration/#delivery-intent)): an edit of one reaches whatever the
config shares it with, so a config that builds every object, map and pointer per call keeps its
next call unchanged.

`GenerateFromCluster` walks the cluster itself, so it renders every application and runs every
`LayoutAugmenter`: their errors surface there. It walks with the `layout.LayoutRules` it is given,
so its paths are the directories `WalkCluster` writes under those rules: pass the rules you write
the tree with. Under `layout.DefaultLayoutRules()` that is the root node at `<root>`, its bundle
at `<root>/<bundle>` (the bundle's `DirName`, or its `Name` without one) and its children at `<root>/<child>`, which is also what the bootstrap sync
path `./<root>` expects; with
`ClusterName: "."` and an unnamed root node it is the root of the tree, not `cluster`. The objects
come back as a list and are placed nowhere, and the list is the same under `FluxSeparate` and
`FluxIntegratedPerBundle`. Rules with `FluxIntegratedPerLayout` are refused: a tree written with
that placement lists no directory child in its parent's `kustomization.yaml`, and the
Kustomizations that apply those children (application, augmenter and bundle-less node
directories) exist only where the integrator places them, so the list would leave them applied by
nothing. Use `CreateLayoutWithResources` with those rules instead. A cluster in which an
application sets a [delivery intent](/api-reference/flux-layout-integration/#delivery-intent) is refused as well, after the walk and so
after the placement refusal and any walk error: the list holds none of the application's objects,
so the intent could not reach the output. On
`WorkflowEngine` (the `stack.Workflow` interface) the rules arrive as a
`stack.LayoutRulesProvider` and must be a `layout.LayoutRules` value: anything else, `nil`
included, is refused, never replaced by the defaults.

`GenerateFromLayout` takes a layout the caller walked, and refuses it when any layout of the tree
carries `FluxIntegratedPerLayout`, for the same reason: the list holds the Kustomizations of the
bundle directories only, and the ones that apply the other child directories are placed by the
integrator alone. The error names the placement and the first layout, in pre-order, that carries
it; use `CreateLayoutWithResources` or `IntegrateWithLayout` instead. Each layout has its own
`FluxPlacement` and the writers decide per parent, so a tree with placements set by hand is
refused for one such layout below a root that carries another. The placement decides, not the
tree's shape: a per-layout tree with no such child directory is refused although its list would
be complete, as `GenerateFromCluster` refuses the rules whatever the cluster holds. A tree the
integrator already placed per layout is refused as well: it holds every Kustomization it needs.
The integrator's own use of `GenerateFromLayout`, under `FluxSeparate`, is not affected: it sets
its placement on every layout of the tree first.

## Configuration

### Kustomization Mode

Controls how kustomization.yaml files reference resources:

- `KustomizationExplicit` - Lists all manifest files explicitly
- `KustomizationRecursive` - Writes no `kustomization.yaml`; the Flux Kustomization that builds
  the directory generates one from every `.yaml` and `.yml` file below it. The integrator marks
  every directory a Kustomization it generated builds (`SetFluxBuild`), and the writers refuse a
  marked Recursive directory that holds no file (a bundle with no applications, say), since the
  Kustomization would name an empty directory, which a Git tree drops, and one whose build would
  differ from the Explicit mode's: another generated target below it that no `kustomization.yaml`
  shields, or a YAML extra file in its build. See "Kustomization Generation" in the layout package
  README.

The marks also tell the writers what Flux applies. In a tree the integrator generated a
Kustomization for, the writers refuse a directory its parent's `kustomization.yaml` does not list
and no Kustomization builds (an umbrella child, a directory that renders bundles, a directory
child of a `FluxIntegratedPerLayout` parent): nothing would apply it. A tree the integrator built
always passes, since every such directory is the `spec.path` of a Kustomization it generated, or
of one already in the tree that it kept in place of its own (typed, unstructured or inside a
List), and it marks the directory for either. One changed afterwards, or
one you place Kustomizations in yourself, needs `SetFluxBuild` on each directory a Kustomization
of yours builds. The writers also refuse two Flux Kustomizations of one namespace and name
anywhere in a tree, whether or not the integrator ran. See "Layout paths" in the layout package
README.

### Flux Placement

Controls where Flux Kustomization resources are placed:

- `FluxSeparate` - Flux resources collected in a separate `flux-system/` directory inside the root layout's own directory (where the root's `kustomization.yaml` references it); children referenced as directories, except those that render bundles, which their own CRs apply. `WriteToDisk` and `WriteToTar` name its files by `LayoutRules.FileNaming`, like the rest of the tree: `flux-system-kustomization-<name>.yaml` by default, `kustomization-<name>.yaml` with `FileNamingKindName` (go-kure/kure#976; before, always the default pattern). Rules passed to `IntegrateWithLayout` that leave `FileNaming` unset take the root layout's. The directory must be free: a root node's bundle or a child node rendered to it (named `flux-system` in whatever case, or a name that resolves to it such as `./flux-system`; `ManifestLayout.SameDirectory` in the layout package) is refused, naming the bundle or node. Rename it (for a bundle, its directory: its `DirName`, or its `Name` without one), or use an integrated placement. Since go-kure/kure#979 the root node's bundle has a directory named after it there; before, such a bundle was written into the root node's directory.
- `FluxIntegratedPerLayout` - a Flux Kustomization CR for every layout that renders bundles (bundles a `GroupFlat` merge puts in one directory share one CR, named after the first) and for every child layout that is not an umbrella child, not `AppFileSingle` and renders no bundle (augmenter-added child layouts included), hosted in its parent layout (the top of the tree `layout.WalkCluster` returns renders no bundle and gets no CR, go-kure/kure#979); the parent's `kustomization.yaml` lists those CR files as its own resources and references no child directory. Not literally every layout: a layout whose CR name another generated CR already uses, such as the application `web` of the bundle `platform` beside a bundle named `platform-web`, is refused instead (see [Non-Bundle Child Layout CRs](/api-reference/flux-child-layout-crs/)). A Kustomization already in the tree with that name, in the namespace of the generated CRs, is kept in place of a generated one when it sits in the layout that would host it and has the same `spec.path` and Source, without `spec.kubeConfig`, and is refused otherwise; where the layout's Source cannot be resolved, the Source is not compared, but one with `spec.kubeConfig` is still not kept. Finest granularity.
- `FluxIntegratedPerBundle` - Flux Kustomization CRs at **bundle boundaries only**, each hosted in its parent layout; a bundle's interior (application and augmenter-added child layouts) is a single kustomize build, with those children referenced as directories. A child that renders bundles is not referenced: its own CR applies it. Coarser: Flux reconciles per bundle, kustomize handles the interior.

External augmenters may add child layouts that are not represented in the bundle model; integrated placement discovers those layouts and emits the required Flux resources.

## Service account

`Bundle.ServiceAccountName` names the service account kustomize-controller impersonates when it
applies the bundle (go-kure/kure#1034). Unset, no Kustomization carries `spec.serviceAccountName`
and the output is what it was before the field existed; the controller then applies with its own
identity, or with the default service account it is started with (`--default-service-account`),
if any.

| Kustomization | `spec.serviceAccountName` |
|---|---|
| a bundle's, in every placement | the bundle's own `ServiceAccountName` |
| an umbrella child's | the child's own; the umbrella's is not passed down |
| one that bundles a `GroupFlat` merge share | the one value every merged bundle sets; two values are refused, naming the field and both bundles |
| a per-layout one, under `FluxIntegratedPerLayout` | that of the bundle holding the layout's application; no layout field (see [Per-layout settings](/api-reference/flux-child-layout-crs/#per-layout-settings)) |
| a node's, under `FluxIntegratedPerLayout` | none: a node Kustomization inherits no settings |

Flux looks the account up in the Kustomization's own namespace, the generator's
`DefaultNamespace`, for every Kustomization kure generates. Creating the account and its RBAC is
the caller's; kure writes the name only. Under the integrated placements a bundle's directory
holds the CRs of its umbrella children and, under `FluxIntegratedPerLayout`, of its application
layouts, so a bundle's account applies those Kustomization objects and needs the RBAC for them.
Each of those Kustomizations then applies its own build under its own account: an umbrella child
without one is applied as an unset one is, whatever its umbrella sets, so give each
bundle that needs an account its own.

The name must be a DNS-1123 subdomain, the rule Kubernetes sets for a ServiceAccount's name
(`stack.ValidateServiceAccountName`). `Bundle.Validate` refuses another, and so does the
generator where it writes the bundle's Kustomization, for a bundle generated without
`Validate`; the error names the bundle by its path and the field `serviceAccountName`. Flux does
not check the field itself, so such a name would be admitted and fail at reconcile.

Flux-operator's multi-tenant lockdown (a default service account for every Kustomization) is a
cluster setting outside kure.

## Validation

All cluster-level entry points (`GenerateFromCluster`, `CreateLayoutWithResources`)
call `stack.ValidateCluster` before walking the tree, and every generation from a
layout runs `layout.IndexOrigins` (see [Kustomization paths](/api-reference/flux-names-and-paths/#kustomization-paths)). Invalid umbrella
configurations — such as a bundle referenced both by a `Node` and by another
bundle's `Children`, shared umbrella ownership, or multi-package umbrellas —
fail fast with a validation error rather than producing malformed output.

The layout rules are validated by the walk (`layout.LayoutRules.Validate`, see the layout
package's "LayoutRules Configuration"): `CreateLayoutWithResources` and `GenerateFromCluster` do
not check them on their own, and an unknown option value or a `ClusterName` with a `..` path
segment fails there with an error naming the field, also when the cluster is nil or has no root
node (`GenerateFromCluster` walks that cluster too, after its placement refusal, and returns
nothing only when the rules are valid). `IntegrateWithLayout` is handed a tree it did not walk and reads the
placement and the file naming from the rules, so it runs the same check first, before it looks at
the tree; it has no placement check of its own. An unknown placement is reported like any other
unknown rule value, naming the field `FluxPlacement`.

`CreateLayoutWithResources` additionally calls `validateSourceRefsForFluxIntegrated`
for **both inline placements** (`FluxIntegratedPerLayout` and
`FluxIntegratedPerBundle`) — both emit bundle/node CRs that carry a `spec.sourceRef`.
(After normalization `FluxUnset` becomes `FluxSeparate`, which skips this gate.)
This checks that every bundle
reachable from the cluster node tree — node bundles and umbrella child bundles
recursively — has a complete `SourceRef` with both `Kind` and `Name` set. A nil,
zero-value, or partially-populated `SourceRef` is rejected before layout walking
begins, and the error names the placement in use (`FluxIntegratedPerLayout mode
requires a SourceRef …` or `FluxIntegratedPerBundle mode requires a SourceRef …`).
The integrator also enforces this at CR-creation time as defense in
depth. `FluxSeparate` and non-Flux paths are unaffected.

### Durations

Every duration this package writes into an object it generates is held to the pattern the Flux
API holds that field to, `^([0-9]+(\.[0-9]+)?(ms|s|m|h))+$`, where the object is created. The
pattern is the same on a Kustomization's `spec.interval`, `spec.timeout` and `spec.retryInterval`
(kustomize-controller v1.10.1), on the `spec.interval` of a `GitRepository` and an `OCIRepository`
(source-controller v1.10.1) and on a `FluxInstance`'s `spec.sync.interval` (flux-operator v0.62.1):

| Generated object | Durations checked |
|---|---|
| a bundle's Kustomization | `Bundle.Interval`, `Timeout` and `RetryInterval`; `ResourceGenerator.DefaultInterval` when the bundle sets no interval |
| a per-layout Kustomization | the `Interval`, `Timeout` and `RetryInterval` in effect, the layout's own or the bundle's it inherits; `ResourceGenerator.DefaultInterval` when neither sets an interval |
| the bootstrap Kustomization (`gotk` mode) | `BootstrapGenerator.DefaultInterval` |
| a `GitRepository` or `OCIRepository`, derived from a `SourceRef` with a `URL` or generated by the bootstrap | the generator's `DefaultInterval` |
| the `FluxInstance` sync (`flux-operator` mode, with a `SourceURL`) | `BootstrapGenerator.DefaultInterval` |

A `stack.SourceRef` has no duration of its own, so a bundle's durations reach its Kustomization
and no source. A `DefaultInterval` is checked only where an object takes it: a generator whose
interval nothing generated carries is not refused for it.

The check is made on the form the value is written in, not the one it was authored in. A duration
is written as Go prints it, so `"90s"` is written `1m30s` and `"1us"` is written `1µs`. Go's
`time.ParseDuration` takes more than Flux does, a sign and the units `ns` and `us`, so two kinds
of value parse and are refused here, because the API server would refuse the object: a
negative duration (`"-1s"`) and a positive one under a millisecond (`"1us"`, `"500ns"`). Zero is written `0s`
and is taken. A day is no unit of a Go duration, so `"1d"` is refused as a value that does not
parse, as it was before.

The error names the bundle by its path (`Bundle 'shop/infra'` for an umbrella child) or the layout
by its directory, then the field, the value as authored and the form it is written in, and what
Flux takes; for a value a layout inherits it names the bundle as well, and for a generator's
default the generator and `DefaultInterval`:

```text
retryInterval "-2m" is written as "-2m0s", which the Flux API does not take: it takes digits with a unit of ms, s, m or h, so no negative duration and no positive one under a millisecond
```

A layout's inherited timeout or retry interval is first written into the Kustomization of the
bundle it comes from, which is created before those of the layouts below it, so a tree is refused
there, naming the bundle.

`Bundle.Validate` and `stack.ValidateCluster` are unchanged and accept these values: the rule is
Flux's, and another workflow generates from a bundle that carries one as before.

## Related Packages

- [stack](/api-reference/stack/) - Core domain model
- [stack/layout](/api-reference/layout/) - Manifest directory organization
- [kubernetes/fluxcd](/api-reference/fluxcd-builders/) - Low-level Flux resource builders
