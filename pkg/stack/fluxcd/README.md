# Flux Engine - FluxCD Workflow Implementation

The `fluxcd` package implements the `stack.Workflow` interface for FluxCD, providing complete Flux resource generation from domain model definitions.

## Overview

The Flux engine transforms Kure's hierarchical domain model (Cluster, Node, Bundle, Application) into FluxCD resources (Kustomizations, source references) organized in a GitOps-ready directory structure.

The engine is composed of three specialized components:

| Component | Responsibility |
|-----------|---------------|
| **ResourceGenerator** | Generates Flux resources from domain objects |
| **LayoutIntegrator** | Integrates resources into directory structures |
| **BootstrapGenerator** | Creates Flux bootstrap manifests |

## Quick Start

```go
import "github.com/go-kure/kure/pkg/stack/fluxcd"

// Create engine with defaults
engine := fluxcd.Engine()

// Generate all Flux resources for a cluster
objects, err := engine.GenerateFromCluster(cluster)

// Or with custom configuration
engine = fluxcd.EngineWithConfig(
    layout.KustomizationExplicit,
    layout.FluxSeparate,
)
```

## Engine Construction

```go
// Default engine
engine := fluxcd.Engine()

// Engine with specific kustomization mode
engine := fluxcd.EngineWithMode(layout.KustomizationExplicit)

// Engine with full configuration
engine := fluxcd.EngineWithConfig(mode, placement)

// Engine with custom components
engine := fluxcd.NewWorkflowEngine()
```

## Resource Generation

Generate Flux resources at different hierarchy levels:

```go
// From entire cluster
objects, err := engine.GenerateFromCluster(cluster)

// From a single node
objects, err := engine.GenerateFromNode(node)

// From a single bundle
objects, err := engine.GenerateFromBundle(bundle)
```

Each bundle produces a Flux Kustomization resource with:
- Path matching the layout directory structure
- Source reference from the node's package ref
- Dependency ordering from `Bundle.DependsOn`
- Interval and pruning configuration

## Layout Integration

Combine resource generation with directory structure:

```go
// Create layout with Flux resources integrated
ml, err := engine.CreateLayoutWithResources(cluster, rules)

// Write to disk
err = layout.WriteManifest(ml, "./clusters")
```

## Bootstrap Generation

Generate Flux system bootstrap manifests:

```go
bootstrapConfig := &stack.BootstrapConfig{
    Enabled:     true,
    FluxMode:    "install",
    FluxVersion: "v2.6.4",
    SourceRef:   sourceRef,
}

objects, err := engine.GenerateBootstrap(bootstrapConfig, rootNode)
```

## Configuration

### Kustomization Mode

Controls how kustomization.yaml files reference resources:

- `KustomizationExplicit` - Lists all manifest files explicitly
- `KustomizationRecursive` - References subdirectories only

### Flux Placement

Controls where Flux Kustomization resources are placed:

- `FluxSeparate` - Flux resources in a separate directory tree
- `FluxIntegrated` - Flux resources alongside application manifests

## Related Packages

- [stack](../) - Core domain model
- [stack/layout](../layout/) - Manifest directory organization
- [kubernetes/fluxcd](../../kubernetes/fluxcd/) - Low-level Flux resource builders
