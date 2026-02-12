+++
title = "Go Kure"
type = "home"
+++

# Kure: Type-Safe Kubernetes Resource Generation

{{< notice warning >}}
Work in Progress: Kure is currently under active development (v0.1.0-alpha.2). APIs and features are subject to change.
{{< /notice >}}

**Kure** is a Go library for programmatically building Kubernetes resources, designed for GitOps workflows with FluxCD. Instead of complex templating engines, Kure provides strongly-typed, composable resource generation in native Go.

## Why Kure?

Building Kubernetes manifests for GitOps can be challenging:
- **YAML templating** is error-prone and hard to maintain
- **Helm charts** add complexity with their templating language
- **Raw manifests** lead to duplication and inconsistency

Kure solves these problems by providing:
- **Type-safe builders** that catch errors at compile time
- **Composable patterns** for reusable resource generation
- **Native Go code** instead of template syntax
- **GitOps-ready output** for Flux

## Quick Example

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

## Features

- **Comprehensive Resource Support**: Core Kubernetes, FluxCD, cert-manager, External Secrets, MetalLB
- **Hierarchical Organization**: Cluster, Node, Bundle, Application structure for clean GitOps layouts
- **Declarative Patching**: JSONPath-based patching system for resource customization
- **Kurel Package System**: Reusable application packages with patch-based customization

ArgoCD support is planned but not yet production-ready.

## Learn More

- [Getting Started](/getting-started) - Installation and quickstart guide
- [Concepts](/concepts) - Architecture and design philosophy
- [Guides](/guides) - How-to guides for common workflows
- [Examples](/examples) - See Kure in action
- [API Reference](/api-reference) - Package documentation

## Get Involved

Kure is open source and welcomes contributions!

- [GitHub Repository](https://github.com/go-kure/kure)
- [Issue Tracker](https://github.com/go-kure/kure/issues)
- [Discussions](https://github.com/go-kure/kure/discussions)
