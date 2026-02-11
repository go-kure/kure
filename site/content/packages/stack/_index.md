+++
title = "Stack"
weight = 30
+++

# Stack Package

The stack package provides the core data model for organizing Kubernetes resources into a hierarchical structure suitable for GitOps deployment.

## Hierarchy

The stack organizes resources in a four-level hierarchy:

- **Cluster** — Top-level grouping representing a Kubernetes cluster
- **Node** — Logical grouping within a cluster (e.g., infrastructure, applications)
- **Bundle** — Collection of related applications
- **Application** — Individual Kubernetes application with its resources

This structure maps naturally to directory layouts consumed by GitOps tools like Flux and ArgoCD.

## Sub-packages

- [Generators](generators) — Resource generation strategies
- [Layout](/packages/layout) — Directory structure generation

## Documentation

- [Design](design) - Stack system design
- [Status](status) - Current implementation status

## API Reference

- [pkg.go.dev/github.com/go-kure/kure/pkg/stack](https://pkg.go.dev/github.com/go-kure/kure/pkg/stack)
