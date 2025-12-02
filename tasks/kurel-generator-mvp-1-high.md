# Task: Finish KurelPackage Generator MVP

**Priority:** 1 - High (Short Term: 2-4 weeks)
**Category:** kurel
**Status:** In Progress
**Dependencies:** None
**Blocked By:** None

---

## Overview

Implement the core functionality of the KurelPackage generator to enable users to author `KurelPackage` YAML configurations and automatically generate kurel package structures.

## Current Status

The KurelPackage generator is **scaffolded but not implemented**. Key functions contain TODO markers:

**File:** `pkg/stack/generators/kurelpackage/v1alpha1.go`

```go
// Line 167 - TODO: Implement resource gathering
// Line 170 - TODO: Implement patch generation
// Line 174 - TODO: Implement values generation
// Line 179 - TODO: Implement extension processing
// Line 187 - TODO: Implement kurel.yaml generation
// Line 209 - TODO: Add more validation
```

## Objectives

Implement `GeneratePackageFiles` method with the following capabilities:

1. **Resource Gathering** - Collect Kubernetes resources from Application configs
2. **Patch Generation** - Generate `.kpatch` files from application configurations
3. **Values Generation** - Extract parameters into `values.yaml`
4. **Extension Processing** - Handle custom extensions if defined
5. **Package Manifest** - Generate `kurel.yaml` with metadata
6. **Validation** - Comprehensive validation of package structure

## Implementation Tasks

### 1. Resource Gathering (Line 167)

```go
func (c *Config) gatherResources(app *stack.Application) ([]*client.Object, error) {
    // Generate resources from ApplicationConfig
    resources, err := app.Config.Generate(app)
    if err != nil {
        return nil, errors.Wrap(err, "failed to generate resources")
    }

    // Apply any transformations based on KurelPackage config
    // (e.g., inject labels, annotations)

    return resources, nil
}
```

**Acceptance Criteria:**
- Resources generated from all Application configs
- Resources include proper labels/annotations
- Error handling for invalid configs

### 2. Patch Generation (Line 170)

```go
func (c *Config) generatePatches() ([]patch.Patch, error) {
    // Extract configurable fields from resources
    // Generate patch files for each parameter
    // Support conditional patches based on features

    var patches []patch.Patch

    // Iterate through Parameters and create patches
    for key, param := range c.Parameters {
        p := patch.Patch{
            Target: param.Path,  // JSONPath to field
            Op:     "replace",
            Value:  fmt.Sprintf("${values.%s}", key),
        }
        patches = append(patches, p)
    }

    return patches, nil
}
```

**Acceptance Criteria:**
- Patches generated for all parameters
- Support for list operations (append, insert)
- Variable substitution syntax (${values.key})

### 3. Values Generation (Line 174)

```go
func (c *Config) generateValues() (map[string]interface{}, error) {
    values := make(map[string]interface{})

    for key, param := range c.Parameters {
        values[key] = param.Default
    }

    return values, nil
}
```

**Acceptance Criteria:**
- Extract all parameters with defaults
- Nested structure support
- Type preservation (string, int, bool, etc.)

### 4. Extension Processing (Line 179)

```go
func (c *Config) processExtensions() error {
    // Process custom extensions if defined
    // This is optional - can be implemented later

    if len(c.Extensions) == 0 {
        return nil
    }

    for _, ext := range c.Extensions {
        // Handle extension-specific logic
    }

    return nil
}
```

**Acceptance Criteria:**
- No-op if extensions empty
- Framework for future extension support

### 5. Kurel.yaml Generation (Line 187)

```go
func (c *Config) generateKurelManifest() (*launcher.PackageDefinition, error) {
    return &launcher.PackageDefinition{
        Metadata: launcher.KurelMetadata{
            Name:        c.Metadata.Name,
            Version:     c.Version,
            Description: c.Description,
        },
        Parameters: c.Parameters,
        Phases:     c.Phases,
        Conditions: c.Conditions,
    }, nil
}
```

**Acceptance Criteria:**
- Complete kurel.yaml structure
- All metadata fields populated
- Valid YAML output

### 6. Enhanced Validation (Line 209)

```go
func (c *Config) Validate() error {
    validator := validation.NewValidator()

    // Validate required fields
    if c.Metadata.Name == "" {
        return errors.NewValidationError("name", "", "KurelPackage", nil)
    }

    // Validate parameters
    for key, param := range c.Parameters {
        if param.Type == "" {
            return errors.NewValidationError("type", "", key, []string{"string", "int", "bool"})
        }
    }

    // Validate resources exist
    if len(c.Resources) == 0 {
        return errors.New("KurelPackage must have at least one resource")
    }

    return nil
}
```

**Acceptance Criteria:**
- Validate all required fields
- Parameter type checking
- Resource existence validation

## Testing Requirements

1. **Unit Tests** - Test each function independently
2. **Integration Test** - End-to-end package generation
3. **Golden Tests** - Compare generated output to expected structure

```go
// Example test structure
func TestGeneratePackageFiles(t *testing.T) {
    config := &Config{
        Metadata: ApplicationMetadata{Name: "test-package"},
        Version: "v1.0.0",
        // ... full config
    }

    // Generate package
    pkg, err := config.GeneratePackageFiles()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    // Validate structure
    if pkg.Metadata.Name != "test-package" {
        t.Errorf("expected name 'test-package', got %s", pkg.Metadata.Name)
    }
}
```

## Files to Modify

1. `pkg/stack/generators/kurelpackage/v1alpha1.go` - Core implementation
2. `pkg/stack/generators/kurelpackage/v1alpha1_test.go` - Add tests
3. `examples/generators/kurelpackage.yaml` - Add example usage

## Definition of Done

### Code Artifacts
- [ ] All TODO markers removed from `pkg/stack/generators/kurelpackage/v1alpha1.go`
- [ ] `GeneratePackageFiles()` returns complete file map (kurel.yaml, parameters.yaml, resources/, patches/)
- [ ] Writes package to disk via `WritePackageToDisk(outputDir)` method
- [ ] Resource gathering implemented and tested
- [ ] Patch generation working with variable substitution (`${values.key}`)
- [ ] Values.yaml generated with defaults and type preservation
- [ ] kurel.yaml manifest generated with all metadata
- [ ] Comprehensive validation added (required fields, parameter types, resource existence)

### Testing
- [ ] Unit tests for each function (gatherResources, generatePatches, generateValues, generateKurelManifest)
- [ ] Integration test demonstrating full workflow (YAML → Package)
- [ ] Golden test comparing generated output to expected structure
- [ ] Test coverage > 80% for kurelpackage package

### Documentation
- [ ] Example KurelPackage YAML in `examples/generators/kurelpackage.yaml`
- [ ] Sample generated package in `examples/kurel/generated-example/`
- [ ] GoDoc comments on all public functions
- [ ] README in `examples/kurel/` explaining generator usage

### Out of Scope
- OCI packaging (tracked in kurel-oci-publishing-3-future.md)
- Interactive mode
- Migration from v1alpha1 to v1beta1

### Acceptance Test
```bash
# Generate package from KurelPackage YAML
go run ./cmd/kurel build --from-generator examples/generators/kurelpackage.yaml --output /tmp/test-pkg

# Verify structure
test -f /tmp/test-pkg/kurel.yaml
test -f /tmp/test-pkg/parameters.yaml
test -d /tmp/test-pkg/resources
test -d /tmp/test-pkg/patches

# Build package
go run ./cmd/kurel build /tmp/test-pkg --output /tmp/manifests

# Should produce valid Kubernetes YAML
kubectl --dry-run=client apply -f /tmp/manifests
```

## References

- Existing generators: `pkg/stack/generators/appworkload/`, `pkg/stack/generators/fluxhelm/`
- Patch system: `pkg/patch/`
- Launcher types: `pkg/launcher/types.go`
- Example package: `examples/kurel/frigate/`

## Estimated Effort

**3-5 days** for experienced Go developer familiar with the codebase.
