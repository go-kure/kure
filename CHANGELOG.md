## v0.1.0-alpha.1 - 2026-01-30

- Summary: (fill in)

## v0.1.0-alpha.0 - 2026-01-30

- Summary: (fill in)

# Changelog

All notable changes to this project will be documented in this file.
## [Unreleased]

### Added

- Add storageclass helpers
- Add kustomize helpers
- Add flux source helpers
- Add helpers
- Add fluxcd builders package
- Add layout grouping and app file mode
- Support file- and dir-per-application layouts
- Implement OCI artifact separation in layout system
- Implement GitOps bootstrap and refactor demo system to data-driven architecture
- Implement comprehensive Kubernetes printer wrappers in io module
- Modernize error handling with custom error types and standardization
- Implement professional Cobra CLI with comprehensive command structure
- Implement comprehensive structured error handling system
- Add shorthand flags for common CLI options across all commands
- Complete kurel package system design documentation
- Implement package loader with hybrid error handling
- Implement variable resolver with cycle detection
- Implement patch processor with dependency resolution
- Implement schema generation and validation for launcher
- Complete Phase 4 - schema generation and validation
- Complete Phase 5 - output builder and local extensions
- Implement Phase 6 - CLI command integration
- Implement Phase 7 - comprehensive integration tests
- Implement GVK-based ApplicationConfig generator system
- Implement GVK-based versioning for stack module structs
- Implement GVK-based versioning for stack module structs
- Add comprehensive Makefile and CI/CD pipeline
- Complete KurelPackage generator implementation
- Enable Kubernetes schema inclusion in kurel CLI
- Implement fluent builder pattern Phase 1
- Implement comprehensive interval validation for GitOps configurations
- Add Go version management tools
- Add fast precommit target for git hooks
- Add PodDisruptionBudget builder
- Add HorizontalPodAutoscaler builder
- Add combined-output mode to kure patch
- Add --diff option to kure patch

### Build

- Update Go to 1.24.12 to fix govulncheck vulnerabilities

### CI

- Add GitHub Action to refresh Go proxy on main branch commits
- Enforce Go version consistency in PR checks
- Remove Qodana workflow due to licensing issues
- Fix security scan action to use official gosec action
- Remove gosec security scan (CodeQL provides coverage)

### Changed

- Loop over YAML prints
- Split appsets module
- Export ApplyPatch
- Register k8s schemes on demand
- Move pkg/layout to pkg/stack/layout for better organization
- Move pkg/fluxcd to pkg/k8s/fluxcd for better organization
- Yaml dir naming and proper marshalling
- Modernize errors package to follow Go best practices
- Modernize patch module with clean syntax and comprehensive tooling
- Rename cmd/patch to cmd/kure for better CLI naming
- Promote patch command from subcommand to top-level command
- Rename .patch files to .kpatch to avoid conflicts with diff patches
- Eliminate circular references in Node and Bundle structures
- Centralize validation logic across Kubernetes builders
- Standardize error handling to use KureError consistently
- Standardize function naming conventions across codebase
- Multi-CLI architecture and package naming standardization
- Implement clean workflow interface architecture
- Implement launcher base types with shared libraries
- Implement shared internal/gvk infrastructure
- Apply go fmt formatting to codebase
- Simplify Claude settings with symlink and expanded permissions
- Reorganize task files with numbered prefixes
- Migrate to GoReleaser v2 workflow
- Consolidate Makefile targets and enhance dev workflow
- Standardize validation patterns across packages
- Consolidate 4 GitHub workflows into 2 (ci.yml + release.yml)
- Consolidate 4 GitHub workflows into 2 (ci.yml + release.yml)
- Improve pkg/kubernetes testability and coverage

### Dependencies

- Align k8s.io/cli-runtime to v0.33.2 to match replace directive
- Bump tj-actions/changed-files
- Bump github.com/external-secrets/external-secrets
- Bump sigs.k8s.io/kustomize/api from 0.20.0 to 0.21.0
- Bump sigs.k8s.io/yaml from 1.5.0 to 1.6.0
- Implement centralized dependency version management
- Document blocked dependency updates for Go 1.25
- Bump github.com/spf13/cobra in the go-safe group
- Bump github.com/cert-manager/cert-manager
- Update versions.yaml for cert-manager 1.16.5

### Documentation

- Add project README
- Mention base resources and expose constructor
- Expand kio package documentation
- Expand kio documentation
- Expand fluxcd package overview
- Correct Flux auto-generated kustomization details
- Update README to reflect current repository state
- Add comprehensive architectural documentation
- Add comprehensive architectural documentation for generators
- Add comprehensive UX design document and recommendations
- Update project status and document remaining features
- Add comprehensive plugin architecture design
- Update CLAUDE.md with current project priorities and status
- Update CLAUDE.md with current project status and accurate metrics
- Update user documentation with current project state
- Add detailed explanation of CEL Validation Enhancement task
- Add comprehensive repository review and task management system
- Update task statuses after upstream rebase
- Add comprehensive puzl-cloud/kubesdk review with kure comparison
- Add task #1 for CEL validation enhancement
- Add workflow guidelines to tasks.md
- Remove references to non-existent demo-internals make target
- Add HPA and PDB builder tasks for Crane OAM support
- Add Crane integration documentation
- Add tasks README and update task 03 status
- Add quickstart guide
- Expand README with end-to-end examples
- Mark high-priority tasks 1-5, 23, 24 as completed
- Mark task #8 as completed
- Add comprehensive GoDoc documentation
- Mark task #10 as completed
- Mark task #6 as completed
- Mark tasks #7, #9, #11, #12 as completed

### Fixed

- Separate helper group comments
- Add missing unstructured import to patch CLI
- Correct type usage in generators package tests
- Resolve all layout module test failures
- Ensure all manifest directories have kustomization.yaml for GitOps compliance
- Resolve test failures in launcher module
- Resolve CLI test output capture issues
- Resolve all failing tests and improve TOML patch support
- Correct appworkload test to match ServiceConfig structure
- Update demo and kure commands to use new GVK-based ApplicationWrapper
- Resolve intermittent test failures in cmd/demo package
- Resolve stdout capture synchronization in demo tests
- Configure golangci-lint compatibility and resolve linting issues
- Correct YAML structure in CI workflow
- Add goimports to make fmt for CI/local parity
- Add GOPATH/bin to PATH in lint and fmt targets
- Upgrade Go to 1.24.11 to resolve security vulnerabilities
- Make max_depth_exceeded test deterministic
- Fix CVE in mapstructure and add workflow permissions
- Resolve repo issues across docs, CI, validation, and caching
- Propagate --strict flag to validator in kurel validate
- Update K8s compatibility matrix to test supported versions
- Remove K8s 0.33 from CI compatibility matrix
- Align mise.toml Go version with CI workflows
- Lower coverage threshold to 70% to match current main coverage
- Improve dependabot wildcard pattern matching in validation
- Block FluxCD major version updates in dependabot

### Testing

- Check errors
- Add runCluster coverage
- Add comprehensive test coverage for all packages
- Add comprehensive test coverage for FluxHelm internal package
- Skip demo integration tests in short mode
- Skip demo tests when examples directory is missing
- Fix data race in TestMainFunction
- Skip max_depth_exceeded test due to resolver bugs
- Add integration tests for stack generation workflows
- Add fuzz tests for patch parser
- Add Kubernetes version matrix to CI
- Add tests to improve coverage and fix Go version
- Add Phase 1 coverage for simple getters/setters
- Add Phase 2 parsing tests, reach 70.5% coverage
- Add Phase 3 validation tests, reach 100% validation coverage
- Add Phase 4 stack domain model tests
- Add Phase 5 layout integrator tests
- Add wrapper function tests, reach 94.8% gvk coverage
- Add setter function tests for internal packages
- Add comprehensive IO table and printer tests
- Add comprehensive appworkload internal tests


