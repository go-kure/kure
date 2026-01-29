# Kure Compatibility Matrix

This document describes the versions of infrastructure tools that Kure supports.

## Version Philosophy

Kure maintains two version concepts for each dependency:

1. **Build Version** (`current` in versions.yaml): The exact library version Kure imports in go.mod
2. **Deployment Compatibility** (`supported_range`): The range of deployed tool versions that Kure can generate YAML for

## Go Version

**Current:** Go 1.24.12

## Infrastructure Dependencies

| Tool | Build Version | Deployment Compatibility | Notes |
|------|---------------|-------------------------|-------|
| cert-manager | 1.16.5 | 1.14 - 1.16 | 1.17+ requires Go 1.25 |
| fluxcd | 2.6.4 | 2.4 - 2.6 | 2.7+ requires Go 1.25, tracked in #128 |
| flux-operator | 0.24.1 | 0.23 - 0.24 | 0.25+ requires Go 1.25 |
| metallb | 0.15.2 | 0.14 - 0.15 | Version pinned to match deployed infrastructure |
| external-secrets | 0.19.2 | 0.18 - 0.19 | Compatible with current Go version |
| controller-runtime | 0.21.0 | 0.19 - 0.21 | 0.22+ requires Go 1.25 |
| kubernetes | 0.33.2 | 1.28 - 1.33 | Tested in CI matrix |

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

