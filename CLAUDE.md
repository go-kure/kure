# Claude Instructions for go-kure/kure Repository

## Project Context

This is **Kure**, a Go library for programmatically building Kubernetes resources used by GitOps tools (Flux, cert-manager, MetalLB, External Secrets). The library emphasizes strongly-typed object construction over templating engines.

## Key Repository Information

### Architecture Overview
- **Domain Model**: Hierarchical structure (Cluster → Node → Bundle → Application)
- **Builder Pattern**: Constructor functions (`Create*`) and helpers (`Add*`, `Set*`)
- **GitOps Agnostic**: Supports both Flux and ArgoCD workflows
- **No Templating**: Uses typed builders instead of string templates

### Package Structure
```
internal/          # Resource builders (kubernetes, fluxcd, certmanager, metallb, externalsecrets)
pkg/stack/         # Core domain model (Cluster, Node, Bundle, Application)
pkg/stack/layout/  # Manifest organization and directory structure
pkg/patch/         # JSONPath-based declarative patching system
pkg/io/            # YAML serialization utilities
pkg/fluxcd/        # Public API for Flux resources
cmd/demo/          # Comprehensive examples and demos
examples/          # Sample configurations
```

### Development Workflow

#### Testing
```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Test specific package
go test ./internal/kubernetes
```

#### Code Quality
- Uses **Qodana** static analysis (configured in `qodana.yaml`)
- **52 test files** with comprehensive coverage
- GitHub Actions CI/CD pipeline
- Go version: **1.24.6**

#### Examples & Demos
```bash
# Run comprehensive demo showing all features
go run ./cmd/demo

# Run with specific flags
go run ./cmd/demo -internals  # Internal package demos
go run ./cmd/demo -app-workload  # AppWorkload example
go run ./cmd/demo -cluster  # Cluster example
```

## Common Tasks

### Adding New Resource Builders
1. Create constructor function: `func Create<ResourceType>(name, namespace string, ...) *<Type>`
2. Add helper functions: `func Add<ResourceType><Field>(...)`
3. Add setter functions: `func Set<ResourceType><Field>(...)`
4. Include comprehensive unit tests in `*_test.go`
5. Follow existing patterns in `internal/` packages

### Extending Domain Model
- Modify `pkg/stack/` for core abstractions
- Update workflow implementations in `pkg/stack/fluxcd/` and `pkg/stack/argocd/`
- Ensure layout generation works in `pkg/stack/layout/`

### Adding Patch Operations
- Extend `pkg/patch/` for new patch types
- Follow JSONPath conventions
- Add tests for path resolution and operations

## Code Conventions

### Function Naming
- **Constructors**: `Create<ResourceType>()`
- **Adders**: `Add<ResourceType><Field>()`
- **Setters**: `Set<ResourceType><Field>()`
- **Helpers**: Descriptive names for utilities

### Error Handling
- Return errors explicitly, don't panic
- Use `fmt.Errorf()` for wrapping errors
- Check for nil pointers before operations

### Testing Patterns
```go
func TestCreate<ResourceType>(t *testing.T) {
    obj := Create<ResourceType>("test", "default")
    if obj == nil {
        t.Fatal("expected non-nil object")
    }
    // Validate required fields...
}

func Test<ResourceType>Helpers(t *testing.T) {
    obj := Create<ResourceType>("test", "default")
    // Test all helper functions...
}
```

### Documentation
- Add package documentation in `doc.go` files
- Use GoDoc conventions for function comments
- Include examples in function documentation

## Security Considerations

### Secret Management
- **Never hardcode secrets** in builders
- Always reference secrets through Kubernetes Secret objects
- Use `SecretKeySelector` and `LocalObjectReference` patterns
- Example:
  ```go
  key := cmmeta.SecretKeySelector{
      LocalObjectReference: cmmeta.LocalObjectReference{Name: "secret-name"},
      Key: "key-name",
  }
  ```

### Certificate Management
- Use cert-manager builders for TLS certificates
- Support ACME challenges (HTTP01, DNS01)
- Handle multiple DNS providers (Cloudflare, Route53, CloudDNS)

### RBAC
- Always use least-privilege principles
- Provide granular role and binding builders
- Test RBAC configurations thoroughly

## Integration Patterns

### Flux Integration
```go
// Create Kustomization
ks := fluxcd.CreateKustomization("app", "default", kustv1.KustomizationSpec{
    Path: "./manifests",
    SourceRef: kustv1.CrossNamespaceSourceReference{
        Kind: "GitRepository",
        Name: "repo",
    },
})
```

### ArgoCD Integration
```go
// Use workflow pattern
wf := argocd.NewWorkflow()
apps, err := wf.Cluster(cluster)
```

### Layout Generation
```go
rules := layout.LayoutRules{
    BundleGrouping:      layout.GroupFlat,
    ApplicationGrouping: layout.GroupFlat,
}
ml, err := layout.WalkCluster(cluster, rules)
```

## Troubleshooting

### Common Issues
1. **Import Errors**: Check `go.mod` for correct versions
2. **Test Failures**: Ensure all required fields are set in constructors
3. **Layout Issues**: Verify LayoutRules configuration
4. **Patch Problems**: Check JSONPath syntax and target existence

### Debugging Tips
- Use `go run ./cmd/demo` to see generated YAML
- Check test output for validation errors
- Verify Kubernetes API versions in dependencies

## Git Workflow

### Branches
- Main branch: `main`
- Create feature branches for new functionality
- Use descriptive commit messages

### Before Committing
1. Run tests: `go test ./...`
2. Run demo: `go run ./cmd/demo`
3. Check formatting: `go fmt ./...`
4. Verify builds: `go build ./...`

## Dependencies

### Core Dependencies
- Kubernetes client libraries (v0.33.2)
- Flux controller APIs (v1.x)
- cert-manager (v1.16.2)
- External Secrets (v0.18.2)
- MetalLB (v0.15.2)
- controller-runtime for Kubernetes integration

### Version Management
- Go 1.24.6 required
- Use `go mod tidy` to clean dependencies
- Pin specific versions for stability

## Best Practices

1. **Type Safety**: Always use strongly-typed builders
2. **Composability**: Design for reusable components
3. **Testing**: Test all public functions thoroughly
4. **Documentation**: Keep docs up-to-date with code changes
5. **Examples**: Update demo code when adding features
6. **Security**: Never expose sensitive data in logs or examples
7. **Compatibility**: Maintain backward compatibility in public APIs

## References

- [API Documentation](https://pkg.go.dev/github.com/go-kure/kure)
- [Design Documents](pkg/*/DESIGN.md)
- [Examples](examples/)
- [Demo Code](cmd/demo/main.go)

## Claude Memories

- kurel just generates YAML
- always implement errors via the kure/errors package; fix this when encountering otherwise
- allow running all possible test commands and file analysis commands (like grep, sed, ..) without asking

## Project Status (as of 2025-08-15)

### Current State
- **Working Tree**: Clean on main branch, all tests passing
- **Codebase**: 214+ Go source files, 76+ test files
- **Test Status**: All tests passing (100% success rate)
- **Code Quality**: 0 TODO/FIXME comments, recent linting improvements

### Recent Achievements
- ✅ **CI/CD Pipeline Implementation** - Comprehensive Makefile and GitHub Actions workflow
- ✅ **Linting Infrastructure** - golangci-lint compatibility and issue resolution
- ✅ **Test Stability** - Fixed intermittent test failures and stdout capture issues
- ✅ **GVK Versioning System** - Kubernetes-style API versioning for stack layers
- ✅ **KurelPackage Generator Framework** - Basic structure implemented, needs completion
- ✅ **Comprehensive Test Coverage** - 102 test cases with benchmarks across core packages

### Architecture Highlights
- **Hierarchical domain model** fully implemented (Cluster → Node → Bundle → Application)
- **Generator system** with ApplicationConfig interface and registry pattern
- **No templating approach** maintained throughout - pure Go builders
- **GitOps dual support** for both Flux and ArgoCD workflows
- **Patching system** with JSONPath support operational
- **Layout management** for flexible manifest organization
- **API Versioning** with stack.gokure.dev/v1alpha1 GVK pattern

### Test Coverage Status
Packages with tests (all passing):
- `internal/`: certmanager, externalsecrets, fluxcd, gvk, kubernetes, metallb
- `pkg/`: errors, io, launcher, patch, stack (including generators, v1alpha1)
- Packages without tests: mainly CLI/cmd packages

### Available Examples
- app-workloads, bootstrap, clusters, generators
- kurel package examples, multi-oci, patches

### Key Metrics
- Go version: 1.24.6
- No outstanding technical debt (0 TODOs)
- Clean commit history with descriptive messages  
- Well-documented with DESIGN.md and ARCHITECTURE.md files
- Performance: 72ns config creation, 1.4μs tree conversion
- CI/CD: GitHub Actions with comprehensive Makefile
- Code Quality: golangci-lint integration, Qodana analysis
- Recent commits: linting fixes, test stability, CI/CD setup

## Known Features Left to Implement

### ❌ POSTPONED: ArgoCD Bootstrap Implementation
**Status**: OUT OF SCOPE - Deferred indefinitely
**Location**: `pkg/stack/argocd/argo.go`
- ArgoCD namespace setup and CRDs
- App repository configuration and RBAC
- sealed-secrets integration
- *Note: Core architecture remains dual-support capable*

### 🎯 Priority 1: KurelPackage Generator Completion
**Status**: HIGH IMPACT - Enables CLI functionality
**Location**: `pkg/stack/generators/kurelpackage/v1alpha1.go`
**Effort**: Medium (1-2 days)
- Resource gathering from filesystem
- Patch generation for resources
- Values file generation with schema support
- Extension processing for conditional features
- Complete kurel.yaml generation
- Additional validation (version format, resource path existence)

### ⚡ Priority 2: Kurel CLI K8s Schema Inclusion
**Status**: QUICK WIN - Easy implementation
**Location**: `pkg/cmd/kurel/cmd.go`
**Effort**: Extra Small (< 1 hour)
- Include Kubernetes schema in generated JSON schemas
- Currently commented out with TODO flag

### 🔧 Priority 3: Testing Coverage Gaps
Packages lacking tests:
- `pkg/cli`
- `pkg/cmd/kure` and subpackages
- `pkg/cmd/kurel`
- `pkg/kubernetes` public API
- `pkg/stack/argocd`
- `pkg/stack/generators/appworkload`
- `pkg/stack/generators/fluxhelm`

### 🚀 Priority 4: Potential Enhancements
**Status**: Future considerations
- Additional generator types
- More external secret providers  
- Extended MetalLB configuration options
- Enhanced patch operations beyond JSONPath
- Fluent Builders Phase 1 (major UX improvement)

## Recommended Development Path

Based on current project state and impact analysis:

### Phase 1: Core CLI Functionality (Immediate - Next 1-2 weeks)
1. **Complete KurelPackage Generator** (Priority 1) 
   - Enables full kurel CLI functionality
   - High user impact for package management
   - Estimated effort: 1-2 days

2. **Enable K8s Schema Inclusion** (Priority 2)
   - Quick win requiring minimal code changes
   - Improves CLI schema validation
   - Estimated effort: < 1 hour

### Phase 2: Quality & Reliability (Next 2-4 weeks)  
3. **Fill Testing Coverage Gaps** (Priority 3)
   - Add tests for CLI packages and ArgoCD workflow
   - Improves overall project reliability
   - Estimated effort: 3-5 days

4. **Fluent Builders Implementation** (Optional)
   - Major UX improvement for library users
   - Large effort but high long-term value
   - Estimated effort: 1-2 weeks

### Phase 3: Future Enhancements (2+ months)
5. **Extended Generator Types** - Additional resource generators
6. **Enhanced Patching System** - Beyond JSONPath operations  
7. **ArgoCD Bootstrap** - If priorities change and demand emerges

**Current Recommendation**: Start with **KurelPackage Generator completion** as it provides the highest immediate impact for CLI users.

## Configuration Management Design Decisions

### Builder Pattern - Immutable
- All configuration builders will follow an **immutable pattern**
- Each `With*` method returns a new builder instance, leaving the original unchanged
- Enables configuration branching and composition from common base configurations
- Thread-safe and follows functional programming patterns
- Example:
  ```go
  base := stack.NewClusterBuilder("prod").WithNode("shared")
  dev := base.WithNode("dev-apps").Build()      // base unchanged
  staging := base.WithNode("staging-apps").Build() // base unchanged
  ```

### Partial Configurations - TBD
- Decision on handling partial/incomplete configurations is **deferred**
- Open questions include: serialization of partial configs, validation timing, fragment composition
- For now, builders must be completed in one flow

### Strict Mode - Out of Scope
- Kure will **not enforce** opinionated best practices or strict validation rules
- Maintains library philosophy of being unopinionated and flexible
- Organizations can implement their own validation layers on top of Kure
- Users have full control over their Kubernetes patterns and practices

### ArgoCD Support - Postponed
- ArgoCD Bootstrap implementation is **temporarily out of scope**
- Core architecture maintains dual GitOps tool support capability
- Focus shifted to completing CLI functionality and testing coverage
- Can be revisited if user demand or priorities change

### Configuration Management Features to Implement

#### Phase 1: Fluent Builders (Priority)
- Method chaining with immutable pattern for better UX
- Example transformation:
  ```go
  // From: Manual step-by-step
  cluster := stack.NewCluster("production", rootNode)
  node := stack.NewNode("infrastructure")
  bundle := stack.NewBundle("monitoring")

  // To: Fluent builder
  cluster := stack.NewClusterBuilder("production").
      WithNode("infrastructure").
          WithBundle("monitoring").
              WithApplication("prometheus", appConfig).
          End().
      End().
      Build()
  ```

#### Phase 2: Preset Configurations
- Common application patterns as starting points (not enforced)
- Examples:
  ```go
  // Web app preset
  appConfig := stack.NewWebAppConfig("frontend").
      WithReplicas(3).
      WithImage("nginx:latest").
      WithPort(80).
      WithIngress(true).
      Build()

  // Monitoring stack preset
  infraConfig := stack.NewMonitoringStackConfig().
      WithPrometheus(true).
      WithGrafana(true).
      WithAlertManager(true).
      Build()

  // Database preset
  dbConfig := stack.NewDatabaseConfig("postgres").
      WithReplicas(1).
      WithStorage("10Gi").
      WithBackup(true).
      Build()
  ```

#### Phase 3: Templates & Inheritance
- Reusable base configurations with inheritance
- Example:
  ```go
  baseAppTemplate := stack.NewApplicationTemplate("base").
      WithCommonLabels(map[string]string{
          "managed-by": "kure",
          "environment": "production",
      }).
      WithResourceLimits(corev1.ResourceRequirements{
          Requests: corev1.ResourceList{
              corev1.ResourceCPU:    resource.MustParse("100m"),
              corev1.ResourceMemory: resource.MustParse("128Mi"),
          },
      }).
      Build()

  // Inherit from template
  frontendApp := stack.NewApplicationFromTemplate("frontend", baseAppTemplate).
      WithImage("frontend:v1.0.0").
      WithReplicas(3).
      Build()
  ```

#### Phase 4: Configuration Mixins & Composition
- Mix and match configuration components
- Example:
  ```go
  monitoringMixin := stack.NewMixin("monitoring").
      WithResources("prometheus", "grafana").
      WithNamespace("monitoring").
      Build()

  securityMixin := stack.NewMixin("security").
      WithNetworkPolicies(true).
      WithRBAC(true).
      Build()

  cluster := stack.NewClusterBuilder("production").
      WithMixin(monitoringMixin).
      WithMixin(securityMixin).
      Build()
  ```

#### Phase 5: Environment Profiles
- Environment-specific configurations (dev/staging/prod)
- Example:
  ```go
  devProfile := &EnvironmentProfile{
      Name: "development",
      Replicas: map[string]int32{
          "frontend": 1,
          "backend":  1,
      },
      Resources: map[string]corev1.ResourceRequirements{
          "frontend": {Requests: corev1.ResourceList{
              corev1.ResourceCPU: resource.MustParse("50m"),
          }},
      },
  }

  cluster.ApplyProfile(devProfile)
  ```

### Additional Implementation Considerations

#### Error Handling
- Collect errors during building, return on Build()
- Clear error messages with context about what failed
- Example:
  ```go
  cluster, err := builder.
      WithReplicas(-1).      // Stores error internally
      WithNode("test").      // Continues building
      Build()                // Returns nil, error with all issues
  ```

#### Configuration Discovery
- Provide CLI helpers for listing available presets/templates
- Generate documentation for all configuration options
- Consider IDE autocomplete support through well-structured APIs

#### Testing Strategy
- Each configuration component should be independently testable
- Provide test helpers for validating generated manifests
- Mock builders for unit testing user code

#### Performance Optimization
- Lazy evaluation where possible
- Consider caching for frequently used templates/mixins
- Efficient memory usage with large configurations

#### Migration Path
- Maintain backward compatibility with existing stack package
- Allow mixing old and new approaches during transition
- Provide migration guide and tooling if needed

## UX Design Recommendations

### Priority Adjustments
1. **CLI-First Approach**: Implement CLI tooling before web UI (`kure init`, `kure template`, `kure validate`)
2. **Simplified Helpers**: Add quick helper functions alongside full builder chains
3. **IDE Integration**: Focus on VSCode/IDE extensions before visual builders
4. **Template Versioning**: Implement version management from day one
5. **GitHub-Based Sharing**: Start with GitHub repository for templates before marketplace

### Technical Recommendations
1. **Validation Enhancement**: Add "fix-it" capabilities and OPA integration
2. **Performance**: Consider lighter UI frameworks (Preact/Svelte) for bundle size
3. **Testing**: Implement property-based and snapshot testing
4. **Migration Tools**: Create YAML-to-Kure converters and reverse engineering
5. **API Versioning**: Define clear API stability guarantees for builders

### Critical Gaps
1. **Multi-tenancy Support**: Define patterns for multi-tenant configurations
2. **Security Review Process**: Establish template certification workflow
3. **Offline/Air-gapped**: Support for disconnected environments
4. **GitOps UI Integration**: Detailed Flux/ArgoCD workflow in UI
5. **Documentation Generation**: Auto-generate docs from code annotations

# important-instruction-reminders
Do what has been asked; nothing more, nothing less.
NEVER create files unless they're absolutely necessary for achieving your goal.
ALWAYS prefer editing an existing file to creating a new one.
NEVER proactively create documentation files (*.md) or README files. Only create documentation files if explicitly requested by the User.
- always implement extensive tests on new code
