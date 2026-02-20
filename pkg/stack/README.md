# Stack - Core Domain Model

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/stack.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/stack)

The `stack` package defines the hierarchical domain model at the heart of Kure. It provides the Cluster, Node, Bundle, and Application abstractions used to describe a complete Kubernetes deployment topology.

## Overview

Kure models Kubernetes infrastructure as a four-level hierarchy:

```
Cluster
  └── Node (tree structure)
        └── Bundle (deployment unit)
              └── Application (workload)
```

Each level maps to a concept in GitOps deployment:

| Level | Purpose | GitOps Mapping |
|-------|---------|----------------|
| **Cluster** | Target cluster | Root directory |
| **Node** | Organizational grouping (e.g., `infrastructure`, `apps`) | Subdirectory tree |
| **Bundle** | Deployment unit with dependencies | Flux Kustomization / ArgoCD Application |
| **Application** | Individual workload or resource set | Kubernetes manifests |

## Key Types

### Cluster

The root of the hierarchy, representing a complete cluster configuration.

```go
cluster := stack.NewCluster("production", rootNode)
cluster.SetGitOps(&stack.GitOpsConfig{
    Type: "flux",
})
```

### Node

A tree structure for organizing bundles into logical groups. Nodes can have children (sub-nodes) and a package reference for multi-source deployments.

```go
node := &stack.Node{
    Name:     "infrastructure",
    Children: []*stack.Node{childNode},
    Bundle:   []*stack.Bundle{monitoringBundle},
}
```

### Bundle

A deployment unit corresponding to a single GitOps resource (e.g., a Flux Kustomization). Bundles support dependency ordering via `DependsOn`.

```go
bundle, err := stack.NewBundle("monitoring", apps, labels)
bundle.DependsOn = []string{"cert-manager"}
bundle.Interval = "10m"
```

### Application

An individual Kubernetes workload. Applications use the `ApplicationConfig` interface to generate their resources.

```go
app := stack.NewApplication("prometheus", "monitoring", prometheusConfig)
resources, err := app.Generate()
```

### ApplicationConfig Interface

Implement this interface to define how an application generates its Kubernetes resources:

```go
type ApplicationConfig interface {
    Generate(*Application) ([]*client.Object, error)
}
```

### Optional Validation

`ApplicationConfig` implementations can optionally implement the `Validator` interface to validate configuration before resource generation:

```go
type Validator interface {
    Validate() error
}
```

When present, `Application.Generate()` calls `Validate()` automatically before `Generate()`. If validation fails, generation stops and the error is returned with application context:

```go
type myConfig struct { Port int }

func (c *myConfig) Validate() error {
    if c.Port <= 0 {
        return errors.New("port must be positive")
    }
    return nil
}

func (c *myConfig) Generate(app *stack.Application) ([]*client.Object, error) {
    // Only called if Validate() passes (or is not implemented)
    ...
}
```

Validation errors are wrapped with application name and namespace:

```
validation failed for application "web" in namespace "prod": port must be positive
```

Configs that do not implement `Validator` continue to work without changes.

## Fluent Builder API

For ergonomic cluster construction, use the fluent builder:

```go
cluster := stack.NewClusterBuilder("production").
    WithNode("infrastructure").
        WithBundle("monitoring").
            WithApplication("prometheus", appConfig).
        End().
    End().
    Build()
```

## Workflow System

The package provides a pluggable workflow abstraction for GitOps tool integration:

```go
// Create a workflow for your GitOps tool
wf, err := stack.NewWorkflow("flux")

// Generate GitOps resources from the cluster definition
objects, err := wf.GenerateFromCluster(cluster)
```

Supported workflow providers: `"flux"` / `"fluxcd"` and `"argo"` / `"argocd"`.

## Source References

Bundles and nodes can reference different source types for multi-source deployments:

```go
node.SetPackageRef(&stack.SourceRef{
    Kind:      "OCIRepository",
    Name:      "my-registry",
    Namespace: "flux-system",
    URL:       "oci://registry.example.com/manifests",
    Tag:       "v1.0.0",
})
```

## Related Packages

- [stack/fluxcd](/api-reference/flux-engine/) - FluxCD workflow engine implementation
- [stack/generators](/api-reference/generators/) - Application generator system
- [stack/layout](/api-reference/layout/) - Manifest directory organization
