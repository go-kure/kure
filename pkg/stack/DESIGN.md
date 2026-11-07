# Stack Module Design Document

> **Status**: Describes the shipped design. Proposals that were dropped (versioned stack structs, a
> `generators.gokure.dev` API group) are not described here; they remain in the git history.

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
  <!-- doc-example:excerpt the struct declaration from pkg/stack/cluster.go, not a call -->
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
  - Flux reconciliation settings (interval, timeout, retryInterval, source)
  - Interval, timeout and retry interval must parse as Go durations (see "Bundle Validation")
  - Dependency management (`DependsOn`) — ordering between sibling bundles
  - Umbrella composition (`Children`) — a parent Kustomization aggregates
    child readiness through `spec.healthChecks` (one entry per child) while
    `Bundle.Wait` is unset or false. See the "Umbrella Bundles" subsection
    below.
  - Label propagation

##### Umbrella Bundles (Containment vs Ordering)

`Bundle.DependsOn` and `Bundle.Children` answer different questions:

- `DependsOn` expresses **ordering** between siblings: "reconcile X only after
  Y is Ready". It does not change which resources either bundle applies.
- `Children` expresses **containment**: the parent bundle is a wrapper around
  its children. The parent's Flux Kustomization renders the child
  Kustomization CRs in its own path and, with `Bundle.Wait` unset or false,
  uses `spec.healthChecks` to aggregate each child's Ready condition into its
  own. The generator does not force `spec.wait`; with `Wait` true it writes no
  child entries (see `pkg/stack/README.md` and the Flux engine's "Umbrella
  Bundles"). The parent becomes Ready iff all children are Ready, giving
  downstream consumers a single stable anchor regardless of how many internal
  tiers the umbrella contains.

Children bundles must be **standalone**: a bundle referenced by `Children`
cannot simultaneously be the `Bundle` of any `stack.Node`. `ValidateCluster`
enforces this disjointness along with cycle detection, shared-ownership
rejection, and multi-package rejection (initial patch restriction).

#### 4. Application (`pkg/stack/application.go`)
- **Purpose**: Single deployable application
- **Contains**: ApplicationConfig interface

### ApplicationConfig System

The ApplicationConfig interface provides pluggable resource generation:

<!-- doc-example:excerpt the interface declaration from pkg/stack/application.go, not a call -->
```go
type ApplicationConfig interface {
    Generate(*Application) ([]*client.Object, error)
}
```

Kure ships no `ApplicationConfig` implementations of its own: each caller implements `ApplicationConfig` for its
application types. `pkg/stack/stacktest` checks an implementation against the generation contract.

### Bundle Validation

The Stack module includes comprehensive validation for Bundle configurations to ensure GitOps best practices and prevent deployment issues.

#### Interval Field Validation

All time-based fields in Bundle configurations are automatically validated:

- **`interval`**: GitOps reconciliation frequency
- **`timeout`**: Maximum wait time for resources to be ready  
- **`retryInterval`**: Frequency for retrying failed reconciliations

**Validation Rules:**
- **Format**: Must follow Go `time.Duration` syntax (`"10m"`, `"1h30m45s"`, `"2.5m"`). Empty means unset.
- **No range check**: `Bundle.Validate` only requires the value to parse (`pkg/stack/bundle.go`,
  `validateDurations`). The Flux generator additionally refuses a duration the Flux API does not
  take: a negative one, or a positive one under a millisecond (`pkg/stack/fluxcd/durations.go`).
  See `pkg/stack/README.md` and the Flux engine's "Durations".

**Examples:**

<!-- doc-example:excerpt a Bundle literal that Validate refuses, not a call -->
```go
bundle := &stack.Bundle{
    Name:          "web-app",
    Interval:      "10m",       // valid
    Timeout:       "15m",       // valid
    RetryInterval: "5 minutes", // refused by Validate: spaces are not allowed
}
```

**Error message:** the error names the bundle, the field and the value, then the parse error; the
message text is:

```
retryInterval "5 minutes" is not a valid duration
```

## Current Workflow Integration

### Flux Workflow (`pkg/stack/fluxcd/`)

The Flux workflow engine generates:
- **Bootstrap resources**: Flux system components
- **Source resources**: GitRepository, OCIRepository, etc.
- **Kustomization resources**: From Bundle configurations

### ArgoCD Workflow (`pkg/stack/argocd/`)

The ArgoCD workflow engine generates:
- **Application resources**: From Bundle configurations (`GenerateBootstrap` returns an error when bootstrap is enabled: ArgoCD bootstrap is not implemented)

### Layout System (`pkg/stack/layout/`)

The layout system handles manifest organization:
- **Directory structure**: Hierarchical file layout
- **Resource grouping**: By namespace, type, or bundle
- **Dependency ordering**: Ensuring proper apply sequence

## Construction

A stack is built by constructing the Go structs directly; there is no serialized, versioned stack
format:

<!-- doc-example: pkg/stack ExampleCluster -->
```go
cluster := &stack.Cluster{
    Name: "prod",
    Node: &stack.Node{Name: "apps"},
}
fmt.Println(cluster.Name, cluster.Node.Name)
```
<!-- doc-example:end -->

That block is `ExampleCluster` in `pkg/stack`, which `go test` runs; the other Go blocks in this
document are declarations or illustrations, marked as excerpts.

## Implementation Status

See [`STATUS.md`](STATUS.md) for what is implemented. `pkg/gvk` holds shared Group, Version, Kind
infrastructure that no other package in this module imports yet; a versioned stack struct format was
tried and removed because nothing consumed it.


## Related Documentation

- [`layout/README.md`](layout/README.md) - Manifest layout and organization
- [`fluxcd/README.md`](fluxcd/README.md) - Flux workflow integration
- [`argocd/README.md`](argocd/README.md) - ArgoCD workflow integration

---
