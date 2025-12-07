# GitHub Workflows Documentation

This document provides an overview of all GitHub Actions workflows used in the kure project.

**Last Updated:** 2025-12-07

---

## Workflow Summary

| Workflow | File | Triggers | Purpose |
|----------|------|----------|---------|
| [CI/CD Pipeline](#cicd-pipeline) | `ci.yml` | push, PR, manual | Comprehensive testing, linting, building, security |
| [Build and Test](#build-and-test) | `build-test.yaml` | push (main), PR | Basic build and test with formatted output |
| [PR Checks](#pr-checks) | `pr-checks.yml` | PR events | Comprehensive PR validation and analysis |
| [Release](#release) | `release.yml` | version tags | GoReleaser-based release with validation |

---

## Workflow Overview

### Test Jobs in CI

| Job | Matrix | Command | Uses Makefile? |
|-----|--------|---------|----------------|
| `test` | `unit` | `make test` | ✅ |
| `test` | `race` | `make test-race` | ✅ |
| `test` | `coverage` | `make test-coverage` | ✅ |
| `integration` | - | `make test-integration` | ✅ |

### Test Targets in Makefile

| Target | Command | Used in CI? | In precommit? |
|--------|---------|-------------|---------------|
| `test` | `go test -timeout 30s ./...` | ✅ | ✅ |
| `test-race` | `go test -race -timeout 30s ./...` | ✅ | ❌ |
| `test-coverage` | `go test -coverprofile=... ./...` | ✅ | ❌ |
| `test-integration` | `go test -tags=integration -timeout 5m ./...` | ✅ | ❌ |
| `vuln` | `govulncheck ./...` | ✅ | ❌ |

### CI vs Pre-commit

| Target | Tasks | Use Case |
|--------|-------|----------|
| `precommit` | fmt, tidy, lint, test | Fast local checks (~10s) |
| `ci` | deps, fmt, tidy, lint, vet, test, test-race, test-coverage, test-integration, build, vuln | Comprehensive CI pipeline (~2min) |

---

## Workflow Details

### CI/CD Pipeline

**File:** `.github/workflows/ci.yml`
**Name:** `CI/CD Pipeline`

**Triggers:**
- Push to: `main`, `develop`, `release/*`
- Pull requests to: `main`, `develop`
- Manual dispatch

**Jobs:**
1. **Test (unit)** - Unit tests with race detection
2. **Test (race)** - Full race condition detection tests
3. **Test (coverage)** - Code coverage analysis
4. **Lint** - Code quality checks with golangci-lint
5. **Build** - Build all binaries
6. **Demo** - Run demo executable
7. **Integration** - Integration tests
8. **Build Matrix** - Cross-platform builds (Linux, macOS, Windows)
9. **Security** - Security vulnerability scanning
10. **Dependency Check** - Check for outdated dependencies

**Configuration:**
- Go Version: `1.24.11`
- Golangci-lint Version: `v1.62.2`
- Platforms: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`

---

### Build and Test

**File:** `.github/workflows/build-test.yaml`
**Name:** `Build and Test`

**Triggers:**
- Push to: `main`
- Pull requests to: `main`

**Jobs:**
1. **Build** - Runs tests with gotestfmt formatter, builds project, runs demo

**Configuration:**
- Go Version: `1.24.11`
- Uses gotestfmt for formatted test output
- Uploads test logs as artifacts

**Purpose:** Provides a quick, formatted test output for main branch changes.

---

### PR Checks

**File:** `.github/workflows/pr-checks.yml`
**Name:** `PR Checks`

**Triggers:**
- Pull requests to: `main`, `develop`
- Events: `opened`, `synchronize`, `reopened`, `ready_for_review`

**Concurrency:** Cancels previous runs for the same PR

**Jobs:**
1. **Quick Check** - Fast validation (Go version consistency, format, lint, vet)
2. **Security Check** - Vulnerability scanning and dependency checks
3. **Coverage Check** - Test coverage validation (80% threshold)
4. **Analyze Changes** - Changed files analysis and impact assessment
5. **Performance Check** - Benchmarks (only if `performance` label present)
6. **Docs Check** - Documentation validation

**Configuration:**
- Go Version: `1.24.11`
- Coverage Threshold: `80%`

---

### Release

**File:** `.github/workflows/release.yml`
**Name:** `Release`

**Triggers:**
- Push tags: `v*` (e.g., v1.0.0, v0.1.0-alpha.0)

**Jobs:**
1. **Validate** - Strict tag format, changelog, and version progression validation
2. **GoReleaser** - Cross-platform builds using GoReleaser v2
3. **Post-release** - Go proxy refresh

**Configuration:**
- Go Version: `1.24.11`
- Build Tool: GoReleaser v2
- Platforms: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`, `windows/arm64`
- Tag Format: `^v[0-9]+\.[0-9]+\.[0-9]+(-alpha\.[0-9]+|-beta\.[0-9]+|-rc\.[0-9]+)?$`
- Changelog: Required (must have `## v0.1.0` section)

**Local Release Management:**
```bash
# Preview release plan
make release TYPE=alpha

# Execute release (creates commits + tag)
make release-do TYPE=alpha

# Push tag to trigger CI
git push origin v0.1.0-alpha.0
```

---

## Configuration Standards

### Go Version

All workflows use Go **1.24.6** consistently, defined via environment variable:

```yaml
env:
  GO_VERSION: '1.24.6'
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

## Maintenance Notes

- **When adding/modifying workflows:** Update this document with changes
- **Version updates:** Ensure Go version consistency across all workflows
- **Action versions:** Keep GitHub Actions up to date (currently using v4-v5)

---

## See Also

- [Makefile](../Makefile) - Local development commands
- [CLAUDE.md](../CLAUDE.md) - Development guidelines
- [mise.toml](../mise.toml) - Local tool version management
