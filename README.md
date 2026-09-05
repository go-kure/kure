# Kure

[![CI](https://github.com/go-kure/kure/actions/workflows/ci.yml/badge.svg)](https://github.com/go-kure/kure/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/go-kure/kure/branch/main/graph/badge.svg)](https://codecov.io/gh/go-kure/kure)
[![Go Report Card](https://img.shields.io/badge/go%20report-A+-brightgreen)](https://goreportcard.com/report/github.com/go-kure/kure)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure.svg)](https://pkg.go.dev/github.com/go-kure/kure)
[![License](https://img.shields.io/github/license/go-kure/kure)](LICENSE)
[![Release](https://img.shields.io/github/v/release/go-kure/kure)](https://github.com/go-kure/kure/releases/latest)

**Kure** is a Go library for describing a GitOps repository and writing it out as plain Kubernetes
YAML. It is a domain model and a layout engine on a thin Kubernetes foundation:

- **Domain model** (`pkg/stack`) — a Cluster → Node → Bundle → Application hierarchy that says
  what a cluster contains, independent of the GitOps tool that reconciles it.
- **Layout engine** (`pkg/stack/layout`, `pkg/stack/fluxcd`) — turns that hierarchy into a
  directory tree of manifests, with the Flux resources and `kustomization.yaml` files that make it
  reconcile.
- **Kubernetes foundation** (`pkg/kubernetes`) — the shared scheme, one generic constructor, the
  generated per-kind wrappers over it, and a small, admissible set of sugar helpers. The upstream
  Go struct is the construction API; kure adds identity and gets out of the way.

The foundation is deliberately thin. Constructors emit `apiVersion`, `kind` and identity and
nothing else, so an object kure builds is one you can reason about field by field against the
upstream type. The rules, and what they mean for a caller, are the
[builder contract](pkg/kubernetes/README.md) — the normative page for everything under
`pkg/kubernetes/...`.

## Features

- GitOps domain model: Cluster → Node → Bundle → Application (`pkg/stack`)
- Layout engine that writes a reconcilable directory tree, `kustomization.yaml` files included
- Flux workflow engine — the supported GitOps path; ArgoCD support exists but its bootstrap is not
  production-ready
- Generic `Create[T]` plus a generated constructor for every registered kind — the count comes
  from the pinned upstream modules, so the [kinds table](docs/api-tables.md) states it and this
  page does not
- CRD builders for cert-manager, Cilium, CloudNativePG, External Secrets, MetalLB, Prometheus
  Operator and VolSync
- Generated kinds, scope and field-maturity tables derived from the upstream modules this build
  pins, so "what does kure support" is answered from the pins rather than from prose

## Installation

```bash
go get github.com/go-kure/kure
```

## Documentation

- [Website](https://www.gokure.dev) — guides, architecture, and tutorials
- [Builder contract](pkg/kubernetes/README.md) — normative: construction, sugar admission, purity,
  and where scope and maturity come from
- [Supported kinds and field maturity](docs/api-tables.md) — generated from the pinned modules
- [Release-1 migration notes](docs/builder-contract-release-1.md) — every helper the builder
  contract removed, with the expression that replaces it
- [API Reference](https://pkg.go.dev/github.com/go-kure/kure) — full Go package documentation
- [`examples/`](examples/) — cluster configurations and patching samples

## Development

- [DEVELOPMENT.md](DEVELOPMENT.md) — setup, build, test, and lint instructions
- [CHANGELOG.md](CHANGELOG.md) — release history

Quick commands:

```bash
make test    # Run tests
make lint    # Run linter
```

## License

This project is licensed under the [Apache License 2.0](LICENSE).
