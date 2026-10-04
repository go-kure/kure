# Layout and Flux naming: current behaviour and target

This page records how kure names, places and wires the directories and Flux objects it generates
from the stack model, and what each planned change makes of it. Part 1 is the behaviour of
`v0.2.0-beta.15`, confirmed by rendering small clusters to disk and to tar. Part 2 is the target,
one section per planned change, each tracked by an issue.

kure, its consumers and their output are pre-release: names, paths and output may change, and
live-cluster upgrade effects are not a constraint.

## Roles

- **kure** generates YAML objects from Go APIs: base Kubernetes objects with their full spec, the
  layout of those objects in directories, and the Flux objects that deliver them. Covering every
  base kind is its responsibility; the one kind left out of scope is recorded, with the reason,
  under [go-kure/kure#981](https://github.com/go-kure/kure/issues/981).
- **An application-level consumer** turns one application into kure's model
  (`Cluster`, `Node`, `Bundle`, `Application`). It is delivery-agnostic: it puts no Flux
  Kustomizations, health checks, reconciliation settings or Flux annotations on top of an
  application. What a delivery engine needs to know (ordering, prune protection, force replace)
  travels as intent in kure's model.
- **A cluster-level consumer** assembles a whole cluster's tree and owns all delivery glue: the
  Flux Kustomizations within and between applications.

A consumer never renames, moves or deletes what kure produced. If it has to, kure lacks a parameter
or has a bug, and the fix belongs in kure.

Two rules follow from that. They are the target, not today's behaviour: sections 1.7 and 1.8 show
where they do not hold yet, and Part 2 is what makes them hold.

- Every generated name has a default and an override.
- Two objects of the same kind in the same namespace never share a name; objects of different
  kinds may.

## Part 1: current behaviour (v0.2.0-beta.15)

Code is cited by symbol name, with the file that holds it, so a citation does not move when the
code around it does. A file named without a directory is in `pkg/stack/layout` or
`pkg/stack/fluxcd`. Where a change has shipped since `v0.2.0-beta.15`, Part 1 describes the code
after it and names the issue: the FileNaming rows and item (section 1.2, section 1.9 item 4), the
empty distribution (section 1.6), the last two rows of section 1.7, and section 1.9. The Part 2
sections on go-kure/kure#974 and go-kure/kure#978, whose changes are open, keep their line
references until those ship.

### 1.1 The model

| Type | Fields that decide names and placement | Source |
|---|---|---|
| `stack.Node` | `Name`, `Children`, `Bundle` (one per node), `PackageRef`. No dependency, Kustomization or directory field. | `Node` in `pkg/stack/cluster.go` |
| `stack.Bundle` | `Name`, `DependsOn []*Bundle`, `NamedDependsOn`, `Children` (umbrella), `SourceRef`, `Interval`, `Prune`, `Wait`, `Timeout`, `RetryInterval`, `Force`, `Suspend`, `HealthChecks`, `Patches`, `PostBuild`, `Labels`, `Annotations`. No field for the Kustomization's name, namespace or directory. | `Bundle` in `pkg/stack/bundle.go` |
| `stack.Application` | `Name`, `Namespace`, `Config`. No delivery or dependency field. | `Application` in `pkg/stack/application.go` |

`layout.WalkCluster` turns the model into a `ManifestLayout` tree. `fluxcd.LayoutIntegrator` adds
Flux objects to that tree in one of three placements: `FluxSeparate` (the default),
`FluxIntegratedPerBundle` and `FluxIntegratedPerLayout`. The writers (`WriteToDisk`,
`WriteToTar`, `WriteManifest`) check the tree and write it.

### 1.2 Directories and files

| What | Rule | Source |
|---|---|---|
| Layout directory | `FullRepoPath() = Namespace/Name` | `ManifestLayout.FullRepoPath` in `pkg/stack/layout/manifest.go` |
| Cluster wrapper | `ClusterName "."`: none. `"prod"`: `prod/`. `""` with a named root: `<root>/`. `""` with an unnamed root: `cluster/`. | `WalkCluster` and `walkClusterWithClusterName` in `pkg/stack/layout/walker.go` |
| Node directory | node name, nested by tree (`NodeGrouping: GroupByName`, the default). `GroupFlat` merges descendant nodes' bundles into the first-level node's directory. | `walkNode`, `renderNodeContent` and `renderChildren` in `walker.go` |
| Bundle directory | none by default: the bundle renders into its node's directory. `BundleGrouping: GroupByName` adds `<node>/<bundle name>`. | `renderBundle` in `walker.go` |
| Umbrella child directory | `<parent dir>/<child bundle name>`, marked `UmbrellaChild` | `renderUmbrellaChildren` in `walker.go` |
| Application directory | none by default. `ApplicationGrouping: GroupByName` adds `<bundle dir>/<app name>`. Under the default flat grouping an application whose config is a `LayoutAugmenter` still gets its own directory, unless the config also implements `LayoutIntentAugmenter` and `WantsOwnLayout()` returns false; it is then merged like any other application. | `renderApps`, `wantsOwnLayout` and `isAugmenter` in `walker.go` |
| Resource file | `WriteToDisk` and `WriteToTar` name by the layout's own FileNaming: default `{namespace}-{kind}-{name}.yaml` (empty namespace: `cluster`); `FileNamingKindName` gives `{kind}-{name}.yaml`. `WriteManifest` names every layout's files from its `Config` instead. | `DefaultManifestFileName` in `pkg/stack/layout/config.go`; `groupResourceFiles` and `manifestPlan` in `writerplan.go` |
| Generated Kustomization file | under `WriteToDisk` and `WriteToTar`, named by the host layout's FileNaming. The `flux-system/` layout of `FluxSeparate` takes the rules' FileNaming (the root layout's when the rules leave it unset): `flux-system-kustomization-<name>.yaml` by default, `kustomization-<name>.yaml` with `FileNamingKindName`. A layout an augmenter adds and leaves unset takes its parent's. | `ManifestLayout.resolveManifestFileName` in `manifest.go`; `addSeparateFluxToLayout` in `pkg/stack/fluxcd/layout_integrator.go`; `inheritFileNaming`, called from `renderApps`, in `walker.go` |
| `kustomization.yaml` child entries | a child directory is listed unless it is an `UmbrellaChild`, renders a bundle, or the parent is `FluxIntegratedPerLayout` | `childEntry` in `writerplan.go` |

`LayoutRules.FlattenSingleTier` (default false) collapses one layer, and only at the walked root:
the root must have a namespace without `/`, exactly one child, no resources of its own, and that
child must be a terminal, non-umbrella layout (`flattenSingleTier` and `canFlatten` in
`pkg/stack/layout/flatten.go`, called from `WalkCluster`). It does not reach an application
directory deeper in the tree.

### 1.3 Flux Kustomizations: name, host and listing

**Names.**

- A bundle's Kustomization is named `Bundle.Name`, in the generator's `DefaultNamespace`
  (`kustomizationForBundle` in `pkg/stack/fluxcd/resource_generator.go`).
- Several bundles in one directory (a `GroupFlat` merge) share one Kustomization named after the
  first bundle (`generateForUnit` in `resource_generator.go`; `OriginIndex.UnitName` and
  `UnitOfName` in `pkg/stack/layout/origin.go`).
- Under `FluxIntegratedPerLayout`, a bundle-less node layout gets a Kustomization named
  `<path with / replaced by ->-node`, and an application or augmenter layout one named after the
  layout (`layoutCRName` in `layout_integrator.go`).

**Where each placement puts them.**

| Placement | Bundle Kustomization hosted in | Extra Kustomizations | Host's `kustomization.yaml` lists |
|---|---|---|---|
| `FluxSeparate` | `flux-system/` directly under the walked root's directory | none | `flux-system/` lists the CR files; the root lists `flux-system` |
| `FluxIntegratedPerBundle` | the parent of the bundle's directory; the walked root hosts its own (`integratedPlacement.host` in `layout_integrator.go`) | none | the CR files |
| `FluxIntegratedPerLayout` | as `FluxIntegratedPerBundle` | one per bundle-less child layout (the per-layout branch of `integratedPlacement.place`) | the CR files only, never a child directory |

Example. Unnamed root → groups `applications`, `backend` → one node per application, each with a
bundle named after it; `shop` is an umbrella with children `shop-infra` → `shop-services` (each
depending on the previous); `ClusterName "."`; `FileNamingKindName`; `FluxIntegratedPerBundle`:

```
applications/kustomization.yaml                    # [kustomization-blog.yaml, kustomization-shop.yaml]
applications/kustomization-blog.yaml               # name blog, path applications/blog
applications/kustomization-shop.yaml               # name shop, path applications/shop
applications/blog/...
applications/shop/kustomization.yaml               # [the two child CR files]
applications/shop/kustomization-shop-infra.yaml    # path applications/shop/shop-infra
applications/shop/kustomization-shop-services.yaml # dependsOn shop-infra
applications/shop/shop-infra/...
applications/shop/shop-services/...
backend/kustomization-api.yaml                     # path backend/api
backend/api/...
kustomization.yaml                                 # [applications, backend]
```

`FluxIntegratedPerLayout` adds `kustomization-applications-node.yaml` and
`kustomization-backend-node.yaml` at the root, and the root lists only those two files.
`FluxSeparate` puts all the CRs in `flux-system/`, and gives the group directories `resources: []`.

### 1.4 `spec.path`

No entry point adds a leading `./` to a Kustomization's `spec.path`. The FluxInstance `sync.path`
is the one path built with it.

| Entry point | `spec.path` | Source |
|---|---|---|
| `LayoutIntegrator` (all placements), `GenerateFromLayout` | the directory of the layout that renders the bundle: `applications/shop/shop-infra` | `generateForUnit` in `resource_generator.go` |
| `GenerateForBundle(b, path)` | `path` verbatim | `ResourceGenerator.GenerateForBundle` |
| `GenerateFromCluster(c)` | walks with `DefaultLayoutRules()`, so the consumer's layout rules, `ClusterName` included, are ignored: `cluster/applications/shop` | `ResourceGenerator.GenerateFromCluster` |
| `FluxIntegratedPerLayout` node Kustomization | the layout's directory | `createKustomizationForLayout` in `resource_generator.go` |
| Bootstrap, gotk mode | `manifests/<root>` | the `spec.path` built in `generateFluxSystemKustomization`, `pkg/stack/fluxcd/bootstrap_generator.go` |
| Bootstrap, FluxInstance `sync.path` | `./<root>`, or `./` for an unnamed root | the sync path built in `generateFluxInstance`, `bootstrap_generator.go` |

### 1.5 Bundle fields on the Kustomization

From `kustomizationForBundle` (`resource_generator.go`):

| Bundle field | Kustomization field | When unset |
|---|---|---|
| `Interval` | `spec.interval` | 60m (`DefaultInterval` in `pkg/stack/fluxcd/defaults.go`) |
| `Prune` | `spec.prune` | `false`, always written (`pruneValue` in `defaults.go`) |
| `Wait` | `spec.wait` | omitted (`waitValue` in `defaults.go`) |
| `Timeout`, `RetryInterval`, `Force`, `Suspend` | same | omitted |
| `SourceRef` | `spec.sourceRef` | namespace omitted when empty |
| `Children` (umbrella) | one `healthChecks` entry per child, in the generator's namespace; `wait` is not set | the umbrella branch of `kustomizationForBundle` |
| `HealthChecks`, `Patches`, `PostBuild` | same | omitted |
| `DependsOn`, `NamedDependsOn` | `spec.dependsOn[].name` | omitted |

Under the integrator, dependencies and health checks that name a bundle are mapped to the
Kustomization of the directory that renders it, and a reference to its own directory is dropped
(`generateForUnit`). Bundles that share a directory must agree on every reconcile setting
(`mergeIntoUnit` in `resource_generator.go`).

The umbrella health checks exist only in generator output: `pkg/stack` exports nothing that returns
them.

### 1.6 Sources, the flux-system directory and bootstrap

**Sources.**
- A Source (`OCIRepository`, `GitRepository`) is generated only when `SourceRef.URL` is set
  (`createSource` in `resource_generator.go`).
- `FluxSeparate` puts it in `flux-system/`.
- The integrated placements put it in the root node's layout (the Source branch of
  `integratedPlacement.add` in `layout_integrator.go`). When the root node renders a bundle, that
  layout is the bundle's own directory.

**The `flux-system` directory.** Its name is a constant, and under `FluxSeparate` it is placed
directly under the walked root (`addSeparateFluxToLayout` in `layout_integrator.go`).

**Bootstrap, flux-operator mode (the default).**
- Emits the embedded flux-operator install bundle (`v0.58.1`) and a `FluxInstance` named `flux`.
- `distribution.registry` and `distribution.version` are written verbatim. `v0.2.0-beta.15` wrote
  an empty value too, as an empty string; since
  [go-kure/kure#975](https://github.com/go-kure/kure/issues/975) an empty value is an error.
- `sync` is emitted only when `SourceURL` is set.

**Bootstrap, gotk mode.** Emits the vendored components, a Kustomization at
`manifests/<root>`, and a Source when `SourceURL` is set (`generateGotkBootstrap` and
`generateFluxSystemKustomization` in `bootstrap_generator.go`).

### 1.7 Collision guards

| Input | Result | Source |
|---|---|---|
| Two bundles with the same name | refused wherever the origin index is built: the integrator under every placement, `GenerateFromLayout` and the ArgoCD workflow. `WalkCluster` and the writers alone do not check it. | `IndexOrigins` in `origin.go`; callers `addIntegratedFluxToLayout`, `ResourceGenerator.GenerateFromLayout` and `WorkflowEngine.generateFromLayout` in `pkg/stack/argocd/argo.go` |
| A payload Kustomization named like its bundle, in the namespace of the generated Kustomization (the generator's `DefaultNamespace`) | refused under `FluxSeparate`. Under the two integrated placements it is refused unless it sits in the layout that hosts the generated Kustomization and has the same `spec.path`: that one is kept as it is, and none is generated for the bundle. A bundle rendered in the walked root layout is its own host, so its payload can meet this. The check compares namespace and name, so it does not refuse the same name in another namespace. | `integratedPlacement.host`, `integratedPlacement.add`, `crKey` and `addSeparateFluxToLayout` in `layout_integrator.go`; `kustomizationForBundle` |
| A Kustomization name generated twice in one integration | refused by the integrator. The key is the bare name: every generated Kustomization is in `DefaultNamespace`. | `integratedPlacement.claim` in `layout_integrator.go`; `kustomizationForBundle` and `createKustomizationForLayout` |
| `FluxIntegratedPerLayout` with `ApplicationGrouping: GroupByName`, application named like its bundle (the common case) | refused as a name used twice | same |
| `FluxIntegratedPerLayout`, augmenter application named like its bundle | refused as a name used twice | same |
| Two directory layouts resolving to one directory | refused by the writers | `checkLayoutTree` in `pkg/stack/layout/treecheck.go` |
| Two single-file layouts (`AppFileSingle`) in one directory | accepted when their file names differ; refused when they resolve to the same file | `checkLayoutTree` |
| Dotted names, names over 63 characters | accepted unchanged | none |
| A hand-built tree with the same Kustomization twice in one layout, or in layouts one kustomize build includes | refused by the writers, as for any object held twice | `checkResourceIdentities` (one layout) and `checkBuildIdentities` (one build), both called from `checkLayoutTree` |
| A hand-built tree with the same Kustomization in layouts that separate builds apply | refused by the writers, in any tree: Kustomization namespace/name is unique across the tree (go-kure/kure#977) | `checkKustomizationNames` in `treecheck.go` |
| An `UmbrellaChild` layout, or any other directory its parent does not list, that no Kustomization applies | refused by the writers when the root is marked `SetFluxBuild` (go-kure/kure#977); written, and listed by nobody, in an unmarked tree | `checkUnappliedLayouts` in `treecheck.go` |

### 1.8 What a consumer cannot control today

1. **The name of a bundle's Kustomization.** Only by naming the bundle; a merged directory takes its
   first bundle's name; `-node` names have no parameter.
2. **The directory that hosts it.** The only lever is the placement.
3. **A directory name separate from the Kustomization name.** An umbrella child's directory and its
   Kustomization are both the child bundle's name (`renderUmbrellaChildren`,
   `kustomizationForBundle`).
   Renaming the walked layout before `IntegrateWithLayout` separates them and is accepted, but no
   document or test covers that route.
4. **Ordering between groups, from the model.** A group gets a Kustomization only under
   `FluxIntegratedPerLayout`, with a fixed name, and `Node` has no dependency field. The one route
   to a `dependsOn` is on the layout, not the model: set `DependsOn` (Kustomization names, as
   strings) on the walked group layout before `IntegrateWithLayout`, which copies it
   (the per-layout branch of `integratedPlacement.place`, `createKustomizationForLayout`).
5. **Engine annotations per application.** There is no field for prune protection or force replace,
   so a consumer writes Flux annotations onto objects itself.

What a consumer can do today:
- **Ordering inside an application** is expressible, as umbrella children with `DependsOn`.
- **A layout without Kustomizations** is available through `WalkCluster` and the writers. No parent
  then lists a bundle directory, so nothing applies the payload.

### 1.9 Documentation that disagreed with the code

All six are corrected: item 4 by go-kure/kure#976, the others by go-kure/kure#980. Each item says
what the text said and what it says now.

1. **Stale line references.** The `normalizeRulesPlacement` comment cited lines of
   `pkg/stack/layout/types.go` and `walker.go` that were not the code it named. It now names
   `layout.DefaultLayoutRules` and `layout.WalkCluster`.
2. **SourceRef message.** It said "FluxIntegratedPerLayout mode requires a SourceRef" when
   `FluxIntegratedPerBundle` triggers it too (`validateBundleSourceRefs` in
   `pkg/stack/fluxcd/validate.go`, reached for both placements from
   `LayoutIntegrator.CreateLayoutWithResources`). It now names the placement in use. The message
   of `integratedPlacement.layoutSource` keeps the per-layout wording: only that placement
   reaches it.
3. **The "every layout" claim.** The fluxcd README said `FluxIntegratedPerLayout` gives a
   Kustomization to every layout, augmenter layouts included. It now says which layouts get one,
   and that an augmenter application named like its bundle is refused instead.
4. **FileNaming.** `LayoutRules.FileNaming` was documented as the naming for manifest files
   while, under `WriteToDisk` and `WriteToTar`, it did not reach `flux-system/` or augmenter
   layouts. It now does, and the field documentation says so.
5. **Helm hook groups.** The helm README said each hook group "becomes one FluxCD Kustomization,
   deployed in order". Nothing in kure converts a `HookGroup`; the README and the package's
   comments now say that building a Kustomization from a group is the caller's.
6. **`ManifestLayout.DependsOn`.** It was documented as becoming `spec.dependsOn` in
   `FluxIntegratedPerLayout` mode. Left unsaid then and stated now: that holds only for a layout
   that gets its own Kustomization, and every other case drops the field without an error.

## Part 2: target behaviour

Each section names the change, the target, a design outline and the acceptance criteria. The
linked issue tracks the implementation. A section whose change has shipped is marked **Shipped**
and describes what the code does now.

### Kustomization name separate from the bundle name ([go-kure/kure#971](https://github.com/go-kure/kure/issues/971))

**Target.** A consumer can name a bundle's Kustomization without renaming the bundle.

**Design outline.**

- **New field:** `Bundle.KustomizationName string`; empty means `Bundle.Name`. The same name is the
  identity of the ArgoCD Application generated for that directory (`IndexOrigins` treats them as
  one identity), so the ticket settles whether the field gets an engine-neutral name.
- **Every place that now reads `Bundle.Name` as a Kustomization name** reads the effective name:
  - `kustomizationForBundle` (`resource_generator.go`);
  - the umbrella health checks, which name each child (the umbrella branch of
    `kustomizationForBundle`);
  - unit naming (`OriginIndex.UnitName` and `UnitOfName` in `origin.go`);
  - dependency translation (`OriginIndex.UnitDependencies`; `generateForUnit` and the
    `DependsOn` loops of `kustomizationForBundle`);
  - the ArgoCD Application name (`applicationForBundle` in `pkg/stack/argocd/argo.go`).
- **Name lookups:** the origin index keys its by-name map on the effective name, so
  `NamedDependsOn` and health checks name Kustomizations, never bundles.
- **Uniqueness:** checked on the effective name (`IndexOrigins`). `Bundle.Name` stays unique as
  well, because a copied bundle is resolved by name.

**Acceptance.**
- A payload Kustomization named like the bundle is accepted when `KustomizationName` differs.
- Two bundles with different names but the same effective name are refused, naming both.
- An umbrella's health checks and its children's `dependsOn` use the children's effective names.

### Directory name separate from the Kustomization name ([go-kure/kure#972](https://github.com/go-kure/kure/issues/972))

**Target.** A bundle that gets its own directory can name that directory independently of its
Kustomization (for example `00-infra` for the Kustomization `shop-infra`).

**Design outline.**

- **New field:** `Bundle.DirName string`; empty means `Bundle.Name`.
- **Where it applies:** where a bundle has its own directory, the umbrella child layout
  (`renderUmbrellaChildren`) and the `BundleGrouping: GroupByName` bundle layout
  (`renderBundle`). A
  bundle rendered into its node's directory has no directory of its own; that directory is
  `Node.Name`.
- **Unchanged:** `spec.path` keeps following `FullRepoPath`, and the duplicate-directory check
  stays. Its message names the two bundles; today it prints the two layouts' paths, which can be
  the same path twice (`checkLayoutTree` in `treecheck.go`).
- **The rename route closes:** `IndexOrigins` refuses a layout that is a bundle's own directory
  (an umbrella child, or a `GroupByName` bundle layout) when its name differs from the bundle's
  directory name. A node layout that renders a bundle keeps `Node.Name`, whatever the bundle is
  called, and is not refused.

**Acceptance.**
- The umbrella child directory is `DirName` and its Kustomization is the effective Kustomization
  name.
- Two siblings with the same `DirName` are refused, naming both bundles.
- A walked umbrella child or `GroupByName` bundle layout renamed before integration is refused.

### Ordering and naming for node-level Kustomizations ([go-kure/kure#973](https://github.com/go-kure/kure/issues/973))

**Target.** A group (a node without a bundle) can have a named Kustomization with dependencies.

**Design outline.**

- **New fields,** mirroring `Bundle`: `Node.KustomizationName`, `Node.DependsOn []*Node` and
  `Node.NamedDependsOn []string`.
- **Where they are used:** they feed `createKustomizationForLayout`
  (`resource_generator.go`). `KustomizationName` replaces the fixed `<path>-node` name
  (`layoutCRName` in `layout_integrator.go`) when set.
- **Placements:** node-level Kustomizations exist only under `FluxIntegratedPerLayout`
  (the per-layout branch of `integratedPlacement.place`), and only for a node whose layout
  renders no bundle (the skip condition at the top of that branch).
  Under the other placements, on a node that `NodeGrouping: GroupFlat` merges away, and on a node
  whose layout renders a bundle, setting these fields is refused, not silently ignored.
- **Unset fields change nothing for nodes:** a node Kustomization keeps its `<path>-node` name.
- **Collisions:** a node `KustomizationName` equal to a bundle's effective Kustomization name is
  refused, as any name used twice is today (`integratedPlacement.claim`).
- **Reconciliation settings:** a node Kustomization takes the generator's interval and prune and
  sets no `wait` today (`createKustomizationForLayout`). The ticket decides whether a node
  carries its own.
- **Application and augmenter layouts, a breaking rename:** under `FluxIntegratedPerLayout` they
  get Kustomizations named after the layout (`layoutCRName`). These collide with
  the bundle's whenever the application shares the bundle's name (section 1.7). The default
  becomes `<unit name>-<layout name>`, which renames these Kustomizations even when no new field
  is set. Changing with it:
  - sibling `dependsOn` entries, which name layouts (`ManifestLayout.DependsOn`, copied in
    `createKustomizationForLayout`), are translated to the new names;
  - the test that pins the old names (`TestAugmenterChildrenGetFluxCRs` in
    `pkg/stack/fluxcd/layout_integrator_test.go`) and the README sentence "The CR is named after
    the layout" (`pkg/stack/fluxcd/README.md`, "Non-Bundle Child Layout CRs").
- **Override for the new default:** a new field, proposed `ManifestLayout.KustomizationName`,
  separate from the layout's `Name`; the ticket settles the name. An augmenter sets it for the
  layouts it creates, and a consumer sets it on a walked layout before integration. A
  default longer than the limit of
  [go-kure/kure#978](https://github.com/go-kure/kure/issues/978) is refused with a message naming
  the layout and the field to set; kure does not shorten it.

**Acceptance.**
- A node with `KustomizationName` and `NamedDependsOn` renders a Kustomization with that name and
  those dependencies.
- Each of the three refusals has a test and an error naming the node: the fields set under a
  placement other than `FluxIntegratedPerLayout`, on a node that `NodeGrouping: GroupFlat` merges
  away, and on a node whose layout renders a bundle.
- `FluxIntegratedPerLayout` with an application named like its bundle is accepted.
- An augmenter layout's `dependsOn` names its sibling's new Kustomization name.
- A layout with the name set renders its Kustomization under that name; a default over the limit
  is refused, naming the layout and the field.

### Delivery intent on applications ([go-kure/kure#974](https://github.com/go-kure/kure/issues/974))

**Target.** An application can ask for prune protection or force replace without writing a
delivery engine's annotations. The Flux workflow turns that intent into the Flux annotations.

**Design outline.**

- **New field:** `stack.Application` gains an engine-neutral intent, `Delivery DeliveryIntent`,
  with `PruneProtection` and `ForceReplace` booleans.
- **Per-application attribution:** today objects are attributed per bundle, not per application:
  `origin.objects` is keyed by bundle (`origin.go:17-29`), and only an application with its own
  layout is recorded. The walker therefore records each application's objects (it has both in
  `renderApps`, `walker.go:386-422`). The layout package stays engine-neutral.
- **The Flux mapping:** `fluxcd.LayoutIntegrator` sets `kustomize.toolkit.fluxcd.io/prune: disabled`
  and `kustomize.toolkit.fluxcd.io/force: enabled` on those objects.
- **Refusal leaves no annotation behind:** a refused integration restores the objects. The
  existing restore does not do that by itself: `saveLayouts` (`layout_integrator.go:111-138`)
  clones the slices but keeps the same object pointers, so an annotation set in place would
  survive. The integrator therefore snapshots the annotations it changes, or annotates copies.
- **Conflicts:** an object that already carries the annotation with another value is refused.
- **Generated ConfigMaps:** a ConfigMap built by a `configMapGenerator` has no object to annotate.
  Its spec is a name and a file list (`manifest.go:84-87`), and the walker holds only a stand-in
  for it (`walker.go:416-418`). The ticket decides between writing the generator's
  `options.annotations` in `kustomization.yaml` and documenting the exception.
- **ArgoCD:** its mapping is out of scope. Until it exists, the ArgoCD workflow refuses a set
  intent instead of dropping it.

**Acceptance.**
- Every object of an application with prune protection carries the prune annotation under every
  Flux placement and grouping, and no other application's objects do.
- The same holds for force replace.
- A conflicting existing annotation is refused.
- After a refused integration no object carries an annotation the integrator added, with a test
  for that failure path.
- A generated ConfigMap behaves as the ticket decides, with a test for it.

### Bootstrap refuses an empty distribution ([go-kure/kure#975](https://github.com/go-kure/kure/issues/975))

**Shipped.** flux-operator mode no longer writes `registry: ""` or `version: ""`.

**What it does.**

- Building a FluxInstance returns a validation error when `BootstrapConfig.FluxVersion` or
  `Registry` is empty. The error names each missing field; when both are empty, one error names
  both.
- Both entry points refuse: `GenerateBootstrap` in flux-operator mode, which is also the mode when
  `FluxMode` is empty, and the public `GenerateFluxInstance`. Both build the FluxInstance in
  `generateFluxInstance`, which checks before it builds.
- A refused `GenerateBootstrap` returns no objects: the install bundle is not returned without its
  FluxInstance.
- No default is filled in. The caller supplies both values, and set values still render verbatim.
- gotk mode is unchanged. It reads both fields too, but an empty value is valid there: an empty
  `FluxVersion` builds from the vendored version without a download, and an empty `Registry` keeps
  the default.

**Breaking.** A caller that relied on the empty distribution being written now gets an error.

**Tests.** `pkg/stack/fluxcd/bootstrap_distribution_test.go` covers each empty field and both
together through both entry points, the set case, and the gotk control.

### FileNaming applied everywhere ([go-kure/kure#976](https://github.com/go-kure/kure/issues/976))

**Shipped.** `LayoutRules.FileNaming` names the resource files `WriteToDisk` and `WriteToTar`
write, in every layout of that tree. It does not name `kustomization.yaml`, an extra file, or the
one `<Name>.yaml` of an `AppFileSingle` layout. Part 1 describes the behaviour below as current.

**What it does.**

- The `FluxSeparate` `flux-system/` layout takes the rules' FileNaming
  (`addSeparateFluxToLayout`). Rules passed to `IntegrateWithLayout` that leave it unset
  take the root layout's, the parent's. A `flux-system/` layout an earlier integration left is
  kept as it is.
- Layouts an augmenter adds inherit their parent's FileNaming when they leave it unset, at any
  depth. One that sets its own keeps it, and the layouts below it inherit that one.
- This is done in the walker, after the augmenter runs (`inheritFileNaming`, called from
  `renderApps`). `ManifestLayout.resolveManifestFileName` has no parent to read.
- `WriteManifest` already names every layout's files from its `Config`, so the gap existed only
  under `WriteToDisk` and `WriteToTar`.
- With `FileNamingKindName`, no file in `flux-system/` or in an augmenter layout is named
  `{namespace}-{kind}-{name}.yaml`, unless that layout, or a layout above it, was given another
  FileNaming. An augmenter can give one to the application's layout too, and the layouts below
  that leave theirs unset then take it.

**Breaking.** With `FileNamingKindName`, the files of `flux-system/` and of augmenter layouts are
renamed.

**Tests.** `pkg/stack/layout/filenaming_inherit_test.go` and
`pkg/stack/fluxcd/filenaming_separate_test.go` cover both cases through both writers.

### The writers validate a Flux-delivered tree ([go-kure/kure#977](https://github.com/go-kure/kure/issues/977))

**Shipped.** The writers refuse a Flux-built tree in which something is written that nothing
applies, any tree in which two Kustomizations share a name, and any layout whose final name or
namespace leaves its directory. Part 1 describes the behaviour below as current (section 1.7).

**What it does.** Three refusals in `checkLayoutTree` (`treecheck.go`, called from
`WriteToDisk`, `WriteToTar` and `WriteManifest`). The first runs for each layout as the tree is
walked; the other two run after the existing checks, so a tree those refuse is refused in their
words.

- **A `..` segment in any layout's name or namespace** (`checkLayoutIdentity` in `augmenter.go`).
  The check used to run only for a layout with extra files and its direct children. An augmenter
  can rename its layout after the walk, so the final values are checked for every layout, with or
  without extra files.
- **A directory nothing applies** (`checkUnappliedLayouts`). When the root is
  marked `SetFluxBuild` (the integrator marks it whenever it generated a Kustomization,
  `markFluxBuilds` in `layout_integrator.go`), every child its parent's `kustomization.yaml` does
  not list must be marked too, meaning some Kustomization's `spec.path` names it. The rule for
  "does not list" is the writers' own (`childEntry` in `writerplan.go`), so the check and the listing
  cannot disagree: an umbrella child, a child that renders a bundle, a directory child of a
  `FluxIntegratedPerLayout` parent and, under `WriteToDisk` and `WriteToTar`, a directory child of
  another package.
  - A child of another package is not exempt. Those two writers write its directory into this
    tree and nothing in the tree lists it, so the question "what applies it" is the same one.
    `WriteManifest` lists it, so the check does not reach it there. Nothing in kure sets
    `PackageRef` on a layout of a walked tree, so this arises only in a hand-built one.
  - An `AppFileSingle` child without resources writes no file and is not checked.
  - ArgoCD trees are not marked and are unaffected. Their placement cannot tell them apart,
    because the ArgoCD walk uses `FluxSeparate` (`WorkflowEngine.CreateLayoutWithResources` in
    `pkg/stack/argocd/argo.go`).
  - A consumer that places its own Kustomizations marks its tree the same way.
  - A Kustomization the integrator keeps in place of one of its own (same name, host layout and
    `spec.path`) marks its directory like a generated one. `markFluxBuilds` used to find
    Kustomizations by a typed assertion directly in a layout's resources, so a kept one that was
    unstructured or sat inside a List left its directory unmarked, and this check would have
    refused a tree that wrote before. It now reads each layout through `resourceItems` and
    `fluxKustomizationPath`. `checkPlacedReconcileOrder` and `indexGenerated` still read by the
    typed assertion; they need the typed fields, so that change is item 6 of go-kure/kure#979.
- **A duplicate Kustomization namespace/name anywhere in the tree**
  (`checkKustomizationNames`), marked or not. The writers already refused one
  object held twice within a layout or within one kustomize build (section 1.7); two
  Kustomizations of one name in separately applied directories passed. The layout package cannot
  import fluxcd (fluxcd imports layout), so the integrator's `integratedPlacement.claim`
  cannot be reused. The check matches the group
  `kustomize.toolkit.fluxcd.io`, kind `Kustomization`, at any version; an omitted namespace is
  `default`.

**Lists.** The duplicate check reads a layout's objects as kustomize builds them (`builtObjects`
in `treecheck.go`): a List is an object whose kind ends in `List` and that has an `items` field,
and a List among the items is opened as well (`inlineAnyEmbeddedLists` in kustomize's
`api/resource/factory.go`). A kind that does not end in `List` is one object whatever fields it
has, as is a List kind without `items`; null `items` hold nothing. A typed List can hold an item
as raw JSON (`runtime.RawExtension`), which the writers serialize as the object it encodes: it is
read as that object, and an empty item holds nothing. An item that carries both raw JSON and an
object is written from the raw JSON, so the raw JSON is what is read. The integrator's
`resourceItems` (`layout_integrator.go`) follows the same rule, so the integrator and the
writers agree on which Kustomizations and Sources a tree holds. An item of a typed List need not
carry object metadata (`metav1.List` has none): the integrator opens such a List too, and reads
any other such item as the object the writers serialize for it. The writers refuse a Flux
Kustomization whose namespace and name they cannot read. The existing per-layout and
per-build identity checks are unchanged.

**Tests.** `pkg/stack/layout/fluxtree_test.go`, `pkg/stack/fluxcd/fluxtree_test.go` and
`TestCreateLayoutWithResources_UmbrellaTreeWrites` in `pkg/stack/argocd/argo_test.go` cover:
- A marked tree with a written directory its parent does not list and that is not marked is
  refused, naming the layout: an umbrella child, a directory that renders a bundle, a directory
  child of a `FluxIntegratedPerLayout` parent, a directory child of another package.
- A tree the integrator built writes in every placement; with one umbrella child's mark removed
  it is refused.
- A tree in which the integrator kept a caller's Kustomization in place of its own writes under
  both integrated placements, whether that Kustomization is typed, unstructured or inside a List.
- A hand-built tree with two Kustomizations of one namespace/name is refused, naming both
  directories, also when one sits in a List or in a List inside a List; a non-List kind with an
  `items` field is one object.
- A layout whose name or namespace has a `..` segment is refused without extra files, and
  nothing is written.
- An ArgoCD tree with umbrella children still writes.

### Name validation ([go-kure/kure#978](https://github.com/go-kure/kure/issues/978))

**Target.** A name that cannot work is refused when the model is validated, not after the cluster
rejects it.

**Design outline.**

- **Names that become Kustomization names** (`Bundle.Name` or `KustomizationName`,
  `Node.KustomizationName`) must be DNS-1123 subdomains of at most 63 characters. The extra limit
  exists because Flux labels the objects it applies with the Kustomization's name, and a label
  value is limited to 63 characters; the ticket verifies this against kustomize-controller.
- **Names that become directories** (node names, bundle and `DirName`, application names under
  `ApplicationGrouping: GroupByName`) must be non-empty, contain neither `/` nor `\`, and not be
  `.` or `..`. Both separators are refused because `FullRepoPath` joins the name with
  `filepath.Join` (`manifest.go:105-111`), which treats `\` as a separator on Windows: a name such
  as `..\outside` would otherwise leave the layout directory.
  The unnamed root is exempt: an empty root name is valid and never becomes a directory name
  (section 1.2, cluster wrapper; `walker.go:28-39`, `pkg/stack/layout/parentpath_test.go:436-446`).
- **Where:** bundle and node names are validated in `Bundle.Validate` (`bundle.go:174-270`) and
  `ValidateCluster` (`pkg/stack/validate.go:29`). An application name becomes a directory only
  where the application gets its own layout, which depends on the layout rules
  (`ApplicationGrouping: GroupByName`, or an augmenter that takes its own layout).
  `ValidateCluster` takes no rules, and `pkg/stack` cannot import the layout package, so that
  check runs in the walker. The error names the bundle's or node's path.
- **Existing fixtures** with a name the rules refuse are renamed in the same change
  (`pkg/stack/fluxcd/fluxcd_test.go:621-631` uses `Y-infra`, `Y-services` and `Y-apps`).

**Acceptance.** A 64-character Kustomization name, an upper-case name, and a directory name with
`/` or with `\` are each refused, naming the path. Dotted names and an unnamed root stay accepted.

### Behaviour bugs ([go-kure/kure#979](https://github.com/go-kure/kure/issues/979))

1. **A root bundle's Kustomization applies its own directory.**
   - Current: when the walked root renders a bundle (`ClusterName ""` with a named root, or after a
     `FlattenSingleTier` collapse), the root hosts its own Kustomization, and its
     `kustomization.yaml` lists it (`integratedPlacement.host`). `FluxSeparate` does the same
     through `<root>/flux-system/`. The bootstrap applies the root, which then applies itself:
     two owners of one directory.
   - Expected: every directory has exactly one owner. Either the root renders no bundle (its
     bundle moves into a child directory), or the root bundle's settings go to the bootstrap and
     no Kustomization is generated for it. The ticket chooses one.
2. **An integrated Source is hosted inside the directory it delivers.**
   - Current: when the root node renders a bundle, the Source lands in that bundle's directory
     (`integratedPlacement.add`), so the Kustomization that needs the Source is the one that
     would apply it.
   - Expected: a Source is hosted in a build that is applied before any Kustomization that
     references it, never inside a directory delivered through it.
3. **`GenerateFromCluster` ignores the consumer's layout rules.**
   - Current: it walks with the default rules (`ResourceGenerator.GenerateFromCluster`), so its paths disagree
     with the layout a consumer writes.
   - Expected: it takes the consumer's `LayoutRules`, or is removed in favour of `WalkCluster` plus
     `GenerateFromLayout`.
4. **The gotk bootstrap path differs from the FluxInstance sync path.**
   - Current: `manifests/<root>` (`generateFluxSystemKustomization`) against `./<root>`
     (`generateFluxInstance`).
   - Expected: both modes point at the same directory for the same root.

**Acceptance.** Each case has a test rendering the input above and asserting the expected tree.

### Documentation corrections ([go-kure/kure#980](https://github.com/go-kure/kure/issues/980))

**Shipped.** The six statements of section 1.9 match the code, and this page cites code by
symbol name instead of by line.

**What it does.**

1. The `normalizeRulesPlacement` comment names `layout.DefaultLayoutRules` and
   `layout.WalkCluster`, no lines.
2. The SourceRef refusal of `validateSourceRefsForFluxIntegrated` names the placement in use:
   `FluxIntegratedPerLayout mode requires a SourceRef with Kind and Name` or
   `FluxIntegratedPerBundle mode requires …`. This is the one change to output: the text of an
   error.
3. The fluxcd README says which layouts get a Kustomization under `FluxIntegratedPerLayout`,
   and that a layout whose Kustomization name is taken is refused. It keeps today's names; the
   naming from [go-kure/kure#973](https://github.com/go-kure/kure/issues/973) is restated there
   when that ships.
4. The FileNaming doc: nothing was left to do,
   [go-kure/kure#976](https://github.com/go-kure/kure/issues/976) made it true and updated it.
5. The helm README and the example it is generated from say that kure does not convert a
   `HookGroup`, and that a consumer builds a Kustomization per group itself.
6. The `ManifestLayout.DependsOn` comment states which layouts it applies to and that it is
   dropped elsewhere without an error.

The fluxcd README claims tied to the behaviour bugs change with
[go-kure/kure#979](https://github.com/go-kure/kure/issues/979).

**Tests.** `TestValidateSourceRefsForFluxIntegrated_MessageNamesPlacementInUse` in
`pkg/stack/fluxcd/validate_test.go` covers the message under both placements. The helm README
block is checked against `ExampleSplitByHookWeight`.

### Builders for missing base kinds ([go-kure/kure#981](https://github.com/go-kure/kure/issues/981))

**Shipped.** kure supports every base object with its full spec. Each kind below has a generated
`Create<Kind>` from its registered scheme (`mise run builders:generate`). One kind is out of scope,
`VerticalPodAutoscaler`; the reason is at the end of this section.

**Shipped: the kinds `k8s.io/api` provides.** Ten kinds have a generated constructor that sets
identity only, are covered by the whole-object identity test and appear in the generated kind
tables:

| Kinds | Group version | Scope |
|---|---|---|
| `PriorityClass` | scheduling.k8s.io/v1 | cluster |
| `EndpointSlice` | discovery.k8s.io/v1 | namespaced |
| `Lease` | coordination.k8s.io/v1 | namespaced |
| `RuntimeClass` | node.k8s.io/v1 | cluster |
| `MutatingWebhookConfiguration`, `ValidatingWebhookConfiguration`, `ValidatingAdmissionPolicy`, `ValidatingAdmissionPolicyBinding`, `MutatingAdmissionPolicy`, `MutatingAdmissionPolicyBinding` | admissionregistration.k8s.io/v1 | cluster |

**Breaking.** Parsing changes for these ten kinds. A strict parse used to refuse them and a parse
with `AllowUnstructured` returned `*unstructured.Unstructured`; both now return the `k8s.io/api`
type. `pkg/manifest` answers their scope from the generated table; `PriorityClass` and the two
webhook configurations left its residual list.

**Breaking: list documents.** `pkg/io` flattens a list of a registered kind into its items, in both
modes: a typed `<Kind>List` (`DeploymentList`, `PriorityClassList`) and the generic `v1` `List`. The
items are typed, keep the list's order and take the list's place in a multi-document stream; an
empty list yields nothing; an item of a generic `List` whose kind is not registered follows
`AllowUnstructured`. Each item is decoded by itself: one that does not decode, a `null` among them,
is an error naming its position and the items beside it are still returned. A list may sit inside
eight generic Lists; one nested deeper is refused. Both shapes used to be refused with "does not implement client.Object", so a
caller that relied on that refusal, for instance to reject list documents in its input, now gets
the objects and has to check for lists itself before parsing.

**Shipped: the Flux image kinds.** `ImageRepository` and `ImagePolicy`
(image.toolkit.fluxcd.io/v1, namespaced) have a generated constructor in `pkg/kubernetes/fluxcd`.
Their Go types come from `github.com/fluxcd/image-reflector-controller/api`, a new direct
dependency at the version the pinned flux2 release uses; it rides the `fluxcd` update group with
the other controller API modules, and the vendored install bundle already carried its two
definitions. Parsing changes for them as it did for the ten kinds above.

**Shipped: `APIService`.** `APIService` (apiregistration.k8s.io/v1, cluster-scoped) has a generated
constructor in `pkg/kubernetes`. It is a built-in whose Go type lives in `k8s.io/kube-aggregator`,
a new direct dependency of which kure imports the API package only. The module is pinned in the
`replace` block of `go.mod` at the release of the other `k8s.io` modules and rides the `kubernetes`
update group with them. Parsing changes for it as it did for the kinds above, and `pkg/manifest`
answers its scope from the generated table: it was the last entry of the residual list, which is
empty now.

**Out of scope: `VerticalPodAutoscaler`.** It gets no constructor and is not registered, for three
reasons. Its Go type lives in `k8s.io/autoscaler/vertical-pod-autoscaler`, which is the whole
autoscaler component and not an API-only module, and the kind is not part of the Kubernetes API.
Requiring that module moves the versions of 17 indirect dependencies kure shares with its other
modules (measured at v1.8.0). And registering its group version also registers
`VerticalPodAutoscalerCheckpoint`, the autoscaler's internal state. What stays available: parse it
with `AllowUnstructured`, or build it with the module's own types in the calling program. The same
is in `pkg/kubernetes/README.md` under "Base kinds covered".

**Tests.** The identity test (`pkg/kubernetes/identity_test.go`) compares each constructor's whole
object, and the generated `pkg/kubernetes/zz_generated_create_test.go` calls each wrapper by name.
`pkg/kubernetes/scheme_test.go` asserts each kind's registration.
`pkg/io/runtime_base_kinds_test.go` parses each kind in both modes and keeps
`VerticalPodAutoscaler` as the control: refused by a strict parse, untyped with `AllowUnstructured`. `pkg/io/runtime_lists_test.go` covers list documents in both modes:
a typed list of each kind, an empty list, a generic `List` with mixed items, the order of a
multi-document stream, an item that does not decode or is `null` in either shape, and the nesting
bound.
