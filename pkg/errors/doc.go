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
//   - [ConfigurationError]: Configuration-related errors
//
// # Predefined Errors
//
// Common validation errors are predefined for efficiency and consistency:
//
//	// Nil resource checks
//	errors.ErrNilDeployment
//	errors.ErrNilPod
//	errors.ErrNilService
//	errors.ErrNilConfigMap
//
//	// GVK errors
//	errors.ErrGVKNotFound
//	errors.ErrGVKNotAllowed
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
//	// Check wrapped errors
//	if errors.Is(err, errors.ErrNilDeployment) {
//	    // handle nil deployment
//	}
//
// # Resource Validation Errors
//
// Resource validation errors include structured fields:
//
//	err := errors.ResourceValidationError(
//	    "Deployment",           // Kind
//	    "my-app",              // Name
//	    "spec.replicas",       // Field
//	    "must be positive",    // Message
//	    originalErr,           // Wrapped error (optional)
//	)
//
// These errors can be introspected for automated handling:
//
//	var resErr *errors.ResourceError
//	if errors.As(err, &resErr) {
//	    fmt.Printf("Resource: %s/%s\n", resErr.Kind, resErr.Name)
//	    fmt.Printf("Field: %s\n", resErr.Field)
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
