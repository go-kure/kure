# Dependency Updates Guide

This guide covers the process for updating Kure's dependencies, including version tracking, risk assessment, and coordinated upgrades.

## Version Management Overview

Kure tracks dependency versions across the following files:

| File | Purpose |
|------|---------|
| `go.mod` | Go module dependencies — authoritative for the build version (the pin) |
| `versions.yaml` | Version metadata: supported range, notes (no build version) |
| `docs/compatibility.md` | Generated from `versions.yaml` + `go.mod` — never edit directly |
| `pkg/versions/versions_gen.go` | Generated from `versions.yaml` — never edit directly; run `mise run versions:generate` |
| `renovate.json` | Bot policy: update gates and caps, on top of the shared `go-kure/.github` preset |

The `sync-versions.sh check` command performs six assertions. It runs in CI's `validate`
job, which is gated on the `go` paths-filter in `.github/workflows/ci.yml` — that filter
includes `versions.yaml`, `docs/compatibility.md` and `scripts/sync-versions.sh` precisely
so a version-metadata-only PR cannot skip it:

1. **Range** — each dependency's build version, read from `go.mod`, falls within the
   `supported_range` declared in `versions.yaml`. For a dependency pinned to a Go
   pseudo-version (no semver tags upstream) that also declares `upstream_release` (see
   "Release-pinned dependencies" below), the declared release is substituted for the
   range check instead of being skipped — `supported_range` is enforced for these too.
2. **go.mod pin comment** — the `// Current pin: vX.Y.Z (Kubernetes 1.N)` comment above
   the `k8s.io/api` replace directive matches that directive's actual version. A drifted
   comment used to be caught only by AI review, on every single Kubernetes bump.
3. **No raw commit SHAs in `versions.yaml` notes** — a `notes:` block may not contain a
   bare 40- or 12-hex commit SHA literal. When a dependency has no semver tags (e.g.
   external-secrets, pinned to a `go.mod` pseudo-version), reference "the pseudo-version
   pinned in `go.mod`" instead of writing out the commit by hand — the literal drifts
   silently the next time the pin moves. This is **not** a ban on patch-level semver in
   justification prose (e.g. metallb's "v0.16.0 is a minor release…" stays fine).
   `upstream_release`/`upstream_release_commit` (below) are structured fields, not
   prose, so this ban does not apply to them.
4. **Drift** — the committed `docs/compatibility.md` is byte-identical to what
   `generate` would produce right now. It regenerates into a temp file and diffs;
   your working tree is never touched. On failure it prints the diff (`-` is what is
   committed, `+` is what `versions.yaml` + `go.mod` currently imply) and tells you to
   run `generate`.
5. **Go API drift** — the committed `pkg/versions/versions_gen.go` is byte-identical to
   what `generate_go_api` would produce right now. Same regenerate-into-a-temp-file-and-diff
   shape as item 4 above.
6. **MVS-floor equality** — for a dependency declaring `floor_module` (see "MVS-floor
   dependencies" below), the go.mod pin exactly equals what `floor_module`'s own go.mod
   currently requires. Degrades to a warning when Go or the module cache is unavailable
   (offline dev machine); a reachable mismatch is always an error.

### Release-pinned dependencies (untagged Go submodules)

Some upstream Go modules version their CRD types in a submodule (e.g. `external-secrets`'
`/apis`) that upstream never tags. Go can then only express the dependency as a
pseudo-version, and `@latest` resolves to upstream `main` HEAD — every upstream commit
looks like a new version, and `supported_range` is unverifiable and unenforceable (there
is no semver to compare).

Such a dependency's `versions.yaml` entry adds structured fields to pin it to a
*named release* instead of `main` HEAD:

```yaml
external-secrets:
  go_module: "github.com/external-secrets/external-secrets/apis"
  upstream_repo: "external-secrets/external-secrets"
  upstream_release: "vX.Y.Z"          # the upstream tag this pin tracks
  upstream_release_commit: "<40-char commit that tag resolves to>"
  supported_range: "X.Y"
```

(Values shown as placeholders deliberately — `sync-eso-pin.sh` only rewrites `versions.yaml`
and regenerates `docs/compatibility.md`, never this guide, so a literal example version here
would go stale the next time Renovate advances the pin. See the live entry in
[`versions.yaml`](https://github.com/go-kure/kure/blob/main/versions.yaml) for the current values.)

- `upstream_release` — the upstream tag the pin tracks.
- `upstream_release_commit` — the full commit that tag resolves to. This is what makes
  the check work **offline**: `sync-versions.sh check` asserts that go.mod's
  pseudo-version's embedded 12-char commit digest is a prefix of this field, on every
  run, with no network access. If the pin ever drifted off the declared release (hand
  edit, bad rebase, …), this assertion fails — the actual drift guard, separate from and
  in addition to the range check.
- `upstream_repo` — the `owner/repo` the tag lives in. Used only by the second,
  best-effort check below; a dependency with no `upstream_repo` skips it and keeps only
  the offline digest check.
- `supported_range` is then enforced for the dependency for the first time, against
  `upstream_release` rather than being skipped as unparseable.

The offline digest check proves go.mod matches `upstream_release_commit`, but not that
`upstream_release_commit` is actually the commit `upstream_release` names upstream — a
hand edit that bumps `upstream_release` (and `supported_range`) without re-running
`sync-eso-pin.sh` would satisfy it while `upstream_release_commit` (and go.mod) stay on
the old release. `sync-versions.sh check` closes that gap with a second, **best-effort
online** check: it resolves `upstream_repo`@`upstream_release` live via the GitHub API
(falling back to `git ls-remote`, same resolution as `sync-eso-pin.sh` below, including
the annotated-vs-lightweight tag peel) and compares it to `upstream_release_commit`. It
distinguishes three outcomes, not two — collapsing "server unreachable" and "tag
definitively doesn't exist" into one signal would let a typo'd `upstream_release`
downgrade to a mere warning:
- resolved and matches `upstream_release_commit` — pass.
- resolved and differs — **error**: the field pair is stale independent of go.mod.
- a reachable server confirms the tag does not exist (API 404, or `git ls-remote`
  enumerates the repo's refs with none matching) — **error**: likely a typo in
  `upstream_release`, not a network problem, so must not be downgraded to a warning.
- neither path could reach the server at all (offline dev machine, DNS/connect failure,
  GitHub rate limit) — **warning** only. The offline digest check above already covers
  the common case, and this must never break an otherwise-valid offline run.

`scripts/sync-eso-pin.sh` re-pins such a dependency: it reads `upstream_release`,
resolves the tag to a commit via the GitHub API (falling back to `git ls-remote`),
`go get`s that exact commit (never hand-constructs the pseudo-version string — Go
verifies its embedded timestamp against the commit), runs `go mod tidy`, writes the
resolved commit back to `upstream_release_commit`, and regenerates
`docs/compatibility.md` and the API tables (`gen-builders.sh generate` — the tables
record the module version each kind and field came from, so re-pinning this module
changes them). It is idempotent: a re-run with `upstream_release` unchanged
leaves the tree untouched. `renovate.json` disables the raw `gomod` manager for this
module (its `@latest` is `main` HEAD, not useful) and instead tracks
`upstream_release` via a `customManagers` regex entry on `versions.yaml`, filtered to
plain `vX.Y.Z` release tags (upstream also cuts non-matching `helm-chart-X.Y.Z` tags),
wired to run `sync-eso-pin.sh` as a `postUpgradeTasks` command — so a PR opens when
upstream cuts a release, arriving already re-pinned and range-checked, rather than on
every upstream commit.

`generate` regenerates `docs/compatibility.md` and the `go.mod` pin comment in place.
There is no other hand-maintained "current" version to keep in sync (see
[#593](https://github.com/go-kure/kure/issues/593)).

The drift assertion exists because the matrix is generated but committed: before it, any
bump that moved a build version or a `supported_range` left the committed matrix silently
stale until someone happened to re-run `generate`, and the staleness was caught only by
human review — on three separate dependency PRs.

### MVS-floor dependencies (tagged upstream, but never chosen directly)

A different shape of pseudo-version problem: the upstream repo **does** cut releases, but
nothing in kure ever picks one — the pin is set by Go's minimum-version selection from a
*different* dependency's `go.mod`. `github.com/cloudnative-pg/barman-cloud` is the example:
kure imports it directly, but `github.com/cloudnative-pg/plugin-barman-cloud` also requires
it, at a pseudo-version *past* barman-cloud's last real tag — so MVS floors kure's pin above
any tag that exists, and a hand-chosen tag would actually be a **downgrade** the build
cannot use.

This needs no release-tracking machinery, because there is no release to track — the pin
should simply always equal the floor:

```yaml
barman-cloud:
  go_module: "github.com/cloudnative-pg/barman-cloud"
  floor_module: "github.com/cloudnative-pg/plugin-barman-cloud"
  supported_range: "0.5"      # major.minor only -- see is_pseudo_version() note below
```

- The pin itself is **derived, never hand-written**: `go mod edit -droprequire=<module> &&
  go mod tidy` re-computes it from whatever the floor-setting dependency currently
  requires. Re-run this whenever the floor-setting dependency (here,
  `plugin-barman-cloud`) bumps.
- `sync-versions.sh check` mechanically asserts this: with `floor_module` set, it reads
  `floor_module`'s own `go.mod` and asserts it requires `go_module` at exactly the pin's
  version — not just the `major.minor` range check the other assertions apply. It
  degrades to a warning when Go or the module cache is unavailable (offline dev machine),
  since that is "could not check," not "checked and it's wrong" — but never when the
  check actually runs and finds a mismatch. A plugin **downgrade** will legitimately fail
  this guard until the pin is re-derived with the `-droprequire` + `tidy` recipe above.
- `supported_range` is expressed in plain `major.minor` because this dependency's
  pseudo-version does **not** match `is_pseudo_version()`'s regex (see the comment above
  that function in `scripts/sync-versions.sh`) — it falls through to the ordinary range
  path, so the check runs unmodified, unlike the release-pinned case above where the range
  check is substituted onto `upstream_release`.
- `renovate.json` disables the standalone `gomod` bump for the module
  (`matchPackageNames` + `enabled: false`, same shape as the release-pinned rule above).
  The shared preset's `postUpdateOptions: ["gomodTidy"]` re-raises the require to the new
  floor automatically whenever the floor-setting dependency's own PR runs `go mod tidy` —
  so the pin still moves, inside that dependency's own reviewable PR, never on its own.
- Without the `versions.yaml` entry, `validate_gomod()`'s loop — keyed on `.infrastructure`
  keys — never visits the module at all: not skipped, not warned, invisible. That silence
  is what let `barman-cloud` digest-bump unreviewed for months before this pattern existed
  (the shared preset's `go-patch` group automerges `gomod` patch/digest bumps).

### Testing the guards

`scripts/sync-versions.sh` has six guards (`validate_gomod`, `validate_gomod_pin_comment`,
`validate_no_sha_in_notes`, `validate_mvs_floors`, `validate_docs_drift`,
`validate_go_api_drift`), and CI only ever runs them against a repo state where they all
pass. Nothing proves a guard actually **fails** when it should — a guard that stops
guarding (a dropped flag, a loosened regex, a silently-skipped branch) looks identical to
one still doing its job. `scripts/test/` closes that gap with a hermetic, network-free
mutation-matrix harness: each `scripts/test/cases/*.sh` file builds a synthetic fixture
tree, sets up exactly one scenario — a fixture mutation, a stub `go`/`curl`/`git` mode, or
the unmutated baseline — runs `sync-versions.sh`, and asserts both the exit code and the
specific message the guard must emit (`assert_err_contains` for a failure case,
`assert_out_contains` for one that must stay green).

Run it locally with:

```bash
mise run versions:test
```

It's part of `mise run verify` and runs in CI as its own step, gated on
`scripts/test/**`/`scripts/sync-versions.sh`/`versions.yaml`/`docs/compatibility.md`
changes (same path filter as `sync-versions.sh check` itself).

**Adding a case:** every case follows the same shape — see `scripts/test/lib.sh` for the
full helper reference (`new_fixture`, `yq_set`, `gomod_sub`, `run_check`/`run_generate`,
the `assert_*` family, `with_stub_go`/`with_stub_net`):

```bash
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
yq_set '.infrastructure.<dep>.<field>' '<mutated value>'   # or gomod_sub for go.mod
run_check
assert_rc 1
assert_err_contains '<the exact error substring the guard must emit>'
```

**A new guard lands with a case proving it fails.** A guard with no case exercising its
failure path is, by this harness's own standard, unproven — the whole point is that CI
should never again be able to report green while a guard silently stopped guarding.

**Portable-environment prerequisites (`timeout`, `mktemp`).** The harness itself needs no
network. Without a `timeout`/`gtimeout` binary on `PATH`, `sync-versions.sh check` runs its
two bounded probes (the MVS-floor `go list` lookup and the tag-resolution `git ls-remote`
fallback) unbounded instead, and says so once at startup — `generate` never reaches either
probe, so it is unaffected either way. This is production-runtime graceful degradation, not
a hard requirement — see `DEVELOPMENT.md`'s Prerequisites block for the full split.

That degradation does **not** extend to the test harness itself: pre-existing case
`09-mvs-floor-hang-timeout.sh` and this PR's own `36-timeout-present-no-warning.sh` both need a
real `timeout`/`gtimeout` on the machine *running the test suite* to pass — this is a
test-harness-only prerequisite, narrower than and separate from production's graceful
degradation, where a real stalled module proxy or git remote has no such bound.

For case 09: it stubs `go list` to sleep 5s (faking a hang) and asserts
`SYNC_VERSIONS_PROBE_TIMEOUT` bounds it to ~1s. On a host with neither binary, the probe isn't
bounded, so the stub's 5s sleep runs to completion and `go list` "succeeds" (echoing a `go.mod`
path) — but the case does **not** then fail on its warning-text assertion. The unbounded
success reaches step 4 (`go mod edit -json` against that path), which the same stub fails for
every mode but `ok`/`norequire`/`editfail`; `validate_mvs_floors` reports this as a hard
**error** ("could not read ... go.mod requirements ... rc=3"), not the "could not resolve ...
skipping" **warning** the case asserts, so `check` itself exits 1. The case therefore fails on
its earlier `assert_rc 0`, not on the warning-text assertion, in ~6-11s (the 5s sleep plus the
rest of `check`'s own guards) — confirmed by direct reproduction, not by hanging indefinitely.
For case 36: with no real `timeout`/`gtimeout`, `resolve_timeout_bin` cannot resolve `TIMEOUT_BIN`
and emits its startup warning, which is exactly the text `36-timeout-present-no-warning.sh`
asserts is *absent* — so the case fails outright rather than skipping.

`mise registry` maps `coreutils` to `aqua:uutils/coreutils`; checked directly against a local
install, that backend provisions one multi-call `coreutils` binary (a `timeout` applet is
reachable only via `coreutils timeout ...`), never a standalone binary literally named
`timeout` — so it is **not** pinned in `mise.toml` for this. `yq` remains the only
externally-required tool mise provisions for this script.

## Update Risk Levels

### Patch Updates (Low Risk)

Patch bumps (e.g., v1.5.0 → v1.5.1) contain bug fixes only.

```bash
go get <module>@v<new-version>
go mod tidy
```

No `versions.yaml` change is needed for an in-range patch — the build version comes from
`go.mod`, and `sync-versions.sh check` passes as long as the new version stays within
`supported_range`. (This is what lets in-range Renovate patch bumps go green untouched.)

### Minor Updates (Medium Risk)

Minor bumps (e.g., v1.19 → v1.20) may add new APIs or deprecate existing ones.

1. Review the upstream changelog for breaking changes
2. Check if Kure uses any deprecated APIs
3. Update `go.mod`; if the new version lands **outside** `supported_range`, widen the
   range and update `notes` in `versions.yaml` (only after confirming API compatibility)
4. Run `make verify` to catch compile-time breakage

### Major Updates (High Risk)

Major bumps (e.g., v1 → v2) likely have breaking API changes.

1. Review the migration guide thoroughly
2. Assess impact on all callers (check with `grep -r` for imports)
3. Update code to use new APIs
4. Update `versions.yaml` and documentation
5. Consider impact on external consumers (see `AGENTS.md` § Consumer Compatibility)

## Coordinated Upgrade Rules

Some dependencies must be upgraded together to avoid version conflicts.

### Flux Ecosystem

All `github.com/fluxcd/*` packages must be upgraded together. Flux releases coordinate versions across:
- `flux2/v2`
- `helm-controller/api`
- `kustomize-controller/api`
- `notification-controller/api`
- `source-controller/api`
- `image-automation-controller/api`
- `pkg/apis/meta`, `pkg/apis/kustomize`

Renovate enforces this with the `fluxcd` group in the shared
`go-kure/.github` preset, which covers `github.com/fluxcd/*` and
`github.com/controlplaneio-fluxcd/*`. Ungrouped, one upstream release arrived as five
separate PRs whose `go.mod` changes conflicted with each other in the merge queue and
had to be consolidated by hand. Flux minors are additionally dashboard-gated by
`renovate.json` (majors are gated org-wide by the preset), so the group only ever
carries patches unless a pending minor is deliberately approved on the dashboard.

### Kubernetes (`k8s.io/*`)

All `k8s.io/` packages must stay at the same patch release. Kure uses `replace` directives in `go.mod` to enforce this. See the comment block in `go.mod` for details.

**When can replace directives be removed?** Only when ALL direct and transitive dependencies converge on the same `k8s.io/` minor version. Check with:

```bash
go mod graph | grep 'k8s.io/' | awk '{print $2}' | sort -u
```

### CNPG Ecosystem

`cloudnative-pg`, `barman-cloud`, `machinery`, and `plugin-barman-cloud` are related but versioned independently. Check compatibility notes in `versions.yaml` before upgrading.

### Vendored `go-kure/.github` Guard

`.github/workflows/ci.yml`'s `forbidden-terms` job pins `go-kure/.github` twice: once as
the `check-forbidden-terms` action's `uses:` digest (tracked by Renovate's
`github-actions` manager) and once as a second checkout's `ref:`, which that manager
cannot see. Left alone, the two drift apart and the job byte-compares the vendored
`site/scripts/check-forbidden-terms.sh` against a stale revision.

`renovate.json` closes the gap with a `customManagers` regex entry that tracks the
`ref:` SHA as a `go-kure/.github` `git-refs` dependency, grouped with the `github-actions`
bump via a `packageRules` entry (`matchDepNames: ["go-kure/.github"]`) so both pins move
in the same PR. That same rule's `postUpgradeTasks` runs `./scripts/vendor-guard.sh` on
the bot's branch, which re-fetches `scripts/check-forbidden-terms.sh` from
`go-kure/.github` at the new `ref:` SHA and re-vendors it to `site/scripts/`. The script
is idempotent — a re-run against an already-synced tree makes no further change.

## Bundling Renovate PRs

Renovate's ecosystem groups already land related bumps as one PR, so bundling is
rarely needed. When separate PRs still accumulate (e.g. several groups at once),
bundle them into a single PR:

1. Create a feature branch: `git checkout -b chore/bundle-dependency-updates main`
2. Run `go get` for all dependencies (Flux packages first for coordinated upgrades)
3. Run `go mod tidy`
4. Update `versions.yaml` `supported_range` / `notes` for any bump that lands outside its range
5. Regenerate docs: `./scripts/sync-versions.sh generate`; regenerate the constructor wrappers and the API tables if an API module moved: `./scripts/gen-builders.sh generate`
6. Validate: `./scripts/sync-versions.sh check` and `./scripts/gen-builders.sh check`
7. Run full verification: `make verify && make test-race`
8. Commit, push, and create PR
9. Reference all Renovate PR numbers in the PR body to auto-close them

## The Generated API Tables

An API module bump can change what kure publishes about that API, so
`scripts/gen-builders.sh generate` runs in Renovate's `postUpgradeTasks` and rewrites
four generated artifacts: the per-kind constructor wrappers, `pkg/kubernetes/zz_generated_tables.go`,
`docs/api-tables.json` and `docs/api-tables.md`. CI's `validate` job runs
`scripts/gen-builders.sh check`, which fails on any of them being stale. All four are
in the `fileFilters` of that `postUpgradeTasks` block — a path missing there is dropped
from the bot's commit silently, never as an error, and surfaces only as a red `check`.

That block matches the `gomod` and `mise` managers, which is every API-module bump but
one: external-secrets arrives through the `customManagers` regex entry on
`versions.yaml` (above), so it matches by manager name nowhere in that rule. Its own
rule therefore carries the same artifacts in `fileFilters`, and `sync-eso-pin.sh` chains
`gen-builders.sh generate` itself, so a manual re-pin regenerates them too.

Two things about those tables specifically:

- **They are derived from two upstream sources**, both read out of the pinned module as
  unpacked in the local module cache: the `+kubebuilder:resource` markers in the Go
  source, and the `CustomResourceDefinition` manifests the module ships. A kind that a
  bump leaves with neither is a **hard generation failure** on that PR, not a silent
  default — see `pkg/kubernetes/README.md` § 9. If a bump fails to generate for that
  reason, the fix is to establish the kind's real scope from upstream, not to make the
  generator guess.
- **A table change trips the doc gate.** `zz_generated_tables.go` lives in the doc-gated
  `pkg/kubernetes` package, and the `// doc-gate:trivial` exemption cannot cover it: that
  exemption applies only to lines containing `=` whose declaration prefix is unchanged,
  and a table row has no `=`. For a bump whose only effect on these artifacts is version
  churn, a maintainer applies the `docs-skip` label. A bump that adds, removes or
  re-scopes a kind is not version churn and should update the prose too.

## Dangerous Upgrades to Watch For

| Dependency | Risk | Watch For |
|-----------|------|-----------|
| cert-manager major (v1 → v2) | Breaking | API group changes, CRD schema changes |
| k8s.io major (e.g., v0.35 → v0.36) | Breaking | API removals, type changes, replace directive updates |
| Flux major (v2 → v3) | Breaking | API version removals (v1beta1 → v1 migrations) |
| controller-runtime major | Breaking | Interface changes affecting all CRD-based packages |

## Validation Checklist

Before merging any dependency update:

- [ ] `./scripts/sync-versions.sh check` — build versions within `supported_range`, the
      `go.mod` pin comment and `versions.yaml` notes match (run `generate` first if it
      reports drift; reword any raw commit SHA in `notes:` to reference `go.mod` instead)
- [ ] `./scripts/gen-builders.sh check` — the constructor wrappers and the API tables
      match the pinned modules (run `generate` first if it reports drift)
- [ ] `make verify` — tidy + lint + test
- [ ] `make test-race` — race condition detection
- [ ] k8s.io replace directives unchanged (unless intentionally bumping)

## See Also

- [Development Guide § Renovate Management](/contributing/guide/#renovate-management) — dashboard workflow for managing Renovate PRs
- [Compatibility Matrix](/api-reference/compatibility/) — Generated compatibility matrix
- [versions.yaml](https://github.com/go-kure/kure/blob/main/versions.yaml) — Version source of truth
