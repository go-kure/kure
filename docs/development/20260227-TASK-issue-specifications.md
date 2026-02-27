# Kure — Issue Specifications

> **Generated**: 2026-02-27
> **Source**: Verified fleet action plan + kure action plan + deep code review
> **Total items**: 15 new + 22 existing (skip)
> **Dedup results**: 15 new, 0 update existing, 22 skip

## Summary Table

| # | Title | Type | Priority | Phase | Effort | Dedup |
|---|-------|------|----------|-------|--------|-------|
| 1 | refactor: enable errcheck linter and fix violations | refactor | high | 3 | high | create |
| 2 | refactor: enable ineffassign linter and fix violations | refactor | high | 3 | low | create |
| 3 | refactor: enable unused linter and remove dead code | refactor | medium | 3 | medium | create |
| 4 | security: enable gosec linter for automated security scanning | security | high | 3 | medium | create |
| 5 | refactor: enable gosimple linter and simplify flagged code | refactor | low | 3 | low | create |
| 6 | refactor: align golangci-lint config with crane linter set | refactor | medium | 3 | medium | create |
| 7 | chore: document k8s.io replace directives in go.mod | chore | low | 3 | low | create |
| 8 | docs: add doc.go examples for CRD builder packages | docs | medium | 4 | low | create |
| 9 | docs: add integration example (Cluster-to-Disk pipeline) | docs | medium | 4 | medium | create |
| 10 | docs: clarify AGENTS.md fmt.Errorf guidance | docs | low | 4 | low | create |
| 11 | refactor: split pkg/launcher into sub-packages | refactor | medium | 5 | high | create |
| 12 | refactor: simplify Bundle.Generate() label propagation | refactor | low | 5 | low | create |
| 13 | chore: plan v1alpha1 API graduation path | chore | low | 5 | low | create |
| 14 | docs: document deepCopyBundle shallow copy behavior | docs | low | 5 | low | create |
| 15 | docs: document Cluster getter/setter duality | docs | low | 5 | low | create |

## Existing Issues Referenced (skip)

The following 22 action plan items map directly to 40 existing GitHub issues. No new issues are needed for these items.

| Action Plan Item | Existing Issue(s) | Phase | Dedup |
|-----------------|-------------------|-------|-------|
| Bug: Fix FluxPlacement no-op | #264 | 2 | skip |
| Feature: CNPG Database CR builder | #259 | 2 | skip |
| Feature: CNPG ObjectStore CR builder | #260 | 2 | skip |
| Feature: CNPG ScheduledBackup + managed roles | #261, #262 | 3 | skip |
| Feature: HelmRelease chartRef + OCIRepository | #267 | 3 | skip |
| Feature: HelmRelease targetNamespace/releaseName/valuesFrom | #268, #234 | 3 | skip |
| Feature: HelmRelease SourceRefName override | #233 | 3 | skip |
| Feature: HelmRelease medium-priority batch | #270, #269, #236, #235 | 4 | skip |
| Feature: Promote internal K8s builders | #241, #245 | 3 | skip |
| Feature: InitContainer support | #272 | 3 | skip |
| Feature: NetworkPolicy + HTTPRoute builders | #242 | 3 | skip |
| Feature: PSA helpers + ResourceRequirements | #243, #244 | 4 | skip |
| Feature: Layout enhancements | #240, #263, #266, #265 | 3-4 | skip |
| Feature: FluxCD bundle enhancements | #237, #258 | 4 | skip |
| Feature: Flux bootstrap enhancements | #254, #256 | 4 | skip |
| Chore: Migrate FluxCD APIs v1beta2 to v1 | #249, #250, #251, #252 | 3 | skip |
| Chore: Upgrade FluxCD ecosystem to 2.8 | #128 | 3 | skip |
| Chore: Upgrade deps (cert-manager, metallb, external-secrets) | #246, #247, #248 | 4 | skip |
| Chore: Expand K8s target range to 1.33-1.35 | #253, #129 | 4 | skip |
| Chore: Update versions.yaml | #257 | 3 | skip |
| Testing: Flux 2.8 feature builder tests | #255 | 4 | skip |
| CI: Upgrade to Go 1.26 | #133 | 5 | skip |

## New Issue Specifications

---

## Issue 1: refactor: enable errcheck linter and fix violations

- **Labels**: `type/refactor`, `priority/high`, `phase/3`, `effort/high`
- **Blocked by**: none
- **Blocks**: "refactor: align golangci-lint config with crane linter set"
- **Fleet phase**: 3
- **Existing issue**: none
- **Dedup recommendation**: create

### Description

The deep code review's top recommendation is enabling the `errcheck` linter. The current `.golangci.yml` disables it with the comment "Too many unchecked errors in existing code," which indicates accumulated technical debt. `errcheck` is the most impactful Go linter — it catches unhandled errors that become silent failures at runtime.

Crane's `.golangci.yml` already enables `errcheck`. Enabling it in kure achieves lint parity across the wharf fleet's Go repositories and eliminates a class of latent bugs. The deep review's lint assessment rated this area 3 out of 5 stars — the only area of the codebase below 4 stars — specifically because 18 linters are disabled while only 6 are enabled.

This is the highest-effort lint item due to the volume of violations implied by the disable comment. The fix should be done as a single dedicated PR containing only errcheck fixes to keep the diff reviewable and bisectable.

### Acceptance Criteria

- [ ] `errcheck` enabled in `.golangci.yml` (remove from disabled list)
- [ ] Zero lint violations from `errcheck` across the entire codebase
- [ ] No functional regressions — all existing tests pass
- [ ] PR contains only errcheck fixes (no mixed changes)
- [ ] Tests pass: `make verify`
- [ ] No regressions in existing functionality

### Dependencies

- **Requires**: none
- **Cross-repo**: crane already enables errcheck (reference config); meta should document kure's lint baseline once aligned

---

## Issue 2: refactor: enable ineffassign linter and fix violations

- **Labels**: `type/refactor`, `priority/high`, `phase/3`, `effort/low`
- **Blocked by**: none
- **Blocks**: "refactor: align golangci-lint config with crane linter set"
- **Fleet phase**: 3
- **Existing issue**: none
- **Dedup recommendation**: create

### Description

The `ineffassign` linter detects assignments to variables that are immediately overwritten without being read. These are often real bugs where a computed value is silently discarded. The deep code review flags this as "should be enabled" and notes that crane already enables it.

The violation count is expected to be low since `ineffassign` catches logical errors that are typically few in number. This makes it a high-value, low-effort lint enablement. It can be done in parallel with the `errcheck` item or sequentially for reviewability.

### Acceptance Criteria

- [ ] `ineffassign` enabled in `.golangci.yml` (remove from disabled list)
- [ ] Zero lint violations from `ineffassign`
- [ ] All existing tests pass
- [ ] Tests pass: `make verify`
- [ ] No regressions in existing functionality

### Dependencies

- **Requires**: none
- **Cross-repo**: none

---

## Issue 3: refactor: enable unused linter and remove dead code

- **Labels**: `type/refactor`, `priority/medium`, `phase/3`, `effort/medium`
- **Blocked by**: none
- **Blocks**: "refactor: align golangci-lint config with crane linter set"
- **Fleet phase**: 3
- **Existing issue**: none
- **Dedup recommendation**: create

### Description

The `.golangci.yml` disables the `unused` linter with the comment "Many utility functions are kept for future use." The deep code review correctly observes that dead code accumulates and should be removed — it can be re-added from git history when needed. Keeping dead code increases maintenance burden and confuses both human reviewers and static analysis tools.

Some utility functions may be genuinely needed for the public API surface in `pkg/kubernetes` or `pkg/stack`. Those should be covered by tests or annotated with `//nolint:unused` and a justification comment explaining the intended consumer. The deep review notes that the `pkg/kubernetes` package has benchmark and coverage tests — promoted builders should have test coverage that makes the `unused` linter happy naturally.

### Acceptance Criteria

- [ ] `unused` enabled in `.golangci.yml` (remove from disabled list)
- [ ] Dead functions, types, and variables removed
- [ ] Retained utility functions justified with tests or `//nolint:unused` comments
- [ ] All existing tests pass
- [ ] Tests pass: `make verify`
- [ ] No regressions in existing functionality

### Dependencies

- **Requires**: none
- **Cross-repo**: none

---

## Issue 4: security: enable gosec linter for automated security scanning

- **Labels**: `type/security`, `priority/high`, `phase/3`, `effort/medium`
- **Blocked by**: none
- **Blocks**: "refactor: align golangci-lint config with crane linter set"
- **Fleet phase**: 3
- **Existing issue**: none
- **Dedup recommendation**: create

### Description

The deep code review explicitly calls out `gosec` as "important for a library that generates Kubernetes resources" and concludes the security section with "Consider enabling gosec." Kure generates RBAC rules, certificates, security contexts, and network policies — automated security scanning is essential for this class of code.

The review's security assessment is otherwise clean: no hardcoded secrets, RBAC follows least-privilege, certificate builders use `SecretKeySelector` patterns, and file path operations use `filepath.Clean`. Enabling `gosec` provides ongoing automated verification of these properties and catches regressions.

After enabling, triage findings into genuine issues (fix them) and false positives (annotate with `//nolint:gosec // reason`). Given the codebase's maturity, most findings are likely to be minor or false positives, but each should be evaluated.

### Acceptance Criteria

- [ ] `gosec` enabled in `.golangci.yml`
- [ ] All genuine security findings fixed
- [ ] False positives annotated with `//nolint:gosec // [justification]`
- [ ] No new `gosec` violations introduced in CI
- [ ] Tests pass: `make verify`
- [ ] No regressions in existing functionality

### Dependencies

- **Requires**: none (can be done in parallel with other lint items)
- **Cross-repo**: meta should document `gosec` as a fleet-wide requirement in `meta/standards/`

---

## Issue 5: refactor: enable gosimple linter and simplify flagged code

- **Labels**: `type/refactor`, `priority/low`, `phase/3`, `effort/low`
- **Blocked by**: none
- **Blocks**: "refactor: align golangci-lint config with crane linter set"
- **Fleet phase**: 3
- **Existing issue**: none
- **Dedup recommendation**: create

### Description

The `.golangci.yml` disables `gosimple` with the comment "Some simplifications reduce readability." The `gosimple` linter suggests Go idiom simplifications — for example, replacing `if x == true` with `if x`, or replacing `for i, _ := range` with `for i := range`.

Most `gosimple` suggestions genuinely improve clarity and align with Go community conventions. For the rare case where the expanded form is more readable (complex boolean expressions, multi-return function calls), use `//nolint:gosimple` with a justification comment. This is the lowest-priority lint item and the lowest-effort fix.

### Acceptance Criteria

- [ ] `gosimple` enabled in `.golangci.yml` (remove from disabled list)
- [ ] Simplifications applied where they improve clarity
- [ ] `//nolint:gosimple` with justification for cases where expanded form is more readable
- [ ] All existing tests pass
- [ ] Tests pass: `make verify`
- [ ] No regressions in existing functionality

### Dependencies

- **Requires**: none
- **Cross-repo**: none

---

## Issue 6: refactor: align golangci-lint config with crane linter set

- **Labels**: `type/refactor`, `priority/medium`, `phase/3`, `effort/medium`
- **Blocked by**: "enable errcheck", "enable ineffassign", "enable unused", "enable gosec", "enable gosimple"
- **Blocks**: none
- **Fleet phase**: 3
- **Existing issue**: none
- **Dedup recommendation**: create

### Description

After enabling the five core linters (errcheck, ineffassign, unused, gosec, gosimple), incrementally enable the additional linters that crane's `.golangci.yml` uses. The deep code review identifies that crane enables: `bodyclose`, `durationcheck`, `errorlint`, `exhaustive`, `misspell`, `nilerr`, and `whitespace` in addition to the baseline linters.

Enable each linter one at a time, fixing violations in dedicated commits (or small PRs) to keep diffs reviewable. This achieves lint parity across the wharf fleet's Go repositories (crane, kure, pkg). Once complete, the active linter set should be documented in `DEVELOPMENT.md` for contributor reference.

The fleet plan's consolidated phase plan (item #46) calls for kure lint alignment with crane in Phase 3, after the individual linter enablements.

### Acceptance Criteria

- [ ] All linters from crane's `.golangci.yml` enabled in kure: `bodyclose`, `durationcheck`, `errorlint`, `exhaustive`, `misspell`, `nilerr`, `whitespace`
- [ ] Zero violations across all newly enabled linters
- [ ] Active linter set documented in `DEVELOPMENT.md` (or `docs/development/`)
- [ ] Tests pass: `make verify`
- [ ] No regressions in existing functionality

### Dependencies

- **Requires**: Issues 1-5 (errcheck, ineffassign, unused, gosec, gosimple must be completed first)
- **Cross-repo**: crane's `.golangci.yml` is the reference config; meta should document the fleet lint baseline after alignment

---

## Issue 7: chore: document k8s.io replace directives in go.mod

- **Labels**: `type/chore`, `priority/low`, `phase/3`, `effort/low`
- **Blocked by**: none
- **Blocks**: none
- **Fleet phase**: 3
- **Existing issue**: none
- **Dedup recommendation**: create

### Description

The deep code review identifies four `k8s.io/*` replace directives in `go.mod` and flags them as a maintenance burden. Replace directives pin specific dependency versions, preventing normal `go mod tidy` upgrades. Each directive exists for a reason (version incompatibility, upstream bug, API breakage between k8s.io modules), but without documentation, future maintainers cannot evaluate when they can be removed.

Add inline comments in `go.mod` above each replace directive explaining: (1) why the pin is needed, (2) which upstream version or fix would allow removal, and (3) a link to the relevant issue or incompatibility. If any directives can be removed now (because the upstream issue is resolved), remove them and document the removal in the commit message.

This item is related to but independent of the K8s target range expansion (#253, #129). The documentation should be done first so that the K8s upgrade can evaluate each directive against the new version.

### Acceptance Criteria

- [ ] Each `k8s.io/*` replace directive in `go.mod` has a comment explaining the reason and removal condition
- [ ] Any directives that can be removed now are removed
- [ ] If removable directives are found, tracked in a follow-up issue or combined with #253
- [ ] Tests pass: `make verify`
- [ ] No regressions in existing functionality

### Dependencies

- **Requires**: none
- **Cross-repo**: none

---

## Issue 8: docs: add doc.go examples for CRD builder packages

- **Labels**: `type/docs`, `priority/medium`, `phase/4`, `effort/low`
- **Blocked by**: none
- **Blocks**: none
- **Fleet phase**: 4
- **Existing issue**: none
- **Dedup recommendation**: create

### Description

The deep code review rates the CRD builder packages (`internal/certmanager`, `internal/externalsecrets`, `internal/metallb`) as 4 out of 5 stars and specifically notes they "could benefit from `doc.go` examples showing common composition patterns." Every other package in kure already has comprehensive `doc.go` documentation — maintaining that standard for CRD packages improves discoverability and onboarding.

Add `example_test.go` files (using Go's testable example convention) that demonstrate typical resource compositions:
- **certmanager**: `ClusterIssuer` + `Certificate` composition (ACME issuer with DNS01 solver)
- **externalsecrets**: `SecretStore` + `ExternalSecret` composition (Vault or AWS Secrets Manager backend)
- **metallb**: `BGPPeer` + `BGPAdvertisement` + `IPAddressPool` composition

These examples serve dual purpose: they appear in `go doc` output AND they are compiled and run by `go test`, ensuring they stay current with API changes.

### Acceptance Criteria

- [ ] `example_test.go` added to `internal/certmanager` with ClusterIssuer + Certificate example
- [ ] `example_test.go` added to `internal/externalsecrets` with SecretStore + ExternalSecret example
- [ ] `example_test.go` added to `internal/metallb` with BGP setup example
- [ ] All examples compile and pass `go test`
- [ ] Tests pass: `make verify`
- [ ] No regressions in existing functionality

### Dependencies

- **Requires**: none (can be done any time, but Phase 4 avoids conflicts with Phase 3 dependency upgrades #246, #247, #248)
- **Cross-repo**: none

---

## Issue 9: docs: add integration example (Cluster-to-Disk pipeline)

- **Labels**: `type/docs`, `priority/medium`, `phase/4`, `effort/medium`
- **Blocked by**: none
- **Blocks**: none
- **Fleet phase**: 4
- **Existing issue**: none
- **Dedup recommendation**: create

### Description

The deep code review recommends a documented "getting started" example showing the full `Cluster -> Workflow -> Layout -> Disk` pipeline. The `examples/` directory exists in the repository but lacks an end-to-end walkthrough that demonstrates kure's core value proposition: programmatic Kubernetes manifest generation for GitOps.

Create a complete, standalone example program that:
1. Builds a `Cluster` using `NewClusterBuilder` with nodes and bundles
2. Registers applications with `ApplicationConfig` implementations
3. Runs the `FluxWorkflow` engine to generate Flux Kustomizations and Sources
4. Produces a `Layout` from the workflow output
5. Writes the layout to disk as a structured manifest directory

This serves as both documentation and a regression test. The example should be a compilable `main.go` that a new contributor can run to understand the library's architecture. The deep review identifies kure's domain model (`Cluster -> Node -> Bundle -> Application`) and workflow system (`ResourceGenerator`, `LayoutIntegrator`, `BootstrapGenerator`) as core strengths — this example demonstrates both.

### Acceptance Criteria

- [ ] `examples/getting-started/` directory with a complete pipeline example
- [ ] Covers: `ClusterBuilder`, Node/Bundle creation, Application registration, FluxWorkflow execution, Layout generation, Disk write
- [ ] Compiles and runs as a standalone program (`go run ./examples/getting-started/`)
- [ ] `README.md` in the example directory explaining each step
- [ ] Tests pass: `make verify`
- [ ] No regressions in existing functionality

### Dependencies

- **Requires**: none
- **Cross-repo**: none (this is a kure-internal documentation improvement)

---

## Issue 10: docs: clarify AGENTS.md fmt.Errorf guidance

- **Labels**: `type/docs`, `priority/low`, `phase/4`, `effort/low`
- **Blocked by**: none
- **Blocks**: none
- **Fleet phase**: 4
- **Existing issue**: none
- **Dedup recommendation**: create

### Description

The deep code review notes a contradiction in the developer guidance: "AGENTS.md says 'Never use `fmt.Errorf` directly' but the errors package itself wraps `fmt.Errorf` in `Wrap`/`Wrapf`/`Errorf`." The current wording could confuse contributors into thinking `fmt.Errorf` is prohibited even within the `pkg/errors` package itself.

The guidance should be clarified to state: "Always use `kure/pkg/errors` functions (`Wrap`, `Wrapf`, `Errorf`, typed error constructors) instead of calling `fmt.Errorf` directly in application code. The `pkg/errors` package itself uses `fmt.Errorf` internally — this is correct and expected."

Add an example showing the preferred pattern vs the discouraged pattern to make the guidance actionable.

### Acceptance Criteria

- [ ] AGENTS.md error handling section updated with precise wording distinguishing "application code" from "pkg/errors internals"
- [ ] Example added showing `errors.Wrap(err, "context")` vs raw `fmt.Errorf("context: %w", err)`
- [ ] No contradictions between AGENTS.md and actual `pkg/errors` implementation
- [ ] Tests pass: `make verify`
- [ ] No regressions in existing functionality

### Dependencies

- **Requires**: none
- **Cross-repo**: none

---

## Issue 11: refactor: split pkg/launcher into sub-packages

- **Labels**: `type/refactor`, `priority/medium`, `phase/5`, `effort/high`
- **Blocked by**: none
- **Blocks**: none
- **Fleet phase**: 5
- **Existing issue**: none
- **Dedup recommendation**: create

### Description

At 6,535 lines of source code, `pkg/launcher` is the largest single package in kure. The deep code review identifies that `interfaces.go` defines 9 distinct interfaces — `PackageLoader`, `Resolver`, `PatchProcessor`, `SchemaGenerator`, `Validator`, `Builder`, and others — each representing a clear boundary within the package. The review recommends splitting into sub-packages such as `launcher/loader`, `launcher/resolver`, and `launcher/builder` (or similar groupings based on the interface boundaries).

Splitting improves navigability, testability, and allows consumers to import only the interfaces they need. It also makes the package's internal architecture self-documenting through the directory structure. However, this is a **breaking change** for crane, which imports `pkg/launcher` directly — crane's imports must be updated in a coordinated PR.

The split should preserve all existing public types and functions (no removed API surface). Tests should be moved to their new package locations and continue to pass. Internal implementation details can be restructured freely.

### Acceptance Criteria

- [ ] `pkg/launcher` split into 3+ sub-packages along interface boundaries
- [ ] All existing tests pass in their new package locations
- [ ] Public API surface preserved — no removed types or functions
- [ ] `internal/` implementation details restructured as appropriate
- [ ] Crane's imports updated in a coordinated PR
- [ ] Tests pass: `make verify`
- [ ] No regressions in existing functionality

### Dependencies

- **Requires**: none (Phase 5 strategic item)
- **Cross-repo**: crane imports `pkg/launcher` — import paths will change; requires a coordinated crane PR

---

## Issue 12: refactor: simplify Bundle.Generate() label propagation

- **Labels**: `type/refactor`, `priority/low`, `phase/5`, `effort/low`
- **Blocked by**: none
- **Blocks**: none
- **Fleet phase**: 5
- **Existing issue**: none
- **Dedup recommendation**: create

### Description

The deep code review identifies a misleading code pattern in `Bundle.Generate()`. The label copy loop creates `obj := *r` (value dereference of a double pointer), then calls `obj.SetLabels(labels)`. Because `client.Object` is an interface backed by a pointer, `obj.SetLabels(labels)` modifies the original object — the `obj := *r` creates a misleading appearance of copying when it actually operates on the same underlying data.

The fix is straightforward: operate directly on `*r` instead of creating the intermediate `obj` variable. This eliminates the false impression of value semantics and makes the mutation visible at the call site. Add a comment explaining the labeling behavior for future maintainers.

This is a code clarity improvement with no behavioral change.

### Acceptance Criteria

- [ ] Label propagation in `Bundle.Generate()` operates directly on `*r` without intermediate value copy
- [ ] Existing label propagation tests pass unchanged (confirming no behavioral change)
- [ ] Comment added explaining why direct mutation is correct (interface-backed pointer semantics)
- [ ] Tests pass: `make verify`
- [ ] No regressions in existing functionality

### Dependencies

- **Requires**: none
- **Cross-repo**: none

---

## Issue 13: chore: plan v1alpha1 API graduation path

- **Labels**: `type/chore`, `priority/low`, `phase/5`, `effort/low`
- **Blocked by**: "feat: promote internal K8s builders to public API" (#241)
- **Blocks**: none
- **Fleet phase**: 5
- **Existing issue**: none
- **Dedup recommendation**: create

### Description

The deep code review notes the `v1alpha1` package (1,278 lines of converters and serialization) and raises the question of when it graduates to `v1beta1` or `v1`. Kure's public API surface is growing with builder promotions (#241), and the `v1alpha1` label sets expectations about stability. Without a documented graduation path, consumers (primarily crane) cannot plan for the eventual API migration.

This is a **planning document only** — no code changes required. Define graduation criteria such as: stability duration (e.g., 6 months without breaking changes), API coverage threshold, consumer count, and test coverage requirements. Document the timeline and inventory of breaking changes that graduation would introduce.

The planning should happen after the builder promotion (#241) is complete, since that significantly expands the public API surface and may change the graduation calculus.

### Acceptance Criteria

- [ ] Graduation criteria documented in `docs/development/` (stability duration, coverage threshold, consumer requirements)
- [ ] Timeline proposed (e.g., "v1beta1 after 6 months of stable v1alpha1 API with 2+ consumers")
- [ ] Breaking changes inventory for the graduation (type renames, package moves, removed deprecated APIs)
- [ ] Tests pass: `make verify`
- [ ] No regressions in existing functionality

### Dependencies

- **Requires**: #241 (promote internal K8s builders) should be completed first to finalize the public API surface
- **Cross-repo**: crane uses `v1alpha1` types — graduation is a breaking change requiring coordinated migration

---

## Issue 14: docs: document deepCopyBundle shallow copy behavior

- **Labels**: `type/docs`, `priority/low`, `phase/5`, `effort/low`
- **Blocked by**: none
- **Blocks**: none
- **Fleet phase**: 5
- **Existing issue**: none
- **Dedup recommendation**: create

### Description

The deep code review identifies that `deepCopyBundle` (used by the `NewClusterBuilder` copy-on-write pattern) copies `*Application` pointers rather than deep-copying the `Application` objects themselves. The code does `copy(newBundle.Applications, b.Applications)` which creates a new slice with the same `*Application` pointers — the `Application` objects are shared between the original and the copy.

The review notes this is "fine for the builder's use case (append-only), but worth documenting as a deliberate shallow copy." The copy-on-write builder pattern (`With*` methods return new builders backed by deep copies) is thread-safe for appending new applications, but would break if a consumer modified an existing `Application` object after branching. Document this as a deliberate design choice with a code comment explaining the safety invariant.

### Acceptance Criteria

- [ ] Code comment on `deepCopyBundle` explaining that `*Application` pointers are shared (shallow copy), not deep-copied
- [ ] Comment explains the append-only usage pattern that makes this safe
- [ ] Comment notes the invariant: callers must not mutate existing `Application` objects after branching
- [ ] Tests pass: `make verify`
- [ ] No regressions in existing functionality

### Dependencies

- **Requires**: none
- **Cross-repo**: none

---

## Issue 15: docs: document Cluster getter/setter duality

- **Labels**: `type/docs`, `priority/low`, `phase/5`, `effort/low`
- **Blocked by**: none
- **Blocks**: none
- **Fleet phase**: 5
- **Existing issue**: none
- **Dedup recommendation**: create

### Description

The deep code review observes that the `Cluster` type in `pkg/stack` has both exported fields and getter/setter methods that do not add validation beyond simple assignment. This creates two ways to access the same data — `cluster.Name` vs `cluster.GetName()` / `cluster.SetName()`. The review rates this as a minor observation ("fine for a library") but notes it could confuse contributors about which style to use.

Document the rationale as a code comment or `doc.go` note: exported fields exist for direct access in tests and internal code where brevity matters; getter/setter methods exist for library consumers who prefer encapsulation and may benefit from future validation additions. Provide guidance on which style to prefer in new code (setters for new public API, direct field access for tests).

### Acceptance Criteria

- [ ] Comment or `doc.go` note on `Cluster` explaining the dual access pattern rationale
- [ ] Guidance on which style to prefer in new code (setters for public API, fields for tests/internals)
- [ ] No code changes — documentation only
- [ ] Tests pass: `make verify`
- [ ] No regressions in existing functionality

### Dependencies

- **Requires**: none
- **Cross-repo**: none
