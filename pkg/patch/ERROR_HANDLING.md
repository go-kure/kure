# Error Handling Philosophy and Guidelines

This document establishes the unified error handling philosophy for the Kure patch module, reconciling the current mix of strict and graceful approaches.

## Core Philosophy: **Graceful by Default, Strict When Critical**

The patch module should prioritize **operational continuity** while maintaining **data integrity**. This means:

1. **Warn and Continue**: Missing targets, optional fields, or recoverable issues generate warnings but don't stop processing
2. **Fail Fast**: Data corruption, syntax errors, or critical system issues cause immediate failures
3. **Rich Context**: All errors include actionable information for debugging and resolution

---

## Error Categories and Handling

### Category 1: **Graceful Warnings** (Continue Processing)

These issues generate warnings but allow processing to continue:

#### Missing Patch Targets
```go
// Current inconsistent behavior - should be unified
if targetResource == nil {
    log.Printf("WARNING: Patch target '%s' not found, skipping patch: %s", 
               target, patch.Path)
    continue // Don't fail the entire operation
}
```

#### Optional Field Access
```go
// Field doesn't exist but could be created
if !fieldExists {
    log.Printf("WARNING: Field '%s' doesn't exist, creating with default value", 
               fieldPath)
    // Create field with appropriate default
}
```

#### Type Inference Fallbacks
```go
// Can't infer type, fall back to string
if inferredType == nil {
    log.Printf("WARNING: Could not infer type for '%s', using string value", 
               fieldName)
    value = stringValue
}
```

#### Selector Resolution Issues
```go
// Key-value selector doesn't match any items
if matchingItems == 0 {
    log.Printf("WARNING: Selector '%s' matched no items, skipping patch", 
               selector)
    continue
}
```

### Category 2: **Critical Failures** (Stop Processing)

These issues require immediate failure with detailed context:

#### Syntax Errors
```go
// Malformed patch syntax
if !isValidPatchSyntax(patchLine) {
    return fmt.Errorf("invalid patch syntax at line %d: %s\n" +
                     "Expected format: path[selector]: value\n" +
                     "Example: spec.containers[name=main].image: nginx:latest", 
                     lineNum, patchLine)
}
```

#### Data Corruption Risks
```go
// Would corrupt existing data structure
if wouldCorruptData(operation, target) {
    return fmt.Errorf("patch operation would corrupt data structure:\n" +
                     "Path: %s\n" +
                     "Operation: %s\n" +
                     "Target type: %T\n" +
                     "Suggested fix: Use correct selector syntax", 
                     path, operation, target)
}
```

#### System/IO Failures
```go
// File system or network issues
if err := writeOutput(data); err != nil {
    return fmt.Errorf("failed to write output to %s: %w\n" +
                     "Check file permissions and available disk space", 
                     outputPath, err)
}
```

#### Resource Schema Violations
```go
// Kubernetes API violations
if !isValidKubernetesField(resource, field) {
    return fmt.Errorf("invalid Kubernetes field for %s %s: %s\n" +
                     "Valid fields: %v\n" +
                     "API Version: %s", 
                     resource.GetKind(), resource.GetName(), 
                     field, validFields, resource.GetAPIVersion())
}
```

---

## Implementation Guidelines

### 1. Error Context Requirements

Every error must include:
- **What** went wrong (specific operation/field)
- **Where** it occurred (file, line, resource)
- **Why** it failed (root cause)
- **How** to fix it (actionable suggestion)

```go
// Good: Rich context with actionable information
return fmt.Errorf("failed to apply patch to container selector:\n" +
                 "Resource: %s/%s\n" +
                 "Path: %s\n" +
                 "Selector: %s\n" +
                 "Reason: %v\n" +
                 "Available containers: %v\n" +
                 "Suggestion: Check container names match exactly", 
                 resource.GetKind(), resource.GetName(),
                 patch.Path, patch.Selector, err, containerNames)

// Bad: Vague, no context
return fmt.Errorf("patch failed: %w", err)
```

### 2. Warning Format Standards

All warnings should follow a consistent format:

```go
func logWarning(category, context, issue, suggestion string) {
    if os.Getenv("KURE_DEBUG") != "" {
        log.Printf("WARNING [%s]: %s - %s\n  Suggestion: %s", 
                   category, context, issue, suggestion)
    }
}

// Usage
logWarning("PATCH_TARGET", 
          "deployment.app containers[name=sidecar]",
          "container not found, skipping patch",
          "verify container name matches spec.template.spec.containers[].name")
```

### 3. Debug Logging Integration

Enhanced debug information when `KURE_DEBUG=1`:

```go
func debugPatchOperation(op PatchOp, resource *unstructured.Unstructured) {
    if os.Getenv("KURE_DEBUG") != "" {
        log.Printf("DEBUG: Applying patch operation:\n" +
                   "  Resource: %s/%s (%s)\n" +
                   "  Operation: %s\n" +
                   "  Path: %s\n" +
                   "  Selector: %s\n" +
                   "  Value: %v (%T)",
                   resource.GetKind(), resource.GetName(), resource.GetAPIVersion(),
                   op.Op, op.Path, op.Selector, op.Value, op.Value)
    }
}
```

### 4. Batch Processing Error Handling

When processing multiple patches:

```go
func processPatchBatch(patches []PatchOp) error {
    var criticalErrors []error
    var warnings []string
    
    for i, patch := range patches {
        if err := processPatch(patch); err != nil {
            if isCriticalError(err) {
                criticalErrors = append(criticalErrors, 
                    fmt.Errorf("patch %d: %w", i+1, err))
            } else {
                warnings = append(warnings, 
                    fmt.Sprintf("patch %d: %v", i+1, err))
            }
        }
    }
    
    // Log all warnings
    for _, warning := range warnings {
        log.Printf("WARNING: %s", warning)
    }
    
    // Fail only if critical errors occurred
    if len(criticalErrors) > 0 {
        return fmt.Errorf("critical errors in patch processing:\n%v", 
                         criticalErrors)
    }
    
    return nil
}
```

---

## Migration Strategy

### Phase 1: Identify Current Inconsistencies
- [x] Audit all error returns in the patch module
- [x] Categorize each error type (graceful vs critical)
- [ ] Document current behavior vs desired behavior

### Phase 2: Implement Unified Error Types
```go
// Define error types for consistent handling
type PatchError struct {
    Type     ErrorType
    Context  string
    Message  string
    Cause    error
    Fix      string
}

type ErrorType int

const (
    ErrorTypeWarning  ErrorType = iota // Log and continue
    ErrorTypeCritical                  // Stop processing
    ErrorTypeDebug                     // Only show in debug mode
)
```

### Phase 3: Update Error Handling
- Replace inconsistent error handling with unified approach
- Add comprehensive test coverage for error scenarios
- Update CLI to respect the graceful/critical distinction

### Phase 4: Documentation and Examples
- Update all examples to show proper error handling
- Create troubleshooting guide with common error scenarios
- Document debug logging capabilities

---

## Testing Error Scenarios

Every error handling path must be tested:

```go
func TestPatchErrorHandling(t *testing.T) {
    tests := []struct {
        name           string
        patch          PatchOp
        expectError    bool
        expectWarning  bool
        errorType      ErrorType
    }{
        {
            name: "missing target - graceful",
            patch: PatchOp{Path: "nonexistent.field", Op: "replace"},
            expectError: false,
            expectWarning: true,
            errorType: ErrorTypeWarning,
        },
        {
            name: "invalid syntax - critical", 
            patch: PatchOp{Path: "malformed[[[syntax", Op: "replace"},
            expectError: true,
            expectWarning: false,
            errorType: ErrorTypeCritical,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test error handling behavior
        })
    }
}
```

This unified approach ensures predictable behavior while maintaining operational robustness and providing excellent debugging support.