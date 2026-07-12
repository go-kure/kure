+++
title = "API Reference"
weight = 60
+++

# API Reference

Kure's public API is organized into focused packages. Each package README below is auto-synced from the source code.

For full Go API documentation, see [pkg.go.dev/github.com/go-kure/kure](https://pkg.go.dev/github.com/go-kure/kure).

<!-- The tables below are generated from site/docs-map.yaml. Do not edit by hand;
     run: bash site/scripts/gen-docs-tables.sh -->
<!-- BEGIN GENERATED: api-reference-nav (source: site/docs-map.yaml) -->
## Core Domain

| Package | Description | Reference |
|---------|-------------|-----------|
| [Stack](/api-reference/stack/) | Cluster, Node, Bundle, Application domain model | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/stack) |
| [Flux Engine](/api-reference/flux-engine/) | FluxCD workflow implementation | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/stack/fluxcd) |
| [Layout Engine](/api-reference/layout/) | Manifest directory organization | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/stack/layout) |

## Resource Operations

| Package | Description | Reference |
|---------|-------------|-----------|
| [IO](/api-reference/io/) | YAML/JSON serialization and resource printing | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/io) |
| [Manifest Classification](/api-reference/manifest/) | CRD recognition and object scope classification | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/manifest) |
| [Kubernetes Builders](/api-reference/kubernetes-builders/) | Core K8s resource constructors (GVK, HPA, PDB) | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes) |
| [Cert-Manager Builders](/api-reference/certmanager-builders/) | cert-manager CRD constructors (Certificate, Issuer, ClusterIssuer) | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/certmanager) |
| [Cilium Builders](/api-reference/cilium-builders/) | Cilium CRD constructors (network policies and related resources) | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/cilium) |
| [CloudNativePG Builders](/api-reference/cnpg-builders/) | CloudNativePG and Barman Cloud CRD constructors | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/cnpg) |
| [External Secrets Builders](/api-reference/externalsecrets-builders/) | External Secrets Operator constructors (ExternalSecret, SecretStore) | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/externalsecrets) |
| [FluxCD Builders](/api-reference/fluxcd-builders/) | Low-level Flux resource constructors | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/fluxcd) |
| [MetalLB Builders](/api-reference/metallb-builders/) | MetalLB constructors (IPAddressPool, L2Advertisement) | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/metallb) |
| [Prometheus Builders](/api-reference/prometheus-builders/) | Prometheus Operator CRD constructors (ServiceMonitor, PodMonitor, PrometheusRule) | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/prometheus) |
| [VolSync Builders](/api-reference/volsync-builders/) | VolSync backup/restore CRD constructors | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/volsync) |

## Utilities

| Package | Description | Reference |
|---------|-------------|-----------|
| [Errors](/api-reference/errors/) | Structured error types | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/errors) |
| [Logger](/api-reference/logger/) | Structured logging | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/pkg/logger) |
<!-- END GENERATED: api-reference-nav -->

## Compatibility

- [Compatibility Matrix](/api-reference/compatibility/) - Supported Kubernetes and dependency versions

## ArgoCD

ArgoCD support exists at `pkg/stack/argocd/` but is not yet production-ready. It is not featured in guides or examples. The Flux workflow is the primary supported GitOps integration.
