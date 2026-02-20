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
- **FluxPlacement**: Where Flux Kustomizations go (FluxSeparate vs FluxIntegrated)

### 3. Two Main Walker Functions
- **WalkCluster()**: Standard hierarchical layout (Node → Bundle → App structure)
- **WalkClusterByPackage()**: Groups by PackageRef for multi-source scenarios

### 4. Writing System
- **WriteManifest()**: Standard hierarchical writing
- **WritePackagesToDisk()**: Package-based writing with sanitized directory names
- Auto-generates kustomization.yaml files with proper resource references

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
- Handles FluxSeparate vs FluxIntegrated placement modes

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

### Kustomization Generation
- **KustomizationExplicit**: Lists all manifest files explicitly
- **KustomizationRecursive**: References subdirectories only
- Smart handling of cross-references and child relationships

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