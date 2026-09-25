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

Each Go block in this section is the body of an `Example` function in `example_test.go`, which
`go test` runs: it imports this package as `errors`, and `fmt` for the lines that print the errors
the example built. In your own code, return these errors rather than print them. `Wrap` and
`Wrapf` return nil for a nil error, so they need no `if err != nil` guard.

### Wrapping Errors

<!-- doc-example: pkg/errors ExampleWrap -->
```go
err := errors.New("connection refused")

// Wrap with context
wrapped := errors.Wrap(err, "failed to load cluster config")

// Wrap with formatted message
formatted := errors.Wrapf(err, "failed to fetch %s/%s", "Deployment", "my-app")

fmt.Println(wrapped)
fmt.Println(formatted)
```
<!-- doc-example:end -->

### Creating Errors

<!-- doc-example: pkg/errors ExampleErrorf -->
```go
// Simple error
simple := errors.New("invalid configuration")

// Formatted error
formatted := errors.Errorf("unknown generator: %s", "AppWorkload")

fmt.Println(simple)
fmt.Println(formatted)
```
<!-- doc-example:end -->

### Typed Errors

<!-- doc-example: pkg/errors ExampleNewValidationError -->
```go
originalErr := errors.New("original cause")

// Validation error with suggestion
validation := errors.NewValidationError(
    "replicas",         // field
    "-1",               // value
    "Deployment",       // component
    []string{"1", "3"}, // valid values
)

// Resource not found
notFound := errors.ResourceNotFoundError(
    "Deployment",                   // resource type
    "my-app",                       // name
    "default",                      // namespace
    []string{"web-app", "api-app"}, // available resources
)

// Patch error
patch := errors.NewPatchError(
    "set",             // operation
    "spec.replicas",   // path
    "my-deployment",   // resource name
    "field not found", // reason
    originalErr,       // cause
)

// Parse error with location
parse := errors.NewParseError(
    "config.yaml",  // source file
    "invalid YAML", // reason
    42,             // line
    10,             // column
    originalErr,    // cause
)

// File error
file := errors.NewFileError("read", "/path/to/file", "permission denied", originalErr)

// Configuration error
config := errors.NewConfigError(
    "mise.toml",              // source
    "go",                     // field
    "1.21",                   // value
    "version too old",        // reason
    []string{"1.23", "1.24"}, // valid values
)

for _, err := range []error{validation, notFound, patch, parse, file, config} {
    fmt.Println(err)
}
```
<!-- doc-example:end -->

### Inspecting Errors

`IsKureError`, `GetKureError` and `IsType` look through wrapping, so they find a typed error
anywhere in the chain:

<!-- doc-example: pkg/errors ExampleIsKureError -->
```go
err := errors.Wrap(
    errors.NewValidationError("replicas", "-1", "Deployment", []string{"1", "3"}),
    "load cluster config",
)

// Check if error is a Kure error
if errors.IsKureError(err) {
    kErr := errors.GetKureError(err)
    fmt.Println(kErr.Type())
    fmt.Println(kErr.Suggestion())
}

// Check specific error type
if errors.IsType(err, errors.ErrorTypeValidation) {
    // Handle validation error
    fmt.Println("validation error:", err)
}
```
<!-- doc-example:end -->

## Predefined Errors

A small set of sentinels is predefined, one for each error Kure code actually returns, so callers
can match them with the standard library's `errors.Is`:

<!-- doc-example:excerpt a list of the exported sentinels, not statements -->
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

<!-- doc-example:excerpt the import block alone, which the example below uses -->
```go
import (
    stderrors "errors"

    kerrors "github.com/go-kure/kure/pkg/errors"
)
```

The block below is the body of an `Example` function in `example_sentinel_test.go`, which
`go test` runs: it imports the two packages as above, `github.com/go-kure/kure/pkg/kubernetes` for
a function that returns a sentinel, and `fmt`.

<!-- doc-example: pkg/errors Example_sentinel -->
```go
// A PSA validator given no pod spec returns the ErrNilPodSpec sentinel.
err := kubernetes.ValidatePodSpecPSA(nil, kubernetes.PSARestricted)

if stderrors.Is(err, kerrors.ErrNilPodSpec) {
    // handle nil pod spec
    fmt.Println("nil pod spec:", err)
}
```
<!-- doc-example:end -->

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
