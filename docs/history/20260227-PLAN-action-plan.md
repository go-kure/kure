# Kure — Action Plan

> **Generated**: 2026-02-27
> **Source**: `20260227-HISTORY-deep-code-review.md`, `wharf-fleet-status-report.md`

## Summary

This action plan captures **42 actionable items** distilled from the kure deep code review (2026-02-26), the wharf fleet status report (2026-02-26), and 51 existing open GitHub issues. The items span 8 categories: 1 bug fix, 14 features, 9 chores (including dependency upgrades), 6 refactors, 1 testing item, 1 CI item, 5 documentation items, and 1 security item. Four items are rated critical, twelve high, nineteen medium, and seven low priority.

The dominant themes are: **(1) lint strictness** — the deep review's top recommendation is enabling disabled linters, which will flush out latent bugs; **(2) public API promotion** — kure has mature internal builders that crane needs exposed; **(3) FluxCD API migration** — three Flux CRDs need v1beta2-to-v1 migration before v1beta2 scheme registrations can be removed; **(4) CNPG expansion** — four new CNPG builders are needed for crane's database component support; and **(5) HelmRelease completeness** — eight HelmRelease enhancements are required for crane's Helm-based deployment support. Cross-repo coordination is required for API promotion (#241), domain naming standardization, and the v1alpha1 graduation path.

---

## Action Items

### Bug: Fix FluxPlacement no-op — LayoutRules.FluxPlacement not checked by write functions

- **Type**: bug
- **Fleet Phase**: 2
- **Priority**: high
- **Effort**: low
- **Dependencies**: none
- **Cross-repo**: crane will benefit immediately; no crane changes needed
- **Existing issues**: #264
- **Description**: The `LayoutRules.FluxPlacement` field is set by callers but never checked by the layout write functions, making it a silent no-op. The write path must inspect FluxPlacement and adjust file placement or kustomization.yaml generation accordingly. This is the only open bug in the kure issue tracker.
- **Acceptance criteria**:
  - [ ] Write functions check `LayoutRules.FluxPlacement` and behave differently based on its value
  - [ ] Unit tests cover all FluxPlacement modes
  - [ ] Existing layout tests continue to pass

---

### Refactor: Enable `errcheck` linter and fix violations

- **Type**: refactor
- **Fleet Phase**: 3
- **Priority**: high
- **Effort**: high
- **Dependencies**: none
- **Cross-repo**: meta standards should document kure's lint baseline once aligned; crane already enables errcheck
- **Existing issues**: none
- **Description**: The deep review's top recommendation. `errcheck` is the most impactful Go linter — it catches unhandled errors that become silent failures. The existing lint config disables it with the note "Too many unchecked errors in existing code." Enable the linter, fix all violations across the codebase, and commit as a single dedicated PR. This is the highest-effort lint item due to the volume of violations implied by the disable comment.
- **Acceptance criteria**:
  - [ ] `errcheck` enabled in `.golangci.yml`
  - [ ] Zero lint violations from `errcheck`
  - [ ] No functional regressions (all tests pass)
  - [ ] PR contains only errcheck fixes (no mixed changes)

### Refactor: Enable `ineffassign` linter and fix violations

- **Type**: refactor
- **Fleet Phase**: 3
- **Priority**: high
- **Effort**: low
- **Dependencies**: none
- **Cross-repo**: none
- **Existing issues**: none
- **Description**: `ineffassign` detects assignments to variables that are immediately overwritten — these are often real bugs. The review flags this as "should be enabled." Enable the linter and fix any violations. Expect low violation count since this catches logical errors that are typically few.
- **Acceptance criteria**:
  - [ ] `ineffassign` enabled in `.golangci.yml`
  - [ ] Zero lint violations
  - [ ] All tests pass

### Refactor: Enable `unused` linter and remove dead code

- **Type**: refactor
- **Fleet Phase**: 3
- **Priority**: medium
- **Effort**: medium
- **Dependencies**: none
- **Cross-repo**: none
- **Existing issues**: none
- **Description**: The lint config disables `unused` with the note "Many utility functions are kept for future use." The review correctly observes that dead code accumulates and should be removed — it can be re-added from git history when needed. Enable the linter, remove dead functions, types, and variables. Some utility functions may be genuinely needed for the public API surface; those should be annotated or covered by tests.
- **Acceptance criteria**:
  - [ ] `unused` enabled in `.golangci.yml`
  - [ ] Dead code removed or justified with tests
  - [ ] All tests pass

### Refactor: Enable `gosimple` linter and simplify flagged code

- **Type**: refactor
- **Fleet Phase**: 3
- **Priority**: low
- **Effort**: low
- **Dependencies**: none
- **Cross-repo**: none
- **Existing issues**: none
- **Description**: The lint config disables `gosimple` with "Some simplifications reduce readability." Enable the linter and apply simplifications where they genuinely improve clarity. Use `//nolint:gosimple` with justification comments for cases where the expanded form is more readable.
- **Acceptance criteria**:
  - [ ] `gosimple` enabled in `.golangci.yml`
  - [ ] Justified `//nolint` comments where simplification hurts readability
  - [ ] All tests pass

### Refactor: Align golangci-lint config with crane's expanded linter set

- **Type**: refactor
- **Fleet Phase**: 3
- **Priority**: medium
- **Effort**: medium
- **Dependencies**: errcheck, ineffassign, unused, gosimple items above
- **Cross-repo**: crane's `.golangci.yml` is the reference; meta should document the fleet lint baseline
- **Existing issues**: none
- **Description**: After enabling the four core linters above, incrementally enable the additional linters that crane uses: `bodyclose`, `durationcheck`, `errorlint`, `exhaustive`, `misspell`, `nilerr`, and `whitespace`. Enable them one at a time, fixing violations in dedicated PRs to keep diffs reviewable. This achieves lint parity across the wharf fleet's Go repos.
- **Acceptance criteria**:
  - [ ] All linters from crane's config enabled in kure
  - [ ] Zero violations across all enabled linters
  - [ ] Documented in DEVELOPMENT.md which linters are active and why

### Refactor: Split `pkg/launcher` into sub-packages

- **Type**: refactor
- **Fleet Phase**: 5
- **Priority**: medium
- **Effort**: high
- **Dependencies**: none
- **Cross-repo**: crane imports `pkg/launcher` — import paths will change
- **Existing issues**: none
- **Description**: At 6,535 lines with 9 interfaces, `pkg/launcher` is the largest single package. The review recommends splitting into `launcher/loader`, `launcher/resolver`, `launcher/builder` (or similar) sub-packages based on the existing interface boundaries: `PackageLoader`, `Resolver`, `PatchProcessor`, `SchemaGenerator`, `Validator`, `Builder`. This improves navigability, testability, and allows consumers to import only what they need. This is a breaking change for any external consumers.
- **Acceptance criteria**:
  - [ ] `pkg/launcher` split into 3+ sub-packages along interface boundaries
  - [ ] All existing tests pass in their new locations
  - [ ] Crane's imports updated in a coordinated PR
  - [ ] Public API surface is preserved (no removed types/functions)

### Refactor: Simplify Bundle.Generate() label propagation

- **Type**: refactor
- **Fleet Phase**: 5
- **Priority**: low
- **Effort**: low
- **Dependencies**: none
- **Cross-repo**: none
- **Existing issues**: none
- **Description**: The label copy loop in `Bundle.Generate()` creates `obj := *r` (value dereference of a double pointer), then calls `obj.SetLabels(labels)` which modifies the original because `client.Object` is an interface backed by a pointer. The `obj := *r` creates a misleading appearance of copying when it actually operates on the same underlying object. Simplify to operate directly on `*r` for clarity.
- **Acceptance criteria**:
  - [ ] Label propagation operates directly on `*r` without intermediate value copy
  - [ ] Existing label propagation tests pass unchanged
  - [ ] Comment added explaining the labeling behavior

---

### Feature: CNPG Database CR builder

- **Type**: feature
- **Fleet Phase**: 2
- **Priority**: critical
- **Effort**: medium
- **Dependencies**: none
- **Cross-repo**: crane needs this for database component type support
- **Existing issues**: #259
- **Description**: Implement a builder for the CloudNativePG `Database` custom resource. This is a critical-priority item needed for crane's CNPG component handler to manage database lifecycle.
- **Acceptance criteria**:
  - [ ] `Create`, `Set*`, `Add*` functions following kure builder conventions
  - [ ] Comprehensive unit tests with table-driven patterns
  - [ ] `doc.go` with usage examples

### Feature: CNPG ObjectStore CR builder

- **Type**: feature
- **Fleet Phase**: 2
- **Priority**: critical
- **Effort**: medium
- **Dependencies**: none
- **Cross-repo**: crane needs this for backup configuration
- **Existing issues**: #260
- **Description**: Implement a builder for the CloudNativePG `ObjectStore` custom resource (`barmancloud.cnpg.io/v1`). Required for crane to configure CNPG backup destinations (S3, Azure Blob, GCS).
- **Acceptance criteria**:
  - [ ] Builder covers all ObjectStore spec fields
  - [ ] Support for S3, Azure, and GCS configurations
  - [ ] Unit tests with table-driven patterns

### Feature: CNPG ScheduledBackup and managed roles

- **Type**: feature
- **Fleet Phase**: 3
- **Priority**: high
- **Effort**: medium
- **Dependencies**: CNPG Database CR builder, CNPG ObjectStore CR builder
- **Cross-repo**: crane's CNPG component handler
- **Existing issues**: #261, #262
- **Description**: Add ScheduledBackup plugin method with advanced configuration knobs, and managed roles builder for CNPG. The ScheduledBackup depends on ObjectStore being available; managed roles depend on Database.
- **Acceptance criteria**:
  - [ ] ScheduledBackup builder with cron schedule, backup method, and target support
  - [ ] Managed roles builder with login, superuser, createdb, and other PostgreSQL role attributes
  - [ ] Unit tests for both builders

### Feature: HelmRelease chartRef and OCIRepository sourcing

- **Type**: feature
- **Fleet Phase**: 3
- **Priority**: high
- **Effort**: medium
- **Dependencies**: none
- **Cross-repo**: crane's Helm component handler
- **Existing issues**: #267
- **Description**: Add support for `chartRef` field in HelmRelease, enabling OCIRepository as a chart source instead of the traditional `HelmRepository` + `chart` + `version` pattern. This is the modern Flux approach for OCI-delivered Helm charts.
- **Acceptance criteria**:
  - [ ] `SetHelmReleaseChartRef` with kind and name parameters
  - [ ] OCIRepository builder for chart sources
  - [ ] Tests validating chartRef vs traditional sourceRef mutual exclusivity

### Feature: HelmRelease targetNamespace, releaseName, and valuesFrom

- **Type**: feature
- **Fleet Phase**: 3
- **Priority**: high
- **Effort**: medium
- **Dependencies**: none
- **Cross-repo**: crane's Helm component handler
- **Existing issues**: #268, #234
- **Description**: Add `targetNamespace` and `releaseName` overrides to HelmRelease builder, and expose the `valuesFrom` field to support ConfigMap/Secret value sources. These are commonly needed for multi-tenant Helm deployments.
- **Acceptance criteria**:
  - [ ] `SetHelmReleaseTargetNamespace` and `SetHelmReleaseReleaseName` setters
  - [ ] `AddHelmReleaseValuesFrom` supporting ConfigMap and Secret references
  - [ ] Tests covering all new fields

### Feature: HelmRelease SourceRefName override

- **Type**: feature
- **Fleet Phase**: 3
- **Priority**: high
- **Effort**: low
- **Dependencies**: none
- **Cross-repo**: crane's Helm component handler
- **Existing issues**: #233
- **Description**: Allow callers to override the `sourceRef.name` in the FluxHelm generator instead of always deriving it from conventions. Needed when the HelmRepository name differs from the chart repository name.
- **Acceptance criteria**:
  - [ ] Generator accepts optional source ref name override
  - [ ] Default behavior preserved when override is not set
  - [ ] Tests for both default and override paths

### Feature: HelmRelease medium-priority enhancements (driftDetection, postRenderers, remediation, CRD lifecycle)

- **Type**: feature
- **Fleet Phase**: 4
- **Priority**: medium
- **Effort**: medium
- **Dependencies**: HelmRelease chartRef item above
- **Cross-repo**: crane's Helm component handler
- **Existing issues**: #270, #269, #236, #235
- **Description**: Four medium-priority HelmRelease enhancements that complete the Flux HelmRelease API surface: (1) `driftDetection` mode configuration, (2) `postRenderers` support for Kustomize overlays on Helm output, (3) `remediation` configuration for install/upgrade failure handling, (4) CRD lifecycle policy fields (create, update, delete). These can be implemented as a batch or individually.
- **Acceptance criteria**:
  - [ ] `SetHelmReleaseDriftDetection` with mode parameter
  - [ ] `AddHelmReleasePostRenderer` for Kustomize-type renderers
  - [ ] `SetHelmReleaseRemediation` for install and upgrade remediation
  - [ ] `SetHelmReleaseCRDPolicy` for create/update/delete CRD behavior
  - [ ] Tests for each new field

### Feature: Promote internal K8s builders to public API

- **Type**: feature
- **Fleet Phase**: 3
- **Priority**: high
- **Effort**: high
- **Dependencies**: Audit internal vs public API duality (below)
- **Cross-repo**: crane currently uses internal builders via kure — promotion changes import paths
- **Existing issues**: #241, #245
- **Description**: Kure has mature resource builders in `internal/kubernetes` that are duplicated (in reduced form) in `pkg/kubernetes`. The review identifies this duality. Item #245 audits what to promote; #241 executes the promotion. This requires defining promotion criteria (stability, test coverage, API surface completeness), then moving builders from internal to pkg with stable API contracts. This is a significant public API commitment — promoted builders become part of kure's semver contract.
- **Acceptance criteria**:
  - [ ] Promotion criteria documented in `docs/development/`
  - [ ] Builders promoted: Deployment, StatefulSet, DaemonSet, Job, CronJob, Service, Ingress, HPA, PDB (minimum set)
  - [ ] All promoted builders have comprehensive tests, doc.go examples, and README.md entries
  - [ ] `internal/kubernetes` deprecated functions removed or redirected
  - [ ] Crane updated to use new import paths

### Feature: InitContainer support for Deployment/StatefulSet/DaemonSet builders

- **Type**: feature
- **Fleet Phase**: 3
- **Priority**: high
- **Effort**: low
- **Dependencies**: none (can be done in internal/ first, promoted later)
- **Cross-repo**: crane's SecurityContextMutator needs InitContainer coverage (fleet report finding)
- **Existing issues**: #272
- **Description**: Add `AddInitContainer` functions to Deployment, StatefulSet, and DaemonSet builders. This is also a prerequisite for crane's `SecurityContextMutator` to cover init containers — the fleet report identified that crane's mutator only covers Deployment and CronJob, missing StatefulSet and DaemonSet entirely. Kure providing complete builders enables crane to fix its coverage gap.
- **Acceptance criteria**:
  - [ ] `AddDeploymentInitContainer`, `AddStatefulSetInitContainer`, `AddDaemonSetInitContainer` functions
  - [ ] Init containers included in PodSpec output
  - [ ] Tests validating init container placement and ordering

### Feature: NetworkPolicy and HTTPRoute builders

- **Type**: feature
- **Fleet Phase**: 3
- **Priority**: high
- **Effort**: medium
- **Dependencies**: none
- **Cross-repo**: crane generates default-deny NetworkPolicy — kure builder would replace inline construction
- **Existing issues**: #242
- **Description**: Add builders for Kubernetes `NetworkPolicy` (networking.k8s.io/v1) and Gateway API `HTTPRoute` (gateway.networking.k8s.io/v1). NetworkPolicy is needed for crane's default-deny security model. HTTPRoute supports the Gateway API migration from Ingress.
- **Acceptance criteria**:
  - [ ] `CreateNetworkPolicy` with ingress/egress rule builders
  - [ ] `CreateHTTPRoute` with match, filter, and backend ref builders
  - [ ] Tests covering common patterns (default-deny, allow-from-namespace, path-based routing)

### Feature: PSA security context helpers and ResourceRequirements builder

- **Type**: feature
- **Fleet Phase**: 4
- **Priority**: medium
- **Effort**: medium
- **Dependencies**: Promote internal K8s builders
- **Cross-repo**: crane's PSA enforcement uses these; fleet report identifies coverage gap
- **Existing issues**: #243, #244
- **Description**: Add Pod Security Admission (PSA) helpers that generate security contexts conforming to `restricted`, `baseline`, and `privileged` profiles. Add a `ResourceRequirements` builder for CPU/memory requests and limits. These support crane's security-by-default approach identified in the fleet report.
- **Acceptance criteria**:
  - [ ] `PSARestricted()`, `PSABaseline()` helpers returning `SecurityContext`
  - [ ] `CreateResourceRequirements` with CPU and memory request/limit setters
  - [ ] Tests validating PSA compliance for each profile level

### Feature: Layout enhancements (flat root, presets, naming, kustomization ref mode)

- **Type**: feature
- **Fleet Phase**: 3-4
- **Priority**: high (flat root, presets) / medium (naming, ref mode)
- **Effort**: high
- **Dependencies**: FluxPlacement bug fix (#264) should be done first
- **Cross-repo**: crane's layout generation
- **Existing issues**: #240, #263, #266, #265
- **Description**: Four layout system enhancements: (1) Flat root output via `NodeGrouping=GroupFlat` (#240, high) — needed for simplified single-cluster layouts; (2) Configurable layout presets with Pattern A as default (#263, high) — codifies common layout patterns; (3) `{kind}-{name}.yaml` file naming (#266, medium) — alternative to current naming scheme; (4) Configurable kustomization.yaml reference mode per FluxPlacement (#265, medium) — controls how kustomizations reference their sources.
- **Acceptance criteria**:
  - [ ] `GroupFlat` mode produces resources at the layout root without node subdirectories
  - [ ] At least one named preset (Pattern A) that configures layout rules in one call
  - [ ] `{kind}-{name}.yaml` naming available via LayoutRules
  - [ ] Kustomization reference mode configurable per FluxPlacement
  - [ ] Golden file tests for each layout variation

### Feature: FluxCD bundle enhancements (HealthChecks, prune protection)

- **Type**: feature
- **Fleet Phase**: 4
- **Priority**: medium
- **Effort**: low
- **Dependencies**: none
- **Cross-repo**: crane's Flux kustomization generation
- **Existing issues**: #237, #258
- **Description**: Add `HealthChecks` field to Bundle for configuring Flux health assessment on generated Kustomizations, and support `kustomize.toolkit.fluxcd.io/prune: disabled` annotation on resources that should not be garbage-collected.
- **Acceptance criteria**:
  - [ ] `Bundle.HealthChecks` field with typed check specifications
  - [ ] Health checks propagated to generated Flux Kustomizations
  - [ ] `SetPruneProtection` annotation helper on resource builders
  - [ ] Tests for health check propagation and prune protection

### Feature: Flux bootstrap enhancements (SourceKind, flux-operator primary mode)

- **Type**: feature
- **Fleet Phase**: 4
- **Priority**: medium
- **Effort**: medium
- **Dependencies**: none
- **Cross-repo**: none
- **Existing issues**: #254, #256
- **Description**: Add `SourceKind` field to `BootstrapConfig` to support both `GitRepository` and `OCIRepository` as bootstrap sources. Make `flux-operator` the primary bootstrap mode, with the current `flux-bootstrap` becoming the legacy path. The flux-operator approach is the modern Flux deployment model.
- **Acceptance criteria**:
  - [ ] `BootstrapConfig.SourceKind` field with `GitRepository` and `OCIRepository` options
  - [ ] `FluxOperator` bootstrap mode generates `FluxInstance` CR
  - [ ] Legacy bootstrap mode still available and tested
  - [ ] Documentation updated to recommend flux-operator as primary

---

### Chore: Migrate FluxCD APIs from v1beta2 to v1

- **Type**: chore
- **Fleet Phase**: 3
- **Priority**: high
- **Effort**: high
- **Dependencies**: FluxCD 2.8 upgrade (#128)
- **Cross-repo**: crane uses these APIs — coordinated migration required
- **Existing issues**: #249 (OCIRepository), #250 (notification), #251 (image-automation), #252 (remove v1beta2 scheme)
- **Description**: Three Flux CRD groups need v1beta2-to-v1 migration: OCIRepository, notification-controller APIs (Alert, Provider, Receiver), and image-automation APIs (ImageUpdateAutomation, ImagePolicy, ImageRepository). After all three are migrated, the v1beta2 scheme registrations can be removed (#252). This is a breaking change that requires a coordinated crane update.
- **Acceptance criteria**:
  - [ ] OCIRepository builders use `source.toolkit.fluxcd.io/v1`
  - [ ] Notification builders use `notification.toolkit.fluxcd.io/v1`
  - [ ] Image automation builders use `image.toolkit.fluxcd.io/v1`
  - [ ] v1beta2 scheme registrations removed
  - [ ] All existing tests updated to v1 API shapes
  - [ ] Crane's imports and usage updated in coordinated PR

### Chore: Upgrade FluxCD ecosystem to 2.8

- **Type**: chore
- **Fleet Phase**: 3
- **Priority**: high
- **Effort**: medium
- **Dependencies**: none
- **Cross-repo**: crane's FluxCD dependency must be upgraded in lockstep
- **Existing issues**: #128
- **Description**: Upgrade all FluxCD dependencies (source-controller, kustomize-controller, helm-controller, notification-controller, image-automation-controller) to the 2.8 release line. This is a prerequisite for the v1beta2-to-v1 API migration and Flux 2.8 feature builders (#255).
- **Acceptance criteria**:
  - [ ] All FluxCD Go module dependencies updated to 2.8.x
  - [ ] `go mod tidy` clean
  - [ ] All tests pass with new dependency versions
  - [ ] `versions.yaml` updated

### Chore: Upgrade dependency ecosystem (cert-manager, metallb, external-secrets)

- **Type**: chore
- **Fleet Phase**: 4
- **Priority**: medium
- **Effort**: medium
- **Dependencies**: none
- **Cross-repo**: crane may use these builders — verify compatibility
- **Existing issues**: #246 (cert-manager 1.19), #247 (metallb 0.15.3), #248 (external-secrets 1.3)
- **Description**: Three CRD ecosystem dependency upgrades. External-secrets 1.3 is a **breaking change** due to a Go module path change. Cert-manager 1.19 and metallb 0.15.3 are minor upgrades. The deep review notes that these packages could benefit from `doc.go` examples — combine the upgrade with documentation improvements.
- **Acceptance criteria**:
  - [ ] cert-manager upgraded to 1.19.x with passing tests
  - [ ] metallb upgraded to 0.15.3 with passing tests
  - [ ] external-secrets upgraded to 1.3.x with module path migration
  - [ ] `doc.go` examples added to each upgraded package

### Chore: Expand Kubernetes target range to 1.33-1.35

- **Type**: chore
- **Fleet Phase**: 4
- **Priority**: medium
- **Effort**: medium
- **Dependencies**: K8s 1.35 upgrade (#129)
- **Cross-repo**: crane's K8s dependency must align
- **Existing issues**: #253, #129
- **Description**: Upgrade `k8s.io/api` and related modules to v0.35 and validate compatibility across K8s 1.33-1.35 range. The deep review notes four `k8s.io/*` replace directives in `go.mod` that pin specific versions — document why each is needed and whether the upgrade removes the need for any of them.
- **Acceptance criteria**:
  - [ ] `k8s.io/api` upgraded to v0.35.x
  - [ ] Tests pass against K8s 1.33, 1.34, and 1.35 API schemas
  - [ ] Replace directives documented with justification comments
  - [ ] Removed replace directives where no longer needed

### Chore: Update versions.yaml for all dependency upgrades

- **Type**: chore
- **Fleet Phase**: 3
- **Priority**: medium
- **Effort**: low
- **Dependencies**: individual dependency upgrade items above
- **Cross-repo**: none
- **Existing issues**: #257
- **Description**: After completing dependency upgrades, update `versions.yaml` to reflect all current dependency versions. This is the single source of truth for version tracking.
- **Acceptance criteria**:
  - [ ] `versions.yaml` reflects all current dependency versions
  - [ ] No stale version entries

### Chore: Document k8s.io replace directives in go.mod

- **Type**: chore
- **Fleet Phase**: 3
- **Priority**: low
- **Effort**: low
- **Dependencies**: none
- **Cross-repo**: none
- **Existing issues**: none
- **Description**: The deep review identifies four `k8s.io/*` replace directives as a maintenance burden. Add inline comments in `go.mod` explaining why each pin is needed (version incompatibility, upstream bug, API breakage) and when each can be removed (specific upstream version, fixed issue).
- **Acceptance criteria**:
  - [ ] Each replace directive has a comment explaining the reason and removal condition
  - [ ] Tracked in a follow-up issue if any can be removed now

### Chore: Plan v1alpha1 API graduation path

- **Type**: chore
- **Fleet Phase**: 5
- **Priority**: low
- **Effort**: low
- **Dependencies**: public API promotion (#241)
- **Cross-repo**: crane uses `v1alpha1` types — graduation is a breaking change
- **Existing issues**: none
- **Description**: The deep review notes the `v1alpha1` package (1,278 lines of converters and serialization) and asks when it graduates to `v1beta1` or `v1`. Define graduation criteria (stability duration, API coverage, consumer count) and document the timeline. This does not require code changes — it is a planning document.
- **Acceptance criteria**:
  - [ ] Graduation criteria documented in `docs/development/`
  - [ ] Timeline proposed (e.g., "v1beta1 after 6 months of stable v1alpha1 API")
  - [ ] Breaking changes inventory for the graduation

---

### Security: Enable `gosec` linter for automated security scanning

- **Type**: security
- **Fleet Phase**: 3
- **Priority**: high
- **Effort**: medium
- **Dependencies**: none (can be done in parallel with other lint items)
- **Cross-repo**: meta should document gosec as a fleet-wide requirement
- **Existing issues**: none
- **Description**: The deep review explicitly calls out `gosec` as "important for a library that generates Kubernetes resources." The security section concludes with "Consider enabling gosec." Enable the linter, triage findings (fix genuine issues, `//nolint` false positives with justification). Given kure generates RBAC, certificates, and security contexts, automated security scanning is essential.
- **Acceptance criteria**:
  - [ ] `gosec` enabled in `.golangci.yml`
  - [ ] All genuine security findings fixed
  - [ ] False positives annotated with `//nolint:gosec // reason`
  - [ ] No new gosec violations in CI

---

### Testing: Add Flux 2.8 feature builder tests

- **Type**: testing
- **Fleet Phase**: 4
- **Priority**: medium
- **Effort**: medium
- **Dependencies**: FluxCD 2.8 upgrade (#128), Flux 2.8 feature builders (#255)
- **Cross-repo**: none
- **Existing issues**: #255
- **Description**: After the FluxCD 2.8 upgrade and new feature builder implementation, ensure comprehensive test coverage for all new Flux 2.8 capabilities. The deep review rates kure's test coverage as excellent — maintain that standard for new builders.
- **Acceptance criteria**:
  - [ ] All new Flux 2.8 builders have table-driven unit tests
  - [ ] Setter coverage tests following existing `setters_test.go` pattern
  - [ ] Golden file tests for complex resource generation

---

### CI: Upgrade to Go 1.26

- **Type**: ci
- **Fleet Phase**: 5
- **Priority**: low
- **Effort**: low
- **Dependencies**: Go 1.26 release availability
- **Cross-repo**: all Go repos in the fleet; meta `versions.env` must be updated first
- **Existing issues**: #133
- **Description**: Track the Go 1.26 upgrade. This is a future item dependent on Go 1.26's release schedule. When available, upgrade `go.mod`, CI matrices, and mise tooling.
- **Acceptance criteria**:
  - [ ] `go.mod` updated to `go 1.26`
  - [ ] All CI workflows use Go 1.26
  - [ ] `mise.toml` updated
  - [ ] All tests pass on Go 1.26

---

### Docs: Add doc.go examples for CRD builder packages

- **Type**: docs
- **Fleet Phase**: 4
- **Priority**: medium
- **Effort**: low
- **Dependencies**: none
- **Cross-repo**: none
- **Existing issues**: none
- **Description**: The deep review notes that `internal/certmanager`, `internal/externalsecrets`, and `internal/metallb` "could benefit from `doc.go` examples showing common composition patterns." Add `Example*` functions in `doc_test.go` or `example_test.go` files that demonstrate typical resource compositions (e.g., ClusterIssuer + Certificate, SecretStore + ExternalSecret, BGPPeer + BGPAdvertisement).
- **Acceptance criteria**:
  - [ ] `example_test.go` added to `internal/certmanager` with ClusterIssuer + Certificate example
  - [ ] `example_test.go` added to `internal/externalsecrets` with SecretStore + ExternalSecret example
  - [ ] `example_test.go` added to `internal/metallb` with BGP setup example
  - [ ] Examples compile and pass `go test`

### Docs: Add integration example (Cluster to Disk pipeline)

- **Type**: docs
- **Fleet Phase**: 4
- **Priority**: medium
- **Effort**: medium
- **Dependencies**: none
- **Cross-repo**: none
- **Existing issues**: none
- **Description**: The deep review recommends a documented "getting started" example showing the full `Cluster -> Workflow -> Layout -> Disk` pipeline. The `examples/` directory exists but lacks an end-to-end walkthrough. Create a complete example that builds a cluster with nodes, bundles, and applications, runs the Flux workflow engine, and writes the layout to disk.
- **Acceptance criteria**:
  - [ ] `examples/getting-started/` directory with a complete pipeline example
  - [ ] Covers: ClusterBuilder, Node/Bundle creation, Application registration, FluxWorkflow, Layout, Disk write
  - [ ] Compiles and runs as a standalone program
  - [ ] README.md explaining each step

### Docs: Clarify AGENTS.md fmt.Errorf guidance

- **Type**: docs
- **Fleet Phase**: 4
- **Priority**: low
- **Effort**: low
- **Dependencies**: none
- **Cross-repo**: none
- **Existing issues**: none
- **Description**: The deep review notes: "AGENTS.md says 'Never use fmt.Errorf directly' but the errors package itself wraps fmt.Errorf in Wrap/Wrapf/Errorf." The guidance should be clarified to state: "Always use `kure/pkg/errors` functions (`Wrap`, `Wrapf`, `Errorf`, typed errors) instead of calling `fmt.Errorf` directly in application code."
- **Acceptance criteria**:
  - [ ] AGENTS.md error handling section updated with precise wording
  - [ ] Example shows `errors.Wrap` vs raw `fmt.Errorf`

### Docs: Document deepCopyBundle shallow copy behavior

- **Type**: docs
- **Fleet Phase**: 5
- **Priority**: low
- **Effort**: low
- **Dependencies**: none
- **Cross-repo**: none
- **Existing issues**: none
- **Description**: The deep review identifies that `deepCopyBundle` copies `*Application` pointers (not deep copies of Application objects) and notes this is "fine for the builder's use case (append-only), but worth documenting as a deliberate shallow copy." Add a code comment explaining the design decision.
- **Acceptance criteria**:
  - [ ] Comment on `deepCopyBundle` explaining shallow copy of Application pointers is deliberate
  - [ ] Comment explains the append-only usage pattern that makes this safe

### Docs: Document Cluster getter/setter duality

- **Type**: docs
- **Fleet Phase**: 5
- **Priority**: low
- **Effort**: low
- **Dependencies**: none
- **Cross-repo**: none
- **Existing issues**: none
- **Description**: The deep review observes that `Cluster` has both exported fields and getter/setter methods that don't add validation. This creates two ways to do the same thing. Document the rationale: exported fields for internal/test use, getters/setters for library consumers who prefer encapsulation. This is a documentation-only change.
- **Acceptance criteria**:
  - [ ] Comment or `doc.go` note explaining the dual access pattern
  - [ ] Guidance on which style to prefer in new code

---

## Gap Analysis

### Items in review NOT in fleet plan

The deep code review identified the following items that are not covered by the fleet status report. These are kure-specific code quality findings that don't have fleet-wide implications:

1. **golangci-lint strictness** — The review's top recommendation; the fleet report mentions linting only for barge
2. **Launcher package decomposition** — Internal structural concern not visible at fleet level
3. **Bundle.Generate() label propagation clarity** — Code-level readability issue
4. **deepCopyBundle shallow copy documentation** — Implementation detail documentation
5. **Cluster getter/setter duality** — API design documentation
6. **AGENTS.md fmt.Errorf guidance** — Agent instruction refinement
7. **doc.go examples for CRD packages** — Internal documentation
8. **Integration examples** — Developer onboarding documentation
9. **k8s.io replace directives documentation** — Dependency maintenance concern
10. **v1alpha1 graduation planning** — API lifecycle planning

### Fleet plan items NOT detailed in review

The fleet status report identifies the following items relevant to kure that the deep code review did not cover:

1. **Domain naming standardization** (`wharf.zone` labels, `zone.wharf` events) — kure generates resources with labels; if the fleet standardizes on `wharf.zone`, kure's label generation may need updating
2. **Release tag creation** — The fleet report emphasizes zero release tags across all repos; the review doesn't mention kure's release status
3. **Shared code extraction to wharf/pkg** (#24 in fleet plan) — kure provides types consumed by wharf/pkg; expanding pkg with shared NATS/CloudEvents code may introduce new kure dependencies
4. **Crane's SecurityContextMutator/LabelMutator coverage gaps** — The fleet report identifies that crane's mutators miss StatefulSet/DaemonSet; kure needs to ensure its builders for these types are complete and promoted for crane to fix this
5. **Cosign/ORAS signing support** — The fleet report mentions crane's Cosign stub; kure issue #134 tracks OCI publishing with cosign support

---

## Dependency Graph

```
Phase 1-2 (Immediate)
  #264 FluxPlacement bug fix ─────────────────────┐
  #259 CNPG Database CR                           │
  #260 CNPG ObjectStore CR ───┐                   │
                              │                   │
Phase 3 (Standards)           │                   │
  errcheck linter ────────┐   │                   │
  ineffassign linter ─────┤   │                   │
  unused linter ──────────┤   │                   │
  gosec linter ───────────┤   │                   │
  gosimple linter ────────┼→ lint alignment       │
                          │   with crane          │
  #128 FluxCD 2.8 ───────┼→ #249 OCIRepo v1 ─┐   │
                          │  #250 Notif v1 ───┤   │
                          │  #251 ImageAuto v1─┼→ #252 remove v1beta2
                          │                   │
  #260+#259 ──────────────┼→ #261 ScheduledBackup
                          │  #262 Managed Roles
                          │
  #245 Audit API ─────────┼→ #241 Promote K8s builders ──→ crane import update
                          │  #272 InitContainer support
                          │  #242 NetworkPolicy + HTTPRoute
                          │
  #267 HelmRelease chartRef│
  #268 HelmRelease ns/name │
  #234 HelmRelease valuesFrom
  #233 HelmRelease sourceRef│
                          │
  #264 ───────────────────┼→ #240 Flat root output
                          │  #263 Layout presets
                          │
Phase 4 (Enhancement)     │
  #241 ───────────────────┼→ #243 PSA helpers
                          │  #244 ResourceRequirements
  #128 ───────────────────┼→ #255 Flux 2.8 features
  #129 K8s 1.35 ──────────┼→ #253 K8s range 1.33-1.35
  #246 cert-manager 1.19  │
  #247 metallb 0.15.3     │
  #248 external-secrets 1.3│
  #270 HelmRelease drift  │
  #269 HelmRelease postRenderers
  #236 HelmRelease remediation
  #235 HelmRelease CRD lifecycle
  #237 Bundle HealthChecks │
  #258 Prune protection   │
  #254 BootstrapConfig SourceKind
  #256 flux-operator primary
  doc.go examples         │
  integration example     │
                          │
Phase 5 (Strategic)       │
  launcher split ─────────┼→ crane import update
  v1alpha1 graduation ────┼→ crane type update
  #133 Go 1.26            │
  Bundle.Generate() cleanup│
  deepCopyBundle docs     │
  getter/setter docs      │
```

## Cross-Repo Coordination

### crane (most coordination needed)

| Action Item | Kure Change | Crane Impact |
|------------|-------------|--------------|
| Promote internal K8s builders (#241) | Move builders from `internal/` to `pkg/` | Update all imports from internal path to public path |
| InitContainer support (#272) | Add builder functions | Update SecurityContextMutator to cover init containers on all workload types |
| NetworkPolicy builder (#242) | Add `pkg/kubernetes` builder | Replace crane's inline NetworkPolicy construction |
| FluxCD v1beta2→v1 migration (#249-251) | Update all Flux builders to v1 | Update crane's Flux resource generation to v1 types |
| HelmRelease enhancements (#233-236, 267-270) | Add builder functions | Update crane's Helm component handler to use new fields |
| CNPG builders (#259-262) | Add CNPG resource builders | Implement CNPG component handler using kure builders |
| Layout enhancements (#240, 263-266) | Add layout modes and presets | Update crane's layout configuration to use new modes |
| Launcher split | Change `pkg/launcher` to sub-packages | Update import paths |
| v1alpha1 graduation | New API version package | Update type references |

### pkg (wharf/pkg shared library)

| Action Item | Coordination |
|------------|-------------|
| Shared code extraction (fleet #24) | If NATS/CloudEvents types are added to wharf/pkg, kure's `pkg/stack` types and wharf/pkg types must not create circular dependencies |
| Release tags (fleet #1-3) | wharf/pkg release unblocks crane release; kure's release is independent (GitHub) |

### meta (standards)

| Action Item | Coordination |
|------------|-------------|
| Lint alignment | Document the fleet-wide lint baseline in `meta/standards/` after kure aligns with crane |
| gosec requirement | Add gosec to the standard Go lint template in meta CI |
| Domain naming | If `meta/standards/cross-repo.md` is updated with the naming decision, kure should follow for any label generation |

### barge

| Action Item | Coordination |
|------------|-------------|
| Domain naming standardization | No direct dependency — kure and barge don't interact, but both should use `wharf.zone` labels |
