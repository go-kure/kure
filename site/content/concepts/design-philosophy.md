+++
title = "Design Philosophy"
weight = 30
+++

# Design Philosophy

Kure is built on a few core principles that guide its design and API choices.

## Type-Safe Builders Over Templating

Traditional Kubernetes tooling relies on string-based templating (Helm, Kustomize overlays, Jsonnet). This creates a class of errors that only surface at deploy time — typos in YAML paths, type mismatches, missing fields.

Kure uses Go's type system instead:

```go
// Compile-time checked — typos and type errors are caught by the compiler
ks := fluxcd.Kustomization(&fluxcd.KustomizationConfig{
    Name:      "my-app",
    Namespace: "flux-system",
    Path:      "./clusters/production/apps",
    Interval:  "10m",
    Prune:     true,
})
```

If you misspell a field name, the Go compiler tells you immediately.

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
- **Generator registry** — pluggable application types via GVK

This allows new application types and GitOps tools to be added without modifying the core domain model.

## "Kurel Just Generates YAML"

The kurel package system follows a simple principle: it takes base manifests, applies patches, resolves variables, and writes YAML files. It's not a runtime system, not a controller, not an orchestrator.

This constraint keeps the system simple and auditable. You can always inspect exactly what will be deployed by looking at the generated output.

## Composition Over Configuration

Rather than a single monolithic configuration format, Kure composes small, focused packages:

| Package | Responsibility |
|---------|---------------|
| `stack` | Domain model (what to deploy) |
| `stack/fluxcd` | Flux resource generation (how to deploy) |
| `stack/layout` | Directory organization (where to write) |
| `patch` | Resource modification (how to customize) |
| `io` | Serialization (how to read/write) |

Each package can be used independently or composed together for complete workflows.
