# Task: Add Quickstart Guide

**Priority:** 1 - High (Short Term: 1 day)
**Category:** docs
**Status:** Completed (577f6ab)
**Dependencies:** None
**Blocked By:** None

---

## Overview

Create a minimal working quickstart guide demonstrating `kure generate cluster` and `kurel build` workflows.

## Problem

Users don't have a simple getting-started path. Documentation is comprehensive but lacks a "quick start in 5 minutes" guide.

## Objectives

Create `docs/quickstart.md` with:
1. Installation instructions
2. Minimal cluster generation example
3. Kurel package building example
4. Next steps / further reading

## Implementation

### 1. Create Quickstart Guide

**File:** `docs/quickstart.md` (new)

Content outline:
- Installation (go install or download binary)
- Hello World: Generate simple cluster
- Build kurel package from example
- Deploy with Flux/ArgoCD
- Further reading

### 2. Update README.md

Add "Quick Start" section near the top with link to quickstart guide.

## Files to Create/Modify

1. `docs/quickstart.md` - Create new file
2. `README.md` - Add quick start section

## Success Criteria

- [ ] Quickstart guide created
- [ ] Can be completed in < 10 minutes
- [ ] All commands tested and work
- [ ] Linked from README.md
- [ ] Includes both Flux and ArgoCD examples

## Estimated Effort

**4-6 hours** - Writing and testing examples
