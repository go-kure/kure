+++
title = "Generating Flux Manifests"
weight = 20
+++

# Generating Flux Manifests

This guide walks through the complete workflow for generating a GitOps repository structure with Flux resources using Kure.

## Overview

The workflow has four stages:

1. **Define** the cluster topology using the domain model
2. **Select** the Flux workflow engine
3. **Generate** Flux resources and directory layout
4. **Write** manifests to disk

The code on this page imports:

<!-- doc-example:excerpt the import block alone, which the examples below share -->
```go
import (
    "github.com/go-kure/kure/pkg/stack"
    "github.com/go-kure/kure/pkg/stack/fluxcd"
    "github.com/go-kure/kure/pkg/stack/layout"
)
```

Every other Go block on this page is the body of an `Example` function in
`pkg/stack/fluxcd`, which `go test` runs: the Kustomization-name, bootstrap and sync-name blocks
are shared with the [Flux Engine reference](/api-reference/flux-engine) and live in its
`example_test.go`, the others in `flux_workflow_example_test.go`. Besides the imports above they use `os`, `kustv1`
(`github.com/fluxcd/kustomize-controller/api/v1`), and `fmt` for the lines that print what the
example built. The test file declares what the examples take as given: `certManagerConfig`,
`frontendConfig` and `apiConfig`, which each emit a Deployment and a Service named after their
application; `productionCluster()` and `productionLayout()`, the cluster Step 1 builds and the
layout Step 3 creates; and `printFiles(dir)`, which prints every file below `dir`.

## Step 1: Define the Cluster

Define your cluster's structure. A cluster has one root node, a node holds at most one bundle, and
a bundle holds applications, so the two groups here are two child nodes of the root, each with a
bundle of its own:

<!-- doc-example: pkg/stack/fluxcd Example_fluxWorkflowDefine -->
```go
// The Flux source every bundle's Kustomization reads from.
source := &stack.SourceRef{Kind: "GitRepository", Name: "flux-system"}

certManager, err := stack.NewBundle("cert-manager", []*stack.Application{
    stack.NewApplication("cert-manager", "cert-manager", certManagerConfig),
}, nil)
if err != nil {
    panic(err)
}
certManager.SourceRef = source
webTier, err := stack.NewBundle("web-tier", []*stack.Application{
    stack.NewApplication("frontend", "web", frontendConfig),
    stack.NewApplication("api-gateway", "web", apiConfig),
}, nil)
if err != nil {
    panic(err)
}
webTier.SourceRef = source

cluster := stack.NewCluster("production", &stack.Node{
    Name: "production",
    Children: []*stack.Node{
        {Name: "infrastructure", Bundle: certManager},
        {Name: "applications", Bundle: webTier},
    },
})
for _, node := range cluster.Node.Children {
    b := node.Bundle
    fmt.Println(node.Name, b.Name, len(b.Applications), b.SourceRef.Kind, b.SourceRef.Name)
}
```
<!-- doc-example:end -->

Each bundle becomes a Flux Kustomization, and each application generates its Kubernetes manifests.
Each bundle needs a `SourceRef` naming the Flux source its Kustomization reads from: `FluxSeparate`,
the placement Step 3 uses, does not enforce one, and without it the Kustomizations written to
`flux-system/` carry an empty `sourceRef`, which Flux's CRD rejects.

Names are checked when the cluster is validated, which the layout walk of Step 3 does before it
renders anything. A bundle name must be a DNS-1123 subdomain: lower-case letters, digits, `-` and
`.`, starting and ending with a letter or digit (`my.app` is valid, `My-App` is not). A node name
must be one path segment: not empty, not `.` or `..`, and without `/`, `\` or a NUL byte; only the
root node may be unnamed. In the Flux workflow a bundle name also becomes the name of its Flux
Kustomization, so it must be at most 63 characters: Flux writes the name into a label value on
every object it applies. That limit is Flux's, so cluster validation does not apply it; the Flux
generator refuses a longer name when it builds the Kustomization, which is still before anything
is written. A refused name is reported with its node or bundle path, and Kure never shortens one.
See the [Stack reference](/api-reference/stack/) for the rules in full.

The fluent builder (`stack.NewClusterBuilder`) builds a single path from the root, not a tree
like this one: `WithNode` sets the root node, so a second call replaces it; a second `WithBundle`
on one node replaces the first; and `WithChild` descends into the child it adds, with no way back
to its parent. Build a tree with sibling nodes from `stack.Node` values, as above.

## Step 2: Create the Flux Engine

<!-- doc-example: pkg/stack/fluxcd Example_fluxWorkflowEngine -->
```go
engine := fluxcd.Engine()
fmt.Println(engine.GetName())
```
<!-- doc-example:end -->

Placement is configured per call on `layout.LayoutRules.FluxPlacement` (see Step 3
below). `FluxUnset` normalizes to `FluxSeparate`. See the
[Flux Engine reference](/api-reference/flux-engine) for configuration options.

## Step 3: Generate Resources with Layout

<!-- doc-example: pkg/stack/fluxcd Example_fluxWorkflowLayout -->
```go
cluster := productionCluster()
engine := fluxcd.Engine()

// Define layout rules
rules := layout.LayoutRules{
    NodeGrouping:        layout.GroupByName,
    BundleGrouping:      layout.GroupByName,
    ApplicationGrouping: layout.GroupByName,
    FilePer:             layout.FilePerResource,
    FluxPlacement:       layout.FluxSeparate, // Flux resources in separate tree
}

// Generate layout with Flux resources integrated
ml, err := engine.CreateLayoutWithResources(cluster, rules)
if err != nil {
    panic(err)
}
for _, child := range ml.(*layout.ManifestLayout).Children {
    fmt.Println(child.FullRepoPath())
}
```
<!-- doc-example:end -->

## Step 4: Write to Disk

<!-- doc-example: pkg/stack/fluxcd Example_fluxWorkflowWrite -->
```go
ml := productionLayout()
out, err := os.MkdirTemp("", "kure-flux-workflow")
if err != nil {
    panic(err)
}
defer func() { _ = os.RemoveAll(out) }()

err = layout.WriteManifest(out, layout.DefaultLayoutConfig(), ml.(*layout.ManifestLayout))
if err != nil {
    panic(err)
}
printFiles(out)
```
<!-- doc-example:end -->

`WriteManifest` writes under `<basePath>/<ManifestsDir>` (`<out>/clusters` here). Every Flux
Kustomization `spec.path` is a layout directory relative to that root, so root the Flux source
there. The example prints every file it wrote, and `go test` checks that list against the
Example's `// Output:` comment. The tree below draws the same files by hand; it is not generated:

```
clusters/
  production/
    kustomization.yaml
    flux-system/                  # FluxSeparate: every Flux Kustomization, one file each
      flux-system-kustomization-cert-manager.yaml
      flux-system-kustomization-web-tier.yaml
      kustomization.yaml
    infrastructure/               # node
      kustomization.yaml
      cert-manager/               # bundle, applied by its Flux Kustomization
        kustomization.yaml
        cert-manager/             # application
          cert-manager-deployment-cert-manager.yaml
          cert-manager-service-cert-manager.yaml
          kustomization.yaml
    applications/
      kustomization.yaml
      web-tier/
        kustomization.yaml
        frontend/
          web-deployment-frontend.yaml
          web-service-frontend.yaml
          kustomization.yaml
        api-gateway/
          web-deployment-api-gateway.yaml
          web-service-api-gateway.yaml
          kustomization.yaml
```

Every `kustomization.yaml`, the one in `flux-system/` included, is a kustomize file, not a Flux
Kustomization: it lists the manifests and subdirectories kustomize builds from its directory. The
Flux Kustomizations are the `flux-system-kustomization-*.yaml` files, which
`flux-system/kustomization.yaml` lists.

Those names are the default pattern, `{namespace}-{kind}-{name}.yaml`. Under `WriteToDisk` and
`WriteToTar`, `LayoutRules.FileNaming` names the files of every layout of the tree, `flux-system/`
and the layouts an augmenter adds included: with `FileNamingKindName` the directory holds
`kustomization-cert-manager.yaml` and `kustomization-web-tier.yaml`. Before go-kure/kure#976 those
two kinds of layout kept the default pattern whatever the rules said, so their files are renamed
for a tree written with `FileNamingKindName`; see the
[Layout Engine reference](/api-reference/layout/#file-naming-modes).

## Layout Configuration

The [Layout Engine](/api-reference/layout) supports multiple grouping and file organization strategies:

| Option | Values | Effect |
|--------|--------|--------|
| NodeGrouping | `GroupByName`, `GroupFlat` | A directory per child node, or merge child nodes into their parent |
| BundleGrouping | `GroupByName`, `GroupFlat` | A directory per bundle, or render bundles in their node's directory |
| ApplicationGrouping | `GroupByName`, `GroupFlat` | A directory per app, or write apps into their bundle's directory |
| FilePer | `FilePerResource`, `FilePerKind` | One file per resource or group by kind |
| FluxPlacement | `FluxSeparate`, `FluxIntegratedPerLayout`, `FluxIntegratedPerBundle` | Separate dir; a Flux CR per layout node; or Flux CRs at bundle boundaries with application children as directories |

The three grouping axes are independent; umbrella child bundles and augmenter applications always get
a directory of their own.

### Layout paths (breaking change in go-kure/kure#771)

A layout's directory is `FullRepoPath()`, which is `Namespace` joined with `Name`. `Namespace` is
always the **parent's** directory, and `"."` is the root of the tree. The walkers build every layout
this way. If you build `ManifestLayout` trees by hand, or add child layouts to a walked tree, set each
child's `Namespace` to its parent's `FullRepoPath()`.

What changed, and what to do:

- **Hand-built children.** `FullRepoPath()` used to drop `Name` when `Namespace` already ended with
  it. A caller that set a child's `Namespace` to the full path, including the child's own `Name`
  (`Namespace: "apps/web", Name: "web"`), now gets that name twice (`apps/web/web`). Pass the
  parent's path instead (`Namespace: "apps"`).
- **Same-name and suffix-sharing layouts nest.** A bundle and an application with the same name
  now give `web/web`. Before, they collapsed onto `web/`, and one of the two `kustomization.yaml`
  files replaced the other. The same applies to a name that merely ends another as a string: `oo`
  under `foo` now gives `foo/oo`.
- **No `ClusterName`, named root.** The root node now sits at `<root>` instead of `cluster/<root>`,
  and its child nodes stay at `<root>/<child>`. With `FluxSeparate`, the `flux-system/` layout now
  sits inside the root's directory, at `<root>/flux-system`, where the root's `kustomization.yaml`
  references it. Before, it was written beside the root, and that reference dangled. The ArgoCD
  `argocd/` layout moved the same way. Point anything outside kure that reads these directories
  (CI scripts, a hand-written sync path) at the new paths.
- **No `ClusterName`, unnamed root.** The root and its child nodes stay at `cluster/` and
  `cluster/<child>`. Deeper descendants move: a grandchild used to be written at
  `<child>/<grandchild>`, outside `cluster/` and referenced by nothing. It now sits at
  `cluster/<child>/<grandchild>`.
- **With a `ClusterName`.** Most output is unchanged, and `flux-system/` stays at
  `<ClusterName>/flux-system`. When the last segment of `ClusterName` is the root node's name
  (`ClusterName: "platform"`, root `platform`), the root is still the cluster directory. When
  `ClusterName` only ends with the root's name as a string (`ClusterName: "myprod"`, root `prod`),
  the old rule collapsed the root onto `myprod`. It now nests at `myprod/prod`, so move any
  external sync path that pointed at `myprod`.
- **Collisions are refused.** `WriteToDisk`, `WriteToTar` and `WriteManifest` check the whole tree
  before writing anything. They refuse two layouts that resolve to the same directory (compared
  case-insensitively), and two `AppFileSingle` layouts that resolve to the same file. They also
  refuse two layouts holding one object when one kustomize build takes in both: a
  `kustomization.yaml` and the child directories it lists, or the Flux build of a marked
  `KustomizationRecursive` directory. kustomize would refuse the second copy. The ConfigMap a
  `configMapGenerator` generates counts as one of those objects.
- **Empty Recursive targets are refused.** A marked `KustomizationRecursive` directory gets no
  `kustomization.yaml`. When no other file lands at or below it (a bundle with no applications,
  or applications that render nothing), every writer would leave it empty, and a Git tree drops
  an empty directory, so the Flux Kustomization would name a path that is not committed. Give the
  layout a resource, or write it `KustomizationExplicit`, which writes it `resources: []`.
- **Directories nothing applies are refused.** In a tree with generated Flux Kustomizations, a
  directory its parent's `kustomization.yaml` does not list is applied only by a Kustomization
  whose `spec.path` names it. The integrator marks the root and every directory its
  Kustomizations build (`SetFluxBuild`), so a tree it built passes. The check runs only when the
  root is marked: a marked tree changed afterwards is refused where such a directory is not
  marked, since its files would be committed and never applied. A tree you place Kustomizations
  in yourself is checked once you mark its root, and then needs the mark on each directory a
  Kustomization of yours builds.
- **Kustomization names are unique in a tree.** Two Flux Kustomizations with one namespace and
  name are refused wherever they sit, also in directories that are applied separately and inside
  a `List`: they are one object in the cluster.
- **No `..` in a layout's name or namespace.** Every writer refuses a layout whose `Name` or
  `Namespace` has a `..` path segment, with or without extra files.
- **The walk validates its rules.** `WalkCluster` and `WalkClusterByPackage` run
  `LayoutRules.Validate` on the rules they are given, and so does every entry point that walks
  (`CreateLayoutWithResources`, `GenerateFromCluster`), also for an absent or empty cluster. An
  unknown value of a grouping, `FilePer`, `FluxPlacement` or
  `FileNaming` is now an error; before, it was walked as if it were another value. A `ClusterName`
  with a `..` path segment is refused at the walk instead of at write, also one the walk would
  have cleaned away (`x/../platform`). An unset value is still valid and takes its default. Fix
  the value the error names. The Flux `IntegrateWithLayout` runs the same check in place of its
  own placement check, so its error for an unknown placement now names the field `FluxPlacement`
  (it was `fluxPlacement`).
- **`LayoutRules` has no `ApplicationFileMode`.** No walk ever applied it, so a valid value set
  there changed no layout and no file; the field is removed. Code that sets it no longer
  compiles: delete the line, the layouts and the files written stay the same. A `LayoutRules`
  value serialised as a whole no longer carries the key, and an unknown value there is no longer
  an error. To write an application as one file, set `Config.ApplicationFileMode` for
  `WriteManifest`, or the layout's own `ApplicationFileMode`.

See the [Layout Engine reference](/api-reference/layout/) for the full rule.

### Flux paths come from the layout (breaking change)

Every Flux Kustomization `spec.path` (and ArgoCD Application `source.path`) is now the directory of
the layout that renders the bundle, taken from the walked layout tree. Before, the generator
derived it from bundle names alone, so a node-level bundle's path was just its name and pointed at
a directory the layout never wrote whenever a node and its bundle were named differently, or a
`ClusterName` prefixed the tree. See
[Kustomization paths](/api-reference/flux-engine/#kustomization-paths) for the rule.

What changed, and what to do:

- **Removed APIs.** `GenerateFromBundle`, `GenerateFromNode`, `EngineWithMode`, `EngineWithConfig`,
  `NewWorkflowEngineWithConfig`, `SetKustomizationMode` and `ResourceGenerator.Mode` are gone. <!-- doc-api-refs:ignore removed in this release -->
  Generate from a walked layout (`ResourceGenerator.GenerateFromLayout`) or for one bundle at a path
  you supply (`GenerateForBundle`).
- **`GenerateFromCluster` takes the layout rules.** It walked with `layout.DefaultLayoutRules()`
  whatever rules the caller wrote the tree with, so with a `ClusterName` or other groupings its
  paths named directories the tree did not have. It now takes the rules as a second argument, on
  `stack.Workflow` and on both engines, and each path is a directory a walk with those rules
  writes. Pass the rules you write the tree with; `layout.DefaultLayoutRules()` keeps the paths it
  returned before. The Flux engine refuses rules with `FluxIntegratedPerLayout`: that tree's child
  directories are applied by Kustomizations only `CreateLayoutWithResources` places, so use that
  entry point for the placement. Rules the walk refuses are an error from both engines, whatever
  the cluster is: an absent or empty cluster returns nothing only with valid rules.
- **Integrate walked layouts only.** `IntegrateWithLayout` refuses a layout `layout.WalkCluster`
  did not build from the same cluster — build the tree with `WalkCluster` instead of by hand.
- **PerLayout hosts.** Under `FluxIntegratedPerLayout` a node bundle's CR now sits in the parent of
  the layout that renders it, like every other PerLayout CR, and the parent's `kustomization.yaml`
  lists it as a resource file. The writers no longer emit a `flux-system-kustomization-<child>.yaml`
  reference guessed from a child's name. A bundle-less node layout gets its own CR named
  `<path, "/" → "-">-node`.
- **Sources live in the root.** Under both integrated placements, every GitRepository or
  OCIRepository the integration derives from a `SourceRef` URL is written once, into the root
  node's layout (the directory the bootstrap sync path `./<root>` names, not a `ClusterName`
  wrapper above it). Before, each unit's parent layout held a copy, so a Source shared by several
  builds had several owners, and pruning one unit deleted a Source the others still used. Golden
  files move the Source to the root. `FluxSeparate` is unchanged.
- **Refusals.** Two bundles with one name, a CR name used twice, a node or bundle layout written as
  `AppFileSingle`, and a directory holding two objects with one identity are errors. So are a Flux
  Kustomization inside a `List` that takes another Kustomization's identity (kustomize builds a
  List's items); a Source identity (kind, namespace, name) defined with different content
  anywhere in the pass, in another layout or at another API version; and, under `FluxSeparate`,
  any Source already in the tree with the identity of one the integration generates into
  `flux-system`, even an identical one. Under `FluxIntegratedPerBundle`, a caller's copy of a
  generated Source in a `ClusterName` wrapper above the root node is refused too: that build also
  includes the root's copy, and kustomize refuses one object twice. When the root node renders a
  bundle, its Kustomization builds the directory the bootstrap applies, so a patch of it that
  applies to a Source the integration hosts there, or a postBuild substitution that changes one
  (with `substituteFrom` set, any `${...}` in the `SourceRef` URL reading a var the inline
  `substitute` does not set), is refused: the bootstrap
  applies that directory without either, and the two would keep overwriting each other's Source.
  Narrow the patch target or move the patch or postBuild to a bundle below the root node.
  A Kustomization already in the tree that the integration keeps in place of its own (same name
  and `spec.path`, in the layout that would host it) is held to the same refusals as a generated
  one, whether it is typed, unstructured or inside a `List`: two copies of a generated Source
  that the integration did not add, in the build of its directory; a root-build patch or
  postBuild that changes a hosted Source; and a cycle through its `dependsOn`, `wait` or health
  checks. One that cannot be read as a Flux Kustomization (a field of the wrong type, for one) is
  refused, naming its layout.
- **Grouping axes.** `NodeGrouping`, `BundleGrouping` and `ApplicationGrouping` are independent;
  they used to take effect only when bundles and applications were both flat, and a `ClusterName`
  always flattened the root bundle. Combinations that were silently rendered fully nested now
  render as configured, and a merged node's umbrella children and augmenter layouts keep their own
  directories and CRs.
- **One Kustomization per directory.** Bundles merged into one directory share one Kustomization
  (and one ArgoCD Application), named after the first of them; they must agree on the settings a
  Kustomization holds once, and dependency cycles are refused. A patch is refused there when its
  target would also select another merged bundle's objects: Flux patches everything the shared
  Kustomization builds, so the merge would widen the patch.
- **One owner per directory.** A directory that renders bundles is applied only by its own
  Kustomization: no parent `kustomization.yaml` lists it, in any placement. Under
  `FluxIntegratedPerBundle` a bundle's CR now sits in the parent directory (as under PerLayout),
  and a parent bundle can no longer depend on a child node's bundle there — the child's CR only
  exists once the parent has applied. Building a parent directory no longer includes its child
  units.
- **FlattenSingleTier** no longer rewrites Flux CRs a caller added to the tree; generated CRs
  already name the surviving directory.

### Kustomization names

A bundle's Kustomization is named after the bundle. Set `Bundle.KustomizationName` to give it
another name without renaming the bundle or moving its directory:

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

It prints `apps-shop clusters/prod/shop apps-db`: the Kustomization and its `dependsOn` entry carry
the names set on the bundles, and `spec.path` is the directory, which did not move. Everything
that refers to a bundle's Kustomization follows the name in effect (`Bundle.UnitName()`):
`dependsOn` entries, an umbrella's health checks, and the name a merged directory takes from its
first bundle. A `NamedDependsOn` entry names a Kustomization, so it reaches a bundle that sets the
field by that value, not by the bundle's name.

A `DependsOn` entry that is a copy of a bundle of the cluster (another `Bundle` value with the same
`Name`) is that bundle, and the dependency is on its Kustomization. Leave `KustomizationName` empty
on the copy or set the name of the bundle's Kustomization: a copy that sets another name is
refused, and so is a copy of a bundle whose Kustomization name is also in `NamedDependsOn`.

Two bundles whose Kustomizations would get one name are refused, whether the name comes from the
field or from the bundle's name. The field is also the way out when a bundle's payload contains a
Flux Kustomization named like the bundle: give the bundle another `KustomizationName` and the two
no longer collide. See the
[Flux Engine reference](/api-reference/flux-engine/#kustomization-names).

## Umbrella Bundles — Readiness Aggregation

A bundle with non-empty `Children` becomes an **umbrella**: Flux will only mark
its Kustomization `Ready` once every child Kustomization is `Ready`. The Flux
engine enforces this by prepending an auto `spec.healthChecks` entry for each
direct child.

`spec.wait` is not forced. It is `Bundle.Wait` like any other input, and leaving
it unset is what makes the `healthChecks` entries take effect — upstream ignores
`healthChecks` when `wait` is enabled.

The resulting umbrella Kustomization aggregates child readiness regardless of
how many children there are, giving external consumers a single stable anchor:

<!-- doc-example: pkg/stack/fluxcd Example_fluxWorkflowUmbrella -->
```go
umbrella := &stack.Bundle{
    Name: "platform",
    Children: []*stack.Bundle{
        {Name: "platform-infra"},
        {Name: "platform-services"},
        {Name: "platform-apps"},
    },
}

objects, err := fluxcd.Engine().ResourceGen.GenerateForBundle(umbrella, "production/apps/platform")
if err != nil {
    panic(err)
}
ks := objects[0].(*kustv1.Kustomization)
for _, hc := range ks.Spec.HealthChecks {
    fmt.Println(hc.Kind, hc.Namespace, hc.Name)
}
```
<!-- doc-example:end -->

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: platform
  namespace: flux-system
spec:
  prune: false
  healthChecks:
  - apiVersion: kustomize.toolkit.fluxcd.io/v1
    kind: Kustomization
    name: platform-infra
    namespace: flux-system
  - apiVersion: kustomize.toolkit.fluxcd.io/v1
    kind: Kustomization
    name: platform-services
    namespace: flux-system
  - apiVersion: kustomize.toolkit.fluxcd.io/v1
    kind: Kustomization
    name: platform-apps
    namespace: flux-system
  # ...rest of spec
```

User-supplied `HealthChecks` on the umbrella bundle are appended AFTER the
auto entries. `Wait` on an umbrella is the caller's choice like on any other
bundle: unset and `Wait: false` emit the same `spec`, and the umbrella still
becomes `Ready` only when every child is, through those health checks.

Umbrella children must be **standalone** — a bundle cannot simultaneously be
the `Bundle` of a `stack.Node` and appear in another bundle's `Children`.
`stack.ValidateCluster` rejects any such overlap before resource generation.

### Disk layout

In `FluxIntegratedPerLayout` placement, each Flux Kustomization CR sits in the
parent of the directory it applies, and that parent's `kustomization.yaml`
lists the CR file (not the child subdirectory). With `GroupByName` bundles:

```
clusters/production/apps/                         # node directory
  flux-system-kustomization-platform.yaml         # umbrella self CR (healthChecks), spec.path: .../apps/platform
  kustomization.yaml                              # lists the platform CR file
  platform/                                       # umbrella bundle directory
    flux-system-kustomization-platform-infra.yaml # child CR (placed at parent), spec.path: .../platform/platform-infra
    flux-system-kustomization-platform-apps.yaml  # child CR (placed at parent)
    kustomization.yaml                            # lists the child CR files
    platform-infra/                               # umbrella child subdirectory
      workload-*.yaml
      kustomization.yaml                          # workloads only, no Flux CRs
    platform-apps/
      workload-*.yaml
      kustomization.yaml
```

The child subdirectories contain **only** their workload manifests and a
per-directory `kustomization.yaml` listing those workloads. They do **not**
contain any `flux-system-kustomization-*.yaml` files — those live in the
parent directory, so Flux applies them once via the parent's Kustomization.

In `FluxSeparate` placement, all Kustomization CRs (the umbrella's own plus
every descendant) are written to the shared `flux-system/` directory as a
flat list.

## Augmenter-Added Child Layouts

A `LayoutAugmenter` can attach sub-`ManifestLayout` children to a per-app layout after resource generation. In `FluxIntegratedPerLayout` mode kure automatically emits a Flux `Kustomization` CR for each eligible child.

### Eligibility

A child layout receives a CR when:
- `!child.UmbrellaChild`
- `child.ApplicationFileMode != AppFileSingle`
- it renders no bundle (a child that does already has that bundle's CR in the parent)
- a source resolves: the `SourceRef` of the nearest bundle-rendering layout at or above the parent, with both `Kind` and `Name` set, else the one `SourceRef` the URL-less bundles below the child share (nil, empty struct, missing either field, or ambiguous is a hard error — a `Kustomization` without `spec.sourceRef` is invalid)

`CreateLayoutWithResources` validates SourceRef completeness for all bundles before layout walking. Both the node bundle and every umbrella child bundle must have `SourceRef.Kind` and `SourceRef.Name` set when either inline mode (`FluxIntegratedPerLayout` or `FluxIntegratedPerBundle`) is active — both emit bundle/node CRs carrying a `spec.sourceRef`. `FluxSeparate` and non-Flux callers are unaffected.

### Ordered reconciliation with DependsOn

Set `ManifestLayout.DependsOn` to the names of the sibling layouts' CRs to express reconciliation order between hook groups; a hook-group layout's CR is named after the layout. The integrator copies the entries verbatim into `spec.dependsOn` on the emitted CR, and only under `FluxIntegratedPerLayout`:

<!-- doc-example: pkg/stack/fluxcd Example_fluxWorkflowDependsOn -->
```go
preInstall := &layout.ManifestLayout{
    Name: "nginx-00-pre-install",
    // ...
}
hooks := &layout.ManifestLayout{
    Name:      "nginx-01-hooks",
    DependsOn: []string{"nginx-00-pre-install"},
    // ...
}
fmt.Println(hooks.Name, "after", hooks.DependsOn[0] == preInstall.Name)
```
<!-- doc-example:end -->

This produces a `nginx-01-hooks` Kustomization CR with:

```yaml
spec:
  dependsOn:
    - name: nginx-00-pre-install
```

Flux reconciles `nginx-01-hooks` only after `nginx-00-pre-install` is healthy.

### Naming uniqueness

The child layout `Name` becomes the Flux `Kustomization` CR's `metadata.name`. Since all `Kustomization` CRs live in the `flux-system` namespace, names must be **globally unique across the cluster**. The recommended convention is `{appName}-{hookGroupDir}` (e.g. `nginx-00-pre-install`, `nginx-01-hooks`). Augmenters are responsible for enforcing this uniqueness.

### Extra files an augmenter attaches

`ExtraFiles` an augmenter attaches must use portable relative names (letters, digits, `.`, `_`,
`-`, and `/` between segments) and may not take a path the writer owns in the layout's directory:
a generated resource file, a kustomize control file, a child layout's directory or file, or
another extra file. The layout writers refuse such a layout with an error instead of letting the
extra file replace a generated manifest. `ConfigMapGenerators` need the layout's own
`kustomization.yaml`, so the writers refuse them on an application written `AppFileSingle` (as
`ArgoProfile` writes applications), which gets none. See the
[Layout Engine reference](/api-reference/layout/) for the full rule.

### Disk layout

```
clusters/production/prod/
  flux-system-kustomization-nginx.yaml      # app CR (placed at node level)
  kustomization.yaml                        # references nginx CR
  nginx/
    flux-system-kustomization-nginx-00-pre-install.yaml
    flux-system-kustomization-nginx-01-hooks.yaml
    kustomization.yaml                      # references hook CRs
    nginx-00-pre-install/
      workload-*.yaml
      kustomization.yaml
    nginx-01-hooks/
      workload-*.yaml
      kustomization.yaml
```

## Bootstrap

Generate Flux system bootstrap manifests. Two modes are available:

- **`"flux-operator"`** (default) — emits a full Flux Operator install bundle (CRDs, Deployment, RBAC). Recommended for new clusters.
- **`"gotk"`** — emits the legacy GitOps Toolkit component manifests directly.

When `FluxMode` is empty, it defaults to `"flux-operator"`.

Both modes point Flux at the same directory: the one named after the root node, relative to the
root of the source, which is where a walk without a `ClusterName` writes a named root node.
`"gotk"` mode writes it as the bootstrap Kustomization's `spec.path` (`prod`, or `.` when the root
node has no name); `"flux-operator"` mode writes it as the `FluxInstance`'s `sync.path` (`./prod`,
or `./`). `"gotk"` mode used to write `manifests/<root>`: if your tree sits under such a prefix,
set `spec.path` on the returned Kustomization yourself. The bootstrap does not know your layout
rules, so the walked root can be somewhere else in two cases: a walk with a `ClusterName` writes
the root in or under the cluster directory, and a walk without one writes an unnamed root node to
`cluster`, while the bootstrap names the root of the source.

`"flux-operator"` mode needs both `FluxVersion` and `Registry`: they become the `FluxInstance`'s
distribution version and registry, and Kure has no default for either. If one is empty,
`GenerateBootstrap` returns an error naming the missing field and emits nothing, rather than a
`FluxInstance` that cannot work. `"gotk"` mode accepts both empty.

The `"flux-operator"` bundle is vendored from one specific upstream flux-operator release
(`FluxOperatorVersion`), so upgrading Kure can also change the CRDs it installs. A CRD that
tightens validation rejects objects the previous release accepted — for example a
`ResourceSetInputProvider` whose `filter.limit` exceeds 10000, or whose OCI `url` names only a
registry host. Kure has no setter for those fields, but you can assign them directly on the
upstream struct, so check hand-built objects against the release notes before upgrading. The
[compatibility matrix](/api-reference/compatibility/) records the supported flux-operator range.

The vendored bundle follows Kure's own `flux-operator` module version: when Renovate bumps the
module it also re-vendors the bundle and updates `FluxOperatorVersion`, so the bundle you get
always matches the release Kure was built against. Widening the supported range to cover a new
release is a separate, manual step that records a compatibility assessment.

`"gotk"` mode is vendored the same way: its components are built from the install manifests of
the flux2 release Kure depends on (`GotkVersion`), with no network access, as long as
`FluxVersion` is empty or names that release. Setting `FluxVersion` to any other release, or to
`"latest"`, opts in to downloading manifests from GitHub each time the bundle is generated: the
named release when written `vX.Y.Z`, otherwise the latest release.

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

`SourceRef` names the revision to sync, and the same value works in both modes: an OCI tag
(empty means `latest`) or a Git branch name. In `"flux-operator"` mode Kure turns a branch name
into the full reference the operator's `GitRepository` needs (`main` becomes `refs/heads/main`).
A value that already starts with `refs/` — a tag such as `refs/tags/v1.0.0` — is passed through
unchanged in `"flux-operator"` mode only; `"gotk"` mode always treats a Git `SourceRef` as a branch.

The root node's name becomes a segment of the path the bootstrap applies, so it is checked as a
directory name wherever a path is built from it: always in `"gotk"` mode, and in
`"flux-operator"` mode when `SourceURL` is set (without one the `FluxInstance` has no sync and the
name is not used). A name holding `/`, `\` or a NUL byte, or `.` or `..`, is refused; no root node
and an unnamed root are valid.

### Bootstrap namespace

The bootstrap namespace is not part of `BootstrapConfig` — it lives on the generator. The engine
holds its own generator and `engine.GenerateBootstrap` delegates to that one, so configure it
through the engine rather than building a second generator the call never reads:

<!-- doc-example: pkg/stack/fluxcd Example_fluxWorkflowBootstrapNamespace -->
```go
engine := fluxcd.Engine()
rootNode := &stack.Node{Name: "production"}
bootstrapConfig := &stack.BootstrapConfig{
    Enabled:     true,
    FluxVersion: "v2.8.2",
    Registry:    "ghcr.io/fluxcd",
}

engine.GetBootstrapGenerator().DefaultNamespace = "custom-flux" // default: "flux-system"

objects, err := engine.GenerateBootstrap(bootstrapConfig, rootNode)
if err != nil {
    panic(err)
}
for _, obj := range objects {
    switch obj.GetObjectKind().GroupVersionKind().Kind {
    case "FluxInstance":
        fmt.Println("FluxInstance in", obj.GetNamespace())
    case "Namespace":
        fmt.Println("Namespace", obj.GetName())
    }
}
```
<!-- doc-example:end -->

**How much it moves depends on the mode, and the two differ sharply.**

With `"gotk"` it relocates the whole bundle: the toolkit components themselves — the controllers'
Deployments, ServiceAccounts, Services, NetworkPolicies and ResourceQuota — alongside the root
Kustomization and the root source. The cluster-scoped objects follow where they name a namespace,
since the emitted `Namespace` and the `ClusterRoleBinding` names and subjects are derived from the
same value; CRDs and ClusterRoles carry no namespace.

With `"flux-operator"` — the default mode — it reaches **only the `FluxInstance`**. The operator's
own install bundle is appended unmodified from an embedded manifest, so its `Namespace`,
ServiceAccount, Service and Deployment stay hardcoded to `flux-system` whatever this field says. A
non-default value therefore yields an operator running in `flux-system` reconciling a `FluxInstance`
in your namespace. That works, but it is not "the bundle moved", and the emitted `Namespace` object
is still named `flux-system`.

One further consequence, for `"gotk"`, worth knowing before you narrow anything: the toolkit
components are generated with `WatchAllNamespaces` at its upstream default of `true`, and that is
not currently derived from configuration. Controllers therefore reconcile across namespaces
regardless of where they run.

### Sync name

In `"flux-operator"` mode the `FluxInstance` sync makes the operator create a source and a
Kustomization of its own. `BootstrapConfig.SyncName` names both; it becomes `spec.sync.name`.
Left empty, the operator names them after the `FluxInstance`'s namespace — `flux-system` unless
you moved it as above. Set `SyncName` when the Kustomizations you generate reference the sync
source under another name, or their `sourceRef` points at a source nothing creates:

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

It only takes effect with `SourceURL` set, since no sync block is emitted without one, and
`"gotk"` mode ignores it: there kure creates and names the root source itself. kure does not
validate the value. The CRD caps it at 63 characters and makes it immutable once set, so renaming
the sync of a running cluster means recreating the `FluxInstance`. It is distinct from the
`FluxInstance`'s own `metadata.name`, which is always `flux` (`fluxcd.FluxInstanceName`): the
flux-operator CRD rejects any other name at admission. The generator's
`BootstrapName` names the bootstrap Kustomization only and never reaches the `FluxInstance`.

## Further Reading

- [Stack](/api-reference/stack) - Domain model reference
- [Flux Engine](/api-reference/flux-engine) - Workflow engine reference
- [Layout Engine](/api-reference/layout) - Directory organization reference
