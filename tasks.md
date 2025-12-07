# Kure Development Tasks

**Last Updated:** 2025-12-03 (after rebase on upstream)
**Source:** Comprehensive repository review and alternate review analysis + upstream commits

This document provides an index of all prioritized development tasks for the Kure library and Kurel package tool. Each task is detailed in a separate file in the `tasks/` directory.

**Recent Changes:** 6 tasks marked as completed after rebasing on upstream commits that implemented them.

---

## ✅ Recently Completed (from upstream)

These tasks were completed in upstream commits before our task list was created:

| # | Task | Category | Completion |
|---|------|----------|-----------|
| 1 | **KurelPackage Generator MVP** | kurel | commit 9453a52 |
| 2 | **Wire generator into kurel build** | kurel | commit 9453a52 |
| 3 | **Fluent Builder Pattern (Phase 1)** | library | commit 28d2ed8 |
| 4 | **K8s OpenAPI Schema Integration** | kurel | commit 8bb7341 |
| 5 | **Document Fluent Builder Pattern** | docs | commit 28d2ed8 |
| 6 | **Align go.mod versions** | deps | commit 6cfdbde |

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

| # | Task | Category | File | Notes |
|---|------|----------|------|-------|
| 1 | **CEL Validation Enhancement** | kurel | 🆕 _New task - needs file creation_ | Validate CEL syntax with cel-go (from commit 3d1c75e) |
| 2 | **Add combined-output mode to kure patch** | cli | [tasks/02-cli-patch-combined-output-1-high.md](tasks/02-cli-patch-combined-output-1-high.md) | |
| 3 | **Fix doc-code drift** | docs | [tasks/03-docs-code-drift-fix-1-high.md](tasks/03-docs-code-drift-fix-1-high.md) | Check if commit 43901b2 resolved |
| 4 | **Add quickstart guide** | docs | [tasks/04-docs-quickstart-guide-1-high.md](tasks/04-docs-quickstart-guide-1-high.md) | DEVELOPMENT.md may satisfy |
| 5 | **Expand README** | docs | [tasks/05-docs-readme-expansion-1-high.md](tasks/05-docs-readme-expansion-1-high.md) | Partially done in commit 43901b2 |

---

## Medium Term (1-2 Months)

| # | Task | Category | File | Notes |
|---|------|----------|------|-------|
| 6 | **Standardize validation across packages** | library | [tasks/06-library-validation-standardize-2-medium.md](tasks/06-library-validation-standardize-2-medium.md) | Interval validation added (f29d3cb) |
| 7 | **Add integration tests** | testing | [tasks/07-testing-integration-tests-2-medium.md](tasks/07-testing-integration-tests-2-medium.md) | 105 test files now (ceeb125) |
| 8 | **Implement --diff option for patches** | cli | [tasks/08-cli-patch-diff-option-2-medium.md](tasks/08-cli-patch-diff-option-2-medium.md) | |
| 9 | **Add kurel validate --strict** | kurel | [tasks/09-kurel-validate-strict-2-medium.md](tasks/09-kurel-validate-strict-2-medium.md) | |
| 10 | **Add GoDoc documentation** | docs | [tasks/10-docs-godoc-api-2-medium.md](tasks/10-docs-godoc-api-2-medium.md) | |
| 11 | **Fuzz tests for patch parser** | testing | [tasks/11-testing-fuzz-tests-2-medium.md](tasks/11-testing-fuzz-tests-2-medium.md) | |
| 12 | **Matrix tests across K8s versions** | testing | [tasks/12-testing-k8s-matrix-2-medium.md](tasks/12-testing-k8s-matrix-2-medium.md) | |

---

## Long Term / Future Enhancements (2-4+ Months)

| # | Task | Category | File | Notes |
|---|------|----------|------|-------|
| 13 | **Plugin Architecture Implementation** | library | 🆕 _New task - needs file creation_ | From commit 30364e7 design doc |
| 14 | **OCI packaging + signing** | kurel | [tasks/14-kurel-oci-publishing-3-future.md](tasks/14-kurel-oci-publishing-3-future.md) | |
| 15 | **Strategic-merge patches** | patch | [tasks/15-patch-strategic-merge-3-future.md](tasks/15-patch-strategic-merge-3-future.md) | |
| 16 | **Blueprint scaffolds (kure init)** | cli | [tasks/16-cli-kure-init-3-future.md](tasks/16-cli-kure-init-3-future.md) | |
| 17 | **Live diff (kurel diff)** | kurel | [tasks/17-kurel-diff-3-future.md](tasks/17-kurel-diff-3-future.md) | |
| 18 | **Multi-env profiles** | library | [tasks/18-library-multienv-profiles-3-future.md](tasks/18-library-multienv-profiles-3-future.md) | |
| 19 | **Fluent Builder Phase 2** | library | [tasks/19-library-fluent-builder-impl-3-future.md](tasks/19-library-fluent-builder-impl-3-future.md) | Phase 1 ✅ complete |
| 20 | **Policy gating (kurel gate)** | kurel | [tasks/20-kurel-gate-policy-3-future.md](tasks/20-kurel-gate-policy-3-future.md) | |
| 21 | **Package catalog (OCI index)** | kurel | [tasks/21-kurel-package-catalog-3-future.md](tasks/21-kurel-package-catalog-3-future.md) | |
| 22 | **Migration tooling** | cli | [tasks/22-cli-yaml-to-kure-converter-3-future.md](tasks/22-cli-yaml-to-kure-converter-3-future.md) | |

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
- [kurel-generator-mvp-1-high.md](tasks/done/kurel-generator-mvp-1-high.md) - **Priority 1** ✅ DONE
- [kurel-build-integration-1-high.md](tasks/done/kurel-build-integration-1-high.md) - **Priority 1** ✅ DONE
- [kurel-openapi-schema-2-medium.md](tasks/done/kurel-openapi-schema-2-medium.md) - Priority 2 ✅ DONE
- [09-kurel-validate-strict-2-medium.md](tasks/09-kurel-validate-strict-2-medium.md) - Priority 2
- [14-kurel-oci-publishing-3-future.md](tasks/14-kurel-oci-publishing-3-future.md) - Priority 3
- [17-kurel-diff-3-future.md](tasks/17-kurel-diff-3-future.md) - Priority 3
- [20-kurel-gate-policy-3-future.md](tasks/20-kurel-gate-policy-3-future.md) - Priority 3
- [21-kurel-package-catalog-3-future.md](tasks/21-kurel-package-catalog-3-future.md) - Priority 3

### Library (Kure)
- [06-library-validation-standardize-2-medium.md](tasks/06-library-validation-standardize-2-medium.md) - Priority 2
- [18-library-multienv-profiles-3-future.md](tasks/18-library-multienv-profiles-3-future.md) - Priority 3
- [19-library-fluent-builder-impl-3-future.md](tasks/19-library-fluent-builder-impl-3-future.md) - Priority 3

### Documentation
- [03-docs-code-drift-fix-1-high.md](tasks/03-docs-code-drift-fix-1-high.md) - **Priority 1**
- [docs-fluent-builder-pattern-1-high.md](tasks/done/docs-fluent-builder-pattern-1-high.md) - **Priority 1** ✅ DONE
- [04-docs-quickstart-guide-1-high.md](tasks/04-docs-quickstart-guide-1-high.md) - **Priority 1**
- [05-docs-readme-expansion-1-high.md](tasks/05-docs-readme-expansion-1-high.md) - **Priority 1**
- [10-docs-godoc-api-2-medium.md](tasks/10-docs-godoc-api-2-medium.md) - Priority 2

### CLI
- [02-cli-patch-combined-output-1-high.md](tasks/02-cli-patch-combined-output-1-high.md) - **Priority 1**
- [08-cli-patch-diff-option-2-medium.md](tasks/08-cli-patch-diff-option-2-medium.md) - Priority 2
- [16-cli-kure-init-3-future.md](tasks/16-cli-kure-init-3-future.md) - Priority 3
- [22-cli-yaml-to-kure-converter-3-future.md](tasks/22-cli-yaml-to-kure-converter-3-future.md) - Priority 3

### Testing
- [07-testing-integration-tests-2-medium.md](tasks/07-testing-integration-tests-2-medium.md) - Priority 2
- [11-testing-fuzz-tests-2-medium.md](tasks/11-testing-fuzz-tests-2-medium.md) - Priority 2
- [12-testing-k8s-matrix-2-medium.md](tasks/12-testing-k8s-matrix-2-medium.md) - Priority 2

### Dependencies
- [deps-gomod-alignment-1-high.md](tasks/done/deps-gomod-alignment-1-high.md) - **Priority 1** ✅ DONE

### Patch System
- [15-patch-strategic-merge-3-future.md](tasks/15-patch-strategic-merge-3-future.md) - Priority 3

---

## Implementation Notes

1. **Start with Priority 1 tasks** - These are blockers or high-impact improvements
2. **Kurel generator is critical** - Many other features depend on it
3. **Documentation is essential** - Needed for adoption and contributions
4. **Test coverage improvements** - Balance with feature development
5. **Future features** - Revisit priorities quarterly

For implementation details, see individual task files in `tasks/` directory.
