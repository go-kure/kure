# Kure Compatibility Matrix

This document describes the versions of infrastructure tools that Kure supports.

## Version Philosophy

Kure maintains two version concepts for each dependency:

1. **Build Version** (`current` in versions.yaml): The exact library version Kure imports in go.mod
2. **Deployment Compatibility** (`supported_range`): The range of deployed tool versions that Kure can generate YAML for

## Go Version

**Current:** Go 1.26.1

## Infrastructure Dependencies

| Tool | Build Version | Deployment Compatibility | Notes |
|------|---------------|-------------------------|-------|
| cert-manager | 1.19.4 | 1.14 - 1.19 | Stable v1 APIs, backward compatible |
| fluxcd | 2.8.1 | 2.4 - 2.8 | v1beta2 APIs removed in 2.8, DependsOn uses DependencyReference.
image-automation-controller promoted to v1.
All github.com/fluxcd/* packages upgraded together. |
| flux-operator | 0.40.0 | 0.23 - 0.40 | Upgraded with FluxCD 2.8 ecosystem. |
| metallb | 0.15.3 | 0.14 - 0.15 | Stable v1beta1 APIs, patch release |
| external-secrets | 0.0.0-20260213133823-31b0c7c37342 | 1.3 | Module path changed from root to /apis submodule in v1.0 (#5494).
No semver tags for apis submodule — use pseudo-versions pinned to release commits.
v1.3.2+ commit: 31b0c7c3734255a92dfe5cf9e1e204de127eb24c (includes controller-runtime v0.23.1 compat) |
| cnpg | 1.28.1 | 1.24 - 1.28 | CloudNativePG operator for PostgreSQL on Kubernetes.
Cluster CR (with managed roles), Database CR (postgresql.cnpg.io/v1),
ObjectStore CR (barmancloud.cnpg.io/v1), and ScheduledBackups.
ObjectStore lives in a separate module (plugin-barman-cloud). |
| cnpg-barman-cloud | 0.11.0 | 0.9 - 0.11 | Barman Cloud plugin for CNPG — provides ObjectStore CR (barmancloud.cnpg.io/v1).
Versioned independently from the CNPG operator. |
| controller-runtime | 0.23.3 | 0.22 - 0.23 | Upgraded with FluxCD 2.8 and external-secrets 1.3 migrations |
| kubernetes | 0.35.1 | 1.33 - 1.35 | Go 1.26 baseline; generated YAML uses stable APIs compatible across this range |

## Known API Version Blockers

### Notification Controller: Provider and Alert on v1beta3

**Status:** Blocked upstream (as of 2026-03-07)
**Tracking:** [#250](https://github.com/go-kure/kure/issues/250)

The Flux notification-controller has three resource types:

| Resource | Current API Version | Target | Status |
|----------|-------------------|--------|--------|
| Receiver | v1 | v1 | Complete |
| Provider | v1beta3 | v1 | Blocked — not yet promoted upstream |
| Alert | v1beta3 | v1 | Blocked — not yet promoted upstream |

Kure ships v0.1.0-stable with Provider and Alert on `notification.toolkit.fluxcd.io/v1beta3`,
which is the highest API version available in the Flux notification-controller.
The v1beta2 scheme registrations have been removed as part of the FluxCD 2.8 upgrade.

When upstream Flux promotes Provider and Alert to v1, kure will migrate (#250) and
remove the v1beta3 scheme registration (#252).

## Understanding the Matrix

### Build Version (go.mod)
The version Kure imports and builds against. This is validated by CI to match `versions.yaml`.

### Deployment Compatibility
The range of versions that Kure can generate valid YAML for. Kure may generate YAML compatible with older or newer versions than it builds against.

For example:
- Kure builds against cert-manager 1.19.4
- But generates YAML compatible with cert-manager 1.14.x through 1.19.x

## Upgrading Dependencies

When upgrading a dependency:

1. Update `versions.yaml` with new `current` and `supported_range`
2. Run `go get <module>@<version>` to update go.mod
3. Update code for any API changes
4. Run `./scripts/sync-versions.sh generate` to update docs
5. Run `./scripts/sync-versions.sh check` to validate consistency

## Related Issues

- [#133](https://github.com/go-kure/kure/issues/133) - Go version upgrade tracking
- [#128](https://github.com/go-kure/kure/issues/128) - FluxCD ecosystem upgrade

