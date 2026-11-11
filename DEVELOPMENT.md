# Development Guide

This guide covers development workflows and tooling for the Kure project.

## Quick Start

```bash
# Get help with all available commands
make help

# Quick development cycle (lint, vet, short tests, tool pins, generated builders)
make check

# The local CI checks (CI also enforces the coverage thresholds and the rendered-site link check)
make ci
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

4. **Pass required CI checks**: `lint`, `test`, `build`, `pr-review / AI Code Review`

5. **Resolve all conversations** (no approving review is required)

6. **Merge** (linear history required — rebase, no merge commits)

### Branch Protection Rules

Enforced via the `main-protection` [repository ruleset](https://github.com/go-kure/kure/rules/12903081):

- **Required status checks**: `lint`, `test`, `build`, `pr-review / AI Code Review`
- **Merge queue**: merging goes through a GitHub merge queue (rebase method) that rebases and tests the merged result before landing — no manual rebasing, no auto-rebase force-pushes
- **Pull requests required**: all changes must go through a PR
- **Conversation resolution**: all review threads must be resolved
- **Linear history**: enforced (rebase only, no merge commits)
- **Force pushes**: disabled
- **Branch deletion**: disabled
- **Bypass actors**: `kure-release-bot` (GitHub App) — allowed to push release commits directly

Release branches (`release/vX.Y`) have their own `release-protection` ruleset with the same rules,
required checks and bot bypass, but no merge queue: a pull request must be up to date with the
branch before it merges. See [Releasing](/contributing/releasing/).

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

# Run the tests of one package, verbosely
go test -v ./pkg/errors/

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

#### Golden fixtures and schema validation

The builder families under `pkg/kubernetes` keep their expected output as golden fixtures in
`testdata/`. `internal/kuretest.Golden` validates an object against the pinned
CustomResourceDefinition schema (and the API server's ObjectMeta rules) before comparing it
with its fixture, and rewrites the fixture under `-update` only when it validates:

```bash
GOWORK=off go test ./pkg/kubernetes/cnpg/ -run TestGolden -update
```

A fixture the pinned schema rejects is a defect in the test or the builder, not a fixture to
update. The validator reads each pinned module's definitions from the module cache through
`go list`, so it needs the module graph as `go.mod` states it: under a parent `go.work` that
replaces a pinned module, the version check fails and its message names `GOWORK=off` (the
Makefile exports it already), and a `replace` directive that changes a pinned module's path or
version is refused the same way. The same validation runs on the Flux bootstrap output of
`pkg/stack/fluxcd` and on everything `examples/getting-started` writes, so a generator that
drifts from the pinned schema fails `make test`, not a cluster. Which kinds are held to a
schema and which get ObjectMeta validation only is listed in `pkg/kubernetes/README.md`
§ Schema validation test.

A second pass holds the objects to each other rather than to a schema: `AssertConsistent`,
`AssertConsistentYAML` and `AssertConsistentDir` fail when a Flux `sourceRef`, `chartRef` or
`dependsOn` names nothing in the tree, or an object or a Kustomization's `targetNamespace`
names a namespace the tree does not create — unless the test declares it with `External` or
`Namespace`, or it is one the cluster makes itself. On a written directory, `AssertConsistentDir`
reads every `.yaml` and `.yml` file but one a `kustomization.yaml` names as data only, and every
file one lists as a resource whatever its name, as kustomize applies it, and a `.yaml` or `.yml`
one named as data under a Flux Kustomization's `spec.path` that has no `kustomization.yaml`, but
not in a subdirectory with its own, since Flux's generator lists it there and takes such a
subdirectory whole; it also fails when a Kustomization's `spec.path` or an entry of a
`kustomization.yaml` names nothing in the tree, or an entry of a `kustomization.yaml` or of a
Kustomization's `spec.components` leaves it, and builds what each Flux Kustomization applies the
way kustomize-controller builds it, on a copy of the tree — nothing, once an entry leaves the tree.

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

# Run the faster linters only (no type analysis)
make lint-fast

# Check for known vulnerabilities (govulncheck)
make vuln
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
- Run all tests
- Check the pinned tool versions and the govulncheck version mentioned in the docs
- Run the `sync-versions.sh` guard tests
- Check that the generated builders are up to date
- Run the offline checks of CI's docs build (`make check-docs`)

## CI/CD Pipeline

The project uses several GitHub Actions workflows:

### Main CI Pipeline (`.github/workflows/ci.yml`)
- **Triggers**: push to `main`, `develop` and `release/*`; every pull request (any base branch);
  the merge queue; a daily schedule; manual dispatch
- **Jobs** (job names as the required checks show them): `lint`, `test`, `build`, `Security`,
  `Coverage Check`, `docs-build`, `doc-gate`, `forbidden-terms`, `action-pins`, `pin-impact` and
  `Analyze Changes`

### Documentation (`.github/workflows/deploy-docs.yml`, `.github/workflows/manage-docs.yml`)
- **Deploy Docs**: publishes the site on a push to `main` that touches `site/`, `docs/`, package or
  example READMEs, `README.md`, `CHANGELOG.md` or `DEVELOPMENT.md`, and on manual dispatch
- **Manage Docs**: manual dispatch only; acts on a version slot (`dev` or `vX.Y`) of the published
  site

### PR Review (`.github/workflows/pr-review.yml`) and Claude Code (`.github/workflows/claude.yml`)
- **PR Review**: the AI code review that is a required check (`pr-review / AI Code Review`), run on
  pull requests and on the merge queue
- **Claude Code**: answers issue comments and newly opened or assigned issues

### Release (`.github/workflows/release.yml`) and Release / Publish (`.github/workflows/release-publish.yml`)

Run **Release** from the Actions tab: pick the branch (`main`, or `release/vX.Y` for a patch), what
to do (`release` by default), and tick **Dry run** for a preview. It makes the release commits and
the tag, and **Release / Publish** then runs by itself on the tag. Nothing is tagged by hand. The
[Releasing](/contributing/releasing/) page (`docs/releasing.md`, shared by every go-kure
repository) explains every option, release branches, and what to do when a release fails. kure's
Publish creates the GitHub release object only: kure is a library, so a complete release carries no
assets and the tag is the artifact.

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
  per-PR entry on the dashboard. Renovate rebases on its own only when the PR
  conflicts with the base branch (`rebaseWhen: conflicted` in `renovate.json`).
  A PR that is only behind `main` stays as it is: the merge queue rebases it and
  tests the merged result. The shared preset rebases on every move of `main`;
  this repository overrides that because kure and launcher share one CI runner
  pool, and each such rebase started a full CI run.
- **Closing a PR**: closing it normally tells Renovate not to recreate that
  version; the dashboard lists it under *Closed/Ignored*.

Renovate regenerates `docs/compatibility.md` on its own branch after gomod or
mise bumps (`postUpgradeTasks` running `./scripts/sync-versions.sh generate`),
so its PRs pass the `validate` drift check without manual help. The same
`postUpgradeTasks` rule also runs `sh scripts/sync-tool-versions.sh`, which
keeps the golangci-lint pin in `Makefile`, `.github/workflows/ci.yml` and
`docs/github-workflows.md` in step with `mise.toml`, and `sh
scripts/sync-go-version.sh`, which keeps `go.mod`'s own `go` directive in step
with `mise.toml`'s `go` pin — a bot PR touching those files, or a
`check-tool-versions`/`check-go-version` failure on one, is this same
automation. The rule's last command, `./scripts/gen-builders.sh generate`,
regenerates the per-kind constructor wrappers under `pkg/kubernetes`
(`zz_generated_*.go`) from the registered scheme, so an API-module bump that
adds or drops a kind arrives with its wrapper; `validate` runs
`./scripts/gen-builders.sh check` and fails on drift. Locally:
`make gen-builders` / `make check-builders` (or `mise run builders:generate` /
`builders:check`).

For the full dependency update process (review, bundling, version tracking), see [Dependency Updates](/contributing/dependency-updates/).

## Makefile Targets Reference

`make help` prints the same list with one-line descriptions.

### Development
- `help` - Display help message
- `info` - Display project information
- `dev` - Set up the development environment (mise, dependencies, git hooks)
- `clean` - Clean build artifacts and caches
- `changelog-preview` - Print the unreleased changelog entries

### Dependencies
- `deps` - Download and tidy Go modules
- `deps-upgrade` - Upgrade all dependencies
- `outdated` - Check for outdated dependencies
- `tools` - Install development tools

### Testing
- `test` - Run all tests
- `test-short` - Run short tests only
- `test-race` - Run tests with race detection
- `test-coverage` - Run tests with coverage report
- `test-benchmark` - Run benchmark tests
- `test-integration` - Run integration tests
- `test-fuzz` - Fuzz target of the Makefile. It points at `./pkg/patch/...`, which no longer exists, so it fails

### Code Quality
- `lint` - Run golangci-lint
- `lint-fast` - Run the fast linters only (no type analysis)
- `fmt` - Format Go code
- `vet` - Run go vet
- `tidy` - Tidy modules
- `vuln` - Run the govulncheck vulnerability check

### Consistency checks and generators
- `sync-go-version` / `check-go-version` - Sync or verify the Go version across files from `mise.toml`
- `sync-tool-versions` / `check-tool-versions` - Sync or verify the golangci-lint pin
- `sync-govulncheck-docs` / `check-govulncheck-docs` - Sync or verify the govulncheck version mentioned in the docs
- `gen-builders` / `check-builders` - Regenerate or verify the generated constructor wrappers under `pkg/kubernetes`
- `check-docs` - Run the offline checks of CI's docs build (API references, site self-links, doc examples)
- `versions-test` - Run the `sync-versions.sh` guard tests (hermetic, no network)

### CI/CD
- `check` - Quick code quality check (lint, vet, short tests, tool pins, generated builders)
- `precommit` - Fast pre-commit checks (see Pre-commit Workflow)
- `ci` - The local CI checks (CI also enforces the coverage thresholds and the rendered-site link check)

### Release
No target releases or previews a release: run the **Release** workflow, with **Dry run** ticked for
a preview ([Releasing](/contributing/releasing/)). `changelog-preview` prints the unreleased
changelog entries.

## Environment Variables

Set from the environment (`?=`, so `VAR=value make <target>` and an exported `VAR` both work):

- `GOROOT` - Go root directory (default: `go env GOROOT`)
- `VERSION` - Version string for builds (default: `git describe`, else `dev`)
- `TEST_TIMEOUT` - Test timeout, applied per test binary (default: `30m`, the same value as the `test` job in CI)

Assigned with `:=`, so the **environment does not reach them** — override with a make command-line
assignment instead, as in `make test TEST_PACKAGES=./pkg/errors/`:

- `GO` - Go command (default: `go`)
- `TEST_PACKAGES` - Package pattern the test targets run (default: `./...`)
- `BUILD_DIR`, `OUTPUT_DIR`, `COVERAGE_DIR` - directories `clean` removes (`bin`, `out`,
  `coverage`); `COVERAGE_DIR` is also where `test-coverage` writes its profile and HTML report

`COVERAGE_THRESHOLD` (`Makefile:30`) is **defined and never referenced** — setting it changes
nothing. It also disagrees with the gate that does run: it reads `80`, while the `coverage-check` CI
job (`.github/workflows/ci.yml`) hardcodes `90` for both total and per-package coverage.
Treat 90 as the number that matters.

## Development Tips

### Testing Strategy
- Use `make test-short` for quick feedback during development
- Use `make test-coverage` to check coverage before PRs
- Use `make test-race` to catch concurrency issues
- Use `make check` for quick pre-commit validation
- Refresh a golden fixture with `-update` (Golden fixtures and schema validation, above); the
  helper refuses to write one the pinned schema rejects

### Code Quality
- CI gates test coverage — see the `COVERAGE_THRESHOLD` note above for the number that applies
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
- Build targets include optimized release builds with `-s -w` flags

## Troubleshooting

### Build Issues
```bash
# Remove build artifacts and the Go caches (this does not rebuild)
make clean

# Check Go installation and environment
make info
```

### Test Failures
```bash
# Run tests with verbose output for more details
go test -v ./pkg/specific/package

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
# Offline checks of CI's docs build: API references, site self-links, doc examples
make check-docs

# Check the code-to-docs mapping in site/docs-map.yaml
mise run site:check

# Build the site
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
