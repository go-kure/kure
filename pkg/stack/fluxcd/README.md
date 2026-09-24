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

// Generate all Flux resources for a cluster (paths of a default-rules walk)
objects, err := engine.GenerateFromCluster(cluster)

// Placement is set on the LayoutRules passed to the layout call,
// not on the engine — see Layout Integration below.
```

## Engine Construction

```go
// Default engine
engine := fluxcd.Engine()

// The same, built from its components
engine := fluxcd.NewWorkflowEngine()
```

The engine has no path mode: every Kustomization `spec.path` is a layout directory (see
[Kustomization paths](#kustomization-paths)).

Placement (FluxIntegratedPerLayout vs FluxSeparate) is configured per call on
`layout.LayoutRules.FluxPlacement`. `FluxUnset` is normalized to
`FluxSeparate` by `LayoutIntegrator.CreateLayoutWithResources` — matching
`layout.DefaultLayoutRules()` and the walker. The call's placement is the
tree's: `IntegrateWithLayout` sets it on every layout it integrates,
whatever placement the tree was walked with. See
[Layout Integration](#layout-integration).

## Resource Generation

Generate Flux resources from a cluster, from a layout walked from it, or for one bundle:

```go
// From an entire cluster: walks it with layout.DefaultLayoutRules()
objects, err := engine.GenerateFromCluster(cluster)

// From a layout you walked (and will write) yourself
ml, err := layout.WalkCluster(cluster, rules)
objects, err = engine.ResourceGen.GenerateFromLayout(ml, cluster)

// For one bundle, at a path you supply
objects, err = engine.ResourceGen.GenerateForBundle(bundle, "clusters/prod/apps")
```

Each directory that renders bundles produces one Flux Kustomization resource (see
[One Kustomization per directory](#one-kustomization-per-directory)) with:
- `spec.path` = that directory
- Source reference from `Bundle.SourceRef` (and a Source when it has a `URL`)
- Dependency ordering from `Bundle.DependsOn` and `Bundle.NamedDependsOn`
- Interval and pruning configuration

`GenerateFromCluster` walks the cluster itself, so it renders every application and runs every
`LayoutAugmenter`: their errors surface there. Its paths are the directories `WalkCluster` writes
under the default rules — the root node at `<root>`, its children at `<root>/<child>` — which is
also what the bootstrap sync path `./<root>` expects. A caller that writes the layout with other
rules generates from that layout instead: `CreateLayoutWithResources`, or `GenerateFromLayout` on
its own `WalkCluster` result.

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
| node `platform` (bundle `platform`) with child node `apps` (bundle `apps-bundle`) | nodeOnly, `ClusterName: "prod"` | `platform`, `apps-bundle` | `prod/platform`, `prod/platform/apps` |
| unnamed root node, bundle `web` | nodeOnly, `ClusterName: "."` | `web` | `.` |

The path is emitted as `FullRepoPath()` returns it (no `./` prefix; Flux treats `x` and `./x`
alike) and is relative to the root of what the writer wrote: the `WriteToDisk` / `WriteToTar`
base, or `<basePath>/<ManifestsDir>` for `layout.WriteManifest`. Root the Flux source there.

### One Kustomization per directory

A directory is what a Flux Kustomization applies, so kure emits exactly one per directory that
renders bundles. When a `GroupFlat` axis (or `FlattenSingleTier`) merges several bundles into one
directory, they share that Kustomization, named after the first of them (the absorbing node's own
bundle when it has one):

| Bundle setting | In the shared Kustomization |
|---|---|
| `SourceRef`, `Interval`, `Timeout`, `RetryInterval`, `Prune`, `Wait`, `Force`, `Suspend`, `PostBuild` | must be the same for every merged bundle (unset compares as the default; an omitted `SourceRef` namespace is the generator's `DefaultNamespace`), else an error naming the setting and the bundles |
| `HealthChecks`, umbrella health checks | combined, each listed once; a check on a Flux Kustomization names the unit that applies that bundle, and one on the unit itself is dropped |
| `Labels`, `Annotations` | combined; one key with two values is an error |
| `Patches` | combined, but only when every patch has a `Target`: an untargeted patch would reach the other bundles' objects |
| `DependsOn` | mapped to the Kustomization that applies each dependency; dependencies between the merged bundles are dropped |
| `NamedDependsOn` | combined |

`GenerateFromLayout` and the integrator refuse a set of Kustomizations kure generates that can
never all become Ready. Applying waits for every `dependsOn` to be Ready; becoming Ready waits for
every Kustomization it health-checks, unless `wait` is set (Flux then ignores health checks and
waits for everything it applied, the CRs it created included); and, under integrated placement, a
CR exists only once the Kustomization whose directory references reach its file has applied — a CR
the root directory reaches is created by the Flux bootstrap. Identities are namespace and name. A
cycle among these waits is refused, naming the chain. Kustomizations an application emits itself,
and objects in other namespaces, are outside this check. A merge can close one (a health
check or dependency on a bundle merged into a unit that waits for it), and so can a parent node's
bundle depending on a child node's bundle whose CR only the parent's directory holds (PerLayout). ArgoCD
Applications get the unit rule but not this check: only a `DependsOn` cycle between units is
refused there. Give bundles directories of their own (`NodeGrouping` or
`BundleGrouping` `GroupByName`) when they need different settings.

The generator computes no path, the integrator matches nothing by name, and `FlattenSingleTier`
rewrites nothing afterwards: when it collapses a tier, the surviving layout takes over the
collapsed layout's origins, so when both carried a bundle they share the surviving directory's one
Kustomization. An umbrella
child's path is its own directory, in every mode (the removed `KustomizationRecursive` rule
pointed it at the parent bundle's directory, whose kustomization excludes the child).

`IndexOrigins` refuses a tree it cannot resolve unambiguously: a hand-built, partial or
other-cluster tree (the rendered set is not what the cluster reaches), a bundle or node rendered
twice, two bundles with one name (the name is the Kustomization's identity), and a node or bundle
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
| `DefaultBootstrapPathRoot` | `manifests` | building the bootstrap Kustomization's `spec.path` | not overrideable; the root node's name is joined onto it |
| `DefaultFluxDirName` | `flux-system` | a separate Flux layout needs a directory | not overrideable |
| `DefaultSourceRef` | `latest` | an OCI source, or an OCI `FluxInstance` sync, has no `SourceRef` | setting `BootstrapConfig.SourceRef` |
| `DefaultSyncPath` | `./` | the root node has no name | not overrideable; it is the prefix a sync path is built from |

Three of these — `DefaultInterval`, `DefaultNamespace` and `DefaultBootstrapName` —
are copied into exported generator fields by `NewResourceGenerator` / `NewBootstrapGenerator`, and
a field assigned afterwards is never overridden. The rest are applied where they are used and are
overridden by naming the corresponding input, as the last column says. Three defaults have no
override at all and say so, rather than being listed as though they had one; `FluxInstanceName`
is the fourth row without one, and is not a default at all (next but one paragraph).

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

// Write to disk: every spec.path is relative to ./out/clusters
err = layout.WriteManifest("./out", layout.DefaultLayoutConfig(), ml.(*layout.ManifestLayout))
```

`IntegrateWithLayout(ml, cluster, rules)` does the same for a layout you walked yourself. It
refuses a tree `layout.WalkCluster` did not build from that cluster (see
[Kustomization paths](#kustomization-paths)). Integrating the same layout twice adds nothing: a
CR already present with the same name and `spec.path` is kept, the same name with another path is
an error, and under `FluxSeparate` an identical `flux-system` child — same directory, same
resources, nothing beneath it — is kept rather than a second one appended; any other is refused. Every Flux Kustomization already in the tree counts — typed or unstructured, placed
by an earlier integration, by the caller or emitted by an application: an identity
(namespace/name) present twice, or taken by a generated CR elsewhere, is refused in every
placement, since the kustomize build would register the id twice.

## Bootstrap Generation

Generate Flux system bootstrap manifests. Two modes are supported:

| Mode | Description |
|------|-------------|
| `"flux-operator"` | **Default.** Emits a full Flux Operator install bundle (CRDs, Deployment, RBAC). Recommended for new clusters. |
| `"gotk"` | Legacy mode. Emits the GitOps Toolkit component manifests directly. |

When `FluxMode` is empty, it defaults to `"flux-operator"`.

The `"flux-operator"` bundle is vendored from the upstream flux-operator release and pinned
in lockstep with the `github.com/controlplaneio-fluxcd/flux-operator` Go module
(`FluxOperatorVersion`, currently **v0.58.1**). Renovate re-vendors the bundle and updates
the constant and this version when it bumps the module (`scripts/sync-flux-operator-pin.sh`);
see `flux_operator_install.go` for the manual refresh procedure.

The `"gotk"` components are built from the flux2 release's install manifests base, vendored
(`gotk_manifests.tar.gz`) in lockstep with the `github.com/fluxcd/flux2/v2` Go module
(`GotkVersion`, currently **v2.9.5**), so gotk generation makes no network call and the same
input always produces the same manifests. That happens when `FluxVersion` is empty or names
`GotkVersion` (with or without the leading `v`). Any other `FluxVersion`, `"latest"` included,
is an explicit opt-in to the upstream behaviour: a manifests base is downloaded from GitHub at
generation time — the named release for a `vX.Y.Z` value, the latest release for anything else
(upstream selects a release only for a `v`-prefixed version). `TestVendoredPinsMatchGoMod` fails when
`GotkVersion` or `FluxOperatorVersion` differs from its `go.mod` require, or when a controller
image in the gotk bundle differs from the matching `fluxcd/<controller>/api` require (controllers
without an API module, such as image-reflector-controller, are not compared); see
`gotk_install.go` for the refresh procedure after a flux2 bump.

```go
bootstrapConfig := &stack.BootstrapConfig{
    Enabled:     true,
    FluxMode:    "flux-operator", // or "gotk"; empty defaults to "flux-operator"
    FluxVersion: "v2.8.2",
    SourceRef:   sourceRef,
}

objects, err := engine.GenerateBootstrap(bootstrapConfig, rootNode)
```

### Sync name

`BootstrapConfig.SyncName` becomes the `FluxInstance`'s `spec.sync.name`: the name flux-operator
gives the source and Kustomization it creates for the sync. When it is empty the operator names
both after the `FluxInstance`'s namespace (`DefaultNamespace`). That fallback is the operator's,
not kure's, which is why the defaults table above has no row for it. Set `SyncName` when the
Kustomizations you generate reference the sync source by another name; otherwise their
`sourceRef` points at a source nothing creates.

```go
bootstrapConfig := &stack.BootstrapConfig{
    Enabled:   true,
    SourceURL: "oci://registry.example.com/fleet",
    SourceRef: "latest",
    SyncName:  "fleet",
}
```

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
- `KustomizationRecursive` - References subdirectories only

### Flux Placement

Controls where Flux Kustomization resources are placed:

- `FluxSeparate` - Flux resources collected in a separate `flux-system/` directory inside the root layout's own directory (where the root's `kustomization.yaml` references it); children referenced as directories
- `FluxIntegratedPerLayout` - a Flux Kustomization CR for **every** layout (incl. augmenter-added child layouts), hosted in its parent layout; the parent's `kustomization.yaml` lists those CR files as its own resources and references no child directory. Finest granularity.
- `FluxIntegratedPerBundle` - Flux Kustomization CRs at **bundle/node boundaries only**; a bundle's interior (incl. augmenter-added child layouts) is a single kustomize build, with children referenced as directories. Coarser: Flux reconciles per bundle, kustomize handles the interior.

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
  own Kustomization by name/namespace)
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
of its own.

### Placement in layouts

`LayoutIntegrator` places umbrella child Flux CRs at the **parent** layout
node, with `spec.path` = the child's own directory:

- **Integrated, non-nodeOnly**: the walker creates a bundle sub-layout
  under the node layout. Umbrella child Kustomization CRs (and their Source
  CRs, if the child has a `SourceRef.URL`) are appended to the bundle
  sub-layout's `Resources`, which that layout lists (it is written in
  `KustomizationExplicit` mode). Nested umbrella children are placed at their
  enclosing umbrella child's layout node.
- **Integrated, nodeOnly (GroupFlat)**: there is no intermediate bundle
  layer, so umbrella children become direct sub-layouts of the node layout,
  and their Flux CRs sit at the node layout.
- **FluxSeparate**: the `flux-system` layout directory receives every bundle's
  Kustomization CR, umbrella descendants included, as a flat list.

Under `FluxIntegratedPerLayout` a node bundle's own CR follows the same rule
as every other PerLayout CR: it is hosted by the **parent** of the layout that
renders the bundle (the root layout hosts its own). Under
`FluxIntegratedPerBundle` it stays at the node's layout.

### On-disk shape

When a parent layout has an umbrella child, the parent's `kustomization.yaml`
lists the child's Kustomization CR file (it is one of the parent's own
resources) instead of the child subdirectory. The child subdirectory still exists and still contains its own
`kustomization.yaml` plus workload YAML files — but **no** Flux CR files, so
Flux does not double-apply the child's resources.

## Non-Bundle Child Layout CRs

In `FluxIntegratedPerLayout` mode every child layout that is not an umbrella child, not `AppFileSingle` and renders no bundle gets a `Kustomization` CR in its parent's `Resources`, with `spec.path` set to `child.FullRepoPath()`. (A child that renders a bundle already has that bundle's CR there.) This covers:

- **Application layouts** — per-app layouts (GroupByName, or augmenter apps in nodeOnly mode). The CR is named after the layout.
- **Augmenter sub-layouts** — hook-group child layouts added by a `LayoutAugmenter` are children of an app layout. `spec.dependsOn` is populated from `ManifestLayout.DependsOn`, enabling ordered reconciliation between hook groups.
- **Bundle-less node layouts** — a GroupByName node layout above its bundle layout, or a node without a bundle. The CR is named `<path with "/" replaced by "-">-node` (with `ClusterName: "."`, node `web`'s path is `web`, which is also its bundle's CR name).

The integrator applies this rule at any depth. The CR's `spec.sourceRef` is the `SourceRef` of the nearest layout at or above the host that renders bundles; with none, the one `SourceRef` the URL-less bundles below the child share. A missing, incomplete or ambiguous source is a hard error — a `Kustomization` without a valid `spec.sourceRef` is rejected by Flux and must not be emitted silently — and so is a layout whose bundles have different `SourceRef`s (a `NodeGrouping: GroupFlat` merge) hosting a layout CR. A CR name used twice (a layout named like a bundle, say) is an error, not a silent skip: Flux Kustomizations share one namespace.

## Validation

All cluster-level entry points (`GenerateFromCluster`, `CreateLayoutWithResources`)
call `stack.ValidateCluster` before walking the tree, and every generation from a
layout runs `layout.IndexOrigins` (see [Kustomization paths](#kustomization-paths)). Invalid umbrella
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
