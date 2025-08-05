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
- Go version: **1.24.5**

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
- Go 1.24.5 required
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