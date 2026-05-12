# Kure Compatibility Matrix

This document describes the versions of infrastructure tools that Kure supports.

## Version Philosophy

Kure maintains two version concepts for each dependency:

1. **Build Version** (`current` in versions.yaml): The exact library version Kure imports in go.mod
2. **Deployment Compatibility** (`supported_range`): The range of deployed tool versions that Kure can generate YAML for

## Go Version

**Current:** Go 1.26.3

## Infrastructure Dependencies

| Tool | Build Version | Deployment Compatibility | Notes |
|------|---------------|-------------------------|-------|
| cert-manager | 1.20.2 | 1.14 - 1.20 | Stable v1 APIs. v1.20 deprecated ObjectReference in favor of IssuerReference (type alias). |
| fluxcd | 2.8.6 | 2.4 - 2.8 | v1beta2 APIs removed in 2.8, DependsOn uses DependencyReference.
image-automation-controller promoted to v1.
All github.com/fluxcd/* packages upgraded together. |
| flux-operator | 0.40.0 | 0.23 - 0.40 | Upgraded with FluxCD 2.8 ecosystem. |
| metallb | 0.15.3 | 0.14 - 0.15 | Stable v1beta1 APIs, patch release |
| prometheus-operator | 0.91.0 | 0.75 - 0.91 | Prometheus operator monitoring API types (ServiceMonitor, PodMonitor, PrometheusRule).
Only the /pkg/apis/monitoring submodule is imported — not the full operator.
Stable v1 APIs (monitoring.coreos.com/v1). |
| external-secrets | 0.0.0-20260213133823-31b0c7c37342 | 1.3 | Module path changed from root to /apis submodule in v1.0 (#5494).
No semver tags for apis submodule — use pseudo-versions pinned to release commits.
v1.3.2+ commit: 31b0c7c3734255a92dfe5cf9e1e204de127eb24c (includes controller-runtime v0.23.1 compat) |
| cnpg | 1.29.1 | 1.24 - 1.29 | CloudNativePG operator for PostgreSQL on Kubernetes.
Cluster CR (with managed roles), Database CR (postgresql.cnpg.io/v1),
ObjectStore CR (barmancloud.cnpg.io/v1), and ScheduledBackups.
ObjectStore lives in a separate module (plugin-barman-cloud). |
| cnpg-barman-cloud | 0.12.0 | 0.9 - 0.12 | Barman Cloud plugin for CNPG — provides ObjectStore CR (barmancloud.cnpg.io/v1).
Versioned independently from the CNPG operator. |
| controller-runtime | 0.24.0 | 0.22 - 0.24 | v0.24.0 requires k8s.io/* v0.36.0 (Kubernetes 1.36) |
| gateway-api | 1.5.1 | 1.0 - 1.5 | Gateway API v1 types (HTTPRoute). Used by pkg/kubernetes HTTPRoute builders.
Kure generates gateway.networking.k8s.io/v1 resources (GA since v1.0). |
| kubernetes | 0.36.0 | 1.33 - 1.36 | Go 1.26 baseline; generated YAML uses stable APIs compatible across this range |

## Understanding the Matrix

### Build Version (go.mod)
The version Kure imports and builds against. This is validated by CI to match `versions.yaml`.

### Deployment Compatibility
The range of versions that Kure can generate valid YAML for. Kure may generate YAML compatible with older or newer versions than it builds against.

For example:
- Kure builds against cert-manager 1.16.2
- But generates YAML compatible with cert-manager 1.14.x, 1.15.x, and 1.16.x

## Upgrading Dependencies

When upgrading a dependency:

1. Update `versions.yaml` with new `current` and `supported_range`
2. Run `go get <module>@<version>` to update go.mod
3. Update code for any API changes
4. Run `./scripts/sync-versions.sh generate` to update docs
5. Run `./scripts/sync-versions.sh check` to validate consistency

## Related Issues

- [#133](https://github.com/go-kure/kure/issues/133) - Go 1.25 upgrade tracking
- [#128](https://github.com/go-kure/kure/issues/128) - FluxCD ecosystem upgrade (blocked by Go 1.25)

