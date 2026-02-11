+++
title = "Errors"
weight = 60
+++

# Errors Package

The errors package provides structured error types and handling utilities for the Kure library and kurel tool.

## Error Types

The package provides several specialized error constructors:

- **ResourceValidationError** — Validation failures for Kubernetes resources, including kind, name, and field information
- **FileError** — File operation failures (read, write, parse) with operation type and path
- **ValidationError** — General validation failures with field details
- **ConfigurationError** — Configuration-related errors

## Predefined Errors

Common validation errors are predefined for efficiency and consistency:

- `ErrNilDeployment`, `ErrNilPod`, `ErrNilService`, `ErrNilConfigMap`
- `ErrGVKNotFound`, `ErrGVKNotAllowed`

## Error Wrapping

The package provides `Wrap` and `Wrapf` functions compatible with Go's standard `errors.Is` and `errors.As` for error unwrapping.

## API Reference

- [pkg.go.dev/github.com/go-kure/kure/pkg/errors](https://pkg.go.dev/github.com/go-kure/kure/pkg/errors)
