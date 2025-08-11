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

# Run demo with internal API examples
make demo-internals

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
- `release-check` - Check if ready for release
- `release-build` - Build release artifacts for multiple platforms

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

# Just internal API examples
make demo-internals

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