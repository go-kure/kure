# Changelog

All notable changes to this project will be documented in this file.
## [0.2.0-alpha.2] - 2026-05-05

### Added

- Add domain config types for Cluster, Database, and ObjectStore

### Fixed

- Use kure errors package instead of fmt.Errorf

## [0.2.0-alpha.1] - 2026-05-04

### Added

- Add configMapGenerator support to ManifestLayout
- Add ReplicationSource and ReplicationDestination builders
- Add LayoutRules.FlattenSingleTier opt-in flag

### CI

- Fix build-binaries timeout and remove make dependency
- Remove make dependency from docs-build job
- Skip apt-get in test job when build-essential already installed

### Dependencies

- Bump FluxCD ecosystem to v2.8.6 and cnpg/barman-cloud to 0.5.1
- Bump sigs.k8s.io/controller-runtime from 0.23.3 to 0.24.0
- Pin k8s.io to v0.36.0 to match controller-runtime v0.24.0
- Update versions.yaml for controller-runtime 0.24.0 and k8s 0.36.0

### Documentation

- Fix stale references to deleted stack/v1alpha1 and generator aliases
- Fix remaining stale references after stack/v1alpha1 removal

### Fixed

- Use kustomize.config.k8s.io domain in generated kustomization.yaml

### Release

- V0.2.0-alpha.1

## [0.2.0-alpha.0] - 2026-05-01

### Added

- Move kure docs site to /kure/ subpath

### Build

- Remove kurel and extracted packages from build and docs

### CI

- Trigger Claude Code action automatically on PRs
- Replace shared workflows with callers to go-kure/.github
- Optimize pipeline — parallel jobs, single test run, Hugo cache, path filtering
- Cancel in-progress runs when new push arrives on same branch

### Fixed

- Handle large PR diffs in review workflow, fix formatting, update docs
- Set skipped output before early exits in pr-review workflow
- Add test to build gate needs to catch skipped cascade on failure
- Guard against empty Hugo version parse from mise.toml
- Add validate to build gate needs so lint failure blocks merge
- Sync hugo.toml mounts and check-mounts after package extraction
- Guard against empty Hugo version read
- Repair 5 broken links on deployed docs site
- Support unnamed root node — merge resources at cluster root

### Testing

- Add missing tests for Patches/PostBuild and stable helm output
- Add OCI layout pattern tests (Namespace:"."/layer naming/3-layer structure)
- Assert spec.path and sourceRef in Layer 2 Kustomization tests

### Release

- V0.2.0-alpha.0

## [0.1.0-rc.11] - 2026-04-20

### Added

- Add public pkg/kubernetes/cnpg wrapper

### Dependencies

- Bump github.com/moby/spdystream from 0.5.0 to 0.5.1
- Bump github.com/cloudnative-pg/machinery
- Bump github.com/cloudnative-pg/plugin-barman-cloud

### Documentation

- Add internal design note for launcher extraction
- Correct patch.go treatment in launcher extraction design
- Document OCI folder layout and split strategy design
- Rename infra to platform in OCI layout design doc
- Register oci-layout.md in docs site scripts

### Release

- V0.1.0-rc.11

## [0.1.0-rc.10] - 2026-04-15

### Added

- Add builders for Role, RoleBinding, ClusterRole, ClusterRoleBinding

### Release

- V0.1.0-rc.10

## [0.1.0-rc.9] - 2026-04-15

### Added

- Add Patches and PostBuild fields to Bundle
- Add Force and Suspend fields to Bundle

### Dependencies

- Bump github.com/cert-manager/cert-manager

### Release

- V0.1.0-rc.9

## [0.1.0-rc.8] - 2026-04-14

### Added

- Add RenderChart for client-side OCI chart rendering

### Dependencies

- Bump github.com/google/cel-go from 0.27.0 to 0.28.0

### Documentation

- Document undocumented features across package READMEs and guides
- Add prometheus-builders API reference page and fix broken links

### Fixed

- Sort manifest keys for stable output

### Release

- V0.1.0-rc.8

## [0.1.0-rc.7] - 2026-04-12

### Fixed

- Honor FileNaming in WriteToDisk, WriteManifest, and package walker

### Release

- V0.1.0-rc.7

## [0.1.0-rc.6] - 2026-04-12

### Added

- Propagate FileNaming to ManifestLayout and WriteToTar

### Fixed

- Force FilePerResource for FluxIntegrated kustomization refs

### Release

- V0.1.0-rc.6

## [0.1.0-rc.5] - 2026-04-11

### Added

- Emit full Flux Operator install bundle in flux-operator mode

### Release

- V0.1.0-rc.5

## [0.1.0-rc.4] - 2026-04-10

### Added

- Add Bundle.Children + shared cluster validator
- Wire ValidateCluster into all entry points
- Umbrella Kustomization spec generation
- Umbrella layout walker + integrated placement + v1alpha1 parity
- Add umbrella cluster demo and fix writer CR duplication

### Dependencies

- Bump github.com/fluxcd/flux2/v2 from 2.8.2 to 2.8.3
- Bump github.com/cert-manager/cert-manager
- Bump github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring
- Bump github.com/cloudnative-pg/cloudnative-pg
- Bump github.com/fluxcd/flux2/v2 from 2.8.2 to 2.8.3

### Fixed

- Bump Go to 1.26.2 for stdlib security fixes
- Switch to claude-max-proxy
- Nest child nodes under root node layout when ClusterName is set

### Testing

- Move gotk network tests to integration, use flux-operator in workflow test

### Release

- V0.1.0-rc.4

## [0.1.0-rc.3] - 2026-03-22

### Added

- Upgrade cert-manager v1.19.4 → v1.20.0
- Add PSAViolationError with field paths

### Dependencies

- Bundle dependency updates

### Documentation

- Update lint job timeout from 10 to 15 minutes

### Fixed

- Map dependency-updates.md to docs site
- Increase lint job timeout to 15 minutes
- Resolve broken links on versioned doc subsites
- Disable setup-go built-in cache to prevent double-caching
- Resolve broken dependency-updates link on contributing guide page

### Release

- V0.1.0-rc.3

## [0.1.0-rc.2] - 2026-03-20

### Added

- Dynamic version notice on homepage

### CI

- Run all GitHub Actions on self-hosted runner

### Changed

- Reorganize examples/ with demo/ grouping and READMEs

### Documentation

- Update CI docs and changelog for isDeepEmpty fix

### Fixed

- Patch pipeline bugs and update demo examples to AppWorkload format
- Correct containerport typo to containerPort in example YAMLs
- Pin yq version and cache Hugo modules in CI
- Add fallback for COMMIT_SHA in gen-versions-toml.sh
- Install missing tools on self-hosted runner in CI workflows
- Run apt-get update before installing make in CI workflows
- Install gcc and enable CGO for race tests on self-hosted runner
- Remove gotestfmt from unit test step to fix self-hosted runner
- Install build-essential to provide C headers for CGO
- Strip zero-value primitives in isDeepEmpty
- Add nil and []any handling to isDeepEmpty
- Install goimports before formatting check
- Increase lint timeout and scope cache to modules only
- Gate release on pre-release tests and fix CGO_ENABLED

### Release

- V0.1.0-rc.1
- V0.1.0-rc.2

## [0.1.0-rc.0] - 2026-03-09

### Changed

- Simplify Bundle.Generate() label propagation
- Standardize CRD builders to void returns
- Standardize ConfigMap and Secret builders to void returns
- Standardize validation strategy with void returns

### Dependencies

- Bump github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring
- Bump sigs.k8s.io/gateway-api from 1.4.0 to 1.5.0

### Fixed

- Simplify SetSecretImmutable and remove unnecessary Immutable pre-allocation

### Testing

- Add golden file tests for InitContainer builders

### Release

- V0.1.0-rc.0

## [0.1.0-beta.7] - 2026-03-08

### Added

- Add public facade package
- Add public facade package
- Add public facade package
- Support flat root output with NodeGrouping=GroupFlat (#240)
- Add configurable layout presets (#263)
- Add NetworkPolicy and HTTPRoute builders (#242)
- Add PSA security context helpers (#243)
- Add Prometheus operator builders (#354)
- Add ResourceRequirements builder (#244)
- Add {kind}-{name}.yaml file naming pattern (#266)
- Set FileNamingKindName in centralized preset LayoutRules
- Add SourceKind field to BootstrapConfig (#254)
- Implement regex pattern validation in schema
- Add configurable kustomization mode per FluxPlacement (#265)
- Add Flux 2.8 remediation and wait strategy builders (#255)
- Promote flux-operator to primary bootstrap mode (#256)
- Add remediation config to ReleaseConfig (#236)

### Documentation

- Add package README
- Add metallb to AGENTS.md reverse mapping table
- Fix version mismatches and outdated references across documentation
- Add godoc comments to namespace builder functions
- Document Provider/Alert v1beta3 blocked state (#250)
- Document k8s.io replace directives in go.mod (#291)

### Fixed

- Add missing types.go, doc.go, tests and SetClusterIssuerCA
- Skip empty ACME solvers, make ACME and CA mutually exclusive
- Append GOPATH/bin to PATH instead of prepending in Makefile
- Use predefined nil error constants in metallb builders
- Use pkg/errors instead of fmt.Errorf in patch package
- Check FluxPlacement in WriteToDisk and WriteToTar (#264)
- Centralize error sentinels and add documentation
- Avoid slice mutation and validate ephemeral containers in PSA
- Use centralized sentinel errors and add missing Prometheus helpers
- Remove trailing blank line in bootstrap generator test

### Testing

- Add comprehensive tests for public facade
- Add tests and docs for externalsecrets facade

### Release

- V0.1.0-beta.7

## [0.1.0-beta.6] - 2026-03-06

### Added

- Add strategic merge patch support with namespace-aware target resolution
- Add HelmRelease targetNamespace and releaseName fields
- Expose valuesFrom in FluxHelm ReleaseConfig
- Add PostRenderer Kustomize builder helpers
- Add CRD lifecycle policy fields to ReleaseConfig
- Add chartRef support to FluxHelm generator (#267)
- Allow SourceRefName override in FluxHelm generator
- Support prune protection annotation on generated resources
- Add DriftDetection builder helpers for HelmRelease
- Add HealthChecks field to Bundle
- Add mise release task with dry-run trigger wrapper
- Promote internal/gvk to pkg/gvk
- Promote 7 internal K8s builders to public API
- Add CNPG Database CR builder
- Add CNPG ObjectStore CR builder
- Add CNPG ScheduledBackup plugin method and advanced knobs
- Enable gosec linter with fixes for all violations
- Add CNPG managed roles builder

### Build

- Add gen-versions-toml.sh for Hugo config overlay
- Use versioned config overlay in docs-build
- Rework deploy-docs for multi-version deployment
- Trigger versioned docs deployment on stable release
- Add manage-docs admin workflow
- Migrate golangci-lint from v1 to v2
- Upgrade to Go 1.26.0
- Remove accidentally committed temp file
- Upgrade k8s.io dependencies to v0.35.1 (Kubernetes 1.35)
- Update versions.yaml for k8s 1.35 upgrade
- Remove broken k8s-compat CI job
- Upgrade cert-manager to v1.19.4 and metallb to v0.15.3
- Upgrade external-secrets to v1.3.2 (module path migration)

### Changed

- Replace any with typed parameters in Workflow interface

### Dependencies

- Upgrade FluxCD ecosystem to 2.8
- Bump sigs.k8s.io/controller-runtime

### Documentation

- Update branch protection docs to reflect ruleset migration
- Add version-aware banner and header display
- Add pkg.go.dev reference links to package READMEs
- Document versioned docs system in github-workflows.md
- Add 2026-02-26 deep code review
- Add action plan, issue specs, and implementation design
- Clarify AGENTS.md fmt.Errorf guidance
- Add example_test.go for CRD builder packages
- Document deepCopyBundle shallow copy behavior
- Document Cluster getter/setter duality
- Add getting-started example for Cluster-to-Disk pipeline

### Fixed

- Add release notes extraction to release workflow
- Use tab-indented code block for YAML example in doc comment
- Guard unstructured fallback from list decode panics
- Release read lock before invoking converter callback
- Fix CI lint baseline and resolve pre-existing lint issues
- Resolve 30+ broken links on gokure.dev
- Use json.Marshal for HelmRelease values encoding
- Exclude docs/development/ from unmapped docs check
- Replace gh CLI with curl in pr-review workflow
- Address AI review findings on pr-review workflow
- Enforce ChartRef validation and mutual exclusivity
- Check remote tags before releasing
- Use pkg/errors and pkg/logger in getting-started example
- Use ToClientObject helper instead of manual pointer-to-interface
- Isolate TestEnsureConfigDir from host environment
- Harden release-trigger.sh remote detection and hint output
- Remove curl|sh auto-install from lint-fast target
- Strip v prefix from pseudo-version in versions.yaml
- Add deprecation markers and update DESIGN.md references
- Add timeline notification comments for PR review updates
- Replace gh CLI with curl for timeline notice comments
- Add Content-Type header to timeline notice API calls
- Replace sticky comments with regular PR comments
- Sync controller-runtime version to 0.23.3 in versions.yaml
- Align pr-review workflow with GitLab mr-review template
- Correct yq pipe precedence in max_dependabot filter
- Migrate cosign signing to v3 bundle format
- Bump Go to 1.26.1 (security patch)
- Remove output flag and use sigstore.json extension for cosign v3
- Add release-notes.md to .gitignore

### Performance

- Add lint-fast Makefile target

### Testing

- Improve test coverage from 78% to 88%

### Pr-review

- Fix broken pipe and stale assessment comment

### Release

- V0.1.0-beta.1
- V0.1.0-beta.2
- V0.1.0-beta.3
- V0.1.0-beta.4
- V0.1.0-beta.5
- V0.1.0-beta.6

## [0.1.0-beta.0] - 2026-02-17

### Added

- Expose HPA helpers in pkg/kubernetes
- Expose PDB helpers in pkg/kubernetes
- Add deterministic YAML serialization option
- Expose Deployment, Service, Ingress helpers in pkg/kubernetes
- Expose CronJob helpers in pkg/kubernetes
- Add optional Validator interface for ApplicationConfig
- Add unstructured fallback for unknown GVKs
- Implement Generate() for stack pipeline integration
- Add comprehensive server-set field stripping (#196)
- Add kure init scaffolding command (#136)
- Rewrite fluent builders with immutable copy semantics (#139)
- Migrate release automation from semver.sh to CI-driven release.sh

### Changed

- Consolidate generator registries into pkg/stack (#179)

### Dependencies

- Bump sigs.k8s.io/kustomize/api in the k8s-ecosystem group

### Documentation

- Add implementation workflow checklist
- Document ApplicationConfig breaking change (#178)

### Release

- V0.1.0-alpha.4
- V0.1.0-beta.0

## [0.1.0-alpha.3] - 2026-02-12

### Documentation

- Add changelog entry for v0.1.0-alpha.3

### Fixed

- Install syft in release workflow for SBOM generation

### Release

- V0.1.0-alpha.3

## [0.1.0-alpha.2] - 2026-02-12

### Added

- Deterministic kustomization.yaml ordering
- Add missing Bundle fields (Prune, Wait, Timeout, etc.)
- Clean YAML output in EncodeObjectsToYAML by default
- Implement createSource() for OCIRepository/GitRepository
- Add WriteToTar(io.Writer) for in-memory layout generation
- Propagate Bundle.Labels to all generated resources
- Rename CI job names to match branch protection check names
- Add Hugo documentation site with CI/CD and mise tasks
- Add auto-rebase workflow and rebase-check job

### Build

- Improve release workflow security and reproducibility

### CI

- Add GitLab mirror push after all checks pass
- Add divergence detection and tag sync to GitLab mirror

### Dependencies

- Bump github.com/google/cel-go from 0.26.1 to 0.27.0

### Documentation

- Archive completed PLAN.md to docs/history/
- Streamline README as landing page with badges
- Use shields.io badge for Go Report Card
- Restructure site around user needs with code-synced READMEs

### Fixed

- Use git-cliff for changelog generation in release script
- Bump Go 1.24.12 → 1.24.13, add govulncheck summary to CI
- Use path-based matching in findLayoutNode()
- Anchor GO_VERSION patterns to avoid matching HUGO_VERSION
- Add rollup build gate job to satisfy branch protection check

### Release

- V0.1.0-alpha.2

## [0.1.0-alpha.1] - 2026-01-30

### Fixed

- Run tests directly in release workflow instead of checking CI status

## [0.1.0-alpha.0] - 2026-01-30

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
- Automate changelog generation with git-cliff

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

### Release

- V0.1.0-alpha.0


