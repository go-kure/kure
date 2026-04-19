# Launcher Extraction — Design Notes

*Date: 2026-04-19 | Type: DESIGN | Scope: kure internal*

> **Note**: This document is not part of the kure documentation website. It is an internal development note about a structural refactoring of the kure module.

---

## Decision

The `pkg/launcher` package, `pkg/patch` package, and the `kurel` CLI (`cmd/kurel`, `pkg/cmd/kurel`) are being extracted from kure into a separate repository: `github.com/go-kure/launcher`.

---

## Rationale

kure is a **library** — a collection of typed Go APIs for building Kubernetes resources programmatically. The launcher and its tooling are an **application** — a CLI tool with its own user audience, design space, release cadence, and dependency footprint. These are different kinds of software and do not belong in the same module.

Concretely:

- `pkg/launcher` does not use `pkg/stack`, `pkg/gvk`, or the GitOps engines — the core of kure. It is a separate system for a different use case.
- `pkg/patch` is only used by `pkg/launcher` and the `kure patch` CLI command. It is not part of kure's public library API for manifest generation.
- The launcher is heading toward an OAM-native redesign (see `github.com/go-kure/launcher/docs/design.md`). Doing this work inside kure would couple kure's stability to an exploratory new project.
- Keeping them separate enables each to have its own versioning, its own issue tracker, and its own contributors without tangling concerns.

---

## What Moves

| Source (kure) | Destination (launcher) |
|---|---|
| `pkg/launcher/` | `pkg/launcher/` |
| `pkg/patch/` | `pkg/patch/` |
| `cmd/kurel/main.go` | `cmd/kurel/main.go` |
| `pkg/cmd/kurel/` | `pkg/cmd/kurel/` |

### Launcher's kure dependencies after the move

Launcher will import kure as an external module (`github.com/go-kure/kure`). The specific packages used:

| kure package | Used by launcher |
|---|---|
| `pkg/errors` | `pkg/launcher`, `pkg/patch`, `pkg/cmd/kurel` |
| `pkg/io` | `pkg/launcher` |
| `pkg/logger` | `pkg/launcher`, `pkg/patch` |
| `pkg/cmd/shared` | `pkg/cmd/kurel` |

These remain in kure. Launcher depends on kure; kure does not depend on launcher.

---

## What Stays in kure

### `pkg/cmd/kure/patch.go`

The `kure patch` CLI subcommand will be **deleted** from kure. kure's CLI demos the kure library; it has no reason to wrap launcher's patch engine. `pkg/cmd/kure/patch.go` and `pkg/cmd/kure/patch_test.go` are removed as part of the extraction. The `runPatchDemo()` function in `cmd/demo/main.go` is also removed.

### `pkg/stack/generators/kurelpackage/`

This is a kure *generator* registered in the `pkg/stack` generator system. It produces kurel package structure as output from a kure Application config — it is a kure concern (generating artifacts from the domain model), not a launcher concern. It stays in kure and has no dependency on `pkg/launcher`.

### Everything else

`pkg/stack`, `pkg/kubernetes`, `pkg/gvk`, `pkg/errors`, `pkg/io`, `pkg/logger`, `pkg/cli` — all unchanged.

---

## Impact on kure users

Consumers of kure who import `pkg/launcher` or `pkg/patch` directly will need to update their import paths from `github.com/go-kure/kure/pkg/launcher` and `github.com/go-kure/kure/pkg/patch` to the corresponding paths in `github.com/go-kure/launcher`.

Consumers of the rest of kure's public API (`pkg/stack`, `pkg/kubernetes`, etc.) are unaffected.

---

## Migration Sequence

1. Set up `github.com/go-kure/launcher` with `go.mod` declaring `module github.com/go-kure/launcher`
2. Copy `pkg/launcher/`, `pkg/patch/`, `cmd/kurel/`, `pkg/cmd/kurel/` into the new repo
3. Update imports in launcher from `github.com/go-kure/kure/pkg/{launcher,patch}` to the new module path
4. Add `github.com/go-kure/kure` as a dependency in launcher's `go.mod`
5. In kure: remove `pkg/launcher/`, `pkg/patch/`, `cmd/kurel/`, `pkg/cmd/kurel/`
6. In kure: delete `pkg/cmd/kure/patch.go`, `pkg/cmd/kure/patch_test.go`; remove `NewPatchCommand()` from `pkg/cmd/kure/cmd.go`; remove `runPatchDemo()` from `cmd/demo/main.go`
7. Tag launcher v0.1.0-alpha.0 once CI passes
8. Tag a new kure release removing the extracted packages (breaking change — minor version bump)

---

## What the launcher becomes

See `github.com/go-kure/launcher/docs/design.md` for the full vision. In short: an OAM-native package manager for Kubernetes, with a clear separation between platform configuration (how traits are implemented) and application configuration (what an app needs). The prototype code being moved is the starting point; the OAM-native design is the direction.
