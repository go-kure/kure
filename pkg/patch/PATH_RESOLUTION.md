# Advanced Path Resolution and Type Inference

This document provides an in-depth technical guide to the sophisticated path resolution and type inference systems in the Kure patch module.

## Path Resolution Architecture

### PathPart Structure

Each patch path is decomposed into structured components:

```go
type PathPart struct {
    Field      string  // The field name (e.g., "containers", "ports")
    MatchType  string  // "", "index", or "key" 
    MatchValue string  // The match criteria ("0", "name=main")
}
```

### Path Parsing Process

The `ParsePatchPath()` function converts dot-notation paths into structured segments:

```
spec.containers[name=main].image
↓
[
  {Field: "spec"},
  {Field: "containers", MatchType: "key", MatchValue: "name=main"},
  {Field: "image"}
]
```

### Context-Aware Resolution

The patch system uses intelligent context-aware resolution:

1. **Resource Type Detection**: Different behavior for Deployments vs Services vs ConfigMaps
2. **Field Validation**: Checks if target fields exist in the resource schema
3. **List vs Object Disambiguation**: Automatically determines if a path targets a list or object field

## TOML Header Intelligence

### Semantic Path Mapping

TOML headers are intelligently mapped to Kubernetes paths:

```toml
[deployment.app.containers.name=main]
# Automatically resolves to: spec.template.spec.containers[name=main]

[service.app.ports.name=http] 
# Automatically resolves to: spec.ports[name=http]

[configmap.config]
# Automatically resolves to: data.*
```

### Header Resolution Algorithm

1. **Resource Identification**: Parse `kind.name` from header
2. **Context Mapping**: Apply resource-specific path transformations
3. **Selector Extraction**: Extract list selectors and match criteria
4. **Path Validation**: Verify the resolved path exists in the target resource

### Resource-Specific Mappings

#### Deployment Resources
- `containers.*` → `spec.template.spec.containers.*`
- `volumes.*` → `spec.template.spec.volumes.*`
- `env.*` → `spec.template.spec.containers[].env.*`

#### Service Resources  
- `ports.*` → `spec.ports.*`
- `selector.*` → `spec.selector.*`

#### ConfigMap/Secret Resources
- Direct field access to `data.*` fields
- Automatic string/binary data type detection

## Type Inference System

### Automatic Type Detection

The `inferValueType()` function performs context-aware type inference:

```go
// Port numbers
"8080" → int64(8080)     // when field contains "port"
"80"   → int64(80)       // when field is "containerPort", "targetPort"

// Resource quantities  
"500m" → "500m"          // preserved as string for Kubernetes resource.Quantity
"1Gi"  → "1Gi"           // preserved as string for memory quantities

// Boolean values
"true"    → true         // when field suggests boolean context
"enabled" → true         // when field is "enabled", "debug", etc.

// Replica counts
"3" → int64(3)           // when field is "replicas", "instances"
```

### Context-Driven Inference

The system considers multiple context clues:

1. **Field Name Patterns**: `port`, `replicas`, `timeout`, `enabled`
2. **Parent Path Context**: `resources.limits.*`, `spec.ports.*`
3. **Value Format Recognition**: Kubernetes quantity formats, duration strings
4. **Resource Type Context**: Different inference rules for different Kubernetes kinds

### Kubernetes Compatibility

All inferred types are compatible with `unstructured.Unstructured`:

```go
func convertValueForUnstructured(value interface{}) interface{} {
    switch v := value.(type) {
    case int:    return int64(v)      // Kubernetes expects int64
    case int32:  return int64(v)      // Convert to standard int64
    case float32: return float64(v)   // Convert to standard float64
    // ... handle maps and slices recursively
    }
}
```

## List Resolution Algorithms

### Index Resolution

The `resolveListIndex()` function handles multiple selector types:

```go
// Numeric index
"2" → index 2

// Negative index (from end)
"-1" → len(list) - 1

// Key-value selector  
"name=main" → find first item where item.name == "main"

// Multi-key selector
"name=web,port=80" → find item matching both criteria
```

### Insertion Logic

Different insertion operations use sophisticated positioning:

```go
// insertBefore: lst = append(lst[:idx], append([]interface{}{value}, lst[idx:]...)...)
// insertAfter:  lst = append(lst[:idx+1], append([]interface{}{value}, lst[idx+1:]...)...)
// append:       lst = append(lst, value)
```

### Bounds Checking

Comprehensive validation prevents index errors:

```go
if i < 0 {
    i = len(list) + i  // Handle negative indices
}
if i < 0 || i > len(list) {
    return -1, fmt.Errorf("index out of bounds: %d", i)
}
```

## Error Handling and Debugging

### Graceful Degradation

The patch system follows a "warn, don't fail" philosophy:

- Missing target resources generate warnings but don't stop processing
- Invalid selectors are logged but don't crash the operation
- Type inference failures fall back to string values

### Debug Logging

Enable comprehensive debugging with `KURE_DEBUG=1`:

```
DEBUG: Resolving TOML header [deployment.app.containers.name=main]
DEBUG: Mapped to path: spec.template.spec.containers[name=main]
DEBUG: Found target container at index 0
DEBUG: Applying patch: image.tag = "v1.2.3"
DEBUG: Type inference: "8080" → int64(8080) (field context: containerPort)
```

### Validation Pipeline

Each patch goes through multiple validation stages:

1. **Syntax Validation**: Parse the patch line syntax
2. **Path Validation**: Verify the target path exists
3. **Type Validation**: Check if the value type is appropriate
4. **Schema Validation**: Ensure compatibility with Kubernetes schemas (future)

## Performance Considerations

### Path Caching

Frequently used paths are cached to avoid repeated parsing:

```go
var pathCache = make(map[string][]PathPart)

func cachedParsePath(path string) []PathPart {
    if cached, ok := pathCache[path]; ok {
        return cached
    }
    // Parse and cache...
}
```

### Batch Processing

Multiple patches are processed efficiently:

1. **Group by Resource**: Batch patches targeting the same resource
2. **Optimize Selectors**: Cache list index resolutions
3. **Minimize Conversions**: Reuse type-converted values

## Integration Points

### Variable Substitution

Path resolution integrates with variable substitution:

```toml
[deployment.${values.app_name}.containers.name=${values.container_name}]
image.tag: "${values.version}"
```

Resolution happens after variable substitution:
1. Substitute variables in headers and values
2. Parse the resolved paths  
3. Apply type inference to final values

### Structure Preservation

Path resolution works seamlessly with YAML structure preservation:

1. **Parse Original Structure**: Maintain comments and formatting
2. **Resolve Target Paths**: Find exact locations in preserved structure
3. **Apply Minimal Changes**: Update only the targeted values
4. **Preserve Everything Else**: Keep original formatting intact

This sophisticated system enables the patch module to handle complex Kubernetes resource modifications while maintaining type safety, performance, and user-friendly error handling.