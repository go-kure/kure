# Kure `patch` Module — Purpose and Design

## Purpose

The `patch` module provides the **core abstraction layer** for loading, representing, and mutating Kubernetes resources using **structured patches** — without templates, overlays, or DSLs.

It enables tools to declaratively define resource configurations and safe modifications using **dual-format support** (YAML and TOML) with **Go-native data structures**. Patches modify an existing **base resource**, with advanced **structure preservation** that maintains comments, formatting, and document order.

This forms the foundation for Kure's deterministic, introspectable Kubernetes manifest generation pipeline with sophisticated **intelligent path resolution** and **automatic type inference**.

---

## Design Principles

1. **Dual-format declarative patching** over templating:
   - Support both YAML and TOML patch formats with automatic detection
   - Use single-line syntax to express changes to Kubernetes objects
   - Avoid logic or conditional expressions in patch files

2. **One patch = one operation**:
   - Patches operate on individual fields or list items
   - Operations include: `replace`, `delete`, `insertbefore`, `insertafter`, `append`

3. **Intelligent patch targeting with disambiguation**:
   - Automatically routes patches to matching resources using metadata and field validation
   - Advanced resolution algorithms handle multiple resources with same names
   - Supports `target:` override and `kind.name` disambiguation
   - Smart target matching validates patch compatibility with resource schemas

4. **Structure preservation and introspection**:
   - All paths are parsed and normalized into `PathPart` structs
   - YAML structure preservation maintains comments, formatting, and document order
   - Path syntax is explicitly validated before application
   - Comprehensive debug logging with `KURE_DEBUG=1`

5. **Automatic type inference and conversion**:
   - Intelligent conversion of string values to appropriate Go types (int, bool, etc.)
   - Context-aware type inference based on Kubernetes field patterns
   - Compatible with unstructured.Unstructured requirements

6. **Graceful error handling**:
   - Missing patch targets generate warnings rather than failures
   - Batch processing continues on individual patch failures
   - Comprehensive error context for debugging

7. **Base resource required**:
   - Each patch is applied on top of an existing object loaded from a file or provided programmatically

---

## Core Types

- `PatchOp`: A single parsed patch line (path, value, operation, selector)
- `PathPart`: A structured representation of a patch path segment with match types
- `PatchableAppSet`: Holds resources and their associated patch operations with structure preservation
- `YAMLDocumentSet`: Advanced structure preservation for comments and formatting
- `YAMLDocument`: Individual document with preserved structure and metadata
- `TOMLHeader`: Parsed TOML-style header with intelligent Kubernetes path mapping
- `VariableContext`: Context for variable substitution with values and features

---

## Interfaces and Helpers

### Resource Loading
- `LoadResourcesFromMultiYAML(io.Reader)` — Load 1..n Kubernetes resources
- `LoadResourcesWithStructure(io.Reader)` — Load resources with structure preservation

### Patch Loading  
- `LoadPatchFile(io.Reader)` — Load patches with automatic format detection
- `LoadPatchFileWithVariables(io.Reader, *VariableContext)` — Load with variable substitution
- `LoadYAMLPatchFile(io.Reader, *VariableContext)` — Load YAML format patches
- `LoadTOMLPatchFile(io.Reader, *VariableContext)` — Load TOML format patches

### PatchableAppSet Construction
- `NewPatchableAppSet([]*unstructured.Unstructured, []PatchSpec)` — Create from in-memory objects
- `NewPatchableAppSetWithStructure(*YAMLDocumentSet, []PatchSpec)` — Create with structure preservation
- `LoadPatchableAppSet([]io.Reader, io.Reader)` — Create a complete working set

### Path Processing and Validation
- `NormalizePath()` — Validate and parse patch paths before application
- `ParsePatchPath(string)` — Parse paths into structured PathPart components
- `ParseTOMLHeader(string)` — Parse TOML headers with intelligent path mapping

### Type System and Variable Support
- `inferValueType(key, value string)` — Automatic type inference for Kubernetes compatibility
- `SubstituteVariables(string, *VariableContext)` — Variable substitution with ${values.key} syntax

---

## Format Support

### YAML Format
Supports expressive list modification syntax:

```yaml
# Simple patch map
spec.containers[3].image: nginx:latest        # replace by index
spec.containers[+name=web].image: sidecar:1   # insert after matching item
spec.containers[-]: { name: debug }           # append to list
metadata.labels[delete=app]: ""               # delete field

# Targeted patch list
- target: my-deployment
  patch:
    spec.replicas: 5
    spec.template.metadata.labels.foo: bar
    metadata.labels[delete=app]: ""
```

### TOML Format (Preferred)
Intelligent header-based targeting with automatic path mapping:

```toml
# Basic resource targeting
[deployment.app]
spec.replicas: 3
metadata.labels.env: production

# Container-specific patches with semantic selectors
[deployment.app.containers.name=main]
resources.requests.cpu: 100m
resources.limits.memory: 512Mi
image.tag: "${values.version}"

# Service port configuration  
[service.app.ports.name=https]
port: 443
nodePort: 30443

# Complex array operations
[deployment.app.containers.name=main.env[-]]
name: DEBUG_MODE
value: "true"
```

## Advanced Features

### Automatic Format Detection
The system automatically detects patch format based on content structure:
- TOML format: Detected by presence of `[header]` sections
- YAML format: Detected by YAML structure patterns

### Structure Preservation
- Maintains original YAML comments, formatting, and document order
- Preserves multi-document YAML structure with `---` separators
- Updates only targeted fields while leaving surrounding structure intact

### Intelligent Path Resolution
- Context-aware mapping of TOML sections to Kubernetes paths
- Different behavior based on resource type (Deployment vs Service vs Pod)
- Smart disambiguation for resources with identical names

### Variable Substitution
```toml
[deployment.app.containers.name=main]
image.tag: "${values.version}"
resources.requests.cpu: "${values.cpu_request}"
debug.enabled: "${features.enable_debug}"
```

### Automatic Type Inference
- Converts string values to appropriate Go types based on field context
- Recognizes port numbers, replica counts, timeout values automatically
- Kubernetes-compatible type conversion for unstructured objects

