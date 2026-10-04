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
  base kind is its responsibility; the kinds without a constructor today are listed under
  [go-kure/kure#981](https://github.com/go-kure/kure/issues/981).
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

Line references are to `v0.2.0-beta.15` and will drift as the code changes.

### 1.1 The model

| Type | Fields that decide names and placement | Source |
|---|---|---|
| `stack.Node` | `Name`, `Children`, `Bundle` (one per node), `PackageRef`. No dependency, Kustomization or directory field. | `pkg/stack/cluster.go:102-119` |
| `stack.Bundle` | `Name`, `DependsOn []*Bundle`, `NamedDependsOn`, `Children` (umbrella), `SourceRef`, `Interval`, `Prune`, `Wait`, `Timeout`, `RetryInterval`, `Force`, `Suspend`, `HealthChecks`, `Patches`, `PostBuild`, `Labels`, `Annotations`. No field for the Kustomization's name, namespace or directory. | `pkg/stack/bundle.go:24-83` |
| `stack.Application` | `Name`, `Namespace`, `Config`. No delivery or dependency field. | `pkg/stack/application.go:10-14` |

`layout.WalkCluster` turns the model into a `ManifestLayout` tree. `fluxcd.LayoutIntegrator` adds
Flux objects to that tree in one of three placements: `FluxSeparate` (the default),
`FluxIntegratedPerBundle` and `FluxIntegratedPerLayout`. The writers (`WriteToDisk`,
`WriteToTar`, `WriteManifest`) check the tree and write it.

### 1.2 Directories and files

| What | Rule | Source |
|---|---|---|
| Layout directory | `FullRepoPath() = Namespace/Name` | `pkg/stack/layout/manifest.go:101-107` |
| Cluster wrapper | `ClusterName "."`: none. `"prod"`: `prod/`. `""` with a named root: `<root>/`. `""` with an unnamed root: `cluster/`. | `pkg/stack/layout/walker.go:106-196` |
| Node directory | node name, nested by tree (`NodeGrouping: GroupByName`, the default). `GroupFlat` merges descendant nodes' bundles into the first-level node's directory. | `walker.go:259-312` |
| Bundle directory | none by default: the bundle renders into its node's directory. `BundleGrouping: GroupByName` adds `<node>/<bundle name>`. | `walker.go:317-344` |
| Umbrella child directory | `<parent dir>/<child bundle name>`, marked `UmbrellaChild` | `walker.go:353-376` |
| Application directory | none by default. `ApplicationGrouping: GroupByName` adds `<bundle dir>/<app name>`. Under the default flat grouping an application whose config is a `LayoutAugmenter` still gets its own directory, unless the config also implements `LayoutIntentAugmenter` and `WantsOwnLayout()` returns false; it is then merged like any other application. | `walker.go:386-421`, `:452-469` |
| Resource file | `WriteToDisk` and `WriteToTar` name by the layout's own FileNaming: default `{namespace}-{kind}-{name}.yaml` (empty namespace: `cluster`); `FileNamingKindName` gives `{kind}-{name}.yaml`. `WriteManifest` names every layout's files from its `Config` instead. | `pkg/stack/layout/config.go:62-70`, `writerplan.go:228-250`, `:299-306` |
| Generated Kustomization file | under `WriteToDisk` and `WriteToTar`, named by the host layout's FileNaming; the `flux-system/` layout of `FluxSeparate` has no FileNaming, so it always uses the default (`flux-system-kustomization-<name>.yaml`). Layouts an augmenter adds do not inherit FileNaming. | `manifest.go:88-95`, `pkg/stack/fluxcd/layout_integrator.go:1384-1389` |
| `kustomization.yaml` child entries | a child directory is listed unless it is an `UmbrellaChild`, renders a bundle, or the parent is `FluxIntegratedPerLayout` | `writerplan.go:69-89` |

`LayoutRules.FlattenSingleTier` (default false) collapses one layer, and only at the walked root:
the root must have a namespace without `/`, exactly one child, no resources of its own, and that
child must be a terminal, non-umbrella layout (`pkg/stack/layout/flatten.go:27-95`, called at
`walker.go:127` and `:141`). It does not reach an application directory deeper in the tree.

### 1.3 Flux Kustomizations: name, host and listing

**Names.**

- A bundle's Kustomization is named `Bundle.Name`, in the generator's `DefaultNamespace`
  (`pkg/stack/fluxcd/resource_generator.go:496-497`).
- Several bundles in one directory (a `GroupFlat` merge) share one Kustomization named after the
  first bundle (`resource_generator.go:134-145`; `pkg/stack/layout/origin.go:166-181`).
- Under `FluxIntegratedPerLayout`, a bundle-less node layout gets a Kustomization named
  `<path with / replaced by ->-node`, and an application or augmenter layout one named after the
  layout (`layout_integrator.go:1205-1214`).

**Where each placement puts them.**

| Placement | Bundle Kustomization hosted in | Extra Kustomizations | Host's `kustomization.yaml` lists |
|---|---|---|---|
| `FluxSeparate` | `flux-system/` directly under the walked root's directory | none | `flux-system/` lists the CR files; the root lists `flux-system` |
| `FluxIntegratedPerBundle` | the parent of the bundle's directory; the walked root hosts its own (`layout_integrator.go:938-943`) | none | the CR files |
| `FluxIntegratedPerLayout` | as `FluxIntegratedPerBundle` | one per bundle-less child layout (`layout_integrator.go:892-923`) | the CR files only, never a child directory |

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
| `LayoutIntegrator` (all placements), `GenerateFromLayout` | the directory of the layout that renders the bundle: `applications/shop/shop-infra` | `resource_generator.go:139` |
| `GenerateForBundle(b, path)` | `path` verbatim | `resource_generator.go:445-468` |
| `GenerateFromCluster(c)` | walks with `DefaultLayoutRules()`, so the consumer's layout rules, `ClusterName` included, are ignored: `cluster/applications/shop` | `resource_generator.go:64-77` |
| `FluxIntegratedPerLayout` node Kustomization | the layout's directory | `resource_generator.go:645-671` |
| Bootstrap, gotk mode | `manifests/<root>` | `pkg/stack/fluxcd/bootstrap_generator.go:322` |
| Bootstrap, FluxInstance `sync.path` | `./<root>`, or `./` for an unnamed root | `bootstrap_generator.go:414-417` |

### 1.5 Bundle fields on the Kustomization

From `kustomizationForBundle` (`resource_generator.go:476-629`):

| Bundle field | Kustomization field | When unset |
|---|---|---|
| `Interval` | `spec.interval` | 60m (`pkg/stack/fluxcd/defaults.go:124`) |
| `Prune` | `spec.prune` | `false`, always written (`defaults.go:144-146`) |
| `Wait` | `spec.wait` | omitted (`defaults.go:155-157`) |
| `Timeout`, `RetryInterval`, `Force`, `Suspend` | same | omitted |
| `SourceRef` | `spec.sourceRef` | namespace omitted when empty |
| `Children` (umbrella) | one `healthChecks` entry per child, in the generator's namespace; `wait` is not set | `resource_generator.go:552-570` |
| `HealthChecks`, `Patches`, `PostBuild` | same | omitted |
| `DependsOn`, `NamedDependsOn` | `spec.dependsOn[].name` | omitted |

Under the integrator, dependencies and health checks that name a bundle are mapped to the
Kustomization of the directory that renders it, and a reference to its own directory is dropped
(`resource_generator.go:171-196`). Bundles that share a directory must agree on every reconcile
setting (`resource_generator.go:213-260`).

The umbrella health checks exist only in generator output: `pkg/stack` exports nothing that returns
them.

### 1.6 Sources, the flux-system directory and bootstrap

**Sources.**
- A Source (`OCIRepository`, `GitRepository`) is generated only when `SourceRef.URL` is set
  (`resource_generator.go:676-709`).
- `FluxSeparate` puts it in `flux-system/`.
- The integrated placements put it in the root node's layout (`layout_integrator.go:1022-1032`).
  When the root node renders a bundle, that layout is the bundle's own directory.

**The `flux-system` directory.** Its name is a constant, and under `FluxSeparate` it is placed
directly under the walked root (`layout_integrator.go:1384-1389`).

**Bootstrap, flux-operator mode (the default).**
- Emits the embedded flux-operator install bundle (`v0.58.1`) and a `FluxInstance` named `flux`.
- `distribution.registry` and `distribution.version` are written verbatim, including as empty
  strings.
- `sync` is emitted only when `SourceURL` is set.

**Bootstrap, gotk mode.** Emits the vendored components, a Kustomization at
`manifests/<root>`, and a Source when `SourceURL` is set (`bootstrap_generator.go:104-131`,
`:310-332`).

### 1.7 Collision guards

| Input | Result | Source |
|---|---|---|
| Two bundles with the same name | refused wherever the origin index is built: the integrator under every placement, `GenerateFromLayout` and the ArgoCD workflow. `WalkCluster` and the writers alone do not check it. | `origin.go:122-123`; callers `layout_integrator.go:306`, `resource_generator.go:90`, `pkg/stack/argocd/argo.go:68` |
| A payload Kustomization named like its bundle, in the namespace of the generated Kustomization (the generator's `DefaultNamespace`) | refused under every placement. The check compares namespace and name, so it does not refuse the same name in another namespace. | `layout_integrator.go:1009-1014`, `:1183-1188`, `:1328-1330`; `resource_generator.go:495-498` |
| A Kustomization name used twice | refused by the integrator | `layout_integrator.go:1198-1204` |
| `FluxIntegratedPerLayout` with `ApplicationGrouping: GroupByName`, application named like its bundle (the common case) | refused as a name used twice | same |
| `FluxIntegratedPerLayout`, augmenter application named like its bundle | refused as a name used twice | same |
| Two directory layouts resolving to one directory | refused by the writers | `pkg/stack/layout/treecheck.go:63-69` |
| Two single-file layouts (`AppFileSingle`) in one directory | accepted when their file names differ; refused when they resolve to the same file | `pkg/stack/layout/treecheck.go:53-62` |
| Dotted names, names over 63 characters | accepted unchanged | none |
| A hand-built tree with the same Kustomization twice in one layout, or in layouts one kustomize build includes | refused by the writers, as for any object held twice | `treecheck.go:74`, `:365-378` (one layout), `:111`, `:139` (one build) |
| A hand-built tree with the same Kustomization in layouts that separate builds apply | written: the writers have no tree-wide check of Kustomization names | none |
| An `UmbrellaChild` layout no Kustomization applies | written, and listed by nobody | none |

### 1.8 What a consumer cannot control today

1. **The name of a bundle's Kustomization.** Only by naming the bundle; a merged directory takes its
   first bundle's name; `-node` names have no parameter.
2. **The directory that hosts it.** The only lever is the placement.
3. **A directory name separate from the Kustomization name.** An umbrella child's directory and its
   Kustomization are both the child bundle's name (`walker.go:358`, `resource_generator.go:496`).
   Renaming the walked layout before `IntegrateWithLayout` separates them and is accepted, but no
   document or test covers that route.
4. **Ordering between groups, from the model.** A group gets a Kustomization only under
   `FluxIntegratedPerLayout`, with a fixed name, and `Node` has no dependency field. The one route
   to a `dependsOn` is on the layout, not the model: set `DependsOn` (Kustomization names, as
   strings) on the walked group layout before `IntegrateWithLayout`, which copies it
   (`layout_integrator.go:892-923`, `resource_generator.go:666-669`).
5. **Engine annotations per application.** There is no field for prune protection or force replace,
   so a consumer writes Flux annotations onto objects itself.

What a consumer can do today:
- **Ordering inside an application** is expressible, as umbrella children with `DependsOn`.
- **A layout without Kustomizations** is available through `WalkCluster` and the writers. No parent
  then lists a bundle directory, so nothing applies the payload.

### 1.9 Documentation that disagrees with the code

1. **Stale line references.** The `normalizeRulesPlacement` comment cites
   `pkg/stack/layout/types.go:154-163` and `walker.go:42-43`
   (`layout_integrator.go:1410-1413`); neither is the code it names.
2. **SourceRef message.** It says "FluxIntegratedPerLayout mode requires a SourceRef" when
   `FluxIntegratedPerBundle` triggers it too (`pkg/stack/fluxcd/validate.go:50`, reached for both
   placements from `layout_integrator.go:217-221`).
3. **The "every layout" claim.** The fluxcd README says `FluxIntegratedPerLayout` gives a
   Kustomization to every layout, augmenter layouts included. An augmenter application named like
   its bundle is refused instead.
4. **FileNaming.** `LayoutRules.FileNaming` is documented as the naming for manifest files
   (`types.go:138-140`). Under `WriteToDisk` and `WriteToTar` it does not reach `flux-system/` or
   augmenter layouts.
5. **Helm hook groups.** The helm README says each hook group "becomes one FluxCD Kustomization,
   deployed in order". Nothing in kure converts a `HookGroup`.
6. **`ManifestLayout.DependsOn`.** It is documented as becoming `spec.dependsOn` in
   `FluxIntegratedPerLayout` mode (`manifest.go:44-48`). Left unsaid: that holds only for a layout
   that gets its own Kustomization, and every other case drops the field without an error.

## Part 2: target behaviour

Each section names the change, the target, a design outline and the acceptance criteria. The
linked issue tracks the implementation.

### Kustomization name separate from the bundle name ([go-kure/kure#971](https://github.com/go-kure/kure/issues/971))

**Target.** A consumer can name a bundle's Kustomization without renaming the bundle.

**Design outline.**

- **New field:** `Bundle.KustomizationName string`; empty means `Bundle.Name`. The same name is the
  identity of the ArgoCD Application generated for that directory (`origin.go:123` treats them as
  one identity), so the ticket settles whether the field gets an engine-neutral name.
- **Every place that now reads `Bundle.Name` as a Kustomization name** reads the effective name:
  - `kustomizationForBundle` (`resource_generator.go:496`);
  - the umbrella health checks, which name each child (`:557-570`);
  - unit naming (`UnitName`, `UnitOfName`, `origin.go:166-181`);
  - dependency translation (`UnitDependencies`, `origin.go:187-197`; `resource_generator.go:171-196`,
    `:616-626`);
  - the ArgoCD Application name (`pkg/stack/argocd/argo.go:107`).
- **Name lookups:** the origin index keys its by-name map on the effective name, so
  `NamedDependsOn` and health checks name Kustomizations, never bundles.
- **Uniqueness:** checked on the effective name (`origin.go:123`). `Bundle.Name` stays unique as
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
  (`walker.go:358`) and the `BundleGrouping: GroupByName` bundle layout (`walker.go:317-344`). A
  bundle rendered into its node's directory has no directory of its own; that directory is
  `Node.Name`.
- **Unchanged:** `spec.path` keeps following `FullRepoPath`, and the duplicate-directory check
  stays. Its message names the two bundles; today it prints the two layouts' paths, which can be
  the same path twice (`treecheck.go:64-67`).
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
  (`resource_generator.go:645-671`). `KustomizationName` replaces the fixed `<path>-node` name
  (`layout_integrator.go:1209-1214`) when set.
- **Placements:** node-level Kustomizations exist only under `FluxIntegratedPerLayout`
  (`layout_integrator.go:892-923`), and only for a node whose layout renders no bundle (`:894`).
  Under the other placements, on a node that `NodeGrouping: GroupFlat` merges away, and on a node
  whose layout renders a bundle, setting these fields is refused, not silently ignored.
- **Unset fields change nothing for nodes:** a node Kustomization keeps its `<path>-node` name.
- **Collisions:** a node `KustomizationName` equal to a bundle's effective Kustomization name is
  refused, as any name used twice is today (`layout_integrator.go:1198-1204`).
- **Reconciliation settings:** a node Kustomization takes the generator's interval and prune and
  sets no `wait` today (`resource_generator.go:645-671`). The ticket decides whether a node
  carries its own.
- **Application and augmenter layouts, a breaking rename:** under `FluxIntegratedPerLayout` they
  get Kustomizations named after the layout (`layout_integrator.go:1209-1214`). These collide with
  the bundle's whenever the application shares the bundle's name (section 1.7). The default
  becomes `<unit name>-<layout name>`, which renames these Kustomizations even when no new field
  is set. Changing with it:
  - sibling `dependsOn` entries, which name layouts (`ManifestLayout.DependsOn`, copied at
    `resource_generator.go:666-669`), are translated to the new names;
  - the test that pins the old names (`pkg/stack/fluxcd/layout_integrator_test.go:707-718`) and
    the README sentence "The CR is named after the layout" (`pkg/stack/fluxcd/README.md:650`).
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
  `renderApps`, `walker.go:386-421`). The layout package stays engine-neutral.
- **The Flux mapping:** `fluxcd.LayoutIntegrator` sets `kustomize.toolkit.fluxcd.io/prune: disabled`
  and `kustomize.toolkit.fluxcd.io/force: enabled` on those objects.
- **Refusal leaves no annotation behind:** a refused integration restores the objects. The
  existing restore does not do that by itself: `saveLayouts` (`layout_integrator.go:110-137`)
  clones the slices but keeps the same object pointers, so an annotation set in place would
  survive. The integrator therefore snapshots the annotations it changes, or annotates copies.
- **Conflicts:** an object that already carries the annotation with another value is refused.
- **Generated ConfigMaps:** a ConfigMap built by a `configMapGenerator` has no object to annotate.
  Its spec is a name and a file list (`manifest.go:80-83`), and the walker holds only a stand-in
  for it (`walker.go:415-417`). The ticket decides between writing the generator's
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

**Target.** flux-operator mode no longer writes `registry: ""` or `version: ""`.

**Design outline.** Building a FluxInstance returns a validation error naming the field when
`Registry` or `FluxVersion` is empty. Two entry points build one, and both validate:
`GenerateBootstrap` in flux-operator mode, through `generateFluxOperatorBootstrap`
(`bootstrap_generator.go:74-93`, `:146`), and the public `GenerateFluxInstance` (`:386-396`). Both
call `generateFluxInstance` (`:399-434`), which returns no error today and gains one.
gotk mode is unchanged. It reads both fields too, but an empty value is valid there: an empty
`FluxVersion` builds from the vendored version without a download, and an empty `Registry` keeps
the default (`:187-208`).

**Acceptance.** Through both entry points, each empty field is refused with its name; set values
still render verbatim.

### FileNaming applied everywhere ([go-kure/kure#976](https://github.com/go-kure/kure/issues/976))

**Target.** `LayoutRules.FileNaming` names every file `WriteToDisk` and `WriteToTar` write for
that tree.

**Design outline.**

- The `FluxSeparate` `flux-system/` layout takes the rules' FileNaming
  (`layout_integrator.go:1384-1389`).
- Layouts an augmenter adds inherit their parent's FileNaming when they leave it unset.
- This is done in the walker, after the augmenter runs (`augmentAppLayout`, `walker.go:438-451`).
  `resolveManifestFileName` (`manifest.go:88-95`) has no parent to read.
- `WriteManifest` already names every layout's files from its `Config`, so the gap exists only
  under `WriteToDisk` and `WriteToTar`.

**Acceptance.** With `FileNamingKindName`, no file in `flux-system/` or in an augmenter layout is
named `{namespace}-{kind}-{name}.yaml`, unless that layout sets its own FileNaming.

### The writers validate a Flux-delivered tree ([go-kure/kure#977](https://github.com/go-kure/kure/issues/977))

**Target.** The writers refuse a Flux-built tree in which something is written that nothing
applies, and any tree in which two Kustomizations share a name.

**Design outline.** Two new refusals in `checkLayoutTree` (`treecheck.go:46`, called from
`manifest.go:216`, `write.go:32`, `tar.go:23`):

- **An unapplied `UmbrellaChild`:** when the root is marked `SetFluxBuild` (the integrator marks it
  whenever it generated a Kustomization, `layout_integrator.go:346-378`), every `UmbrellaChild`
  layout must be marked too, meaning some Kustomization's `spec.path` names it. ArgoCD trees are
  not marked and are unaffected. Their placement cannot tell them apart, because the ArgoCD walk
  uses `FluxSeparate` (`pkg/stack/argocd/argo.go:168-172`). A consumer that places its own
  Kustomizations marks its tree the same way.
- **A duplicate Kustomization namespace/name anywhere in the tree.** Today the writers refuse one
  object held twice only within a layout or within one kustomize build (section 1.7); two
  Kustomizations of one name in separately applied directories pass. The layout package cannot
  import fluxcd (fluxcd imports layout), so the integrator's `claim`
  (`layout_integrator.go:1198-1204`) cannot be reused. The check matches the group
  `kustomize.toolkit.fluxcd.io`, kind `Kustomization`, by GVK.

**Acceptance.**
- A marked tree with an unapplied `UmbrellaChild` is refused, naming the layout.
- A hand-built tree with two Kustomizations of one namespace/name is refused, naming both
  directories.
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
  `filepath.Join` (`manifest.go:101-107`), which treats `\` as a separator on Windows: a name such
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
     `kustomization.yaml` lists it (`layout_integrator.go:938-943`). `FluxSeparate` does the same
     through `<root>/flux-system/`. The bootstrap applies the root, which then applies itself:
     two owners of one directory.
   - Expected: every directory has exactly one owner. Either the root renders no bundle (its
     bundle moves into a child directory), or the root bundle's settings go to the bootstrap and
     no Kustomization is generated for it. The ticket chooses one.
2. **An integrated Source is hosted inside the directory it delivers.**
   - Current: when the root node renders a bundle, the Source lands in that bundle's directory
     (`layout_integrator.go:1022-1032`), so the Kustomization that needs the Source is the one that
     would apply it.
   - Expected: a Source is hosted in a build that is applied before any Kustomization that
     references it, never inside a directory delivered through it.
3. **`GenerateFromCluster` ignores the consumer's layout rules.**
   - Current: it walks with the default rules (`resource_generator.go:64-77`), so its paths disagree
     with the layout a consumer writes.
   - Expected: it takes the consumer's `LayoutRules`, or is removed in favour of `WalkCluster` plus
     `GenerateFromLayout`.
4. **The gotk bootstrap path differs from the FluxInstance sync path.**
   - Current: `manifests/<root>` (`bootstrap_generator.go:322`) against `./<root>` (`:414-417`).
   - Expected: both modes point at the same directory for the same root.

**Acceptance.** Each case has a test rendering the input above and asserting the expected tree.

### Documentation corrections ([go-kure/kure#980](https://github.com/go-kure/kure/issues/980))

**Target.** Section 1.9 corrected:

1. the stale line references;
2. the SourceRef message, which names the placement actually in use;
3. the "every layout" claim, restated with the naming from
   [go-kure/kure#973](https://github.com/go-kure/kure/issues/973);
4. the FileNaming doc, which becomes true with
   [go-kure/kure#976](https://github.com/go-kure/kure/issues/976) (if that lands first, nothing
   remains here);
5. the hook-group sentence in the helm README, rewritten as what a consumer may build;
6. `ManifestLayout.DependsOn`, stating which layouts it applies to and that it is dropped elsewhere.

The fluxcd README claims tied to the behaviour bugs change with
[go-kure/kure#979](https://github.com/go-kure/kure/issues/979).

**Acceptance.** No sentence in section 1.9 remains true of the code after the change.

### Builders for missing base kinds ([go-kure/kure#981](https://github.com/go-kure/kure/issues/981))

**Target.** kure supports every base object with its full spec. Each kind below gets a generated
`Create<Kind>` from its registered scheme (`mise run builders:generate`).

**Design outline.**

| Kinds | Upstream API | New dependency |
|---|---|---|
| `PriorityClass` (scheduling/v1), `EndpointSlice` (discovery/v1), `Lease` (coordination/v1), `RuntimeClass` (node/v1) | `k8s.io/api` | no |
| `MutatingWebhookConfiguration`, `ValidatingWebhookConfiguration`, `ValidatingAdmissionPolicy`, `ValidatingAdmissionPolicyBinding`, `MutatingAdmissionPolicy`, `MutatingAdmissionPolicyBinding` (admissionregistration/v1; registering the group version brings all six) | `k8s.io/api` | no |
| `APIService` | `k8s.io/kube-aggregator` | yes |
| `VerticalPodAutoscaler` | `k8s.io/autoscaler/vertical-pod-autoscaler` | yes; not an upstream core API |
| `ImageRepository`, `ImagePolicy` | `github.com/fluxcd/image-reflector-controller/api` | yes; kure has only image-automation-controller today |

The ticket decides each new dependency.

**Acceptance.** Every kind above that the `k8s.io/api` version in use provides has a constructor
that sets identity only, covered by the whole-object identity test, and appears in the generated
kind tables. The family README and every guide mapped to the family are updated in the same
change, as the repository requires for a new kind (`AGENTS.md:220-222`). For `APIService`,
`VerticalPodAutoscaler`, `ImageRepository` and `ImagePolicy` the ticket decides per kind; a kind
that is added meets the same criteria, and a kind that is not is listed with its reason.
