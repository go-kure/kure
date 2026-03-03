# Getting Started: Cluster-to-Disk Pipeline

This example demonstrates the complete Kure pipeline that transforms a declarative
Cluster definition into on-disk Kubernetes manifests ready for GitOps reconciliation.

## Pipeline Steps

### 1. Build a Cluster (`ClusterBuilder`)

Use the fluent `ClusterBuilder` API to define your cluster hierarchy:

- **Cluster** — top-level container with a name and GitOps configuration
- **Node** — a directory-level grouping (e.g., `infrastructure`, `applications`)
- **Bundle** — a reconciliation unit within a node, holding applications and a source reference
- **Application** — a workload described by an `ApplicationConfig` implementation

The builder uses a copy-on-write pattern, so intermediate builder values can safely
be reused to create divergent cluster configurations.

### 2. Create a Workflow Engine (`FluxCD`)

Kure supports pluggable workflow engines. This example uses `fluxcd.NewWorkflowEngineWithConfig`
to create a FluxCD engine with explicit Kustomization mode and separate Flux resource placement.

### 3. Generate a Layout (`CreateLayoutWithResources`)

The workflow engine walks the cluster tree, calls each `ApplicationConfig.Generate()` to
produce Kubernetes objects, generates Flux Kustomization resources, and assembles everything
into a `ManifestLayout` tree.

### 4. Write to Disk (`WriteManifest`)

`layout.WriteManifest` serialises the layout tree as YAML files organised into directories
with `kustomization.yaml` files at each level, ready for Flux to reconcile.

## Running

```bash
go run ./examples/getting-started/
```

By default, output is written to a temporary directory. Set `OUT_DIR` to control the
output location:

```bash
OUT_DIR=./output go run ./examples/getting-started/
```

## Output Structure

```
clusters/
  staging/
    infrastructure/
      cache-deployment-redis.yaml
      web-deployment-web-app.yaml
      web-service-web-app.yaml
      kustomization.yaml
    flux-system/
      flux-system-kustomization-platform-services.yaml
      kustomization.yaml
```

## Implementing ApplicationConfig

Each application type implements the `stack.ApplicationConfig` interface:

```go
type ApplicationConfig interface {
    Generate(*Application) ([]*client.Object, error)
}
```

The `Generate` method returns a slice of Kubernetes objects (Deployments, Services,
ConfigMaps, etc.) that the layout writer serialises to YAML. See `RedisConfig` and
`WebAppConfig` in `main.go` for complete examples.
