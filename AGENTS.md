# Kure Agent Instructions

This document provides comprehensive guidance for AI agents working on the Kure codebase.

## Project Overview

Kure is a Go library for programmatically building Kubernetes resources used by GitOps tools (Flux, cert-manager, MetalLB, External Secrets). The library emphasizes strongly-typed object construction over templating engines.

### Technology Stack

- **Language**: Go 1.26.1
- **Core Dependencies**: Kubernetes APIs (v0.35.1), Flux v2.8.2, cert-manager v1.20.0, MetalLB v0.15.3
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
├── internal/         # Resource builders
│   ├── certmanager/  # cert-manager resources
│   ├── externalsecrets/  # External Secrets resources
│   ├── fluxcd/       # FluxCD resources
│   ├── gvk/          # GroupVersionKind utilities
│   ├── kubernetes/   # Core K8s resources
│   ├── metallb/      # MetalLB resources
│   └── validation/   # Validation utilities
├── pkg/
│   ├── errors/       # Error handling (use in app code instead of fmt.Errorf)
│   ├── io/           # YAML serialization
│   ├── kubernetes/   # Public K8s utilities
│   ├── logger/       # Logging (use this for all logging)
│   └── stack/        # Core domain model
│       ├── argocd/   # ArgoCD workflow
│       ├── fluxcd/   # FluxCD workflow
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

#### Coverage Thresholds

- **CI gate (enforced):** 90% total — PRs fail if total coverage drops below this
- **Target:** 95% total, ≥90% per package — maintain this as the working standard
- Check per-package detail: `GOWORK=off go test ./... -covermode=atomic 2>&1 | grep "coverage:"`
- Check total: `GOWORK=off go test ./... -coverprofile=/tmp/cov.out -covermode=atomic 2>/dev/null && go tool cover -func=/tmp/cov.out | tail -1`

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
- **Required CI checks** that must pass: `lint`, `test`, `build`
- **Merge queue**: merging goes through a GitHub merge queue (rebase method) that rebases and tests the merged result before landing — no manual rebasing needed
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

Always use `github.com/go-kure/kure/pkg/errors` in application code — never call `fmt.Errorf` directly outside of `pkg/errors` itself. The `pkg/errors` package wraps `fmt.Errorf` internally; this is correct and expected.

```go
import "github.com/go-kure/kure/pkg/errors"

// Preferred: use the errors package
return errors.Wrap(err, "context about what failed")
return errors.Wrapf(err, "failed to process %s", name)
return errors.New("description of error")
return errors.Errorf("invalid value: %s", val)

// Discouraged: raw fmt.Errorf in application code
// return fmt.Errorf("context: %w", err)   // use errors.Wrap instead
// return fmt.Errorf("invalid value: %s", val) // use errors.Errorf instead
```

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

See [OCI Artifact Layout](https://github.com/go-kure/.github/blob/main/docs/design/oci-layout.md)
for the directory structure and naming conventions that `ManifestLayout` and `WriteToTar` enforce.

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
5. **golangci-lint version mismatch**: If lint fails with "Go language version used to build golangci-lint is lower than the targeted Go version", update the golangci-lint version in both `mise.toml` and `Makefile`. When bumping Go, always check that golangci-lint is built with a compatible Go version.
6. **Stale GOPATH binaries shadowing mise**: The Makefile appends (not prepends) `GOPATH/bin` to PATH so mise-managed tools take precedence. If you see unexpected tool versions, check `which <tool>` vs `mise which <tool>`.

### Debugging Tips

- Check test output for validation errors
- Verify Kubernetes API versions in dependencies

## Dependencies

### Core Dependencies

- Kubernetes client libraries (v0.35.1)
- Flux controller APIs (v2.8.2)
- cert-manager (v1.20.0)
- External Secrets (v1.3)
- MetalLB (v0.15.3)
- controller-runtime for Kubernetes integration

### Version Management

- Go version pinned via mise.toml
- Use `go mod tidy` to clean dependencies
- Pin specific versions for stability

## Documentation Synchronization

**Documentation sync is mandatory and CI-enforced.** This repo follows the go-kure
organization documentation-sync standard (`go-kure/.github` → `docs/standards.md`).

### Single source of truth

`site/docs-map.yaml` is the normative map of code→docs. Every public package appears
there exactly once (`mount:` to publish, or `mounted: false` + `reason:`). The site
mounts, the AGENTS reverse-mapping table below, and the `api-reference/_index.md` nav
are all generated from or validated against it — **never hand-edit them**. To change
what's published, edit `docs-map.yaml` and run `bash site/scripts/gen-docs-tables.sh`.

### Rule

**Code and documentation changes must be in the same PR.** If you change a package,
update its `README.md` (and any guides listed in the reverse mapping) in the same PR.
Removing/renaming a package or symbol must repoint every reference; a 404 in the
rendered site is a CI failure.

### Enforcement

- `site/scripts/check-doc-sync.sh` — every public package is mapped; READMEs and
  mount targets exist; generated tables are current (blocking, `docs-build` job).
- `site/scripts/check-links.sh` — all internal links resolve in the rendered site
  (lychee, blocking, `docs-build` job).
- `site/scripts/check-doc-gate.sh` — a mapped package's source change must touch its
  mapped docs (the `doc-gate` job). Bypass only with the maintainer-restricted
  `docs-skip` label.

### Cross-cutting guides

Hand-authored guides in `site/content/guides/` describe multi-package workflows; use
the reverse mapping table below to know which to review. Full Go API reference lives
on pkg.go.dev.

### Reverse Mapping: Code to Docs

This table is generated from `site/docs-map.yaml`. Do not edit it by hand — edit
the map and run `bash site/scripts/gen-docs-tables.sh`.

<!-- BEGIN GENERATED: reverse-mapping (source: site/docs-map.yaml) -->
| Package Changed | Auto-Synced (README) | Guides to Review |
|-----------------|---------------------|------------------|
| `pkg/stack/` | `api-reference/stack` | `guides/flux-workflow`, `concepts/domain-model` |
| `pkg/stack/fluxcd/` | `api-reference/flux-engine` | `guides/flux-workflow` |
| `pkg/stack/layout/` | `api-reference/layout` | `guides/flux-workflow` |
| `pkg/io/` | `api-reference/io` | `guides/library-usage` |
| `pkg/manifest/` | `api-reference/manifest` | `guides/library-usage` |
| `pkg/kubernetes/` | `api-reference/kubernetes-builders` | `guides/library-usage` |
| `pkg/kubernetes/certmanager/` | `api-reference/certmanager-builders` | `guides/library-usage` |
| `pkg/kubernetes/cilium/` | `api-reference/cilium-builders` | `guides/library-usage` |
| `pkg/kubernetes/cnpg/` | `api-reference/cnpg-builders` | `guides/library-usage` |
| `pkg/kubernetes/externalsecrets/` | `api-reference/externalsecrets-builders` | `guides/library-usage` |
| `pkg/kubernetes/fluxcd/` | `api-reference/fluxcd-builders` | `guides/library-usage` |
| `pkg/kubernetes/metallb/` | `api-reference/metallb-builders` | `guides/library-usage` |
| `pkg/kubernetes/prometheus/` | `api-reference/prometheus-builders` | `guides/library-usage` |
| `pkg/kubernetes/volsync/` | `api-reference/volsync-builders` | `guides/library-usage` |
| `pkg/errors/` | `api-reference/errors` | — |
| `pkg/logger/` | `api-reference/logger` | — |
| `.github/workflows/` | — | `contributing/github-workflows` |
| `go.mod` / `versions.yaml` | — | `docs/dependency-updates`, `docs/compatibility` |
| `scripts/gen-versions-toml.sh` | — | `contributing/github-workflows` |
<!-- END GENERATED: reverse-mapping -->

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

## Organization Resources

The go-kure org governance, design documents, and community files are maintained in
[go-kure/.github](https://github.com/go-kure/.github).

- **Design documents** (`docs/design/`):
  - [OCI Artifact Layout](https://github.com/go-kure/.github/blob/main/docs/design/oci-layout.md) — layout tree conventions, layer structure
  - [API Stability Contract](https://github.com/go-kure/.github/blob/main/docs/design/api-stability.md) — versioning, pkg/ vs internal/, deprecation policy
  - [Package Structure](https://github.com/go-kure/.github/blob/main/docs/design/package-structure.md) — kure + launcher organization
  - [OAM Runtime](https://github.com/go-kure/.github/blob/main/docs/design/oam-runtime.md) — kurel design
- **Standards**: [docs/standards.md](https://github.com/go-kure/.github/blob/main/docs/standards.md)
- **Contributing**: [CONTRIBUTING.md](https://github.com/go-kure/.github/blob/main/CONTRIBUTING.md)
- **Reusable workflows**: release, pr-review, claude — all hosted in go-kure/.github

## Questions?

Refer to:
1. `DEVELOPMENT.md` - Detailed development workflow
2. `docs/dependency-updates.md` - Dependency upgrade procedures
3. Crane's `PLAN.md` - Authoritative requirements for API design
4. Crane's `AGENTS.md` - Crane integration details
