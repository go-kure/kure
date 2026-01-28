# Kure

**Kure** is a Go library for programmatically building Kubernetes resources used by GitOps tools like Flux, cert-manager, MetalLB, and External Secrets. The library emphasizes strongly-typed object construction over templating engines, providing a clean, type-safe approach to Kubernetes manifest generation.

## Design Philosophy

Kure prioritizes **type safety**, **composability**, and **simplicity** over template-based approaches. By providing strongly-typed builders and avoiding string interpolation, it reduces errors and improves maintainability in Kubernetes resource generation workflows.

## Architecture

### Core Design Patterns

- **Hierarchical Domain Model**: Cluster → Node → Bundle → Application structure for organizing resources
- **Builder Pattern**: Extensive use of constructor functions (`Create*`) and helper methods (`Add*`, `Set*`)
- **Strategy Pattern**: ApplicationConfig interface enables different resource generation strategies
- **GitOps Tool Agnostic**: Core abstractions can target both Flux and ArgoCD workflows

### Key Components

#### 1. Domain Model (`pkg/stack/`)
- **Cluster**: Root container with hierarchical node structure
- **Node**: Tree structure containing child nodes and bundles
- **Bundle**: Deployment unit for Flux Kustomization reconciliation
- **Application**: Individual deployable application with configuration
- **Workflow Interface**: Converts stack objects to GitOps tool resources

#### 2. Resource Builders (`internal/`)
- **kubernetes/**: Core K8s resources (Deployments, Services, ConfigMaps, RBAC)
- **fluxcd/**: Flux resources (Kustomizations, GitRepositories, HelmReleases)
- **certmanager/**: Certificate management (Certificates, Issuers, ACME)
- **metallb/**: Load balancer resources (IPAddressPools, BGP configuration)
- **externalsecrets/**: External secret management (SecretStores, ExternalSecrets)

#### 3. Layout Management (`pkg/stack/layout/`)
- Controls manifest organization on disk
- Handles Flux (`./path`) vs ArgoCD (`path`) conventions
- Supports various grouping and file organization strategies
- Generates kustomization.yaml files

#### 4. Patching System (`pkg/patch/`)
- JSONPath-based declarative patching
- Operations: replace, delete, insert, append
- TOML-inspired syntax for patch files
- Avoids complex overlay management

#### 5. Utilities
- **pkg/io/**: YAML serialization and multi-document parsing
- **pkg/kubernetes/fluxcd/**: Public API facade over internal Flux builders
- **pkg/k8s/**: Kubernetes scheme and utility functions

## Getting Started

> **Quick Start**: See [docs/quickstart.md](docs/quickstart.md) for a step-by-step tutorial.

### Installation

```bash
go get github.com/go-kure/kure
```

### Basic Usage

To use Kure in your project, import the public API packages:

```go
import (
    "github.com/go-kure/kure/pkg/stack"
    "github.com/go-kure/kure/pkg/stack/layout"
    "github.com/go-kure/kure/pkg/kubernetes/fluxcd"
    "github.com/go-kure/kure/pkg/io"
)
```

## Key Features

### Domain Model Usage

```go
// Create cluster structure
cluster := stack.NewCluster("production")
node := stack.NewNode("control-plane")
bundle := stack.NewBundle("monitoring")

// Add to hierarchy
cluster.AddNode(node)
node.AddBundle(bundle)

// Configure application
app := stack.NewApplication("prometheus", appConfig)
bundle.AddApplication(app)
```

### GitOps Workflow Integration

```go
// Create Flux resources using the public API
ks := fluxcd.NewKustomization(&fluxcd.KustomizationConfig{
    Name:      "app",
    Namespace: "flux-system",
    Interval:  "1m",
    SourceRef: kustv1.CrossNamespaceSourceReference{
        Kind: "GitRepository",
        Name: "app-repo",
    },
})

repo := fluxcd.NewGitRepository(&fluxcd.GitRepositoryConfig{
    Name:      "app-repo",
    Namespace: "flux-system",
    URL:       "https://github.com/example/app",
    Interval:  "1m",
    Ref:       "main",
})

// Write YAML output
printer := io.NewYAMLPrinter()
printer.PrintObj(ks, os.Stdout)
printer.PrintObj(repo, os.Stdout)
```

### Layout and Manifest Generation

`LayoutRules` control how resources are organized on disk:

```go
// Configure layout rules
rules := layout.LayoutRules{
    BundleGrouping:      layout.GroupFlat,
    ApplicationGrouping: layout.GroupFlat,
}

// Generate manifest layout
ml, err := layout.WalkCluster(cluster, rules)
if err != nil {
    log.Fatal(err)
}

// Write manifests to disk
cfg := layout.DefaultLayoutConfig()
if err := layout.WriteManifest("./repo", cfg, ml); err != nil {
    log.Fatal(err)
}
```

### Declarative Patching

The `kure` CLI provides patching functionality:

```bash
kure patch --base examples/patches/cert-manager-simple.yaml --patch examples/patches/resources.kpatch
```

The command applies JSONPath-based patches to Kubernetes manifests.

Example patch file (`.kpatch` format):
```toml
[[patches]]
path = "spec.replicas"
value = 5
operation = "replace"

[[patches]]
path = "spec.template.spec.containers[0].resources.limits.memory"
value = "512Mi"
operation = "replace"
```

## Security Features

### Certificate Management
- ACME integration with Let's Encrypt
- DNS01 and HTTP01 challenge solvers
- Support for Cloudflare, Route53, CloudDNS providers
- Automated certificate provisioning

### Secret Management
- External Secrets integration
- Support for AWS Secrets Manager, GCP Secret Manager, Azure Key Vault
- No hardcoded sensitive data - all references through Kubernetes secrets
- SecretStore and ClusterSecretStore builders

### Access Control
- Comprehensive RBAC builders for Roles, ClusterRoles, Bindings
- ServiceAccount management with token controls
- Network Policy construction for traffic segmentation

## Flux vs ArgoCD Paths

Flux Kustomizations reference directories in a Git repository using `spec.path`. The value must begin with `./` and is interpreted relative to the repository root. ArgoCD Applications use `spec.source.path` without the `./` prefix but with the same relative semantics.

When nodes or bundles are stored in subfolders, the path has to point directly to that folder unless the directory tree only contains files for a single node or bundle. Flux will recursively auto-generate a `kustomization.yaml` when one is missing and include every manifest under the specified path. ArgoCD does not auto-generate a `kustomization.yaml` and therefore ignores nested directories unless they are referenced from a `kustomization.yaml` at the target path.

For example:

```text
repo/
  clusters/
    prod/
      nodes/
        cp/
          kustomization.yaml
      bundles/
        monitoring/
          kustomization.yaml
```

Flux Kustomization for the control-plane node:

```yaml
spec:
  path: ./clusters/prod/nodes/cp
```

Equivalent ArgoCD Application:

```yaml
spec:
  source:
    path: clusters/prod/nodes/cp
```

With this layout, each node or bundle is targeted individually. Pointing a Flux Kustomization to the parent directory (`./clusters/prod`) would combine the `cp` and `monitoring` manifests into a single deployment because it would auto-generate a `kustomization.yaml` for the entire tree. ArgoCD will only process the manifests under `clusters/prod` itself unless a `kustomization.yaml` aggregates the subdirectories, so each subfolder must be referenced separately.

## Configuration Validation

Kure provides built-in validation for common GitOps configuration fields to prevent deployment issues and ensure best practices.

### Interval Format Validation

Kure automatically validates time interval fields to ensure they follow Go's duration format and GitOps best practices:

**Validated Fields:**
- `Interval` - GitOps reconciliation frequency
- `Timeout` - Maximum wait time for resources to be ready
- `RetryInterval` - Frequency for retrying failed reconciliations

**Supported Formats:**
```go
// Simple durations
"1s"      // 1 second
"30s"     // 30 seconds  
"5m"      // 5 minutes
"1h"      // 1 hour
"24h"     // 24 hours (maximum)

// Complex durations
"1h30m"     // 1 hour 30 minutes
"2h15m30s"  // 2 hours, 15 minutes, 30 seconds
"1.5m"      // 1.5 minutes (90 seconds)
```

**Validation Rules:**
- **Minimum**: 1 second (`1s`)
- **Maximum**: 24 hours (`24h`)
- **Format**: Must follow Go time.Duration syntax
- **Empty Values**: Allowed (uses system defaults)

**Common Validation Errors:**

```go
// ❌ Invalid - Missing unit
bundle.Spec.Interval = "30"

// ❌ Invalid - Wrong unit
bundle.Spec.Interval = "5x"

// ❌ Invalid - Too short
bundle.Spec.Interval = "500ms"

// ❌ Invalid - Too long  
bundle.Spec.Interval = "48h"

// ❌ Invalid - Spaces
bundle.Spec.Interval = "5 minutes"

// ✅ Valid examples
bundle.Spec.Interval = "5m"
bundle.Spec.Timeout = "10m"
bundle.Spec.RetryInterval = "2m"
```

**Error Messages:**

When validation fails, you'll see descriptive error messages:

```
validation error in Bundle "my-app" at spec.interval: 
interval "500ms" is too short, minimum is 1s

validation error in Bundle "my-app" at spec.timeout:
invalid interval format: "5 minutes", expected format like '5m', '1h', '30s'
```

**Best Practices:**

- **Reconciliation Intervals**: Use `5m` to `30m` for most applications
- **Timeouts**: Set 2-3x longer than expected deployment time
- **Retry Intervals**: Use shorter intervals (`1m`-`5m`) for faster failure recovery
- **Production**: Avoid very short intervals (`<1m`) to reduce API load

### Bundle Configuration Example

```go
bundle := v1alpha1.NewBundleConfig("web-app")
bundle.Spec.Interval = "10m"        // Reconcile every 10 minutes
bundle.Spec.Timeout = "15m"         // Wait up to 15 minutes for readiness
bundle.Spec.RetryInterval = "2m"    // Retry failed deployments every 2 minutes

// Validation happens automatically when calling Validate()
if err := bundle.Validate(); err != nil {
    log.Fatalf("Bundle validation failed: %v", err)
}
```

## Development & Testing

### Running Tests

All packages include unit tests. Run them with:

```bash
# Run all tests
make test

# Run tests with verbose output
make test-verbose

# Run tests with coverage
make test-coverage

# Run specific package tests
go test ./pkg/stack/...

# Run all development tasks
make all
```

The `make test` command will discover and execute tests across all packages. Use `make help` to see all available development commands.

### Code Quality
- **105 test files** with comprehensive unit test coverage
- GitHub Actions CI/CD pipeline with Go 1.24.6
- Qodana static analysis with vulnerability scanning
- gotestfmt for enhanced test output formatting

### Dependencies
- **Go 1.24.6** with modern Kubernetes client libraries (v0.33.2)
- Flux controller APIs (v1.x series)
- cert-manager v1.16.2, External Secrets v0.18.2, MetalLB v0.15.2
- Built on controller-runtime for Kubernetes integration

### CLI Tools

Kure provides two complementary CLI tools:

#### kure - Library CLI
```bash
# Generate Kubernetes manifests
kure generate cluster --config cluster.yaml

# Apply patches to manifests
kure patch --file deployment.yaml --patch patches/

# Show version and help
kure version
kure --help
```

#### kurel - Package Manager
```bash
# Build a kurel package
kurel build ./my-package

# Validate package structure
kurel validate ./my-package

# Show package information
kurel info ./my-package

# Generate JSON schemas (with Kubernetes support)
kurel schema generate ./output --k8s

# Show version and help
kurel version
kurel --help
```

The kurel CLI provides package-based resource management, allowing teams to create reusable, configurable Kubernetes applications without complex templating engines.

### Examples & Documentation
- Example cluster configurations in `examples/`
- API documentation available at [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure)
- Design documents in `pkg/*/DESIGN.md` files
- CLI reference via `kure --help` and `kurel --help`

## Use Cases

- **GitOps Platform Development**: Building tools that generate Kubernetes manifests
- **Cluster Management**: Programmatic cluster configuration and resource management
- **CI/CD Integration**: Generating deployment configurations from application metadata
- **Multi-Environment Deployments**: Consistent resource generation across environments
- **Complex Application Stacks**: Managing interdependent services and infrastructure

## Documentation

### API Reference

Full API documentation is available at [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure). The main public packages are:

- `pkg/stack` - Core domain model and workflow interfaces
- `pkg/stack/layout` - Manifest organization and file layout
- `pkg/kubernetes/fluxcd` - Flux resource creation and management
- `pkg/patch` - JSONPath-based patching system
- `pkg/io` - YAML serialization and printing utilities

## License

This project is licensed under the Apache License 2.0. See the [LICENSE](LICENSE) file for details.