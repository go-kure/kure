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
after it, and the row or paragraph that describes it names the issue. A Part 2 section whose
change has not shipped may still cite lines; it moves to symbol names when the change ships.

### 1.1 The model

| Type | Fields that decide names and placement | Source |
|---|---|---|
| `stack.Node` | `Name`, `Children`, `Bundle` (one per node), `PackageRef`. No dependency, Kustomization or directory field. | `Node` in `pkg/stack/cluster.go` |
| `stack.Bundle` | `Name`, `KustomizationName` (go-kure/kure#971; `v0.2.0-beta.15` had no field for the Kustomization's name), `DependsOn []*Bundle`, `NamedDependsOn`, `Children` (umbrella), `SourceRef`, `Interval`, `Prune`, `Wait`, `Timeout`, `RetryInterval`, `Force`, `Suspend`, `HealthChecks`, `Patches`, `PostBuild`, `Labels`, `Annotations`. No field for the Kustomization's namespace or directory. | `Bundle` in `pkg/stack/bundle.go` |
| `stack.Application` | `Name`, `Namespace`, `Config`. No dependency field. No delivery field in `v0.2.0-beta.15`; `Delivery` was added after it ([go-kure/kure#974](https://github.com/go-kure/kure/issues/974)) and decides no name or placement. | `Application` in `pkg/stack/application.go` |

`layout.WalkCluster` turns the model into a `ManifestLayout` tree. `fluxcd.LayoutIntegrator` adds
Flux objects to that tree in one of three placements: `FluxSeparate` (the default),
`FluxIntegratedPerBundle` and `FluxIntegratedPerLayout`. The writers (`WriteToDisk`,
`WriteToTar`, `WriteManifest`) check the tree and write it.

### 1.2 Directories and files

| What | Rule | Source |
|---|---|---|
| Layout directory | `FullRepoPath() = Namespace/Name` | `ManifestLayout.FullRepoPath` in `pkg/stack/layout/manifest.go` |
| Cluster wrapper | `ClusterName "."`: none. `"prod"`: `prod/`. `""` with a named root: `<root>/`. `""` with an unnamed root: `cluster/`. A `ClusterName` with a `..` path segment is refused at the walk (go-kure/kure#979). | `WalkCluster` and `walkClusterWithClusterName` in `pkg/stack/layout/walker.go`; `LayoutRules.Validate` in `pkg/stack/layout/types.go` |
| Node directory | node name, nested by tree (`NodeGrouping: GroupByName`, the default). `GroupFlat` merges descendant nodes' bundles into the first-level node's directory. | `walkNode`, `renderNodeContent` and `renderChildren` in `walker.go` |
| Bundle directory | none by default: the bundle renders into its node's directory. The root node is the exception (go-kure/kure#979): its layout renders no bundle, so the bundles the default would render there (its own, and under `NodeGrouping: GroupFlat` those of the nodes it absorbs) share one directory inside it, named after the first of them: `<root>/<first bundle name>`. A child node of the root with that name is refused, and so is a bundle name that resolves to the root node's directory itself (`.`, `/`) or to the directory of a layout below a child node (`web/api` beside the nodes `web` and `api`). `BundleGrouping: GroupByName` adds `<node>/<bundle name>` for every bundle. | `renderBundle`, `rootUnit` and `rootUnit.checkRootUnitName` in `walker.go` |
| Umbrella child directory | `<parent dir>/<child bundle name>`, marked `UmbrellaChild` | `renderUmbrellaChildren` in `walker.go` |
| Application directory | none by default. `ApplicationGrouping: GroupByName` adds `<bundle dir>/<app name>`. Under the default flat grouping an application whose config is a `LayoutAugmenter` still gets its own directory, unless the config also implements `LayoutIntentAugmenter` and `WantsOwnLayout()` returns false; it is then merged like any other application. | `renderApps`, `wantsOwnLayout` and `isAugmenter` in `walker.go` |
| Resource file | `WriteToDisk` and `WriteToTar` name by the layout's own FileNaming: default `{namespace}-{kind}-{name}.yaml` (empty namespace: `cluster`); `FileNamingKindName` gives `{kind}-{name}.yaml`. `WriteManifest` names every layout's files from its `Config` instead. | `DefaultManifestFileName` in `pkg/stack/layout/config.go`; `groupResourceFiles` and `manifestPlan` in `writerplan.go` |
| Generated Kustomization file | under `WriteToDisk` and `WriteToTar`, named by the host layout's FileNaming. The `flux-system/` layout of `FluxSeparate` takes the rules' FileNaming (the root layout's when the rules leave it unset): `flux-system-kustomization-<name>.yaml` by default, `kustomization-<name>.yaml` with `FileNamingKindName`. A layout an augmenter adds and leaves unset takes its parent's. | `ManifestLayout.resolveManifestFileName` in `manifest.go`; `addSeparateFluxToLayout` in `pkg/stack/fluxcd/layout_integrator.go`; `inheritFileNaming`, called from `renderApps`, in `walker.go` |
| `kustomization.yaml` child entries | a child directory is listed unless it is an `UmbrellaChild`, renders a bundle, or the parent is `FluxIntegratedPerLayout` | `childEntry` in `writerplan.go` |

`LayoutRules.FlattenSingleTier` (default false) collapses one layer, and only at the walked root:
the root must have a namespace without `/`, exactly one child, no resources of its own, and that
child must be a terminal, non-umbrella layout that renders no bundle (`flattenSingleTier` and
`canFlatten` in `pkg/stack/layout/flatten.go`, called from `WalkCluster`). It does not reach an
application directory deeper in the tree. The last condition is go-kure/kure#979: a directory that
renders a bundle is applied by its own Kustomization, which the top of the tree cannot host. In a
walked tree the one child left to collapse is a directory that renders no bundle and has none
below it: a node without a bundle and without child nodes, or, under `NodeGrouping: GroupFlat`, a
node with the bundle-less nodes below it merged into it.

### 1.3 Flux Kustomizations: name, host and listing

**Names.**

- A bundle's Kustomization is named `Bundle.KustomizationName`, or `Bundle.Name` when that is
  empty, in the generator's `DefaultNamespace` (`Bundle.UnitName` in `pkg/stack/bundle.go`;
  `kustomizationForBundle` in `pkg/stack/fluxcd/resource_generator.go`). The field is there since
  go-kure/kure#971; `v0.2.0-beta.15` named it `Bundle.Name`. Below, a bundle's Kustomization name
  is that name in effect.
- Several bundles in one directory (a `GroupFlat` merge) share one Kustomization, which takes the
  Kustomization name of the first bundle (`generateForUnit` in `resource_generator.go`;
  `OriginIndex.UnitName` and `UnitOfName` in `pkg/stack/layout/origin.go`).
- Under `FluxIntegratedPerLayout`, a bundle-less node layout gets a Kustomization named
  `<path with / replaced by ->-node`, and an application or augmenter layout one named after the
  layout (`layoutCRName` in `layout_integrator.go`).

**Where each placement puts them.**

| Placement | Bundle Kustomization hosted in | Extra Kustomizations | Host's `kustomization.yaml` lists |
|---|---|---|---|
| `FluxSeparate` | `flux-system/` directly under the walked root's directory | none | `flux-system/` lists the CR files; the root lists `flux-system` |
| `FluxIntegratedPerBundle` | the parent of the bundle's directory (`integratedPlacement.place` in `layout_integrator.go`). The top of the tree `WalkCluster` returns renders no bundle, so every such directory has a parent; a tree whose top renders one is refused (go-kure/kure#979). | none | the CR files |
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
| `GenerateFromCluster(c, rules)` | walks with the caller's `rules` and generates from that layout, so the paths are those of the first row for a tree written with the same rules. Rules with `FluxIntegratedPerLayout` are refused, pointing to `CreateLayoutWithResources` (go-kure/kure#979; `v0.2.0-beta.15` took no rules and walked with `DefaultLayoutRules()`). | `ResourceGenerator.GenerateFromCluster` in `resource_generator.go` |
| `FluxIntegratedPerLayout` node Kustomization | the layout's directory | `createKustomizationForLayout` in `resource_generator.go` |
| Bootstrap, gotk mode | `<root>`, or `.` for an unnamed root: the directory the FluxInstance `sync.path` names (go-kure/kure#979; `v0.2.0-beta.15` wrote `manifests/<root>`) | `bootstrapDir` and the `spec.path` built in `generateFluxSystemKustomization`, `pkg/stack/fluxcd/bootstrap_generator.go` |
| Bootstrap, FluxInstance `sync.path` | `./<root>`, or `./` for an unnamed root | `bootstrapDir` and the sync path built in `generateFluxInstance`, `bootstrap_generator.go` |

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

Under the integrator, dependencies and health checks that name a bundle's Kustomization are mapped
to the Kustomization of the directory that renders the bundle, and a reference to its own
directory is dropped (`generateForUnit`). Bundles that share a directory must agree on every
reconcile setting (`mergeIntoUnit` in `resource_generator.go`).

The umbrella health checks exist only in generator output: `pkg/stack` exports nothing that returns
them.

### 1.6 Sources, the flux-system directory and bootstrap

**Sources.**
- A Source (`OCIRepository`, `GitRepository`) is generated only when `SourceRef.URL` is set
  (`createSource` in `resource_generator.go`).
- `FluxSeparate` puts it in `flux-system/`.
- The integrated placements put it in the root node's layout (the Source branch of
  `integratedPlacement.add` in `layout_integrator.go`). That layout renders no bundle
  (go-kure/kure#979), so a Source is not in the directory a bundle's own Kustomization applies.

**The `flux-system` directory.** Its name is a constant, and under `FluxSeparate` it is placed
directly under the walked root (`addSeparateFluxToLayout` in `layout_integrator.go`).

**Bootstrap, flux-operator mode (the default).**
- Emits the embedded flux-operator install bundle (`v0.58.1`) and a `FluxInstance` named `flux`.
- `distribution.registry` and `distribution.version` are written verbatim. `v0.2.0-beta.15` wrote
  an empty value too, as an empty string; since
  [go-kure/kure#975](https://github.com/go-kure/kure/issues/975) an empty value is an error.
- `sync` is emitted only when `SourceURL` is set.

**Bootstrap, gotk mode.** Emits the vendored components, a Kustomization whose `spec.path` is
`<root>` (`.` for an unnamed root), and a Source when `SourceURL` is set (`generateGotkBootstrap`
and `generateFluxSystemKustomization` in `bootstrap_generator.go`). That is the directory the
FluxInstance `sync.path` names: both modes take it from `bootstrapDir`, since go-kure/kure#979
(`v0.2.0-beta.15` wrote `manifests/<root>`).

### 1.7 Collision guards

| Input | Result | Source |
|---|---|---|
| Two bundles with the same name | refused wherever the origin index is built: the integrator under every placement, `GenerateFromLayout` and the ArgoCD workflow. `WalkCluster` and the writers alone do not check it. | `IndexOrigins` in `origin.go`; callers `addIntegratedFluxToLayout`, `ResourceGenerator.GenerateFromLayout` and `WorkflowEngine.generateFromLayout` in `pkg/stack/argocd/argo.go` |
| Two bundles whose Kustomizations would get one name, whether it comes from `KustomizationName` or from `Name` (go-kure/kure#971) | refused in the same places; the error names both bundles by their paths. `GenerateForBundle` builds no index and sees one bundle: it refuses a `DependsOn` bundle, an umbrella child or a `NamedDependsOn` entry with that bundle's own Kustomization name, a health check on that Kustomization unless `Wait` is true, and two children with one name. | `IndexOrigins` in `origin.go`; `checkOwnUnitName`, called from `ResourceGenerator.GenerateForBundle`, in `resource_generator.go` |
| A payload Kustomization named like the one generated for its bundle (the bundle's name, unless the bundle sets `KustomizationName`: go-kure/kure#971), in the namespace of the generated Kustomization (the generator's `DefaultNamespace`) | refused under `FluxSeparate`. Under the two integrated placements it is refused unless it sits in the layout that hosts the generated Kustomization and has the same `spec.path`: that one is kept as it is, and none is generated for the bundle. The host is the parent of the bundle's directory, never that directory itself (go-kure/kure#979), so a bundle's own payload cannot meet this. The check compares namespace and name, so it does not refuse the same name in another namespace. | `integratedPlacement.place`, `integratedPlacement.add`, `crKey` and `addSeparateFluxToLayout` in `layout_integrator.go`; `kustomizationForBundle` |
| A Kustomization name generated twice in one integration | refused by the integrator. The key is the bare name: every generated Kustomization is in `DefaultNamespace`. | `integratedPlacement.claim` in `layout_integrator.go`; `kustomizationForBundle` and `createKustomizationForLayout` |
| `FluxIntegratedPerLayout` with `ApplicationGrouping: GroupByName`, application named like its bundle's Kustomization (the common case: a bundle without `KustomizationName` and an application with the bundle's name) | refused as a name used twice | same |
| `FluxIntegratedPerLayout`, augmenter application named like its bundle's Kustomization | refused as a name used twice | same |
| Two directory layouts resolving to one directory | refused by the writers | `checkLayoutTree` in `pkg/stack/layout/treecheck.go` |
| Two single-file layouts (`AppFileSingle`) in one directory | accepted when their file names differ; refused when they resolve to the same file | `checkLayoutTree` |
| Dotted names | accepted unchanged | none |
| A bundle's Kustomization name (its `KustomizationName`, or its name without one) over 63 characters | refused by the Flux workflow where it builds the Kustomization (go-kure/kure#978); accepted by `Bundle.Validate` and by the ArgoCD workflow | `checkKustomizationName` in `resource_generator.go` |
| A Kustomization name derived for a layout under `FluxIntegratedPerLayout` (an application directory's name, a bundle-less node's `<path>-node`) that is over 63 characters or not a DNS-1123 subdomain | refused by the integrator where it creates that Kustomization (go-kure/kure#978) | `stack.ValidateKustomizationName`, called in `layout_integrator.go` |
| A `NamedDependsOn` entry over 63 characters | accepted unchanged | none |
| A hand-built tree with the same Kustomization twice in one layout, or in layouts one kustomize build includes | refused by the writers, as for any object held twice | `checkResourceIdentities` (one layout) and `checkBuildIdentities` (one build), both called from `checkLayoutTree` |
| A hand-built tree with the same Kustomization in layouts that separate builds apply | refused by the writers, in any tree: Kustomization namespace/name is unique across the tree (go-kure/kure#977) | `checkKustomizationNames` in `treecheck.go` |
| An `UmbrellaChild` layout, or any other directory its parent does not list, that no Kustomization applies | refused by the writers when the root is marked `SetFluxBuild` (go-kure/kure#977); written, and listed by nobody, in an unmarked tree | `checkUnappliedLayouts` in `treecheck.go` |

### 1.8 What a consumer cannot control today

1. **The name of a Kustomization that is not one bundle's own.** A bundle's Kustomization takes
   `Bundle.KustomizationName` since go-kure/kure#971 (in `v0.2.0-beta.15`, only by naming the
   bundle). A merged directory takes its first bundle's Kustomization name; `-node` names have no
   parameter.
2. **The directory that hosts it.** The only lever is the placement.
3. **A directory name separate from the bundle name.** An umbrella child's directory is the child
   bundle's name (`renderUmbrellaChildren`), whatever its Kustomization is named
   (`kustomizationForBundle`; go-kure/kure#971).
   Renaming the walked layout before `IntegrateWithLayout` gives the directory another name and is
   accepted, but no document or test covers that route.
4. **Ordering between groups, from the model.** A group gets a Kustomization only under
   `FluxIntegratedPerLayout`, with a fixed name, and `Node` has no dependency field. The one route
   to a `dependsOn` is on the layout, not the model: set `DependsOn` (Kustomization names, as
   strings) on the walked group layout before `IntegrateWithLayout`, which copies it
   (the per-layout branch of `integratedPlacement.place`, `createKustomizationForLayout`).
5. **Engine annotations per application.** In `v0.2.0-beta.15` there is no field for prune
   protection or force replace, so a consumer writes Flux annotations onto objects itself. No
   longer so: `Application.Delivery` carries the intent
   ([go-kure/kure#974](https://github.com/go-kure/kure/issues/974), below).

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
   that gets its own Kustomization, and every other case drops the field without an error. It
   also said the entries are sibling layout names; they are Kustomization names, copied
   verbatim, which differ from the layout's name for a node layout.

## Part 2: target behaviour

Each section names the change, the target, a design outline and the acceptance criteria. The
linked issue tracks the implementation. A section whose change has shipped is marked **Shipped**
and describes what the code does now.

### Kustomization name separate from the bundle name ([go-kure/kure#971](https://github.com/go-kure/kure/issues/971))

**Shipped.** A consumer can name a bundle's Kustomization without renaming the bundle.

**What it does.**

- **The field:** `Bundle.KustomizationName string`; empty means `Bundle.Name`. `Bundle.UnitName()`
  returns the name in effect (`pkg/stack/bundle.go`). `Bundle.Name` stays the bundle's identity
  and its directory, so every `spec.path` is what it is without the field.
- **One name for both workflows:** under the ArgoCD workflow the same value names the Application
  generated for the bundle. The accessor is called `UnitName` because it names the reconciliation
  unit generated for the bundle, the word `layout.OriginIndex` uses for the same thing.
- **Everything that refers to a bundle's Kustomization** uses the name in effect:
  - the Kustomization itself (`kustomizationForBundle` in `resource_generator.go`);
  - an umbrella's health check on each child (the umbrella branch of `kustomizationForBundle`);
  - `spec.dependsOn` for each `DependsOn` bundle (the `DependsOn` loop of
    `kustomizationForBundle`), and after a merge the unit that applies it
    (`OriginIndex.UnitDependencies` in `origin.go`);
  - unit naming (`OriginIndex.UnitName` and `UnitOfName`): bundles merged into one directory
    share the first bundle's name in effect;
  - the ArgoCD Application's name and its `spec.dependencies` entries (`applicationForBundle` in
    `pkg/stack/argocd/argo.go`).
- **Name lookups:** the origin index looks a name up among the names in effect
  (`OriginIndex.UnitOfName`), so a `NamedDependsOn` entry or a health check on a Flux
  Kustomization names a Kustomization, never a bundle. The `Name` of a bundle that sets
  `KustomizationName` is not that bundle's Kustomization: such an entry reaches the bundle whose
  Kustomization has that name, and is kept as written, as an external reference is, when no
  rendered bundle's has.
- **Uniqueness:** two bundles whose Kustomizations would get one name are refused wherever the
  origin index is built, and the error names both bundles by their paths (`IndexOrigins`).
  `Bundle.Name` stays unique as well, because a copied bundle is resolved by its `Name`.
  `GenerateForBundle` builds no index: it refuses a `DependsOn` bundle, an umbrella child or a
  `NamedDependsOn` entry with the bundle's own Kustomization name, the bundle itself in
  `DependsOn` or `Children`, and two children with one name (`checkOwnUnitName` in
  `resource_generator.go`). A health check on the bundle's own Kustomization is refused too,
  unless `Wait` is true: Flux ignores `spec.healthChecks` under `spec.wait`, so that check is
  written as given. The entry points that build the index drop a unit's reference to itself
  instead.
- **Validation:** `Bundle.Validate` compares the names in effect wherever it compares against a
  Kustomization reference: a `DependsOn` bundle against a `NamedDependsOn` entry, a child against
  both lists, and a child that names its parent in `NamedDependsOn` (`validateChildren` in
  `pkg/stack/bundle.go`). The checks on the bundle's own identity (two children with one `Name`,
  a child named like its parent) stay on `Name`.
- **A `DependsOn` copy:** a `DependsOn` entry that is not one of the cluster's bundles but has the
  `Name` of one is a copy of it and stands for it, so the dependency is on that bundle's
  Kustomization. A copy that leaves `KustomizationName` empty, or sets the bundle's name in
  effect, is accepted. A copy that names another Kustomization is refused, naming both, and so is
  a copy of a bundle whose Kustomization name is also in the dependant's `NamedDependsOn`
  (`validateDependencyCopies` in `pkg/stack/validate.go`, called from `ValidateCluster`;
  `OriginIndex.checkDependencyCopies`, called from `IndexOrigins`, for a tree walked earlier).
  `Bundle.Validate` sees one bundle and compares on the name the copy carries, so one valid input
  is refused: a name-only copy of a bundle that sets `KustomizationName`, beside a
  `NamedDependsOn` entry equal to the bundle's `Name`. Setting the bundle's `KustomizationName`
  on the copy says which Kustomization is meant.
- **The payload collision:** the integrator compares a payload Kustomization with the generated
  name, so a payload Kustomization named like the bundle no longer collides once the generated
  one has another name, and one named like the generated Kustomization still does.
- **The value is checked:** a `KustomizationName` must be a DNS-1123 subdomain (`Bundle.Validate`),
  and in the Flux workflow the name in effect must be at most 63 characters; see the section on
  go-kure/kure#978.

**Breaking.** With the field unset, every object and path generated for a model that was valid is
unchanged by that change (item 1 of go-kure/kure#979 has since moved the root node's bundle's
directory, see its section). `GenerateForBundle` does not call `Bundle.Validate`, and it used to return a
Kustomization that depends on or waits for itself for a bundle that depends on another bundle
whose Kustomization gets the same name, lists itself in `DependsOn`, names its own Kustomization
in `NamedDependsOn`, or (unless `Wait` is true) has a health check on its own Kustomization. For
an umbrella with a child whose Kustomization gets the umbrella's name, or two children whose
Kustomizations get one name, it returned a health check on itself or the same check twice. Each
is an error now, the two about children also when `Wait` is true. With the field unset the name
compared is `Name`, so these inputs are refused without the field as well; a bundle that sets
another `KustomizationName` may depend on a bundle with its own `Name`. A bundle that lists
itself in `Children` is an error too, where the call did not return before.

**Tests.** `pkg/stack/fluxcd/kustomization_name_test.go` covers the name, `dependsOn` and umbrella
health checks under the three placements, the payload Kustomization, the duplicate name and the
entry points without an integrator. `pkg/stack/layout/origin_unitname_test.go` covers the index,
`pkg/stack/bundle_unitname_test.go` the validation pairs, and
`pkg/stack/argocd/kustomization_name_test.go` the Application. The `DependsOn` copies are covered
by `pkg/stack/validate_copy_test.go`, `pkg/stack/layout/origin_copy_test.go` and
`dependency_copy_test.go` in `pkg/stack/fluxcd` and `pkg/stack/argocd`. `unset_output_test.go` in
`pkg/stack/fluxcd` and `pkg/stack/argocd` compares what a cluster without the field produces,
byte for byte, with output stored from the code before the field existed (one tree, the all-flat
`merged.txt`, was rewritten since for the path move of go-kure/kure#979 item 1, with no object
name changed).

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

**Shipped.** An application asks for prune protection or force replace without writing a delivery
engine's annotations. The Flux workflow turns that intent into the Flux annotations.

**What it does.**

- **Field:** `stack.Application.Delivery` is an engine-neutral `DeliveryIntent` with
  `PruneProtection` and `ForceReplace` booleans. Without an intent the output is unchanged.
- **Per-application attribution:** the walker records each application's objects after its layout
  augmenter ran, the child layouts the augmenter appended included
  (`ManifestLayout.OriginApplicationObjects`). The layout package stays engine-neutral and applies
  nothing.
- **The Flux mapping:** `fluxcd.LayoutIntegrator` sets `kustomize.toolkit.fluxcd.io/prune: disabled`
  and `kustomize.toolkit.fluxcd.io/force: enabled` on those objects, under every placement
  (`applyDeliveryIntents` in `delivery.go`). Only the integrator applies the intent: a layout
  written without `IntegrateWithLayout` or `CreateLayoutWithResources` carries none.
- **`GenerateFromCluster` refuses a set intent:** the call returns Kustomizations and Sources and
  none of the application's objects, so the intent could not reach its output. After the walk,
  and so after its placement refusal and any walk error, it returns an error naming the
  application and pointing to `CreateLayoutWithResources` (`refuseDeliveryIntent` in
  `delivery.go`). `GenerateFromLayout` does not refuse: its caller holds the layout and may have
  integrated it, and the list it returns carries no intent by itself.
- **Lists:** a `List` contributes what it holds, a `List` inside it included, never the envelope.
  What is a `List` is decided as kustomize decides on the written file: a kind ending in `List`
  that has an `items` field. A typed object whose written form still holds an object without the
  annotation, because its Go value gives no access to it, is refused; an item held as raw JSON is
  read from that JSON, as the writers serialize it.
- **Sources:** an application may emit the Source a bundle's `SourceRef` also derives; the
  integrated placements take the two for one object. With an intent on that application the
  derived Source carries the same annotations, so the two still compare equal.
- **Refusal leaves no annotation behind:** the annotations are set in place, on the objects the
  layout holds, and the integrator records what it added. A refused integration, for a conflict or
  for any later reason, takes them back. After a successful one they stay on those objects.
- **Conflicts:** an object or generator that already carries the annotation with another value is
  refused, naming the application and the object.
- **Generated ConfigMaps (decided in the ticket):** covered. `ConfigMapGeneratorSpec.Annotations`
  is written as the generator's `options.annotations` in `kustomization.yaml`. Consequence: kustomize
  names a generated ConfigMap after its content, so under prune protection each content change
  leaves the previous one in the cluster.
- **Patches:** in a directory several bundles share, the patch-scope check sees a generated
  ConfigMap with the annotations its generator sets, the intent's included, and for the generators
  of the application's whole layout subtree. A bundle's own patch is applied by Flux after the
  build and can still change or remove a delivery annotation; kure does not read patch bodies.
- **ArgoCD (decided in the ticket):** its mapping is out of scope. Until it exists, every ArgoCD
  entry point refuses a set intent, naming the application, instead of dropping it.

**Tests.** `pkg/stack/fluxcd/delivery_intent_test.go` covers both flags under every placement and
grouping, on disk and in tar, the objects of an augmenter's child layout, a generated ConfigMap,
the conflict refusals, the rollback after a later refusal, and the unchanged output without an
intent. `TestDeliveryIntent_GenerateFromClusterRefused` covers the `GenerateFromCluster` refusal
with the intent set and unset. The delivery-intent tests in `pkg/stack/argocd` cover the ArgoCD
refusal.

**Not breaking.** `Delivery` is new with this change, so no existing caller sets an intent, and
`GenerateFromCluster` returns what it returned for a cluster without one.

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
    `fluxKustomizationPath`. `checkPlacedReconcileOrder` and `indexGenerated` need the typed
    fields and read a kept one through `generatedKustomizations` since item 6 of
    go-kure/kure#979 (below).
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

**Shipped.** A name that cannot work is refused before anything is written, not after the cluster
rejects it: when the model is validated, or, for a limit only Flux has, when the Flux workflow
generates. kure shortens and rewrites nothing; the caller chooses a valid name.

**What it does.**

- **Three rules, two of them exported** (`pkg/stack/names.go`):
  - a bundle name is a DNS-1123 subdomain of up to 253 characters, which every delivery engine
    needs for the object it applies the bundle with (`validateBundleName`). `my.app` is valid,
    `My-App` is not;
  - a Flux Kustomization name is a DNS-1123 subdomain of at most 63 characters
    (`ValidateKustomizationName`, `KustomizationNameMaxLength`);
  - a directory name is one path segment: not empty, not `.` or `..`, without `/`, `\` or a NUL
    byte (`ValidateDirectoryName`). Both separators are refused on every platform, because
    `ManifestLayout.FullRepoPath` joins names with `filepath.Join`, which reads `\` as a separator
    on Windows: `..\outside` would otherwise leave its directory. A NUL byte is refused because
    such a directory cannot be created. Characters that only some file systems refuse are not
    checked.
- **Why 63.** Flux labels every object it applies with the Kustomization's name, and a label
  value is at most 63 characters. Verified in kustomize-controller v1.9.5, the version this module
  pins: the reconciler hands the name to the apply manager as the owner of the objects
  (`internal/controller/kustomization_controller.go:462`), and the manager writes it as the value
  of the `kustomize.toolkit.fluxcd.io/name` label (`github.com/fluxcd/pkg/ssa` v0.76.2,
  `manager.go:66-78`). The API server admits a longer name and every apply then fails.
- **The model** (`Bundle.Validate`, `ValidateCluster`): a bundle's `Name`, and that of every
  umbrella descendant, is a bundle name and a directory name (`Bundle.validateNames`). A
  `KustomizationName`, when set, is a DNS-1123 subdomain as well, since it names the same object;
  `Name` is checked as before, being still the bundle's identity and its directory. A node name is
  a directory name. The 63-character limit is not the model's: a name of 64 to 253 characters
  validates, and the ArgoCD workflow renders an Application named after it.
- **Node names are checked under every layout grouping.** A node name is a segment of the node's
  path in the model before it is a directory: `Node.GetPath` and the path map join node names
  with `/`, whatever the layout rules are. Two unnamed children of one node share a path, and so
  do a node named `a/b` and a node `b` below its sibling `a`. Such a cluster is refused also where
  `NodeGrouping: GroupFlat` gives the node no directory. The unnamed root is exempt: it adds no
  segment and has no directory of its own.
- **The Flux workflow** checks the 63-character limit on the name a Kustomization gets, the
  bundle's name in effect (`Bundle.UnitName`; `checkKustomizationName` in
  `resource_generator.go`). The error names the bundle by its path, an umbrella child with its
  umbrella's, and the field that holds the name. A `Name` over the limit next to a
  `KustomizationName` within it generates. Which names are checked depends on what the entry
  point returns:
  - `GenerateForBundle` returns one Kustomization and builds none of those it refers to, so it
    checks the bundle's own name, each umbrella child's (written as a health check) and each
    `DependsOn` bundle's (written as a dependency);
  - `GenerateFromCluster`, `GenerateFromLayout` and the layout integrator build one Kustomization
    per directory that renders bundles (`generateForUnit`). They check that unit's name, which is
    its first bundle's. The name of a bundle merged into the unit is written nowhere and is not
    checked. A reference to a rendered bundle, an umbrella child or a `DependsOn` bundle with the
    `Name` of one, is written as the name of that bundle's unit, which the same pass generates
    and checks. A `DependsOn` bundle that is not one of the cluster's bundles gets no
    Kustomization from the pass and its name is written as it is, so it is checked there;
  - a `NamedDependsOn` entry and a caller's own health check are references the caller supplies,
    and are not checked: one that names a rendered bundle's Kustomization is written as the name
    of that bundle's unit, any other as given.
- **Application names** become directories only where the layout rules give the application one:
  `ApplicationGrouping: GroupByName`, or an augmenter application that takes its own layout.
  `ValidateCluster` takes no rules and `pkg/stack` cannot import the layout package, so the walk
  checks them, on the tree it returns (`checkApplicationDirs` in `walker.go`): an application
  whose directory `FlattenSingleTier` merged into its parent's keeps any name.
- **Entry points that run no cluster validation** check for themselves. `GenerateForBundle` does
  as above. The bootstrap generator checks a non-empty root node name as a directory name where
  it builds a path from it (`validateRootName` and `validateSyncRootName` in
  `bootstrap_generator.go`): always in gotk mode (the bootstrap Kustomization's `spec.path`), and
  in flux-operator mode and `GenerateFluxInstance` only when a sync is built, that is when
  `SourceURL` is set (the FluxInstance's `sync.path`). Without a sync the root name is not read.
  In gotk mode a named root also names the generated `GitRepository` or `OCIRepository` and the
  bootstrap Kustomization's `sourceRef`, so there it must be a DNS-1123 subdomain as well
  (`validateRootSourceName`), with or without a `SourceURL`.
- **Per-layout Kustomizations.** Under `FluxIntegratedPerLayout` a directory that renders no
  bundle gets a Kustomization named after its layout: an application directory's name, or
  `<path>-node` for a bundle-less node. The integrator checks that name with
  `stack.ValidateKustomizationName` where it creates the Kustomization, and refuses with the
  layout's path. The check is on the result, whatever derived it.
- **Not covered here:** how kure derives a Kustomization name, and the check of the names that
  change adds, are
  [go-kure/kure#973](https://github.com/go-kure/kure/issues/973)'s; a layout name changed after
  the walk is contained by the writers' check
  ([go-kure/kure#977](https://github.com/go-kure/kure/issues/977)). Names of payload objects are
  out of scope.

**Breaking.** A model that was written out before can now be refused: a bundle name or
`KustomizationName` that is not a DNS-1123 subdomain (an upper-case letter, an underscore); a
bundle or node name that is not one path segment; an unnamed node below the root; an application
name that is not one path segment where the application gets a directory; in the Flux workflow, a
Kustomization name over 63 characters, and under `FluxIntegratedPerLayout` a layout whose name
cannot name its Kustomization; a root node name that is not one path segment where the bootstrap
builds a path from it, or in gotk mode not a DNS-1123 subdomain. None of these could be applied or
written safely. A nil `DependsOn` entry is refused at validation, where it used to fail in the
generator. Existing fixtures with such names are renamed in the same change: the upper-case
bundles `Y-infra`, `Y-services` and `Y-apps` in `pkg/stack/fluxcd/fluxcd_test.go` become
lower-case, `webA` and `webB` become `web-a` and `web-b`, and two unnamed child nodes in an ArgoCD
test get names.

**Tests.** `pkg/stack/names_test.go` holds the rule tables and the model's checks, the
`KustomizationName` value included. `bundle_names_test.go` in `pkg/stack/fluxcd` covers the
63-character limit at every entry point: a bundle and an umbrella descendant, the name in effect,
a merged bundle's name, a reference outside the cluster, and the per-layout Kustomization names.
`bootstrap_names_test.go` covers the root name under both rules, `pkg/stack/layout/names_test.go` and `flatten_test.go` the application names, and
`pkg/stack/argocd` (`argo_test.go`, `kustomization_name_test.go`) the names over 63 characters
that the ArgoCD workflow renders.

### Behaviour bugs ([go-kure/kure#979](https://github.com/go-kure/kure/issues/979))

The items carry the ticket's numbers. Each says whether it has shipped or is a target.

1. **A root bundle's Kustomization applies its own directory.** Shipped.
   - Before: when the walked root rendered a bundle (`ClusterName ""` with a named root, or after
     a `FlattenSingleTier` collapse), the root hosted its own Kustomization, and its
     `kustomization.yaml` listed it. `FluxSeparate` did the same through `<root>/flux-system/`.
     The bootstrap applied the root, which then applied itself: two owners of one directory.
   - Now: the root node's layout renders no bundle. With `BundleGrouping: GroupFlat` the bundles
     that would render there (the root node's own, and under `NodeGrouping: GroupFlat` those of
     every node it absorbs) are rendered into one directory inside it, named after the first of
     them: `<root>/<first bundle name>` (`renderBundle` and `rootUnit` in `walker.go`).
     `ManifestLayout.OriginUnit` (`origin.go`) returns that directory's layout. They stay one
     unit: one directory, one Kustomization, named as before. The directory takes the bundle's
     `Name`; a `KustomizationName` names the Kustomization alone, as for every other bundle
     directory. `BundleGrouping: GroupByName`
     already gave each bundle a directory and is unchanged. Both walkers do this,
     `WalkClusterByPackage` in the tree of the package the root node is in. Its trees are
     otherwise placed as before, and not as `WalkCluster` places them: the root node without the
     `ClusterName`, and no directory for a node outside the package.
   - A child node of the root named like that directory is refused by the walk, naming the node,
     the bundle and the directory (`rootUnit.checkRootUnitName` in `walker.go`). The two
     directories are compared as the writers compare them (`ManifestLayout.SameDirectory` in
     `manifest.go`: the path resolved under the output directory and cleaned, without regard to
     case), so a name that differs only in case, or one that resolves to the same directory
     (`/web`, `./web`, or the same path under a rooted `ClusterName`), is refused too. The
     engines' refusals below compare the same way. A bundle name that resolves to the root
     node's directory itself (`.`, `/`) names no directory inside it and is refused by the same
     check, naming the bundle and the directory. So is a bundle name with a path separator
     that resolves to the directory of a layout below a child node (`web/api` beside the nodes
     `web` and `api`; `rootUnit.checkBelow`). `BundleGrouping: GroupByName` is no way
     around either: the walk does not check there, and the writers refuse the tree.
   - The engines' own directory at the top of the tree must be free. Under `FluxSeparate` a root
     node's bundle (or a child node) rendered to `<top>/flux-system` is refused by
     `addSeparateFluxToLayout` in `layout_integrator.go`, and one rendered to `<top>/argocd` by
     `WorkflowEngine.CreateLayoutWithResources` in `pkg/stack/argocd/argo.go`. Before item 1 a
     root bundle of that name was written into the root node's directory and accepted; a child
     node of that name was already refused, by the writers.
   - An unnamed root under a `ClusterName` is rendered into the `ClusterName` directory, which
     keeps its `kustomization.yaml` under `WriteManifest` although it now holds no resource
     (`manifestPlan` in `writerplan.go`): without one, the Kustomization that builds it would
     take in every file below it, each bundle's included.
   - Every Kustomization is hosted in the parent of the directory it applies
     (`integratedPlacement.place` in `layout_integrator.go`), or in `flux-system/` under
     `FluxSeparate`. `LayoutIntegrator.IntegrateWithLayout` refuses a tree whose top renders a
     bundle, in every placement. `WalkCluster` returns no such tree. A subtree of a walked tree
     can be one, and so can the `WalkClusterByPackage` tree of a package the root node is not
     in: its unnamed wrapper is not the root node's layout, and under `NodeGrouping: GroupFlat`
     it still renders the bundles merged into it. The walk of that wrapper is unchanged.
   - `FlattenSingleTier` no longer collapses a child that renders a bundle (`canFlatten` in
     `flatten.go`): the collapse would put the bundle back into the top of the tree. Section 1.2
     says what it still collapses.
   - Kept: the `SourceRef` of the root node's own bundle is still the source of the root node's
     layout and of the layout Kustomizations below it that no other bundle encloses
     (`unitSource`, used by `integratedPlacement.place` and `integratedPlacement.layoutSource`),
     although the bundle now renders one directory lower. Under `BundleGrouping: GroupByName`
     the root node's layout never had a source of its own, and such a tree is refused as before.
     Accepting it would be a change of its own.
   - Breaking: the path of the root node's bundles moves one directory down, from `<root>` to
     `<root>/<first bundle name>`: the files, the Kustomization's `spec.path` and the ArgoCD
     Application's `source.path`. `FlattenSingleTier` leaves a directory it used to collapse.
     On a deployed tree with `prune` on, the objects can be deleted by the outer owner and
     re-created by the inner Kustomization.
   - Tests: `TestWalk_RefusesChildNodeNamedLikeTheRootBundlesDirectory` and
     `TestFlattenSingleTier_KeepsBundleDirectories` in `pkg/stack/layout/origin_test.go`;
     `TestFlatten_KeepsADirectoryThatRendersABundle` in `pkg/stack/layout/flatten_test.go`;
     `TestReconcileOrder_RootBundleAppliesNoCR`,
     `TestRootBundleDirectoryHostsItsUmbrellaChildren`,
     `TestPerLayout_RootNodeLayoutKeepsTheRootBundlesSource` and
     `TestIntegrateWithLayout_RefusesATopThatRendersABundle` in
     `pkg/stack/fluxcd/units_test.go`; `TestFluxSeparate_UnnamedRootBuildAppliesNoBundle` and
     `TestFluxSeparate_RefusesLayoutInTheFluxDirectory` in
     `pkg/stack/fluxcd/generate_from_cluster_rules_test.go`;
     `TestKustomizationName_RootBundleDirectoryFollowsName` in
     `pkg/stack/fluxcd/kustomization_name_test.go`;
     `TestCreateLayoutWithResources_RefusesLayoutInTheArgoDirectory` in
     `pkg/stack/argocd/argo_test.go`; `TestGenerateFromCluster_UsesCallerRules` in
     `pkg/stack/argocd/generate_from_cluster_rules_test.go` for `source.path`;
     `TestGenerateFromCluster_RootBundleDirectoryFollowsName` in
     `pkg/stack/argocd/kustomization_name_test.go`. The stored all-flat tree that
     `unset_output_test.go` in `pkg/stack/fluxcd` compares
     (`testdata/unset-kustomization-name/merged.txt`) was rewritten for the path move, so it is
     no longer output from before `KustomizationName` existed; no object name in it changed.
2. **An integrated Source is hosted inside the directory it delivers.** Target.
   - Before item 1: when the root node rendered a bundle, the Source landed in that bundle's
     directory (`integratedPlacement.add`), so the Kustomization that needed the Source was the
     one that applied it.
   - Current: the Source is hosted in the root node's layout, which since item 1 renders no
     bundle, so no bundle's Kustomization applies the directory that holds its Source. Nothing
     renders a tree and asserts that yet.
   - Two checks can no longer be met by a root bundle on a walked tree and stay in place.
     `integratedPlacement.checkRootBuildKeepsHostedSources` still words its remedy for one
     ("move the patch to a bundle below the root node"), but no bundle's patches or postBuild
     reach the root node's layout any more. It still guards a Kustomization the integration
     keeps in place of its own when that one builds the root node's layout: the layout
     Kustomization of the root node's layout under `FluxIntegratedPerLayout` below a
     `ClusterName` wrapper. The reconcile-order check (`checkPlacedReconcileOrder`) refused a
     root bundle that waits while a child node's bundle depends on it; the root bundle's
     directory now holds no child node's Kustomization, so that cycle is gone. (It still hosts
     those of the bundle's umbrella children and, under `FluxIntegratedPerLayout`, of the
     layouts inside it.) The check still refuses it below the root, and for a kept
     Kustomization.
   - Expected: a Source is hosted in a build that is applied before any Kustomization that
     references it, never inside a directory delivered through it, with a render test of that
     invariant and refusals worded for what they still guard.
3. **`GenerateFromCluster` takes the caller's layout rules.** Shipped.
   - Before: it took no rules and walked with `DefaultLayoutRules()`, so its paths disagreed with
     a layout written with rules that place directories differently (a `ClusterName`, a flat
     grouping).
   - Now: `GenerateFromCluster(c, rules)` of `stack.Workflow` (`pkg/stack/workflow.go`) and of
     both engines (`WorkflowEngine.GenerateFromCluster` in `pkg/stack/fluxcd/workflow_engine.go`
     and in `pkg/stack/argocd/argo.go`) walks with `rules`, so every Kustomization's `spec.path`
     and every ArgoCD Application's `source.path` is a directory a walk with those rules writes.
     `rules` must be a `layout.LayoutRules` value; anything else, nil included, is refused.
   - `ResourceGenerator.GenerateFromCluster` (`resource_generator.go`) refuses rules with
     `FluxIntegratedPerLayout` and points to `CreateLayoutWithResources`: the Kustomizations
     that apply a per-layout tree's child directories exist only where the integrator places
     them (item 7). The ArgoCD engine refuses both integrated placements, as its
     `CreateLayoutWithResources` does (`layoutRules` in `argo.go`).
   - Invalid rules (item 8) are an error whatever the cluster is, an absent or empty one
     included.
   - Breaking: the signature. A caller passes the rules it writes the tree with.
   - Tests: `TestGenerateFromCluster_UsesCallerRules`, `TestGenerateFromCluster_RefusesPerLayout`
     and `TestGenerateFromCluster_InvalidRules` in
     `pkg/stack/fluxcd/generate_from_cluster_rules_test.go`; `TestGenerateFromCluster_UsesCallerRules`,
     `TestGenerateFromCluster_RefusedRules` and `TestGenerateFromCluster_InvalidRules` in
     `pkg/stack/argocd/generate_from_cluster_rules_test.go`.
4. **Both bootstrap modes name one directory.** Shipped.
   - Before: the gotk Kustomization had `spec.path: manifests/<root>`, the FluxInstance
     `sync.path: ./<root>`.
   - Now: both take the directory from `bootstrapDir` (`bootstrap_generator.go`), which is the
     root node's name. The gotk `spec.path` is that directory, `.` for an unnamed or absent root
     node (`generateFluxSystemKustomization`); the FluxInstance `sync.path` is `./` followed by
     it (`generateFluxInstance`).
   - Breaking: the gotk `spec.path` loses its `manifests/` prefix.
   - Tests: `TestBootstrapModesNameTheSameDirectory` and
     `TestBootstrapPathIsTheWalkedRootDirectory` in `pkg/stack/fluxcd/bootstrap_path_test.go`.
     The second covers a named root walked without a `ClusterName`; the other shapes are item 9.
5. **Under `FluxIntegratedPerLayout` the patch-scope check does not see a generated ConfigMap.**
   Target.
   - Current: `checkPatchScope` in `resource_generator.go` counts only the objects held in the
     unit's `Resources` and in those of its `AppFileSingle` application children. A ConfigMap
     that a `configMapGenerator` builds is not among them, so it
     is skipped, also when the unit's own directory builds it. Before item 1, with every
     grouping flat and `FlattenSingleTier: true`, a bundle whose one augmenter application adds
     a generator collapsed into the shared directory; another merged bundle's patch that
     targeted that ConfigMap was accepted, although the shared Kustomization built the ConfigMap
     and applied the patch to it. Since item 1 `FlattenSingleTier` collapses no directory that
     renders a bundle, so that input no longer reaches the gap; whether another one does is
     part of this item.
   - Expected: the check sees every object the unit's directory builds, generated ConfigMaps
     included, with a test that pins it.
6. **A kept Kustomization is checked in whatever form it has.** Shipped.
   - Under the two integrated placements, the checks that look at a layout's Flux Kustomizations
     read both the Kustomizations the integration generates and the ones it keeps in place of
     its own through one helper, `generatedKustomizations` in `layout_integrator.go`, which
     returns them in typed form. Those checks are: a Source once per kustomize build
     (`integratedPlacement.hostSourcesOncePerBuild`), the root build leaving the Sources it
     hosts unchanged (`integratedPlacement.checkRootBuildKeepsHostedSources`) and the reconcile
     order of placed objects (`checkPlacedReconcileOrder`).
   - A kept Kustomization is therefore held to the same refusals whether the caller supplied it
     typed, unstructured or inside a `List`. One that cannot be read as a Flux Kustomization is
     refused with an error naming the layout and the object's namespace and name, and the
     caller's tree is left as it was. Which Kustomizations count as kept did not change, and
     `FluxSeparate` is not affected.
   - Tests: `pkg/stack/fluxcd/kept_forms_test.go` covers each check and the refusal under the
     integrated placements, with the kept Kustomization typed, unstructured and inside a `List`.
7. **`GenerateFromLayout` on a per-layout tree returns an incomplete set.** Target.
   - Current: `ResourceGenerator.GenerateFromLayout` (`resource_generator.go`) generates one
     Kustomization per directory that renders bundles. In a tree walked with
     `FluxIntegratedPerLayout` the writers do not list a directory child in its parent's
     `kustomization.yaml` (`childEntry` in `writerplan.go`), and the Kustomization of such a
     child (an application, augmenter or bundle-less node layout) is created only by the
     integrator. A caller who walks with those rules, calls `GenerateFromLayout` and writes the
     tree gets child directories that nothing applies, without an error. `GenerateFromCluster`
     refuses that placement (item 3).
   - Expected: the call refuses a per-layout tree with an error that points to the integrator,
     or generates the per-layout Kustomizations too. The choice follows items 1 and 2, which
     rework those functions.
8. **The walk validates its rules.** Shipped.
   - Before: `LayoutRules.Validate` had no caller outside tests. An unknown grouping, `FilePer`,
     placement or file-naming value was walked as if it were another value, and a `ClusterName`
     with a `..` segment was refused only by the writers.
   - Now: `WalkCluster` and `WalkClusterByPackage` (`walker.go`) and the Flux
     `LayoutIntegrator.IntegrateWithLayout` (`layout_integrator.go`) run `LayoutRules.Validate`
     (`pkg/stack/layout/types.go`) on the rules as given, before anything else, so invalid rules
     are an error for a nil cluster too. `CreateLayoutWithResources` and `GenerateFromCluster` of
     both engines get the check from the walk; none checks on its own.
   - `Validate` refuses a `ClusterName` with a `..` path segment, also one the walk would clean
     away (`x/../platform`). Every `ClusterName` it accepts is one the writers write.
   - Tests: `TestLayoutRules_Validate_ClusterName`, `TestWalk_RefusesInvalidRules` and
     `TestWalk_AcceptedClusterNameIsWritten` in `pkg/stack/layout/rules_validation_test.go`;
     `TestLayoutIntegrator_RefusesInvalidRules` in `pkg/stack/fluxcd/layout_integrator_test.go`;
     `TestCreateLayoutWithResources_RefusesInvalidRules` in `pkg/stack/argocd/argo_test.go`.
9. **The bootstrap directory does not follow the layout rules.** Target.
   - Current: both bootstrap modes point Flux at the root node's name (`bootstrapDir`); the
     bootstrap is given the root node, not the rules or the tree. The walked tree's top is
     elsewhere for an unnamed root without `ClusterName` (`cluster/`) and for a `ClusterName`
     whose directory is not the root's name (section 1.2).
   - Expected: the directory the bootstrap applies is the top directory of the written tree.
     That needs the bootstrap to learn the directory, an API change; its form is decided after
     items 1 and 2.
10. **The rules carry no application file mode.** Shipped.
    - Before: `LayoutRules.ApplicationFileMode` was validated, but no walk applied it, so
      `AppFileSingle` in the rules changed nothing.
    - Now: the field is gone from `LayoutRules`. One file per application stays available
      through `Config.ApplicationFileMode` (`pkg/stack/layout/config.go`) for `WriteManifest`,
      and through a layout's own `ManifestLayout.ApplicationFileMode` (`manifest.go`).
    - Breaking: a caller that set the field gets a compile error. Output does not change.
    - Tests: `TestLayoutRules_HasNoApplicationFileMode` in `pkg/stack/layout/types_test.go`.

**Acceptance.** Each target has a test rendering the input above and asserting the expected tree;
the shipped items name theirs.

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
3. The fluxcd README says which layouts get a Kustomization under `FluxIntegratedPerLayout`
   (one per layout that renders bundles, one per bundle-less child layout that is neither an
   umbrella child nor `AppFileSingle`), that a layout whose Kustomization name another generated
   Kustomization uses is refused, and that one already in the tree with that name, in the
   namespace of the generated ones, is kept when it has the same host and `spec.path`. It keeps
   today's names; the
   naming from [go-kure/kure#973](https://github.com/go-kure/kure/issues/973) is restated there
   when that ships.
4. The FileNaming doc: nothing was left to do,
   [go-kure/kure#976](https://github.com/go-kure/kure/issues/976) made it true and updated it.
5. The helm README and the example it is generated from say that kure does not convert a
   `HookGroup`, and that a consumer builds a Kustomization per group itself.
6. The `ManifestLayout.DependsOn` comment, the layout README and the Flux workflow guide state
   which layouts it applies to, that it is dropped elsewhere without an error, and that its
   entries are Kustomization names copied verbatim.

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
