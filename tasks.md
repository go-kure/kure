# Kure Development Tasks

**Last Updated:** 2025-12-02
**Source:** Comprehensive repository review and alternate review analysis

This document provides an index of all prioritized development tasks for the Kure library and Kurel package tool. Each task is detailed in a separate file in the `tasks/` directory.

---

## Task Organization

Tasks are organized by:
- **Category**: `kurel`, `library`, `docs`, `cli`, `testing`, `deps`, `patch`, `future`
- **Subject**: Brief topic identifier
- **Priority**: `1-high`, `2-medium`, `3-future`

**File naming convention:** `tasks/{category}-{subject}-{priority}.md`

---

## Short Term (High Priority)

These tasks should be completed within 2-4 weeks:

| # | Task | Category | File |
|---|------|----------|------|
| 1 | **Finish KurelPackage Generator MVP** | kurel | [tasks/kurel-generator-mvp-1-high.md](tasks/kurel-generator-mvp-1-high.md) |
| 2 | **Wire generator into kurel build** | kurel | [tasks/kurel-build-integration-1-high.md](tasks/kurel-build-integration-1-high.md) |
| 3 | **Align go.mod versions** | deps | [tasks/deps-gomod-alignment-1-high.md](tasks/deps-gomod-alignment-1-high.md) |
| 4 | **Fix doc-code drift** | docs | [tasks/docs-code-drift-fix-1-high.md](tasks/docs-code-drift-fix-1-high.md) |
| 5 | **Add combined-output mode to kure patch** | cli | [tasks/cli-patch-combined-output-1-high.md](tasks/cli-patch-combined-output-1-high.md) |
| 6 | **Document Fluent Builder Pattern** | docs | [tasks/docs-fluent-builder-pattern-1-high.md](tasks/docs-fluent-builder-pattern-1-high.md) |
| 7 | **Add quickstart guide** | docs | [tasks/docs-quickstart-guide-1-high.md](tasks/docs-quickstart-guide-1-high.md) |
| 8 | **Expand README** | docs | [tasks/docs-readme-expansion-1-high.md](tasks/docs-readme-expansion-1-high.md) |

---

## Medium Term (1-2 Months)

| # | Task | Category | File |
|---|------|----------|------|
| 9 | **Standardize validation across packages** | library | [tasks/library-validation-standardize-2-medium.md](tasks/library-validation-standardize-2-medium.md) |
| 10 | **Add integration tests** | testing | [tasks/testing-integration-tests-2-medium.md](tasks/testing-integration-tests-2-medium.md) |
| 11 | **Implement --diff option for patches** | cli | [tasks/cli-patch-diff-option-2-medium.md](tasks/cli-patch-diff-option-2-medium.md) |
| 12 | **K8s OpenAPI schema integration** | kurel | [tasks/kurel-openapi-schema-2-medium.md](tasks/kurel-openapi-schema-2-medium.md) |
| 13 | **Add kurel validate --strict** | kurel | [tasks/kurel-validate-strict-2-medium.md](tasks/kurel-validate-strict-2-medium.md) |
| 14 | **Add GoDoc documentation** | docs | [tasks/docs-godoc-api-2-medium.md](tasks/docs-godoc-api-2-medium.md) |
| 15 | **Fuzz tests for patch parser** | testing | [tasks/testing-fuzz-tests-2-medium.md](tasks/testing-fuzz-tests-2-medium.md) |
| 16 | **Matrix tests across K8s versions** | testing | [tasks/testing-k8s-matrix-2-medium.md](tasks/testing-k8s-matrix-2-medium.md) |

---

## Long Term / Future Enhancements (2-4+ Months)

| # | Task | Category | File |
|---|------|----------|------|
| 17 | **OCI packaging + signing** | kurel | [tasks/kurel-oci-publishing-3-future.md](tasks/kurel-oci-publishing-3-future.md) |
| 18 | **Strategic-merge patches** | patch | [tasks/patch-strategic-merge-3-future.md](tasks/patch-strategic-merge-3-future.md) |
| 19 | **Blueprint scaffolds (kure init)** | cli | [tasks/cli-kure-init-3-future.md](tasks/cli-kure-init-3-future.md) |
| 20 | **Live diff (kurel diff)** | kurel | [tasks/kurel-diff-3-future.md](tasks/kurel-diff-3-future.md) |
| 21 | **Multi-env profiles** | library | [tasks/library-multienv-profiles-3-future.md](tasks/library-multienv-profiles-3-future.md) |
| 22 | **Fluent Builder implementation** | library | [tasks/library-fluent-builder-impl-3-future.md](tasks/library-fluent-builder-impl-3-future.md) |
| 23 | **Policy gating (kurel gate)** | kurel | [tasks/kurel-gate-policy-3-future.md](tasks/kurel-gate-policy-3-future.md) |
| 24 | **Package catalog (OCI index)** | kurel | [tasks/kurel-package-catalog-3-future.md](tasks/kurel-package-catalog-3-future.md) |
| 25 | **Migration tooling** | cli | [tasks/cli-yaml-to-kure-converter-3-future.md](tasks/cli-yaml-to-kure-converter-3-future.md) |

---

## Optional / Deferred

| # | Task | Category | Notes |
|---|------|----------|-------|
| 26 | **ArgoCD bootstrap** | library | Low priority - keep pluggable, fix obvious bugs only |
| 27 | **Interactive patch mode** | cli | Placeholder exists, keep as aspirational |
| 28 | **Multi-modal UX** | future | Separate UI project, not CLI focus |

---

## Quick Reference by Category

### Kurel (Package Tool)
- [kurel-generator-mvp-1-high.md](tasks/kurel-generator-mvp-1-high.md) - **Priority 1**
- [kurel-build-integration-1-high.md](tasks/kurel-build-integration-1-high.md) - **Priority 1**
- [kurel-openapi-schema-2-medium.md](tasks/kurel-openapi-schema-2-medium.md) - Priority 2
- [kurel-validate-strict-2-medium.md](tasks/kurel-validate-strict-2-medium.md) - Priority 2
- [kurel-oci-publishing-3-future.md](tasks/kurel-oci-publishing-3-future.md) - Priority 3
- [kurel-diff-3-future.md](tasks/kurel-diff-3-future.md) - Priority 3
- [kurel-gate-policy-3-future.md](tasks/kurel-gate-policy-3-future.md) - Priority 3
- [kurel-package-catalog-3-future.md](tasks/kurel-package-catalog-3-future.md) - Priority 3

### Library (Kure)
- [library-validation-standardize-2-medium.md](tasks/library-validation-standardize-2-medium.md) - Priority 2
- [library-multienv-profiles-3-future.md](tasks/library-multienv-profiles-3-future.md) - Priority 3
- [library-fluent-builder-impl-3-future.md](tasks/library-fluent-builder-impl-3-future.md) - Priority 3

### Documentation
- [docs-code-drift-fix-1-high.md](tasks/docs-code-drift-fix-1-high.md) - **Priority 1**
- [docs-fluent-builder-pattern-1-high.md](tasks/docs-fluent-builder-pattern-1-high.md) - **Priority 1**
- [docs-quickstart-guide-1-high.md](tasks/docs-quickstart-guide-1-high.md) - **Priority 1**
- [docs-readme-expansion-1-high.md](tasks/docs-readme-expansion-1-high.md) - **Priority 1**
- [docs-godoc-api-2-medium.md](tasks/docs-godoc-api-2-medium.md) - Priority 2

### CLI
- [cli-patch-combined-output-1-high.md](tasks/cli-patch-combined-output-1-high.md) - **Priority 1**
- [cli-patch-diff-option-2-medium.md](tasks/cli-patch-diff-option-2-medium.md) - Priority 2
- [cli-kure-init-3-future.md](tasks/cli-kure-init-3-future.md) - Priority 3
- [cli-yaml-to-kure-converter-3-future.md](tasks/cli-yaml-to-kure-converter-3-future.md) - Priority 3

### Testing
- [testing-integration-tests-2-medium.md](tasks/testing-integration-tests-2-medium.md) - Priority 2
- [testing-fuzz-tests-2-medium.md](tasks/testing-fuzz-tests-2-medium.md) - Priority 2
- [testing-k8s-matrix-2-medium.md](tasks/testing-k8s-matrix-2-medium.md) - Priority 2

### Dependencies
- [deps-gomod-alignment-1-high.md](tasks/deps-gomod-alignment-1-high.md) - **Priority 1**

### Patch System
- [patch-strategic-merge-3-future.md](tasks/patch-strategic-merge-3-future.md) - Priority 3

---

## Implementation Notes

1. **Start with Priority 1 tasks** - These are blockers or high-impact improvements
2. **Kurel generator is critical** - Many other features depend on it
3. **Documentation is essential** - Needed for adoption and contributions
4. **Test coverage improvements** - Balance with feature development
5. **Future features** - Revisit priorities quarterly

For implementation details, see individual task files in `tasks/` directory.
