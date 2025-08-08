# Stack Module Design Document

## Overview

The Stack module provides the core domain model for Kure's hierarchical configuration system. It defines a tree structure (Cluster → Node → Bundle → Application) that represents complete Kubernetes deployments organized for GitOps workflows.

## Current Architecture

### Hierarchy Structure

The stack follows a four-level hierarchy:

```
Cluster                    # Top-level configuration
└── Node                   # Hierarchical packaging unit
    └── Bundle             # Deployment unit (Flux Kustomization)
        └── Application    # Single deployable app
```

### Core Components

#### 1. Cluster (`pkg/stack/cluster.go`)
- **Purpose**: Represents complete cluster configuration
- **Contains**: Root Node and GitOps bootstrap configuration
- **Current Structure**:
  ```go
  type Cluster struct {
      Name   string        `yaml:"name"`
      Node   *Node         `yaml:"node,omitempty"`
      GitOps *GitOpsConfig `yaml:"gitops,omitempty"`
  }
  ```

#### 2. Node (`pkg/stack/cluster.go`)
- **Purpose**: Hierarchical tree structure for packaging
- **Contains**: Bundles and child Nodes
- **Features**: 
  - Package references for OCI/Git artifacts
  - Hierarchical path management
  - Dependency relationships

#### 3. Bundle (`pkg/stack/bundle.go`)  
- **Purpose**: Unit of deployment (maps to Flux Kustomization)
- **Contains**: Multiple Applications
- **Features**:
  - Flux reconciliation settings (interval, source)
  - Dependency management
  - Label propagation

#### 4. Application (`pkg/stack/application.go`)
- **Purpose**: Single deployable application
- **Contains**: ApplicationConfig interface
- **Current GVK Support**: ✅ Already implemented

### ApplicationConfig System

The ApplicationConfig interface provides pluggable resource generation:

```go
type ApplicationConfig interface {
    Generate(*Application) ([]*client.Object, error)
}
```

**Current Generators** (with GVK support):
- `generators.gokure.dev/v1alpha1/AppWorkload` - Standard Kubernetes workloads
- `generators.gokure.dev/v1alpha1/FluxHelm` - Flux HelmRelease resources

## Planned GVK Integration

### Motivation

Currently, only the ApplicationConfig layer has GVK versioning. The upper layers (Cluster, Node, Bundle) are unversioned structs, which limits:

1. **API Evolution**: No clear versioning for schema changes
2. **Multiple Formats**: Different stack representation approaches
3. **Tooling Integration**: Different tools may prefer different formats
4. **Schema Validation**: No OpenAPI schema generation capability

### Proposed GVK Design

#### API Group Structure

**Domain**: `stack.gokure.dev`

```yaml
# Cluster Configuration
apiVersion: stack.gokure.dev/v1alpha1
kind: Cluster
metadata:
  name: production-cluster
spec:
  gitops:
    type: flux
    bootstrap:
      enabled: true
      fluxVersion: v2.2.0
  node:
    name: root
    # ... node specification

---
# Node Configuration (can be separate artifact)
apiVersion: stack.gokure.dev/v1alpha1
kind: Node
metadata:
  name: infrastructure
  namespace: flux-system
spec:
  packageRef:
    name: infra-packages
    version: v1.0.0
  bundles:
    - name: cert-manager
    - name: ingress-nginx
  children:
    - name: monitoring
      packageRef:
        name: monitoring-packages
        version: v2.1.0

---
# Bundle Configuration  
apiVersion: stack.gokure.dev/v1alpha1
kind: Bundle
metadata:
  name: cert-manager
  namespace: cert-manager
spec:
  interval: 10m
  sourceRef:
    kind: GitRepository
    name: fleet-infra
  dependsOn:
    - name: crds
  applications:
    - apiVersion: generators.gokure.dev/v1alpha1
      kind: AppWorkload
      metadata:
        name: cert-manager
      spec:
        # ... application config
```

#### Version Evolution Strategy

- **v1alpha1**: Initial implementation, API may change
- **v1beta1**: API stabilizing, backward compatibility within beta  
- **v1**: Stable API, backward compatibility guaranteed

### Implementation Architecture

#### Shared GVK Infrastructure

Create `internal/gvk` package with reusable components:

```go
// Generic GVK representation
type GVK struct {
    Group   string
    Version string
    Kind    string
}

// Generic registry for any GVK-enabled type
type Registry[T any] struct {
    factories map[GVK]func() T
    mu        sync.RWMutex
}

// Generic wrapper for type-aware unmarshaling
type TypedWrapper[T any] struct {
    APIVersion string            `yaml:"apiVersion"`
    Kind       string            `yaml:"kind"`
    Metadata   map[string]any    `yaml:"metadata"`
    Spec       T                 `yaml:"spec"`
}

// Common interfaces
type VersionedType interface {
    GetAPIVersion() string
    GetKind() string
}
```

#### Migration Plan

**Phase 1: Create Shared Infrastructure**
1. Extract generic GVK components to `internal/gvk`
2. Update generators to use shared infrastructure
3. Add comprehensive tests

**Phase 2: Add GVK to Stack Structs**
1. Create versioned wrappers for Cluster/Node/Bundle
2. Implement custom YAML marshaling/unmarshaling
3. Maintain backward compatibility during transition

**Phase 3: Migration & Validation**
1. Add schema validation for each version
2. Create migration utilities for existing configs
3. Update documentation and examples

## Current Workflow Integration

### Flux Workflow (`pkg/stack/fluxcd/`)

The Flux workflow engine generates:
- **Bootstrap resources**: Flux system components
- **Source resources**: GitRepository, OCIRepository, etc.
- **Kustomization resources**: From Bundle configurations

### ArgoCD Workflow (`pkg/stack/argocd/`)

The ArgoCD workflow engine generates:
- **Application resources**: From Bundle configurations
- **AppProject resources**: For grouping and RBAC

### Layout System (`pkg/stack/layout/`)

The layout system handles manifest organization:
- **Directory structure**: Hierarchical file layout
- **Resource grouping**: By namespace, type, or bundle
- **Dependency ordering**: Ensuring proper apply sequence

## Benefits of GVK Integration

### 1. **Versioned APIs**
- Clear schema evolution path
- Backward compatibility guarantees
- Migration tooling support

### 2. **Multi-Format Support**
```yaml
# Hierarchical format (current)
apiVersion: stack.gokure.dev/v1alpha1
kind: Cluster
spec:
  node:
    bundles: [...]

# Flat format (future)
apiVersion: stack.gokure.dev/v1beta1
kind: ClusterFlat
spec:
  bundles: [...]  # Direct bundle list
```

### 3. **Schema Validation**
- OpenAPI schemas for each version
- IDE support with autocompletion
- Runtime validation

### 4. **Tooling Integration**
- Different tools can support different versions
- Clear API contracts
- Automated conversion between versions

### 5. **Future Extensibility**
- Plugin system for custom stack types
- Alternative hierarchy models
- Integration with external tools

## Compatibility Considerations

### Backward Compatibility

During transition, support both formats:
```go
// Current direct struct usage
cluster := &stack.Cluster{
    Name: "prod",
    Node: &stack.Node{...},
}

// New GVK-based usage
var wrapper stack.ClusterWrapper
yaml.Unmarshal(data, &wrapper)
cluster := wrapper.ToCluster()
```

### Migration Path

1. **Phase 1**: Internal infrastructure (no breaking changes)
2. **Phase 2**: Add GVK support alongside existing APIs
3. **Phase 3**: Deprecate old APIs (with migration period)
4. **Phase 4**: Remove old APIs in next major version

## Testing Strategy

### Unit Tests
- GVK parsing and generation
- Registry functionality  
- Wrapper marshaling/unmarshaling
- Version compatibility

### Integration Tests
- Full stack configuration parsing
- Workflow engine compatibility
- Layout generation with GVK structs

### Migration Tests
- Conversion between formats
- Backward compatibility
- Schema validation

## Future Enhancements

### 1. **Advanced Versioning**
- Conversion webhooks for Kubernetes-style migration
- Automatic version detection and upgrade
- Version-specific optimizations

### 2. **Schema Management**
- OpenAPI schema generation
- JSON Schema validation
- Documentation generation from schemas

### 3. **Tooling**
- CLI commands for version management
- Migration utilities
- Validation tools

### 4. **Alternative Stack Models**
```yaml
# GitOps-native stack (future)
apiVersion: stack.gokure.dev/v2alpha1
kind: GitOpsStack
spec:
  repositories:
    - url: github.com/org/infra
      path: clusters/prod
  kustomizations: [...]
  helmReleases: [...]
```

## Implementation Status

- ✅ **ApplicationConfig GVK**: Complete
- 🔄 **Shared GVK Infrastructure**: In Progress (Phase 1)
- ⏳ **Stack Struct GVK**: Planned (Phase 2)
- ⏳ **Schema Validation**: Planned (Phase 3)

## Related Documentation

- [`generators/DESIGN.md`](generators/DESIGN.md) - ApplicationConfig generator system
- [`layout/README.md`](layout/README.md) - Manifest layout and organization
- [`fluxcd/README.md`](fluxcd/README.md) - Flux workflow integration
- [`argocd/README.md`](argocd/README.md) - ArgoCD workflow integration

---

*This design document is a living document that will be updated as the implementation progresses.*