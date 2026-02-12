+++
title = "Domain Model"
weight = 20
+++

# Domain Model

Kure models Kubernetes infrastructure as a four-level hierarchy. Each level maps to a concept in GitOps deployment workflows.

## The Hierarchy

```
Cluster
  └── Node (tree structure)
        └── Bundle (deployment unit)
              └── Application (workload)
```

### Cluster

The root of the hierarchy, representing a target Kubernetes cluster. A cluster has a name, a tree of nodes, and GitOps configuration specifying which workflow engine to use (Flux or ArgoCD).

### Node

An organizational grouping within a cluster. Nodes form a tree structure — for example, a cluster might have top-level nodes for `infrastructure` and `applications`, each with child nodes for specific concerns.

Nodes map to **directory structures** in the GitOps repository. Each node can also reference a source (Git repository, OCI registry, S3 bucket) for multi-source deployments.

### Bundle

A deployment unit corresponding to a single GitOps reconciliation resource (e.g., a Flux Kustomization or ArgoCD Application). Bundles contain applications and support:

- **Dependency ordering** via `DependsOn` (e.g., "deploy cert-manager before my app")
- **Reconciliation settings**: interval, pruning, timeouts
- **Labels and annotations** for metadata

### Application

An individual Kubernetes workload or resource set. Applications implement the `ApplicationConfig` interface, which defines how to generate Kubernetes resource objects.

## Fluent Builder API

For ergonomic cluster construction:

```go
cluster := stack.NewClusterBuilder("production").
    WithNode("infrastructure").
        WithBundle("cert-manager").
            WithApplication("cert-manager", certManagerConfig).
        End().
        WithBundle("monitoring").
            WithApplication("prometheus", prometheusConfig).
        End().
    End().
    WithNode("applications").
        WithBundle("web-apps").
            WithApplication("frontend", frontendConfig).
            WithApplication("api", apiConfig).
        End().
    End().
    Build()
```

## How It Maps to GitOps

The domain model maps directly to a GitOps repository structure:

```
clusters/
  production/                    # Cluster
    infrastructure/              # Node
      cert-manager/              # Bundle → Flux Kustomization
        cert-manager/            # Application → K8s manifests
      monitoring/                # Bundle → Flux Kustomization
        prometheus/              # Application → K8s manifests
    applications/                # Node
      web-apps/                  # Bundle → Flux Kustomization
        frontend/                # Application → K8s manifests
        api/                     # Application → K8s manifests
```

The [Layout Engine](/api-reference/layout) handles this mapping, and the [Flux Engine](/api-reference/flux-engine) generates the corresponding Flux Kustomization resources.

## Further Reading

- [Stack package reference](/api-reference/stack) for API details
- [Flux workflow guide](/guides/flux-workflow) for end-to-end usage
- [Architecture](architecture) for system-level design
