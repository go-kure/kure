+++
title = "IO"
weight = 50
+++

# IO Package

The io package provides utilities for reading, writing, and parsing YAML representations of Kubernetes resources. It acts as a thin wrapper around `sigs.k8s.io/yaml` and the Kubernetes runtime scheme from client-go.

## YAML Helpers

Basic marshalling and unmarshalling is performed through `Marshal` and `Unmarshal` functions which operate on the standard `io.Reader` and `io.Writer` interfaces. For persisting data to disk, `SaveFile` and `LoadFile` helpers wrap file creation and reading.

For in-memory operations the `Buffer` type implements both `io.Reader` and `io.Writer` and exposes `Marshal` and `Unmarshal` methods.

## Parsing Runtime Objects

Kubernetes manifests frequently contain multiple YAML documents separated by `---`. `ParseFile` reads such a manifest and decodes each document into a `runtime.Object` using the client-go scheme. Several additional API schemes from projects like FluxCD, cert-manager and MetalLB are registered so their custom resources can be parsed without further setup.

## Resource Printing

The package includes comprehensive resource printing capabilities compatible with kubectl output formats:

- **ResourcePrinter** — unified formatting for YAML, JSON, table, wide, and name output modes
- **SimpleTablePrinter** — kubectl-style table output without external dependencies
- **Convenience functions** — `PrintObjectsAsYAML`, `PrintObjectsAsJSON`, `PrintObjectsAsTable`

## API Reference

- [pkg.go.dev/github.com/go-kure/kure/pkg/io](https://pkg.go.dev/github.com/go-kure/kure/pkg/io)
