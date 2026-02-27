# GitHub Workflows Documentation

This document provides an overview of all GitHub Actions workflows used in the kure project.

**Last Updated:** 2026-02-27

---

## Workflow Summary

| Workflow | File | Triggers | Purpose |
|----------|------|----------|---------|
| [CI](#ci-workflow) | `ci.yml` | push, PR, schedule, manual | Comprehensive testing, linting, building, security |
| [Deploy Docs](#deploy-docs-workflow) | `deploy-docs.yml` | push to main (docs paths), `workflow_dispatch` | Multi-version docs deployment |
| [Manage Docs](#manage-docs-workflow) | `manage-docs.yml` | `workflow_dispatch` | Remove, rebuild, or re-point doc versions |
| [Auto-Rebase](#auto-rebase-workflow) | `auto-rebase.yml` | push to main | Rebase all open PRs when main is updated |
| [Release](#release-workflow) | `release.yml` | version tags | GoReleaser-based release with versioned docs deploy |
| [PR Review](#pr-review-workflow) | `pr-review.yml` | pull_request | Two-pass AI code review via ccproxy |

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
└─────────┬───────────┘
          │
          ▼
┌─────────────────────────────────────────────────────────────┐
│ mirror-to-gitlab (main push only, after all checks)         │
└─────────────────────────────────────────────────────────────┘

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
| `docs-build` | `docs-build` | 10 min | - | Hugo build validation with versioned config overlay |
| `docs-check` | `Docs Check` | 5 min | - | API changes need docs check (PR only) |
| `mirror-to-gitlab` | `Mirror to GitLab` | 5 min | build, security, k8s-compat, cross-platform, docs-build | Push main and tags to GitLab mirror; fails on divergence (main only) |

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

1. **test** - Full test run with race detection
2. **validate** - Strict tag format, changelog, and version progression validation
3. **goreleaser** - Cross-platform builds using GoReleaser v2
4. **deploy-docs** - Trigger versioned docs deployment (stable tags only)
5. **post-release** - Go proxy refresh

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

### Release Management

```bash
# Preview release plan locally (dry-run)
make release TYPE=alpha

# Create release via CI:
#   Actions > "Create Release" > type=alpha > Run workflow
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

## PR Review Workflow

**File:** `.github/workflows/pr-review.yml`
**Name:** `PR Review`

### Triggers

- Pull requests: `opened`, `synchronize`, `ready_for_review`, `reopened`
- Skips draft PRs and fork PRs (self-hosted runner security)

### How It Works

Uses a two-pass AI review system via ccproxy (ported from the GitLab `mr-review.yml` template):

1. **Pass 1 — Review:** Sends the PR diff + project context (`AGENTS.md`) to the review model (default: `gpt-5.3-codex`). The model returns up to 3 findings ranked by severity in a structured table. Posted as a sticky PR comment.

2. **Pass 2 — Assessment:** If the review found issues (not LGTM), sends the review + diff to an assessment model (default: `claude-sonnet-4-6`) which fact-checks each finding against the actual diff. Catches hallucinations and false positives. Posted as a second sticky PR comment.

### Requirements

- **Self-hosted runner:** Runs on `autops-kube` label (ARC runner with in-cluster access)
- **ccproxy:** Reachable at `http://openclaw-ccproxy.openclaw.svc:8000` from the runner pod
- **No API keys needed:** ccproxy handles model authentication

### Configuration

Configurable via repository variables or workflow env defaults:

| Variable | Default | Purpose |
|----------|---------|---------|
| `PR_REVIEW_MODEL` | `gpt-5.3-codex` | Model for code review pass |
| `PR_REVIEW_MAX_DIFF_CHARS` | `50000` | Truncation threshold for large diffs |
| `PR_REVIEW_MAX_TOKENS` | `1500` | Max response tokens for review |
| `PR_REVIEW_CONTEXT` | kure project description | Additional system prompt context |
| `PR_REVIEW_ASSESS_ENABLED` | `true` | Enable/disable assessment pass |
| `PR_REVIEW_ASSESS_MODEL` | `claude-sonnet-4-6` | Model for hallucination checking |
| `PR_REVIEW_ASSESS_MAX_TOKENS` | `4096` | Max response tokens for assessment |
| `PR_REVIEW_AGENTS_FILE` | `AGENTS.md` | Project context file path |

### Non-Blocking

The workflow uses `continue-on-error: true` so review failures never block PR merges.

---

## Deploy Docs Workflow

**File:** `.github/workflows/deploy-docs.yml`
**Name:** `Deploy Docs`

### Triggers

- **Push to main** (paths: `site/**`, `docs/**`, `pkg/**/*.md`, `examples/**/*.md`, `README.md`, `CHANGELOG.md`, `DEVELOPMENT.md`)
- **Manual dispatch** with inputs: `version_slot`, `version_label`, `set_latest`

### How It Works

1. Runs `scripts/gen-versions-toml.sh` to generate a versioned Hugo config overlay
2. Builds the Hugo site with `--config hugo.toml,versions.toml`
3. Deploys the built site to a subdirectory of `go-kure.github.io`

### Trigger Matrix

| Event | What Deploys | Path | BaseURL |
|-------|-------------|------|---------|
| Push to `main` (docs paths) | Dev docs | `/dev/` | `www.gokure.dev/dev/` |
| `workflow_dispatch` | Versioned | `/vX.Y/` | `www.gokure.dev/vX.Y/` |
| `workflow_dispatch` + `set_latest=true` | Versioned + root | `/vX.Y/` + `/` | Both |

### Concurrency

Per-slot concurrency group (`deploy-docs-<slot>`) prevents race conditions when deploying different versions simultaneously.

### Preservation

During deployment, existing version subdirectories (`dev/`, `v*/`), `CNAME`, and `.nojekyll` are preserved. Only the target slot is replaced.

---

## Manage Docs Workflow

**File:** `.github/workflows/manage-docs.yml`
**Name:** `Manage Docs`

### Triggers

- **Manual dispatch only** with inputs: `action`, `version_slot`

### Actions

| Action | Description | Implementation |
|--------|-------------|----------------|
| `remove-version` | Delete a version's docs | Removes `/vX.Y/` directory from deploy target |
| `set-latest` | Change root `/` to a specific version | Triggers `deploy-docs.yml` with `set_latest=true` |
| `rebuild-version` | Re-trigger a docs build | Triggers `deploy-docs.yml` for the specified version |

### Common Scenarios

```bash
# Roll back latest to an older version:
#   Actions > "Manage Docs" > action=set-latest > version_slot=v0.1

# Remove a yanked version:
#   Actions > "Manage Docs" > action=remove-version > version_slot=v0.2

# Rebuild after theme or script changes:
#   Actions > "Manage Docs" > action=rebuild-version > version_slot=dev
```

---

## Versioned Documentation

The docs site supports multiple documentation versions at different URL paths.

### URL Structure

| Path | Content | Updated By |
|------|---------|------------|
| `/` | Latest stable release | Release workflow (`set_latest=true`) |
| `/vX.Y/` | Specific stable version | Release workflow or manual dispatch |
| `/dev/` | Development (from `main`) | Every push to `main` that touches docs |

### Version Switcher

The [Relearn theme](https://mcshelby.github.io/hugo-theme-relearn/) provides a native version dropdown in the sidebar. It is configured via `params.versions` entries in `versions.toml`, which `gen-versions-toml.sh` generates from git tags.

### How `gen-versions-toml.sh` Works

```bash
# Generate config overlay for a dev build:
./scripts/gen-versions-toml.sh --version dev

# Generate for a stable release:
./scripts/gen-versions-toml.sh --version v0.1.0 --latest v0.1.0
```

The script:
1. Reads all stable tags (`vX.Y.Z` without pre-release suffix) from git
2. Deduplicates to minor level (keeps highest patch per `vX.Y`)
3. Generates `site/versions.toml` with `[params]` section and `[[params.versions]]` entries
4. Marks the latest version with `isLatest = true` and root `baseURL`
5. Always includes a "Development" entry pointing to `/dev/`

### WIP Banner

The development version shows a warning banner linking to the latest stable version (if one exists). Stable versions show no banner.

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
- [mise.toml](../mise.toml) - Local tool version management
- [gen-versions-toml.sh](../scripts/gen-versions-toml.sh) - Versioned docs config generator
