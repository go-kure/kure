# Generators - Application Generator System

The `generators` package provides a type-safe system for creating Kubernetes application workloads from configuration. Generators implement the `stack.ApplicationConfig` interface, allowing them to be used as applications within the domain model.

## Overview

Generators use the GroupVersionKind (GVK) type system to identify and instantiate application configurations. Each generator is registered in the `stack` package's global registry and can be referenced by its GVK identifier.

## Available Generators

| Generator | GVK | Description |
|-----------|-----|-------------|
| **AppWorkload** | `generators/AppWorkload` | General-purpose application workload with Deployment, Service, ConfigMap |
| **FluxHelm** | `generators/FluxHelm` | HelmRelease-based application using Flux |
| **KurelPackage** | `generators/KurelPackage` | Kurel package reference for pre-built application packages |

## Usage

### Creating a Generator from GVK

```go
import "github.com/go-kure/kure/pkg/stack"

// Create application config by apiVersion and kind
config, err := stack.CreateApplicationConfig("generators.gokure.dev/v1alpha1", "AppWorkload")

// Use in domain model
app := stack.NewApplication("my-app", "default", config)
```

### YAML Configuration Format

Generators are typically configured via YAML:

```yaml
apiVersion: generators/v1
kind: AppWorkload
metadata:
  name: my-web-app
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

### Registry

Register custom generators:

```go
stack.RegisterApplicationConfig(gvk.GVK{
    Group:   "mycompany.dev",
    Version: "v1",
    Kind:    "CustomApp",
}, func() stack.ApplicationConfig {
    return &MyCustomConfig{}
})
```

Query available generators:

```go
// List all registered generator GVKs
gvks := stack.ListApplicationConfigGVKs()
```

## Sub-packages

### appworkload

General-purpose application generator that produces:
- Deployment with configurable replicas, image, resources
- Service with port mappings
- Optional ConfigMap for configuration data
- Optional ServiceAccount

### fluxhelm

Generates Flux HelmRelease resources for Helm-based applications:
- HelmRelease with chart reference
- HelmRepository source (if needed)
- Values configuration

### kurelpackage

Generates Kubernetes resource objects from kurel packages. The `Generate()` method delegates to `GeneratePackageFiles()`, extracts files under the `resources/` prefix, and parses each one into typed `client.Object` values. Non-resource files (kurel.yaml, patches, values, extensions) are excluded — they are package metadata. This makes KurelPackage configs usable in the stack generation pipeline alongside other generators.

## Related Packages

- [stack](../) - Core domain model and ApplicationConfig interface
- [stack/fluxcd](/api-reference/flux-engine/) - Flux workflow engine
