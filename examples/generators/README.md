# Generator Examples

This directory contains examples of the GVK-based generator system for Kure.

## Overview

Kure uses a Group, Version, Kind (GVK) pattern similar to Kubernetes for identifying generator types. Each generator:
- Has a unique GVK identifier
- Implements the ApplicationConfig interface
- Can generate specific types of Kubernetes resources

## Available Generators

### AppWorkload (generators.gokure.dev/v1alpha1)

Creates standard Kubernetes workloads (Deployments, StatefulSets, DaemonSets) with associated resources.

**Example:** [`appworkload.yaml`](appworkload.yaml)

This example creates:
- A Deployment with 3 replicas
- A LoadBalancer Service
- An Ingress resource
- Proper resource limits and volume mounts

### FluxHelm (generators.gokure.dev/v1alpha1)

Creates Flux HelmRelease resources with their source configurations.

**Examples:**
- [`fluxhelm.yaml`](fluxhelm.yaml) - Traditional Helm repository source
- [`fluxhelm-oci.yaml`](fluxhelm-oci.yaml) - OCI registry source

These examples demonstrate:
- HelmRepository and OCIRepository sources
- Values customization
- Release configuration options
- Flux-specific settings (interval, timeout, suspend)

### KurelPackage (generators.gokure.dev/v1alpha1)

Generates Kubernetes resource objects from kurel packages. `Generate()` collects resources from the package structure and returns them as typed objects, making kurel packages usable in the stack generation pipeline.

## Usage

To parse and generate resources from these examples:

```go
package main

import (
    "fmt"
    "io/ioutil"
    "gopkg.in/yaml.v3"
    
    "github.com/go-kure/kure/pkg/stack"
    _ "github.com/go-kure/kure/pkg/stack/generators/appworkload"
    _ "github.com/go-kure/kure/pkg/stack/generators/fluxhelm"
    _ "github.com/go-kure/kure/pkg/stack/generators/kurelpackage"
)

func main() {
    // Read YAML file
    data, err := ioutil.ReadFile("appworkload.yaml")
    if err != nil {
        panic(err)
    }
    
    // Parse into ApplicationWrapper
    var wrapper stack.ApplicationWrapper
    if err := yaml.Unmarshal(data, &wrapper); err != nil {
        panic(err)
    }
    
    // Convert to Application
    app := wrapper.ToApplication()
    
    // Generate Kubernetes resources
    resources, err := app.Config.Generate(app)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Generated %d resources\n", len(resources))
}
```

## Creating New Generators

To create a new generator type:

1. Create a package under `pkg/stack/generators/<type>/`
2. Implement the ApplicationConfig interface
3. Register with GVK in init()
4. Add version files (v1alpha1.go, etc.)

Example structure:
```
generators/
└── mytype/
    ├── v1alpha1.go      # Version implementation
    ├── internal/        # Internal logic
    │   └── mytype.go
    └── doc.go          # Documentation
```

## GVK Convention

All generators follow the pattern:
- **Group:** `generators.gokure.dev`
- **Version:** `v1alpha1`, `v1beta1`, `v1`
- **Kind:** Generator type name (e.g., `AppWorkload`, `FluxHelm`)

This allows for:
- Clear type identification
- Version evolution
- Backward compatibility
- Schema validation