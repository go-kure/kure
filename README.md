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

#### 3. Layout Management (`pkg/layout/`)
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
- **pkg/fluxcd/**: Public API facade over internal Flux builders
- **pkg/k8s/**: Kubernetes scheme and utility functions

## Getting Started

### Quick Example

The repository includes an extensive example in [cmd/demo/main.go](cmd/demo/main.go) that constructs several resources and prints them as YAML. A short excerpt is shown below:

```go
y := printers.YAMLPrinter{}

ns := kubernetes.CreateNamespace("demo")
kubernetes.AddNamespaceLabel(ns, "env", "demo")

if err := y.PrintObj(ns, os.Stdout); err != nil {
    fmt.Fprintf(os.Stderr, "failed to print YAML: %v\n", err)
}
```

### Running Examples

```bash
# Run the comprehensive demo
go run ./cmd/demo

# Run with specific options
go run ./cmd/demo -internals     # Internal package demos
go run ./cmd/demo -app-workload  # AppWorkload example
go run ./cmd/demo -cluster       # Cluster example
```

### Using in Your Project

To use the helpers in your own code, import the desired package from `github.com/go-kure/kure`:

```go
import "github.com/go-kure/kure/internal/kubernetes"
```

## Key Features

### Strongly-Typed Resource Construction

```go
ns := kubernetes.CreateNamespace("demo")
kubernetes.AddNamespaceLabel(ns, "env", "demo")

dep := kubernetes.CreateDeployment("app", "demo")
kubernetes.AddDeploymentContainer(dep, container)
```

### GitOps Workflow Integration

```go
// Generate Flux Kustomizations
wf := fluxcd.NewWorkflow()
fluxObjs, err := wf.Cluster(cluster)

// Or ArgoCD Applications
argoWf := argocd.NewWorkflow()
argoObjs, err := argoWf.Cluster(cluster)
```

### Layout Rules and FluxCD Integration

`LayoutRules` control how nodes, bundles and applications are grouped when writing manifests. The example below flattens bundles and applications under their parent node, writes the manifests to `./repo` and then generates Flux Kustomizations for the cluster:

```go
rules := layout.LayoutRules{
    BundleGrouping:      layout.GroupFlat,
    ApplicationGrouping: layout.GroupFlat,
}

ml, err := layout.WalkCluster(cluster, rules)
if err != nil {
    // handle error
}

cfg := layout.DefaultLayoutConfig()
if err := layout.WriteManifest("./repo", cfg, ml); err != nil {
    // handle error
}

wf := fluxcd.NewWorkflow()
fluxObjs, err := wf.Cluster(cluster)
if err != nil {
    // handle error
}
```

### Declarative Patching

The `kure` CLI can patch a base manifest using a file of patch operations:

```bash
kure patch --base examples/patch/base-config.yaml --patch examples/patch/patch.yaml
```

The command reads the base resource(s) and applies the patches, printing the resulting YAML to stdout.

Example patch file:
```toml
[[patches]]
path = "spec.replicas"
value = 5
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

## Development & Testing

### Running Tests

All packages include unit tests. Run them with:

```bash
go test ./...
```

The `go test` command will discover and execute tests across all packages.

### Code Quality
- **52 test files** with comprehensive unit test coverage
- GitHub Actions CI/CD pipeline with Go 1.24.4
- Qodana static analysis with vulnerability scanning
- gotestfmt for enhanced test output formatting

### Dependencies
- **Go 1.24.5** with modern Kubernetes client libraries (v0.33.2)
- Flux controller APIs (v1.x series)
- cert-manager v1.16.2, External Secrets v0.18.2, MetalLB v0.15.2
- Built on controller-runtime for Kubernetes integration

### Examples & Documentation
- Comprehensive demo in `cmd/demo/main.go` showcasing all features
- Example cluster configurations in `examples/`
- API documentation available at [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure)
- Design documents in `pkg/*/DESIGN.md` files

## Use Cases

- **GitOps Platform Development**: Building tools that generate Kubernetes manifests
- **Cluster Management**: Programmatic cluster configuration and resource management
- **CI/CD Integration**: Generating deployment configurations from application metadata
- **Multi-Environment Deployments**: Consistent resource generation across environments
- **Complex Application Stacks**: Managing interdependent services and infrastructure

## Documentation

API reference documentation is available at [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure). Packages like `kubernetes`, `fluxcd`, `certmanager`, and `metallb` are located under the `internal/` directory and include helpers for constructing related resources.

## License

This project is licensed under the Apache License 2.0. See the [LICENSE](LICENSE) file for details.