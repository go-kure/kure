+++
title = "Working with Generators"
weight = 30
+++

# Working with Generators

Generators provide a type-safe way to create application workloads from configuration. They implement the `ApplicationConfig` interface and are identified by GroupVersionKind (GVK) strings.

## Getting Started

The fastest way to start a new project is with `kure init`:

```bash
kure init my-cluster
```

This creates a ready-to-use directory structure with `cluster.yaml` and an
example application under `apps/`. You can then generate manifests with:

```bash
kure generate cluster cluster.yaml
```

See `kure init --help` for options like `--gitops argocd`.

## The GVK System

Each generator is registered with a GVK identifier that uniquely identifies its type:

| GVK | Generator | Output |
|-----|-----------|--------|
| `generators/AppWorkload` | AppWorkload | Deployment, Service, ConfigMap |
| `generators/FluxHelm` | FluxHelm | HelmRelease, HelmRepository |
| `generators/KurelPackage` | KurelPackage | Kubernetes resources from kurel packages |

## Using Generators

### From Code

```go
import "github.com/go-kure/kure/pkg/stack/generators"

// Look up generator by GVK
factory, err := generators.GetGenerator("generators/AppWorkload")

// Create config from YAML data
config, err := factory.FromConfig(yamlData)

// Use in the domain model
app := stack.NewApplication("my-app", "default", config)
```

### From YAML Configuration

Generators can be configured via YAML files:

```yaml
apiVersion: generators/v1
kind: AppWorkload
metadata:
  name: web-frontend
spec:
  image: nginx:1.25
  replicas: 3
  ports:
    - containerPort: 80
      servicePort: 80
  env:
    - name: LOG_LEVEL
      value: info
```

### Listing Available Generators

```go
// List all registered generator GVKs
gvks := generators.ListRegistered()
```

## Built-in Generators

### AppWorkload

Generates a complete application deployment:
- Deployment with configurable replicas, image, resource limits
- Service with port mappings
- Optional ConfigMap for configuration data
- Optional ServiceAccount

### FluxHelm

Generates Flux HelmRelease resources:
- HelmRelease with chart reference and values
- HelmRepository source (when needed)

### KurelPackage

Generates Kubernetes resource objects from kurel packages. `Generate()` delegates to `GeneratePackageFiles()`, extracts the resource files from the package, and parses them into typed objects. Non-resource metadata (kurel.yaml, patches, values, extensions) is excluded from the output. This allows kurel packages to participate in the stack generation pipeline alongside code-generated resources.

## Custom Generators

Register your own generators:

```go
generators.Register("mycompany/CustomApp", &MyGeneratorFactory{})
```

The factory must implement the generator interface, providing a `FromConfig` method that returns an `ApplicationConfig`.

## Further Reading

- [Generators reference](/api-reference/generators) for API details
- [Generator examples](/examples/generators) for working samples
- [Flux workflow](/guides/flux-workflow/) for using generators in clusters
