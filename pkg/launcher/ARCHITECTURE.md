# Launcher Module Architecture

## Overview

The launcher module is a declarative Kubernetes manifest generation system that processes Kurel packages through a well-defined pipeline. It follows clean architecture principles with clear separation of concerns and interface-driven design.

## Core Design Principles

1. **Interface-Driven Design**: All major components are defined as interfaces, enabling testability and flexibility
2. **Immutability**: Core data structures use deep copy patterns to ensure thread safety
3. **Hybrid Error Handling**: Distinguishes between blocking errors and non-blocking warnings
4. **Performance Optimization**: Uses caching, memoization, and efficient algorithms
5. **Thread Safety**: All components are safe for concurrent use

## Architecture Layers

```
┌─────────────────────────────────────────┐
│           CLI/API Layer                 │
├─────────────────────────────────────────┤
│          Launcher Pipeline              │
│  ┌────────────────────────────────┐     │
│  │ Load → Resolve → Patch → Valid │     │
│  └────────────────────────────────┘     │
├─────────────────────────────────────────┤
│         Core Components                 │
│  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐  │
│  │Loader│ │Resolv│ │Patch │ │Valid │  │
│  └──────┘ └──────┘ └──────┘ └──────┘  │
├─────────────────────────────────────────┤
│          Data Structures                │
│  PackageDefinition, Resource, Patch     │
├─────────────────────────────────────────┤
│         Foundation Layer                │
│  errors, logger, io utilities           │
└─────────────────────────────────────────┘
```

## Component Architecture

### 1. Package Loader
**Responsibility**: Load and parse Kurel packages from disk

```go
type PackageLoader interface {
    LoadDefinition(ctx context.Context, path string, opts *LauncherOptions) (*PackageDefinition, error)
    LoadInstance(ctx context.Context, def *PackageDefinition, userValues ParameterMap) (*PackageInstance, error)
}
```

**Key Features**:
- YAML parsing with strict validation
- Resource discovery and loading
- Patch file detection
- Parameter merging with precedence

### 2. Variable Resolver
**Responsibility**: Resolve variable substitutions in parameters

```go
type Resolver interface {
    ResolveVariables(ctx context.Context, instance *PackageInstance) (*ResolvedPackage, error)
    ResolveValue(ctx context.Context, value interface{}, params ParameterMap) (interface{}, error)
}
```

**Key Features**:
- Recursive variable substitution
- Cycle detection using DFS
- Memoization for performance
- Configurable depth limits

### 3. Patch Processor
**Responsibility**: Apply patches to resources with dependency resolution

```go
type PatchProcessor interface {
    ProcessPatches(ctx context.Context, def *PackageDefinition) (*PackageDefinition, error)
    ApplyPatch(ctx context.Context, resource Resource, patch Patch) (Resource, error)
}
```

**Key Features**:
- Topological sort for dependencies
- Conflict detection
- JSONPath-based patching
- Conditional patch application

### 4. Schema Generator
**Responsibility**: Generate JSON schemas for validation

```go
type SchemaGenerator interface {
    GeneratePackageSchema(ctx context.Context) (*JSONSchema, error)
    GenerateResourceSchema(ctx context.Context, gvk schema.GroupVersionKind) (*JSONSchema, error)
}
```

**Key Features**:
- Automatic schema generation
- Type inference from values
- Field usage tracing
- Schema caching

### 5. Validator
**Responsibility**: Validate packages against schemas and business rules

```go
type Validator interface {
    ValidatePackage(ctx context.Context, def *PackageDefinition) (*ValidationResult, error)
    ValidateResource(ctx context.Context, resource Resource) (*ValidationResult, error)
}
```

**Key Features**:
- Schema-based validation
- Semantic validation
- Error/warning distinction
- Strict mode support

## Data Flow

```mermaid
graph LR
    A[YAML Files] --> B[Loader]
    B --> C[PackageDefinition]
    C --> D[Resolver]
    D --> E[ResolvedPackage]
    E --> F[PatchProcessor]
    F --> G[PatchedResources]
    G --> H[Validator]
    H --> I[ValidatedPackage]
    I --> J[OutputBuilder]
    J --> K[Manifests]
```

## Error Handling Strategy

The module uses a sophisticated error handling approach:

1. **Error Types**: All errors use `github.com/go-kure/kure/pkg/errors` for stack traces
2. **Error Categories**:
   - **Blocking Errors**: Stop processing immediately
   - **Warnings**: Collected and reported but don't block
3. **Validation Levels**:
   - **Error**: Must be fixed before proceeding
   - **Warning**: Should be addressed but not critical
   - **Info**: Informational messages

## Performance Optimizations

### Current Optimizations

1. **Schema Caching**: Schemas are cached to avoid regeneration
2. **Variable Memoization**: Resolved variables are cached
3. **Efficient Algorithms**: O(n) cycle detection, topological sort
4. **Lazy Evaluation**: Resources loaded on-demand

### Performance Targets

- Full package build: < 1 second
- Package validation: < 100ms
- Variable resolution: < 50ms
- Patch application: < 200ms

## Thread Safety

All components ensure thread safety through:

1. **Immutability**: Core types use deep copy
2. **Mutexes**: `sync.RWMutex` for shared state
3. **Stateless Operations**: Most operations are stateless
4. **Concurrent Processing**: Safe for parallel execution

## Testing Strategy

### Coverage Goals
- Target: 80%+ coverage
- Current: 74.1% coverage

### Test Categories
1. **Unit Tests**: Component isolation
2. **Integration Tests**: Pipeline validation
3. **Table-Driven Tests**: Comprehensive scenarios
4. **Benchmarks**: Performance validation

## Security Considerations

1. **No Secret Storage**: Secrets referenced, never stored
2. **Path Traversal Protection**: Validated file paths
3. **Input Validation**: All inputs sanitized
4. **Resource Limits**: Memory and depth limits

## Future Enhancements

### Short Term
- [ ] Increase test coverage to 80%+
- [ ] Add context cancellation checks
- [ ] Implement custom error types
- [ ] Add performance benchmarks

### Medium Term
- [ ] Plugin architecture for validators
- [ ] Metrics and observability
- [ ] Connection pooling
- [ ] Advanced caching strategies

### Long Term
- [ ] Distributed processing
- [ ] Real-time validation
- [ ] AI-assisted error resolution
- [ ] GitOps integration

## Code Quality Metrics

| Metric | Current | Target | Status |
|--------|---------|--------|--------|
| Test Coverage | 74.1% | 80% | 🟡 |
| Cyclomatic Complexity | Low-Med | Low | ✅ |
| Code/Test Ratio | 1.56 | < 2.0 | ✅ |
| Documentation | Good | Excellent | 🟡 |
| Performance | Good | Excellent | 🟡 |

## Dependencies

### Core Dependencies
- `k8s.io/apimachinery`: Kubernetes API machinery
- `sigs.k8s.io/controller-runtime`: Controller utilities
- `gopkg.in/yaml.v3`: YAML processing
- `github.com/go-kure/kure/pkg/errors`: Error handling
- `github.com/go-kure/kure/pkg/logger`: Logging

### Version Management
- Go 1.21+ required
- Kubernetes API v0.28+
- Strict dependency versioning

## Best Practices

1. **Always use kure/errors package** for error creation
2. **Check context cancellation** in long operations
3. **Use defer for cleanup** operations
4. **Document all public APIs** with examples
5. **Maintain backward compatibility** in public interfaces
6. **Write table-driven tests** for comprehensive coverage
7. **Use interfaces** for testability and flexibility