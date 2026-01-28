# Task: Implement proper CEL validation using cel-go library

**Priority:** 1 - High (2-4 weeks)
**Category:** kurel
**Status:** Completed (a2b2376)

---

## Overview

Replace basic CEL expression validation with proper cel-go library parsing to catch syntax errors early.

## Context

CEL (Common Expression Language) is used for validation rules in kurel packages. Currently,
`pkg/stack/generators/kurelpackage/v1alpha1.go:852-870` only performs basic string checks.

**Current limitations**:
- Only checks for `.Values` reference
- Character whitelist validation
- No syntax tree parsing
- Runtime errors instead of build-time errors

## Objectives

1. Add cel-go dependency to go.mod
2. Replace `validateCELExpression()` with proper cel-go parsing
3. Validate expression syntax at package generation time
4. Provide clear error messages for invalid CEL syntax

## Implementation

**Key file**: `pkg/stack/generators/kurelpackage/v1alpha1.go:852-870`

1. Import `github.com/google/cel-go/cel`
2. Create CEL environment with standard declarations
3. Parse and type-check expressions
4. Return detailed syntax errors

## Success Criteria

- cel-go library integrated
- All test cases pass (v1alpha1_test.go:419-447)
- Invalid CEL syntax detected at build time
- Clear error messages with position information
- No regression in existing validation

## Estimated Effort

**1-2 weeks**
