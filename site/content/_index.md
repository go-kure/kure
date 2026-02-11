+++
title = "Go Kure"
type = "home"
+++

# Kure: Type-Safe Kubernetes Resource Generation

{{< notice warning >}}
Work in Progress: Kure is currently under active development and has not been released yet. APIs and features are subject to change.
{{< /notice >}}

**Kure** is a powerful Go library for programmatically building Kubernetes resources, designed specifically for GitOps workflows. Say goodbye to complex templating engines and hello to strongly-typed, composable resource generation.

## Why Kure?

Building Kubernetes manifests for GitOps can be challenging:
- **YAML templating** is error-prone and hard to maintain
- **Helm charts** add complexity with their templating language
- **Raw manifests** lead to duplication and inconsistency

Kure solves these problems by providing:
- **Type-safe builders** that catch errors at compile time
- **Composable patterns** for reusable resource generation
- **Native Go code** instead of template syntax
- **GitOps-ready output** for Flux and ArgoCD

## Features

- **Comprehensive Resource Support**
  - Core Kubernetes resources (Deployments, Services, ConfigMaps, etc.)
  - FluxCD resources (Kustomizations, HelmReleases, Sources)
  - cert-manager integration
  - External Secrets Operator
  - MetalLB configuration

- **Hierarchical Organization**
  - Cluster → Node → Bundle → Application structure
  - Logical grouping of related resources
  - Clean directory layout generation

- **Advanced Capabilities**
  - JSONPath-based patching system
  - Multi-environment support
  - OCI registry integration
  - Validation and error handling

## Quick Start

```go
import (
    "os"

    "github.com/go-kure/kure/pkg/kubernetes/fluxcd"
    "github.com/go-kure/kure/pkg/io"
    kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
)

// Create a Flux Kustomization
ks := fluxcd.Kustomization(&fluxcd.KustomizationConfig{
    Name:      "my-app",
    Namespace: "default",
    Path:      "./manifests",
    Interval:  "5m",
    SourceRef: kustv1.CrossNamespaceSourceReference{
        Kind: "GitRepository",
        Name: "my-repo",
    },
})

// Output as YAML
io.Marshal(os.Stdout, ks)
```

## Learn More

- [Overview](/overview) - Introduction and design philosophy
- [Getting Started](/getting-started) - Installation and quickstart guide
- [Architecture](/architecture) - Deep dive into Kure's design
- [Packages](/packages) - Core library packages
- [Examples](/examples) - See Kure in action

## Get Involved

Kure is open source and welcomes contributions!

- [GitHub Repository](https://github.com/go-kure/kure)
- [Issue Tracker](https://github.com/go-kure/kure/issues)
- [Discussions](https://github.com/go-kure/kure/discussions)
