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

### 2. LayoutRules Configuration
- **NodeGrouping**: How nodes are organized (GroupByName creates dirs, GroupFlat flattens)
- **BundleGrouping**: How bundles within nodes are organized  
- **ApplicationGrouping**: How applications within bundles are organized
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
- **WriteToTar()**: Same as WriteToDisk but writes to a tar archive (used by Crane for OCI artifacts)
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
  cluster/
    web/
      app-manifests.yaml
git-packages/
  cluster/
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
- Uses `spec.path: ./clusters/cluster-name/node` format
- Auto-generates kustomization.yaml files
- Supports recursive discovery of manifests
- Handles FluxSeparate vs FluxIntegratedPerLayout placement modes

### ArgoCD Integration  
- Uses `spec.source.path: clusters/cluster-name/node` format
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
- **KustomizationRecursive**: References subdirectories only
- Smart handling of cross-references and child relationships

### Extra Files and ConfigMap Generators

`ManifestLayout.ExtraFiles` lets callers attach arbitrary files (e.g. a `values.yaml`) into a layout's directory alongside the resource YAMLs. `ManifestLayout.ConfigMapGenerators` adds entries to a `configMapGenerator:` section in the generated `kustomization.yaml`. kustomize appends a content-hash suffix to the generated ConfigMap name and rewrites references (e.g. `HelmRelease.spec.valuesFrom`) on build, so any change to the source file forces re-reconciliation — the canonical FluxCD pattern for tracking Helm values changes.

`LayoutAugmenter` is an optional interface on `stack.ApplicationConfig`:

```go
type LayoutAugmenter interface {
    AugmentLayout(layout *ManifestLayout) error
}
```

When `app.Config` implements it, the walker invokes `AugmentLayout` on the per-app `ManifestLayout` after resource generation, giving the config a chance to attach `ExtraFiles`, `ConfigMapGenerators`, and sub-`ManifestLayout` children. Only invoked on per-app layouts produced by the non-flat (`GroupByName`) walker paths; `GroupFlat` and umbrella layouts merge resources into shared parent layouts and are not currently augmented.

#### Sub-Layout Children and Flux Integration

Augmenters may attach sub-layouts as `Children` of a per-app `ManifestLayout`. In `FluxIntegratedPerLayout` mode each such child that is eligible (see below) receives a Flux `Kustomization` CR automatically placed in the parent layout's `Resources`.

**Eligibility for CR generation.** A child layout receives a Flux `Kustomization` CR when ALL of the following hold:

- The ancestor node bundle's layout operates in `FluxIntegratedPerLayout` mode.
- `!child.UmbrellaChild`
- `child.ApplicationFileMode != AppFileSingle`
- The ancestor bundle has a non-nil, non-empty `SourceRef` with both `Kind` and `Name` set. A nil, empty struct, or incomplete `SourceRef` (missing either field) causes `IntegrateWithLayout` to return a hard error — a `Kustomization` without `spec.sourceRef` is rejected by Flux.

This rule mirrors exactly what the writers use to emit `flux-system-kustomization-{child.Name}.yaml` from the parent's `kustomization.yaml`, so every file reference the writers produce has a backing CR. The integrator applies this rule recursively: it covers both direct children of the node layout and augmenter-added sub-layouts at any depth.

#### Naming Constraint

Child layout `Name` is used as the Flux `Kustomization` CR name in `FluxIntegratedPerLayout` mode (matching the filename emitted by the writers: `flux-system-kustomization-{child.Name}.yaml`). Flux `Kustomization` CRs live in the `flux-system` namespace, so names must be **globally unique across all apps in the cluster** — two CRs with the same `metadata.name` collide.

Augmenters are responsible for ensuring uniqueness. The recommended convention is to prefix each child name with the app name: `{appName}-{hookGroupDir}` (e.g. `nginx-00-pre-install`).

#### DependsOn

Set `ManifestLayout.DependsOn` to a list of sibling layout names. In `FluxIntegratedPerLayout` mode the layout integrator translates these into `spec.dependsOn` entries on the child's `Kustomization` CR, enabling ordered reconciliation between hook groups (e.g. pre-install → hooks → post-install).

### ClusterName-Aware Layouts

Setting `LayoutRules.ClusterName` prepends the cluster name as a root directory, producing paths like `{clusterName}/{nodeName}/...` instead of `{nodeName}/...`. This is useful when a single repository manages multiple clusters.

### Flatten Single Tier (opt-in)

`LayoutRules.FlattenSingleTier` collapses one vestigial intermediate directory layer when the wrapping Node adds no semantic value. Typical case: a flat single-bundle app whose caller wraps the Bundle in an extra Node (e.g. crane's `apps` Node), producing `cluster-name/apps/manifests.yaml` where the `apps/` layer is redundant. Enabling the flag yields `cluster-name/manifests.yaml` directly.

Conservative collapse preconditions — ALL must hold:

- `LayoutRules.FlattenSingleTier` is `true`.
- The parent layout is top-level (`Namespace` has no path separator).
- Parent has exactly one `Children` entry.
- Parent has no own `Resources`.
- The single child is not an `UmbrellaChild`.
- The single child has no `Children` of its own (terminal layer).

Multi-tier apps with sub-Kustomizations are unaffected: the precondition that the child be terminal preserves them. Empty containers (`only-Children`) are also unaffected: the precondition requiring the parent to have no own resources doesn't apply to them.

When the layout participates in Flux integration, the flatten helper records redirect tables (`nodeAliases` for `findLayoutNode` lookups, `pathRewrites` for `Spec.Path` rewriting). `IntegrateWithLayout` consults the aliases during integrated placement and calls `ApplyFlattenPathRewrites(root)` before returning, regardless of placement mode (FluxIntegratedPerLayout or FluxSeparate). Direct callers using `WalkCluster` + `IntegrateWithLayout` (without going through `CreateLayoutWithResources`) get the rewrite for free.

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