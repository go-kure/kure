# Dependency Updates Guide

This guide covers the process for updating Kure's dependencies, including version tracking, risk assessment, and coordinated upgrades.

## Version Management Overview

Kure tracks dependency versions in three places:

| File | Purpose |
|------|---------|
| `go.mod` | Go module dependencies (authoritative for build) |
| `versions.yaml` | Version metadata: current version, supported range, constraints |
| `docs/compatibility.md` | Generated from `versions.yaml` — never edit directly |

The `sync-versions.sh` script validates consistency between `go.mod` and `versions.yaml`, and regenerates `docs/compatibility.md`.

## Update Risk Levels

### Patch Updates (Low Risk)

Patch bumps (e.g., v1.5.0 → v1.5.1) contain bug fixes only.

```bash
go get <module>@v<new-version>
go mod tidy
```

Update `versions.yaml` `current` field. No `supported_range` change needed.

### Minor Updates (Medium Risk)

Minor bumps (e.g., v1.19 → v1.20) may add new APIs or deprecate existing ones.

1. Review the upstream changelog for breaking changes
2. Check if Kure uses any deprecated APIs
3. Update `go.mod`, `versions.yaml` (both `current` and `supported_range`)
4. Run `make verify` to catch compile-time breakage

### Major Updates (High Risk)

Major bumps (e.g., v1 → v2) likely have breaking API changes.

1. Review the migration guide thoroughly
2. Assess impact on all callers (check with `grep -r` for imports)
3. Update code to use new APIs
4. Update `versions.yaml` and documentation
5. Consider impact on Crane (see `AGENTS.md` § Crane Integration)

## Coordinated Upgrade Rules

Some dependencies must be upgraded together to avoid version conflicts.

### Flux Ecosystem

All `github.com/fluxcd/*` packages must be upgraded together. Flux releases coordinate versions across:
- `flux2/v2`
- `helm-controller/api`
- `kustomize-controller/api`
- `notification-controller/api`
- `source-controller/api`
- `image-automation-controller/api`
- `pkg/apis/meta`, `pkg/apis/kustomize`

### Kubernetes (`k8s.io/*`)

All `k8s.io/` packages must stay at the same patch release. Kure uses `replace` directives in `go.mod` to enforce this. See the comment block in `go.mod` for details.

**When can replace directives be removed?** Only when ALL direct and transitive dependencies converge on the same `k8s.io/` minor version. Check with:

```bash
go mod graph | grep 'k8s.io/' | awk '{print $2}' | sort -u
```

### CNPG Ecosystem

`cloudnative-pg`, `barman-cloud`, `machinery`, and `plugin-barman-cloud` are related but versioned independently. Check compatibility notes in `versions.yaml` before upgrading.

## Bundling Dependabot PRs

When multiple Dependabot PRs accumulate, bundle them into a single PR:

1. Create a feature branch: `git checkout -b chore/bundle-dependency-updates main`
2. Run `go get` for all dependencies (Flux packages first for coordinated upgrades)
3. Run `go mod tidy`
4. Update `versions.yaml` entries
5. Regenerate docs: `./scripts/sync-versions.sh generate`
6. Validate: `./scripts/sync-versions.sh check`
7. Run full verification: `make verify && make test-race`
8. Commit, push, and create PR
9. Reference all Dependabot PR numbers in the PR body to auto-close them

## Dangerous Upgrades to Watch For

| Dependency | Risk | Watch For |
|-----------|------|-----------|
| cert-manager major (v1 → v2) | Breaking | API group changes, CRD schema changes |
| k8s.io major (e.g., v0.35 → v0.36) | Breaking | API removals, type changes, replace directive updates |
| Flux major (v2 → v3) | Breaking | API version removals (v1beta1 → v1 migrations) |
| controller-runtime major | Breaking | Interface changes affecting all CRD-based packages |

## Validation Checklist

Before merging any dependency update:

- [ ] `./scripts/sync-versions.sh check` — versions.yaml ↔ go.mod consistency
- [ ] `make verify` — tidy + lint + test
- [ ] `make test-race` — race condition detection
- [ ] k8s.io replace directives unchanged (unless intentionally bumping)
- [ ] `docs/compatibility.md` regenerated if `versions.yaml` changed

## See Also

- [Development Guide § Dependabot Management](/contributing/guide/#dependabot-management) — PR commands for managing Dependabot PRs
- [Compatibility Matrix](/api-reference/compatibility/) — Generated compatibility matrix
- [versions.yaml](https://github.com/go-kure/kure/blob/main/versions.yaml) — Version source of truth
