# Task: Fix Doc-Code Drift

**Priority:** 1 - High (Short Term: 1-2 days)
**Category:** docs
**Status:** Completed (a2b2376)
**Dependencies:** None
**Blocked By:** None

---

## Overview

Update architecture documentation to reference actual code paths and remove references to non-existent packages.

## Problem

Documentation references `pkg/workflow` which doesn't exist. The actual implementation is in `pkg/stack/*` with separate workflow packages for Flux and ArgoCD.

This confuses contributors and makes it harder to understand the codebase structure.

## Issues Identified

### 1. pkg/workflow References

**Files to check:**
- `docs/ARCHITECTURE.md`
- `docs/ux-design.md`
- `pkg/stack/DESIGN.md`
- `pkg/launcher/ARCHITECTURE.md`

**Find occurrences:**

```bash
grep -r "pkg/workflow" docs/
grep -r "pkg/workflow" pkg/*/DESIGN.md
grep -r "pkg/workflow" pkg/*/ARCHITECTURE.md
```

**Correct paths:**
- `pkg/stack/workflow.go` - Workflow interface definition
- `pkg/stack/fluxcd/` - Flux workflow implementation
- `pkg/stack/argocd/` - ArgoCD workflow implementation

### 2. Other Potential Drift

- Outdated package descriptions
- References to removed features
- Incorrect file paths in examples

## Implementation Tasks

### 1. Audit All Documentation

```bash
# Find all markdown files
find . -name "*.md" -not -path "./vendor/*" -not -path "./.git/*"

# Check for common drift patterns
grep -r "TODO" docs/
grep -r "FIXME" docs/
grep -r "pkg/workflow" docs/
```

### 2. Update Architecture Docs

**File:** `docs/ARCHITECTURE.md`

**Changes needed:**
- Replace `pkg/workflow` with `pkg/stack/workflow.go` (interface)
- Document `pkg/stack/fluxcd/` and `pkg/stack/argocd/` separately
- Update package diagram to match actual structure

**Example fix:**

```markdown
<!-- BEFORE -->
The workflow system is implemented in `pkg/workflow/` with support for Flux and ArgoCD.

<!-- AFTER -->
The workflow system is defined by the `Workflow` interface in `pkg/stack/workflow.go`.
Implementations are in:
- `pkg/stack/fluxcd/` - Flux workflow (production-ready)
- `pkg/stack/argocd/` - ArgoCD workflow (partial implementation)
```

### 3. Update Package-Level Docs

**Files to review:**
- `pkg/stack/DESIGN.md`
- `pkg/stack/STATUS.md`
- `pkg/stack/generators/DESIGN.md`
- `pkg/stack/generators/ARCHITECTURE.md`
- `pkg/launcher/DESIGN.md`
- `pkg/launcher/ARCHITECTURE.md`
- `pkg/patch/DESIGN.md`

**For each file:**
1. Verify all code references are current
2. Update package structure diagrams
3. Add "Last Updated" date at top
4. Remove or mark deprecated content

### 4. Verify Examples

**Files:** `examples/*/README.md`

```bash
# Check example references
find examples/ -name "README.md" -exec grep -H "pkg/" {} \;
```

Ensure all example code snippets reference actual packages.

### 5. Update Main README

**File:** `README.md`

- Verify Getting Started section matches current API
- Update package structure diagram
- Ensure code examples work

### 6. Create Documentation Standards

**File:** `docs/DOCUMENTATION-STANDARDS.md` (new)

```markdown
# Documentation Standards

## Keeping Docs in Sync

1. **Update docs when changing code**
2. **Add "Last Updated" dates to design docs**
3. **Test all code examples**
4. **Reference actual file paths**

## Review Checklist

- [ ] All package paths exist
- [ ] Code examples tested
- [ ] Diagrams match implementation
- [ ] TODOs removed or tracked
```

## Files to Modify

1. `docs/ARCHITECTURE.md` - Update workflow references
2. `docs/ux-design.md` - Verify all paths
3. `pkg/stack/DESIGN.md` - Update if needed
4. `pkg/launcher/ARCHITECTURE.md` - Update if needed
5. `pkg/*/doc.go` - Add/update package documentation
6. `README.md` - Verify examples
7. `docs/DOCUMENTATION-STANDARDS.md` - Create new file

## Success Criteria

- [ ] No references to `pkg/workflow`
- [ ] All file paths in docs are valid
- [ ] Package structure diagrams match actual code
- [ ] Code examples in docs are tested
- [ ] "Last Updated" dates added to design docs
- [ ] Documentation standards guide created

## Script to Help

```bash
#!/bin/bash
# check-docs.sh - Verify documentation accuracy

echo "Checking for non-existent package references..."
grep -r "pkg/workflow" docs/ *.md pkg/*/DESIGN.md pkg/*/ARCHITECTURE.md && {
    echo "ERROR: Found references to pkg/workflow"
    exit 1
}

echo "Checking for broken file paths..."
find docs/ pkg/ -name "*.md" -exec grep -H "pkg/" {} \; | while read line; do
    path=$(echo "$line" | grep -oP 'pkg/[a-z/]+' | head -1)
    if [ -n "$path" ] && [ ! -d "$path" ] && [ ! -f "$path" ]; then
        echo "WARN: Possibly invalid path: $path in $line"
    fi
done

echo "Checking for TODO/FIXME in docs..."
grep -r "TODO\|FIXME" docs/ && {
    echo "WARN: Found TODOs in documentation"
}

echo "Done!"
```

## Testing

```bash
# Make script executable
chmod +x check-docs.sh

# Run checks
./check-docs.sh

# Manual verification
# 1. Read each DESIGN.md and verify accuracy
# 2. Test code examples in README.md
# 3. Verify package diagrams match `tree pkg/`
```

## References

- Current package structure: Run `tree -L 3 pkg/`
- Workflow implementation: `pkg/stack/workflow.go`, `pkg/stack/fluxcd/`, `pkg/stack/argocd/`

## Estimated Effort

**4-6 hours** - Document review and updates
