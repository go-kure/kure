# Task Improvements Applied

**Date:** 2025-12-02
**Based on:** Review feedback

---

## Summary of Changes

Applied improvements to task specifications based on comprehensive review feedback.

### Quick Wins Completed

1. **✅ deps-gomod-alignment-1-high.md**
   - Status: Marked as ✅ Completed
   - Added completion date and commit reference (6cfdbde)

2. **✅ kurel-generator-mvp-1-high.md**
   - Status: Updated to "In Progress"
   - Added comprehensive "Definition of Done" section with:
     - Code artifacts (8 criteria)
     - Testing requirements (4 criteria)
     - Documentation requirements (4 criteria)
     - Out of Scope items
     - Acceptance test script

3. **✅ cli-patch-combined-output-1-high.md**
   - Added UX Clarifications:
     - Document separators preservation
     - `--group-by` flag for resource ordering
     - Deterministic output requirements
   - Added "Definition of Done" section
   - Added "Out of Scope" items
   - Added acceptance test
   - Added risk mitigation notes

---

## Improvements by Category

### Definition of Done (DoD)

Added comprehensive DoD sections to critical tasks:
- **kurel-generator-mvp-1-high.md** - 4-section DoD with acceptance test
- **cli-patch-combined-output-1-high.md** - 3-section DoD with test cases

**DoD Structure:**
- Code Artifacts (what to deliver)
- Testing (unit, integration, coverage)
- Documentation (examples, GoDoc, READMEs)
- Out of Scope (avoid creep)
- Acceptance Test (executable verification)

### UX Clarifications

Enhanced user experience specifications:
- Document separator handling for multi-doc YAML
- Resource grouping strategies (`--group-by none|file|kind`)
- Deterministic sorting for reproducible output
- Exit code documentation

### Status Tracking

Improved task metadata:
- Completion tracking (date + commit)
- "In Progress" status where work exists
- Cross-references between related tasks

---

## Cross-Reference with pkg/stack/STATUS.md

**Issue:** Tasks overlap with `pkg/stack/STATUS.md` content.

**Resolution Options:**

### Option A: tasks/ as Source of Truth (Recommended)
- Keep detailed task specs in `tasks/*.md`
- Update `pkg/stack/STATUS.md` to reference tasks
- Add this header to STATUS.md:

```markdown
# Status

**Note:** Detailed implementation tasks are tracked in `/tasks/`. This file provides high-level status only.

See `/tasks.md` for prioritized task list.
```

### Option B: Merge into Single File
- Consolidate into `pkg/stack/STATUS.md`
- Remove `tasks/` directory
- Less flexible, harder to track individual tasks

**Recommendation:** Implement Option A.

---

## Remaining Improvements Needed

### High Priority Tasks Still Needing DoD

1. **docs-code-drift-fix-1-high.md** - Good (has script and checklist)
2. **docs-fluent-builder-pattern-1-high.md** - Good (clear deliverables)
3. **docs-quickstart-guide-1-high.md** - Needs DoD
4. **docs-readme-expansion-1-high.md** - Needs DoD
5. **kurel-build-integration-1-high.md** - Needs DoD enhancement:
   - Define CLI flags (`--output-dir`, `--format directory|oci`)
   - Define exit codes
   - Add integration test plan

### Medium Priority Tasks Needing Enhancement

All medium-priority tasks are currently stubs. Need to add:
- Detailed implementation sections
- Definition of Done
- Out of Scope
- Risk mitigation

Specifically:
1. **library-validation-standardize-2-medium.md**
   - Add consistency test requirements
   - Define validation parity goals
2. **kurel-openapi-schema-2-medium.md**
   - Align with kube-openapi usage
   - Define minimal parity goals (core workload types first)
   - Add performance/size guardrails
3. **kurel-validate-strict-2-medium.md**
   - Define strict mode behavior
   - Specify severity levels and exit codes
4. **cli-patch-diff-option-2-medium.md**
   - Specify diff format (unified vs side-by-side)
   - Define `--check` mode exit behavior

---

## Action Items

### Immediate (Today)
- [x] Mark deps task as completed
- [x] Enhance kurel-generator DoD
- [x] Enhance cli-patch DoD
- [ ] Add cross-reference to pkg/stack/STATUS.md
- [ ] Add DoD to kurel-build-integration task
- [ ] Add DoD to docs quickstart/readme tasks

### Short Term (This Week)
- [ ] Expand medium-priority task stubs with detailed plans
- [ ] Add risk mitigation sections to critical tasks
- [ ] Create lint script to detect task-STATUS.md drift
- [ ] Update tasks.md index with DoD completion status

### Medium Term (Next Sprint)
- [ ] Review all tasks quarterly
- [ ] Update completion status as work progresses
- [ ] Archive completed tasks
- [ ] Refine estimates based on actual effort

---

## Review Feedback Addressed

### ✅ Completed
- Marked deps task as completed with commit reference
- Added comprehensive DoD to kurel-generator
- Added DoD to cli-patch task
- Added UX clarifications for patch behavior
- Added Out of Scope sections

### ⏳ In Progress
- Identifying duplication with STATUS.md
- Enhancing medium-priority tasks

### 📋 Planned
- Add DoD to remaining high-priority tasks
- Create drift detection script
- Quarterly task review process

---

## Metrics

**Before improvements:**
- Tasks with DoD: 0/25 (0%)
- Tasks with completion status: 0/25 (0%)
- Tasks with acceptance tests: 0/25 (0%)

**After improvements:**
- Tasks with DoD: 3/25 (12%)
- Tasks with completion status: 1/25 (4%)
- Tasks with acceptance tests: 2/25 (8%)

**Target (by end of week):**
- Tasks with DoD: 13/25 (52% - all high + some medium)
- Tasks with completion status: 2/25 (8%)
- Tasks with acceptance tests: 8/25 (32%)
