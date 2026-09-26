# Layout Module

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/stack/layout.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/stack/layout)

The layout module is a sophisticated system for organizing and writing Kubernetes manifests to disk in directory structures that work with GitOps tools like Flux and ArgoCD.

## Core Purpose

The layout module transforms Kure's in-memory stack representation (Clusters → Nodes → Bundles → Applications) into organized directory structures with proper kustomization.yaml files that GitOps tools can consume.

## Key Components

### 1. ManifestLayout Structure
- Central data structure representing a directory with its resources and children
- Contains: Name, Namespace, Resources (K8s objects), Children (subdirectories)
- Supports package-aware layouts for multi-OCI/Git scenarios

#### Layout paths

A layout's directory is `FullRepoPath()`, which is `Namespace` joined with `Name`. `Namespace` is
always the **parent's** directory: set a child's `Namespace` to its parent's `FullRepoPath()`, and
use `"."` for the root of the tree. An empty `Namespace` means `cluster`. An `AppFileSingle` layout
writes one file, `<Namespace>/<Name>.yaml`, into its `Namespace`, normally its parent's directory,
and the parent's `kustomization.yaml` lists it. An `AppFileSingle` root writes that file and its `kustomization.yaml`
into its `Namespace`, and lists its children there: build them with the root's `Namespace`, not its
`FullRepoPath()`.

The parent lists an `AppFileSingle` child's file by its path relative to the parent's directory,
slash-separated (go-kure/kure#879): `svc.yaml` when the child's `Namespace` is the parent's
directory, `sub/svc.yaml` when it is `<parent directory>/sub`. Before, the entry was always
`<Name>.yaml`, which named no file for a child below the parent's directory, and named one file
twice for two such children with one name. A listed child whose file lands outside the parent's
directory (a sibling or an ancestor directory) is refused before anything is written: the entry
would start with `../`, and kustomize's default load restrictor rejects it. The parent's directory
must match exactly: a child `Namespace` of `P` or `P/sub` under a parent directory `p` is refused
too, since the two spellings name one directory only on a case-insensitive volume. Below the
parent's directory the child's own spelling is kept, so `p/Sub` under `p` is listed as
`Sub/svc.yaml`. A child the parent does not list (an umbrella child, one that renders bundles, one
with no resources), or any child of a parent that writes no `kustomization.yaml`, may land
anywhere. A `FluxIntegratedPerLayout` parent, or a parent of another package, lists an
`AppFileSingle` child's file all the same. An `AppFileSingle` child's `Name` must be one file name:
one that is rooted or holds a path separator (`/svc`, `sub/svc`, `../q/svc`) is refused, listed or
not. A rooted `Namespace` is joined onto the writer's base path, and `WriteToTar` resolves it
under the archive root, so a child `Namespace` of `/p/sub` under a parent directory `p` is listed
as `sub/svc.yaml` by all three writers.

When a parent's `kustomization.yaml` references a child by directory (`- <Name>`), the child's
directory must be `<parent directory>/<Name>` for the reference to resolve; building every child
with its parent's `FullRepoPath()` as `Namespace` gives exactly that. Layouts with the same name nest:
a bundle `web` with an application `web` gives `web/web`. A caller may still place a child
elsewhere on purpose, for example an OCI artifact whose root lists sibling layers that live under a
group directory; nothing refuses that, but such a root's own `kustomization.yaml` is not a valid
kustomize entry point.

Before go-kure/kure#771, `FullRepoPath()` dropped `Name` whenever `Namespace` ended with it as a
string, and callers commonly set `Namespace` to the full path including the child's own name. That
rule also collapsed layouts that merely shared a name or a name suffix onto one directory, losing
resources from the kustomize graph. A caller still joining the child's `Name` into its `Namespace`
now gets that name twice (`.../<name>/<name>`); pass the parent's path instead.

Every writer (`WriteToDisk`, `WriteToTar`, `WriteManifest`) checks the whole tree before writing
anything, and refuses two layouts that resolve to the same directory, or two `AppFileSingle`
layouts that resolve to the same file, or an `AppFileSingle` child with children of its own or
with `ConfigMapGenerators`, or any other layout it writes no `kustomization.yaml` for that has
`ConfigMapGenerators`.
It also refuses an `AppFileSingle` layout whose file, `<Name>.yaml`, or one of whose extra files
would replace another file written into the same directory: the `kustomization.yaml` there, one of
the resource or extra files of the layout that owns the directory, or another `AppFileSingle`
layout's file (go-kure/kure#871). A child named `kustomization`, or
named like one of its parent's generated files (`default-configmap-a`, or `configmap` under
`FileNamingKindName` + `FilePerKind`), is refused, as is an `AppFileSingle` root named
`kustomization`. A child with no resources writes no `<Name>.yaml`, so its name is never refused as
replacing another file; its directory, which the writers still create, and its extra files are
checked.
Nor may such a file need a directory where another layout writes a file, or be a file where
another layout needs a directory (go-kure/kure#878): an extra file `default-configmap-a.yaml/v.yaml`
next to the parent's `default-configmap-a.yaml`, an extra file `sub` where a child layout's
directory `sub` is (or a directory below it), or an extra file `x` of one `AppFileSingle` layout
and `x/y.yaml` of another. Nor may a layout's directory, including the one the writers create for
an `AppFileSingle` layout without resources, be or lie beneath a file another layout writes.
Before this, `WriteToDisk` and `WriteManifest` failed partway with `not a directory`, and
`WriteToTar` wrote an archive `tar` could not extract. Of the kustomize control files only
`kustomization.yaml` is written, so a directory named `kustomization`, `Kustomization` or
`kustomization.yml` clashes with nothing. An `AppFileSingle` layout's extra file inside a sibling
layout's directory (`sub/v.yaml` next to its parent's directory child `sub`) is written; that
directory's `kustomization.yaml` does not list it. A layout's own extra file inside its own
child's directory is refused (see [Extra Files and ConfigMap Generators](#extra-files-and-configmap-generators)).
Directories and file names are compared case-insensitively, as on default macOS volumes: an
`AppFileSingle` child named `Kustomization`, or `Default-ConfigMap-A`, is refused too, and so is a
directory child `Default-ConfigMap-A.yaml` next to its parent's `default-configmap-a.yaml`.

The writers also refuse two layouts that hold an object with one kustomize identity when one
kustomize build takes in both (go-kure/kure#880): kustomize refuses the second copy (`may not add
resource with an already registered id`), so that build would fail. The identity is kustomize's
own: group, version, kind, name and namespace, where an omitted namespace counts as `default` and
a kind kustomize knows is cluster-scoped (such as `Namespace`) has none, even when the object sets
one; so one layout holding such an object with and without a namespace field is refused too. Two versions of one object (`autoscaling/v1` and `autoscaling/v2`) are two identities, and
kustomize builds both. A build is every `kustomization.yaml` a writer writes, with the files and the
`AppFileSingle` child files it lists and, recursively, the build of each child directory it lists;
and the Flux build of a `KustomizationRecursive` directory marked with `SetFluxBuild` (see
"Kustomization Generation"). The error names both layouts and the innermost build. A child its
parent does not list (an umbrella child, one that renders bundles, a directory child of a
`FluxIntegratedPerLayout` parent, or under `WriteToDisk` and `WriteToTar` a directory child of
another package) is a build of its own, so it may hold an object its parent holds. An
`AppFileSingle` child of such a parent is still listed, so its objects count in the parent's build.

The ConfigMap a `configMapGenerator` generates counts as an object of the layout whose
`kustomization.yaml` holds the generator (go-kure/kure#894): `v1` `ConfigMap`, the generator's
name and no namespace, so `default`. kustomize compares the name before the content-hash suffix, so
two generators of one name are one identity whatever their files hold. It refuses a generator
whose ConfigMap its own kustomization already holds (`id ... exists; ... behavior must be merge or
replace`; kure writes no `behavior`), and one object reaching a kustomization twice from its
resources, the generator's ConfigMap included (`may not add resource with an already registered
id`). So a generator named `x` is refused next to a ConfigMap `default/x`, or one without a
namespace, anywhere in the same kustomize build (its own layout, a listed child, the parent that
lists it, or a sibling that parent lists), and so are
two generators named `x` in one layout or in two layouts of one build. A ConfigMap `x` in another
namespace, or a generator `x` in a child its parent does not list, is written.

### 2. LayoutRules Configuration
- **NodeGrouping**: whether each child node gets a directory (`GroupByName`, default) or merges into its parent's (`GroupFlat`; the root keeps its directory)
- **BundleGrouping**: whether each bundle gets a directory inside its node's (`GroupByName`) or renders in the node's directory (`GroupFlat`, default)
- **ApplicationGrouping**: whether each application gets a directory inside its bundle's (`GroupByName`) or writes into the bundle's directory (`GroupFlat`, default)

The three axes are independent and apply the same way to the root node (also under a
`ClusterName`), to umbrella children and in `WalkClusterByPackage`. A level set to `GroupFlat` is
rendered into the layout above it: its resources, its child layouts and its origins. Umbrella child
bundles and augmenter applications always keep their own directory, because each carries its own
Flux Kustomization or writer-owned files. `FilePer` is honoured on every layout. The writers refuse
a merge that makes two layouts claim one directory, and a directory that would hold two objects
with the same group, kind, namespace and name (kustomize cannot build it); objects that merely
share a file name are written into one multi-document file, as `FilePerKind` intends.
- **FilePer**: How resources are written (FilePerResource vs FilePerKind)
- **FluxPlacement**: Where/at what granularity Flux Kustomizations go — `FluxSeparate`, `FluxIntegratedPerLayout` (a CR per layout node), or `FluxIntegratedPerBundle` (CRs at bundle boundaries; application children included as directories)
- **FileNaming**: Resource file naming pattern (see [File Naming Modes](#file-naming-modes))
- **ClusterName**: Optional cluster name prefix for cluster-aware directory paths

### 3. Two Main Walker Functions
- **WalkCluster()**: Standard hierarchical layout (Node → Bundle → App structure)
- **WalkClusterByPackage()**: Groups by PackageRef for multi-source scenarios

### 4. Writing System
- **WriteManifest()**: Config-driven writing — uses `Config` to resolve file naming, kustomization mode, and directory structure
- **WriteToDisk()**: Self-contained method on ManifestLayout — uses the layout's own `FileNaming` and `FluxPlacement` fields
- **WriteToTar()**: Same as WriteToDisk but writes to a tar archive for OCI artifact consumers
- **WritePackagesToDisk()**: Package-based writing with sanitized directory names
- All writers auto-generate kustomization.yaml files with proper resource references

## Directory Structure Patterns

### Standard Layout (WalkCluster)
```
clusters/
  cluster-name/
    node1/
      bundle1/
        app1/
          manifest-files.yaml
          kustomization.yaml
        app2/...
      bundle2/...
    node2/...
```

### Package-Based Layout (WalkClusterByPackage)  
```
oci-packages/
  web/
    app-manifests.yaml
git-packages/
  monitoring/
    app-manifests.yaml
```

### Flat Layout (GroupFlat rules)
```
clusters/
  cluster-name/
    all-manifests-together.yaml
    kustomization.yaml
```

## GitOps Tool Compatibility

### Flux Integration
- Every Kustomization `spec.path` is the directory of the layout that renders the bundle,
  e.g. `cluster-name/node` (see [Layout origins](#layout-origins))
- Auto-generates kustomization.yaml files
- Handles FluxSeparate, FluxIntegratedPerLayout and FluxIntegratedPerBundle placement modes

### ArgoCD Integration  
- `spec.source.path` is the same layout directory, e.g. `cluster-name/node`
- Requires explicit kustomization.yaml files (no auto-discovery)
- Each target directory needs its own Application

## Advanced Features

### Package Reference Support
- Tracks different source types (OCIRepository, GitRepository, Bucket)
- Enables multi-source deployments with proper isolation
- Sanitizes package keys into valid directory names

### Flexible File Organization
- **FilePerResource**: Each K8s object gets its own file
- **FilePerKind**: Group objects by Kind (all Services together, etc.)
- **AppFileSingle**: All app resources in one file

### File Naming Modes

Controls how resource YAML files are named:

| Mode | Format | Example |
|------|--------|---------|
| `FileNamingDefault` | `{namespace}-{kind}-{name}.yaml` | `default-service-web.yaml` |
| `FileNamingKindName` | `{kind}-{name}.yaml` | `service-web.yaml` |

`FileNamingKindName` drops the namespace prefix, which is useful when each application already has its own directory (e.g., Pattern A / CentralizedControlPlane). The naming mode is propagated through all writers: `WriteManifest`, `WriteToDisk`, and `WriteToTar`.

### Kustomization Generation
- **KustomizationExplicit**: Lists all manifest files explicitly
- **KustomizationRecursive**: Writes no `kustomization.yaml` into the layout's directory; every
  other file is written as in Explicit mode. The directory is meant for a Flux Kustomization,
  which generates the `kustomization.yaml` from every `.yaml` and `.yml` file below it, adding a
  subdirectory that has its own kustomization file as a whole (go-kure/kure#868). Every writer
  refuses, before writing anything:
  - `ConfigMapGenerators` on a Recursive layout: they exist only inside a `kustomization.yaml`;
  - a parent's `kustomization.yaml` that lists a Recursive layout's directory, which kustomize
    cannot build without one.

  A Flux Kustomization kure generated builds a directory when `SetFluxBuild` marks it: the fluxcd
  integrator marks each generated `spec.path` layout, and the root, which the Flux bootstrap
  applies. Nothing else marks: a caller placing Flux Kustomizations from `GenerateFromLayout` or its
  own code calls `SetFluxBuild` on their `spec.path` layouts to get these checks.

  A marked Recursive directory that holds no file is refused (go-kure/kure#904): no resource file,
  extra file, `AppFileSingle` child file, file of a layout below it or `kustomization.yaml` a
  directory below it gets lands at or below it. The writers write it no `kustomization.yaml`, so
  `WriteManifest` returns no entry for its path and `WriteToDisk` and `WriteToTar` write an empty
  directory, which a Git tree drops: the Kustomization would name a path the output does not
  contain. This happens to a bundle with no applications yet, one whose applications were all
  removed, or one whose applications render nothing, when the whole tree is Recursive. Give the
  layout a resource, or write it `KustomizationExplicit`, which writes it `resources: []`.

  For a marked Recursive directory, the writers also refuse what would make its build differ from the Explicit
  mode's (its build is its own directory and every directory below it that no `kustomization.yaml`
  shields):
  - another marked layout below it with no `kustomization.yaml` in any directory in between (its
    own does not count): its objects would be applied twice;
  - an extra file named `*.yaml` or `*.yml` in its build: Flux would apply it, or fail the whole
    build on one it cannot decode, and Explicit mode never lists extra files;
  - an extra or resource file named like a kustomization file (`kustomization.yaml`,
    `kustomization.yml`, `Kustomization`) in its build: Flux would build the directory holding it
    as a kustomization;
  - a resource file whose name does not end in `.yaml` or `.yml` (only a `Config.ManifestFileName`
    can produce one): Flux would skip it, where Explicit mode lists it;
  - the file of an `AppFileSingle` umbrella child, or of one that renders bundles, in its build:
    Flux would apply it, where Explicit mode leaves it to the child's own Kustomization;
  - the file of an `AppFileSingle` child in its build that a `kustomization.yaml` at or above the
    Recursive directory lists (the child's parent lists it by its path below the parent's
    directory, see "Layout paths"): both builds would apply it;
  - a child directory its parent's `kustomization.yaml` would not list, in its build: under
    `WriteToDisk` and `WriteToTar`, a child of another package (its `PackageRef` and its parent's
    are both set, with different values). Flux would apply
    it, where Explicit mode leaves it out. `WriteManifest` lists such a child, so it is accepted
    there.

  Two layouts in a marked Recursive directory's build that hold one object are refused as well,
  as in any other build (see "Layout paths"): that fails the Flux build and the Explicit one alike.

  So Recursive fits a generated target with no other target below it: a leaf bundle directory,
  or a GroupByName bundle directory over the application directories it lists, plus any unmarked
  Recursive layout inside one. A Recursive layout with children under `FluxIntegratedPerLayout`
  is refused (each child is a target), and so is a Recursive root over any target no
  `kustomization.yaml` shields. How Argo CD or a caller's own Kustomization treats a Recursive
  directory is not checked: Argo CD Applications do not set `directory.recurse`
  (go-kure/kure#144), and an unmarked Recursive directory holding no file is accepted, although
  Git does not keep an empty directory.
- A `FluxIntegratedPerLayout` layout references no child directory: each child is applied by the
  Flux Kustomization the integrator placed in the parent's `Resources`, listed as one of its own
  files. No reference is derived from a child's name.
- No layout references a child that renders bundles, in any placement: that child is a
  reconciliation unit, applied by its own Flux Kustomization or ArgoCD Application and by nothing
  else, so every object has one owner. Building a parent directory therefore does not include its
  child units; a tree written without Flux or ArgoCD integration has no CR applying them.
- GroupByName bundle layouts are written `KustomizationExplicit`, so the CRs hosted there
  (umbrella children, PerLayout applications) are listed
- A layout that renders a bundle, and every child of a `FluxIntegratedPerLayout` layout, always
  gets a `kustomization.yaml` unless it is `KustomizationRecursive`, even when it holds nothing else (a bundle with no applications, an
  empty node, an empty augmenter layout): a Flux Kustomization or ArgoCD Application names that
  directory, and an empty directory does not survive a Git tree. A kustomization that lists
  nothing is written `resources: []` (kustomize rejects a bare `resources:` as empty). A
  Recursive one that holds nothing and that a generated Flux Kustomization names is refused, as
  described above.
- A `resources` entry, a `configMapGenerator` name and a generator's `files` entry are written
  plain when kustomize reads them back as the same string, and double-quoted otherwise
  (go-kure/kure#896). kustomize reads a `kustomization.yaml` as YAML 1.1, so a child directory or
  generator named `y`, `on`, `null` or `1e3` would otherwise be read as a bool, null or number and
  fail the build.
- An `AppFileSingle` child writes one file, `<Name>.yaml`, into its `Namespace`, normally its
  parent's directory, and never a
  `kustomization.yaml` of its own (before go-kure/kure#860 it replaced the parent's, dropping the
  parent's files), and its file may not take the name of a file the parent writes there (see
  "Layout paths"). The parent's `kustomization.yaml` lists that file next to its own, by its path
  relative to the parent's directory (`sub/<Name>.yaml` for a child below it), and the writers
  refuse a listed file outside that directory or in a spelling of it that differs only in case; in
  `WriteManifest` the child's mode is its effective one, so a child with no mode of its own under
  `ArgoProfile`'s `AppFileSingle` is listed as `<Name>.yaml`, not as a directory, and the unnamed
  cluster root, which `WriteManifest` otherwise leaves without a `kustomization.yaml`, gets one
  when it holds such a file. A child with no resources writes no file, so nothing lists it and
  it gives the cluster root no `kustomization.yaml`.
  An `AppFileSingle` root, with no parent to list its file, still writes a `kustomization.yaml`
  next to it, which lists its children in that same directory (see "Layout paths").
- Every writer refuses an `AppFileSingle` child that has children of its own before writing
  anything: it writes no `kustomization.yaml`, so nothing would list them and they would drop out
  of the build. The walkers never build one themselves; the root is not checked, since the synthetic cluster
  wrapper a walker builds takes `ArgoProfile`'s `AppFileSingle` in `WriteManifest`.
- Every writer likewise refuses an `AppFileSingle` child, with or without resources, that carries
  `ConfigMapGenerators` (go-kure/kure#891): a `configMapGenerator` exists only inside a
  `kustomization.yaml`, the child writes none, and before this its generators were silently
  dropped. They are not moved into the parent's `kustomization.yaml`. In `WriteManifest` this
  includes any application layout written as one file through `Config` (`ArgoProfile`, or
  `ApplicationFileMode: AppFileSingle` set explicitly) that an augmenter gives generators: under
  `WalkCluster` with every Flux placement but `FluxIntegratedPerLayout`, and under
  `WalkClusterByPackage` with every placement, since it clears the placement. An augmenter that also sets
  its layout's `ApplicationFileMode` to `AppFilePerResource` keeps its generators: the layout gets
  its own directory and `kustomization.yaml`.
- The same holds for any other layout the writer writes no `kustomization.yaml` for
  (go-kure/kure#899). That is an `AppFileSingle` root with no resources and no children, in every
  writer, including one made `AppFileSingle` through `Config` in `WriteManifest`. In
  `WriteManifest` it is also a layout shaped like the synthetic cluster root (`Name ""`, a
  single-segment `Namespace`) with no resource file, no bundle and no `AppFileSingle` child that
  writes a file. A `KustomizationRecursive` root, `AppFileSingle` or not, is refused as
  Recursive. Each is refused before anything is written when it carries `ConfigMapGenerators`,
  which were silently dropped before. Under `WalkCluster` with `FlattenSingleTier`,
  `WriteToDisk` and `WriteToTar` now refuse an augmenter app that is the only one, emits no
  objects and sets its layout `AppFileSingle`: flattening moves it and its generators onto the
  root. `WriteManifest` already refuses that root, since it renders a node or bundle and cannot
  be written as `AppFileSingle`. Give such a root a resource, or put the
  generators on a layout that writes a `kustomization.yaml`.

### Layout origins

Every layout the walkers build records what it renders: `OriginNodes()`, `OriginBundles()` and
`OriginApplication()`. A node layout renders its node (plus its bundle when `BundleGrouping` is
`GroupFlat`); a GroupByName bundle layout and an umbrella-child layout render their bundle; a
per-app layout its application. A level merged by a `GroupFlat` axis, and a `FlattenSingleTier`
collapse, move the absorbed nodes and bundles into the absorbing layout's origins; a merged
application has no origin of its own. Hand-built layouts have none.
`OriginBundleObjects(b)` returns the objects bundle `b`'s applications render in that directory
or its per-app directories (plus a kind-and-name stand-in for each ConfigMap an augmenter's
`configMapGenerator` makes): the Flux generator uses it to refuse a patch that would reach another
bundle sharing the directory.

`IndexOrigins(root, cluster)` resolves a cluster's bundles and nodes to those layouts
(`BundleLayout`, `NodeLayout`, `Parent`, `Bundles` in layout pre-order) and
`KustomizationPath(b)` is the directory of the layout that renders `b` — the one path every Flux
Kustomization and ArgoCD Application kure emits for a bundle uses. It refuses a tree it cannot
resolve: an object rendered twice, a rendered set that differs from what the cluster reaches
(hand-built, partial or other-cluster trees), two bundles with one name, a node or bundle
layout set to `AppFileSingle` mode, and a dependency cycle between units. `WriteManifest` refuses a
node or bundle layout whose own `ApplicationFileMode` is `AppFileSingle` too: its files would go
into its `Namespace`, so no directory would exist at its path. `Config.ApplicationFileMode` is only
the default for application (and hand-built) layouts, so `ArgoProfile`'s `AppFileSingle` writes one
file per application while nodes and bundles keep their directories.

`Units()` returns the layouts that render bundles: each is one reconciliation unit, the directory a
Flux Kustomization or ArgoCD Application applies. `UnitName(b)` is the unit that applies `b` (the
first bundle its directory renders), and `UnitDependencies(l)` / `UnitNamedDependencies(l)` map the
bundles' `DependsOn` / `NamedDependsOn` to units, dropping dependencies inside `l`'s own unit.
`IndexOrigins` refuses a cycle in those dependencies; waits a workflow adds on top (Flux health
checks, creation order) are checked by that workflow.
Bundles are resolved by name, which is unique, so a copy of a bundle resolves like the original.

`NodeGrouping: GroupFlat` (as in the `CentralizedControlPlane` preset) moves a merged node's
umbrella children and augmenter layouts under the absorbing node, where they keep their own
directories, files and Flux CRs.

### Extra Files and ConfigMap Generators

`ManifestLayout.ExtraFiles` lets callers attach arbitrary files (e.g. a `values.yaml`) into a layout's directory alongside the resource YAMLs. `ManifestLayout.ConfigMapGenerators` adds entries to a `configMapGenerator:` section in the generated `kustomization.yaml`. kustomize appends a content-hash suffix to the generated ConfigMap name and rewrites references (e.g. `HelmRelease.spec.valuesFrom`) on build, so any change to the source file forces re-reconciliation — the canonical FluxCD pattern for tracking Helm values changes.

A generator needs a `kustomization.yaml` the writer writes, so the writers refuse one on a
`KustomizationRecursive` layout, an `AppFileSingle` child, or a root that writes no
`kustomization.yaml`, and they refuse a generated ConfigMap
whose identity its build already holds (see "Layout paths").

An `ExtraFile.Name` is a relative path of `/`-separated segments made of letters, digits, `.`, `_`
and `-`, with no `.` or `..` segment; a name in a subdirectory (`assets/dashboard.json`) creates
that directory. Every writer (`WriteToDisk`, `WriteManifest`, `WriteToTar`) refuses, before writing
any file of the tree, an extra file that would take a path it owns in that directory: a generated
resource file, a kustomize control file (`kustomization.yaml`, `kustomization.yml`,
`Kustomization`), a child layout's output directory where it lies inside this layout's (umbrella
children included; anything under it, or a file on the way to it) or the `<name>.yaml` of an
`AppFileSingle` child with resources, or another extra file (listed twice); nor may it use any of those files as
a directory (`kustomization.yaml/x`), or be a file where one of them needs a directory. A child's
output directory is the one its writer uses (for `WriteManifest`, after applying the `Config`
defaults). All names are compared case-insensitively, as on default macOS volumes. When a layout
has extra files, a `..` segment in its `Namespace` or `Name`, in a direct child's `Namespace` or
`Name`, or in a generated file name refuses the tree before any file is written; a
rooted name (`/x`) is not refused, since every writer joins it under its base. Previously such an
extra file silently replaced the generated one on disk, or shadowed it as a later tar entry, while
`kustomization.yaml` still listed the path.

`LayoutAugmenter` is an optional interface on `stack.ApplicationConfig`:

<!-- doc-example:excerpt the interface declaration, not a call -->
```go
type LayoutAugmenter interface {
    AugmentLayout(layout *ManifestLayout) error
}
```

When `app.Config` implements it, the walker invokes `AugmentLayout` on the per-app `ManifestLayout` after resource generation, giving the config a chance to attach `ExtraFiles`, `ConfigMapGenerators`, and sub-`ManifestLayout` children. It runs on every per-app layout a walker creates, in both walkers and inside umbrella children — with `ApplicationGrouping: GroupFlat` an augmenter app still gets its own per-app sub-layout instead of merging into its bundle's directory.

`LayoutIntentAugmenter` is an optional companion to `LayoutAugmenter`, for a config whose desire for its own layout varies per instance rather than being fixed for the whole type:

<!-- doc-example:excerpt the interface declaration, not a call -->
```go
type LayoutIntentAugmenter interface {
    LayoutAugmenter
    WantsOwnLayout() bool
}
```

`WantsOwnLayout()` gates placement only, and only where `ApplicationGrouping` is `GroupFlat` (in
`WalkCluster` and `WalkClusterByPackage` alike, umbrella children included):

| `ApplicationGrouping` | `WantsOwnLayout()` absent or `true` | `WantsOwnLayout() == false` |
|---|---|---|
| `GroupByName` | own layout + `AugmentLayout` | own layout + `AugmentLayout` |
| `GroupFlat` | own child layout + `AugmentLayout` | resources merged into the bundle's directory, `AugmentLayout` **not** called (no layout exists to pass it) |

A config that implements only `LayoutAugmenter` keeps today's presence-only behaviour unchanged.

#### Sub-Layout Children and Flux Integration

Augmenters may attach sub-layouts as `Children` of a per-app `ManifestLayout`. In `FluxIntegratedPerLayout` mode each such child that is eligible (see below) receives a Flux `Kustomization` CR automatically placed in the parent layout's `Resources`.

**Eligibility for CR generation.** A child layout receives a Flux `Kustomization` CR when ALL of the following hold:

- Integration runs with `FluxIntegratedPerLayout`.
- `!child.UmbrellaChild`
- `child.ApplicationFileMode != AppFileSingle`
- The child renders no bundle (a child that does already has that bundle's CR in the parent).
- A source can be resolved: the `SourceRef` of the nearest layout at or above the parent that renders bundles, else the one `SourceRef` the URL-less bundles below the child share. A missing or incomplete `SourceRef` (nil, empty, or without `Kind` or `Name`) causes `IntegrateWithLayout` to return a hard error — a `Kustomization` without `spec.sourceRef` is rejected by Flux.

The parent's `kustomization.yaml` lists that CR file as one of its own resources, so every child the writers do not reference as a directory has a backing CR. The integrator applies this rule at any depth.

#### Naming Constraint

Child layout `Name` is used as the Flux `Kustomization` CR name in `FluxIntegratedPerLayout` mode. Flux `Kustomization` CRs live in the `flux-system` namespace, so names must be **globally unique across all apps in the cluster** — two CRs with the same `metadata.name` collide, and the integrator refuses such a tree rather than skipping one of them.

Augmenters are responsible for ensuring uniqueness. The recommended convention is to prefix each child name with the app name: `{appName}-{hookGroupDir}` (e.g. `nginx-00-pre-install`).

#### DependsOn

Set `ManifestLayout.DependsOn` to a list of sibling layout names. In `FluxIntegratedPerLayout` mode the layout integrator translates these into `spec.dependsOn` entries on the child's `Kustomization` CR, enabling ordered reconciliation between hook groups (e.g. pre-install → hooks → post-install).

### ClusterName-Aware Layouts

Setting `LayoutRules.ClusterName` prepends the cluster name as a root directory, producing paths like `{clusterName}/{nodeName}/...` instead of `{nodeName}/...`. This is useful when a single repository manages multiple clusters. When the last segment of `ClusterName` is the root node's name (for example `ClusterName` `platform` with a root node `platform`), the root node is the cluster directory itself: the output is `platform/...`, not `platform/platform/...`.

Without a `ClusterName` the root node sits at `{rootName}` and every child node nests under it (`{rootName}/{childName}/...`), which is also where the Flux bootstrap sync path `./{rootName}` points. An unnamed root node has no directory of its own and stays at `cluster`, with its children at `cluster/{childName}` as before; deeper descendants now nest under them (`cluster/{childName}/{grandchildName}`), where they used to be written outside `cluster/`, unreferenced. `WalkClusterByPackage` places each package's root the same way, below the package directory; a package whose root node belongs to another package has no root directory of its own either, so its unnamed wrapper stays at `cluster`. A node outside the package adds no directory of its own: its in-package descendants attach to the nearest in-package ancestor (or the package root), including where the tree leaves a package and re-enters it lower down.

### Flatten Single Tier (opt-in)

`LayoutRules.FlattenSingleTier` collapses one vestigial intermediate directory layer when the wrapping Node adds no semantic value. Typical case: a flat single-bundle app whose caller wraps the Bundle in an extra `apps` Node, producing `cluster-name/apps/manifests.yaml` where the `apps/` layer is redundant. Enabling the flag yields `cluster-name/manifests.yaml` directly.

Conservative collapse preconditions — ALL must hold:

- `LayoutRules.FlattenSingleTier` is `true`.
- The parent layout is top-level (`Namespace` has no path separator).
- Parent has exactly one `Children` entry.
- Parent has no own `Resources`.
- The single child is not an `UmbrellaChild`.
- The single child has no `Children` of its own (terminal layer).

Multi-tier apps with sub-Kustomizations are unaffected: the precondition that the child be terminal preserves them. Empty containers (`only-Children`) are also unaffected: the precondition requiring the parent to have no own resources doesn't apply to them.

The absorbing layout takes over the collapsed layout's origins (see [Layout origins](#layout-origins)), so the Flux Kustomization and ArgoCD Application generated from the tree name the surviving directory; when parent and child each carried a bundle, the two share that directory's one Kustomization (see the fluxcd README, "One Kustomization per directory"). Nothing is rewritten after generation: a Flux CR a caller adds to the walked tree keeps the `spec.path` it was given.

Scoped to `WalkCluster`. `WalkClusterByPackage` is unaffected — its synthetic unnamed wrappers express package boundaries that the flatten helper would otherwise erroneously collapse.

Default: `false` — no behaviour change for existing callers.

## Layout Presets

Three named presets provide pre-configured LayoutRules for common deployment patterns. Use `LayoutRulesForPreset()` to get rules, or `ConfigForPreset()` to get a matching Config.

| Preset | Pattern | FluxPlacement | NodeGrouping | FileNaming |
|--------|---------|---------------|--------------|------------|
| `CentralizedControlPlane` | A | FluxSeparate | GroupFlat | FileNamingKindName |
| `SiblingControlPlane` | B | FluxSeparate | GroupByName | FileNamingDefault |
| `ParentDeployedControl` | C | FluxIntegratedPerLayout | GroupByName | FileNamingDefault |

This block and the one under [Example Usage](#example-usage) are the bodies of `Example` functions
in `example_test.go`, which `go test` runs; the two interface blocks above are declarations, shown
as excerpts. The examples import this package as `layout`, `os`, `path/filepath`, and `fmt` for the
lines that print what they built. Two helpers are declared in the same file: `exampleCluster()`
builds cluster `prod`, whose root node `apps` holds one bundle `web` with applications `api` and
`ui`, each emitting a ConfigMap; `printFiles(dir)` prints every file below `dir`.

<!-- doc-example: pkg/stack/layout ExampleLayoutRulesForPreset -->
```go
rules, err := layout.LayoutRulesForPreset(layout.PresetCentralizedControlPlane)
if err != nil {
    panic(err)
}
cfg, err := layout.ConfigForPreset(layout.PresetCentralizedControlPlane)
if err != nil {
    panic(err)
}
fmt.Println(rules.FluxPlacement, rules.NodeGrouping, rules.FileNaming, cfg.KustomizationFileName("web"))
```
<!-- doc-example:end -->

## Real-World Use Cases

1. **Simple Cluster**: Single source, hierarchical structure
2. **Multi-OCI Deployment**: Different services from different OCI registries  
3. **Monorepo**: Everything flattened into minimal directory structure
4. **Bootstrap Scenarios**: Special handling for Flux/ArgoCD system components

## Example Usage

<!-- doc-example: pkg/stack/layout ExampleWalkCluster -->
```go
cluster := exampleCluster()
out, err := os.MkdirTemp("", "kure-layout-example")
if err != nil {
    panic(err)
}
defer func() { _ = os.RemoveAll(out) }()

// Create layout rules
rules := layout.DefaultLayoutRules()
rules.BundleGrouping = layout.GroupFlat
rules.ApplicationGrouping = layout.GroupFlat

// Walk cluster to create layout
ml, err := layout.WalkCluster(cluster, rules)
if err != nil {
    panic(err)
}

// Write to disk
cfg := layout.DefaultLayoutConfig()
err = layout.WriteManifest(filepath.Join(out, "out/manifests"), cfg, ml)
if err != nil {
    panic(err)
}
printFiles(out)
```
<!-- doc-example:end -->

## Key Files

- **types.go**: Core types and configuration options
- **walker.go**: Tree traversal algorithms (WalkCluster, WalkClusterByPackage)
- **manifest.go**: ManifestLayout structure and package-based writing
- **write.go**: Standard manifest writing with kustomization generation  
- **config.go**: Configuration and file naming conventions

The layout module essentially bridges the gap between Kure's programmatic resource construction and the file-based expectations of GitOps workflows, with extensive configurability for different organizational preferences and tool requirements.
