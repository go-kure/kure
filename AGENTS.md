# Kure Agent Instructions

This document provides comprehensive guidance for AI agents working on the Kure codebase.

## Project Overview

Kure is a Go library for programmatically building Kubernetes resources used by GitOps tools (Flux, cert-manager, MetalLB, External Secrets). The library emphasizes strongly-typed object construction over templating engines.

### Technology Stack

- **Language**: Go 1.24
- **Core Dependencies**: Kubernetes APIs (v0.33.2), Flux v2.6.4, cert-manager v1.16.5, MetalLB v0.15.2
- **CLI Tools**: kure (main CLI), kurel (package system), demo (comprehensive examples)
- **Testing**: 105 test files with 100% pass rate
- **Build System**: Comprehensive Makefile with 40+ targets, mise for cross-repo consistency

### Architecture

- **Domain Model**: Hierarchical structure (Cluster → Node → Bundle → Application)
- **Builder Pattern**: Constructor functions (`Create*`) and helpers (`Add*`, `Set*`)
- **GitOps Agnostic**: Supports both Flux and ArgoCD workflows
- **No Templating**: Uses typed builders instead of string templates

## Repository Structure

```
kure/
├── cmd/
│   ├── kure/         # Main CLI
│   ├── kurel/        # Package system CLI
│   └── demo/         # Comprehensive examples
├── internal/         # Resource builders
│   ├── certmanager/  # cert-manager resources
│   ├── externalsecrets/  # External Secrets resources
│   ├── fluxcd/       # FluxCD resources
│   ├── gvk/          # GroupVersionKind utilities
│   ├── kubernetes/   # Core K8s resources
│   ├── metallb/      # MetalLB resources
│   └── validation/   # Validation utilities
├── pkg/
│   ├── cli/          # CLI utilities
│   ├── cmd/          # Command implementations
│   │   ├── kure/     # kure command
│   │   └── kurel/    # kurel command
│   ├── errors/       # Error handling (use this, not fmt.Errorf)
│   ├── io/           # YAML serialization
│   ├── kubernetes/   # Public K8s utilities
│   ├── launcher/     # Package launcher
│   ├── logger/       # Logging (use this for all logging)
│   ├── patch/        # JSONPath-based patching
│   └── stack/        # Core domain model
│       ├── argocd/   # ArgoCD workflow
│       ├── fluxcd/   # FluxCD workflow
│       ├── generators/  # Application generators
│       └── layout/   # Manifest organization
├── examples/         # Sample configurations
├── docs/             # Documentation
├── .claude/          # Claude Code configuration
├── mise.toml         # Tool versions and tasks
├── Makefile          # Build system (40+ targets)
├── AGENTS.md         # This file
└── DEVELOPMENT.md    # Development workflow guide
```

## Development Workflow

### Setup

```bash
# Install tools via mise
mise install

# Build all executables
mise run build
# or: make build

# Run tests
mise run test
# or: make test
```

### Testing

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

### Code Quality

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

### Building

```bash
# Build all executables
make build

# Build specific tools
make build-kure    # Main CLI
make build-kurel   # Package system
make build-demo    # Demo executable
```

### Pre-commit Workflow

Before committing changes:

```bash
# Quick check
make check

# Or comprehensive pre-commit
make precommit
```

## Git Workflow

- **`main` is protected** — never commit directly to `main`
- Always create a feature branch from `main` before making changes:
  ```bash
  git checkout -b <type>/<description> main
  ```
- **Branch prefixes**: `feat/`, `fix/`, `docs/`, `chore/`
- **Required CI checks** that must pass: `lint`, `test`, `build`, `rebase-check`
- **Auto-rebase**: open PRs are automatically rebased when main is updated
- **1 approving review** required
- **Linear history** enforced — rebase only, no merge commits
- **All conversations** must be resolved before merge
- Use `gh pr create` to open pull requests
- PR template: `.github/PULL_REQUEST_TEMPLATE.md`

## Code Conventions

### Function Naming

- **Constructors**: `Create<ResourceType>()`
- **Adders**: `Add<ResourceType><Field>()`
- **Setters**: `Set<ResourceType><Field>()`
- **Helpers**: Descriptive names for utilities

### Error Handling

Always use the kure/errors package:

```go
import "github.com/go-kure/kure/pkg/errors"

// Wrapping errors
return errors.Wrap(err, "context about what failed")

// Creating new errors
return errors.New("description of error")
```

**Never use `fmt.Errorf` directly** - always use the errors package.

### Logging

Always use pkg/logger for logging:

```go
import "github.com/go-kure/kure/pkg/logger"

logger.Info("message", "key", value)
logger.Error("message", "error", err)
```

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

## Adding New Resource Builders

1. Create constructor function: `func Create<ResourceType>(name, namespace string, ...) *<Type>`
2. Add helper functions: `func Add<ResourceType><Field>(...)`
3. Add setter functions: `func Set<ResourceType><Field>(...)`
4. Include comprehensive unit tests in `*_test.go`
5. Follow existing patterns in `internal/` packages

## Security Considerations

### Secret Management

- **Never hardcode secrets** in builders
- Always reference secrets through Kubernetes Secret objects
- Use `SecretKeySelector` and `LocalObjectReference` patterns

```go
key := cmmeta.SecretKeySelector{
    LocalObjectReference: cmmeta.LocalObjectReference{Name: "secret-name"},
    Key: "key-name",
}
```

### RBAC

- Always use least-privilege principles
- Provide granular role and binding builders
- Test RBAC configurations thoroughly

## Crane Integration

Kure is a dependency of Crane (`/home/serge/src/autops/wharf/crane`).

### Relationship

- **Crane** transforms OAM → Kure domain model → Kubernetes manifests
- **Kure** provides the domain model and manifest generation engine
- Both repos are **co-developed** with local replace directives

### Key Interfaces Crane Depends On

| Interface | Location | Crane Usage |
|-----------|----------|-------------|
| `Application` | `pkg/stack/application.go` | Workload container with ApplicationConfig |
| `ApplicationConfig` | `pkg/stack/application.go` | Component handlers implement this |
| `Bundle` | `pkg/stack/bundle.go` | Deployment unit with DependsOn |
| `Cluster` | `pkg/stack/cluster.go` | Target cluster representation |
| `Node` | `pkg/stack/cluster.go` | Organizational unit in hierarchy |

### Development Priority

When Crane needs something from Kure:
1. Crane defines the requirement (what interface/behavior is needed)
2. Kure implements it
3. Crane consumes the new capability
4. Both repos stay in sync

### Key Files

- Crane's requirements: `/home/serge/src/autops/wharf/crane/PLAN.md`
- Crane's agent guide: `/home/serge/src/autops/wharf/crane/AGENTS.md`

### Before Modifying Kure APIs

1. **Check Crane's PLAN.md** - Does the change align with Crane's needs?
2. **Consider Crane impact** - Will this break or help Crane's integration?
3. **Keep interfaces stable** - Crane depends on kure's public APIs
4. **Update both repos** - Changes may require Crane updates

## Integration Patterns

### Flux Integration

```go
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

## Fluent Builders

Kure provides fluent builders for ergonomic configuration:

```go
cluster := stack.NewClusterBuilder("production").
    WithNode("infrastructure").
        WithBundle("monitoring").
            WithApplication("prometheus", appConfig).
        End().
    End().
    Build()
```

Fluent builders follow an immutable pattern - each `With*` method returns a new builder instance.

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

## Dependencies

### Core Dependencies

- Kubernetes client libraries (v0.33.2)
- Flux controller APIs (v1.x)
- cert-manager (v1.16.2)
- External Secrets (v0.19.2)
- MetalLB (v0.15.2)
- controller-runtime for Kubernetes integration

### Version Management

- Go version pinned via mise.toml
- Use `go mod tidy` to clean dependencies
- Pin specific versions for stability

## Documentation Synchronization

Kure uses a 4-layer documentation model designed so that most content stays in sync automatically.

### Layer 1: Package READMEs (auto-synced)

Each `pkg/` package has a `README.md` that lives alongside the code. These are automatically mounted to the documentation site via `inject-frontmatter.sh`. When you change a package, update its README in the same PR.

### Layer 2: Cross-cutting guides (manually synced)

Hand-authored guides in `site/content/guides/` describe multi-package workflows. They focus on the flow (which steps, in what order) and link to package READMEs for specifics. Use the reverse mapping table below to know which guides to review.

### Layer 3: CLI reference (auto-generated)

Generated from cobra command definitions by `cmd/gendocs/main.go`. Run `make docs-cli` to regenerate.

### Layer 4: API reference (external)

Links to pkg.go.dev. Updated automatically when the module is published.

### Rule

**Code and documentation changes must be in the same PR.** If you modify a package's public API, update its README and check the guides listed in the reverse mapping.

### Reverse Mapping: Code to Docs

| Package Changed | Auto-Synced (README) | Guides to Review |
|-----------------|---------------------|------------------|
| `pkg/stack/` | `api-reference/stack` | `guides/flux-workflow`, `concepts/domain-model` |
| `pkg/stack/fluxcd/` | `api-reference/flux-engine` | `guides/flux-workflow` |
| `pkg/stack/generators/` | `api-reference/generators` | `guides/generators` |
| `pkg/stack/layout/` | `api-reference/layout` | `guides/flux-workflow` |
| `pkg/launcher/` | `api-reference/launcher` | `guides/kurel-packages` |
| `pkg/patch/` | `api-reference/patch` | `guides/patching` |
| `pkg/io/` | `api-reference/io` | `guides/library-usage` |
| `pkg/errors/` | `api-reference/errors` | — |
| `pkg/cli/` | `api-reference/cli` | — |
| `pkg/kubernetes/` | `api-reference/kubernetes-builders` | `guides/library-usage` |
| `pkg/kubernetes/fluxcd/` | `api-reference/fluxcd-builders` | `guides/library-usage` |
| `pkg/logger/` | `api-reference/logger` | — |
| `cmd/kure/` | CLI ref (auto-generated) | — |
| `cmd/kurel/` | CLI ref (auto-generated) | — |
| `.github/workflows/` | — | `contributing/github-workflows` |
| `scripts/gen-versions-toml.sh` | — | `contributing/github-workflows` |

## Implementation Workflow

When implementing a GitHub issue, follow this checklist in order:

1. **Branch** — create a feature branch from latest `main` before writing any code.
2. **Validate the issue** — compare the issue description against project standards (naming conventions, error handling, package placement). Question anything that conflicts before implementing.
3. **Implement with tests** — write or update tests next to every new or changed function.
4. **Update documentation** — update package READMEs, the reverse-mapping table, and any affected guides in the same changeset.
5. **Run all checks** — execute `make precommit` and fix any failures. When all checks pass, stop and ask for a user review.
6. **Iterate on review feedback** — address every comment, then return to step 5.
7. **Verify the diff** — before committing, review the full working-tree diff. If there are more changes than expected, ask the user what should be committed.
8. **Commit, push, PR** — commit with a conventional-commit message, push, and open a PR with `gh pr create`.

## Questions?

Refer to:
1. `DEVELOPMENT.md` - Detailed development workflow
2. Crane's `PLAN.md` - Authoritative requirements for API design
3. Crane's `AGENTS.md` - Crane integration details
