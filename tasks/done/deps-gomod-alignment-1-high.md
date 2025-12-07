# Task: Align go.mod Versions

**Priority:** 1 - High (Short Term: 1 day)
**Category:** deps
**Status:** ✅ Completed
**Completed:** 2025-12-02 (commit 6cfdbde)
**Dependencies:** None
**Blocked By:** None

---

## Overview

Fix version inconsistency in `go.mod` where `k8s.io/cli-runtime` is required at v0.33.0 but replaced with v0.33.2.

## Current Status

**File:** `go.mod`

```go
require (
    k8s.io/cli-runtime v0.33.0  // Requires v0.33.0
    // ... other deps
)

replace (
    k8s.io/cli-runtime => k8s.io/cli-runtime v0.33.2  // But replaces with v0.33.2
)
```

This causes confusion for contributors and can lead to unexpected behavior.

## Problem

- Version mismatch reduces clarity
- Potential build issues in different environments
- Contributors may be confused about which version is actually used
- Go module resolution may behave unexpectedly

## Solution

**Option A: Align to v0.33.2 (Recommended)**

```go
require (
    k8s.io/cli-runtime v0.33.2  // Match replacement version
    // ...
)

// Remove replace directive if no longer needed
```

**Option B: Align to v0.33.0**

```go
require (
    k8s.io/cli-runtime v0.33.0
    // ...
)

// Remove replace directive
```

## Implementation Tasks

### 1. Verify Current Usage

```bash
# Check which version is actually used
go list -m k8s.io/cli-runtime

# Check if v0.33.2 is required for compatibility
grep -r "cli-runtime" --include="*.go"
```

### 2. Update go.mod

**If choosing Option A (v0.33.2):**

```bash
# Update require statement
sed -i 's/k8s.io\/cli-runtime v0.33.0/k8s.io\/cli-runtime v0.33.2/' go.mod

# Remove replace directive
sed -i '/k8s.io\/cli-runtime => k8s.io\/cli-runtime v0.33.2/d' go.mod

# Tidy
go mod tidy
```

### 3. Verify All K8s Dependencies

Check if other Kubernetes dependencies should be aligned:

```bash
grep "k8s.io" go.mod | grep require
grep "k8s.io" go.mod | grep replace
```

Current state from review:
- `k8s.io/api` v0.33.2
- `k8s.io/apimachinery` v0.33.2
- `k8s.io/client-go` v0.33.2
- `k8s.io/cli-runtime` v0.33.0 → v0.33.2 (inconsistent)

**Recommendation:** Align all to v0.33.2

### 4. Test Build

```bash
# Clean build cache
go clean -cache

# Build all packages
go build ./...

# Run tests
go test ./...

# Run CI locally if possible
```

## Files to Modify

1. `go.mod` - Update require/replace statements
2. `go.sum` - Will be updated by `go mod tidy`

## Success Criteria

- [ ] All K8s dependencies at consistent version
- [ ] No unnecessary replace directives
- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes
- [ ] `go mod tidy` produces no changes
- [ ] CI build passes

## Testing

```bash
# Verify versions
go list -m all | grep k8s.io

# Should show consistent versions:
# k8s.io/api v0.33.2
# k8s.io/apimachinery v0.33.2
# k8s.io/client-go v0.33.2
# k8s.io/cli-runtime v0.33.2
```

## Risks

- **Low Risk** - This is a patch version update (v0.33.0 → v0.33.2)
- No API breaking changes expected in patch releases
- All other K8s deps already at v0.33.2

## References

- Kubernetes versioning: https://github.com/kubernetes/community/blob/master/contributors/design-proposals/release/versioning.md
- Go modules documentation: https://go.dev/ref/mod

## Estimated Effort

**1 hour** - Simple version alignment
