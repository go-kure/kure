# Development Guide

This guide covers development workflows and tooling for the Kure project.

## Quick Start

```bash
# Get help with all available commands
make help

# Run all standard development tasks
make all

# Quick development cycle
make check
```

## Contributing Workflow

The `main` branch is protected — all changes must go through pull requests.

### Branch Workflow

1. **Create a feature branch** from `main`:
   ```bash
   git checkout -b feat/my-feature main
   ```
   Use branch prefixes: `feat/`, `fix/`, `docs/`, `chore/`

2. **Develop and test locally**:
   ```bash
   make check       # Quick validation
   make precommit   # Full pre-commit checks
   ```

3. **Push and create a pull request**:
   ```bash
   git push -u origin feat/my-feature
   gh pr create
   ```
   Fill out the PR template (`.github/PULL_REQUEST_TEMPLATE.md`).

4. **Pass required CI checks**: `lint`, `test`, `build`

5. **Get 1 approving review**, resolve all conversations

6. **Merge** (linear history required — rebase, no merge commits)

### Branch Protection Rules

Enforced via the `main-protection` [repository ruleset](https://github.com/go-kure/kure/rules/12903081):

- **Required status checks**: `lint`, `test`, `build`
- **Merge queue**: merging goes through a GitHub merge queue (rebase method) that rebases and tests the merged result before landing — no manual rebasing, no auto-rebase force-pushes
- **Pull requests required**: all changes must go through a PR
- **Conversation resolution**: all review threads must be resolved
- **Linear history**: enforced (rebase only, no merge commits)
- **Force pushes**: disabled
- **Branch deletion**: disabled
- **Bypass actors**: `kure-release-bot` (GitHub App) — allowed to push release commits directly

## Development Workflow

### 1. Initial Setup

**Prerequisites:**

- `go` and `mise` — required
- `make`, `git` — required
- `bash` 4+ — required
- `yq` — required by `scripts/sync-versions.sh`; pinned in `mise.toml`
- `mktemp` — required by `scripts/sync-versions.sh` and `scripts/vendor-guard.sh`, and also by the
  `scripts/test/` harness itself (`lib.sh`'s `new_fixture`/`_run`). An external coreutils utility,
  not a bash builtin, but present on virtually every host with coreutils installed; the two
  scripts name the failure clearly on the rare host without it.
- `timeout` (or macOS Homebrew's `gtimeout`) from GNU coreutils — split by scope, not one blanket
  status:
  - **optional for `scripts/sync-versions.sh check`** (production runtime): the two bounded
    probes it uses degrade gracefully to running unbounded, with a startup warning, when neither
    binary is found.
  - **required to run the full local gate** (`mise run verify` / `scripts/test/run-tests.sh`):
    harness cases `09-mvs-floor-hang-timeout.sh` (pre-existing) and
    `36-timeout-present-no-warning.sh` (new) both need a real `timeout`/`gtimeout` to pass — see
    `docs/dependency-updates.md`'s harness section for the caveat.

```bash
# Install dependencies
make deps

# Install development tools
make tools
```

### 2. Development Cycle

```bash
# Format code
make fmt

# Run quick checks (lint, vet, short tests)
make check

# Run all tests
make test

# Run tests with coverage
make test-coverage
```

### 3. Testing

```bash
# Run all tests
make test

# Run tests with verbose output
make test-verbose

# Run tests with race detection
make test-race

# Run only short tests (good for quick feedback)
make test-short

# Run tests with coverage report
make test-coverage

# Run benchmark tests
make test-benchmark

# Run integration tests (when available)
make test-integration
```

### 4. Code Quality

```bash
# Run all linting
make lint

# Format code
make fmt

# Run go vet
make vet

# Tidy modules
make tidy

# Run Qodana static analysis (requires Docker)
make qodana
```

## Pre-commit Workflow

Before committing changes, run:

```bash
make precommit
```

This will:
- Format code with `go fmt`
- Tidy modules
- Run linters
- Run `go vet`  
- Run all tests

## CI/CD Pipeline

The project uses several GitHub Actions workflows:

### Main CI Pipeline (`.github/workflows/ci.yml`)
- **Triggers**: Push to main/develop, PRs
- **Jobs**:
  - Test (unit, race, coverage)
  - Lint and format check
  - Integration tests (main branch only)
  - Security scanning
  - Dependency vulnerability checks

### Qodana Code Quality (`.github/workflows/code_quality.yml`)
- **Triggers**: Push, PRs
- **Purpose**: Static analysis with JetBrains Qodana
- **Uses**: `make deps` for setup

### Create Release (`.github/workflows/release-create.yml`)
- **Triggers**: Manual (`workflow_dispatch`)
- **Inputs**: Release type (alpha/beta/rc/stable/bump), scope, dry-run
- **Purpose**: Creates release commits and tags on `main`, pushes atomically
- **Auth**: Uses GitHub App token (`RELEASE_APP_ID` + `RELEASE_APP_PRIVATE_KEY`); the `kure-release-bot` App is listed as a bypass actor in the `main-protection` repository ruleset, allowing it to push release commits directly to `main`
- **Concurrency**: Only one release at a time (`release-create` group)

To create a release:
1. Go to Actions > "Create Release" > Run workflow
2. Select release type and optional scope
3. Optionally enable dry-run for preview
4. Click "Run workflow"

The pushed tag triggers the release pipeline below.

### Release Pipeline (`.github/workflows/release.yml`)
- **Triggers**: Version tags (`v*.*.*`)
- **Jobs**:
  - Pre-release validation with `make ci-coverage`
  - Release readiness check with `make release-check`
  - Multi-platform build with `make release-build`
  - GitHub release creation
  - Go proxy refresh

### PR Checks (`.github/workflows/pr-checks.yml`)
- **Triggers**: PR events
- **Jobs**:
  - Quick validation with `make check`
  - Security and dependency checks
  - Test coverage validation
  - Changed files analysis
  - Performance benchmarks (when labeled)
  - Documentation validation

## Renovate Management

Dependency updates come from Renovate (`renovate.json`, extending the shared
`go-kure/.github` preset). The **Dependency Dashboard issue** is the control
surface:

- **Gated updates** (every major, all Go-toolchain updates, Flux minors) sit
  under *Pending Approval* — tick the checkbox to let Renovate open the PR.
  Nothing gated is ever proposed on its own.
- **Deferring an update**: leave its dashboard checkbox unticked; there is
  nothing to close. To reopen a closed/ignored update, tick its checkbox on
  the dashboard.
- **Rebasing a PR**: tick the "rebase/retry" checkbox in the PR body, or the
  per-PR entry on the dashboard. Renovate also rebases automatically when the
  PR falls behind the base branch.
- **Closing a PR**: closing it normally tells Renovate not to recreate that
  version; the dashboard lists it under *Closed/Ignored*.

Renovate regenerates `docs/compatibility.md` on its own branch after gomod or
mise bumps (`postUpgradeTasks` running `./scripts/sync-versions.sh generate`),
so its PRs pass the `validate` drift check without manual help. The same
`postUpgradeTasks` rule also runs `sh scripts/sync-tool-versions.sh`, which
keeps the golangci-lint pin in `Makefile`, `.github/workflows/ci.yml` and
`docs/github-workflows.md` in step with `mise.toml` — a bot PR touching those
files, or a `check-tool-versions` failure on one, is this same automation.

For the full dependency update process (review, bundling, version tracking), see [Dependency Updates](/contributing/dependency-updates/).

## Makefile Targets Reference

### Development
- `help` - Display help message
- `all` - Run all standard development tasks
- `info` - Display project information
- `clean` - Clean build artifacts and caches

### Dependencies
- `deps` - Download and tidy Go modules
- `deps-upgrade` - Upgrade all dependencies
- `tools` - Install development tools
- `outdated` - Check for outdated dependencies

### Testing
- `test` - Run all tests
- `test-verbose` - Run tests with verbose output
- `test-race` - Run tests with race detection
- `test-short` - Run short tests only
- `test-coverage` - Run tests with coverage report
- `test-benchmark` - Run benchmark tests
- `test-integration` - Run integration tests

### Code Quality
- `lint` - Run all linters
- `lint-go` - Run golangci-lint
- `fmt` - Format Go code
- `vet` - Run go vet
- `tidy` - Tidy modules
- `qodana` - Run Qodana static analysis

### CI/CD
- `ci` - Run CI pipeline tasks
- `ci-coverage` - Run CI with coverage
- `ci-integration` - Run CI with integration tests
- `check` - Quick code quality check
- `precommit` - Run all pre-commit checks

### Release
- `release TYPE=<type>` - Preview release (dry-run); types: alpha, beta, rc, stable, bump
- `release-check` - Check if ready for release
- `release-build` - Build release artifacts for multiple platforms
### Utilities
- `generate` - Run go generate
- `mod-graph` - Display module dependency graph
- `list-packages` - List all packages
## Environment Variables

Key environment variables the Makefile respects:

- `GO` - Go command (default: `go`)
- `GOROOT` - Go root directory
- `VERSION` - Version string for builds
- `BUILD_DIR` - Clean target artifact directory (default: `bin`)
- `OUTPUT_DIR` - Clean target artifact directory (default: `out`)
- `TEST_TIMEOUT` - Test timeout (default: `30s`)
- `PACKAGE_PATH` - Package path for kurel operations

## Development Tips

### Testing Strategy
- Use `make test-short` for quick feedback during development
- Use `make test-coverage` to check coverage before PRs
- Use `make test-race` to catch concurrency issues
- Use `make check` for quick pre-commit validation

### Code Quality
- The CI pipeline enforces 85% test coverage
- All code must pass golangci-lint checks
- Code must be properly formatted with `go fmt`
- Modules must be tidy

### Active Linters

The `.golangci.yml` enables these linters, aligned with the shared Go standard:

| Linter | Category | Purpose |
|--------|----------|---------|
| `errcheck` | Default | Unchecked errors |
| `govet` | Default | Suspicious constructs |
| `ineffassign` | Default | Ineffectual assignments |
| `staticcheck` | Default | Comprehensive static analysis (includes gosimple S* checks) |
| `unused` | Default | Unused code |
| `bodyclose` | Required | HTTP response body closed |
| `durationcheck` | Required | time.Duration mistakes |
| `errorlint` | Required | Error wrapping issues |
| `exhaustive` | Required | Exhaustive enum switches |
| `misspell` | Required | Common misspellings |
| `nilerr` | Required | Nil error returns |
| `unconvert` | Required | Unnecessary conversions |
| `whitespace` | Required | Unnecessary whitespace |
| `gosec` | Optional | Security checks (kure-specific) |

Formatters: `gofmt`, `goimports` (with `github.com/go-kure/kure` as local prefix).

### Performance
- Benchmark tests can be run with `make test-benchmark`
- PR checks include performance benchmarks when labeled with `performance`
- Build targets include optimized release builds with `-s -w` flags

## Troubleshooting

### Build Issues
```bash
# Clean everything and rebuild
make clean all

# Check Go installation and environment
make info
```

### Test Failures
```bash
# Run tests with verbose output for more details
make test-verbose

# Run specific test
go test -v ./pkg/specific/package -run TestSpecific
```

### Dependency Issues
```bash
# Update dependencies
make deps-upgrade

# Check for outdated or vulnerable dependencies
make outdated
```

This development guide provides a comprehensive overview of the development workflow using the Makefile and CI/CD pipeline.

## Documentation Updates

When modifying a package's public API, update documentation in the same PR:

1. **Package README** — Update the `README.md` in the package directory (e.g., `pkg/stack/README.md`)
2. **Guides** — Check the reverse mapping in `AGENTS.md` for guides that reference the changed package

To verify the docs site builds correctly:

```bash
# Check all mounted files exist
bash site/scripts/check-mounts.sh

# Build site
mise run site:build
```

## Consumer Compatibility

Kure's public packages are consumed by external projects. Keep public APIs stable when possible,
describe integration requirements as reusable library capabilities, and follow the organization
API stability contract for deprecations or breaking changes.

For local co-development, a consumer can use a Go workspace without adding a committed replace
directive:

```bash
go work init
go work use ./kure ./consumer
```

Before publishing a Kure change needed by a consumer, verify Kure with `GOWORK=off`, then update
and verify the consumer against the released Kure version in its own repository.
