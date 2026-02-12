# GitHub Workflows Documentation

This document provides an overview of all GitHub Actions workflows used in the kure project.

**Last Updated:** 2026-01-28

---

## Workflow Summary

| Workflow | File | Triggers | Purpose |
|----------|------|----------|---------|
| [CI](#ci) | `ci.yml` | push, PR, schedule, manual | Comprehensive testing, linting, building, security |
| [Auto-Rebase](#auto-rebase-workflow) | `auto-rebase.yml` | push to main | Rebase all open PRs when main is updated |
| [Release](#release) | `release.yml` | version tags | GoReleaser-based release with CI validation |

---

## CI Workflow

**File:** `.github/workflows/ci.yml`
**Name:** `CI`

### Triggers

- Push to: `main`, `develop`, `release/*`
- Pull requests to: `main`, `develop`
- Schedule: 4am UTC daily (catch external changes)
- Manual dispatch

### Concurrency

Uses `github.sha` to avoid duplicate runs:
- Same commit won't run CI twice (e.g., PR merge → push to main)
- Different commits run independently

```yaml
concurrency:
  group: ci-${{ github.sha }}
  cancel-in-progress: false
```

### Job Dependency Graph

```
┌─────────────────┐
│   lint          │  ← Fast checks: go-version, fmt, tidy, vet, lint
└────────┬────────┘
         │
    ┌────┴────┐
    ▼         ▼
┌───────┐ ┌───────────┐
│ test  │ │ security  │  ← Tests + govulncheck (parallel)
└───┬───┘ └───────────┘
    │
    ▼
┌───────────────────┐
│ coverage-check    │  ← 80% threshold enforcement
└─────────┬─────────┘
          │
    ┌─────┴─────┐
    ▼           ▼
┌───────┐  ┌────────────┐
│ build │  │ k8s-compat │  ← Build artifacts + K8s matrix
└───┬───┘  └────────────┘
    │
    ▼
┌─────────────────────┐
│ cross-platform      │  ← Only on main/release branches
└─────────────────────┘

PR-only jobs (parallel, no blocking):
┌──────────────┐  ┌─────────────────┐  ┌────────────┐
│ rebase-check │  │ analyze-changes │  │ docs-check │
└──────────────┘  └─────────────────┘  └────────────┘
```

### Jobs Detail

| Job | Check Name | Timeout | Dependencies | Purpose |
|-----|------------|---------|--------------|---------|
| `validate` | `lint` | 5 min | - | Go version check, fmt, tidy, vet, lint |
| `test` | `test` | 15 min | validate | Unit tests, race tests, coverage |
| `security` | `Security` | 10 min | validate | govulncheck, outdated deps, sensitive file check |
| `coverage-check` | `Coverage Check` | 5 min | test | 80% threshold, Codecov upload, PR comment |
| `build` | `build` | 10 min | coverage-check | Build kure, kurel, demo |
| `k8s-compat` | `K8s Compatibility` | 15 min | coverage-check | K8s 0.34, 0.35 compatibility matrix |
| `cross-platform` | `Cross-Platform Build` | 15 min | build | linux/darwin/windows × amd64/arm64 (main/release only) |
| `rebase-check` | `rebase-check` | 2 min | - | Verify PR branch is rebased on main (PR only) |
| `analyze-changes` | `Analyze Changes` | 5 min | - | Changed files analysis, breaking change warnings (PR only) |
| `docs-check` | `Docs Check` | 5 min | - | API changes need docs check (PR only) |

### Configuration

- Go Version: `1.24.13`
- Golangci-lint Version: `v1.64.8`
- Coverage Threshold: `80%`
- K8s Versions: `0.34`, `0.35`
- Platforms: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`

### Features

- **gotestfmt** - Nice formatted test output
- **Fail fast** - Jobs depend on validate, so lint failure stops everything
- **Artifact sharing** - Coverage uploaded as artifact, reused by coverage-check
- **PR comments** - Coverage report comment on PRs
- **Skip draft PRs** - `if: github.event.pull_request.draft == false`
- **Sensitive file check** - Warn about potential secrets in code
- **Matrix fail-fast: false** - K8s and cross-platform continue if one fails

---

## Release Workflow

**File:** `.github/workflows/release.yml`
**Name:** `Release`

### Triggers

- Push tags: `v*` (e.g., v1.0.0, v0.1.0-alpha.0)

### Jobs

1. **check-ci** - Verify CI passed for this commit (waits up to 5 min)
2. **validate** - Strict tag format, changelog, and version progression validation
3. **goreleaser** - Cross-platform builds using GoReleaser v2
4. **post-release** - Go proxy refresh

### Configuration

- Go Version: `1.24.13`
- Build Tool: GoReleaser v2
- Platforms: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`, `windows/arm64`
- Tag Format: `^v[0-9]+\.[0-9]+\.[0-9]+(-alpha\.[0-9]+|-beta\.[0-9]+|-rc\.[0-9]+)?$`
- Changelog: Required (must have `## v0.1.0` section)

### CI Status Check

Release workflow verifies CI passed before releasing:

```yaml
check-ci:
  name: Verify CI passed
  steps:
    - name: Check CI status for this commit
      run: |
        # Wait up to 5 minutes for CI to complete
        for i in {1..30}; do
          STATUS=$(gh api repos/.../commits/$COMMIT_SHA/status --jq '.state')
          if [ "$STATUS" = "success" ]; then exit 0; fi
          if [ "$STATUS" = "failure" ]; then exit 1; fi
          sleep 10
        done
```

### Local Release Management

```bash
# Preview release plan
make release TYPE=alpha

# Execute release (creates commits + tag)
make release-do TYPE=alpha

# Push tag to trigger CI
git push origin v0.1.0-alpha.0
```

---

## Auto-Rebase Workflow

**File:** `.github/workflows/auto-rebase.yml`
**Name:** `Auto-Rebase`

### Triggers

- Push to `main` (runs after every merge to main)

### Purpose

Automatically rebases all open PRs targeting main when main is updated. This mirrors the GitLab auto-rebase CI template used in other Wharf repositories.

### How It Works

Uses [`peter-evans/rebase@v4`](https://github.com/peter-evans/rebase) to:
1. Find all open PRs targeting `main`
2. Rebase each PR branch onto the latest `main`
3. Force-push the rebased branch (triggers CI re-run)
4. Skip PRs with conflicts (reports them without failing)

### Configuration

- **Excluded labels:** `dependencies` (Dependabot manages its own branches)
- **Excluded drafts:** yes (no point rebasing work-in-progress)
- **Fork protection:** only runs on `go-kure/kure` (forks lack the required secret)
- **Concurrency:** `cancel-in-progress: true` (newer main state supersedes)

### Authentication

Requires `AUTO_REBASE_PAT` repository secret — a fine-grained PAT with:
- Repository: `go-kure/kure` only
- Permissions: `Contents: Read+Write`, `Pull requests: Read`

A PAT is required because pushes made with `GITHUB_TOKEN` do not trigger subsequent workflow runs. The PAT ensures CI re-runs on rebased branches.

---

## Test Jobs in CI

| Job | Matrix | Command | Uses Makefile? |
|-----|--------|---------|----------------|
| `test` | - | `go test -json -v ./...` | ✅ (deps) |
| `test` | - | `make test-race` | ✅ |
| `test` | - | `make test-coverage` | ✅ |

## Test Targets in Makefile

| Target | Command | Used in CI? | In precommit? |
|--------|---------|-------------|---------------|
| `test` | `go test -timeout 30s ./...` | ✅ | ✅ |
| `test-race` | `go test -race -timeout 30s ./...` | ✅ | - |
| `test-coverage` | `go test -coverprofile=... ./...` | ✅ | - |
| `test-integration` | `go test -tags=integration -timeout 5m ./...` | - | - |
| `vuln` | `govulncheck ./...` | ✅ | - |

## CI vs Pre-commit

| Target | Tasks | Use Case |
|--------|-------|----------|
| `precommit` | fmt, tidy, lint, test | Fast local checks (~10s) |
| `ci` | deps, fmt, tidy, lint, vet, test, test-race, test-coverage, test-integration, build, vuln | Comprehensive CI pipeline (~2min) |

---

## Configuration Standards

### Go Version

All workflows use Go **1.24.13** consistently, defined via environment variable:

```yaml
env:
  GO_VERSION: '1.24.13'
```

### Caching

Most workflows use Go module caching:

```yaml
- name: Cache Go modules
  uses: actions/cache@v4
  with:
    path: |
      ~/.cache/go-build
      ~/go/pkg/mod
    key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
    restore-keys: |
      ${{ runner.os }}-go-
```

### Branch Patterns

- Release branches: `release/*` (note: not `releases/*`)
- Development branches: `main`, `develop`

---

## Estimated CI Time

| Scenario | Before (4 workflows) | After (2 workflows) |
|----------|---------------------|---------------------|
| PR opened | ~8 min (duplicate work) | ~4 min |
| Push to main | ~5 min | ~4 min |
| PR merge | ~5 min (full re-run) | ~0 min (same SHA, skipped) |

---

## Maintenance Notes

- **When adding/modifying workflows:** Update this document with changes
- **Version updates:** Run `make sync-go-version` to update Go version in all files
- **Version check:** Run `make check-go-version` to verify consistency
- **Action versions:** Keep GitHub Actions up to date (currently using v4-v6)

---

## See Also

- [Makefile](../Makefile) - Local development commands
- [CLAUDE.md](../CLAUDE.md) - Development guidelines
- [mise.toml](../mise.toml) - Local tool version management
