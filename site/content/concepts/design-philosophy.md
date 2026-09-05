+++
title = "Design Philosophy"
weight = 30
+++

# Design Philosophy

Kure is built on a few core principles that guide its design and API choices.

## The Upstream Type Over Templating

Traditional Kubernetes tooling relies on string-based templating (Helm, Kustomize overlays, Jsonnet). This creates a class of errors that only surface at deploy time — typos in YAML paths, type mismatches, missing fields.

Kure uses Go's type system instead. The type it uses is the upstream one: a Flux `Kustomization` is `kustomizev1.Kustomization`, not a kure struct that resembles it.

```go
// Compile-time checked against the real API type — typos and type errors are
// caught by the compiler, and the field names are the ones in the CRD.
ks := fluxcd.CreateKustomization("my-app", "flux-system")
ks.Spec.Path = "./clusters/production/apps"
ks.Spec.Interval = metav1.Duration{Duration: 10 * time.Minute}
ks.Spec.Prune = true
```

If you misspell a field name, the Go compiler tells you immediately. And because the struct is upstream's, a field added in the next controller release is reachable the moment the module pin moves — no kure release in between, no forwarder to write. That is the whole argument for keeping the foundation thin, and it is written down as the [builder contract](/concepts/builder-contract/).

## GitOps-Native Output

Kure generates plain Kubernetes YAML manifests organized for GitOps tools. The output is not a runtime artifact — it's files in a directory structure that Flux (or eventually ArgoCD) reconciles.

This means:
- **Predictable output** — same inputs always produce the same manifests
- **Tool independence** — the output is standard Kubernetes YAML
- **Debugging simplicity** — you can read the generated manifests directly
- **Git-friendly** — changes are visible as diffs

## Interface-Driven Design

Kure separates concerns through interfaces:

- **`ApplicationConfig`** — how an application generates its resources
- **`Workflow`** — how a GitOps tool creates reconciliation resources

This allows new application types and GitOps tools to be added without modifying the core domain model.

## Historical: "Kurel Just Generates YAML"

> **Note (2026-05-15)**: This section describes the historical kurel prototype. The current kurel is
> an OAM-native package manager in [go-kure/launcher](https://github.com/go-kure/launcher) — it uses
> OAM Application/Component/Trait semantics rather than the patch-and-variables model below.

The original kurel package system followed a simple principle: it takes base manifests, applies patches, resolves variables, and writes YAML files. It's not a runtime system, not a controller, not an orchestrator.

This constraint keeps the system simple and auditable. You can always inspect exactly what will be deployed by looking at the generated output.

## Composition Over Configuration

Rather than a single monolithic configuration format, Kure composes small, focused packages:

| Package | Responsibility |
|---------|---------------|
| `stack` | Domain model (what to deploy) |
| `stack/fluxcd` | Flux resource generation (how to deploy) |
| `stack/layout` | Directory organization (where to write) |
| `kubernetes` | Kubernetes foundation (how to build one object) |
| [`launcher/pkg/patch`](https://github.com/go-kure/launcher) | Resource modification (how to customize) |
| `io` | Serialization (how to read/write) |

Each package can be used independently or composed together for complete workflows.
