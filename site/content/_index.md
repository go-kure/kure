+++
title = "Kure"
type = "home"
+++

**A GitOps domain model and layout engine, on a thin Kubernetes foundation.**

Kure helps platform teams describe a cluster in Go and write it out as plain Kubernetes YAML for GitOps workflows — without Helm templates, runtime controllers, or fragile string-based YAML manipulation.

{{< button link="/getting-started/quickstart/" >}}Get Started{{< /button >}} {{< button link="https://github.com/go-kure/kure" >}}View on GitHub{{< /button >}}

{{< notice style="info" >}}
Development docs: Kure has not reached a stable release yet. These docs track the current development version, so APIs and examples may change.
{{< /notice >}}

## Why Kure?

Building Kubernetes manifests for GitOps can be challenging:

- **YAML templating** is error-prone and hard to maintain at scale
- **Helm charts** add complexity with their templating language and release lifecycle
- **Raw manifests** lead to duplication and inconsistency across clusters

Kure answers this in three layers. A **domain model** says what a cluster contains. A **layout engine** turns that into a reconcilable directory tree. Underneath both, a deliberately **thin Kubernetes foundation** hands you the upstream Go struct with its identity filled in and then gets out of the way — so what you write is checked by the compiler against the real API types, not against kure's idea of them.

## Quick Example

Go code generates plain Kubernetes YAML — no templates, no runtime.

{{< tabs >}}
{{< tab title="Go" >}}
```go
import (
    "os"
    "github.com/go-kure/kure/pkg/io"
    "github.com/go-kure/kure/pkg/kubernetes"
)

cm := kubernetes.CreateConfigMap("app-config", "default")
kubernetes.AddConfigMapData(cm, "DATABASE_HOST", "postgres.db.svc")
kubernetes.AddConfigMapData(cm, "DATABASE_PORT", "5432")

io.Marshal(os.Stdout, cm)
```
{{< /tab >}}
{{< tab title="YAML Output" >}}
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: default
data:
  DATABASE_HOST: postgres.db.svc
  DATABASE_PORT: "5432"
```
{{< /tab >}}
{{< /tabs >}}

## How It Fits GitOps

```
Go program  →  Kure builders  →  YAML files  →  Git repository  →  FluxCD reconciles
```

Kure is a code-time tool. It runs during your build step, outputs plain YAML, and exits. It has no runtime presence in your cluster.

## Core Model

Kure organises cluster configuration into a four-level hierarchy:

```
Cluster  →  Node  →  Bundle  →  Application
```

| Level | Purpose |
|-------|---------|
| **Cluster** | Root; holds GitOps engine config (Flux or ArgoCD) |
| **Node** | Subdirectory tree; logical grouping of bundles |
| **Bundle** | Deployment unit; maps to a Flux Kustomization or ArgoCD Application |
| **Application** | Individual workload; generates Kubernetes manifests |

See [Domain Model](/concepts/domain-model) for the full reference.

## What Kure Is and Is Not

**Kure is:**
- A domain model and layout engine for GitOps repositories, on a thin Kubernetes foundation
- A compile-time tool — it runs in your build step, not in your cluster
- FluxCD-first for GitOps workflow integration
- Designed to output plain, readable YAML

**Kure is not:**
- A runtime controller or operator
- A wrapper around the Kubernetes API types — the upstream struct is the construction API, and kure adds identity, not a parallel vocabulary
- A package manager by itself
- A replacement for every Helm chart use case
- Required to run inside your target cluster

## Related Projects

Kure is the manifest-generation library. Related tooling:

- **[go-kure/launcher](https://github.com/go-kure/launcher)**: Package manager and customization workflows (patching, overlays). Optional — only needed if you use those workflows. References to "kurel" in older docs refer to this project's predecessor.

## Current Status

- **FluxCD** workflow is the primary supported path — fully implemented
- **ArgoCD** support is present but bootstrap is not yet production-ready
- No stable release yet; APIs may change between versions

## Start Here

Recommended reading order:

1. [Quickstart](/getting-started/quickstart/) — generate your first Kubernetes manifest
2. [Domain Model](/concepts/domain-model) — understand the Cluster → Node → Bundle → Application hierarchy
3. [Builder Contract](/concepts/builder-contract/) — what a constructor sets, when a helper exists, how to read the maturity table
4. [Using Kure as a Library](/guides/library-usage/) — import paths, creating resources, generating YAML
5. [Generating Flux Manifests](/guides/flux-workflow/) — end-to-end GitOps layout generation
6. [Examples](/examples/) — practical, runnable examples
7. [API Reference](/api-reference/) — package documentation

## Get Involved

Kure is open source.

- [GitHub Repository](https://github.com/go-kure/kure)
- [Issue Tracker](https://github.com/go-kure/kure/issues)
- [Discussions](https://github.com/go-kure/kure/discussions)
