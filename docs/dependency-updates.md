# Dependency Updates Guide

This guide covers the process for updating Kure's dependencies, including version tracking, risk assessment, and coordinated upgrades.

## Version Management Overview

Kure tracks dependency versions in three places:

| File | Purpose |
|------|---------|
| `go.mod` | Go module dependencies — authoritative for the build version (the pin) |
| `versions.yaml` | Version metadata: supported range, notes (no build version) |
| `docs/compatibility.md` | Generated from `versions.yaml` + `go.mod` — never edit directly |
| `renovate.json` | Bot policy: update gates and caps, on top of the shared `go-kure/.github` preset |

The `sync-versions.sh check` command performs four assertions. It runs in CI's `validate`
job, which is gated on the `go` paths-filter in `.github/workflows/ci.yml` — that filter
includes `versions.yaml`, `docs/compatibility.md` and `scripts/sync-versions.sh` precisely
so a version-metadata-only PR cannot skip it:

1. **Range** — each dependency's build version, read from `go.mod`, falls within the
   `supported_range` declared in `versions.yaml`.
2. **go.mod pin comment** — the `// Current pin: vX.Y.Z (Kubernetes 1.N)` comment above
   the `k8s.io/api` replace directive matches that directive's actual version. A drifted
   comment used to be caught only by AI review, on every single Kubernetes bump.
3. **No raw commit SHAs in `versions.yaml` notes** — a `notes:` block may not contain a
   bare 40- or 12-hex commit SHA literal. When a dependency has no semver tags (e.g.
   external-secrets, pinned to a `go.mod` pseudo-version), reference "the pseudo-version
   pinned in `go.mod`" instead of writing out the commit by hand — the literal drifts
   silently the next time the pin moves. This is **not** a ban on patch-level semver in
   justification prose (e.g. metallb's "v0.16.0 is a minor release…" stays fine).
4. **Drift** — the committed `docs/compatibility.md` is byte-identical to what
   `generate` would produce right now. It regenerates into a temp file and diffs;
   your working tree is never touched. On failure it prints the diff (`-` is what is
   committed, `+` is what `versions.yaml` + `go.mod` currently imply) and tells you to
   run `generate`.

`generate` regenerates `docs/compatibility.md` and the `go.mod` pin comment in place.
There is no other hand-maintained "current" version to keep in sync (see
[#593](https://github.com/go-kure/kure/issues/593)).

The drift assertion exists because the matrix is generated but committed: before it, any
bump that moved a build version or a `supported_range` left the committed matrix silently
stale until someone happened to re-run `generate`, and the staleness was caught only by
human review — on three separate dependency PRs.

## Update Risk Levels

### Patch Updates (Low Risk)

Patch bumps (e.g., v1.5.0 → v1.5.1) contain bug fixes only.

```bash
go get <module>@v<new-version>
go mod tidy
```

No `versions.yaml` change is needed for an in-range patch — the build version comes from
`go.mod`, and `sync-versions.sh check` passes as long as the new version stays within
`supported_range`. (This is what lets in-range Renovate patch bumps go green untouched.)

### Minor Updates (Medium Risk)

Minor bumps (e.g., v1.19 → v1.20) may add new APIs or deprecate existing ones.

1. Review the upstream changelog for breaking changes
2. Check if Kure uses any deprecated APIs
3. Update `go.mod`; if the new version lands **outside** `supported_range`, widen the
   range and update `notes` in `versions.yaml` (only after confirming API compatibility)
4. Run `make verify` to catch compile-time breakage

### Major Updates (High Risk)

Major bumps (e.g., v1 → v2) likely have breaking API changes.

1. Review the migration guide thoroughly
2. Assess impact on all callers (check with `grep -r` for imports)
3. Update code to use new APIs
4. Update `versions.yaml` and documentation
5. Consider impact on external consumers (see `AGENTS.md` § Consumer Compatibility)

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

Renovate enforces this with the `fluxcd` group in the shared
`go-kure/.github` preset, which covers `github.com/fluxcd/*` and
`github.com/controlplaneio-fluxcd/*`. Ungrouped, one upstream release arrived as five
separate PRs whose `go.mod` changes conflicted with each other in the merge queue and
had to be consolidated by hand. Flux minors are additionally dashboard-gated by
`renovate.json` (majors are gated org-wide by the preset), so the group only ever
carries patches unless a pending minor is deliberately approved on the dashboard.

### Kubernetes (`k8s.io/*`)

All `k8s.io/` packages must stay at the same patch release. Kure uses `replace` directives in `go.mod` to enforce this. See the comment block in `go.mod` for details.

**When can replace directives be removed?** Only when ALL direct and transitive dependencies converge on the same `k8s.io/` minor version. Check with:

```bash
go mod graph | grep 'k8s.io/' | awk '{print $2}' | sort -u
```

### CNPG Ecosystem

`cloudnative-pg`, `barman-cloud`, `machinery`, and `plugin-barman-cloud` are related but versioned independently. Check compatibility notes in `versions.yaml` before upgrading.

### Vendored `go-kure/.github` Guard

`.github/workflows/ci.yml`'s `forbidden-terms` job pins `go-kure/.github` twice: once as
the `check-forbidden-terms` action's `uses:` digest (tracked by Renovate's
`github-actions` manager) and once as a second checkout's `ref:`, which that manager
cannot see. Left alone, the two drift apart and the job byte-compares the vendored
`site/scripts/check-forbidden-terms.sh` against a stale revision.

`renovate.json` closes the gap with a `customManagers` regex entry that tracks the
`ref:` SHA as a `go-kure/.github` `git-refs` dependency, grouped with the `github-actions`
bump via a `packageRules` entry (`matchDepNames: ["go-kure/.github"]`) so both pins move
in the same PR. That same rule's `postUpgradeTasks` runs `./scripts/vendor-guard.sh` on
the bot's branch, which re-fetches `scripts/check-forbidden-terms.sh` from
`go-kure/.github` at the new `ref:` SHA and re-vendors it to `site/scripts/`. The script
is idempotent — a re-run against an already-synced tree makes no further change.

## Bundling Renovate PRs

Renovate's ecosystem groups already land related bumps as one PR, so bundling is
rarely needed. When separate PRs still accumulate (e.g. several groups at once),
bundle them into a single PR:

1. Create a feature branch: `git checkout -b chore/bundle-dependency-updates main`
2. Run `go get` for all dependencies (Flux packages first for coordinated upgrades)
3. Run `go mod tidy`
4. Update `versions.yaml` `supported_range` / `notes` for any bump that lands outside its range
5. Regenerate docs: `./scripts/sync-versions.sh generate`
6. Validate: `./scripts/sync-versions.sh check`
7. Run full verification: `make verify && make test-race`
8. Commit, push, and create PR
9. Reference all Renovate PR numbers in the PR body to auto-close them

## Dangerous Upgrades to Watch For

| Dependency | Risk | Watch For |
|-----------|------|-----------|
| cert-manager major (v1 → v2) | Breaking | API group changes, CRD schema changes |
| k8s.io major (e.g., v0.35 → v0.36) | Breaking | API removals, type changes, replace directive updates |
| Flux major (v2 → v3) | Breaking | API version removals (v1beta1 → v1 migrations) |
| controller-runtime major | Breaking | Interface changes affecting all CRD-based packages |

## Validation Checklist

Before merging any dependency update:

- [ ] `./scripts/sync-versions.sh check` — build versions within `supported_range`, the
      `go.mod` pin comment and `versions.yaml` notes match (run `generate` first if it
      reports drift; reword any raw commit SHA in `notes:` to reference `go.mod` instead)
- [ ] `make verify` — tidy + lint + test
- [ ] `make test-race` — race condition detection
- [ ] k8s.io replace directives unchanged (unless intentionally bumping)

## See Also

- [Development Guide § Renovate Management](/contributing/guide/#renovate-management) — dashboard workflow for managing Renovate PRs
- [Compatibility Matrix](/api-reference/compatibility/) — Generated compatibility matrix
- [versions.yaml](https://github.com/go-kure/kure/blob/main/versions.yaml) — Version source of truth
