+++
title = "API Reference"
weight = 60
+++

# API Reference

Kure's public API is organized into focused packages. Each package README below is auto-synced from the source code.

For full Go API documentation, see [pkg.go.dev/github.com/go-kure/kure](https://pkg.go.dev/github.com/go-kure/kure).

## Core Domain

| Package | Description | Reference |
|---------|-------------|-----------|
| [Stack](stack) | Cluster, Node, Bundle, Application domain model | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/stack) |
| [Flux Engine](flux-engine) | FluxCD workflow implementation | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/stack/fluxcd) |
| [Generators](generators) | Application generator system (GVK) | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/stack/generators) |
| [Layout Engine](layout) | Manifest directory organization | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/stack/layout) |

## Resource Operations

| Package | Description | Reference |
|---------|-------------|-----------|
| [IO](io) | YAML/JSON serialization and resource printing | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/io) |
| [Kubernetes Builders](kubernetes-builders) | Core K8s resource constructors (GVK, HPA, PDB) | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes) |
| [FluxCD Builders](fluxcd-builders) | Low-level Flux resource constructors | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/fluxcd) |
| [Prometheus Builders](prometheus-builders) | Prometheus Operator CRD constructors (ServiceMonitor, PodMonitor, PrometheusRule) | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/prometheus) |

## Utilities

| Package | Description | Reference |
|---------|-------------|-----------|
| [Errors](errors) | Structured error types | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/errors) |
| [Logger](logger) | Structured logging | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/logger) |

## Compatibility

- [Compatibility Matrix](compatibility) - Supported Kubernetes and dependency versions

## ArgoCD

ArgoCD support exists at `pkg/stack/argocd/` but is not yet production-ready. It is not featured in guides or examples. The Flux workflow is the primary supported GitOps integration.
