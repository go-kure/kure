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
// to the layout call, not on the engine — see Layout Integration below.
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
[Kustomization paths](#kustomization-paths)).

Placement (FluxIntegratedPerLayout vs FluxSeparate) is configured per call on
`layout.LayoutRules.FluxPlacement`. `FluxUnset` is normalized to
`FluxSeparate` by `LayoutIntegrator.CreateLayoutWithResources` — matching
`layout.DefaultLayoutRules()` and the walker. The call's placement is the
tree's: `IntegrateWithLayout` sets it on every layout it integrates,
whatever placement the tree was walked with. A tree that already holds Flux
Kustomizations an earlier integration or the caller placed keeps the placement
they were made for (another is refused; Kustomizations an application emits
do not count), and a refused call leaves the tree exactly as it was. See
[Layout Integration](#layout-integration).

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
[One Kustomization per directory](#one-kustomization-per-directory)) with:
- `metadata.name` from the bundle (see [Kustomization names](#kustomization-names))
- `spec.path` = that directory
- Source reference from `Bundle.SourceRef` (and a Source when it has a `URL`)
- Dependency ordering from `Bundle.DependsOn` and `Bundle.NamedDependsOn`
- Interval and pruning configuration

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
application sets a [delivery intent](#delivery-intent) is refused as well, after the walk and so
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

## Kustomization names

A bundle's Kustomization is named after the bundle, unless the bundle sets `KustomizationName`:

<!-- doc-example: pkg/stack/fluxcd ExampleResourceGenerator_GenerateForBundle -->
```go
// The bundles keep their names and directories; their Kustomizations
// get the names set here.
db := &stack.Bundle{Name: "db", KustomizationName: "apps-db"}
shop := &stack.Bundle{Name: "shop", KustomizationName: "apps-shop", DependsOn: []*stack.Bundle{db}}

objects, err := fluxcd.NewResourceGenerator().GenerateForBundle(shop, "clusters/prod/shop")
if err != nil {
    panic(err)
}
kust := objects[0].(*kustv1.Kustomization)
fmt.Println(kust.Name, kust.Spec.Path, kust.Spec.DependsOn[0].Name)
```
<!-- doc-example:end -->

`Bundle.Name` stays the bundle's identity, and the field does not move the bundle's directory
(named by `Bundle.DirName`, or by `Name` without one; see [Directory names](#directory-names)),
so `spec.path` is the same with and without the field. `Bundle.UnitName()` returns the name in effect, and everything that refers to
the bundle's Kustomization uses it:

| Reference | Written as |
|---|---|
| `metadata.name` of the bundle's Kustomization | the bundle's `KustomizationName`, or its `Name` |
| `spec.dependsOn` of a bundle that lists it in `DependsOn` | the same name |
| an umbrella's health check on it as a child | the same name |
| a `NamedDependsOn` entry | it names a Kustomization, not a bundle: a bundle that sets `KustomizationName` is reached by that value. Its `Name` then reaches whichever bundle's Kustomization has that name, and is kept as written when none has, like any name no generated Kustomization has |

Two bundles whose Kustomizations would get one name are refused, and the error names both bundles
by their paths. `Bundle.Name` stays unique too. A bundle may hold an object that is itself a Flux
Kustomization named like the bundle (a component that delivers its own content): give the bundle
another `KustomizationName` and the generated Kustomization no longer collides with it.

Without it the tree is refused under every placement: the generated Kustomization and the one in
the tree would be two objects of one name in one namespace. The error names the one in the tree by
its layout and `spec.path`, and by the application and bundle that hold it; it names what the
generated one is for (the bundle, or under `FluxIntegratedPerLayout` the node or layout that gets
a Kustomization of its own); and it names the way out: the field that names the generated one
(`Bundle.KustomizationName`, `Node.KustomizationName` or `ManifestLayout.KustomizationName`), or
another name for the one in the tree. For a node whose walked layout carries a name the node does
not set, that is the layout and its field: the layout's name is the one in effect. A Kustomization
that no application holds, such as one added to a walked layout by hand or put where an
application's was, is named by its layout alone.

`Bundle.Validate` compares the names in effect wherever it compares against a Kustomization
reference. A bundle that lists the bundle `db` in `DependsOn` and the name `db` in
`NamedDependsOn` is valid when `db` sets `KustomizationName: "db-cr"`: the two are different
Kustomizations. The entry `db-cr` is then the duplicate.

A `DependsOn` entry that is a copy of a bundle of the cluster (another `Bundle` value with its
`Name`) is that bundle: `spec.dependsOn` gets the bundle's Kustomization name whether the copy
carries that name as its `KustomizationName` or none. Every entry point that takes a cluster or a
walked layout refuses a copy that sets another `KustomizationName`, naming both, and a copy of a
bundle whose Kustomization name is also in `NamedDependsOn`. `GenerateForBundle` has no cluster to
resolve a copy against and writes the name the entry carries. One valid input is refused: a name-only
copy of a bundle that sets `KustomizationName`, beside a `NamedDependsOn` entry equal to the
bundle's `Name`; set the bundle's `KustomizationName` on the copy (see the
[stack](/api-reference/stack/)).

## Kustomization paths

The layout tree is the only authority on directories. Every walked layout records which stack
nodes, bundles and application it renders (`ManifestLayout.OriginNodes`, `OriginBundles`,
`OriginApplication`), and `layout.IndexOrigins(root, cluster)` resolves each bundle to the one
layout whose directory holds its resources. A bundle's Kustomization `spec.path` is that layout's
`FullRepoPath()` — `OriginIndex.KustomizationPath(b)` — and nothing else:

| Tree | Rules | Bundle | `spec.path` |
|---|---|---|---|
| node `platform`, bundle `web` | `GroupByName` | `web` | `platform/web` |
| node `platform`, bundle `platform` | `GroupByName` | `platform` | `platform/platform` |
| root node `platform` (bundle `platform`) with child node `apps` (bundle `apps-bundle`) | bundles and applications `GroupFlat`, `ClusterName: "prod"` | `platform`, `apps-bundle` | `prod/platform/platform`, `prod/platform/apps` |
| unnamed root node, bundle `web` | bundles and applications `GroupFlat`, `ClusterName: "."` | `web` | `web` |

The path is emitted as `FullRepoPath()` returns it (no `./` prefix; Flux treats `x` and `./x`
alike) and is relative to the root of what the writer wrote: the `WriteToDisk` / `WriteToTar`
base, or `<basePath>/<ManifestsDir>` for `layout.WriteManifest`. Root the Flux source there.

The root node's directory renders no bundle (go-kure/kure#979): with a flat `BundleGrouping` its
bundle has a directory inside it, named after the bundle (by its `DirName` when it sets one, see
[Directory names](#directory-names)), as the last two rows show. Before, the
bundle rendered in the root node's directory (`prod/platform` and `.` in those rows), and the
Kustomization that applied that directory was hosted inside it. See "The root node's bundles" in
the layout package README, also for the path move this is on a deployed tree.

### Directory names

A bundle's directory is named after the bundle. `Bundle.DirName` names it otherwise, and
leaves the Kustomization's name and every reference to it alone; `spec.path` follows the
directory, in every placement:

| Umbrella `shop` at `platform/apps`, child | Directory | Kustomization | `spec.path` |
|---|---|---|---|
| `Name: "shop-infra"` | `platform/apps/shop-infra` | `shop-infra` | `platform/apps/shop-infra` |
| `Name: "shop-infra"`, `DirName: "00-infra"` | `platform/apps/00-infra` | `shop-infra` | `platform/apps/00-infra` |
| `Name: "shop-infra"`, `DirName: "00-infra"`, `KustomizationName: "apps-shop-infra"` | `platform/apps/00-infra` | `apps-shop-infra` | `platform/apps/00-infra` |

The field applies wherever a bundle's name becomes a directory: for an umbrella child, for a
node's bundle under `BundleGrouping: GroupByName`, and for the root node's bundle under
`GroupFlat`, which has a directory inside the root node's (root node `platform` with bundle
`platform-bundle` and `DirName: "00-platform"` gives the Kustomization `platform-bundle` with
`spec.path: platform/00-platform`). Under `GroupFlat` below the root a node's bundle is rendered
in its node's directory, which the node names, and its `DirName` has no effect. Bundles a flat
`NodeGrouping` merges into the root node share one directory and one Kustomization (see below):
the directory takes the first merged bundle's `DirName`, or its `Name`, and a `DirName` on a
later one has no effect.

A `DirName` must be one path segment (`stack.ValidateDirectoryName`); every entry point that
takes a cluster refuses any other value before it walks. It is no object name and may hold upper
case.

Two bundles whose own directories would be one, two umbrella children with one `DirName` for
example (or with two that differ in case only, which the writers treat as one directory), are
refused before any Kustomization is generated, and the error names both bundles by their paths. The checks on the root node's bundle directory compare the directory it gets, so
they follow a `DirName`: a child node of the root with that name is refused, and so is a
`DirName` of `flux-system` under `FluxSeparate`. The directory is set on the bundle, not on the
layout: a bundle's own layout renamed between `WalkCluster` and `IntegrateWithLayout` is refused,
and the error points at `DirName`. Such a rename used to be accepted, and gave the directory
another name without renaming the bundle or its Kustomization; a caller that relied on it sets
`DirName` instead.

### One Kustomization per directory

A directory is what a Flux Kustomization applies, so kure emits exactly one per directory that
renders bundles. When a `GroupFlat` axis merges several bundles into one directory, they share that
Kustomization, named after the first of them (the absorbing node's own bundle when it has one), by
its `KustomizationName` when it sets one. A reference to any of the merged bundles' Kustomization
names resolves to the shared one.

Each such directory has **one owner**: it is applied by its own Kustomization and by nothing else.
No parent `kustomization.yaml` lists a child directory that renders bundles, in any placement, so
two Kustomizations never apply (and prune, and patch) the same objects. The child's CR is what
applies it — in `flux-system` under `FluxSeparate`, in the parent directory under the integrated
placements. Building a parent directory therefore does not include its child units.

A Kustomization is never hosted inside the directory it applies. Every directory that renders
bundles has a parent to host its CR: the top of the tree `layout.WalkCluster` returns renders none
(go-kure/kure#979). `IntegrateWithLayout` refuses a tree whose top does render a bundle, in every
placement: integrate the whole tree `layout.WalkCluster` returns. A subtree of a walked tree,
integrated on its own, can be such a tree, and so can the `layout.WalkClusterByPackage` tree of a
package the root node is not in, under a flat `NodeGrouping`.

The bundles merged into one directory combine as follows:

| Bundle setting | In the shared Kustomization |
|---|---|
| `SourceRef`, `Interval`, `Timeout`, `RetryInterval`, `Prune`, `Wait`, `Force`, `Suspend`, `PostBuild` | must be the same for every merged bundle (unset compares as the default; an omitted `SourceRef` namespace is the generator's `DefaultNamespace`; without a `URL` only its kind, name and namespace are compared, the fields a Kustomization carries), else an error naming the setting and the bundles |
| `HealthChecks`, umbrella health checks | combined, each listed once; a check on a Flux Kustomization names the unit that applies that bundle, and one on the unit itself is dropped |
| `Labels`, `Annotations` | combined; one key with two values is an error |
| `Patches` | combined, but only when every patch has a `Target` that selects none of the other merged bundles' objects (see [Patches in a shared directory](#patches-in-a-shared-directory)) |
| `DependsOn` | mapped to the Kustomization that applies each dependency; dependencies between the merged bundles are dropped |
| `NamedDependsOn` | combined and mapped like `DependsOn`: a name that is a rendered bundle's Kustomization name (see [Kustomization names](#kustomization-names)) becomes the name of the Kustomization that applies it (dropped when that is this one); any other name is kept as given |

`GenerateFromLayout` and the integrator refuse a set of Kustomizations kure generates that can
never all become Ready. Applying waits for every `dependsOn` to be Ready; becoming Ready waits for
every Kustomization it health-checks, unless `wait` is set (Flux then ignores health checks and
waits for everything it applied, the CRs it created included); and, under integrated placement, a
CR exists only once the Kustomization whose directory references reach its file has applied — a CR
the root directory reaches is created by the Flux bootstrap. Identities are namespace and name. A
cycle among these waits is refused, naming the chain. Kustomizations an application emits itself,
and objects in other namespaces, are outside this check. A merge can close one (a health
check or dependency on a bundle merged into a unit that waits for it), and so can a parent node's
bundle depending on a child node's bundle whose CR only the parent's directory holds (both
integrated placements). The root node's bundle cannot close that one any more: the CRs of the
root's child nodes are hosted in the root node's directory, which the bundle's Kustomization does
not build, and are created by whatever applies that directory (go-kure/kure#979). The bundle's own
directory still hosts the CRs of its umbrella children and, under `FluxIntegratedPerLayout`, of
the layouts inside it, as every bundle's directory does. A per-layout Kustomization with `wait`
(its layout's, or the one it inherits: see [Per-layout settings](#per-layout-settings)) waits for
the CRs its directory hosts like any other, so a layout that depends on the application layout
above it closes a cycle once that one has `wait`. ArgoCD
Applications get the unit rule but not this check: only a `DependsOn` cycle between units is
refused there. Give bundles directories of their own (`NodeGrouping` or
`BundleGrouping` `GroupByName`) when they need different settings.

#### Patches in a shared directory

Flux applies a Kustomization's patches to **everything that Kustomization builds**, not to the
bundle that declared them. While a bundle has a directory of its own that is the same thing. Once
a `GroupFlat` axis puts several bundles in one directory, the shared
Kustomization builds all their objects, so a patch meant for one bundle would also change the
others' — and a layout setting would silently change what a bundle's patch does. kure refuses
that instead of generating it:

- an untargeted patch in a shared directory is refused;
- a targeted patch is refused when its `Target` selects any object another bundle in that
  directory renders, a ConfigMap an augmenter's `configMapGenerator` makes included, in the
  application's own layout or a child layout below it (matched with the annotations its entry
  sets, those of a [delivery intent](#delivery-intent) among them). Matching is
  kustomize's own: `Group`, `Version`, `Kind`, `Name` and `Namespace` are anchored regular
  expressions (an empty one matches anything; `Namespace` is matched against the object's
  effective namespace, `default` for a namespaced object that names none), `LabelSelector` and
  `AnnotationSelector` are Kubernetes selector expressions. Only objects the shared Kustomization
  builds count: under `FluxIntegratedPerLayout` a per-app directory is applied by its own CR, so
  its objects are outside the patch's reach. A bundle `b1` with `Target: {Kind: ConfigMap}` merged with a
  bundle that renders a ConfigMap is refused; `Target: {Kind: ConfigMap, Name: one-cm}`, naming
  `b1`'s own ConfigMap, is accepted.

The error names both bundles, the target and the object it reaches. Narrow the target to the
bundle's own objects, or give the bundles directories of their own (`NodeGrouping` or
`BundleGrouping` `GroupByName`). Not covered: objects inside an `ExtraFiles` file (they are not
kustomize resources) and objects without a kind. Under `FluxIntegratedPerLayout` the check counts
no generated ConfigMap, and needs none: no walked tree has a shared directory that builds one. An
augmenter application keeps a directory of its own, which its own Kustomization applies without
patches, and `FlattenSingleTier` collapses no directory that renders a bundle (go-kure/kure#979).

The generator computes no path, the integrator matches nothing by name, and `FlattenSingleTier`
rewrites nothing afterwards: it collapses no directory that renders a bundle
(go-kure/kure#979), so it changes no Kustomization's path and merges no bundles. An umbrella
child's path is its own directory, in every mode (an earlier `KustomizationRecursive` rule
pointed it at the parent bundle's directory, whose kustomization excludes the child).

`IndexOrigins` refuses a tree it cannot resolve unambiguously: a hand-built, partial or
other-cluster tree (the rendered set is not what the cluster reaches), a bundle or node rendered
twice, two bundles with one name or with one Kustomization name, a bundle's own directory
under another name than the bundle's `DirName` (or its `Name` without one), two bundles with
one directory of their own, and a node or bundle
layout in `AppFileSingle` mode (written into its `Namespace`, not its own directory). Build the
tree with `layout.WalkCluster`. Not covered: a bundle whose `SourceRef.URL` names another
artifact — its path is relative to that artifact.

## Defaults are declared, named and exported

This package is a workflow layer above `pkg/kubernetes`, so unlike the builders it may hold
opinions — but only as declared inputs with names a consumer can read, compare against and
override. Every fallback this package applies is one of the exported identifiers in `defaults.go`,
or comes from `layout.DefaultLayoutRules()` where the value belongs to `pkg/stack/layout`; grep
for the identifier to find every place its value can reach emitted YAML.

| Identifier | Value | Applies when | Override by |
|---|---|---|---|
| `DefaultInterval` | `60m` | the caller names no interval (a non-empty `Bundle.Interval` that does not parse is a validation error, not a fallback; see [Durations](#durations) for the values Flux does not take) | assigning `.DefaultInterval` on either generator |
| `DefaultNamespace` | `flux-system` | generated resources need a namespace | assigning `.DefaultNamespace` on either generator |
| `DefaultMode` | `layout.KustomizationExplicit` | informational: the listing mode the layout writers use for a layout with no `Mode` of its own | setting `ManifestLayout.Mode` (or `Config.KustomizationMode` for `WriteManifest`) |
| `DefaultBootstrapName` | `flux-system` | naming the bootstrap Kustomization | assigning `BootstrapGenerator.BootstrapName` |
| `FluxInstanceName` | `flux` | naming the `FluxInstance` in `flux-operator` mode | not overrideable; the CRD admits no other name (see below) |
| `DefaultSourceName` | `flux-system` | the root node has no name | naming the root `stack.Node` |
| `DefaultFluxMode` | `flux-operator` | `BootstrapConfig.FluxMode` is empty | setting `BootstrapConfig.FluxMode` |
| `DefaultSourceKind` | `OCIRepository` | `BootstrapConfig.SourceKind` does not name `GitRepository`, the empty string included | setting `BootstrapConfig.SourceKind` |
| `DefaultFluxDirName` | `flux-system` | a separate Flux layout needs a directory | not overrideable |
| `DefaultSourceRef` | `latest` | an OCI source, or an OCI `FluxInstance` sync, has no `SourceRef` | setting `BootstrapConfig.SourceRef` |
| `DefaultSyncPath` | `./` | the bootstrap applies the root of the source (no root node, no `ClusterName`) | not overrideable; it is the prefix a sync path is built from |

Three of these — `DefaultInterval`, `DefaultNamespace` and `DefaultBootstrapName` —
are copied into exported generator fields by `NewResourceGenerator` / `NewBootstrapGenerator`, and
a field assigned afterwards is never overridden. The rest are applied where they are used and are
overridden by naming the corresponding input, as the last column says. Two defaults have no
override at all and say so, rather than being listed as though they had one; `FluxInstanceName`
is the third row without one, and is not a default at all (next but one paragraph).

An empty `BootstrapGenerator.BootstrapName` resolves back to `DefaultBootstrapName` at emission.
A generator built as a struct literal rather than through `NewBootstrapGenerator` leaves the field
zero, and a Kustomization with no `metadata.name` is invalid — the field is an override, not a way
to remove the name.

`FluxInstanceName` is in the table but is not a default: it is the one value the flux-operator CRD
accepts. The CRD requires `metadata.name: flux` and rejects any other name at admission
(`x-kubernetes-validations`: `self.metadata.name == 'flux'`, in the vendored install bundle), so
`BootstrapName` does not reach the `FluxInstance` and nothing else does either. It used to — both
objects took `bootstrapName()` — so a `flux-operator` bundle with the default `BootstrapName`, or
any override other than `flux`, was refused with `the only accepted name for a FluxInstance is
'flux'`. A test compares the constant against the vendored
CRD's rule, so an operator bump that changes the rule fails in this package rather than at apply.

`DefaultNamespace` governs the **whole** gotk bundle, not just the objects this package constructs
itself. The bootstrap bundle has three producers — the root Kustomization, the root source, and the
toolkit components generated by `install.Generate` — and the third takes its namespace from an
upstream option whose own default is also `flux-system`. That option is set from
`BootstrapGenerator.DefaultNamespace`, so overriding the field moves the controllers' Deployments,
ServiceAccounts, Services, NetworkPolicies and ResourceQuota along with everything else. The
cluster-scoped objects follow without further work — `install.Generate` derives the emitted
`Namespace`'s name and the `ClusterRoleBinding` names and subjects from the same option, while CRDs
and ClusterRoles carry no namespace to correct. Before this was wired up the field half-applied: a
non-default value left the components in `flux-system` while the objects depending on them moved.
That split still reconciles, because the components default to `WatchAllNamespaces: true` — so it
surfaces only when someone narrows the controllers to their own namespace, at which point
reconciliation stops with no error. `WatchAllNamespaces` is deliberately left at the upstream
default and is not currently derived from any field here.

In `flux-operator` mode — the default — the field reaches only the `FluxInstance`. That bundle's
other objects come from the vendored upstream install manifest, appended unmodified, so its
`Namespace`, ServiceAccount, Service and Deployment stay at the upstream `flux-system` whatever this
field says. Relocating the operator itself is not something this package offers.

The wiring is guarded on the field being non-empty, which matters only for a struct-literal
generator. `install.Generate` uses the option as the emitted `Namespace`'s **name**, so assigning it
unconditionally would make a generator with a zero `DefaultNamespace` fail the whole bundle —
`missing metadata.name in object {{v1 Namespace}}` — rather than fall back. Guarded, that generator
keeps the upstream `flux-system` for the components.

That generator is nonetheless inconsistent, and the guard does not fix it: the root Kustomization
and the root source read the same empty value and emit no namespace at all, so the components sit in
`flux-system` while the objects referencing them are unnamespaced. This predates the namespace wiring
and is a property of struct-literal construction across this package rather than of one option.
**Construct through `NewBootstrapGenerator`**, which populates the field; a struct literal is not a
supported way to get a coherent bundle.

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

The third is that layout's own `Mode`, left unset. The layout writer resolves an unset mode to
`KustomizationExplicit`, and the separate Flux layout never has children, so the mode selects
nothing there.

### One resolved source kind, not three

`resolvedSourceKind` is the only place the source kind is decided. Three sites need it — the
source object itself, the bootstrap Kustomization's `spec.sourceRef.Kind`, and the `FluxInstance`
sync block — and they used to decide it separately, with the `sourceRef` testing for
`OCIRepository` while the other two tested for `GitRepository`. The three agreed only when
`SourceKind` named a kind exactly; an empty or unrecognised `SourceKind` emitted an
`OCIRepository` under a `sourceRef` naming a `GitRepository` that was never created.

### One resolved source ref, on both bootstrap paths

For an OCI tag, an empty ref or a bare Git branch name, `BootstrapConfig.SourceRef` selects the
same revision in both modes. The gotk source reads it as an OCI tag, falling back to
`DefaultSourceRef`, or as a Git branch. flux-operator renders the `FluxInstance`'s
`spec.sync.ref` as the source's `ref.tag` for OCI and as `ref.name` for Git, and `ref.name` takes
a full Git reference. `resolvedSyncRef` bridges the two: an empty OCI ref becomes
`DefaultSourceRef`, a Git branch name becomes `refs/heads/<name>`, and an empty Git ref stays
empty. A Git ref that already starts with `refs/` — a tag, say — passes through unchanged in
flux-operator mode only: the gotk source still puts any Git `SourceRef` into `ref.branch`, so
there a full reference is not equivalent. Before this, `spec.sync.ref` was `SourceRef` verbatim:
an empty OCI tag where the gotk source used `latest`, and a bare branch name where Flux needs a
full reference.

### `prune` and `wait` are inputs, not policy

`Bundle.Prune` and `Bundle.Wait` are `*bool` and are passed through untouched.
`BootstrapConfig.Prune` does the same for the bootstrap Kustomization, and
`ResourceGenerator.Prune` for a Kustomization generated from a `layout.ManifestLayout` where
neither the layout nor the bundle that holds its application sets one. A layout carries both
settings, `ManifestLayout.Prune` and `ManifestLayout.Wait`, unset meaning that bundle's (see
[Per-layout settings](#per-layout-settings)).

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

<!-- doc-example:excerpt the generator's own source: g is the ResourceGenerator, ref the bundle's SourceRef -->
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

<!-- doc-example: pkg/stack/fluxcd ExampleWorkflowEngine_CreateLayoutWithResources -->
```go
cluster := exampleCluster()
engine := fluxcd.Engine()
rules := layout.DefaultLayoutRules()
out, err := os.MkdirTemp("", "kure-fluxcd-example")
if err != nil {
    panic(err)
}
defer func() { _ = os.RemoveAll(out) }()

// Create layout with Flux resources integrated
ml, err := engine.CreateLayoutWithResources(cluster, rules)
if err != nil {
    panic(err)
}

// Write to disk: every spec.path is relative to <out>/clusters
err = layout.WriteManifest(out, layout.DefaultLayoutConfig(), ml.(*layout.ManifestLayout))
if err != nil {
    panic(err)
}
printFiles(out)
```
<!-- doc-example:end -->

`IntegrateWithLayout(ml, cluster, rules)` does the same for a layout you walked yourself. It
refuses a tree `layout.WalkCluster` did not build from that cluster (see
[Kustomization paths](#kustomization-paths)). Integrating the same layout twice adds nothing: a
CR already present with the same name and `spec.path`, in the layout that would host it, is kept;
the same name in another layout or with another path is an error, and under `FluxSeparate` an identical `flux-system` child — same directory, same
resources, nothing beneath it — is kept rather than a second one appended; any other is refused. Every Flux Kustomization already in the tree counts — typed or unstructured, placed
by an earlier integration, by the caller or emitted by an application, top-level or inside a
`List` (kustomize builds a List's items; a List is an object whose kind ends in `List` and that
has an `items` field, and a List among the items is opened too, while a kind that does not end in
`List` is one object whatever fields it has; an item a typed List holds as raw JSON is read as
the object it encodes, also when it carries an object beside the raw JSON, and one without object
metadata as the object it serializes to): an
identity (namespace/name) present twice, or taken by
a generated CR elsewhere, is refused in every placement, since the kustomize build would register
the id twice. Where a generated CR meets one in the tree, the error names both and the way out
(see [Kustomization names](#kustomization-names)). A generated Source has one definition across the whole pass: every Source of its
identity — kind, namespace and name, whatever the API version (a `v1beta2` `GitRepository` is the
same Source as the generated `v1` one) — anywhere in the tree, top-level or inside a `List`, and
every Source the pass places in another layout, must have the same API version and content, or the
integration is refused. Content is compared as the objects' unstructured form, so a typed Source
and an unstructured copy of it are the same.

Under the integrated placements every Source the integration generates is hosted **once, in the
root node's layout**, whichever layouts hold the Kustomizations that use it (go-kure/kure#876),
and not in a `ClusterName` wrapper above it. For a named root node under the default rules that is
the directory the bootstrap applies (see
[The directory the bootstrap applies](#the-directory-the-bootstrap-applies), and
[Kustomization paths](#kustomization-paths) for other rules), so the Source is in one build, the
root's, and exists before any Kustomization that uses it. Below a `ClusterName` wrapper the
bootstrap applies the wrapper, and the root node's directory with it or through that directory's
own Kustomization (below).

**No Kustomization takes its source from inside what it applies** (go-kure/kure#979). A generated
Source is in a build that is applied before every Kustomization that names it, never in a
directory that Kustomization delivers: the bootstrap applies the top of the written tree (the
`ClusterName` directory, see
[The directory the bootstrap applies](#the-directory-the-bootstrap-applies)), and where nothing
there delivers that Source, the Kustomization waits for a Source only its own apply creates and
never reconciles. On a walked tree every bundle's Kustomization meets the rule
by construction, because the root node's directory renders no bundle. The one Kustomization that
applies the root node's directory is that directory's layout Kustomization below a `ClusterName`
wrapper under `FluxIntegratedPerLayout`, hosted in the wrapper. It takes no
generated Source: the `SourceRef` of the root node's bundle is passed over when it has a URL or
names a Source another `SourceRef` generates, and its source is the one `SourceRef` the other
URL-less bundles below share, or, where those differ, the root node's own bundle's when it was
not passed over. When nothing is left (every `SourceRef` in the tree has a URL, say)
the integration is refused, naming the Kustomization, its `spec.path`, the Source and the
directory that hosts it. Give a bundle a `SourceRef` without a URL, naming a Source that exists
before the tree is applied (the one the bootstrap creates, for one), or use
`FluxIntegratedPerBundle`, under which those directories have no Kustomization of their own.
Without a `ClusterName` wrapper the root node's directory has no Kustomization and nothing
changes. A `SourceRef` with a URL on a kind the integration generates no Source for (anything but
`GitRepository` and `OCIRepository`) is not counted as a generated Source; where generation is
reached, it is the generator's own error. The rule is held for the Kustomizations
the integration generates or keeps and the directories they apply: a Kustomization of your own
placed inside one of them and pointing back up the tree is not followed (see below, the builds
kure answers for).

Among the bootstrap and the Kustomizations the integration generates or keeps, each hosted Source
has one applier. Without a `ClusterName` wrapper, and below a wrapper whose `kustomization.yaml`
lists the root node's directory, that is the bootstrap, and none of those Kustomizations builds
that directory. Below a wrapper under
`FluxIntegratedPerLayout` it is the layout Kustomization of the root node's directory: the
bootstrap applies the wrapper, which lists that Kustomization and not the directory. A
Kustomization of your own kept in its place (below) may therefore carry patches or a postBuild
that change a hosted Source; they are the only ones that Source gets (go-kure/kure#979: the
bootstrap used to be pointed at the root node's directory, beside that Kustomization, and such a
patch or postBuild was refused). A `sourceRef` names the object, not the layout holding it.

The root build also covers the child directories the root's `kustomization.yaml` lists and the
`AppFileSingle` files written into them. When that build already holds a copy the integration did
not add (an earlier integration's, the caller's or an application's), that copy is kept and the
integration adds none, since kustomize refuses one object twice. Two copies the integration did
not add, in any one build (the root's, a `ClusterName` wrapper's or a generated Kustomization's
`spec.path`), are refused. So is one such copy in a wrapper whose `kustomization.yaml` lists the
root node's directory: the wrapper's build would hold it and the root's copy, and the integration
removes neither; move the copy into the root node's build or remove it. A copy the caller or an application puts in any other
build is theirs to keep, and the integration still hosts its own at the root: that Source then has
two owners, one of them the caller's.
Kustomizations the caller or an application places are not builds kure answers for. Under
`FluxSeparate` every generated Source goes into
`flux-system`, which is built beside the rest of the tree, so a Source with a generated Source's
identity anywhere else in the tree is refused even when it is identical.

A Kustomization the integration keeps in place of its own (same name and `spec.path`, in the
layout that would host it) is checked as the generated one would be, with what the kept object
itself sets and in whatever form it has: typed, unstructured or inside a `List`
(go-kure/kure#979). Its `spec.path` is a build in which a generated Source may appear once; when
that build holds the root node's layout, its `sourceRef` may not name a Source hosted there (an
omitted `sourceRef` namespace is the Kustomization's own): the integration is refused, naming it,
since it would take its source from inside what it applies; and its `dependsOn`, `wait` and health
checks enter the reconcile-order check (see
[One Kustomization per directory](#one-kustomization-per-directory)). An unstructured one is read
through the typed Flux `Kustomization`; one that cannot be read that way (a field of the wrong
type, for one) is refused, with an error naming the layout that holds it and its namespace and
name.

### Delivery intent

An application's `Delivery` (see [stack](/api-reference/stack/)) is written as kustomize-controller
annotations on that application's objects, and on no other application's:

| `stack.DeliveryIntent` field | Annotation |
|---|---|
| `PruneProtection` | `kustomize.toolkit.fluxcd.io/prune: disabled` |
| `ForceReplace` | `kustomize.toolkit.fluxcd.io/force: enabled` |

`IntegrateWithLayout` (and so `CreateLayoutWithResources`) sets them, under all three placements
and every grouping, on:

- every object the application emits. For a `List` that is what it holds, a `List` inside it
  included, and never the envelope. As in kustomize, an object is a `List` when its kind ends in
  `List` and it has an `items` field in the written file; a kind ending in `List` without `items`
  (a typed object that leaves an empty one out included), and any other kind with an `items`
  field, is one object and carries the annotation itself. The written file decides: a typed
  object whose written form still holds an object without the annotation, because its Go value
  gives no access to it, is refused. An item held as raw JSON (`runtime.RawExtension.Raw`) is
  such an object: it is written as it is, so it has to carry the annotation already. An object
  the Go value holds that is not among the written items is not the application's and is left
  as it is. A `List`
  among a typed List's items is opened whether or not it carries object metadata of its own
  (`metav1.List`, the core `v1` List as a Go value, has none), and the objects it holds as Go
  values are annotated like any other (go-kure/kure#1006);
- every object a `LayoutAugmenter` adds to the application's own layout or a child layout below it;
- every `configMapGenerator` of those layouts, as the entry's `options.annotations`, because
  kustomize builds those ConfigMaps after kure has written the tree.

An object's other annotations are kept. An application that sets no intent gets no annotation and
its output does not change.

An application may emit the Flux Source that a bundle's `SourceRef` also derives; under the
integrated placements the two are one object, hosted once per build. With an intent on that
application, the derived Source carries the same annotations, so every copy that is applied is
the same.

An object or generator that already carries one of these annotations with **another** value is
refused, with an error naming the application, the object (kind, namespace/name) or generator, the
annotation and both values; the same value is left as it is, so integrating a layout again changes
nothing. When the integration is refused, for this or any later reason, the annotations it added
are taken back with the rest of its changes.

Things to know:

- The annotations are set on the objects the layout holds, not on copies. An `ApplicationConfig`
  that returns the same object pointers on every `Generate` call therefore keeps them after a
  successful integration, also when the intent is cleared afterwards; return fresh objects per call
  to avoid that.
- The integrator applies the intent. `GenerateFromCluster` returns Kustomizations and Sources and
  none of the application's objects, so it refuses a cluster in which an application sets an
  intent, with an error naming the application and pointing to `CreateLayoutWithResources`.
  `GenerateFromLayout` does not refuse an intent: its caller holds the layout, and the list it
  returns carries no intent by itself. The intent is applied by `IntegrateWithLayout` or
  `CreateLayoutWithResources` on the layout, and a layout that is walked and written without
  either carries none.
- A bundle patch runs after the build: `Bundle.Patches` become the Kustomization's `spec.patches`,
  which Flux applies to what the directory builds. A patch can therefore still change or remove a
  delivery annotation on the applied object; kure does not read patch bodies and does not override
  an explicit patch.
- kustomize names a generated ConfigMap after its content. With `PruneProtection`, each content
  change therefore leaves the previous ConfigMap in the cluster instead of pruning it; remove old
  ones by hand, or leave the intent off an application whose generated values change often.
- `Bundle.Prune` and `Bundle.Force` set `spec.prune` and `spec.force` on the whole generated
  Kustomization and are independent of the per-application intent.

## Bootstrap Generation

Generate Flux system bootstrap manifests. Two modes are supported:

| Mode | Description |
|------|-------------|
| `"flux-operator"` | **Default.** Emits a full Flux Operator install bundle (CRDs, Deployment, RBAC). Recommended for new clusters. |
| `"gotk"` | Legacy mode. Emits the GitOps Toolkit component manifests directly. |

When `FluxMode` is empty, it defaults to `"flux-operator"`.

In `"flux-operator"` mode `FluxVersion` and `Registry` are both required. They go verbatim into
the `FluxInstance`'s `spec.distribution`, and a `FluxInstance` with an empty version or registry
cannot work, so `GenerateBootstrap` and `GenerateFluxInstance` return a validation error naming
each missing field instead of emitting one, and no part of the bundle is returned with it. There
is no default for either: the caller supplies both, which is why the defaults table above has no
row for them. `"gotk"` mode reads the same two fields and keeps accepting them empty: there an
empty `FluxVersion` means the vendored release and an empty `Registry` the upstream registry.

The `"flux-operator"` bundle is vendored from the upstream flux-operator release and pinned
in lockstep with the `github.com/controlplaneio-fluxcd/flux-operator` Go module
(`FluxOperatorVersion`, currently **v0.58.1**). Renovate re-vendors the bundle and updates
the constant and this version when it bumps the module (`scripts/sync-flux-operator-pin.sh`);
see `flux_operator_install.go` for the manual refresh procedure.

The `"gotk"` components are built from the flux2 release's install manifests base, vendored
at the module root (`internal/gotk/manifests.tar.gz`, shared with the tests that read the
toolkit's CustomResourceDefinitions out of it) in lockstep with the
`github.com/fluxcd/flux2/v2` Go module
(`GotkVersion`, currently **v2.9.5**), so gotk generation makes no network call and the same
input always produces the same manifests. That happens when `FluxVersion` is empty or names
`GotkVersion` (with or without the leading `v`). Any other `FluxVersion`, `"latest"` included,
is an explicit opt-in to the upstream behaviour: a manifests base is downloaded from GitHub at
generation time — the named release for a `vX.Y.Z` value, the latest release for anything else
(upstream selects a release only for a `v`-prefixed version). `TestVendoredPinsMatchGoMod` fails when
`GotkVersion` or `FluxOperatorVersion` differs from its `go.mod` require, or when a controller
image in the gotk bundle differs from the matching `fluxcd/<controller>/api` require (a controller
whose API module kure does not require is not compared); see
`internal/gotk` for the refresh procedure after a flux2 bump.

<!-- doc-example: pkg/stack/fluxcd ExampleWorkflowEngine_GenerateBootstrap -->
```go
engine := fluxcd.Engine()
rootNode := &stack.Node{Name: "prod"}

bootstrapConfig := &stack.BootstrapConfig{
    Enabled:     true,
    FluxMode:    "flux-operator",  // or "gotk"; empty defaults to "flux-operator"
    FluxVersion: "v2.8.2",         // required in flux-operator mode
    Registry:    "ghcr.io/fluxcd", // required in flux-operator mode
    SourceURL:   "oci://registry.example.com/fleet",
    SourceRef:   "latest",
}

objects, err := engine.GenerateBootstrap(bootstrapConfig, rootNode, layout.LayoutRules{})
if err != nil {
    panic(err)
}
for _, obj := range objects {
    if obj.GetObjectKind().GroupVersionKind().Kind == "FluxInstance" {
        fmt.Println(obj.GetNamespace(), obj.GetName())
    }
}
```
<!-- doc-example:end -->

### The directory the bootstrap applies

`GenerateBootstrap` and `GenerateFluxInstance` take the layout rules the tree is written with, and
both modes point Flux at the top directory of that tree (go-kure/kure#979): the directory
`layout.TopDirectory` returns for the root node and the rules, which is where `WalkCluster` takes
its own top from, so the two cannot differ. Each mode spells it the way its field requires:

| Root node | Rules | `"gotk"`: bootstrap Kustomization `spec.path` | `"flux-operator"`: `FluxInstance` `spec.sync.path` |
|---|---|---|---|
| named `prod` | no `ClusterName` | `prod` | `./prod` |
| unnamed | no `ClusterName` | `cluster` | `./cluster` |
| named, unnamed or none passed | `ClusterName` `clusters/prod` | `clusters/prod` | `./clusters/prod` |
| none passed | no `ClusterName` | `.` | `./` |

`spec.path` is spelled like every other Kustomization path this package writes (see
[Kustomization paths](#kustomization-paths)): no leading `./`, and `.` for the root of the source.
`spec.sync.path` is `DefaultSyncPath` followed by the directory. Flux treats `x` and `./x` alike.
`"gotk"` mode used to write `manifests/<root>`, a prefix none of the layout writers produces; a
consumer that depends on such a prefix sets `spec.path` on the returned Kustomization.

With no root node there is no tree to walk: without a `ClusterName` the bootstrap applies the root
of the source, and the caller writes there. The source's name and the bootstrap Kustomization's
`sourceRef` stay on the root node's name whatever the rules are (see
[Root node name](#root-node-name)).

The rules are validated first, as a walk validates them, so invalid rules are an error whatever
the `BootstrapConfig` is, a nil or disabled one included. `WorkflowEngine.GenerateBootstrap`
takes them as `stack.LayoutRulesProvider` and refuses anything that is not a `layout.LayoutRules`
value, nil included, as `GenerateFromCluster` does; it does not fall back to the defaults.

The organisation layout document shows the bootstrap Kustomization's path as the `flux-system`
directory. kure's bootstrap applies the top directory of the written tree, whose
`kustomization.yaml` lists `flux-system` under the Separate placement, so the objects that
directory holds are applied either way.

Two limits. The directory is relative to the directory the tree is written into: a tree written
below a sub-path of the repository (the base directory a writer is given is the caller's) is at
that sub-path joined with it, and the caller sets `spec.path` or `spec.sync.path` accordingly.
And it describes a `WalkCluster` tree only: `WalkClusterByPackage` writes one tree per package,
placed without the `ClusterName`, and the bootstrap does not point at those.

**Breaking change (go-kure/kure#979).** `GenerateBootstrap` (on `BootstrapGenerator`,
`WorkflowEngine`, the ArgoCD engine and the `stack.Workflow` interface) and
`GenerateFluxInstance` take the layout rules as a third argument. The directory used to be the
root node's name whatever the rules were, so two outputs move:

- with a `ClusterName`, both paths move from the root node's name (`.` and `./` for an unnamed or
  absent one) to the cluster directory;
- an unnamed root node without a `ClusterName` moves from the root of the source (`.`, `./`) to
  `cluster` (`./cluster`), where the walk writes it.

A named root node without a `ClusterName` keeps its path. Under a `ClusterName` the bootstrap no
longer checks the root node's name as a directory name: its path is the cluster directory and
takes no segment from the name. The name still becomes a directory of the walked tree
(`<ClusterName>/<root>`), where the walk's cluster validation checks it. A cluster bootstrapped
with the old path keeps applying the old directory until the regenerated bootstrap objects are
applied.

One refusal is no longer met (go-kure/kure#979). Below a `ClusterName` wrapper under
`FluxIntegratedPerLayout`, a layout Kustomization of the root node's directory that you keep in
place of the generated one was refused when a patch or postBuild of it changed a Source hosted
there, because the bootstrap applied that directory too. The bootstrap now applies the wrapper,
which lists the Kustomization, so it is accepted (see
[Layout Integration](#layout-integration)).

### Sync name

`BootstrapConfig.SyncName` becomes the `FluxInstance`'s `spec.sync.name`: the name flux-operator
gives the source and Kustomization it creates for the sync. When it is empty the operator names
both after the `FluxInstance`'s namespace (`DefaultNamespace`). That fallback is the operator's,
not kure's, which is why the defaults table above has no row for it. Set `SyncName` when the
Kustomizations you generate reference the sync source by another name; otherwise their
`sourceRef` points at a source nothing creates.

<!-- doc-example: pkg/stack/fluxcd ExampleBootstrapGenerator_GenerateFluxInstance -->
```go
engine := fluxcd.Engine()
rootNode := &stack.Node{Name: "prod"}

bootstrapConfig := &stack.BootstrapConfig{
    Enabled:     true,
    FluxVersion: "v2.8.2",
    Registry:    "ghcr.io/fluxcd",
    SourceURL:   "oci://registry.example.com/fleet",
    SourceRef:   "latest",
    SyncName:    "fleet",
}

fi, err := engine.GetBootstrapGenerator().GenerateFluxInstance(bootstrapConfig, rootNode, layout.LayoutRules{})
if err != nil {
    panic(err)
}
fmt.Println(fi.Name, fi.Spec.Sync.Name, fi.Spec.Sync.Ref)
```
<!-- doc-example:end -->

- flux-operator mode only: gotk mode ignores it, as flux-operator mode ignores `Prune`.
- It needs `SourceURL`: without one no sync block is emitted, so there is nothing to name.
- kure passes it through unvalidated. The CRD caps it at 63 characters and makes it immutable
  once set, so renaming the sync of a live cluster means recreating the `FluxInstance`.
- It is not the `FluxInstance`'s own `metadata.name`, which is always `FluxInstanceName` (`flux`):
  the CRD accepts no other. `BootstrapGenerator.BootstrapName` names the bootstrap Kustomization
  only.

### Root node name

Without a `ClusterName` the root node's name is the gotk bootstrap Kustomization's `spec.path`
and the directory in the `FluxInstance`'s `spec.sync.path`. `GenerateBootstrap` and
`GenerateFluxInstance` take a node, not a cluster, so they check that name themselves with
`stack.ValidateDirectoryName`: a name holding `/`, `\` or a NUL byte, or `.` or `..`, is refused.
No root node and an unnamed root are valid; neither is a path segment. Nor is any root node under
rules with a `ClusterName`, where the path is the cluster directory: the bootstrap does not check
the name as a directory name there. The walk does, in every case: under a `ClusterName` a named
root node is the directory `<ClusterName>/<root>` of the written tree.

The check runs only where a path is built. In `gotk` mode the bootstrap Kustomization always has
a `spec.path`, so the name is always checked. A `FluxInstance` gets a `spec.sync` only when
`BootstrapConfig.SourceURL` is set: without one, flux-operator mode and `GenerateFluxInstance` do
not use the root name and do not check it.

In `gotk` mode a named root is also an object name: the generated `GitRepository` or
`OCIRepository` and the bootstrap Kustomization's `sourceRef` carry it. There the name must be a
DNS-1123 subdomain as well, with or without a `SourceURL`; `Prod` and `prod_root` are refused.

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
- `FluxIntegratedPerLayout` - a Flux Kustomization CR for every layout that renders bundles (bundles a `GroupFlat` merge puts in one directory share one CR, named after the first) and for every child layout that is not an umbrella child, not `AppFileSingle` and renders no bundle (augmenter-added child layouts included), hosted in its parent layout (the top of the tree `layout.WalkCluster` returns renders no bundle and gets no CR, go-kure/kure#979); the parent's `kustomization.yaml` lists those CR files as its own resources and references no child directory. Not literally every layout: a layout whose CR name another generated CR already uses, such as the application `web` of the bundle `platform` beside a bundle named `platform-web`, is refused instead (see [Non-Bundle Child Layout CRs](#non-bundle-child-layout-crs)). A Kustomization already in the tree with that name, in the namespace of the generated CRs, is kept in place of a generated one when it sits in the layout that would host it and has the same `spec.path`, and is refused otherwise. Finest granularity.
- `FluxIntegratedPerBundle` - Flux Kustomization CRs at **bundle boundaries only**, each hosted in its parent layout; a bundle's interior (application and augmenter-added child layouts) is a single kustomize build, with those children referenced as directories. A child that renders bundles is not referenced: its own CR applies it. Coarser: Flux reconciles per bundle, kustomize handles the interior.

External augmenters may add child layouts that are not represented in the bundle model; integrated placement discovers those layouts and emits the required Flux resources.

## Umbrella Bundles

A `Bundle` with a non-empty `Children` slice becomes an **umbrella**: a parent
Flux Kustomization that aggregates the readiness of its children via
auto-generated `spec.healthChecks`. This gives downstream
consumers a single stable anchor regardless of how many internal tiers the
umbrella contains.

### Resource generation

`ResourceGenerator.GenerateForBundle` detects umbrella bundles and:
- prepends one `HealthChecks` entry per direct child (referencing the child's
  own Kustomization by name/namespace; the name is the child's `KustomizationName` when it sets
  one)
- leaves user-supplied `HealthChecks` appended after the auto entries

`spec.wait` is **not** forced to `true` here; it is the caller's `Bundle.Wait` input like any
other field. Forcing it was self-defeating: upstream documents that when `wait` is enabled
"the HealthChecks are ignored", so the auto entries the generator had just built were inert.
Leaving `wait` unset is what makes them take effect. A caller who does set `Wait=true` gets
upstream's whole-of-resources health assessment instead, which also gates on the child
Kustomizations.

`GenerateForBundle(b, path)` is strictly self-only — it never recurses into
`b.Children`. `GenerateFromLayout` and `GenerateFromCluster` cover the whole
umbrella closure, because the walker renders every umbrella child as a layout
of its own. It builds no origin index either, so it cannot see every bundle
that shares a Kustomization name; what it does see it checks: a `DependsOn`
bundle or a child that would get `b`'s own Kustomization name
(`KustomizationName`, or `Name` without it), `b` itself in `DependsOn` or in
`Children`, that name in `NamedDependsOn`, or a health check on `b`'s own
Kustomization, is an error, since the Kustomization would otherwise depend on
itself or wait for itself. A longer cycle of `Children` is `Bundle.Validate`'s
to refuse. Two children that would get one Kustomization name are an error as
well: the umbrella would health-check one Kustomization twice. The health
check on `b`'s own Kustomization stays accepted when `b.Wait` is true, and is
written as given: Flux ignores `spec.healthChecks` under `spec.wait`, so
nothing waits for itself. `GenerateFromLayout` and `GenerateFromCluster` drop
a bundle's dependency or health check on itself instead.

The name a Kustomization gets is the bundle's name in effect (`UnitName()`:
its `KustomizationName`, or its `Name` without one), so the generator refuses
one that `stack.ValidateKustomizationName` refuses: not a DNS-1123 subdomain,
or longer than 63 characters. The check runs before anything is written: in
`GenerateFromCluster`, `GenerateFromLayout`, `GenerateForBundle` and the
layout integrator under every `FluxPlacement`. The 63-character limit is
checked only there. It is Flux's, so `stack.ValidateCluster` and
`Bundle.Validate` accept a name of up to 253 characters, which the
ArgoCD workflow renders; they do refuse a `Name` or `KustomizationName` that
is not a DNS-1123 subdomain, so that refusal reaches `GenerateFromCluster` and
`CreateLayoutWithResources` from cluster validation first. A `Name` over 63
characters beside a `KustomizationName` within the limit generates: that
`Name` is no Kustomization's. The error names the bundle by `Bundle.GetPath`,
an umbrella child of a walked cluster with its umbrella's path before its own
name (`platform/platform-infra`), and the field that holds the name.

Which names are checked follows from what the entry point returns:

- `GenerateForBundle` takes a bundle no validation has seen and returns one
  Kustomization. It checks the bundle's own name and the names that
  Kustomization refers to: each umbrella child, named in a health check, and
  each `DependsOn` bundle, named in `spec.dependsOn`. It builds no
  Kustomization for them, so it refuses an umbrella whose child's name Flux
  cannot reconcile instead of returning a reference to it.
- `GenerateFromCluster`, `GenerateFromLayout` and the layout integrator build
  one Kustomization per directory that renders bundles, and check that
  Kustomization's name, which is its first bundle's (see "One Kustomization
  per directory"). The name of a bundle merged into it is written nowhere and
  is not checked. A reference to one of the cluster's bundles, an umbrella
  child or a `DependsOn` bundle, is written as the name of the Kustomization
  that applies that bundle, which the same call builds and checks. A
  `DependsOn` bundle that is not one of the cluster's bundles gets no
  Kustomization from the call and is written under its own name, so that name
  is checked like the bundle's own.
- A `NamedDependsOn` entry and a health check the caller lists are not
  checked. `GenerateForBundle` writes them as given; the other entry points
  write one that names a rendered bundle's Kustomization as the name of the
  Kustomization that applies that bundle, and any other as given.

### Placement in layouts

`LayoutIntegrator` places umbrella child Flux CRs at the **parent** layout
node, with `spec.path` = the child's own directory:

- **Integrated, `BundleGrouping: GroupByName`**: the walker creates a bundle sub-layout
  under the node layout. Umbrella child Kustomization CRs are appended to the
  bundle sub-layout's `Resources`, which that layout lists (it is written in
  `KustomizationExplicit` mode); their Source CRs, if the child has a
  `SourceRef.URL`, go to the root node's layout like every generated Source. Nested umbrella children are placed at their
  enclosing umbrella child's layout node.
- **Integrated, `BundleGrouping: GroupFlat`**: there is no intermediate bundle
  layer, so umbrella children become direct sub-layouts of the node layout,
  and their Flux CRs sit at the node layout. The root node is the exception
  (go-kure/kure#979): its bundle has a directory of its own inside the root
  node's, and the umbrella children and their Flux CRs sit there.
- **FluxSeparate**: the `flux-system` layout directory receives every bundle's
  Kustomization CR, umbrella descendants included, as a flat list.

Under both integrated placements a node bundle's own CR is hosted by the
**parent** of the layout that renders the bundle: that directory is applied by
its own Kustomization only, so the CR that creates it cannot live inside it.
The root node's bundle has a parent too, the root node's layout
(go-kure/kure#979).

### On-disk shape

When a parent layout has an umbrella child, the parent's `kustomization.yaml`
lists the child's Kustomization CR file (it is one of the parent's own
resources) instead of the child subdirectory. The child subdirectory still exists and still contains its own
`kustomization.yaml` plus workload YAML files — but **no** Flux CR files, so
Flux does not double-apply the child's resources.

## Non-Bundle Child Layout CRs

In `FluxIntegratedPerLayout` mode every child layout that is not an umbrella child, not `AppFileSingle` and renders no bundle gets a `Kustomization` CR in its parent's `Resources`, with `spec.path` set to `child.FullRepoPath()`. (A child that renders a bundle already has that bundle's CR there.) The CR's name is checked with `stack.ValidateKustomizationName` where the CR is created: one that is not a DNS-1123 subdomain of at most 63 characters is refused, and the error names what to set to change it (see [Per-layout name rule](#per-layout-name-rule)). This covers:

- **Application layouts** — per-app layouts (`ApplicationGrouping: GroupByName`, or augmenter apps, which keep a directory under `GroupFlat`). The CR is named `<unit name>-<layout name>`.
- **Augmenter sub-layouts** — hook-group child layouts added by a `LayoutAugmenter` are children of an app layout. They are named `<unit name>-<layout name>` as well, and `spec.dependsOn` is populated from `ManifestLayout.DependsOn`, enabling ordered reconciliation between hook groups.
- **Bundle-less node layouts** — a GroupByName node layout above its bundle layout, or a node without a bundle. The CR is named `<path with "/" replaced by "-">-node` (with `ClusterName: "."`, node `web`'s path is `web`, which is also its bundle's CR name when the bundle sets no `KustomizationName`), unless the node names it: see [Node Kustomizations](#node-kustomizations).

The unit name is the name in effect of the Kustomization that applies the bundles the layout belongs to: the nearest layout at or above its parent that renders bundles (`Bundle.KustomizationName`, or the bundle's `Name`; the first bundle's when a grouping axis merged several into one directory). The application `web` of the bundle `web` therefore gets the Kustomization `web-web`, not a second `web`. `ManifestLayout.KustomizationName` replaces the default name: an augmenter sets it on a layout it creates, and a caller may set it on a walked layout before integration.

### Per-layout name rule

The name in effect of a per-layout CR is a Flux Kustomization name: a DNS-1123 subdomain of at most 63 characters (`stack.ValidateKustomizationName`). It is checked where the CR is created, so nothing is written for a tree with a name Flux could not reconcile. kure does not shorten a name: the error names the object and the field that changes it.

| Where the name comes from | The error names |
|---|---|
| `Node.KustomizationName` | the node by its path (`Node 'prod/apps'`) and the field `kustomizationName` |
| the name derived for a node, `<path>-node` | the node, and `Node.KustomizationName` as the field to set |
| `ManifestLayout.KustomizationName`, set by an augmenter or on a walked layout | the layout by its directory (`ManifestLayout 'prod/web/api'`) and the field `kustomizationName` |
| the default of an application or augmenter layout, `<unit name>-<layout name>` | the layout, and `ManifestLayout.KustomizationName` as the field to set |

The default of an application or augmenter layout is longer than the layout's own name by the unit name, so a directory name within the limit can give a default over it: the application `checkout-service-payments-reconciler-worker` (43 characters) of the bundle `platform-services-payments` (26) gets a default of 70 characters and is refused until its layout sets a `KustomizationName`.

A `ManifestLayout.DependsOn` entry on an application or augmenter layout is a layout name. Where it names a layout of the same unit that has a CR of its own, the entry is written as that layout's CR name: a sibling first, else the one layout with that name elsewhere below the unit's directory (the parent application layout of a hook group, for instance). Two such layouts with no sibling among them are refused, since the entry does not say which: set `KustomizationName` on the one meant and list that name. Any other entry is written as given, as the name of a Kustomization.

The integrator applies this rule at any depth. The CR's `spec.sourceRef` is the `SourceRef` of the nearest layout at or above the host that renders bundles; with `BundleGrouping: GroupFlat` the root node's layout renders none and counts with the `SourceRef` of its own bundle, rendered in the directory inside it (`OriginUnit`, go-kure/kure#979), so a bundle-less child node of the root still takes the root bundle's source; with none, the one `SourceRef` the URL-less bundles below the child share; with none of those either (no `SourceRef` without a URL below the child, or two that differ), the `SourceRef` of the bundle of the nearest node, at or above the child's own, that has one. That last rule is what a node's layout takes with `BundleGrouping: GroupByName`, where no node's layout renders a bundle, the root's included, when every `SourceRef` at and below it has a URL or the ones without a URL below it differ: its own node's bundle's, rendered one directory lower, or for a node without a bundle the one of the nearest node above, the bundle that encloses it with `GroupFlat` (go-kure/kure#979). It comes last, so it gives a source only to a layout that has no other. The CR of the root node's layout (below a `ClusterName` wrapper; on a tree built by hand, that of any layout above it too) takes no Source the integration generates, whichever of these would give it one (see [Layout Integration](#layout-integration): no Kustomization takes its source from inside what it applies). A missing, incomplete or ambiguous source is a hard error — a `Kustomization` without a valid `spec.sourceRef` is rejected by Flux and must not be emitted silently — and so is a layout whose bundles have different `SourceRef`s (a `NodeGrouping: GroupFlat` merge) hosting a layout CR. A node without a bundle, with no bundle on a node above it, above bundles whose `SourceRef`s all have a URL has no source under either `BundleGrouping`: the error names its Kustomization and the three ways out (a `SourceRef` without a URL on a bundle below, a bundle with a `SourceRef` on a node at or above, or `FluxIntegratedPerBundle`). Such a node above bundles whose `SourceRef`s without a URL differ has none either, and its error names the bundle on a node at or above it as the way out: without a URL where the layout is the root node's below a `ClusterName` wrapper. A CR name used twice (a node whose `KustomizationName` is also a bundle's Kustomization name, say) is an error, not a silent skip: Flux Kustomizations share one namespace. When this integration generates both, the error names both owners, each as `bundle "<path>"`, `node "<path>"` or `layout "<directory>"`; a clash with a Kustomization already in the tree names the two layouts that hold them.

### Per-layout settings

Besides its name, path, source and dependencies, a per-layout Kustomization carries eleven settings. Five came with go-kure/kure#1015: `spec.wait`, `spec.timeout`, `spec.retryInterval`, `metadata.labels` and `metadata.annotations`. Six came with go-kure/kure#1021: `spec.interval`, `spec.prune`, `spec.force`, `spec.suspend`, `spec.postBuild` and `spec.patches`. They come from the bundle and, all but the last two, from the layout.

**From the bundle.** The Kustomization of an application's own layout, and of every layout below it at any depth (the ones the application's `LayoutAugmenter` added), takes them from the bundle that holds the application: `Bundle.Wait`, `Timeout`, `RetryInterval`, `Labels`, `Annotations`, `Interval`, `Prune`, `Force`, `Suspend`, `PostBuild` and `Patches`. Where a grouping axis merged several bundles into the directory above the application's, that bundle is the one whose `Applications` lists the application, not the first of them and not the merged set. An application of an umbrella child inherits from the child bundle, not from the umbrella above it.

**From the layout.** `layout.ManifestLayout` has nine of the fields, set by an augmenter on a layout it creates or by a caller on a walked layout before integration:

| Layout field | Unset | Set |
|---|---|---|
| `Wait` (`*bool`) | the bundle's | replaces the bundle's; a pointer to `false` turns an inherited wait off |
| `Timeout`, `RetryInterval` | the bundle's | replaces the bundle's. An inherited value can be replaced, not removed |
| `Labels`, `Annotations` | the bundle's | merged with the bundle's key by key, the layout's value winning. An inherited key can be given another value, not dropped |
| `Interval` | the bundle's, or else `ResourceGenerator.DefaultInterval` | replaces the bundle's |
| `Prune` (`*bool`) | the bundle's, or else `ResourceGenerator.Prune` | replaces the bundle's; a pointer to `false` turns an inherited prune off |
| `Force`, `Suspend` (`*bool`) | the bundle's; with neither set the field is left out | replaces the bundle's; a pointer to `false` turns an inherited one off |

A layout's own fields apply to that layout's Kustomization only: a layout below it inherits from the bundle again, not from the layout above. The fields are read on every layout that gets a Kustomization of its own under this placement, a node's layout included (see [Node Kustomizations](#node-kustomizations)); a layout that belongs to no application inherits nothing, so its own fields are all it has. On a layout that gets no Kustomization of its own, and under `FluxSeparate` and `FluxIntegratedPerBundle`, the fields are dropped without an error, as `DependsOn` and `KustomizationName` are.

**Interval and prune fall back to the generator's.** Where neither the layout nor the holding bundle sets one, `spec.interval` is `ResourceGenerator.DefaultInterval` and `spec.prune` is `ResourceGenerator.Prune`, the values every per-layout Kustomization had before the two were read from the bundle: a tree that sets none of the six renders byte for byte as it did. A bundle that does set one now passes it on. In particular a bundle with `Prune` on turns garbage collection on for the Kustomizations of its applications' layouts, which rendered with the generator's `Prune` before, and a bundle's `Interval` replaces the generator's on them.

**The substitution is the bundle's, whole.** A per-layout Kustomization takes the holding bundle's `PostBuild` as it is, the inline variables and the `SubstituteFrom` references, each Kustomization with a copy of its own. There is no layout field for it. Flux reads a referenced ConfigMap or Secret from the Kustomization's namespace, and every Kustomization of the integration is in the generator's, so a reference the bundle's own Kustomization resolves is one the per-layout ones resolve.

**The parent's substitution leaves them as written.** A per-layout Kustomization is a resource of its parent's build, whose postBuild substitution runs over it before it applies its own postBuild and patches. Where that would change its postBuild or its patches at all (a `${VAR}`, plain or not, or an escape `$${VAR}`), the integration writes Flux's opt-out annotation `kustomize.toolkit.fluxcd.io/substitute: disabled` on it, and the Kustomization applies them as if no parent had run. What a patch the parent substituted first comes to depends on the objects it patches (one may carry the opt-out itself, and a value's YAML type is read from the object), so any change counts. The opt-out also stops the parent substituting the Kustomization's labels, annotations and path, so where the parent substitutes one of those as well the integration is refused, naming the Kustomization, its layout and the fields. The annotations are checked once more with the opt-out in them, and the integration is refused, naming the layout, where they are then more than the total size the Kubernetes API accepts.

**No health checks.** A per-layout Kustomization takes no `spec.healthChecks`: `wait` is its readiness setting, and with `wait` Flux ignores health checks. A bundle's `HealthChecks` stay on the bundle's Kustomization.

**An application rendered in the bundle's directory** (flat `ApplicationGrouping`, no `LayoutAugmenter`) has no Kustomization of its own: the bundle's Kustomization applies it and its settings are the bundle's, as before.

#### Patches of a bundle with per-layout Kustomizations

Under this placement the bundle's Kustomization builds the bundle's directory, and the Kustomizations of its applications' directories build those. A patch reaches an object only from the Kustomization whose build holds the object, so a bundle's `Patches` are placed by object once every Kustomization and Source of the tree is placed. There is no layout field for patches.

A build holds the objects of its directory and of the directories its `kustomization.yaml` reaches, the Flux Kustomizations and Sources hosted there included, and the ConfigMap each `configMapGenerator` entry there generates, which a patch names by the entry's name. A generated ConfigMap has no namespace: an entry has no field for one and the `kustomization.yaml` kure writes sets none for the directory. Kustomize reads that as `default`, so a patch document names it without a namespace or in `default`; a document that names it in another namespace names an object no build holds.

| Patch | Written on |
|---|---|
| with a `Target` | the bundle's own Kustomization and every per-layout Kustomization of the bundle. Kustomize applies it to what the target selects in that build, and to nothing where it selects nothing |
| without a `Target`, strategic-merge | the Kustomizations of the bundle, its own among them, whose build holds every object the patch names, and no other. Objects are compared as kustomize compares them: group, version, kind, name and namespace, a document without a namespace naming the object in `default` |
| without a `Target`, strategic-merge, after an entry of `Bundle.Patches` that is not a plain strategic-merge patch | the bundle's own Kustomization, and besides the per-layout Kustomizations whose build holds every object the patch names. Never refused; see "After an entry that is not a plain strategic-merge patch" below |
| without a `Target`, not a plain strategic-merge patch (a JSON6902 patch, a patch with one of kustomize's build annotations, or one that removes its object: a top-level `$patch: delete`, or `config.kubernetes.io/local-config` with any value but `"false"`) | the bundle's own Kustomization only, and never refused. This is a limit: it is not read, so there is no object to place it by. Kustomize refuses a JSON6902 patch without a target wherever it is |

Each Kustomization keeps the order of `Bundle.Patches` among the patches it takes.

**The bundle's own Kustomization loses a patch.** An untargeted strategic-merge patch whose object the bundle's own Kustomization does not build is no longer written on it: it is on the Kustomization that builds the object, and there alone. Before go-kure/kure#1021 it was on the bundle's own, whose build fails on the cluster with `no resource matches strategic merge patch` when the object is in an application's directory.

**A failure moves from the cluster to the integration.** Kustomize fails the build of a Kustomization that holds an untargeted strategic-merge patch for an object it does not build. Two cases are therefore refused before anything is written, the error naming the bundle, the patch by its index in `Bundle.Patches`, the object and the Kustomizations looked at:

- no Kustomization of the bundle builds an object the patch names. Give the patch a `Target` (for a patch of several objects: one patch per object first, each with its `Target`), or place the object in a build of the bundle.
- every object a multi-document patch names is built, but no one Kustomization builds them all. The documents of one patch are applied in one build: write one patch per object.

**A `Target` is for a patch of one object.** Kustomize refuses a target on an entry that holds several strategic-merge patches, before it selects anything: ``Multiple Strategic-Merge Patches in one `patches` entry is not allowed to set `patches.target` field``. It counts the objects of the entry: its documents, and for a document that is a `List` its items, each of which it loads as a patch of its own. A patch with a target is written on the bundle's own Kustomization and on every per-layout one, so each of their builds then fails, the ones whose objects the target does not select included. Wherever this section gives a `Target` as the remedy, a patch of several objects is therefore first written as one entry of `Bundle.Patches` per object, each with its own `Target`.

A tree that built on the cluster before is not refused, and its bundle's own Kustomization builds what it built. Where every entry of the list is a plain strategic-merge patch (next paragraph), that build held every object its untargeted patches name, so each of those patches stays there, and is written on a per-layout Kustomization as well only where that one builds an object of the same identity. For a bundle without per-layout Kustomizations nothing changes at all: its own Kustomization keeps every patch, byte for byte, and no patch is examined.

**After an entry that is not a plain strategic-merge patch.** The objects of a build are compared as generated, which is what a patch names only while no earlier entry of the list has changed an object's identity. Placement by object therefore holds for a patch only while every entry of `Bundle.Patches` before it, and the patch itself, is a *plain* strategic-merge patch: its text parses as resources, no document of it carries an annotation whose key begins with `internal.config.kubernetes.io/`, the domain kustomize keeps its own build state in, and no document removes its object from the build, with a top-level `$patch: delete` or with the annotation `config.kubernetes.io/local-config` at any value but `"false"`. Kustomize keeps the `apiVersion`, kind, name and namespace of the object a plain patch is merged into, with a `Target` or without, and a Flux patch has no option that allows the change. Any other entry is not known to keep them:

- a JSON6902 patch with a `Target` may replace `metadata.name`, `metadata.namespace`, `kind` or `apiVersion`, and kustomize then matches a later strategic-merge patch against the new identity as well as the one before;
- kustomize reads its build annotations from the text of a patch as it reads its own: `internal.config.kubernetes.io/allowNameChange` and `allowKindChange` let a strategic-merge patch change the name or the kind of the object it is merged into, and the `previousNames`, `previousNamespaces` and `previousKinds` annotations give a document the identity it names its object by;
- a document with a top-level `$patch: delete` removes its object from the build, and so does one that gives it `config.kubernetes.io/local-config` with any value but `"false"`: kustomize drops every object with that annotation once its patches have run. One that sets the annotation to null counts as well, since what the merge leaves is not read. Where that object is a per-layout Kustomization, the objects of its build are no longer applied, so a later untargeted patch placed by object on that Kustomization alone would never run.

kure does not read what such an entry does, so each one counts, one that changes no identity included. The entry itself is written where it was before go-kure/kure#1021 and is never refused: with a `Target` on the bundle's own Kustomization and on every per-layout one, without one on the bundle's own only. An untargeted strategic-merge patch that comes after it in `Bundle.Patches` is not placed by object and never refused either: it stays on the bundle's own Kustomization, where every patch was before, and is written besides on each per-layout Kustomization whose build holds every object it names as generated. A bundle that renames an object of its own directory and then patches it, by the new name or the old, renders its own Kustomization byte for byte as before.

This is a limit. Three cases are as they were before go-kure/kure#1021: the build of the bundle's own Kustomization fails on the cluster with `no resource matches strategic merge patch`, and the integration refuses nothing.

- A later untargeted patch names an object only a layout builds. It is on that layout's Kustomization, and on the bundle's own, which does not build the object. Put the untargeted patch before the entry in `Bundle.Patches`, or give it a `Target`.
- A later untargeted patch names the name the entry gives an object of a layout. The entry, when it has a target, is on that layout's Kustomization and renames the object there; the untargeted patch is not, since that build does not hold the new name as generated. Give the patch a `Target` that selects the new name: it is then written on every Kustomization and applied where the object is.
- An untargeted patch with a build annotation, or one that removes its object, names an object only a layout builds. It is on the bundle's own Kustomization alone. Give it a `Target`.

Each `Target` here is for a patch of one object; a patch of several is first written as one entry per object, as above.

**A patch on a Kustomization of the bundle** lands on the Kustomization whose build holds that object, which is the one that builds the directory hosting it. The Kustomization of an application's own layout is hosted in the bundle's directory, so a patch that names it is written on the bundle's own Kustomization; the Kustomization of a layout below is hosted in the layout above it, so a patch that names it is written on that layout's Kustomization. A patch that names the bundle's own Kustomization is refused, since no build of the bundle holds it. The check that the generated Kustomizations can all become Ready reads them as generated, not as patched: what a patch changes on one of them (`dependsOn`, `wait`, `healthChecks`) is not part of it, as before.

**Bundles merged into one directory.** The shared Kustomization keeps the merged patches as [Patches in a shared directory](#patches-in-a-shared-directory) describes, untargeted ones being refused there already; a per-layout Kustomization below takes the targeted patches of the bundle that holds its application.

**Bundle labels and annotations.** In a tree walked from a cluster, `Bundle.Labels` and `Bundle.Annotations` are on the bundle's Kustomization and on the per-layout Kustomizations above, and on no object the applications generate: the walker generates each application on its own, and only `Bundle.Generate` adds them to objects.

**Refusals.** The settings in effect are checked where the Kustomization is created, before anything is written. An `Interval`, `Timeout` or `RetryInterval` must parse as a Go duration and be one Flux takes, neither negative nor positive and under a millisecond (see [Durations](#durations)). Every label and annotation in effect is checked with the validators the Kubernetes API uses for `metadata.labels` and `metadata.annotations`, key by key and then as a whole (the total size of the annotations). The error names the layout by its directory (`ManifestLayout 'prod/shop/db'`) and the field, and for an inherited entry the bundle (`inherited from bundle "shop"`). A bundle's own Kustomization carries the bundle's labels and annotations unchecked, as before: the check is made only where a per-layout Kustomization takes them. The two refusals of an untargeted patch are in [Patches of a bundle with per-layout Kustomizations](#patches-of-a-bundle-with-per-layout-kustomizations).

**A second integration** keeps the per-layout Kustomization the first one placed, as it is: its settings and patches are not written again, so a field changed on the layout or the bundle in between is not on it. A kept Kustomization still counts as a build that can hold a patch's object, so a second integration of an unchanged tree refuses nothing the first took and changes no byte: the patch for an object of a layout is on that layout's kept Kustomization already. A patch the bundle got, lost or changed between two integrations of one tree is not written on a kept Kustomization and not taken off it: the kept Kustomization then carries the patches of the first integration, stale like every other setting on it, and a new untargeted patch whose object only a kept Kustomization builds is written on none. Remove the kept Kustomization or walk the cluster again.

**Readiness through the chain.** With `wait`, a Kustomization health-checks every object it applied, the Kustomization objects among them, and becomes Ready only after that check. A Kustomization object is healthy once the controller has observed its current generation and its `Ready` condition is true. With `Bundle.Wait` set, the bundle's Kustomization is therefore Ready only when the Kustomization of each application directory is, which is Ready only when the Kustomization of each layout below it is, each of them Ready only when the objects in its directory are: a `dependsOn` on the bundle's Kustomization waits for the workloads, where it used to wait only until each directory had been applied. This is how kustomize-controller v1.9.5 reads, with the status rules of fluxcd/cli-utils v1.2.3, and it holds for a first apply and for every change of a Kustomization's own spec. Two limits:

- **A change of content only.** When a new revision of the source changes the files in a layout's directory and leaves the layout's Kustomization object as it was, the Kustomization above finds that object unchanged, with its generation observed and `Ready` true from the revision before. The health check does not look at the revision an object last applied, so the Kustomization above can be Ready for the new revision before the one below has applied it.
- **Nested timeouts.** A health check runs under its own Kustomization's timeout (`spec.timeout`, or the interval less 30 seconds without one) and has to outlast everything below it. One timeout inherited on every level gives the innermost Kustomization as long as the outermost, which then fails at the moment the innermost would. Set a shorter `Timeout` on the layouts below: the layout fields are how the layouts get a shorter timeout than their bundle. Flux raises any timeout under 30 seconds to 30 seconds, so a shorter value gives no margin below that: the timeout above has to exceed the timeout in effect below, which is never less than 30 seconds.

**Wait and a dependency on the layout above.** A layout that lists the application layout above it in `DependsOn` (a hook group that applies after the application's own objects) cannot be combined with `wait` on that application layout's Kustomization: it would wait for the hook group's Kustomization, which waits for it. With `Bundle.Wait` set both now have `wait`, and the reconcile-order check refuses the tree, naming the chain, where it rendered before. Set `Wait` to a pointer to `false` on the application's layout; the layouts below keep the bundle's.

### Node Kustomizations

A node whose directory renders no bundle (a group of nodes, or a node above its bundle's own directory under `BundleGrouping: GroupByName`) is applied by a Kustomization of its own. Three fields of `stack.Node` name and order it:

<!-- doc-example: pkg/stack/fluxcd ExampleLayoutIntegrator_nodeKustomization -->
```go
// Two groups of nodes, neither with a bundle of its own. Each group gets
// a Kustomization that applies its directory; the one for apps waits for
// the one for platform.
source := &stack.SourceRef{Kind: "GitRepository", Name: "flux-system", Namespace: "flux-system"}
platform := &stack.Node{Name: "platform", KustomizationName: "platform", Children: []*stack.Node{
    {Name: "cert-manager", Bundle: &stack.Bundle{Name: "cert-manager", SourceRef: source}},
}}
apps := &stack.Node{Name: "apps", KustomizationName: "apps", DependsOn: []*stack.Node{platform}, Children: []*stack.Node{
    {Name: "shop", Bundle: &stack.Bundle{Name: "shop", SourceRef: source}},
}}
cluster := &stack.Cluster{Name: "prod", Node: &stack.Node{Name: "prod", Children: []*stack.Node{platform, apps}}}

rules := layout.DefaultLayoutRules()
rules.FluxPlacement = layout.FluxIntegratedPerLayout
ml, err := fluxcd.NewLayoutIntegrator(fluxcd.NewResourceGenerator()).CreateLayoutWithResources(cluster, rules)
if err != nil {
    panic(err)
}
for _, obj := range ml.Resources {
    kust, ok := obj.(*kustv1.Kustomization)
    if !ok {
        continue
    }
    var waitsFor []string
    for _, dep := range kust.Spec.DependsOn {
        waitsFor = append(waitsFor, dep.Name)
    }
    fmt.Println(kust.Name, kust.Spec.Path, waitsFor)
}
```
<!-- doc-example:end -->

It prints `platform prod/platform []` and `apps prod/apps [platform]`: each group's Kustomization has the name set on the node and applies the node's directory, which the field does not move.

| Field | Effect |
|---|---|
| `KustomizationName` | names the node's Kustomization instead of the derived `-node` name. The name in effect, set or derived, is held to the [per-layout name rule](#per-layout-name-rule): a derived name over 63 characters is refused until the node sets this field |
| `DependsOn` | nodes whose Kustomizations this one waits for. Each is written into `spec.dependsOn` under the target's name in effect: its `KustomizationName`, or its derived `-node` name. A target that has no Kustomization of its own is refused, and the error names both nodes |
| `NamedDependsOn` | Kustomization names, written as given after the `DependsOn` entries. An entry is a caller-supplied reference: it need not belong to this cluster and its value is not checked, as for a bundle's. An empty entry, a repeated one, and one that names the Kustomization of a node in `DependsOn` are refused, the node named by its path |

The three fields are refused, not ignored, wherever the node has no Kustomization of its own. The error names the node by its path from the root (`node "prod/apps"`) and the fields it sets:

- under `FluxSeparate` or `FluxIntegratedPerBundle`, and in `GenerateFromCluster` and `GenerateFromLayout`: only bundles get a Kustomization there;
- on a node that `NodeGrouping: GroupFlat`, or a `FlattenSingleTier` collapse, renders into another node's directory;
- on a node whose directory renders a bundle: that bundle's Kustomization applies the directory, so name and order the bundle (`Bundle.KustomizationName`, `Bundle.DependsOn`);
- on the node whose directory is the top of the written tree, which the Flux bootstrap applies;
- on a node whose layout a caller marked `UmbrellaChild` after the walk.

The ArgoCD workflow generates nothing for a node and refuses the fields as well. A cycle through node dependencies is refused by the reconcile-order check, like any other.

Set `KustomizationName` and `NamedDependsOn` before the cluster is walked. The walker copies both onto the node's layout (`ManifestLayout.KustomizationName`, `ManifestLayout.DependsOn`) and the Kustomization is built from the layout, so a name or an entry the node gets between `WalkCluster` and `IntegrateWithLayout`, or before a second integration, is not there. That is refused, naming the node and its layout: walk the cluster again, or put the value on the layout. A name a caller sets on the walked layout wins over the node's. `DependsOn` is read from the node at integration.

A second integration of the same tree keeps the node Kustomization the first one placed. If the node sets `DependsOn` or `NamedDependsOn` and that Kustomization's `spec.dependsOn` lacks one of the Kustomizations they ask for (in the Kustomization's own namespace), the integration is refused rather than dropping the dependency: remove the kept Kustomization, add the entry to it, or walk the cluster again.

A node Kustomization inherits no settings and `stack.Node` has no field for any. `spec.interval` and `spec.prune` are those the node's layout sets (`ManifestLayout.Interval`, `Prune`) or else the generator's (`ResourceGenerator.DefaultInterval` and `Prune`). `spec.wait`, `spec.timeout`, `spec.retryInterval`, `spec.force`, `spec.suspend`, labels and annotations are those the node's layout sets (`ManifestLayout.Wait`, `Timeout`, `RetryInterval`, `Force`, `Suspend`, `Labels`, `Annotations`, on the walked layout before integration: see [Per-layout settings](#per-layout-settings)) and are left out otherwise. It takes no `spec.postBuild` and no `spec.patches`: those are a bundle's, and a node's layout has no holding bundle. What a group node applies is the Kustomizations of the nodes below it; the settings of the workloads stay on their bundles. Without `Wait` on its layout, a node Kustomization is therefore Ready once those Kustomization objects are applied, and a dependency on it (`Node.DependsOn`) waits for that, not for the workloads below.

## Validation

All cluster-level entry points (`GenerateFromCluster`, `CreateLayoutWithResources`)
call `stack.ValidateCluster` before walking the tree, and every generation from a
layout runs `layout.IndexOrigins` (see [Kustomization paths](#kustomization-paths)). Invalid umbrella
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
(kustomize-controller v1.9.5), on the `spec.interval` of a `GitRepository` and an `OCIRepository`
(source-controller v1.9.5) and on a `FluxInstance`'s `spec.sync.interval` (flux-operator v0.58.1):

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
