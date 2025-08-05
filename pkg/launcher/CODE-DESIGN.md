# Launcher Module - Code Design

## Overview

The launcher module is the core engine for the Kurel package system, implementing a declarative approach to generating Kubernetes manifests with validation and customization capabilities. This document captures all design decisions made during the architecture planning phase.

## Design Philosophy

**Core Principle**: "Kurel just generates YAML" - The launcher is a declarative system for generating Kubernetes manifests, not a runtime system or orchestrator.

## Architecture Decisions

### 1. Core Package Structure

**Decision**: Separate `PackageDefinition` and `PackageInstance` pattern

```go
// Immutable package definition
type PackageDefinition struct {
    Path        string
    Metadata    KurelMetadata     // From kurel: key in parameters.yaml
    Parameters  ParameterMap      // Default parameters including global:
    Resources   []Resource        // Base K8s manifests
    Patches     []Patch          // Available patches with metadata
}

// Instance with user customization
type PackageInstance struct {
    Definition  *PackageDefinition  // Immutable package reference
    UserValues  ParameterMap        // User-provided overrides
    Resolved    ParameterMap        // Final resolved values
    LocalPath   string              // Path to .local.kurel if exists
}
```

**Rationale**: 
- Clear separation between immutable package and user state
- Enables processing same package with different configs in parallel
- Package definitions are cacheable and reusable
- More functional style with immutable data + transformations

### 2. Interface Organization

**Decision**: Separate `interfaces.go` file (Option C)

```go
// pkg/launcher/interfaces.go
type Loader interface { ... }
type Resolver interface { ... }
type Builder interface { ... }
type Validator interface { ... }
type SchemaGenerator interface { ... }

// pkg/launcher/types.go
type PackageDefinition struct { ... }
type PackageInstance struct { ... }
```

**Rationale**:
- Clean separation of contracts from data types
- Easy to see all capabilities at a glance
- Follows Go stdlib patterns (like `io` package)

### 3. Package Loading Strategy

**Decision**: Hybrid error handling approach

- **Critical files** (parameters.yaml): Must load successfully or fail immediately
- **Other files** (resources, patches): Collect all errors, load what's possible
- Return partial package with LoadErrors for non-critical failures

```go
func LoadDefinition(path string) (*PackageDefinition, error) {
    // Critical: parameters.yaml MUST load
    params, err := loadYAML("parameters.yaml")
    if err != nil {
        return nil, fmt.Errorf("critical: parameters.yaml: %w", err)
    }
    
    // Best effort for others, collect errors
    var errs []error
    resources, resourceErrs := loadAllResources()
    patches, patchErrs := loadAllPatches()
    
    if len(errs) > 0 {
        return &PackageDefinition{...}, &LoadErrors{Errors: errs}
    }
}
```

**Rationale**:
- Can't proceed without valid parameters.yaml
- See all syntax errors at once for debugging
- Allows partial inspection with `kurel info`

### 4. Variable Resolution

**Decision**: No inline defaults, configurable depth

- Variables must exist in parameters.yaml (no `${var|default}` syntax)
- Configurable maximum nesting depth to prevent infinite recursion
- Parameters.yaml is where all defaults are defined

```go
type variableResolver struct {
    maxDepth int  // Configurable, default 10
}

// Resolution without inline defaults
// ${monitoring.namespace} - ERROR if not defined
// No fallback syntax supported
```

**Rationale**:
- Keeps variable syntax simple
- All defaults in one place (parameters.yaml)
- Prevents infinite recursion while allowing deep nesting

### 5. Patch Processing

**Decision**: Strict validation with verbose logging

- **Conflicts**: Hard error, refuse to continue
- **Auto-enable**: Verbose logging to stderr
- **Missing targets**: Error, patches must match something

```go
// Hard error on conflicts
if hasConflicts(enabledPatches) {
    return nil, fmt.Errorf("conflict: %s and %s cannot both be enabled", p1, p2)
}

// Verbose logging during build
INFO: Enabling patch 10-monitoring.kpatch (monitoring.enabled=true)
INFO: Auto-enabling 05-metrics.kpatch (required by 10-monitoring)

// Error if patch doesn't match
if matchCount == 0 {
    return error("patch targets non-existent resource: deployment.frontend")
}
```

**Rationale**:
- Ensures patches work as expected
- Clear visibility into auto-enabled dependencies
- No silent failures

### 6. Schema Generation

**Decision**: Hybrid approach with package-defined CRD URLs

- Bundle schemas for resources known in internal/ packages
- Package maintainers can specify CRD schema URLs in parameters.yaml
- Auto-generate if missing, allow explicit regeneration

```yaml
# In parameters.yaml
kurel:
  name: my-app
  schemas:
    - https://raw.githubusercontent.com/cert-manager/cert-manager/v1.13.0/deploy/crds/crd-certificates.yaml
    - https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.68.0/deploy/crds/crd-prometheuses.yaml
```

**Rationale**:
- Leverages existing Kure knowledge
- Extensible for custom CRDs
- Balance between convenience and flexibility

### 7. Validation System

**Decision**: Errors block, warnings don't; best-effort K8s validation

```go
type ValidationResult struct {
    Errors   []ValidationError
    Warnings []ValidationWarning
}

// Errors prevent build
if result.HasErrors() {
    return nil, result.Errors()
}

// Warnings are logged but don't block
if result.HasWarnings() {
    logger.Warn(result.Warnings())
}

// Full validation when schemas available
if schema := getK8sSchema(resource); schema != nil {
    validateFull(resource, schema)
} else {
    validateMedium(resource)  // Basic constraints only
}
```

**Rationale**:
- Clear distinction between blocking and non-blocking issues
- Best-effort validation based on available information
- Graceful degradation when schemas unavailable

### 8. Output Generation

**Decision**: No GitOps-specific support, configurable output format

- Kurel only manages `kurel.gokure.dev/` annotations
- GitOps integration handled elsewhere (e.g., stack/generators)
- Configurable output: stdout (default), single file, by-kind, by-resource

```bash
# Default: multi-doc YAML to stdout
kurel build my-app.kurel/

# Output to directory with by-kind grouping
kurel build my-app.kurel/ -o out/ --format=by-kind

# Single file output
kurel build my-app.kurel/ -o manifests.yaml --format=single

# JSON output
kurel build my-app.kurel/ --output-format=json
```

**Rationale**:
- Keeps kurel focused on YAML generation
- Flexible output for different workflows
- Clean separation of concerns

### 9. Local Extensions

**Decision**: Full integration with validation

- Local patches CAN reference package patches in dependencies
- Parameter conflicts are validated for compatibility
- Local extensions can only add, not replace

```yaml
# my-app.local.kurel/patches/50-custom.yaml
requires:
  - "features/10-monitoring.kpatch"  # Can reference package patches
conflicts:
  - "features/20-basic-monitoring.kpatch"  # Can conflict with package

# Parameter override validation
# Error if local changes parameter type/structure incompatibly
```

**Rationale**:
- Allows sophisticated customization
- Prevents breaking changes
- Maintains package integrity

### 10. CLI Integration

**Decision**: YAML to stdout, logs to stderr, multiple output options

```go
// Build command flags
cmd.Flags().StringVarP(&valuesFile, "values", "v", "", "Values file")
cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output path (default: stdout)")
cmd.Flags().StringVar(&outputFormat, "format", "single", "Output format: single|by-kind|by-resource")
cmd.Flags().StringVar(&outputType, "output-format", "yaml", "Output type: yaml|json")
cmd.Flags().BoolVar(&showPatches, "show-patches", false, "Show patch application details")
cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output to stderr")
```

**Output behavior**:
- YAML/JSON to stdout for piping
- Progress/logs to stderr with -v flag
- Default output directory: `out/` (project standard)

**Rationale**:
- Unix philosophy: stdout for data, stderr for logs
- Composable with other tools via piping
- Consistent with project conventions

## Module Organization

```
pkg/launcher/
├── interfaces.go       # All public interfaces
├── types.go           # Core data types
├── loader.go          # Package loading implementation
├── variables.go       # Variable resolution engine
├── patches.go         # Patch discovery and processing
├── schema.go          # Schema generation
├── validator.go       # Validation logic
├── builder.go         # Manifest building and output
├── extensions.go      # Local extension handling
├── errors.go          # Custom error types
└── testdata/         # Test fixtures
    └── packages/     # Sample packages for testing
```

## Error Handling Philosophy

1. **Fail fast** for critical errors (missing parameters.yaml)
2. **Collect errors** for non-critical issues (malformed patches)
3. **Clear error messages** with context and suggestions
4. **Distinguish** between errors (blocking) and warnings (advisory)

## Testing Strategy

- **Unit tests**: Each module tested in isolation
- **Integration tests**: Full package processing flows
- **Table-driven tests**: For validators and resolvers
- **Mock filesystem**: For loader testing
- **Fixture packages**: Real-world package examples in testdata/

## Performance Considerations

1. **Lazy loading**: Load resources only when needed
2. **Caching**: Cache schemas and resolved variables
3. **Parallel processing**: Where safe (e.g., resource validation)
4. **Streaming output**: For large manifest sets

## Security Considerations

1. **Path traversal protection** in package loading
2. **URL validation** for schema URLs
3. **Variable injection prevention** in resolution
4. **Resource validation** against schemas

## Future Extensibility Points

1. **Plugin system** for custom validators
2. **Remote package loading** (git, https)
3. **Package signing** and verification
4. **Advanced patch operations** (JSONPatch, strategic merge)
5. **Dependency resolution** between packages

## Design Constraints

- No templating engines (use patches)
- No runtime operations (just generate YAML)
- No cluster connectivity required
- No package registry dependency
- Deterministic output (same input = same output)