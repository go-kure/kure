# Bootstrap Root Contract — Design Notes

*Date: 2026-10-25 | Type: DESIGN | Scope: kure public API (`pkg/stack`, `pkg/stack/fluxcd`)*

> **Note**: This document is not part of the kure documentation website. It is an internal
> development note about how go-kure/kure#1068 ("define bootstrap generation support that
> eliminates consumer root replacement") is decided. The evidence is
> `pkg/stack/fluxcd/bootstrap_modes_test.go` and the comparisons it stores under
> `pkg/stack/fluxcd/testdata/bootstrap-modes/`.

**Status: proposed.** The maintainer decides on the pull request that adds this note, in
particular between the two contracts under Options considered.

---

## Decision

**kure owns the bootstrap root, and the contract is a supported root contract, not an
installation-only boundary.** A consumer states the root it wants in `BootstrapConfig`, and both modes emit exactly
that root. A consumer never discards a root kure generated and builds another.

The root is one source and one Flux Kustomization that applies the tree. In gotk mode kure emits
both. In flux-operator mode kure emits a FluxInstance, and flux-operator renders both from its
`spec.sync`. In both modes the contract covers:

| Concern | Contract | Today |
|---|---|---|
| Documents | One source and one Kustomization per bootstrap, in both modes | Holds; the modes test enforces it |
| Directory | The tree's top directory (`layout.TopDirectory`), spelled per mode (`demo`, `./demo`) | Holds |
| Namespace | The generator's `DefaultNamespace` for the source, the Kustomization and the FluxInstance. The Kustomization's `sourceRef` names no namespace, so it resolves in its own | Holds |
| Source kind, URL, interval | From `BootstrapConfig`, the same in both modes | Holds |
| Access | `SourceSecretRef`, `SourceProvider`, `SourceInsecure` (go-kure/kure#1060) | Holds, stacked below this PR |
| Names | `SyncName` names the source and the Kustomization in both modes | flux-operator only. gotk names the source after the root node and the Kustomization `BootstrapName` |
| Ref selector | Tag, or a semver range, chosen explicitly | Tag only |
| Prune | An explicit `Prune` reaches the root Kustomization in both modes | gotk only. flux-operator always prunes |
| Verification | OCI signature verification, in both modes | Not expressible |

In flux-operator mode, anything `spec.sync` cannot express reaches the rendered objects as a
targeted `spec.kustomize.patches` entry: a JSON6902 patch on the sync's source or Kustomization,
by kind and name. go-kure/kure#1060 already uses this for plain HTTP. One mechanism covers names,
semver, prune and verification. It is pinned to the flux-operator version whose template the
modes test models.

**Selectors.** ASSUMPTION, to be confirmed on go-kure/kure#1068 by the consumer that asked:
"selectors" means the OCI reference selector (`tag`, `semver`, `semverFilter`, `digest` in
source-controller `OCIRepositoryRef`). This note proposes semver alongside tag. Digest and the
layer selector are the same mechanism and can follow on request.

**Git sources keep today's scope.** go-kure/kure#1060 confines source access to OCI, and this
contract does the same for semver and verification. A Git root with those options is a separate
scope proposal.

## The problem

At `289ba622` plus go-kure/kure#1060, the generator already owns most of the root:

- path (`bootstrapDir`, `bootstrap_generator.go:323-325`);
- namespace (`bg.DefaultNamespace`);
- source kind and ref (`resolvedSourceKind`, `resolvedSyncRef`, `:421-455`);
- access.

The issue's own probe showed that the path and namespace work. The rest is bounded:

- **The modes name the root differently.** gotk takes the source name from the root node
  (`sourceName`, `:355-360`) and the Kustomization name from the generator's `BootstrapName`
  (`:58-63`, `:533`). flux-operator names both `SyncName`, or the FluxInstance's namespace when
  that is empty (flux-operator v0.58.1 `internal/controller/fluxinstance_controller.go:394-398`).
  A consumer that needs one identity across modes cannot state it.
- **Tag only.** gotk sets `OCIRepositoryRef{Tag}` (`:599`). flux-operator renders an OCI sync ref
  as `ref.tag` (`internal/builder/templates.go:437-438`). `SemVer` exists upstream in both
  `OCIRepositoryRef` and `GitRepositoryRef`.
- **Prune diverges.** `BootstrapConfig.Prune` reaches only the gotk Kustomization (`:539`).
  Its godoc promises that unset means `prune: false`. flux-operator renders `prune: true` and
  `interval: 10m0s` regardless (`templates.go:461-463`). So the same unset config garbage-collects
  in one mode and not in the other (`testdata/bootstrap-modes/prune-unset.txt`).
- **No verification.** `OCIRepositorySpec.Verify` exists upstream. kure sets it nowhere, and
  `fluxv1.Sync` has no field for it.

`TestBootstrapModes_RootIdentities` renders nine configs in both modes and reduces each mode to
its root as the cluster sees it. In flux-operator mode that is modelled from the v0.58.1
template. The test holds both modes to one root each. It fails when they disagree on the source
kind, namespace, URL, revision, interval or access, the Kustomization namespace, or the
directory. Every other difference is stored as `<- differs` in the comparison files, so a change
in either mode shows up in review.

The recorded differences today:

- the source name;
- the Kustomization name, with `SyncName` set;
- the path spelling;
- the ref spelling for Git (`branch: main` against `name: refs/heads/main`, the same revision);
- prune;
- the Kustomization interval;
- the plain-HTTP patch;
- without a source URL, gotk still emits the Kustomization, while flux-operator emits no sync.

## Options considered

### 1. Installation-only boundary

kure emits only the installation:
- gotk components;
- flux-operator with a FluxInstance that has no `spec.sync`.

The consumer owns the root, and builds it with the ordinary builders. The ownership rules would be:
- kure never emits a root source or root Kustomization;
- the consumer emits exactly one of each, in the namespace kure installs into;
- `BootstrapConfig.Source*`, `SyncName` and `Prune` are deprecated.

Not recommended:
- It moves a solved problem (path, namespace, kind, ref, access) back to every consumer.
- It breaks every caller that relies on today's root.
- It leaves flux-operator mode with a sync-less FluxInstance, which gives up the operator's own
  reconciliation of its root.

### 2. Supported root contract, with sync expressed by patches where needed — recommended

As in Decision. It is additive:
- each new field defaults to today's output;
- the stored bootstrap output of go-kure/kure#1060 stays byte for byte for configs that set none;
- the modes test records what converges.

The cost:
- coupling to flux-operator's template, through the patch targets;
- a modes model that must be re-read on every flux-operator bump.

### 3. Supported root contract, with gotk only

Make gotk the complete mode and document flux-operator as limited. Rejected: flux-operator is
the default mode (`GenerateBootstrap`, `DefaultFluxMode`). A contract that excludes the default
mode does not remove the consumer's rebuild.

## Prune compatibility

Today an unset `Prune` means no garbage collection in gotk mode and garbage collection in
flux-operator mode. Making flux-operator honour "unset means false" would silently stop garbage
collection on every existing flux-operator cluster that never set `Prune`. The proposed rule:
- an **explicit** `Prune` is honoured in both modes;
- **unset** keeps each mode's default: false for gotk, true for flux-operator;
- the godoc states this instead of promising false.

The maintainer may prefer the stricter rule, unset meaning false everywhere, with a release note.
That is a behaviour change and is recorded as the alternative.

## Follow-ups

Registered before go-kure/kure#1068 closes, each with its completion criteria in the issue:

1. **Prune in flux-operator mode** (go-kure/kure#1088). An explicit `Prune` patches the sync
   Kustomization's `spec.prune`, and the godoc states each mode's unset default.
2. **`SyncName` names the root in gotk mode** as well (go-kure/kure#1089). Unset keeps today's
   names.
3. **A semver ref selector for OCI sources** (go-kure/kure#1090), refused with an explicit tag.
4. **OCI source verification** in both modes (go-kure/kure#1091).
5. **Release and adoption** (go-kure/kure#1092). A kure release ships 1-4. The consumer's adapter
   retirement needs that release plus the consumer's adoption, not the closure of
   go-kure/kure#1068.

Each implementation follow-up is done when:
- the modes test's stored comparison shows the field agreeing, or recorded as an intended
  difference;
- a whole-output test covers the new field in both modes.

## What this does not solve

- **Live behaviour.** No bootstrap is run against a cluster or a flux-operator. The flux-operator
  side of the comparison is a model of its v0.58.1 template, not its output. The issue's own probe
  was local too.
- **Git roots with semver, verification or access**: a separate scope.
- **The controller image pull secret in flux-operator mode**, go-kure/kure#1078: a separate
  defect, unrelated to the sync source.
- **The flux-operator Kustomization interval** (10m0s, fixed): recorded, not part of the
  contract. It is patchable by the same mechanism if a consumer needs it.
