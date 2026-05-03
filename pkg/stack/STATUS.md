# Stack Module Status

> **Last Updated**: 2025-01-08  
> **Overall Status**: Beta - Core functionality complete, additional features in progress

## Completed Features ✅

### Core Infrastructure
- ✅ **Hierarchical Domain Model**: Cluster → Node → Bundle → Application
- ✅ **GitOps Workflow Support**: Flux and ArgoCD implementations
- ✅ **Layout Generation**: Flexible manifest organization strategies
- ✅ **GVK System**: Full Group/Version/Kind support for all types

### GVK Implementation
- ✅ **GVK Package** (`pkg/gvk`)
  - Generic registry with Go generics
  - Type-safe factory patterns
  - YAML unmarshaling with automatic type detection
  - Version conversion framework
  
- ✅ **ApplicationConfig Generators** (`generators.gokure.dev`)
  - AppWorkload v1alpha1 - Deployments, StatefulSets, DaemonSets
  - FluxHelm v1alpha1 - HelmRelease with multiple source types
  
- ✅ **Stack Types** (`stack.gokure.dev/v1alpha1`)
  - ClusterV1Alpha1 - Cluster configuration with GitOps
  - NodeV1Alpha1 - Hierarchical node structure
  - BundleV1Alpha1 - Application bundles with dependencies

## In Progress 🔄

### Kurel Package Generator
- 🔄 **KurelPackage Generator** (`generators.gokure.dev/v1alpha1`)
  - Generate kurel packages from stack configurations
  - Support for package dependencies and extensions
  - Integration with launcher module for package validation

## Planned Features 📋

### High Priority

#### 1. **Kurel Package Generator** 🎯
Create a new generator that produces kurel packages:
```yaml
apiVersion: generators.gokure.dev/v1alpha1
kind: KurelPackage
metadata:
  name: my-package
  namespace: kurel-system
spec:
  package:
    name: my-app
    version: 1.0.0
    description: "My application package"
  
  resources:
    - source: ./manifests
      includes: ["*.yaml"]
      excludes: ["*-test.yaml"]
  
  patches:
    - target:
        kind: Deployment
        name: my-app
      patch: |
        - op: replace
          path: /spec/replicas
          value: 3
  
  values:
    schema: ./values.schema.json
    defaults: ./values.yaml
  
  extensions:
    - name: monitoring
      when: .Values.monitoring.enabled
      resources:
        - source: ./monitoring
  
  dependencies:
    - name: base-config
      version: ">=1.0.0"
```

**Implementation Tasks:**
- [ ] Create `pkg/stack/generators/kurelpackage/v1alpha1.go`
- [ ] Implement spec types for package metadata, resources, patches
- [ ] Add values schema support
- [ ] Implement extension conditions
- [ ] Create package dependency resolution
- [ ] Generate kurel.yaml and package structure
- [ ] Add validation against launcher module
- [ ] Write comprehensive tests
- [ ] Add integration with `kurel build` command

### Medium Priority

#### 2. **Additional Generators**
- [ ] **CronJobGenerator** - Kubernetes CronJob resources
- [x] **ConfigMapGenerator** - ConfigMaps from files/literals (#473)
- [ ] **SecretGenerator** - Secrets with encoding support
- [ ] **NetworkPolicyGenerator** - Network policies
- [ ] **KustomizationGenerator** - Flux Kustomizations

#### 3. **Version Migration**
- [ ] Implement `Convertible` interface for all types
- [ ] Add conversion webhooks
- [ ] Create migration paths (v1alpha1 → v1beta1 → v1)
- [ ] Version compatibility matrix

#### 4. **CLI Integration**
- [ ] `kurel validate` - Validate GVK resources
- [ ] `kurel convert` - Convert between versions
- [ ] `kurel list-kinds` - List registered types
- [ ] `kurel generate` - Generate resources from GVK

### Low Priority

#### 5. **Schema Validation**
- [ ] OpenAPI schema generation
- [ ] Runtime validation rules
- [ ] Custom validation functions
- [ ] Schema documentation generation

#### 6. **Registry Enhancements**
- [ ] Plugin system for external generators
- [ ] Type deprecation warnings
- [ ] Type aliases
- [ ] Registry introspection API

#### 7. **Performance Optimizations**
- [ ] Parsing cache
- [ ] Lazy generator loading
- [ ] Parallel YAML processing
- [ ] Memory usage optimization

## Known Issues 🐛

1. **Limited Error Context**: YAML parsing errors don't include line numbers
2. **No Version Negotiation**: Can't automatically upgrade old configs
3. **Missing Validation**: No schema validation for generator configs

## Documentation Needs 📚

- [ ] User guide for writing GVK YAML
- [ ] Generator development guide
- [ ] API reference documentation
- [ ] Migration guide from old format
- [ ] Example configurations

## Testing Coverage 🧪

Current coverage:
- ✅ Unit tests for all GVK types
- ✅ Integration tests for generators
- ✅ YAML parsing tests
- ⏳ Multi-version migration tests
- ⏳ Performance benchmarks
- ⏳ Fuzz testing

## Dependencies 📦

Key dependencies that need monitoring:
- `sigs.k8s.io/controller-runtime` - v0.19.3
- `github.com/fluxcd/pkg/apis` - v0.38.0
- `gopkg.in/yaml.v3` - v3.0.1

## Contributing 🤝

Priority areas for contribution:
1. Kurel package generator implementation
2. Additional generator types
3. Documentation and examples
4. Test coverage improvements
5. Performance optimizations

## Release Milestones 🚀

### v0.9.0 (Current)
- ✅ Core GVK infrastructure
- ✅ Basic generators (AppWorkload, FluxHelm)
- ✅ Stack versioning

### v0.10.0 (Next)
- [ ] Kurel package generator
- [ ] Version migration support
- [ ] CLI integration

### v1.0.0 (Target)
- [ ] All planned generators
- [ ] Complete documentation
- [ ] Production-ready validation
- [ ] Performance optimized

---

*This status document is updated regularly to reflect the current state of development.*