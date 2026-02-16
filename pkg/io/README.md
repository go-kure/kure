# IO - YAML Serialization and Resource Printing

The `io` package provides utilities for parsing, serializing, and printing Kubernetes resources. It supports multiple output formats including YAML, JSON, and kubectl-compatible table views.

## Overview

This package handles the I/O boundary of Kure: reading Kubernetes manifests from files, serializing resources to YAML/JSON, and printing resources in human-readable formats. It integrates with Kure's registered scheme for type-aware parsing.

## Parsing

### Parse YAML Files

```go
import "github.com/go-kure/kure/pkg/io"

// Parse a multi-document YAML file into typed Kubernetes objects
objects, err := io.ParseFile("manifests/deployment.yaml")

// Parse YAML bytes directly
objects, err := io.ParseYAML(yamlData)
```

### Unstructured Fallback

By default, only GVKs registered in the kure scheme are accepted. To parse
arbitrary Kubernetes YAML (CRDs, custom operators, etc.) use
`ParseYAMLWithOptions` or `ParseFileWithOptions` with `AllowUnstructured`:

```go
opts := io.ParseOptions{AllowUnstructured: true}
objects, err := io.ParseYAMLWithOptions(yamlData, opts)
// Known types are returned as typed objects (e.g. *corev1.Pod).
// Unknown types are returned as *unstructured.Unstructured.
```

### Load and Save

```go
// Load a single object from file
obj, err := io.LoadFile("service.yaml")

// Save an object to file
err := io.SaveFile("output.yaml", deployment)
```

## Serialization

### Marshal and Unmarshal

```go
// Serialize to YAML bytes
data, err := io.Marshal(deployment)

// Deserialize from YAML bytes
var obj appsv1.Deployment
err := io.Unmarshal(data, &obj)
```

### Encode Multiple Objects

```go
// Encode as multi-document YAML
yamlData, err := io.EncodeObjectsToYAML(objects)

// Encode as JSON array
jsonData, err := io.EncodeObjectsToJSON(objects)
```

### Deterministic Field Ordering

```go
// Encode with Kubernetes-conventional field ordering
opts := io.EncodeOptions{KubernetesFieldOrder: true}
yamlData, err := io.EncodeObjectsToYAMLWithOptions(objects, opts)
// Output: apiVersion, kind, metadata, spec, ... status (last)
```

## Printing

### Output Formats

The package supports kubectl-compatible output formats:

| Format | Constant | Description |
|--------|----------|-------------|
| YAML | `OutputFormatYAML` | Full YAML output |
| JSON | `OutputFormatJSON` | Full JSON output |
| Table | `OutputFormatTable` | Columnar table view |
| Wide | `OutputFormatWide` | Extended table with extra columns |
| Name | `OutputFormatName` | Resource names only |

### Usage

```go
// Print as YAML to stdout
err := io.PrintObjectsAsYAML(objects, os.Stdout)

// Print as table
err := io.PrintObjectsAsTable(objects, false, false, os.Stdout)

// Use ResourcePrinter for configurable output
printer := io.NewResourcePrinter(io.PrintOptions{
    OutputFormat: io.OutputFormatTable,
    ShowLabels:   true,
})
err := printer.Print(objects, os.Stdout)
```

## Related Packages

- [errors](../errors/) - Error types for parse failures
- [kubernetes](../kubernetes/) - Scheme registration for type-aware parsing
