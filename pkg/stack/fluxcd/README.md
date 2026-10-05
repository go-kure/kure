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
at `<root>/<bundle>` and its children at `<root>/<child>`, which is also what the bootstrap sync
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

`Bundle.Name` stays the bundle's identity and its directory, so `spec.path` is the same with and
without the field. `Bundle.UnitName()` returns the name in effect, and everything that refers to
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
bundle has a directory inside it, named after the bundle, as the last two rows show. Before, the
bundle rendered in the root node's directory (`prod/platform` and `.` in those rows), and the
Kustomization that applied that directory was hosted inside it. See "The root node's bundles" in
the layout package README, also for the path move this is on a deployed tree.

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
the layouts inside it, as every bundle's directory does. ArgoCD
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
kustomize resources) and objects without a kind. Known gap: under `FluxIntegratedPerLayout` the
check does not see a generated ConfigMap that the shared directory itself builds. A
`FlattenSingleTier` collapse was the known way to such a directory, and it no longer collapses a
directory that renders a bundle; whether another input reaches the gap is open (go-kure/kure#979).

The generator computes no path, the integrator matches nothing by name, and `FlattenSingleTier`
rewrites nothing afterwards: it collapses no directory that renders a bundle
(go-kure/kure#979), so it changes no Kustomization's path and merges no bundles. An umbrella
child's path is its own directory, in every mode (an earlier `KustomizationRecursive` rule
pointed it at the parent bundle's directory, whose kustomization excludes the child).

`IndexOrigins` refuses a tree it cannot resolve unambiguously: a hand-built, partial or
other-cluster tree (the rendered set is not what the cluster reaches), a bundle or node rendered
twice, two bundles with one name or with one Kustomization name, and a node or bundle
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
| `DefaultInterval` | `60m` | the caller names no interval (a non-empty `Bundle.Interval` that does not parse is a validation error, not a fallback) | assigning `.DefaultInterval` on either generator |
| `DefaultNamespace` | `flux-system` | generated resources need a namespace | assigning `.DefaultNamespace` on either generator |
| `DefaultMode` | `layout.KustomizationExplicit` | informational: the listing mode the layout writers use for a layout with no `Mode` of its own | setting `ManifestLayout.Mode` (or `Config.KustomizationMode` for `WriteManifest`) |
| `DefaultBootstrapName` | `flux-system` | naming the bootstrap Kustomization | assigning `BootstrapGenerator.BootstrapName` |
| `FluxInstanceName` | `flux` | naming the `FluxInstance` in `flux-operator` mode | not overrideable; the CRD admits no other name (see below) |
| `DefaultSourceName` | `flux-system` | the root node has no name | naming the root `stack.Node` |
| `DefaultFluxMode` | `flux-operator` | `BootstrapConfig.FluxMode` is empty | setting `BootstrapConfig.FluxMode` |
| `DefaultSourceKind` | `OCIRepository` | `BootstrapConfig.SourceKind` does not name `GitRepository`, the empty string included | setting `BootstrapConfig.SourceKind` |
| `DefaultFluxDirName` | `flux-system` | a separate Flux layout needs a directory | not overrideable |
| `DefaultSourceRef` | `latest` | an OCI source, or an OCI `FluxInstance` sync, has no `SourceRef` | setting `BootstrapConfig.SourceRef` |
| `DefaultSyncPath` | `./` | the root node has no name | not overrideable; it is the prefix a sync path is built from |

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
the id twice. A generated Source has one definition across the whole pass: every Source of its
identity — kind, namespace and name, whatever the API version (a `v1beta2` `GitRepository` is the
same Source as the generated `v1` one) — anywhere in the tree, top-level or inside a `List`, and
every Source the pass places in another layout, must have the same API version and content, or the
integration is refused. Content is compared as the objects' unstructured form, so a typed Source
and an unstructured copy of it are the same.

Under the integrated placements every Source the integration generates is hosted **once, in the
root node's layout**, whichever layouts hold the Kustomizations that use it (go-kure/kure#876),
and not in a `ClusterName` wrapper above it. For a named root node under the default rules that is
the directory the bootstrap sync path `./<root>` names (see [Kustomization paths](#kustomization-paths)
for other rules), so the Source is in one build, the root's, and exists before any Kustomization
that uses it.

**No Kustomization takes its source from inside what it applies** (go-kure/kure#979). A generated
Source is in a build that is applied before every Kustomization that names it, never in a
directory that Kustomization delivers: where nothing else applies that directory (Flux pointed at
the top of the written tree, the `ClusterName` directory), the Kustomization waits for a Source
only its own apply creates and never reconciles. On a walked tree every bundle's Kustomization meets the rule
by construction, because the root node's directory renders no bundle. The one Kustomization that
applies the root node's directory is that directory's layout Kustomization below a `ClusterName`
wrapper under `FluxIntegratedPerLayout`, hosted in the wrapper. It takes no
generated Source: the `SourceRef` of the root node's bundle is passed over when it has a URL or
names a Source another `SourceRef` generates, and its source is the one `SourceRef` the other
URL-less bundles below share. When nothing is left (every `SourceRef` in the tree has a URL, say)
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

A Kustomization whose `spec.path` build holds that directory applies it beside the
bootstrap: both hold the same objects, so neither prunes what the other keeps, but the
Kustomization's patches and postBuild apply only in its own build. On a walked tree no bundle's
Kustomization is one any more: the root node's directory renders no bundle (go-kure/kure#979;
before, the root node's bundle rendered there, and its patches and postBuild met this check). What
is left is the layout Kustomization of the root node's directory below a `ClusterName` wrapper
under `FluxIntegratedPerLayout`, which carries patches or postBuild only when it is the caller's
own, kept (below), and a tree built by hand. A Kustomization whose build holds
the root node's layout is refused when one of its patches applies to a Source the
integration hosts there — a target that selects it the way kustomize selects one, or a target-less
strategic-merge patch whose body names its apiVersion, kind, name and effective namespace — or when
its postBuild substitution changes one: Flux's own substitution, run offline with the inline
`substitute` vars. When `substituteFrom` is set, whose values are only in the cluster, a `${...}`
expression that reads a var the inline vars do not set is refused whatever the offline result; one
that reads only inline vars, which override `substituteFrom`'s, is decided by it. Otherwise the two
would apply the Source differently and keep
overwriting each other. The error names the Kustomization, the Source and the patch index or
postBuild; narrow the patch target so that it leaves the Source out, remove the patch or postBuild
from that Kustomization, or drop the `${...}` from the `SourceRef` URL (the remedies are worded
for the Kustomization that can still meet the check; before go-kure/kure#979 they told a root
bundle to move the patch to a bundle below the root node). A patch that selects a hosted Source but leaves it
unchanged is refused too. A copy the integration did not add, anywhere in the root build (see
below), is its owner's: the integration hosts none of its own then and does not check what a
Kustomization's patches do to it. A `sourceRef` names the object, not the layout holding it.

The root build also covers the child directories the root's `kustomization.yaml` lists and the
`AppFileSingle` files written into them. When that build already holds a copy the integration did
not add (an earlier integration's, the caller's or an application's), that copy is kept and the
integration adds none, since kustomize refuses one object twice. Two copies the integration did
not add, in any one build (the root's, a `ClusterName` wrapper's or a generated Kustomization's
`spec.path`), are refused. So is one such copy in a wrapper whose `kustomization.yaml` lists the
root node's directory: the root's copy cannot stay beside it, and without the root's copy no build
the bootstrap applies holds the Source. A copy the caller or an application puts in any other
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
that build holds the root node's layout, its patches and postBuild are checked against the Sources
hosted there, and its `sourceRef` may not name one of them (an omitted `sourceRef` namespace is
the Kustomization's own): the integration is refused, naming it, since it would take its source
from inside what it applies; and its `dependsOn`, `wait` and health checks enter the reconcile-order check (see
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
  such an object: it is written as it is, so it has to carry the annotation already;
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
  `GenerateFromLayout` does not refuse: its caller holds the layout, and the list it returns
  carries no intent by itself. The intent is applied by `IntegrateWithLayout` or
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

objects, err := engine.GenerateBootstrap(bootstrapConfig, rootNode)
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

Both modes point Flux at the same directory for the same root node (go-kure/kure#979): the one
named after the root node, relative to the root of the source, which is where a walk without a
`ClusterName` writes a named root node. Each mode spells it the way its field requires:

| Root node | `"gotk"`: bootstrap Kustomization `spec.path` | `"flux-operator"`: `FluxInstance` `spec.sync.path` |
|---|---|---|
| named `prod` | `prod` | `./prod` |
| unnamed, or none passed | `.` | `./` |

`spec.path` is spelled like every other Kustomization path this package writes (see
[Kustomization paths](#kustomization-paths)): no leading `./`, and `.` for the root of the source.
`spec.sync.path` is `DefaultSyncPath` followed by the directory. Flux treats `x` and `./x` alike.
`"gotk"` mode used to write `manifests/<root>`, a prefix none of the layout writers produces; a
consumer that depends on such a prefix sets `spec.path` on the returned Kustomization.

The bootstrap is given the root node, not the layout rules, and its path does not follow them, so
the walked root can be somewhere else. A walk with a `ClusterName` writes the root in or under
the cluster directory (`<ClusterName>/<root>`; the cluster directory itself for an unnamed root,
or when its last segment is the root's name). A walk without one writes an unnamed root node to
`cluster`, not to the root of the source. The table under
[Kustomization paths](#kustomization-paths) has examples.

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

fi, err := engine.GetBootstrapGenerator().GenerateFluxInstance(bootstrapConfig, rootNode)
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

- `FluxSeparate` - Flux resources collected in a separate `flux-system/` directory inside the root layout's own directory (where the root's `kustomization.yaml` references it); children referenced as directories, except those that render bundles, which their own CRs apply. `WriteToDisk` and `WriteToTar` name its files by `LayoutRules.FileNaming`, like the rest of the tree: `flux-system-kustomization-<name>.yaml` by default, `kustomization-<name>.yaml` with `FileNamingKindName` (go-kure/kure#976; before, always the default pattern). Rules passed to `IntegrateWithLayout` that leave `FileNaming` unset take the root layout's. The directory must be free: a root node's bundle or a child node rendered to it (named `flux-system` in whatever case, or a name that resolves to it such as `./flux-system`; `ManifestLayout.SameDirectory` in the layout package) is refused, naming the bundle or node. Rename it, or use an integrated placement. Since go-kure/kure#979 the root node's bundle has a directory named after it there; before, such a bundle was written into the root node's directory.
- `FluxIntegratedPerLayout` - a Flux Kustomization CR for every layout that renders bundles (bundles a `GroupFlat` merge puts in one directory share one CR, named after the first) and for every child layout that is not an umbrella child, not `AppFileSingle` and renders no bundle (augmenter-added child layouts included), hosted in its parent layout (the top of the tree `layout.WalkCluster` returns renders no bundle and gets no CR, go-kure/kure#979); the parent's `kustomization.yaml` lists those CR files as its own resources and references no child directory. Not literally every layout: a layout whose CR name another generated CR already uses, such as an augmenter application named like its bundle, is refused instead (see [Non-Bundle Child Layout CRs](#non-bundle-child-layout-crs)). A Kustomization already in the tree with that name, in the namespace of the generated CRs, is kept in place of a generated one when it sits in the layout that would host it and has the same `spec.path`, and is refused otherwise. Finest granularity.
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

In `FluxIntegratedPerLayout` mode every child layout that is not an umbrella child, not `AppFileSingle` and renders no bundle gets a `Kustomization` CR in its parent's `Resources`, with `spec.path` set to `child.FullRepoPath()`. (A child that renders a bundle already has that bundle's CR there.) This covers:

- **Application layouts** — per-app layouts (`ApplicationGrouping: GroupByName`, or augmenter apps, which keep a directory under `GroupFlat`). The CR is named after the layout.
- **Augmenter sub-layouts** — hook-group child layouts added by a `LayoutAugmenter` are children of an app layout. `spec.dependsOn` is populated from `ManifestLayout.DependsOn`, enabling ordered reconciliation between hook groups.
- **Bundle-less node layouts** — a GroupByName node layout above its bundle layout, or a node without a bundle. The CR is named `<path with "/" replaced by "-">-node` (with `ClusterName: "."`, node `web`'s path is `web`, which is also its bundle's CR name when the bundle sets no `KustomizationName`).

The integrator applies this rule at any depth. The CR's `spec.sourceRef` is the `SourceRef` of the nearest layout at or above the host that renders bundles; with `BundleGrouping: GroupFlat` the root node's layout renders none and counts with the `SourceRef` of its own bundle, rendered in the directory inside it (`OriginUnit`, go-kure/kure#979), so a bundle-less child node of the root still takes the root bundle's source (with `GroupByName` no node's layout renders a bundle, the root's included, and none counts); with none, the one `SourceRef` the URL-less bundles below the child share. The CR of the root node's layout (below a `ClusterName` wrapper; on a tree built by hand, that of any layout above it too) takes no Source the integration generates, whichever of these would give it one (see [Layout Integration](#layout-integration): no Kustomization takes its source from inside what it applies). A missing, incomplete or ambiguous source is a hard error — a `Kustomization` without a valid `spec.sourceRef` is rejected by Flux and must not be emitted silently — and so is a layout whose bundles have different `SourceRef`s (a `NodeGrouping: GroupFlat` merge) hosting a layout CR. A CR name used twice (a layout named like a bundle's Kustomization, say, which without a `KustomizationName` is the bundle's own name) is an error, not a silent skip: Flux Kustomizations share one namespace.

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

## Related Packages

- [stack](/api-reference/stack/) - Core domain model
- [stack/layout](/api-reference/layout/) - Manifest directory organization
- [kubernetes/fluxcd](/api-reference/fluxcd-builders/) - Low-level Flux resource builders
