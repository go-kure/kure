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
writes one file, `<Namespace>/<Name>.yaml`, into its parent's directory.

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
layouts that resolve to the same file. Directories are compared case-insensitively, as on default
macOS volumes.

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
- **FluxPlacement**: Where/at what granularity Flux Kustomizations go — `FluxSeparate`, `FluxIntegratedPerLayout` (a CR per layout node), or `FluxIntegratedPerBundle` (CRs at bundle boundaries; children included as directories)
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
- **KustomizationRecursive**: References subdirectories only — except that a
  `FluxIntegratedPerLayout` layout still lists the files holding the Flux objects it hosts (its
  children's Kustomizations and any Source generated for them), which stand in for its child
  references
- A `FluxIntegratedPerLayout` layout references no child directory: each child is applied by the
  Flux Kustomization the integrator placed in the parent's `Resources`, listed as one of its own
  files. No reference is derived from a child's name.
- GroupByName bundle layouts are written `KustomizationExplicit`, so the CRs hosted there
  (umbrella children, PerLayout applications) are listed
- A layout that renders a bundle, and every child of a `FluxIntegratedPerLayout` layout, always
  gets a `kustomization.yaml`, even when it holds nothing else (a bundle with no applications, an
  empty node, an empty augmenter layout): a Flux Kustomization or ArgoCD Application names that
  directory, and an empty directory does not survive a Git tree. A kustomization that lists
  nothing is written `resources: []` (kustomize rejects a bare `resources:` as empty). An
  `AppFileSingle` layout, which writes into its parent's directory, never gets one of its own

### Layout origins

Every layout the walkers build records what it renders: `OriginNodes()`, `OriginBundles()` and
`OriginApplication()`. A node layout renders its node (plus its bundle when `BundleGrouping` is
`GroupFlat`); a GroupByName bundle layout and an umbrella-child layout render their bundle; a
per-app layout its application. A level merged by a `GroupFlat` axis, and a `FlattenSingleTier`
collapse, move the absorbed nodes and bundles into the absorbing layout's origins; a merged
application has no origin of its own. Hand-built layouts have none.

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

An `ExtraFile.Name` is a relative path of `/`-separated segments made of letters, digits, `.`, `_`
and `-`, with no `.` or `..` segment; a name in a subdirectory (`assets/dashboard.json`) creates
that directory. Every writer (`WriteToDisk`, `WriteManifest`, `WriteToTar`) refuses, before writing
any file of the layout, an extra file that would take a path it owns in that directory: a generated
resource file, a kustomize control file (`kustomization.yaml`, `kustomization.yml`,
`Kustomization`), a child layout's output directory where it lies inside this layout's (umbrella
children included; anything under it, or a file on the way to it) or an `AppFileSingle` child's
`<name>.yaml`, or another extra file (listed twice); nor may it use any of those files as
a directory (`kustomization.yaml/x`), or be a file where one of them needs a directory. A child's
output directory is the one its writer uses (for `WriteManifest`, after applying the `Config`
defaults). All names are compared case-insensitively, as on default macOS volumes. When a layout
has extra files, a `..` segment in its `Namespace` or `Name`, in a direct child's `Namespace` or
`Name`, or in a generated file name refuses the layout before any of its files is written; a
rooted name (`/x`) is not refused, since every writer joins it under its base. Previously such an
extra file silently replaced the generated one on disk, or shadowed it as a later tar entry, while
`kustomization.yaml` still listed the path.

`LayoutAugmenter` is an optional interface on `stack.ApplicationConfig`:

```go
type LayoutAugmenter interface {
    AugmentLayout(layout *ManifestLayout) error
}
```

When `app.Config` implements it, the walker invokes `AugmentLayout` on the per-app `ManifestLayout` after resource generation, giving the config a chance to attach `ExtraFiles`, `ConfigMapGenerators`, and sub-`ManifestLayout` children. It runs on every per-app layout a walker creates, in both walkers and inside umbrella children — with `ApplicationGrouping: GroupFlat` an augmenter app still gets its own per-app sub-layout instead of merging into its bundle's directory.

`LayoutIntentAugmenter` is an optional companion to `LayoutAugmenter`, for a config whose desire for its own layout varies per instance rather than being fixed for the whole type:

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

```go
rules, err := layout.LayoutRulesForPreset(layout.PresetCentralizedControlPlane)
cfg, err := layout.ConfigForPreset(layout.PresetCentralizedControlPlane)
```

## Real-World Use Cases

1. **Simple Cluster**: Single source, hierarchical structure
2. **Multi-OCI Deployment**: Different services from different OCI registries  
3. **Monorepo**: Everything flattened into minimal directory structure
4. **Bootstrap Scenarios**: Special handling for Flux/ArgoCD system components

## Example Usage

```go
// Create layout rules
rules := layout.DefaultLayoutRules()
rules.BundleGrouping = layout.GroupFlat
rules.ApplicationGrouping = layout.GroupFlat

// Walk cluster to create layout
ml, err := layout.WalkCluster(cluster, rules)
if err != nil {
    return err
}

// Write to disk
cfg := layout.DefaultLayoutConfig()
err = layout.WriteManifest("out/manifests", cfg, ml)
```

## Key Files

- **types.go**: Core types and configuration options
- **walker.go**: Tree traversal algorithms (WalkCluster, WalkClusterByPackage)
- **manifest.go**: ManifestLayout structure and package-based writing
- **write.go**: Standard manifest writing with kustomization generation  
- **config.go**: Configuration and file naming conventions

The layout module essentially bridges the gap between Kure's programmatic resource construction and the file-based expectations of GitOps workflows, with extensive configurability for different organizational preferences and tool requirements.
