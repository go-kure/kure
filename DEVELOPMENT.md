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

- **Required status checks** (strict): `lint`, `test`, `build`, `rebase-check`
- **Auto-rebase**: open PRs are automatically rebased when main is updated (via `auto-rebase.yml`)
- **Pull requests required**: all changes must go through a PR
- **Conversation resolution**: all review threads must be resolved
- **Linear history**: enforced (rebase only, no merge commits)
- **Force pushes**: disabled
- **Branch deletion**: disabled
- **Bypass actors**: `kure-release-bot` (GitHub App) — allowed to push release commits directly

## Development Workflow

### 1. Initial Setup

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

### 3. Building

```bash
# Build all executables
make build

# Build specific executable
make build-kure
make build-kurel
make build-demo

# Build with race detection for debugging
make build-race
```

### 4. Testing

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

### 5. Code Quality

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

### 6. Demo and Examples

```bash
# Run comprehensive demo
make demo

# Run GVK generators demo
make demo-gvk

# Generate all examples (alias for demo)
make examples
```

### 7. Package Operations

```bash
# Build a kurel package
make kurel-build PACKAGE_PATH=path/to/package

# Show package information
make kurel-info PACKAGE_PATH=path/to/package
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
  - Build executables
  - Generate demo outputs
  - Integration tests (main branch only)
  - Cross-platform builds
  - Security scanning
  - Dependency vulnerability checks

### Qodana Code Quality (`.github/workflows/code_quality.yml`)
- **Triggers**: Push, PRs
- **Purpose**: Static analysis with JetBrains Qodana
- **Uses**: `make deps` for setup

### Auto-Rebase (`.github/workflows/auto-rebase.yml`)
- **Triggers**: Push to main
- **Purpose**: Automatically rebases all open PRs targeting main
- **Uses**: `peter-evans/rebase@v4`
- **Excludes**: Dependabot PRs (`dependencies` label), draft PRs
- **Auth**: Requires `AUTO_REBASE_PAT` secret (PAT needed to trigger CI on rebased branches)

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

## Dependabot Management

### Handling PRs

Use `@dependabot` commands in PR comments (not `gh pr close`):

| Command | Effect |
|---------|--------|
| `@dependabot close` | Close PR, prevent recreation |
| `@dependabot ignore this dependency` | Close PR, ignore dependency permanently |
| `@dependabot ignore this major version` | Ignore major version updates |
| `@dependabot ignore this minor version` | Ignore minor version updates |
| `@dependabot rebase` | Rebase the PR |
| `@dependabot recreate` | Recreate the PR from scratch |

### Deferring Updates

When an update requires a blocked dependency (e.g., newer Go version):
1. Comment `@dependabot close` with explanation and link to blocking issue
2. Do not use `gh pr close` directly - Dependabot will recreate the PR

Reference: [GitHub Docs - Dependabot PR Commands](https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-pull-request-comment-commands)

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

### Building
- `build` - Build all executables
- `build-kure` - Build kure executable
- `build-kurel` - Build kurel executable
- `build-demo` - Build demo executable
- `build-race` - Build with race detection

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
- `release-snapshot` - Test GoReleaser locally (no tag, no publish)

### Utilities
- `generate` - Run go generate
- `mod-graph` - Display module dependency graph
- `list-packages` - List all packages
- `demo*` - Various demo commands

## Environment Variables

Key environment variables the Makefile respects:

- `GO` - Go command (default: `go`)
- `GOROOT` - Go root directory
- `VERSION` - Version string for builds
- `BUILD_DIR` - Build output directory (default: `bin`)
- `OUTPUT_DIR` - Demo output directory (default: `out`)
- `TEST_TIMEOUT` - Test timeout (default: `30s`)
- `PACKAGE_PATH` - Package path for kurel operations

## Development Tips

### Running Demos
The demo system generates example YAML files showing Kure's capabilities:

```bash
# Run all demos
make demo

# Generated files appear in out/ directory
ls -la out/
```

### Testing Strategy
- Use `make test-short` for quick feedback during development
- Use `make test-coverage` to check coverage before PRs
- Use `make test-race` to catch concurrency issues
- Use `make check` for quick pre-commit validation

### Code Quality
- The CI pipeline enforces 80% test coverage
- All code must pass golangci-lint checks
- Code must be properly formatted with `go fmt`
- Modules must be tidy

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
3. **CLI reference** — Regenerated automatically by `make docs-cli` (no manual updates needed)

To verify the docs site builds correctly:

```bash
# Check all mounted files exist
bash site/scripts/check-mounts.sh

# Generate CLI reference + build site
mise run site:build
```

## Crane Integration

Kure is a dependency of the Crane project (`~/src/autops/wharf/crane`).

### Relationship

- **Crane** transforms OAM → Kure domain model → Kubernetes manifests
- **Kure** provides the domain model and manifest generation engine
- Both repos are **co-developed** with local replace directives

### Key Files

- Crane's requirements: `~/src/autops/wharf/crane/PLAN.md`
- Crane's agent guide: `~/src/autops/wharf/crane/AGENTS.md`

### When Making Changes

1. Check if change affects Crane's integration
2. Keep public API (`pkg/stack/`) stable when possible
3. Update Crane if breaking changes are necessary
4. Test with `go mod tidy` in Crane to verify compatibility

### Go Workspaces

Crane uses Go workspaces for local development. The workspace file lives in the parent directory:

```bash
# From wharf/ directory
go work init
go work use ./crane ./kure
```

This allows Crane to use your local Kure changes without pushing.

**Before pushing Kure changes that Crane depends on:**
1. Push Kure changes first
2. In Crane: `GOWORK=off go get github.com/go-kure/kure@main`
3. Commit the updated go.mod/go.sum in Crane