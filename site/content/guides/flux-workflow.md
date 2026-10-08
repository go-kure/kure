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
is written. A bundle that sets `KustomizationName` gives its Kustomization that name instead: the
value must be a DNS-1123 subdomain as well, and the 63-character limit then applies to it and not
to the bundle name. A refused name is reported with its node or bundle path, and Kure never shortens one.
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
| BundleGrouping | `GroupByName`, `GroupFlat` | A directory per bundle, or render bundles in their node's directory (the root node's bundle keeps a directory, see below) |
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
  parent's path instead (`Namespace: "apps"`). Since go-kure/kure#979 the writers refuse such a
  child where its parent's `kustomization.yaml` lists it: the entry `web` of the parent at `apps`
  names `apps/web`, a directory without a `kustomization.yaml`. `WriteToDisk`, `WriteToTar` and
  `WriteManifest` return an error that names both directories, before anything is written. A
  `Namespace` that is the child's own path in another case (`apps/web` for a child named `Web`)
  is refused the same way. A child its parent does not list, or one placed elsewhere on purpose,
  is written as before; see the [Layout Engine reference](/api-reference/layout/) for the full
  rule.
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
- **Objects without a kind or an apiVersion are refused** (breaking change in go-kure/kure#1020).
  Each resource, and each item of a `List` however deep, must be written with both; a typed
  object whose `TypeMeta` is unset has neither. kustomize fails on a document without a kind, and
  one without an apiVersion builds into an object nothing applies. Set the `TypeMeta`, as the
  `Create*` constructors do, or the two fields of an unstructured object. A `List`'s own
  apiVersion is not required, nor the apiVersion of an object with the
  `config.kubernetes.io/local-config` annotation at any string value but `"false"`, which kustomize
  drops from the build.
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
  returned before that change (the root node's bundle's path has moved since, in
  go-kure/kure#979: see "The root node's bundle has its own directory" below). The Flux engine
  refuses rules with `FluxIntegratedPerLayout`: that tree's child
  directories are applied by Kustomizations only `CreateLayoutWithResources` places, so use that
  entry point for the placement. `ResourceGenerator.GenerateFromLayout` refuses a tree in which
  any layout carries that placement, for the same reason: use `CreateLayoutWithResources`, or
  `IntegrateWithLayout` on the tree you walked. Before, it returned the bundle directories'
  Kustomizations only. Rules the walk refuses are an error from both engines, whatever the
  cluster is: an absent or empty cluster returns nothing only with valid rules.
- **Integrate walked layouts only.** `IntegrateWithLayout` refuses a layout `layout.WalkCluster`
  did not build from the same cluster — build the tree with `WalkCluster` instead of by hand.
- **PerLayout hosts.** Under `FluxIntegratedPerLayout` a node bundle's CR now sits in the parent of
  the layout that renders it, like every other PerLayout CR, and the parent's `kustomization.yaml`
  lists it as a resource file. The writers no longer emit a `flux-system-kustomization-<child>.yaml`
  reference guessed from a child's name. A bundle-less node layout gets its own CR named
  `<path, "/" → "-">-node`.
- **Sources live in the root.** Under both integrated placements, every GitRepository or
  OCIRepository the integration derives from a `SourceRef` URL is written once, into the root
  node's layout (the root node's directory, not a `ClusterName` wrapper above it). Before, each unit's parent layout held a copy, so a Source shared by several
  builds had several owners, and pruning one unit deleted a Source the others still used. Golden
  files move the Source to the root. `FluxSeparate` is unchanged.
- **Refusals.** Two bundles with one name, a CR name used twice, a node or bundle layout written as
  `AppFileSingle`, and a directory holding two objects with one identity are errors; a `List` is
  read for the objects it holds, so the two are refused however many Lists deep one of them
  sits. So are a Flux
  Kustomization inside a `List` that takes another Kustomization's identity (kustomize builds a
  List's items); a Source identity (kind, namespace, name) defined with different content
  anywhere in the pass, in another layout or at another API version; and, under `FluxSeparate`,
  any Source already in the tree with the identity of one the integration generates into
  `flux-system`, even an identical one. Under `FluxIntegratedPerBundle`, a caller's copy of a
  generated Source in a `ClusterName` wrapper above the root node is refused too: that build also
  includes the root's copy, and kustomize refuses one object twice.
  No Kustomization takes its source from inside what it applies (go-kure/kure#979): under
  `FluxIntegratedPerLayout` with a `ClusterName` wrapper above a named root node, the root node's
  directory has a Kustomization of its own, which applies the directory that hosts every
  generated Source. It takes none of them; its source is the one `SourceRef` without a URL that
  the bundles below share (where two of them differ, the root node's own bundle's, when that has
  no URL), and a tree in which every `SourceRef` has a URL is refused, naming the
  Kustomization and the Source. Give one bundle a `SourceRef` without a URL, naming a Source that
  exists before the tree is applied, or use `FluxIntegratedPerBundle`. Before, the root bundle's
  generated Source was accepted there; the bootstrap points Flux at the `ClusterName` directory,
  the top of the written tree, so the Kustomization would have waited for a Source only its own
  apply creates.
  A Kustomization already in the tree that the integration keeps in place of its own (same name
  and `spec.path`, in the layout that would host it) is held to the same refusals as a generated
  one, whether it is typed, unstructured or inside a `List`: two copies of a generated Source
  that the integration did not add, in the build of its directory; a `sourceRef` that names a
  generated Source when its build holds the root node's directory; and a cycle through its
  `dependsOn`, `wait` or health checks. One that cannot be read as a Flux Kustomization (a field
  of the wrong type, for one) is refused, naming its layout. Its patches and postBuild are its
  own: the one kept Kustomization that builds the directory hosting the generated Sources is the
  root node's layout Kustomization below a `ClusterName` wrapper under
  `FluxIntegratedPerLayout`, and since go-kure/kure#979 the bootstrap applies the wrapper, which
  lists that Kustomization and not the directory, so it alone applies those Sources (before, a
  patch or postBuild of it that changed one was refused).
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
no longer collide. Without it the tree is refused, and the error names the application and bundle
that hold the Kustomization in the tree, what the generated one is for, and the field to set. See
the
[Flux Engine reference](/api-reference/flux-engine/#kustomization-names).

### The root node's bundle has its own directory (breaking change in go-kure/kure#979)

The root node's directory renders no bundle. With a flat `BundleGrouping` (the default) the root
node's bundle used to render in the root node's directory, which the bootstrap applies when there
is no `ClusterName`, and the
Flux Kustomization that applied that directory was hosted inside it: it was part of the build it
applied. The bundle now has a directory inside the root node's, named after the bundle
(`platform/platform-bundle` for root node `platform` with bundle `platform-bundle`) or by the
bundle's `DirName` when it sets one (see [Directory names](#directory-names)), and its
Kustomization sits in the root node's directory under the integrated placements, and in the
`flux-system/` directory at the top of the tree under `FluxSeparate`. Bundles that a flat `NodeGrouping` merges into the root node share that directory, named
after the first of them. With `BundleGrouping: GroupByName` nothing changes: the bundle already
had its directory.

What changed, and what to do:

- **Paths move.** The root node's bundle's files, the `spec.path` of its Flux Kustomization and the
  `source.path` of its ArgoCD Application are one directory lower. Golden files change, and so
  does anything outside kure that names the old path. The bootstrap sync path is unchanged.
- **On a deployed tree, expect a handover.** The bundle's objects used to be applied by the
  bootstrap as well, as part of the root node's directory. They leave that build, so with prune
  on, the objects can be deleted by the outer owner and re-created by the inner Kustomization.
- **A child node named like that directory is refused.** A child node of the root node whose name
  is the name of the root node's bundle's directory would render to the same path; the walk
  refuses it, naming both, also when the names differ only in case (`WEB` for `web`). Rename the
  node, or give the bundle's directory another name (its `DirName`, or its `Name` without one).
  A name that is not one path segment is refused earlier, when the cluster is validated: a node
  named `./web`, a bundle named `.`, `/` or `web/api`.
- **A root node's bundle whose directory is `flux-system` is refused under `FluxSeparate`.** That
  is a bundle with `DirName: "flux-system"`, or one named `flux-system` without a `DirName`.
  Without a `ClusterName` its directory would be the one the Flux resources are written to. Give
  the bundle's directory another name, or use an integrated placement. The ArgoCD engine refuses
  the directory `argocd` the same way.
- **`FlattenSingleTier` collapses no directory that renders a bundle.** A collapsed directory's
  Kustomization would have no parent to sit in. On a walked tree the option now only collapses a
  single child directory that renders no bundle and has no directory below it (a node with neither
  a bundle nor child nodes, or one with bundle-less nodes merged into it by a flat
  `NodeGrouping`); a tree that relied on it to put a single-bundle application at the top keeps
  its directories.
- **A tree whose top renders a bundle is not integrated.** `IntegrateWithLayout` refuses it in
  every placement; integrate the whole tree `layout.WalkCluster` returns, not a subtree of it.

The `SourceRef` of the root node's bundle is still the source of the Kustomizations of the root
node's directory and of the bundle-less directories below it, as before (the Kustomization of the
root node's directory itself takes it only when it has no URL, see "Refusals" above). A parent bundle still
cannot depend on a child node's bundle under the integrated placements, except the root node's
bundle, whose directory no longer holds the child's Kustomization. See
[The root node's bundles](/api-reference/layout/#the-root-nodes-bundles) for the rule.

### Directory names

A bundle's directory is named after the bundle too. Set `Bundle.DirName` to give it another
name, an ordering prefix for example, without renaming the bundle or its Kustomization:

<!-- doc-example: pkg/stack/fluxcd ExampleEngine_dirName -->
```go
// The children's directories get the names set here; the bundles and
// their Kustomizations keep theirs.
infra := &stack.Bundle{Name: "shop-infra", DirName: "00-infra"}
services := &stack.Bundle{Name: "shop-services", DirName: "10-services", DependsOn: []*stack.Bundle{infra}}
shop := &stack.Bundle{Name: "shop", Children: []*stack.Bundle{infra, services}}
cluster := &stack.Cluster{Name: "prod", Node: &stack.Node{Name: "apps", Bundle: shop}}

objects, err := fluxcd.Engine().GenerateFromCluster(cluster, layout.DefaultLayoutRules())
if err != nil {
    panic(err)
}
for _, obj := range objects {
    kust := obj.(*kustv1.Kustomization)
    fmt.Println(kust.Name, kust.Spec.Path)
}
```
<!-- doc-example:end -->

It prints `shop apps/shop`, `shop-infra apps/shop/00-infra` and `shop-services
apps/shop/10-services`: the children are written to `apps/shop/00-infra` and
`apps/shop/10-services`, their Kustomizations are still `shop-infra` and `shop-services` with
`spec.path` at those directories, and the `dependsOn` entry and the umbrella's health checks keep
naming the Kustomizations. The disk and tar writers produce the same tree. (`shop` is the root
node's bundle here, so it has its directory inside `apps`; a `DirName` on it would name that one.)

The field applies wherever a bundle's name becomes a directory: for an umbrella child, for a node's
bundle under `BundleGrouping: GroupByName`, and for the root node's bundle under the default rules.
Below the root the default rules render a node's bundle in its node's directory, and `DirName` has
no effect on it. Bundles a flat `NodeGrouping` merges into the root node share one directory: it
takes the first merged bundle's `DirName`, and one on a later bundle has no effect. A `DirName` must
be one path segment; it is no object name, so `00_Infra` is valid. Two bundles that would get one
directory are refused, and the error names both. Name the directory on the bundle: a bundle's
layout renamed after `WalkCluster` is refused by `IntegrateWithLayout`. See the
[Flux Engine reference](/api-reference/flux-engine/#directory-names).

### Node Kustomizations

Under `FluxIntegratedPerLayout` a node whose directory renders no bundle, such as a group of
nodes, is applied by a Kustomization of its own. `Node.KustomizationName` names it, and
`Node.DependsOn` and `Node.NamedDependsOn` order it:

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

It prints `platform prod/platform []` and `apps prod/apps [platform]`. Without the field the two
would be named `prod-platform-node` and `prod-apps-node`, after their paths, and a `DependsOn`
entry names its target either way. The fields are refused, not ignored, on a node that has no
Kustomization of its own (its directory renders a bundle, a grouping rule merged it into another
node's directory, or it is the top of the tree) and under every other placement. Set the name and
the named dependencies before the cluster is walked: the walker copies them onto the node's
layout, and a value the node gets after the walk is refused too. See the
[Flux Engine reference](/api-reference/flux-engine/#node-kustomizations).

### Application layout Kustomizations are named after their unit (breaking change)

Under `FluxIntegratedPerLayout` the Kustomization of an application layout or of an augmenter's
child layout is now named `<unit name>-<layout name>`, the unit being the Kustomization of the
bundle it belongs to. It used to be the layout's name alone, so an application named like its
bundle gave two Kustomizations one name and was refused. What to do:

- **Expect the rename on upgrade.** The application `web-app` of the bundle `web` is now applied
  by the Kustomization `web-web-app`. With pruning on, Flux removes the old Kustomization once the
  parent no longer lists it; with pruning off, delete it. A Kustomization that prunes deletes
  what it applied when it is removed, so the layout's objects can be deleted and applied again
  under the new one: keep the old name (last point) where that is not acceptable.
- **`ManifestLayout.DependsOn` keeps working for layout names.** An entry that names a layout of
  the same bundle is written as that layout's new Kustomization name.
- **A Kustomization name written out by hand changes.** A `NamedDependsOn` entry, or a
  `ManifestLayout.DependsOn` entry that names an application layout of another bundle, has to
  carry the new name.
- **A Kustomization you placed yourself under the old name stays.** A tree that already holds a
  Kustomization named `<layout name>` for the layout's directory gets the new one beside it, and
  both apply that directory until you remove the old one or set
  `ManifestLayout.KustomizationName` on the layout to the old name, which keeps yours and
  generates no second one.
- **A new name over 63 characters is shortened.** The new name is longer than the old one by the
  unit name, and Flux cannot reconcile a Kustomization whose name is over 63 characters, so the
  integration shortens it to `<unit prefix>-<hash>-<layout name>`, keeping the layout's name. A
  layout name over 54 characters leaves no room for the hash beside it, and the name is its first
  54 bytes, without the hyphens and dots that end them, and the hash: `<layout name
  prefix>-<hash>`, or the hash alone where nothing is left of those bytes (go-kure/kure#1036; it
  was refused before). Set `ManifestLayout.KustomizationName` on the layout for a name of your own.
- **A new name another Kustomization already has is refused.** The application `web` of the
  bundle `platform` is now applied by `platform-web`. If a bundle, a node or another layout
  already has a Kustomization of that name, the integration refuses the tree and names both,
  where the old name `web` was free. Set `ManifestLayout.KustomizationName` on the layout, or
  `KustomizationName` on the other owner.
- **Keep an old name** by setting `ManifestLayout.KustomizationName` on the layout.

### Per-layout Kustomizations take the bundle's readiness settings and labels (breaking change)

Under `FluxIntegratedPerLayout` the Kustomization of an application's directory, and of every
layout below it, now carries `wait`, `timeout`, `retryInterval`, labels and annotations: those of
the bundle that holds the application, unless the layout sets its own (`ManifestLayout.Wait`,
`Timeout`, `RetryInterval`, `Labels`, `Annotations`). They used to carry none, so a bundle with
`Wait` was Ready once those Kustomizations had applied their directories, not once the workloads
were. Interval, prune and four more settings follow in the next section. What to do:

- **Expect the new fields in the output.** A tree whose bundle sets one of the five gains it on
  those Kustomizations. A tree that sets none renders as before.
- **Give the layouts below a shorter timeout than their bundle.** With `wait`, the bundle's
  Kustomization waits for the ones below it under its own timeout. Set `Timeout` on the layouts,
  and keep the bundle's above theirs; Flux treats any timeout under 30 seconds as 30 seconds.
- **A layout that depends on the application layout above it is refused once the bundle sets
  `Wait`.** That application layout's Kustomization would wait for the one that depends on it.
  Set `Wait` to a pointer to `false` on the application's layout; the layouts below keep the
  bundle's.
- **A label or annotation the Kubernetes API does not accept is refused** where a per-layout
  Kustomization takes it, naming the layout and, for one it inherits, the bundle.
- **A duration Flux does not take is refused**, on every placement and on the bootstrap: a
  negative one (`"-1s"`) or a positive one under a millisecond (`"1us"`), in a bundle's `Interval`,
  `Timeout` or `RetryInterval`, a layout's `Interval`, `Timeout` or `RetryInterval`, or a generator's
  `DefaultInterval`. The Flux API refuses the object such a value is written into, so no
  object it ever accepted changes. `Bundle.Validate` still accepts these values.
- **A node's Kustomization inherits nothing.** Set the fields on the node's walked layout.

What readiness through such a chain means, and where it stops, is in the
[Flux Engine reference](/api-reference/flux-engine/#per-layout-settings).

### Per-layout Kustomizations take the bundle's interval, prune, force, suspend, postBuild and patches (breaking change)

Under `FluxIntegratedPerLayout` the same Kustomizations now also carry `interval`, `prune`,
`force`, `suspend`, `postBuild` and `patches` (go-kure/kure#1021). Interval, prune, force and
suspend are the layout's own (`ManifestLayout.Interval`, `Prune`, `Force`, `Suspend`), or else the
bundle's; interval and prune fall back to the generator's. `postBuild` is the bundle's, copied
whole, and the bundle's patches are placed by object. What to do:

- **Expect the new fields in the output.** A tree that sets none of the six renders as before.
  A bundle with `Prune` on turns garbage collection on for its applications' Kustomizations,
  where the generator's `Prune` applied before.
- **Expect a patch to move.** A patch with a target is on the bundle's own Kustomization and on
  every per-layout one. A plain untargeted strategic-merge patch is on the Kustomizations whose
  build holds every object it names, so the bundle's own loses one whose object it does not build;
  its build failed on the cluster before. An untargeted JSON6902 patch, or one carrying a build
  annotation, stays on the bundle's own Kustomization alone and fails there when only a layout
  builds its object: give it a target.
- **A plain untargeted patch no build can take is refused** at integration instead of failing on the
  cluster: when no Kustomization of the bundle builds an object it names, give it a target or
  place the object in a build of the bundle; when no one Kustomization builds all the objects it
  names, write one patch per object. A target is for a patch of one object: kustomize refuses one
  on an entry of several documents, or of a `List` of several items, so split such a patch first.
- **After a JSON6902 patch, a patch with one of kustomize's build annotations**
  (`internal.config.kubernetes.io/`) **or a patch that can remove an object from the build**
  (`$patch: delete`, or `config.kubernetes.io/local-config` at any value but `"false"`),
  the patches that follow are not placed by object and stay
  on the bundle's own Kustomization as well; one whose object only a layout builds still fails
  there. Put it before that entry, or give it a target.
- **Expect Flux's opt-out on a Kustomization whose postBuild or patches the parent would
  change.** A per-layout Kustomization is in its parent's build, whose postBuild substitution runs
  over its own postBuild and patches first. Where that would change either at all, a plain
  `${VAR}` in a bundle patch included, it gets `kustomize.toolkit.fluxcd.io/substitute: disabled`
  and applies them as if no parent had run. That also stops the parent substituting its labels,
  annotations and path, so a tree where the parent's substitution changes a `${...}` in the
  bundle's patches or substitute values and one in its labels, annotations or the layout's path
  is refused, where it rendered before: drop one of the two. A tree whose annotations leave no
  room for the opt-out under the Kubernetes API's total size is refused as well: shorten them.

The full rules, the refusals and the limit are in the
[Flux Engine reference](/api-reference/flux-engine/#per-layout-settings).

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
becomes `Ready` only when every child is, through those health checks. With
`Wait: true` no entry is written for the children, since Flux ignores
`healthChecks` under `wait`; the user-supplied entries are handled as before. The
integrated placements then still wait for the children, whose Kustomization
objects the umbrella applies; `FluxSeparate` does not, so leave `Wait` unset
there for the umbrella to wait for them. With `GenerateForBundle` alone it
depends on whether the caller's build at the given path applies the children's
Kustomization objects.

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
- a source resolves: the `SourceRef` of the nearest bundle-rendering layout at or above the parent, with both `Kind` and `Name` set (with `BundleGrouping: GroupFlat` the root node's directory renders no bundle and counts with the `SourceRef` of the root node's own bundle; with `GroupByName` no node's directory counts), else the one `SourceRef` the URL-less bundles below the child share, else (no `SourceRef` without a URL below the child, or two that differ) the `SourceRef` of the bundle of the nearest node, at or above the child's own, that has one: what a node's directory takes with `GroupByName` when every `SourceRef` at and below it has a URL or the ones without a URL below it differ (nil, empty struct, missing either field, or still ambiguous with no such bundle is a hard error — a `Kustomization` without `spec.sourceRef` is invalid); the Kustomization of the root node's directory, below a `ClusterName` wrapper, takes no Source the integration generates and is refused when no other is left (go-kure/kure#979)

`CreateLayoutWithResources` validates SourceRef completeness for all bundles before layout walking. Both the node bundle and every umbrella child bundle must have `SourceRef.Kind` and `SourceRef.Name` set when either inline mode (`FluxIntegratedPerLayout` or `FluxIntegratedPerBundle`) is active — both emit bundle/node CRs carrying a `spec.sourceRef`. `FluxSeparate` and non-Flux callers are unaffected.

### Ordered reconciliation with DependsOn

Set `ManifestLayout.DependsOn` to the names of the sibling layouts to express reconciliation order between hook groups. A hook-group layout's CR is named `<unit name>-<layout name>` (or by its `KustomizationName`), and the integrator writes that name into `spec.dependsOn` for an entry that names a layout of the same unit: a sibling first, else the one layout of that name below the unit's directory. Any other entry is a Kustomization name and is written as given. This happens only under `FluxIntegratedPerLayout`:

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

Each layout's CR is named `<unit name>-<layout name>`, and the entry is written as the CR name of
the layout it names. Below the bundle `web` this produces a `web-nginx-01-hooks` Kustomization CR
with:

```yaml
spec:
  dependsOn:
    - name: web-nginx-00-pre-install
```

Flux reconciles `web-nginx-01-hooks` only after `web-nginx-00-pre-install` is healthy. An entry
may also name another layout of the same bundle, the parent application layout for instance
(not while that layout's Kustomization has `wait`, its own or the bundle's: see the section
above). An entry that names no such layout is written as given, as the name of a Kustomization.

### Naming uniqueness

The Flux `Kustomization` CR's `metadata.name` is `<unit name>-<child layout Name>`, or the child's `KustomizationName` when the augmenter sets one. Since all `Kustomization` CRs live in the `flux-system` namespace, names must be **globally unique across the cluster**. The recommended convention for the layout name is `{appName}-{hookGroupDir}` (e.g. `nginx-00-pre-install`, `nginx-01-hooks`). Augmenters are responsible for enforcing this uniqueness within their bundle.

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

With the application `nginx` in the bundle `web`, each CR and its file carry the unit name:

```
clusters/production/prod/
  flux-system-kustomization-web-nginx.yaml  # app CR (placed at node level)
  kustomization.yaml                        # references nginx CR
  nginx/
    flux-system-kustomization-web-nginx-00-pre-install.yaml
    flux-system-kustomization-web-nginx-01-hooks.yaml
    kustomization.yaml                      # references hook CRs
    nginx-00-pre-install/
      workload-*.yaml
      kustomization.yaml
    nginx-01-hooks/
      workload-*.yaml
      kustomization.yaml
```

## Delivery Intent per Application

`Bundle.Prune` and `Bundle.Force` apply to a whole Flux Kustomization. To protect one application
from pruning, or to allow delete-and-recreate for one application, set its `Delivery`:

<!-- doc-example: pkg/stack/fluxcd Example_fluxWorkflowDeliveryIntent -->
```go
database := stack.NewApplication("database", "data", databaseConfig)
database.Delivery = stack.DeliveryIntent{
    PruneProtection: true, // keep its objects when they leave the source
    ForceReplace:    true, // delete and recreate on an immutable-field change
}
cache := stack.NewApplication("cache", "data", cacheConfig)

dataTier, err := stack.NewBundle("data-tier", []*stack.Application{database, cache}, nil)
if err != nil {
    panic(err)
}
dataTier.SourceRef = &stack.SourceRef{Kind: "GitRepository", Name: "flux-system"}
cluster := stack.NewCluster("production", &stack.Node{Name: "production", Bundle: dataTier})

// The Flux engine annotates the database's objects, and only those.
ml, err := fluxcd.Engine().CreateLayoutWithResources(cluster, layout.DefaultLayoutRules())
if err != nil {
    panic(err)
}
var show func(l *layout.ManifestLayout)
show = func(l *layout.ManifestLayout) {
    for _, rec := range l.OriginApplicationObjects() {
        for _, obj := range rec.Objects {
            kind := obj.GetObjectKind().GroupVersionKind().Kind
            fmt.Println(rec.Application.Name, kind, len(obj.GetAnnotations()))
            for _, key := range slices.Sorted(maps.Keys(obj.GetAnnotations())) {
                fmt.Printf("  %s: %s\n", key, obj.GetAnnotations()[key])
            }
        }
    }
    for _, child := range l.Children {
        show(child)
    }
}
show(ml.(*layout.ManifestLayout))
```
<!-- doc-example:end -->

The engine writes `kustomize.toolkit.fluxcd.io/prune: disabled` and
`kustomize.toolkit.fluxcd.io/force: enabled` on that application's objects under every placement
and grouping, including the objects an augmenter adds to the application's own layout and the
ConfigMaps its `configMapGenerator` entries build. For a `List` the application emits, the objects
it holds are annotated, those of a `List` inside it included, and never the List itself. An
object that already carries one of the two
annotations with another value is refused with an error naming it. The annotations are applied by
`IntegrateWithLayout` and `CreateLayoutWithResources`; see the
[Flux Engine reference](/api-reference/flux-engine/) for the full rule, including what prune
protection means for generated ConfigMaps.

## Bootstrap

Generate Flux system bootstrap manifests. Two modes are available:

- **`"flux-operator"`** (default) — emits a full Flux Operator install bundle (CRDs, Deployment, RBAC). Recommended for new clusters.
- **`"gotk"`** — emits the legacy GitOps Toolkit component manifests directly.

When `FluxMode` is empty, it defaults to `"flux-operator"`.

`GenerateBootstrap` takes the layout rules you write the tree with, and both modes point Flux at
the top directory of that tree, relative to the root of the source: the cluster directory when
the rules have a `ClusterName`; without one the root node's name, `cluster` for an unnamed root
node, and the root of the source when you pass no root node. `"gotk"` mode writes it as the
bootstrap Kustomization's `spec.path` (`prod`, `clusters/prod`, or `.` for the root of the
source); `"flux-operator"` mode writes it as the `FluxInstance`'s `sync.path` (`./prod`,
`./clusters/prod`, or `./`). Pass the same rules you pass to `GenerateFromCluster` or
`WalkCluster`: the engine refuses rules that are not a `layout.LayoutRules` value, nil included,
and invalid ones.

**Breaking change (go-kure/kure#979).** `GenerateBootstrap` and `GenerateFluxInstance` take the
rules as a third argument. The bootstrap used to name the root node whatever the rules were, so
with a `ClusterName` both paths move to the cluster directory, and an unnamed root node without
one moves from the root of the source to `cluster`. A named root node without a `ClusterName`
keeps its path. Apply the regenerated bootstrap objects; until then a cluster keeps applying the
old directory.

Two limits. The directory is relative to the directory the tree is written into: if you write the
tree below a sub-path of the repository, or under a prefix such as the `manifests/<root>` that
`"gotk"` mode once wrote, set `spec.path` or `sync.path` on the returned object yourself. And it
describes a `WalkCluster` tree: `WalkClusterByPackage` writes one tree per package, placed
without the `ClusterName`, and the bootstrap does not point at those.

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

`SourceRef` names the revision to sync, and the same value works in both modes: an OCI tag
(empty means `latest`) or a Git branch name. In `"flux-operator"` mode Kure turns a branch name
into the full reference the operator's `GitRepository` needs (`main` becomes `refs/heads/main`).
A value that already starts with `refs/` — a tag such as `refs/tags/v1.0.0` — is passed through
unchanged in `"flux-operator"` mode only; `"gotk"` mode always treats a Git `SourceRef` as a branch.

Without a `ClusterName` the root node's name is the path the bootstrap applies, so it is checked
as a directory name wherever a path is built from it: always in `"gotk"` mode, and in
`"flux-operator"` mode when `SourceURL` is set (without one the `FluxInstance` has no sync and the
name is not used). A name holding `/`, `\` or a NUL byte, or `.` or `..`, is refused; no root node
and an unnamed root are valid. Under a `ClusterName` the path is the cluster directory and the
bootstrap does not check the name as a directory name; the walk still does, as a named root node
is the directory `<ClusterName>/<root>` of the written tree. In `"gotk"` mode a named root also names the generated source and
the bootstrap Kustomization's `sourceRef`, so there it must be a DNS-1123 subdomain as well:
`Prod` and `prod_root` are refused.

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

objects, err := engine.GenerateBootstrap(bootstrapConfig, rootNode, layout.LayoutRules{})
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

fi, err := engine.GetBootstrapGenerator().GenerateFluxInstance(bootstrapConfig, rootNode, layout.LayoutRules{})
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
