# Claude Instructions for Kure

## Primary Reference

**Read `AGENTS.md` first** - it contains comprehensive instructions for working with this codebase, including:
- Repository structure
- Development workflow
- Code conventions
- Testing patterns
- Crane integration details
- Common tasks

## Claude-Specific Notes

### Context Files

When working on Kure, load these files for context:
- `AGENTS.md` - Agent instructions and development guide
- `DEVELOPMENT.md` - Development workflow documentation
- `go.mod` - Dependencies and module path

### Code Generation Patterns

When generating resource builders:

```go
// internal/<package>/<resource>.go
package <package>

// Create<ResourceType> creates a new <resource> with required fields
func Create<ResourceType>(name, namespace string) *<Type> {
    return &<Type>{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: namespace,
        },
    }
}

// Add<ResourceType><Field> adds a <field> to the <resource>
func Add<ResourceType><Field>(obj *<Type>, ...) {
    // Implementation
}

// Set<ResourceType><Field> sets a <field> on the <resource>
func Set<ResourceType><Field>(obj *<Type>, ...) {
    // Implementation
}
```

### Testing Pattern

Always create comprehensive tests:

```go
func TestCreate<ResourceType>(t *testing.T) {
    obj := Create<ResourceType>("test", "default")
    if obj == nil {
        t.Fatal("expected non-nil object")
    }
    // Validate required fields...
}
```

### Error Handling

Always use the kure/errors package:
```go
import "github.com/go-kure/kure/pkg/errors"

return errors.Wrap(err, "context about what failed")
```

### Commits

Follow conventional commits:
- `feat:` - New features
- `fix:` - Bug fixes
- `chore:` - Maintenance
- `build:` - Build system changes
- `test:` - Test additions/changes
- `docs:` - Documentation

### Git Workflow

`main` is protected — always create a feature branch before making changes:

```bash
git checkout -b <type>/<description> main
# make changes, commit
git push -u origin <type>/<description>
gh pr create
```

Required checks: `lint`, `test`, `build`. See `AGENTS.md` § Git Workflow for full details.

## Quick Commands

```bash
# Build all executables
mise run build
# or: make build

# Test
mise run test
# or: make test

# Lint
mise run lint
# or: make lint

# Tidy dependencies
mise run tidy
# or: make tidy

# Quick pre-commit check
mise run check
# or: make check

# Run all checks (tidy, lint, test)
mise run verify
# or: make precommit
```

## Memory Notes

- kurel just generates YAML
- Always implement errors via the kure/errors package
- When updating GitHub workflows, also update docs/github-workflows.md
- Always use pkg/logger for logging

## Crane Integration

Kure is a dependency of Crane (`/home/serge/src/autops/wharf/crane`).

Before modifying kure's public APIs (`pkg/stack/`):
1. Check Crane's `PLAN.md` for requirements
2. Consider impact on Crane's integration
3. Keep interfaces stable when possible
4. Update Crane if breaking changes are necessary

Reference: `/home/serge/src/autops/wharf/crane/PLAN.md` - Authoritative requirements document
