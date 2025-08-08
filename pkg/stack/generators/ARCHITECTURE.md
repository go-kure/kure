# Generators Architecture Guide

## Overview

The generators package implements a versioned, extensible system for generating Kubernetes resources and other artifacts from declarative configurations. This document explains the architectural patterns and conventions used throughout the generators subsystem.

## Directory Structure

```
generators/
├── DESIGN.md           # High-level design documentation
├── ARCHITECTURE.md     # This file - architectural patterns
├── registry.go         # Global registry for all generators
├── interfaces.go       # Common interfaces
├── doc.go             # Package documentation
│
├── appworkload/       # AppWorkload generator
│   ├── v1alpha1.go    # Version v1alpha1 API types and registration
│   └── internal/      # Shared implementation
│       └── appworkload.go
│
├── fluxhelm/          # FluxHelm generator
│   ├── v1alpha1.go    # Version v1alpha1 API types and registration
│   └── internal/      # Shared implementation
│       └── fluxhelm.go
│
└── kurelpackage/      # KurelPackage generator (in development)
    └── v1alpha1.go    # Version v1alpha1 API types and registration
    └── internal/      # (Will be added when implementation is complete)
```

## The Version/Internal Pattern

### Why This Pattern?

Each generator follows a consistent pattern that separates API versioning from implementation:

1. **Version Files** (`v1alpha1.go`, `v1beta1.go`, `v1.go`)
   - Define the YAML/JSON schema as Go structs
   - Register with the GVK system
   - Implement thin wrapper methods
   - Delegate actual work to internal package

2. **Internal Package** (`internal/*.go`)
   - Contains the actual implementation logic
   - Shared across all versions
   - Complex business logic and resource generation
   - Not exposed in public API

### Benefits

#### 1. **Version Evolution**
```go
// v1alpha1.go
type ConfigV1Alpha1 struct {
    Replicas int32 `yaml:"replicas"`
}

// v1beta1.go - can add fields
type ConfigV1Beta1 struct {
    Replicas *int32 `yaml:"replicas"` // Now optional
    Scaling  *ScalingConfig `yaml:"scaling"`
}

// Both use the same internal implementation
func (c *ConfigV1Alpha1) Generate() {
    internal.GenerateResources(...)
}
```

#### 2. **Code Reuse**
The internal package is shared across versions, avoiding duplication:
```go
// internal/appworkload.go
func GenerateResources(config *Config) ([]*client.Object, error) {
    // 500+ lines of complex logic used by all versions
}
```

#### 3. **Clean API Surface**
Version types are pure data structures:
```go
// Clean, declarative API in v1alpha1.go
type ConfigV1Alpha1 struct {
    Workload  WorkloadType
    Replicas  int32
    Container ContainerConfig
}

// Complex logic hidden in internal/
func createDeployment(...) { /* complex logic */ }
func createService(...) { /* complex logic */ }
```

#### 4. **Independent Testing**
```go
// Test API parsing
func TestV1Alpha1Parsing(t *testing.T) { }

// Test implementation logic separately
func TestDeploymentGeneration(t *testing.T) { }
```

## Implementation Examples

### Example 1: AppWorkload Generator

**Structure:**
- `appworkload/v1alpha1.go` - 100 lines of type definitions and registration
- `appworkload/internal/appworkload.go` - 500+ lines of implementation

**The version file (`v1alpha1.go`):**
```go
package appworkload

import (
    "github.com/go-kure/kure/pkg/stack"
    "github.com/go-kure/kure/pkg/stack/generators/appworkload/internal"
)

// API types - clean data structures
type ConfigV1Alpha1 struct {
    Workload  internal.WorkloadType
    Replicas  int32
    Container internal.ContainerConfig
}

// Thin wrapper that delegates to internal
func (c *ConfigV1Alpha1) Generate(app *stack.Application) ([]*client.Object, error) {
    return internal.GenerateResources(&internal.Config{
        Workload:  c.Workload,
        Replicas:  c.Replicas,
        Container: c.Container,
    }, app)
}
```

**The internal implementation (`internal/appworkload.go`):**
```go
package internal

// Actual implementation - complex logic
func GenerateResources(cfg *Config, app *stack.Application) ([]*client.Object, error) {
    var resources []*client.Object
    
    // Complex deployment generation
    switch cfg.Workload {
    case DeploymentWorkload:
        deployment := createDeployment(cfg, app)
        resources = append(resources, &deployment)
    case StatefulSetWorkload:
        sts := createStatefulSet(cfg, app)
        resources = append(resources, &sts)
    }
    
    // Service generation with port mapping
    if len(cfg.Services) > 0 {
        for _, svc := range cfg.Services {
            service := createService(svc, app)
            resources = append(resources, &service)
        }
    }
    
    // Ingress, volumes, etc...
    return resources, nil
}
```

### Example 2: FluxHelm Generator

Similar pattern with different domain logic:

**Version file focuses on API:**
```go
type ConfigV1Alpha1 struct {
    Chart   ChartConfig
    Source  SourceConfig
    Values  interface{}
}
```

**Internal handles Flux-specific logic:**
```go
func GenerateResources(cfg *Config) ([]*client.Object, error) {
    // Generate HelmRelease
    // Handle different source types (Helm, OCI, Git, S3)
    // Process values and dependencies
}
```

### Example 3: KurelPackage Generator (Future)

Currently just has API definitions, but will need internal package for:

```go
// Future internal/kurelpackage.go
package internal

func GeneratePackageFiles(cfg *Config) (map[string][]byte, error) {
    // Read resource files from disk
    // Apply include/exclude patterns
    // Process patches
    // Handle values schemas
    // Build package structure
    // Generate OCI artifacts
}
```

## When to Use Internal Package

### Use Internal Package When:
- Implementation is more than ~100 lines
- Complex business logic exists
- Multiple versions will share the logic
- You need helper functions not part of the API
- Resource generation involves multiple steps

### Don't Use Internal Package When:
- Generator is very simple (< 50 lines)
- It's a prototype or proof of concept
- The entire logic fits cleanly in the Generate method
- No version evolution is expected

## Adding a New Generator

### Step 1: Create the generator directory
```bash
mkdir -p pkg/stack/generators/mygenerator
```

### Step 2: Define v1alpha1 API types
```go
// pkg/stack/generators/mygenerator/v1alpha1.go
package mygenerator

type ConfigV1Alpha1 struct {
    generators.BaseMetadata `yaml:",inline"`
    // Your fields here
}

func init() {
    // Register with GVK system
}

func (c *ConfigV1Alpha1) Generate(app *stack.Application) ([]*client.Object, error) {
    // For simple generators, implement here
    // For complex ones, delegate to internal package
}
```

### Step 3: Add internal package (if needed)
```go
// pkg/stack/generators/mygenerator/internal/mygenerator.go
package internal

func GenerateResources(cfg *Config) ([]*client.Object, error) {
    // Complex implementation here
}
```

### Step 4: Add tests
```go
// pkg/stack/generators/mygenerator/v1alpha1_test.go
func TestMyGeneratorV1Alpha1(t *testing.T) {
    // Test YAML parsing
    // Test generation
}
```

### Step 5: Document
- Update this ARCHITECTURE.md if you introduce new patterns
- Add examples to DESIGN.md
- Include sample YAML in your test files

## Best Practices

1. **Keep version files thin** - They should only define types and delegate
2. **Share types via internal** - Common types go in internal package
3. **Version independence** - Each version can have different fields
4. **Forward compatibility** - Design v1alpha1 with future versions in mind
5. **Comprehensive tests** - Test both API parsing and generation logic
6. **Document YAML schema** - Include examples in tests and comments

## Migration Between Versions

When evolving versions:

```go
// v1alpha1 → v1beta1 migration
func (c *ConfigV1Alpha1) ConvertTo(version string) (interface{}, error) {
    if version == "v1beta1" {
        return &ConfigV1Beta1{
            // Map fields, provide defaults for new fields
        }, nil
    }
    return nil, fmt.Errorf("unsupported version %s", version)
}
```

## Testing Strategy

### 1. API Tests (version files)
- YAML parsing
- Field validation
- Version conversion

### 2. Implementation Tests (internal package)
- Resource generation logic
- Edge cases
- Error handling

### 3. Integration Tests
- End-to-end with actual Kubernetes objects
- Multi-generator scenarios
- Layout generation

## Future Considerations

### Plugin System
Eventually support external generators:
```go
// External generators could register themselves
func RegisterExternalGenerator(gvk GVK, factory GeneratorFactory) {
    externalRegistry.Register(gvk, factory)
}
```

### Code Generation
Consider generating version boilerplate:
```bash
# Future tool
kurel generate generator --name MyGenerator --version v1alpha1
```

## Conclusion

The version/internal pattern provides a clean separation between API versioning and implementation logic. This enables:
- Clean API evolution
- Code reuse across versions
- Testable implementations
- Maintainable codebase

Follow this pattern for all new generators unless there's a compelling reason to deviate.

---

*Last updated: 2025-01-08*