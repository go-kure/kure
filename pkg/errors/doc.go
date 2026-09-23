// Package errors provides structured error types and handling utilities for
// the Kure library and kurel tool.
//
// # Overview
//
// This package extends Go's standard error handling with domain-specific error
// types that provide structured information for Kubernetes resource validation,
// file operations, and configuration errors.
//
// # Error Types
//
// The package provides several specialized error constructors:
//
//   - [ResourceValidationError]: Validation failures for Kubernetes resources
//   - [FileError]: File operation failures (read, write, parse)
//   - [ValidationError]: General validation failures with field details
//   - [ConfigError]: Configuration-related errors
//
// # Predefined Errors
//
// A small set of sentinels is predefined, one for each error kure code
// actually returns; a sentinel no kure code returns is removed, not kept:
//
//	// Nil resource checks
//	errors.ErrNilPodSpec
//	errors.ErrNilContainer
//	errors.ErrNilBundle
//	errors.ErrNilObject
//	errors.ErrNilRuntimeObject
//
//	// GVK and kind errors
//	errors.ErrGVKNotFound
//	errors.ErrGVKNotAllowed
//	errors.ErrUnsupportedKind
//
// # Error Wrapping
//
// The package provides wrappers compatible with Go's error unwrapping:
//
//	// Wrap with message
//	err := errors.Wrap(originalErr, "failed to load config")
//
//	// Wrap with formatted message
//	err := errors.Wrapf(originalErr, "failed to process %s", filename)
//
//	// Check wrapped errors with the standard library's errors.Is; this
//	// package does not re-export it, so import the two under distinct names
//	// (stderrors "errors", kerrors "github.com/go-kure/kure/pkg/errors")
//	if stderrors.Is(err, kerrors.ErrNilPodSpec) {
//	    // handle nil pod spec
//	}
//
// # Resource Validation Errors
//
// Resource validation errors include structured fields:
//
//	err := errors.ResourceValidationError(
//	    "Deployment",           // resourceType
//	    "my-app",              // name
//	    "spec.replicas",       // field
//	    "must be positive",    // reason
//	    originalErr,           // cause (optional)
//	)
//
// These errors can be introspected for automated handling:
//
//	var resErr *kerrors.ResourceError
//	if stderrors.As(err, &resErr) {
//	    fmt.Printf("Resource: %s/%s\n", resErr.ResourceType, resErr.Name)
//	}
//
// # File Errors
//
// File operation errors include the operation type and path:
//
//	err := errors.NewFileError("read", "/path/to/file", "permission denied", nil)
//
// # Integration
//
// All error types implement the standard error interface and support
// Go 1.13+ error wrapping with errors.Is and errors.As.
package errors
