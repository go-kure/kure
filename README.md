# Kure

[![CI](https://github.com/go-kure/kure/actions/workflows/ci.yml/badge.svg)](https://github.com/go-kure/kure/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/go-kure/kure/branch/main/graph/badge.svg)](https://codecov.io/gh/go-kure/kure)
[![Go Report Card](https://img.shields.io/badge/go%20report-A+-brightgreen)](https://goreportcard.com/report/github.com/go-kure/kure)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure.svg)](https://pkg.go.dev/github.com/go-kure/kure)
[![License](https://img.shields.io/github/license/go-kure/kure)](LICENSE)
[![Release](https://img.shields.io/github/v/release/go-kure/kure)](https://github.com/go-kure/kure/releases/latest)

**Kure** is a Go library for programmatically building Kubernetes resources used by GitOps tools like Flux, cert-manager, MetalLB, and External Secrets. The library emphasizes strongly-typed object construction over templating engines, providing a clean, type-safe approach to Kubernetes manifest generation.

## Features

- Type-safe Kubernetes resource builders (Deployments, Services, RBAC, etc.)
- GitOps workflow support (Flux CD and ArgoCD)
- Hierarchical domain model (Cluster → Node → Bundle → Application)
- Declarative JSONPath-based patching
- Certificate management (cert-manager, ACME)
- Secret management (External Secrets, multiple cloud providers)
- Network configuration (MetalLB)
- Manifest layout engine with auto-generated kustomization.yaml
- Built-in configuration validation

## Installation

```bash
go get github.com/go-kure/kure
```

## CLI Tools

Kure ships two CLI tools: **kure** for manifest generation and patching, and **kurel** for package-based resource management. Run `kure --help` or `kurel --help` for usage details.

## Documentation

- [Website](https://www.gokure.dev) — guides, architecture, and tutorials
- [API Reference](https://pkg.go.dev/github.com/go-kure/kure) — full Go package documentation
- [`examples/`](examples/) — cluster configurations, kurel packages, and patching samples

## Development

- [DEVELOPMENT.md](DEVELOPMENT.md) — setup, build, test, and lint instructions
- [CHANGELOG.md](CHANGELOG.md) — release history

Quick commands:

```bash
make build   # Build all executables
make test    # Run tests
make lint    # Run linter
```

## License

This project is licensed under the [Apache License 2.0](LICENSE).
