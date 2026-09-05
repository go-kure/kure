# Claude Instructions for Kure

## Primary Reference

**Read `AGENTS.md` first** - it contains comprehensive instructions for working with this codebase, including:
- Repository structure
- Development workflow
- Code conventions
- Testing patterns
- Consumer compatibility guidance
- Common tasks

## Claude-Specific Notes

### Context Files

When working on Kure, load these files for context:
- `AGENTS.md` - Agent instructions and development guide
- `DEVELOPMENT.md` - Development workflow documentation
- `go.mod` - Dependencies and module path

### Builder Patterns

**Do not hand-write a constructor.** `Create<Kind>` is generated from the registered scheme into
`pkg/kubernetes/<family>/zz_generated_create.go` and emits `apiVersion`, `kind`, `metadata.name`
and — for a namespaced kind — `metadata.namespace`, and nothing else. Register the scheme and
run `mise run builders:generate`.

Write a `Set*`/`Add*` helper only where the write falls into one of the three admitted classes —
appends to a slice or inserts into a map, assigns a pointer-typed field or initialises a nil
pointer intermediate, or composes an upstream struct under a name that states the opinion.
Everything else is the caller's own field assignment (`obj.Spec.Field = value`), and
`pkg/kubernetes/admission_test.go` fails the build on a helper that fits no class. The four
generic metadata helpers (`SetLabels`, `AddLabel`, `SetAnnotations`, `AddAnnotation`) are admitted
by name; that set never grows.

The normative contract is `pkg/kubernetes/README.md`; the procedure for adding a kind is
`docs/ARCHITECTURE.md` § Developer Guidelines.

### Testing Pattern

Compare the whole object, not a few of its fields — "identity and nothing else" is a claim about
everything the constructor did not write:

```go
func TestCreateNewKind(t *testing.T) {
    got := CreateNewKind("test", "default")

    want := &v1.NewKind{}
    want.GetObjectKind().SetGroupVersionKind(
        schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "NewKind"})
    want.SetName("test")
    want.SetNamespace("default")

    if !reflect.DeepEqual(got, want) {
        t.Errorf("constructor emitted more than identity:\n got %#v\nwant %#v", got, want)
    }
}
```

`pkg/kubernetes/identity_test.go` already asserts this for every registered kind.

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

Required checks: `lint`, `test`, `build`. Merging goes through a GitHub merge queue (rebase method), which rebases and tests the merged result before landing — no manual rebasing needed. See `AGENTS.md` § Git Workflow for full details.

## Quick Commands

```bash
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
- Each pkg/ package has a README.md mounted to the docs site — update it when changing public APIs
- Code and documentation changes must be in the same PR (mandatory, CI-enforced; org standard in go-kure/.github docs/standards.md)
- `site/docs-map.yaml` is the single source of code→docs mapping; the AGENTS reverse-map table + site nav are generated from it (`bash site/scripts/gen-docs-tables.sh`) — never hand-edit them
- Doc-sync is enforced by the canonical check-doc-sync (structure), check-links (rendered links) and check-doc-gate (source change must touch its docs; bypass only via maintainer `docs-skip` label) scripts, consumed from go-kure/.github via composite actions — kure no longer vendors its own copies
- See AGENTS.md "Reverse Mapping" table to know which guides to review when changing a package
## Consumer Compatibility

Before modifying Kure's public APIs (`pkg/`):
1. Follow the organization API stability contract
2. Consider impact on external consumers
3. Keep interfaces stable when possible
4. Coordinate breaking migrations in the consuming repository
