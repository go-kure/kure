# ApplicationConfig Generators Design Document

## Overview

The ApplicationConfig system provides a pluggable architecture for generating Kubernetes resources from different configuration formats. This document describes the design for supporting multiple ApplicationConfig implementations with automatic type detection and versioning.

## Background

The `ApplicationConfig` interface is embedded in the hierarchical structure:
- **Cluster** → contains **Nodes** (tree structure)
- **Node** → contains a **Bundle** and child **Nodes**
- **Bundle** → contains multiple **Applications**
- **Application** → contains an **ApplicationConfig** implementation

Each ApplicationConfig implementation generates Kubernetes resources specific to its type (e.g., AppWorkload, HelmChart, Kustomize).

## Design Goals

1. **Type Safety**: Compile-time checking for each generator type
2. **Extensibility**: Easy addition of new generator types without modifying core interfaces
3. **Version Management**: Support for evolving generator schemas over time
4. **API Readiness**: Dynamic type selection and validation for API consumers
5. **Clean Separation**: Self-contained generator implementations

## Architecture

### GVK Convention

Following Kubernetes' Group, Version, Kind (GVK) pattern for type identification:

```yaml
apiVersion: generators.gokure.dev/v1alpha1
kind: AppWorkload
metadata:
  name: web-app
  namespace: default
spec:
  # ... generator-specific configuration
```

This provides:
- **Group**: `generators.gokure.dev` - namespace for all generators
- **Version**: `v1alpha1`, `v1beta1`, `v1` - schema evolution
- **Kind**: `AppWorkload`, `HelmChart`, `Kustomize` - generator type

### Type Registry

A central registry manages all available ApplicationConfig implementations:

```go
type GVK struct {
    Group   string
    Version string
    Kind    string
}

type ApplicationConfigFactory func() ApplicationConfig

type Registry struct {
    factories map[GVK]ApplicationConfigFactory
    mu        sync.RWMutex
}
```

### Generator Interface Hierarchy

```go
// Core interface - unchanged
type ApplicationConfig interface {
    Generate(*Application) ([]*client.Object, error)
}

// Versioned interface for GVK support
type VersionedConfig interface {
    ApplicationConfig
    GetAPIVersion() string  // Returns "group/version"
    GetKind() string
}

// Metadata interfaces (optional)
type NamedConfig interface {
    GetName() string
    SetName(string)
}

type NamespacedConfig interface {
    GetNamespace() string
    SetNamespace(string)
}
```

## Implementation

### Registry Implementation

Located in `pkg/stack/generators/registry.go`:

```go
package generators

import (
    "fmt"
    "strings"
    "sync"
)

type GVK struct {
    Group   string
    Version string
    Kind    string
}

func (g GVK) String() string {
    return fmt.Sprintf("%s/%s, Kind=%s", g.Group, g.Version, g.Kind)
}

func ParseAPIVersion(apiVersion, kind string) GVK {
    parts := strings.Split(apiVersion, "/")
    if len(parts) == 2 {
        return GVK{
            Group:   parts[0],
            Version: parts[1],
            Kind:    kind,
        }
    }
    // Handle core/v1 style
    return GVK{
        Group:   "",
        Version: parts[0],
        Kind:    kind,
    }
}

type ApplicationConfigFactory func() ApplicationConfig

var (
    registry = &Registry{
        factories: make(map[GVK]ApplicationConfigFactory),
    }
)

type Registry struct {
    factories map[GVK]ApplicationConfigFactory
    mu        sync.RWMutex
}

func (r *Registry) Register(gvk GVK, factory ApplicationConfigFactory) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.factories[gvk] = factory
}

func (r *Registry) Create(gvk GVK) (ApplicationConfig, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    factory, exists := r.factories[gvk]
    if !exists {
        return nil, fmt.Errorf("unknown application config type: %s", gvk)
    }
    return factory(), nil
}

func (r *Registry) ListKinds() []GVK {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    kinds := make([]GVK, 0, len(r.factories))
    for gvk := range r.factories {
        kinds = append(kinds, gvk)
    }
    return kinds
}

// Global registry functions
func Register(gvk GVK, factory ApplicationConfigFactory) {
    registry.Register(gvk, factory)
}

func Create(apiVersion, kind string) (ApplicationConfig, error) {
    gvk := ParseAPIVersion(apiVersion, kind)
    return registry.Create(gvk)
}
```

### Type-Aware Wrapper

Located in `pkg/stack/application_wrapper.go`:

```go
package stack

import (
    "fmt"
    "gopkg.in/yaml.v3"
    "github.com/go-kure/kure/pkg/stack/generators"
)

// ApplicationWrapper provides type detection and unmarshaling
type ApplicationWrapper struct {
    APIVersion string                 `yaml:"apiVersion"`
    Kind       string                 `yaml:"kind"`
    Metadata   ApplicationMetadata    `yaml:"metadata"`
    Spec       generators.ApplicationConfig `yaml:"spec"`
}

type ApplicationMetadata struct {
    Name      string            `yaml:"name"`
    Namespace string            `yaml:"namespace,omitempty"`
    Labels    map[string]string `yaml:"labels,omitempty"`
}

func (w *ApplicationWrapper) UnmarshalYAML(node *yaml.Node) error {
    // First pass: extract GVK
    var gvkDetect struct {
        APIVersion string `yaml:"apiVersion"`
        Kind       string `yaml:"kind"`
    }
    if err := node.Decode(&gvkDetect); err != nil {
        return fmt.Errorf("failed to detect GVK: %w", err)
    }
    
    if gvkDetect.APIVersion == "" || gvkDetect.Kind == "" {
        return fmt.Errorf("apiVersion and kind are required fields")
    }
    
    // Create appropriate config instance
    config, err := generators.Create(gvkDetect.APIVersion, gvkDetect.Kind)
    if err != nil {
        return fmt.Errorf("failed to create config for %s/%s: %w", 
            gvkDetect.APIVersion, gvkDetect.Kind, err)
    }
    
    // Decode full content
    var raw struct {
        APIVersion string                      `yaml:"apiVersion"`
        Kind       string                      `yaml:"kind"`
        Metadata   ApplicationMetadata         `yaml:"metadata"`
        Spec       yaml.Node                   `yaml:"spec"`
    }
    
    if err := node.Decode(&raw); err != nil {
        return fmt.Errorf("failed to decode wrapper: %w", err)
    }
    
    // Decode spec into the specific config type
    if err := raw.Spec.Decode(config); err != nil {
        return fmt.Errorf("failed to decode spec: %w", err)
    }
    
    w.APIVersion = raw.APIVersion
    w.Kind = raw.Kind
    w.Metadata = raw.Metadata
    w.Spec = config
    
    return nil
}

func (w *ApplicationWrapper) ToApplication() *Application {
    app := NewApplication(w.Metadata.Name, w.Metadata.Namespace, w.Spec)
    
    // If the config supports metadata injection, apply it
    if named, ok := w.Spec.(generators.NamedConfig); ok {
        named.SetName(w.Metadata.Name)
    }
    if namespaced, ok := w.Spec.(generators.NamespacedConfig); ok {
        namespaced.SetNamespace(w.Metadata.Namespace)
    }
    
    return app
}
```

### Example Generator Implementations

#### AppWorkload v1alpha1

Located in `pkg/stack/generators/appworkload.go`:

```go
package generators

func init() {
    Register(GVK{
        Group:   "generators.gokure.dev",
        Version: "v1alpha1",
        Kind:    "AppWorkload",
    }, func() ApplicationConfig { return &AppWorkloadConfig{} })
}

// AppWorkloadConfig with GVK support
type AppWorkloadConfig struct {
    // Existing fields...
    Name      string `yaml:"name"`
    Namespace string `yaml:"namespace,omitempty"`
    
    // ... rest of existing implementation
}

func (c *AppWorkloadConfig) GetAPIVersion() string {
    return "generators.gokure.dev/v1alpha1"
}

func (c *AppWorkloadConfig) GetKind() string {
    return "AppWorkload"
}

func (c *AppWorkloadConfig) GetName() string {
    return c.Name
}

func (c *AppWorkloadConfig) SetName(name string) {
    c.Name = name
}

func (c *AppWorkloadConfig) GetNamespace() string {
    return c.Namespace
}

func (c *AppWorkloadConfig) SetNamespace(namespace string) {
    c.Namespace = namespace
}
```

#### HelmChart v1alpha1

Located in `pkg/stack/generators/helmchart.go`:

```go
package generators

import (
    fluxhelm "github.com/fluxcd/helm-controller/api/v2beta1"
)

func init() {
    Register(GVK{
        Group:   "generators.gokure.dev",
        Version: "v1alpha1",
        Kind:    "HelmChart",
    }, func() ApplicationConfig { return &HelmChartConfig{} })
}

type HelmChartConfig struct {
    Name      string                 `yaml:"name"`
    Namespace string                 `yaml:"namespace,omitempty"`
    
    // Helm-specific fields
    Chart      string                 `yaml:"chart"`
    Version    string                 `yaml:"version"`
    Repository string                 `yaml:"repository,omitempty"`
    Values     map[string]interface{} `yaml:"values,omitempty"`
    
    // Advanced options
    CreateNamespace bool              `yaml:"createNamespace,omitempty"`
    Wait            bool              `yaml:"wait,omitempty"`
    Timeout         string            `yaml:"timeout,omitempty"`
    DependsOn       []string          `yaml:"dependsOn,omitempty"`
}

func (h *HelmChartConfig) Generate(app *Application) ([]*client.Object, error) {
    // Generate HelmRelease for Flux or Application for ArgoCD
    // based on the workflow context
}

// Implement VersionedConfig
func (h *HelmChartConfig) GetAPIVersion() string {
    return "generators.gokure.dev/v1alpha1"
}

func (h *HelmChartConfig) GetKind() string {
    return "HelmChart"
}

// Implement NamedConfig and NamespacedConfig
func (h *HelmChartConfig) GetName() string { return h.Name }
func (h *HelmChartConfig) SetName(name string) { h.Name = name }
func (h *HelmChartConfig) GetNamespace() string { return h.Namespace }
func (h *HelmChartConfig) SetNamespace(ns string) { h.Namespace = ns }
```

## Configuration Examples

### AppWorkload Configuration

```yaml
apiVersion: generators.gokure.dev/v1alpha1
kind: AppWorkload
metadata:
  name: web-app
  namespace: production
spec:
  workload:
    type: Deployment
    replicas: 3
  container:
    image: nginx:1.21
    ports:
      - containerPort: 80
        name: http
  service:
    enabled: true
    type: LoadBalancer
```

### HelmChart Configuration

```yaml
apiVersion: generators.gokure.dev/v1alpha1
kind: HelmChart
metadata:
  name: postgresql
  namespace: database
spec:
  chart: postgresql
  version: 12.0.0
  repository: https://charts.bitnami.com/bitnami
  values:
    auth:
      database: myapp
    persistence:
      size: 10Gi
```

### Kustomize Configuration

```yaml
apiVersion: generators.gokure.dev/v1alpha1
kind: Kustomize
metadata:
  name: config-app
  namespace: default
spec:
  path: ./overlays/production
  prune: true
  patches:
    - target:
        kind: Deployment
        name: app
      patch: |
        - op: replace
          path: /spec/replicas
          value: 5
```

## Version Evolution

### Schema Versioning Strategy

1. **v1alpha1**: Initial implementation, API may change
2. **v1beta1**: API stabilizing, backward compatibility within beta
3. **v1**: Stable API, backward compatibility guaranteed

### Version Migration

When updating generator versions:

```go
// Conversion interface for version upgrades
type Convertible interface {
    ConvertTo(version string) (ApplicationConfig, error)
    ConvertFrom(from ApplicationConfig) error
}

// Example: AppWorkloadConfig v1alpha1 to v1beta1
func (c *AppWorkloadConfigV1Alpha1) ConvertTo(version string) (ApplicationConfig, error) {
    switch version {
    case "v1beta1":
        return &AppWorkloadConfigV1Beta1{
            // Map fields from v1alpha1 to v1beta1
        }, nil
    default:
        return nil, fmt.Errorf("unsupported version: %s", version)
    }
}
```

## Benefits

1. **Version Management**: Clear versioning strategy following Kubernetes conventions
2. **Type Discovery**: Automatic detection of generator types from configuration
3. **API Evolution**: Support for backward compatibility and migrations
4. **Extensibility**: New generators can be added without modifying core code
5. **Validation**: Type-specific validation at unmarshal time
6. **Tooling Support**: GVK pattern enables better IDE support and schema validation

## Future Enhancements

1. **Schema Validation**: OpenAPI schema generation for each GVK
2. **Webhook Validation**: Admission webhooks for API validation
3. **CRD Generation**: Generate CRDs for each ApplicationConfig type
4. **Conversion Webhooks**: Automatic version conversion support
5. **Discovery API**: Runtime discovery of available generator types

## Testing Strategy

1. **Unit Tests**: Each generator implementation with its own test suite
2. **Integration Tests**: Cross-generator workflow tests
3. **Version Migration Tests**: Ensure conversions work correctly
4. **Registry Tests**: Validate registration and factory patterns
5. **YAML Parsing Tests**: Comprehensive unmarshal testing

## Migration Path

Since the project is still in development with no releases:

1. **Direct Migration**: Update all existing configurations to use GVK format
2. **Update Examples**: Modify demo and example code to use new format
3. **Generator Updates**: Retrofit existing AppWorkloadConfig with GVK support
4. **Documentation**: Update all documentation with new examples
5. **No Backward Compatibility**: Clean break from old format

## Conclusion

This design provides a robust, extensible system for managing multiple ApplicationConfig implementations with proper versioning and type detection. The GVK convention ensures compatibility with Kubernetes patterns and enables future API evolution.