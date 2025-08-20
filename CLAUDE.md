# Claude Instructions for go-kure/kure Repository

## Project Context

This is **Kure**, a Go library for programmatically building Kubernetes resources used by GitOps tools (Flux, cert-manager, MetalLB, External Secrets). The library emphasizes strongly-typed object construction over templating engines.

### Technology Stack
- **Language**: Go 1.24.6
- **Core Dependencies**: Kubernetes APIs (v0.33.2), Flux v2.6.4, cert-manager v1.16.2, MetalLB v0.15.2
- **CLI Tools**: kure (main CLI), kurel (package system), demo (comprehensive examples)
- **Testing**: 105 test files with 100% pass rate
- **Build System**: Comprehensive Makefile with CI/CD pipeline

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
make test

# Run with verbose output
make test-verbose

# Run tests with coverage
make test-coverage

# Run race detection tests
make test-race

# Quick test (short tests only)
make test-short

# Run integration tests
make test-integration
```

#### Code Quality
```bash
# Run linting
make lint

# Format code
make fmt

# Run static analysis
make vet

# Run all quality checks
make precommit

# Run Qodana analysis (requires Docker)
make qodana
```

- Uses **golangci-lint** v1.64.8 and **Qodana** static analysis
- **105 test files** with 100% pass rate across 140 source files
- Comprehensive Makefile with 40+ targets for development workflow
- GitHub Actions CI/CD pipeline

#### Examples & Demos
```bash
# Build and run comprehensive demo
make demo

# Run internal API demo
make demo-internals

# Run GVK generators demo
make demo-gvk

# Build all CLI tools
make build

# Build specific tools
make build-kure    # Main CLI
make build-kurel   # Package system
make build-demo    # Demo executable
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
```bash
# Run comprehensive pre-commit checks
make precommit

# Or run individual checks
make test
make lint
make fmt
make vet

# Verify builds
make build
```

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
- **Codebase**: 140 Go source files, 105 test files
- **Test Status**: All tests passing (100% success rate)
- **Code Quality**: Zero technical debt, comprehensive linting with golangci-lint v1.64.8

### Recent Achievements
- ✅ **CI/CD Pipeline Implementation** - Comprehensive Makefile with 40+ targets and GitHub Actions workflow
- ✅ **Linting Infrastructure** - golangci-lint v1.64.8 compatibility and full issue resolution
- ✅ **Test Stability** - Fixed intermittent test failures and stdout capture issues
- ✅ **GVK Versioning System** - Kubernetes-style API versioning for stack layers
- ✅ **KurelPackage Generator** - **COMPLETED** - Full implementation with validation and testing
- ✅ **Kurel CLI K8s Schema Inclusion** - **COMPLETED** - Enhanced validation with --k8s flag
- ✅ **Comprehensive Test Coverage** - 105 test files with 100% pass rate across entire codebase

### Architecture Highlights
- **Hierarchical domain model** fully implemented (Cluster → Node → Bundle → Application)
- **Generator system** with ApplicationConfig interface and registry pattern
- **No templating approach** maintained throughout - pure Go builders
- **GitOps dual support** for both Flux and ArgoCD workflows
- **Patching system** with JSONPath support operational
- **Layout management** for flexible manifest organization
- **API Versioning** with stack.gokure.dev/v1alpha1 GVK pattern

### Test Coverage Status
**EXCELLENT COVERAGE** - 105 test files across all major packages (all passing):
- `internal/`: certmanager, externalsecrets, fluxcd, gvk, kubernetes, metallb, validation
- `pkg/`: cli, errors, io, launcher, patch, stack (including all generators, v1alpha1)
- `pkg/cmd/`: kure, kurel (including generate subpackages)
- `pkg/stack/`: argocd, fluxcd, generators (appworkload, fluxhelm, kurelpackage)
- `pkg/kubernetes/`: fluxcd integration, scheme, utilities

### Available Examples
- app-workloads, bootstrap, clusters, generators
- kurel package examples, multi-oci, patches

### Key Metrics
- **Go Version**: 1.24.6 with modern dependency management
- **Code Quality**: Zero technical debt, comprehensive linting pipeline
- **Performance**: 72ns config creation, 1.4μs tree conversion benchmarks
- **Development Workflow**: 40+ Makefile targets, GitHub Actions CI/CD
- **Documentation**: Comprehensive README, DESIGN.md, ARCHITECTURE.md files
- **Build System**: Multi-platform builds (Linux, macOS, Windows), release automation
- **Recent Status**: All priority development tasks completed

## Implementation Status

### ✅ COMPLETED PRIORITIES

#### ✅ Priority 1: KurelPackage Generator - **COMPLETED**
**Location**: `pkg/stack/generators/kurelpackage/v1alpha1.go`
- ✅ Resource gathering from filesystem with pattern matching
- ✅ JSONPatch and strategic merge patch generation
- ✅ Values file generation with schema support  
- ✅ Extension processing for conditional features
- ✅ Complete kurel.yaml manifest generation
- ✅ Comprehensive validation (version format, CEL expressions, resource paths)
- ✅ Full test coverage with 105 test files passing

#### ✅ Priority 2: Kurel CLI K8s Schema Inclusion - **COMPLETED** 
**Location**: `pkg/cmd/kurel/cmd.go`, `pkg/launcher/schema.go`
- ✅ Kubernetes schema inclusion with `--k8s` flag
- ✅ Enhanced validation for apiVersion and kind fields
- ✅ Enum constraints for common K8s resource types
- ✅ Backward compatibility maintained
- ✅ CLI help documentation updated

#### ✅ Priority 3: Testing Coverage - **ALREADY COMPLETE**
**Status**: Previously implemented with excellent coverage
- ✅ 105 test files across all major packages
- ✅ pkg/cli, pkg/cmd/kure, pkg/cmd/kurel fully tested
- ✅ pkg/kubernetes public API tested
- ✅ pkg/stack/argocd workflow tested  
- ✅ All generator packages comprehensively tested
- ✅ 100% test pass rate maintained

### 🚀 Priority 4: Future Enhancements
**Status**: Available for implementation
- Additional generator types and resource providers
- Extended MetalLB configuration options
- Enhanced patch operations beyond JSONPath
- Fluent Builders Phase 1 (major UX improvement)
- Performance optimizations and caching

### ❌ DEFERRED: ArgoCD Bootstrap Implementation
**Status**: Out of scope, architecture supports future implementation
**Note**: Core dual GitOps tool support remains in place

## Current Development Status

### ✅ **ALL PRIORITY TASKS COMPLETED**

The project has successfully completed all planned priority development work:

1. **✅ KurelPackage Generator** - Fully implemented with comprehensive testing
2. **✅ Kurel CLI K8s Schema Enhancement** - Working with `--k8s` flag for enhanced validation  
3. **✅ Test Coverage** - Excellent coverage with 105 test files, 100% pass rate

### 🎯 **Ready for Next Phase**

The codebase is now in excellent condition for **Priority 4: Future Enhancements**:

#### Immediate Opportunities
- **✅ Fluent Builders Phase 1** - **COMPLETED** - Major UX improvement with method chaining
- **CEL Validation Enhancement** - Implement proper CEL validation using cel-go library
  
  **What is CEL Validation Enhancement?**
  
  CEL (Common Expression Language) is a mini programming language for writing validation rules in kurel packages. Currently, Kure accepts CEL expressions as plain text without validating their syntax.
  
  **The Problem**: 
  ```yaml
  # In kurel.yaml
  validation:
    - rule: "size >> invalid syntax"  # Invalid CEL
      message: "Size must be positive"
  ```
  - **Current**: Kure accepts this ✅ → Deployment fails ❌ 
  - **Enhanced**: Kure catches syntax errors immediately ❌ → Fix before deployment ✅
  
  **Real-World Example**:
  ```yaml
  validation:
    rules:
      - cel: "object.spec.replicas >= 1"           # ✅ Valid
        message: "Need at least 1 replica"
      - cel: "object.metadata.name.length() > 0"   # ✅ Valid  
        message: "Name cannot be empty"
      - cel: "invalid..syntax..here"               # ❌ Would catch this!
        message: "This rule is broken"
  ```
  
  **Benefits**: Early error detection, better developer experience, more reliable kurel packages
- **✅ Interval Format Validation** - **COMPLETED** - Time interval validation with comprehensive documentation
- **Additional Generator Types** - Expand resource generation capabilities
- **Performance Optimizations** - Caching and parallel processing improvements

#### Long-term Enhancements  
- **Extended Patch Operations** - Beyond current JSONPath capabilities
- **Enhanced MetalLB Support** - Additional configuration options
- **ArgoCD Bootstrap** - If user demand emerges

### 📊 **Project Health Summary**
- **Codebase**: Mature, well-tested, zero technical debt
- **CI/CD**: Comprehensive Makefile with full automation
- **Documentation**: Complete with architecture details
- **Performance**: Excellent benchmarks (72ns config creation)
- **Status**: **Ready for production use and advanced feature development**

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

#### ✅ Phase 1: Fluent Builders - **COMPLETED**
**Location**: `pkg/stack/builders.go`, `pkg/stack/builders_test.go`
- ✅ Method chaining with immutable pattern implemented
- ✅ Complete fluent interfaces: ClusterBuilder, NodeBuilder, BundleBuilder
- ✅ Deep copying for immutability with copyCluster(), copyNode(), copyBundle()
- ✅ Hierarchical navigation with End() methods
- ✅ Error collection and aggregation
- ✅ Comprehensive test coverage
- **Usage**:
  ```go
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

## Article Series Project

### Kure/Kurel Introduction Article Series
**Location**: `../articles/PROJECT_STATUS.md`

A 4-part article series designed to gradually introduce Kure and Kurel principles to the Kubernetes community without naming them directly. The strategy focuses on problem identification and solution exploration to create natural demand for these tools.

**Status**: 3/4 articles complete (75% done)
- ✅ Article 1: YAML templating problems and production impact
- ✅ Article 2: Type-safe infrastructure and builder patterns  
- ✅ Article 3: Patch-based package management as Helm alternative
- 🎯 Article 4: Hierarchical domain modeling and GitOps integration (pending)

**Key Achievement**: Successfully translated all major Kure/Kurel concepts into compelling problem-solution narratives using real-world case studies and production examples.

See `../articles/PROJECT_STATUS.md` for complete project details, content strategy, and publication planning.

# important-instruction-reminders
Do what has been asked; nothing more, nothing less.
NEVER create files unless they're absolutely necessary for achieving your goal.
ALWAYS prefer editing an existing file to creating a new one.
NEVER proactively create documentation files (*.md) or README files. Only create documentation files if explicitly requested by the User.
- always implement extensive tests on new code
