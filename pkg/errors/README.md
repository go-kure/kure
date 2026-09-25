# Errors - Structured Error Handling

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/errors.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/errors)

The `errors` package provides structured error types with contextual information for Kubernetes resource operations. All Kure packages use this instead of `fmt.Errorf`.

## Overview

Errors in Kure carry context: the type of error, what resource was affected, suggestions for fixing the problem, and the original cause. This makes debugging easier and enables programmatic error handling.

## Error Types

| Type | Use Case | Key Fields |
|------|----------|------------|
| `ValidationError` | Field validation failures | Field, Value, ValidValues, Suggestion |
| `ResourceError` | Resource-specific issues | Kind, Name, Namespace, Available |
| `PatchError` | Patch operation failures | Operation, Path, ResourceName |
| `ParseError` | File/YAML parsing errors | Source, Line, Column |
| `FileError` | File system operations | Operation, Path |
| `ConfigError` | Configuration problems | Source, Field, Value, ValidValues |

## Usage

### Wrapping Errors

```go
import "github.com/go-kure/kure/pkg/errors"

// Wrap with context
if err != nil {
    return errors.Wrap(err, "failed to load cluster config")
}

// Wrap with formatted message
return errors.Wrapf(err, "resource %s/%s not found", kind, name)
```

### Creating Errors

```go
// Simple error
return errors.New("invalid configuration")

// Formatted error
return errors.Errorf("unknown generator: %s", name)
```

### Typed Errors

```go
// Validation error with suggestion
return errors.NewValidationError(
    "replicas",          // field
    "-1",                // value
    "Deployment",        // component
    []string{"1", "3"},  // valid values
)

// Resource not found
return errors.ResourceNotFoundError(
    "Deployment",                    // resource type
    "my-app",                        // name
    "default",                       // namespace
    []string{"web-app", "api-app"},  // available resources
)

// Patch error
return errors.NewPatchError(
    "set",                           // operation
    "spec.replicas",                 // path
    "my-deployment",                 // resource name
    "field not found",               // reason
    originalErr,                     // cause
)

// Parse error with location
return errors.NewParseError(
    "config.yaml",   // source file
    "invalid YAML",  // reason
    42,              // line
    10,              // column
    originalErr,     // cause
)

// File error
return errors.NewFileError("read", "/path/to/file", "permission denied", originalErr)

// Configuration error
return errors.NewConfigError(
    "mise.toml",                    // source
    "go",                           // field
    "1.21",                         // value
    "version too old",              // reason
    []string{"1.23", "1.24"},       // valid values
)
```

### Inspecting Errors

```go
// Check if error is a Kure error
if errors.IsKureError(err) {
    kErr := errors.GetKureError(err)
    fmt.Println(kErr.Type())
    fmt.Println(kErr.Suggestion())
}

// Check specific error type
if errors.IsType(err, errors.ErrorTypeValidation) {
    // Handle validation error
}
```

## Predefined Errors

A small set of sentinels is predefined, one for each error Kure code actually returns, so callers
can match them with the standard library's `errors.Is`:

```go
errors.ErrNilPodSpec       // PSA validators (pkg/kubernetes)
errors.ErrNilContainer     // PSA validators (pkg/kubernetes)
errors.ErrNilBundle        // bundle validation (pkg/stack)
errors.ErrNilObject
errors.ErrNilRuntimeObject
errors.ErrGVKNotFound
errors.ErrGVKNotAllowed
errors.ErrUnsupportedKind
```

This package does not re-export `Is` or `As`, so import both packages under distinct names:

```go
import (
    stderrors "errors"

    kerrors "github.com/go-kure/kure/pkg/errors"
)

if stderrors.Is(err, kerrors.ErrNilPodSpec) {
    // handle nil pod spec
}
```

A sentinel that no Kure code returns is removed rather than kept exported: callers could compare
against it, but nothing would ever produce it. `TestExportedSentinelsHaveProducers` fails the build
if an exported `Err*` sentinel, declared in any file of this package, has no producer outside it: a
non-test reference through an import of this package. A comparison (`errors.Is`/`errors.As`
argument, `==`/`!=` operand, `switch` tag or case) is not a producer, and neither is an assignment
to the blank identifier (`var _ = errors.ErrX`). The check reads the source, so any other mention, <!-- doc-api-refs:ignore ErrX is a placeholder for any sentinel -->
such as a sentinel stored in a variable and only then compared, or passed to a logger, still counts
as produced.

## Related Packages

All Kure packages import this package for error handling. Never use `fmt.Errorf` directly.
