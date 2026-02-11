+++
title = "Packages"
weight = 30
+++

# Kure Packages

Explore the core packages that make up the Kure library. Each package provides specific functionality for building and managing Kubernetes resources.

## Core Packages

### [Launcher](/packages/launcher)
The launcher package provides a package system for creating reusable, customizable Kubernetes applications without the complexity of templating engines. It uses a declarative patch-based approach to customize base Kubernetes manifests.

### [Patch](/packages/patch)
The patch package implements a JSONPath-based declarative patching system. It allows you to modify Kubernetes resources using a simple, powerful patch language that maintains YAML structure and comments.

### [Stack](/packages/stack)
The stack package provides the core data model for organizing Kubernetes resources into a hierarchical structure (Cluster → Node → Bundle → Application) suitable for GitOps deployment.

### [Layout](/packages/layout)
The layout package handles manifest organization and directory structure generation. It provides flexible rules for grouping and organizing generated Kubernetes resources into a clean directory structure.

### [IO](/packages/io)
The io package provides utilities for reading, writing, and parsing YAML representations of Kubernetes resources, including kubectl-compatible resource printing.

### [Errors](/packages/errors)
The errors package provides structured error types and handling utilities for Kubernetes resource validation, file operations, and configuration errors.

### [CLI](/packages/cli)
The cli package provides shared utilities and abstractions for building command-line interfaces in the Kure and kurel tools.

## Additional Resources

- [Architecture Overview](/architecture) - Understand how these packages fit together
- [Examples](/examples) - See the packages in action
- [API Documentation](https://pkg.go.dev/github.com/go-kure/kure) - Detailed API reference
