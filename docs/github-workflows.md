# GitHub Workflows Documentation

This document provides an overview of all GitHub Actions workflows used in the kure project.

**Last Updated:** 2026-06-02

---

## Workflow Summary

| Workflow | File | Triggers | Purpose |
|----------|------|----------|---------|
| [CI](#ci-workflow) | `ci.yml` | push, PR, schedule, manual | Comprehensive testing, linting, building, security |
| [Deploy Docs](#deploy-docs-workflow) | `deploy-docs.yml` | push to main (docs paths), `workflow_dispatch` | Multi-version docs deployment |
| [Manage Docs](#manage-docs-workflow) | `manage-docs.yml` | `workflow_dispatch` | Remove, rebuild, or re-point doc versions |
| [Release / Create](#release--create-workflow) | `release-create.yml` | manual | Auto-infer release type from VERSION, create tag |
| [Release / Promote](#release--promote-workflow) | `release-promote.yml` | manual | Promote to explicit release type (beta/rc/stable) |
| [Release / Bump](#release--bump-workflow) | `release-bump.yml` | manual | Advance version cycle (minor/major/prerelease), no tag |
| [Release / Publish](#release--publish-workflow) | `release-publish.yml` | tag push | GoReleaser, SBOM, cosign signing, docs deploy, proxy refresh |
| [PR Review](#pr-review-workflow) | `pr-review.yml` | pull_request | Two-pass AI code review via claude-max-proxy |

---

## CI Workflow

**File:** `.github/workflows/ci.yml`
**Name:** `CI`

### Triggers

- Push to: `main`, `develop`, `release/*`
- Pull requests to: `main`, `develop`
- Merge group (merge queue's temporary branch — required checks must report here)
- Schedule: 4am UTC daily (catch external changes)
- Manual dispatch

### Concurrency

Uses `github.ref` to cancel superseded runs on the same branch or PR:

```yaml
concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true
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
    │
    ▼
┌───────┐
│ build │  ← Aggregation gate
└───┬───┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ mirror-to-gitlab (main push only, after all checks)         │
└─────────────────────────────────────────────────────────────┘

PR-only jobs (parallel, no blocking):
┌─────────────────┐  ┌────────────┐
│ analyze-changes │  │ docs-check │
└─────────────────┘  └────────────┘
```

On `merge_group` events (merge queue), `lint`/`test`/`build` run against the queue's
temporary branch — the merged result — before the PR is allowed to land.

### Jobs Detail

| Job | Check Name | Timeout | Dependencies | Purpose |
|-----|------------|---------|--------------|---------|
| `validate` | `lint` | 15 min | changes | Go fmt, tidy, vet, lint; caches goimports + yq binaries |
| `test` | `test` | 20 min | changes | Unit tests with race detection and coverage; `-race` compilation takes ~5 min on the in-cluster runner, so 20 min allows compilation + 15 min for test execution |
| `security` | `Security` | 5 min | changes | govulncheck (`-scan package`, v1.1.4, informational — findings warn but do not fail), outdated deps, sensitive file check |
| `coverage-check` | `Coverage Check` | 5 min | test | 85% threshold, Codecov upload, PR comment |
| `build` | `build` | 1 min | validate, test, docs-build, coverage-check | Aggregation gate — fails if any required job failed |
| `analyze-changes` | `Analyze Changes` | 5 min | - | Changed files analysis, breaking change warnings (PR only) |
| `docs-build` | `docs-build` | 15 min | changes | Hugo build; separate Go + Hugo caches |
| `docs-check` | `Docs Check` | 5 min | changes | API changes need docs check (PR only) |
| `mirror-to-gitlab` | `Mirror to GitLab` | 5 min | build, security, docs-build | Push main and tags to GitLab mirror; fails on divergence (main only) |

### Configuration

- Go Version: read from `go.mod` (`go-version-file: go.mod`)
- Golangci-lint Version: `v2.10.1`
- govulncheck Version: `v1.1.4` (pinned, cached binary, `-scan package` mode)
- Coverage Threshold: `85%`

### Features

- **gotestfmt** - Nice formatted test output
- **Fail fast** - Jobs depend on validate, so lint failure stops everything
- **Artifact sharing** - Coverage uploaded as artifact, reused by coverage-check; both upload and download use `continue-on-error: true` to tolerate `ACTIONS_RESULTS_URL` failures on in-cluster ARC runners
- **PR comments** - Coverage report comment on PRs
- **Skip draft PRs** - `if: github.event.pull_request.draft == false`
- **Sensitive file check** - Warn about potential secrets in code
- **goimports** - Installed as a tool dependency for the formatting check (`goimports -l`)
- **Matrix fail-fast: false** - Cross-platform builds continue if one fails

---

## Release / Create Workflow

**File:** `.github/workflows/release-create.yml`
**Name:** `Release / Create`

### Triggers

- Manual dispatch with input: `dry_run` (boolean)

### How It Works

The release type is **auto-inferred from the VERSION file** by `release.sh`. No `type` or `scope` inputs are needed. The regression guard in `release.sh` blocks invalid transitions automatically.

### Pre-release Test Gate

The workflow runs a full test suite (with race detection) **before** creating the tag. This prevents tags from being pushed when tests fail.

```
workflow_dispatch
  → test job (go test -race ./...)
    → release job (needs: test)
      → release.sh (auto-infer type from VERSION) → creates tag + pushes
        → triggers release-publish.yml (tag push)
```

If the pre-release test fails, the release job never runs and no tag is created.

### Jobs

1. **test** — Full test run with race detection and CGO enabled (`build-essential` + `CGO_ENABLED=1`)
2. **release** — Runs `scripts/release.sh` to generate changelog, commit, create tag, and push

### Authentication

Uses a GitHub App token (`RELEASE_APP_ID` + `RELEASE_APP_PRIVATE_KEY`) so that the tag push triggers subsequent workflows (tag-triggered `release-publish.yml`).

### Usage

```bash
# Preview release (auto-infer from VERSION)
./scripts/release-trigger.sh

# Create release via CI:
#   Actions > "Release / Create" > Run workflow (type is auto-inferred)
```

---

## Release / Promote Workflow

**File:** `.github/workflows/release-promote.yml`
**Name:** `Release / Promote`

### Triggers

- Manual dispatch with inputs: `to` (beta/rc/stable) and `dry_run`

### Purpose

Explicit type transition (e.g., beta → rc). The regression guard in `release.sh` blocks invalid downgrade transitions (e.g., rc → beta will fail with an error).

### Usage

```bash
# Preview promotion to rc
./scripts/release-trigger.sh promote rc

# Execute via CI:
#   Actions > "Release / Promote" > to=rc > Run workflow
./scripts/release-trigger.sh promote rc --do-it
```

---

## Release / Bump Workflow

**File:** `.github/workflows/release-bump.yml`
**Name:** `Release / Bump`

### Triggers

- Manual dispatch with inputs: `scope` (minor/major/prerelease) and `dry_run`

### Purpose

Advance the version cycle without creating a tag. Use before starting a new prerelease cycle (e.g., after a stable release, to begin the next minor version's alpha).

### Usage

```bash
# Preview minor version bump
./scripts/release-trigger.sh bump minor

# Execute via CI:
#   Actions > "Release / Bump" > scope=minor > Run workflow
./scripts/release-trigger.sh bump minor --do-it
```

---

## Release / Publish Workflow

**File:** `.github/workflows/release-publish.yml`
**Name:** `Release / Publish`

### Triggers

- Push tags: `v*` (e.g., v1.0.0, v0.1.0-beta.2)

### Jobs

1. **test** - Full test run with race detection
2. **validate** - Strict tag format, changelog, and version progression validation
3. **goreleaser** - Build release artifacts, generate SBOM, sign with cosign
4. **deploy-docs** - Trigger versioned docs deployment (stable tags only)
5. **post-release** - Go proxy refresh

### Configuration

- Go Version: read from `mise.toml`
- Tag Format: `^v[0-9]+\.[0-9]+\.[0-9]+(-alpha\.[0-9]+|-beta\.[0-9]+|-rc\.[0-9]+)?$`
- Changelog: Required (must have `## [0.1.0]` section — version without `v` prefix, in square brackets)

### Release Management

```bash
# Preview release (auto-infer from VERSION)
./scripts/release-trigger.sh

# Preview type promotion
./scripts/release-trigger.sh promote rc

# Preview version bump
./scripts/release-trigger.sh bump minor

# Execute via CI (add --do-it to any of the above)
./scripts/release-trigger.sh --do-it
./scripts/release-trigger.sh promote rc --do-it
./scripts/release-trigger.sh bump minor --do-it
```

---

## Merge Queue

kure merges through GitHub's native **merge queue** (configured in the `main-protection`
ruleset, not a workflow file). This replaced the former `rebase-check` job and `auto-rebase.yml`
workflow — it is the native equivalent of GitLab's merged-results pipelines.

### How It Works

1. A reviewed PR is added to the queue ("Merge when ready").
2. The queue creates a temporary branch combining `main` + the PR and fires a `merge_group`
   event; `lint`/`test`/`build` run against that **merged result**.
3. If green, the PR lands on `main` with the **rebase** merge method (linear history preserved).
   If the merged result fails, the PR is dropped from the queue and `main` stays green.

### Why

- Tests the actual merged result, which `rebase-check` (ancestry-only) could not.
- No force-pushing contributor branches and no per-merge auto-rebase storm — the queue rebases
  once, at merge time.

### Configuration (ruleset `merge_queue` rule)

- **Merge method:** `REBASE` (linear history)
- **Grouping:** `ALLGREEN` (a failing entry is dropped from the group)
- **Batch size:** 1 (conservative; tune after observing runner load)
- **Required checks on the queue:** `lint`, `test`, `build` (must also trigger on `merge_group`)

Auto-merge is **not** enabled — every PR is reviewed and queued manually. The merge queue rule is
managed centrally in `go-kure/.github` (`governance/repository-settings-policy.yaml`).

---

## PR Review Workflow

**File:** `.github/workflows/pr-review.yml`
**Name:** `PR Review`

### Triggers

- Pull requests: `opened`, `synchronize`, `ready_for_review`, `reopened`
- Skips draft PRs and fork PRs (self-hosted runner security)

### How It Works

Uses a two-pass AI review system via the in-cluster claude-max-proxy (ported from the GitLab `mr-review.yml` template):

1. **Pass 1 — Review:** Sends the PR diff + project context (`AGENTS.md`, `.claude/CLAUDE.md`) to the review model (default: `claude-opus-4`). Anti-hallucination rules prevent the model from inventing standards or referencing code not in the diff. The model returns up to 3 findings ranked by severity in a structured table. Posted as a PR comment.

2. **Pass 2 — Assessment:** If the review found issues (not LGTM), sends the review + diff to an assessment model (default: `claude-sonnet-4-6`) which fact-checks each finding against the actual diff and project context. Includes standards verification — claims about "standards violations" are checked against actually-provided standards. Catches hallucinations and false positives. Posted as a second PR comment.

### Requirements

- **Self-hosted runner:** Runs on `autops-kube` label (ARC runner with in-cluster access)
- **claude-max-proxy:** Reachable at `http://openclaw-claude-proxy.openclaw.svc:3456` from the runner pod
- **No API keys needed:** the proxy handles model authentication

### Configuration

Configurable via repository variables or workflow env defaults:

| Variable | Default | Purpose |
|----------|---------|---------|
| `PR_REVIEW_MODEL` | `claude-opus-4` | Model for code review pass (cosmetic label; backend ignores the model field) |
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
| `ci` | deps, fmt, tidy, lint, vet, test, test-race, test-coverage, test-integration, vuln | Comprehensive CI pipeline (~2min) |

---

## Configuration Standards

### Go Version

All jobs use `go-version-file: go.mod` — the `go` directive in `go.mod` is the single
source of truth (kept in sync with `mise.toml` via `make check-go-version`).

### Caching

CI jobs use explicit `actions/cache@v5` steps with `cache: false` on `setup-go` to
control cache keys precisely. Two separate Go caches are maintained:

```yaml
# Module cache: stable, invalidates only when go.sum changes
- name: Cache Go modules
  uses: actions/cache@v5
  with:
    path: ~/go/pkg/mod
    key: ${{ runner.os }}-gomod-${{ hashFiles('**/go.sum') }}
    restore-keys: |
      ${{ runner.os }}-gomod-

# Build cache: invalidates when go.sum changes, shared across commits with same deps
- name: Cache Go build cache
  uses: actions/cache@v5
  with:
    path: ~/.cache/go-build
    key: ${{ runner.os }}-gobuild-${{ hashFiles('**/go.sum') }}
    restore-keys: |
      ${{ runner.os }}-gobuild-
```

Tool binaries are also cached to avoid reinstalling on every run:
- `goimports` — keyed by `go.sum` hash (tied to `golang.org/x/tools` version)
- `yq` — keyed by pinned version (`4.44.6`)
- `govulncheck` — keyed by pinned version (`v1.1.4`)

Cache and artifact traffic is routed through an in-cluster falcondev cache server backed by
Garage S3. Two layers work together:

1. **Binary patch** (`containers/actions-runner/Dockerfile` in opsmaster): `Runner.Worker.dll`
   is patched to read `CUSTOM_ACTIONS_RESULTS_URL` instead of `ACTIONS_RESULTS_URL` for its own
   internal connection. `CUSTOM_ACTIONS_RESULTS_URL` is set as a pod env var in the runner's
   `HelmRelease`. This ensures the Worker process itself connects through the cache server.

2. **Workflow env** (`ACTIONS_RESULTS_URL`): The binary patch replaces **all** UTF-16LE
   occurrences of `ACTIONS_RESULTS_URL` in the DLL — including the name the Worker injects into
   step process environments (renamed to `ACTIONS_RESULTS_ORL` as a side effect). Setting
   `ACTIONS_RESULTS_URL` in the workflow `env:` block overrides this so step processes
   (`upload-artifact`, `download-artifact`, `actions/cache` v2) see the correct URL.

`ACTIONS_CACHE_URL` / `ACTIONS_CACHE_SERVICE_V2` are not needed — cache actions use the v2
Results API path through `ACTIONS_RESULTS_URL`.

### docs-build Caching

The `docs-build` job uses two separate caches:
- `gomod` — Go module cache
- `hugo` — Hugo module cache (`$HUGO_CACHEDIR` only, **not** `~/go/pkg/mod`)

### Path Filters

The `changes` job uses `dorny/paths-filter` to skip jobs when unrelated files change:

- `go:` filter — triggers lint/test/security/build jobs. Includes `**.go`, `go.mod`, `go.sum`,
  `Makefile`, and **`.github/workflows/**`** so that workflow-only PRs are also validated.
- `docs:` filter — triggers docs-build/docs-check jobs. Includes `site/**`, `docs/**`, `*.md`,
  `scripts/**`, and `.github/workflows/ci.yml` (only ci.yml, since other workflows don't affect
  the docs build).

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

## Self-Hosted Runner Requirements

All jobs run on the `autops-kube-kure` GitHub ARC scale-set, which uses a custom runner image
(`ghcr.io/ginsys/opsmaster/actions-runner:latest`) — a minimal Ubuntu image that includes
`curl` and `git` but **not** `make` or `wget`.

To account for this:
- Every job that calls `make` includes an explicit install step: `sudo apt-get install -y --no-install-recommends make`
- All `wget` calls have been replaced with `curl -fsSL -o`

---

## Maintenance Notes

- **When adding/modifying workflows:** Update this document with changes
- **Version updates:** Run `make sync-go-version` to update Go version in all files
- **Version check:** Run `make check-go-version` to verify consistency
- **Action versions:** Keep GitHub Actions up to date (currently using v3-v6)
- **New jobs using `make`:** Add the `Install build tools` step (see above) if the job runs on `autops-kube`

---

## See Also

- [Makefile](https://github.com/go-kure/kure/blob/main/Makefile) - Local development commands
- [mise.toml](https://github.com/go-kure/kure/blob/main/mise.toml) - Local tool version management
- [gen-versions-toml.sh](https://github.com/go-kure/kure/blob/main/scripts/gen-versions-toml.sh) - Versioned docs config generator
