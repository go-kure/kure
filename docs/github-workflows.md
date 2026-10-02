# GitHub Workflows Documentation

This document provides an overview of all GitHub Actions workflows used in the kure project.

**Last Updated:** 2026-10-02

---

## Workflow Summary

| Workflow | File | Triggers | Purpose |
|----------|------|----------|---------|
| [CI](#ci-workflow) | `ci.yml` | push, PR, schedule, manual | Comprehensive testing, linting, building, security |
| [Deploy Docs](#deploy-docs-workflow) | `deploy-docs.yml` | push to main (docs paths), `workflow_dispatch` | Multi-version docs deployment |
| [Manage Docs](#manage-docs-workflow) | `manage-docs.yml` | `workflow_dispatch` | Remove or rebuild doc versions |
| [Release](/contributing/releasing/) | `release.yml` | manual | The one manual release workflow: release, promote, or start the next version |
| [Release / Publish](/contributing/releasing/) (automatic on tag) | `release-publish.yml` | tag push, `workflow_dispatch` | GoReleaser (release object only — no artifacts, see [Releasing](#releasing)), docs deploy, proxy refresh |
| [Release / State](/contributing/releasing/) | `release-state.yml` | `workflow_dispatch` (tag) | Read-only report of what a tag's publish run did, in the job summary |
| [PR Review](#pr-review-workflow) | `pr-review.yml` | pull_request, merge_group | AI code review via the shared workflow; see [Shared Workflows](/contributing/shared-workflows/) |
| [Claude](/contributing/shared-workflows/) | `claude.yml` | issue/PR comments, reviews, issues opened or assigned | `@claude` assistant via the shared workflow |

---

## CI Workflow

**File:** `.github/workflows/ci.yml`
**Name:** `CI`

### Triggers

- Push to: `main`, `develop`, `release/*`
- Pull requests against any base branch, on types
  `opened`, `synchronize`, `reopened`, `labeled`, `unlabeled`
- Merge group (merge queue's temporary branch — required checks must report here)
- Schedule: 4am UTC daily (catch external changes)
- Manual dispatch

Every job runs on draft PRs the same as ready ones (2026-08-19, GitLab `mr-review` parity — see
[Draft PRs](#draft-prs)), so `ready_for_review` is not declared: it would only re-trigger a suite
that already ran.

`labeled` and `unlabeled` are declared so that adding the `pin-impact` job's `pin-impact-ack`
override label starts a new run — without them the required `build` check stayed failed until an
unrelated push, so the acknowledgement path existed but nothing re-evaluated it. The cost is that
**any** label change reruns the whole pipeline, not just a `pin-impact-ack` one.

**No `branches:` filter on `pull_request`, so a stacked PR gets the full suite** (go-kure/kure#798).
A `branches:` filter matches the PR's *base*, so with one a PR based on another feature branch ran
none of this workflow's jobs while `pr-review` (which has no filter) still reported green. Now `CI`
runs on a PR whatever its base, from its first push. When the base merges and GitHub retargets the
PR to `main` (sending `edited`, which is not in the `types:` list), nothing needs re-running: the
required checks already ran on the PR's head commit, and the merge queue re-tests the merged
result against `main` anyway.

### Concurrency

Uses `github.ref` to cancel superseded runs on the same branch or PR:

```yaml
concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true
```

### Job Dependency Graph

Boxes are job ids from `ci.yml`; where a job's check name differs, it follows in parentheses.

```
                   ┌──────────────────────────┐
                   │ changes (detect-changes) │  ← which paths the change touches
                   └────────────┬─────────────┘
       ┌───────────────┬────────┴──────┬────────────────┐
       ▼               ▼               ▼                ▼
┌────────────────┐ ┌────────┐ ┌─────────────────────┐ ┌────────────┐
│ validate (lint)│ │ test   │ │ security (Security) │ │ docs-build │  ← each needs only changes,
└────────────────┘ └───┬────┘ └─────────────────────┘ └────────────┘    so they run in parallel
                       ▼
             ┌──────────────────┐
             │ coverage-check   │  (Coverage Check)
             └──────────────────┘

No needs, start at once:  action-pins, forbidden-terms
PR-only, no needs:        doc-gate, analyze-changes (Analyze Changes)
PR and queue, no needs:   pin-impact

                 ┌───────┐
                 │ build │  ← aggregation gate (if: always()): validate, test, coverage-check,
                 └───────┘    security, docs-build, doc-gate, action-pins, forbidden-terms,
                              pin-impact
```

The PR-only job `doc-gate` still feeds `build`; on a push or merge-queue run it is skipped, and
`build` accepts `skipped` from every job except `forbidden-terms`. `pin-impact` is skipped on a
push; on a merge-queue run it checks only that the merged tree's `go-kure/.github` pins agree (see
the pin-impact gate below). `analyze-changes` is informational and feeds nothing.

On `merge_group` events (merge queue), `lint`/`test`/`build` run against the queue's
temporary branch — the merged result — before the PR is allowed to land.

### Jobs Detail

| Job | Check Name | Timeout | Dependencies | Purpose |
|-----|------------|---------|--------------|---------|
| `validate` | `lint` | 25 min | changes | Go fmt, tidy, vet, lint, tool-version parity (golangci-lint pin across Makefile/ci.yml/docs), govulncheck doc parity; `sync-versions.sh check`; `scripts/gen-builders.sh check` (fails when the generated constructor wrappers under `pkg/kubernetes` are stale against the registered scheme); syntax-check on the version-sync scripts, `sh -n` or `bash -n` per each script's own shebang; `scripts/test/run-tests.sh` (hermetic mutation-matrix guard tests for `sync-versions.sh`'s own six guards, no network); caches goimports + yq binaries |
| `action-pins` | `action-pins` | 2 min | — | Fails if any third-party `uses:` ref is not pinned to a 40-char commit SHA (`go-kure/.github` canonical checker) |
| `forbidden-terms` | `forbidden-terms` | 2 min | — | Runs the canonical full-tree downstream-reference guard on every workflow event and verifies the vendored release guard and release guide against `go-kure/.github` at a ref derived from the guard action's own pin (go-kure/kure#813) |
| `test` | `test` | 20 min | changes | Unit tests with race detection and coverage; `-race` compilation takes ~5 min on the in-cluster runner, so 20 min allows compilation + 15 min for test execution |
| `security` | `Security` | 15 min | changes | govulncheck (`-scan symbol`, v1.8.0), gated on reachable advisories via the canonical `govulncheck-gate` action from `go-kure/.github` — blocking, not informational; the accepted-risk entries (`GO-2026-5377`, `GO-2026-6596`) carry their justification inline in `ci.yml` |
| `coverage-check` | `Coverage Check` | 5 min | test | Two separate gates — 90% total coverage, and 90% on each individual package — plus Codecov upload and PR comment |
| `build` | `build` | 1 min | validate, test, docs-build, coverage-check, doc-gate, action-pins, forbidden-terms, security, pin-impact | Aggregation gate — fails if any required job failed; `forbidden-terms` must report success and may not be skipped |
| `analyze-changes` | `Analyze Changes` | 5 min | - | Changed files analysis, line counts against the PR's own base branch (for example `main` or a `release/vX.Y` branch), breaking change warnings (PR only) |
| `docs-build` | `docs-build` | 15 min | changes | Hugo build; separate Go + Hugo caches; validates the docs map and rendered internal links via the canonical `check-doc-sync`/`check-links` actions from `go-kure/.github`, the documented API references via `scripts/check-doc-api-refs.sh`, the Go blocks of every documentation page, generated from `Example` functions or marked as excerpts, via `scripts/gen-doc-examples.sh`, and absolute links to the site itself via `scripts/check-site-self-links.sh` |
| `doc-gate` | `doc-gate` | 5 min | — | API changes need docs check (PR only; no `needs`, not path-filtered); runs the canonical `check-doc-gate` action from `go-kure/.github`. Bypass via the maintainer `docs-skip` label, or automatically for a generated-table row whose only change is a provenance field (`ModuleVersion` — pure version churn from a dependency bump); adding, removing, or re-scoping a kind is not exempt |
| `pin-impact` | `pin-impact` | 3 min | — | On a merge-queue run, only checks that every `go-kure/.github` reference in the merged tree pins the same commit. On a PR, resolves every `go-kure/.github` action kure's workflows reference to the `scripts/*.sh` (and the sibling scripts those `source` or run, transitively) each runs, compares base vs. head, and fails if the pin bump touched a path kure actually executes — vendored `scripts/check-pin-impact.sh` (not a canonical action: it must run at the SHA it's vetting, not the SHA a bump would move it to) |

### Configuration

- Go Version: read from `go.mod` (`go-version-file: go.mod`)
- Golangci-lint Version: `v2.14.0`
- govulncheck Version: `v1.8.0` (pinned, cached binary, `-scan symbol` mode)
- Coverage Threshold (total): `90%` — the overall figure from `go tool cover -func`
- Coverage Threshold (per-package): `90%` — checked separately for every package, and a single
  package below it fails the job even when the total passes. Packages whose import path contains
  `/examples/` are exempt.

### Features

- **Independent jobs** - `validate`, `test` and `security` each depend only on `changes`, so a lint
  failure in `validate` does not stop `test` or `security` from running; the `build` gate reports
  all of them together
- **Artifact sharing** - Coverage is uploaded as an artifact and reused by `coverage-check`; upload,
  download, missing-file, and invalid-profile failures are blocking so the coverage gates cannot
  pass without valid data
- **PR comments** - Coverage report comment on PRs
- **Runs on draft PRs** - no draft gate on any job (see [below](#draft-prs))
- **Credential detection** - GitHub **secret scanning** and **push protection**, both enabled on
  this repository, not a CI step. Where push protection applies it is the stronger control: it
  blocks the push, where a CI step can only report once the secret is already in the history. It is
  **not a superset**, and it is narrower than secret scanning itself — GitHub's scope note is
  *"Push protection only blocks leaked secrets on a subset of the most identifiable user-alerted
  patterns"*, and separately *"push protection only supports the most recent token versions that
  secret scanning can identify with confidence"*. Matching a supported provider pattern therefore
  earns an alert, not necessarily a block. Beyond those patterns, a hand-rolled credential is not
  covered by either control. `secret_scanning_non_provider_patterns`
  is disabled here, but enabling it would not close that gap: non-provider patterns are a defined
  inventory — private keys, database connection strings, HTTP Basic/Bearer headers — not arbitrary
  high-entropy strings, and GitHub states plainly that *"push protection and validity checks are not
  supported for passwords"*. Enabling that setting, and `secret_scanning_validity_checks` alongside
  it, is a repository-settings decision rather than a change to this repository's contents; it would
  widen detection, not make an arbitrary password blockable. The removed step did not cover the gap
  either: at most it could have *printed* such a line, if the line happened to contain one of its
  four words and was among the ten it showed. That step grepped `password|secret|token|key` over
  `*.go`/`*.yaml`/`*.yml`: a match produced an `::warning` annotation rather than a build failure,
  and `head -10` capped it at ten printed lines out of thousands matching, picked by traversal
  order — on every run observed, those ten were all from this workflow file itself. It was removed
  in go-kure/kure#788, which records the full analysis
- **goimports** - Installed as a tool dependency for the formatting check (`goimports -l`)
- **Doc-sync checks** - `docs-build` and `doc-gate` run the shared `check-doc-sync`,
  `check-links` and `check-doc-gate` actions; see [Shared Workflows](/contributing/shared-workflows/)
- **API-reference check** - `docs-build` also runs `scripts/check-doc-api-refs.sh`, which fails
  when a live page names a `Create*`/`Set*`/`Add*` function `pkg/**` no longer exports, or any
  exported name it qualifies with the base name of a package under `pkg/` (`layout.Config`,
  `errors.ParseErrors`) that the package does not declare. A variable spelled like a package is read
  as the package, so a snippet naming one fails until the variable is renamed; standard-library
  names whose package shares a base name with one of kure's (`errors.Is`, `io.Reader`) are listed
  in the script's `EXTERNAL_QUALIFIED`. It is the
  kure-specific complement to `check-doc-sync`: that action proves every package has a page, this
  one proves the pages describe the API that shipped. The page set comes from `site/docs-map.yaml`
  (so it needs `yq`, installed earlier in the same job) plus the repository-root Markdown — every
  `*.md` there, not just `README.md`, since `AGENTS.md` carries worked examples that agents follow
  — `.claude/CLAUDE.md` (and any other `*.md` directly in `.claude/` except `*.local.md`), which
  every agent loads before editing, and the `docs/`, `examples/` and `site/content/` trees plus
  every Markdown file under `pkg/`,
  again not just the READMEs: `pkg/stack/DESIGN.md` describes the shipped design and
  `pkg/stack/STATUS.md` says in its own first line that it reflects current implementation state, so
  a page mounted from a new directory cannot escape the set and neither can one that sits beside the
  code without being mounted at all. The public Go files under `pkg/` and `examples/` are in the
  set too — pkg.go.dev publishes the former's doc comments, and an example's instructional comment
  sits beside the call it describes — and only their comment lines are read, since code calling a
  removed function does not compile. That includes suppression markers: marker text in Go code (a
  string literal) is inert. Every `doc-api-refs` marker on a line is validated before any is
  honoured, so a malformed one cannot hide behind a valid neighbour, and a marker whose body holds
  another `<!--` is an error. A reference
  written with a package selector (`fluxcd.CreateGitRepository`) is resolved in the
  package that selector names rather than anywhere in the tree, so a helper that moves or is
  removed from one package is not answered by a same-named declaration in another. A reference
  written with an exported type selector that names a type declared under `pkg/`
  (`LayoutIntegrator.CreateLayoutWithResources`) is resolved against that type's own methods — in
  the page's own package first when it lives in one declaring the type, otherwise in any package
  that does — so a method that moves between types is not answered by its old name on the other;
  the index keeps the receiver type of every method, and every exported non-interface,
  non-alias type from its own declaration, so a type whose last exported method moved away still
  counts as a type and the stale reference fails. The index is built by parsing the Go source —
  `scripts/docapiindex`, a `go/parser` walk the check builds with the job's Go toolchain — not by
  matching its text: a `func Create…` line inside a block comment or a raw string is not a
  declaration, where a text match indexed it and so let a page keep naming the function after the
  real one was deleted; and declarations in grouped `const`/`var`/`type` blocks, parenthesised
  receivers and type-parameter lists holding a bracket are read like any other. Exported consts,
  vars and types answer a reference as functions do, since a page naming one names live API. A Go file under `pkg/` that does not parse fails the run rather
  than dropping out of the index. A reference
  written with a selector that is neither — a variable, a field, a type from another module — or
  without one still resolves tree-wide, because an import alias and a variable receiver are
  spelled alike. The generic constructor is recognised both qualified (`kubernetes.Create[T]`)
  and bare (`Create[T]`); a bare `Set[...]`/`Add[...]` is not, being type syntax elsewhere. Dated records under `docs/history/` and `docs/reviews/`, the generated `CHANGELOG.md` and
  the two proposal documents are exempt by name in the script, each with its reason. The release-1
  migration ledger and the release-2 migration notes are not exempt: they name removed functions and
  their live replacements side by side, so excluding them would stop checking the replacements — the
  names a caller actually types. The exemption is per name instead of per page, and the names are written down in
  `scripts/doc-api-refs-removed.txt` rather than inferred from where they sit on the page. Inferring
  them does not work: that page's tables are not all removal tables — some pair an old signature
  with a new one under the same name, some pair a builder that still ships with the field it no
  longer sets, and some list the helpers that stay and say why — so a positional rule exempts about
  thirty live builders the page recommends. Being written down also makes the set a snapshot: a
  derived set would grow by itself, so deleting a builder tomorrow would make it exempt on that page
  the same day, silently. The list is validated rather than trusted — every name in it must be
  absent from `pkg/`, and a live name there fails the run with the names to drop — and a removed
  name the ledger mentions but the list omits fails the run too, with the name to add. The step
  runs `--self-test` first, which pins the extractor, the declaration index and the
  resolver against a synthetic tree — a fence marker that matches too much, an identifier boundary
  that stops matching, or a selector that stops being carried would otherwise turn the repo run
  quietly green. Every malformed suppression is an error rather than a silent pass: an
  `ignore-start` with no terminator, an `ignore-end` with nothing open, a reversed pair on one line,
  a second `ignore-start` inside an open fence, which would otherwise be closed by the first
  `ignore-end` and leave the outer fence open with nothing said, and the same repetition written on
  a single line, so the spelling never decides whether a malformed suppression is an error
- **Doc-example check** - `docs-build` also runs `scripts/gen-doc-examples.sh --self-test` and
  `--check`. Every page is enabled: the script's `ENABLED_PAGES` list is every tracked Markdown file
  except its `EXEMPT_PAGES` (`.github/`, and the dated records and proposals that
  `scripts/check-doc-api-refs.sh` excludes too), and `--check` fails when the list and the tracked
  files drift apart, so a new page with a Go block cannot go unchecked by not being listed. On
  every enabled page, each ```` ```go ```` block is either
  generated from an `Example` function (between `<!-- doc-example: <pkgdir> <ExampleName> -->` and
  `<!-- doc-example:end -->`) or sits directly under `<!-- doc-example:excerpt <reason> -->`. The
  generated block is the function body with its `// Output:` comment dropped, and the `test` job's
  `go test` runs the function, so a snippet on an enabled page compiles and prints what its
  `Output` says. `--check` fails on an unmarked block (`go` or `golang`, any case), an excerpt
  without a reason or not directly above a ```` ```go ```` fence, a doc-example marker with no end
  marker, an unrecognised marker, a missing Example and a block that drifted from its function;
  `scripts/gen-doc-examples.sh` with no argument rewrites the blocks.
  The self-test covers the page-list comparison and runs `go test ./scripts/docexamples`;
  `mise run site:check-doc-examples` runs both locally
- **Site self-link check** - `docs-build` also runs `scripts/check-site-self-links.sh`
  (`mise run site:check-self-links` locally), which fails when a published page links to the docs
  site itself with an absolute or scheme-relative URL. Only `CHANGELOG.md` and `cliff.toml`, which
  are also read outside the site, may link to the dev slot; every other page links
  version-relatively. `check-links` cannot see these: it runs
  lychee `--offline`, which skips every `http(s)` URL. They break because the release root is
  rebuilt only by a `set_latest=true` deploy (see [Versioned Documentation](#versioned-documentation)), so an
  absolute link into it 404s for every page added since the last release. The page set is every
  package README with a `mount:` block and every extra mount in `site/docs-map.yaml` (needs
  `yq`), every file under `site/content/`, and `cliff.toml`, whose header template writes the
  links at the top of `CHANGELOG.md`; a `mounted: false` README is not published and not checked.
  A page lister that fails part-way stops the run rather than shortening the set. The site base
  comes from `site/hugo.toml`'s `baseURL`. The step runs `--self-test` first, which pins the URL
  classifier and those failure paths against a synthetic tree
- **Downstream-reference guard** - the unconditional `forbidden-terms` job runs the shared guard
  and byte-compares the three vendored copies (`site/scripts/check-forbidden-terms.sh`,
  `docs/releasing.md`, `docs/shared-workflows.md`); the mechanism is on
  [Shared Workflows](/contributing/shared-workflows/)
- **Pin-impact gate** - `pin-impact` renders a `go-kure/.github` pin bump's real effect (which
  `scripts/*.sh` a referenced action actually runs, whether the compare touches any of them) into
  the job summary and fails on a match, so a bump touching consumed code cannot merge unreviewed
  (go-kure/kure#719, 2026-08-30). It follows three things, and nothing else:
  - **Pins.** It reads `uses: go-kure/.github/.github/actions/<subpath>@<sha>`, including nested
    or dotted subpaths. It also reads the `ref: <sha>` in the `with:` mapping of a block-style
    checkout whose same `with:` mapping holds `repository: go-kure/.github`, whatever the key
    order. The repository name is matched case-insensitively and with or without a trailing `.git`
    (`actions/checkout` clones `https://github.com/<repository>`, the same repository either way),
    and the SHA may be written in either case. Keys are read as YAML reads them: `"uses":`, `'ref':` and `repository :` are
    the plain keys, in a workflow and an `action.yml`.
  - **Action scripts.** It follows each `$GITHUB_ACTION_PATH/<rel>.sh` or
    `${GITHUB_ACTION_PATH}/<rel>.sh` in an action's single `run:` step, written as one whole word:
    the path, at most a closing quote, then whitespace, `;&|)<>` or the line end. The word must be
    the command run: at a line start or after a separator, optionally behind a shell keyword
    (`if`, `then`, `else`, `elif`, `do`, `while`, `until` or `!`) and `exec`, `bash`, `sh`,
    `source` or `.` with options. The path is resolved from the
    action's own directory, counting its `..` hops.
  - **Sibling scripts.** It follows, transitively, a whole line
    `source "$SCRIPT_DIR/<name>.sh"` or `[exec] [bash|sh] "$SCRIPT_DIR/<name>.sh" [args]`. It
    trusts only `SCRIPT_DIR` defined as exactly `$(dirname "$0")` or
    `$(cd "$(dirname "$0")" && pwd)`, optionally with `&>/dev/null`, `>/dev/null`,
    `>/dev/null 2>&1` or `2>/dev/null` before the `&&` and `pwd -P` for `pwd`, and either with
    `"${BASH_SOURCE[0]}"` for `"$0"`. The definition may sit behind `declare -r`, `readonly` or
    `export`; `cd --`, `dirname --`, `1>` for `>` and a space after `&>` or `>` are the same
    definition. The run-when-executed guard `if [[ "${BASH_SOURCE[0]}" == "$0" ]]`
    (also `!=`, `"${0}"` and `; then`), alone on its line, names no directory and is accepted.

  It refuses rather than guesses on:
  - **Pins.** Inconsistent pins. Any `go-kure/.github` reference in a `uses:` or `repository:`
    context that yields no 40-hex pin: `@main`, a flow-mapping checkout, a checkout with a branch,
    another expression or no `ref:`, and similar. Only two are exempt: a `ref:` that is exactly
    `${{ steps.<id>.outputs.<name> }}`, and a job-level reusable-workflow call at a non-SHA ref.
    A reusable-workflow call pinned to a SHA, or one inside a step, is refused. A
    `go-kure/.github` checkout step is also refused when it has a `ref:` other than a key at the
    column of the `with:` mapping's first child (under `env:` or another key, in a block scalar
    body, at the step's own level), a line that is no key the scan parses (`a b:`, `a/b:`, the
    rest of a multi-line value), a value that does not end on its line (an unterminated or
    escaped quote), more than one `ref:` in any letter case, or more than one `with:`. A
    `repository: go-kure/.github` anywhere else in a step (under `env:`, deeper under `with:`, at
    the step's own level) is refused too: it used to mark the step as that checkout, so another
    repository's `ref:` in the same step was read as a pin.
  - **Workflow YAML a line scan cannot read**, whatever it names. A `uses:` or `repository:`
    value that is not whole on its own line: empty, continued on the next line, a block scalar,
    an alias, anchor, tag or flow collection, or a quoted value with an escape (`\` in double
    quotes, `''` in single quotes) or no closing quote. A `repository:` given as an expression
    (`${{ github.repository_owner }}/.github`). A `uses` or `repository` key that does not start
    its line (a flow mapping, a tagged or anchored key) or is not in lower case. A quoted key
    with an escape sequence, and a `? ` complex key. A `uses:` expression is refused only when it
    names `go-kure/.github`: GitHub does not evaluate expressions in `uses:`.
  - **Actions.** An action that is not a composite action (JavaScript or Docker), a `using` in a
    flow mapping or not in lower case included. A nested `uses:`, quoted, in a flow mapping or in
    any letter case, or more than one `run:` step (flow-mapping, quoted and `RUN:` steps counted; the lines of a
    `run: |` body are text, not keys).
    A `github.action_path` expression. Any `GITHUB_ACTION_PATH` mention that is not one whole
    `$GITHUB_ACTION_PATH/<path>` or `${GITHUB_ACTION_PATH}/<path>` word (reassigned, cut down with
    `${GITHUB_ACTION_PATH%/*}`, a bare trailing `/`, or a suffix after the path), and the
    runner's `_actions` directory by path. In an action that mentions `GITHUB_ACTION_PATH`:
    `dirname`, `realpath`, `readlink`, a parameter trim (`${name%...}`, `${name#...}`,
    `${name/...}`, `${name:offset}`), and a `$GITHUB_ACTION_PATH/<path>` word other than as the
    command run (assigned, or passed as an argument). A quoted key with an escape sequence, or a `? `
    complex key. A non-`.sh` or unaccounted-for script reference. Whether the runner reads
    `USES:`, `Using:` or `RUN:` as its key is not established here, so each is taken as that key.
  - **Paths.** A path that climbs above the repository root, or that has a `.`, `..` or empty
    (`//`) segment where it cannot be normalised.
  - **Scripts.** Any line using `$SCRIPT_DIR` in another shape, which includes `if !`, a wrapper
    command, `$( )`, a pipe and a non-`.sh` sibling. Any `SCRIPT_DIR=` assignment other than the
    trusted definitions. The word `SCRIPT_DIR` in any other form (`SCRIPT_DIR+=`,
    `SCRIPT_DIR[0]=`, `read SCRIPT_DIR`, `for SCRIPT_DIR in`, `n=SCRIPT_DIR`). Name indirection:
    `${!name}` (the array-keys form `${!name[@]}` is allowed), a `declare -n`, `local -n` or
    `typeset -n` nameref, `eval`, and a `declare`, `typeset`, `local`, `export` or `readonly`
    whose variable name holds a `$` or a backtick (`declare -g "$n+=/lib"`). Any other way of
    computing the script's own directory (`dirname "$0"`, `${0%/*}`, `BASH_SOURCE`, `BASH_ARGV`,
    a positional slice `${@:...}` or `${*:...}`, and `$_` or `${_}`, which holds the script's
    path right after an exempted `$0` message). Any `$0` outside a message to stderr (`echo "usage: $0 ..." >&2` or `1>&2`), a
    `sed -n '<lines>p' "$0"` read of the script itself or an awk record (`f($0`, `, $0`, ` = $0`,
    `$0 ~`, `$0 !~`): `x=$0`, `a=($0)`, `printf -v`, `read <<<"$0"`, a function argument or a
    message to another fd would carry the directory under another name. A line naming
    `$GITHUB_ACTION_PATH` or the runner's
    `_actions` directory. Any other `source` or script invocation at a command position, and a
    sibling that cannot be fetched.
  - **`$SCRIPT_DIR` that is not the script's own directory.** A file sourced from a script in
    another directory that names `SCRIPT_DIR` at all, since it shares its caller's. A script run
    as its own process that uses `$SCRIPT_DIR` before a trusted definition, since it reads an
    inherited one. A relative definition, `$(dirname "$0")`, in any walked script while any walked
    script changes the working directory (`cd`, `pushd` or `popd` outside a `$(cd` or `(cd`
    subshell).
  - **The compare.** A compare that is not `ahead`, and the pagination cap.

  It does not see the following. Its threat model is a trusted organisation's own files: it
  catches shapes written by accident that would hide consumed code, not a determined adversary,
  and a shape built to evade a line scan can still pass.
  - A sibling reached without naming `$SCRIPT_DIR`, `$0` or the checkout: a hard-coded absolute
    path, a name found on `PATH`, or a path assembled at run time (`/proc/self`, a variable filled
    from a file).
  - Job-level reusable-workflow calls at a non-SHA ref
    (`go-kure/.github/.github/workflows/<file>.yml@main` here). They run at their own ref, so no
    pin bump changes them.
  - The order in which a script runs: a trusted `SCRIPT_DIR` definition inside a function or a
    branch that never runs still counts as defining it for the lines below.
  - A `,$0` outside awk, which is taken for an awk record (`for p in {x,$0}`).
  - A `$0` message to stderr that is read back: the stderr exemption assumes stderr is not
    redirected into a file or a capture (`exec 2>f`, `$(f 2>&1)`) that the script then reads.
  - The script's path taken from the call stack with `caller`, which names no `$0`.
  - An assignment through a name built at run time other than by a declaration builtin:
    `printf -v "$n"`, `read "$n"`, `mapfile "$n"`. A consumed script assigns through
    `printf -v "$destination"`, so refusing it would abort real runs.
  - A declaration builtin not written as a plain word at a command position:
    `\declare -g "$n+=/lib"`, `d=declare; $d -g …`, or one continued onto the next line with `\`.
  - In an action's `run:` step, the action path carried past the command and cut to its directory
    by a tool other than `dirname`, `realpath`, `readlink` or a trim (`sed`, `awk`, `cut`, a
    Python one-liner). It can be carried by `$_` after the command, an array assignment
    `x=("$GITHUB_ACTION_PATH/…")`, or an argument on a `\` continuation line.
  - The action path read through a name built at run time
    (`n=GITHUB_ACTION; n+=_PATH; "${!n}/…"`).

  It also aborts, harmlessly but falsely, on:
  - another directory derived from the script's own, such as
    `ROOT=$(cd "$(dirname "$0")/.." && pwd)`. Following it would mean tracking an arbitrary
    variable through every script that sources or inherits it, and real scripts reuse such names
    for argument-derived paths;
  - `uses:` or `repository:` text anywhere in a single-line workflow value
    (`run: grep -n "repository:" ci.yml`, `with: { repository: foo/bar }`), and `ref:` text
    anywhere in a `go-kure/.github` checkout step, and a ` #` inside a quoted value there (read
    as a comment, which leaves the quote open);
  - a second `repository: go-kure/.github` outside the `with:` mapping of a checkout that already
    names it there, such as under `env:`;
  - `${!prefix@}`, and `export SCRIPT_DIR` or `readonly SCRIPT_DIR` on a line of its own;
  - `dirname`, `realpath`, `readlink` or a parameter trim anywhere in an action that mentions
    `GITHUB_ACTION_PATH`, and a `$GITHUB_ACTION_PATH/<path>` command behind a wrapper (`env`,
    `timeout`), an environment assignment (`VAR=1 "$GITHUB_ACTION_PATH/…"`), a shell option
    (`bash --noprofile`), a `{ …; }` group or a `case` arm.
  The refusal paths, plus the no-change, inert, affected and acknowledged outcomes and the
  merge-queue mode, are pinned by hermetic cases in `scripts/test/cases/`
  (`pin-impact-lib.sh` stubs `curl` and builds a throwaway git repo; no network). A maintainer who
  has reviewed a real hit and judged it safe adds
  the `pin-impact-ack` label to merge anyway — same convention as `check-doc-gate`'s `docs-skip`
  label; there is no other override. **Rerun gotcha:** the `strip-ack` step only runs when the
  triggering event's action was `synchronize` or `reopened`; re-running a stale/failed run of one of
  *those* (`gh run rerun`, or the Actions UI) replays that same original action and silently strips
  a freshly-added `pin-impact-ack` again before the gate re-checks it, even though nothing was
  pushed. A rerun of an `opened`/`labeled`/`unlabeled`-triggered run is unaffected — `strip-ack`
  skips it either way. **Scoped to same-repo PRs:** `strip-ack`'s `if:` also requires the PR's head
  repo to equal this repo, so it never runs at all on a fork PR — but that's moot, since the gate
  separately forces `PIN_IMPACT_ACK=false` unconditionally on forks; a fork PR has no acknowledgment
  path regardless of labels. Add (or re-add) the label rather than rerunning, on a same-repo PR; full
  writeup in `go-kure/.github`'s `docs/standards.md` § "Pin-impact-ack"

  **In the merge queue** the job runs `check-pin-impact.sh --consistency` instead. It checks only
  that every `go-kure/.github` reference in the merged tree pins the same commit. It reads no base,
  fetches nothing and has no label override. Two PRs that each passed on their own can combine
  into mixed pins: one adds a reference at the old pin while the other bumps the rest. On `main`,
  that tree is every later PR's base, and the gate refuses an inconsistent base before it reads the
  label, so it would refuse the repair PR too (go-kure/kure#951). The queue now ejects the PR
  instead; rebase it onto `main` and align its pins. The impact itself is still checked only on
  the PR (go-kure/kure#730).

### Draft PRs

CI and PR Review run on draft PRs the same as on ready ones; see
[Shared Workflows](/contributing/shared-workflows/).

`edited` is deliberately not in the type list: it fires on every title and body edit, which
would run the full suite for text-only changes. A PR retargeted to another base also sends
`edited`, but needs no run of its own: with no `branches:` filter the suite already ran on the head
commit (see [Triggers](#triggers)).

---

## Releasing

**Release** (`.github/workflows/release.yml`) is the one manual release workflow: pick the branch,
what to do, and whether it is a dry run. It calls `go-kure/.github`'s shared `release.yml`, whose
script makes the release commits, the tag and, when `main` moves to a new line, the release branch.
**Release / Publish** (`.github/workflows/release-publish.yml`) then runs by itself on the tag.
**Release / State** (`.github/workflows/release-state.yml`) is dispatched with a tag and reports
what that tag's publish run did; it calls the shared `release-state.yml` and sets the
`Release state <tag>` run title the guide relies on. How to release, what each option does, release branches and the recovery procedure for a failed
publish are on the [Releasing](/contributing/releasing/) page, which is the same text in every
go-kure repository: `docs/releasing.md` is vendored from `go-kure/.github` and CI's
`forbidden-terms` job byte-compares it at the pinned revision.

What is specific to kure:

- **What Publish produces:** a GitHub release object and nothing else. kure is a library, so
  `.goreleaser.yml` skips builds, disables checksums and declares no SBOM or signing stanza: **a
  complete release carries zero assets**; the tag is the artifact.
- **`guard-tag-ref`:** the wrapper's own job refuses a `workflow_dispatch` on anything but a `v*`
  tag before the privileged publisher starts. The check that a tag has no release yet runs inside
  the shared publisher.

---

## Merge Queue

kure merges through the merge queue. How it works, its settings and the required checks are on
[Shared Workflows](/contributing/shared-workflows/).

---

## PR Review Workflow

**File:** `.github/workflows/pr-review.yml`, calling `go-kure/.github`'s `pr-review.yml@main`.
What it does, its configuration and the incident switch are on
[Shared Workflows](/contributing/shared-workflows/). kure's `pr_review_context`:

> Upstream Go library for programmatically building Kubernetes resources. Uses typed builders
> instead of templates.

---

## Deploy Docs Workflow

**File:** `.github/workflows/deploy-docs.yml`
**Name:** `Deploy Docs`

### Triggers

- **Push to main** (paths: `site/**`, `docs/**`, `pkg/**/*.md`, `examples/**/*.md`, `README.md`, `CHANGELOG.md`, `DEVELOPMENT.md`)
- **Manual dispatch** with inputs: `version_slot`, `version_label`, `set_latest`

### How It Works

1. Runs `scripts/gen-versions-toml.sh` to generate a versioned Hugo config overlay
2. Builds the Hugo site with `--config hugo.toml,versions.toml`
3. Deploys the built site to `kure/<slot>/` in `go-kure.github.io` through the shared `deploy-docs-push` action; see [Shared Workflows](/contributing/shared-workflows/)

### Trigger Matrix

| Event | What Deploys | Path | BaseURL |
|-------|-------------|------|---------|
| Push to `main` (docs paths) | Dev docs | `/dev/` | `www.gokure.dev/dev/` |
| `workflow_dispatch` | Versioned | `/vX.Y/` | `www.gokure.dev/vX.Y/` |
| `workflow_dispatch` + `set_latest=true` | Versioned + root (root only if no stable tag is higher than the label; the label must then be a tag at the dispatched commit) | `/vX.Y/` + `/` | Both |

### Concurrency and Preservation

Per-slot concurrency group `deploy-docs-<slot>`. Root re-check, retry and what a deploy keeps are
on [Shared Workflows](/contributing/shared-workflows/).

---

## Manage Docs Workflow

**File:** `.github/workflows/manage-docs.yml`
**Name:** `Manage Docs`

### Triggers

- **Manual dispatch only** with inputs: `action`, `version_slot`

`version_slot` must be `dev` or `vX.Y`. A `validate` job checks it and both actions wait for that job, so any other value fails the run before anything is changed and before `remove-version` joins Deploy Docs' per-slot concurrency group. Every job reads it through `env:`, never interpolated into a script.

### Actions

| Action | Description | Implementation |
|--------|-------------|----------------|
| `remove-version` | Delete a version's docs | Removes `kure/<slot>/` from `go-kure.github.io` through the shared `deploy-docs-push` action in remove mode, which works on the pages branch's current tip and retries a push rejected because another slot's deploy landed first; fails if that directory does not exist there. Runs in Deploy Docs' `deploy-docs-<slot>` concurrency group, so it never overlaps a deploy of that slot. A deploy of the slot that starts later writes it again, and GitHub keeps only the newest waiting run in a group, so a removal and a deploy of the same slot that both wait cancel the older one |
| `rebuild-version` | Re-trigger a docs build | Dispatches `deploy-docs.yml` with `set_latest=false`: `dev` from `main`; `vX.Y` from the line's highest stable `vX.Y.Z` tag, with that tag as the label, as Release / Publish deploys it. Fails if the line has no stable tag |

There is no action that points the root `/` at a chosen version: the root is meant to hold the highest stable tag's docs, and the deploy step writes it only for a stable label that no existing stable tag exceeds and that is a tag at the commit the deploy checked out. A ref pinning an older step skips the tag check, and a ref older than the step checks nothing; see the Deploy Docs concurrency notes above. To put the root back on the highest stable tag, follow the recovery on the [Releasing](/contributing/releasing/) page, which dispatches `deploy-docs.yml` with `--ref` set to that tag.

### Common Scenarios

```bash
# Remove a yanked version:
#   Actions > "Manage Docs" > action=remove-version > version_slot=v0.2

# Rebuild after theme or script changes:
#   Actions > "Manage Docs" > action=rebuild-version > version_slot=dev
```

---

## Versioned Documentation

The docs site supports multiple documentation versions at different URL paths.

### URL Structure

| Path | Content | Updated By |
|------|---------|------------|
| `/` | Latest stable release | Release / Publish of the highest stable tag (`set_latest=true`) |
| `/vX.Y/` | Specific stable version | Release / Publish of the line's highest stable tag, or manual dispatch |
| `/dev/` | Development (from `main`) | Every push to `main` that touches docs |

### Version Switcher

The [Relearn theme](https://mcshelby.github.io/hugo-theme-relearn/) provides a native version dropdown in the sidebar. It is configured via `params.versions` entries in `versions.toml`, which `gen-versions-toml.sh` generates from git tags.

### How `gen-versions-toml.sh` Works

```bash
# Generate config overlay for a dev build:
./scripts/gen-versions-toml.sh --version dev

# Generate for a stable release:
./scripts/gen-versions-toml.sh --version v0.1.0 --latest v0.1.0
```

The script:
1. Reads all stable tags (`vX.Y.Z` without pre-release suffix) from git
2. Deduplicates to minor level (keeps highest patch per `vX.Y`)
3. Generates `site/versions.toml` with `[params]` section and `[[params.versions]]` entries
4. Marks the latest version with `isLatest = true` and root `baseURL`
5. Always includes a "Development" entry pointing to `/dev/`

### WIP Banner

The development version shows a warning banner linking to the latest stable version (if one exists). Stable versions show no banner.

### Links Between Pages

Each slot is built separately, and only `/dev/` follows `main`. The root `/` is rebuilt only by a
`set_latest=true` deploy, so it lacks every page added since that release. Link between site pages with a
version-relative path such as `/api-reference/kubernetes-builders/`; Hugo resolves it inside the
version being built. Text that is also read outside the site (`CHANGELOG.md` on GitHub, and the
`cliff.toml` header that writes it) links to the dev slot, the site's base URL followed by
`dev/`. Any other absolute or scheme-relative (`//host/...`) link to the site, including a
dev-slot link from any other page, fails the `docs-build` job
(`scripts/check-site-self-links.sh`).

---

## Test Jobs in CI

There is **one** Go test job, and it runs `go test` inline rather than through a `make` target.

| Job | Command | Uses Makefile? |
|-----|---------|----------------|
| `test` | `go test -v -race -coverprofile=coverage/coverage.out -covermode=atomic -timeout 15m ./...`, tee'd to a `test-log` artifact (`ci.yml:534`) | `make deps` only |
| `coverage-check` | no tests; downloads the `coverage` artifact and thresholds it with `go tool cover -func` | - |

That single command produces the race check and the coverage profile together, so there is no
separate race or coverage test run. **Nothing under `.github/workflows/` invokes `make test`,
`make test-race`, `make test-coverage`, `make vuln`, `make precommit` or `make ci`** — the `make`
targets CI does use are `deps`, `fmt`, `tidy`, `lint`, `vet`, `outdated`, `check-go-version`,
`check-tool-versions` and `check-govulncheck-docs`.

## Test Targets in Makefile

**No `make` target in this table is invoked by CI.** The column below states that explicitly,
because a ✅ there would claim pipeline coverage these targets do not have: a green pipeline says
nothing about whether any of them passes.

| Target | Command | Invoked by CI? | In precommit? |
|--------|---------|----------------|---------------|
| `test` | `go test -timeout 15m ./...` | - (CI runs its own inline `go test`) | ✅ |
| `test-race` | `go test -race -timeout 15m ./...` | - (the inline command already carries `-race`) | - |
| `test-coverage` | `go test -coverprofile=... ./...` | - (the inline command already writes the profile) | - |
| `test-integration` | `go test -tags=integration -timeout 5m ./...` | - | - |
| `vuln` | `govulncheck ./...` | - (CI runs the canonical `govulncheck-gate` action instead) | - |
| `versions-test` | `bash scripts/test/run-tests.sh` | - (CI runs the script directly, not via `make`) | ✅ |

## CI vs Pre-commit

Both are local aggregate targets. **Neither is what CI runs** — the pipeline calls individual
targets and its own inline commands, so `make ci` passing locally is not the pipeline passing, and
the pipeline passing is not `make ci` passing.

| Target | Tasks (`Makefile:349`, `:352`) | Use Case |
|--------|-------|----------|
| `precommit` | fmt, tidy, lint, test, check-tool-versions, check-govulncheck-docs, versions-test, check-builders | Local pre-commit gate. Dominated by `test`: `pkg/kubernetes/internal/gen` alone runs ~1 minute |
| `ci` | deps, fmt, tidy, lint, vet, test, test-race, test-coverage, test-integration, vuln, check-tool-versions, check-govulncheck-docs, versions-test, check-builders | Local superset — runs the race, coverage and integration passes `precommit` skips |

---

## Configuration Standards

### Go Version

All jobs use `go-version-file: go.mod` — the `go` directive in `go.mod` is the single
source of truth (kept in sync with `mise.toml` via `make check-go-version`).

### yq and lychee Versions

Both are read from `mise.toml` at run time by a dedicated step, the same shape as the Hugo
version read in `deploy-docs.yml`:

```yaml
- name: Read yq version from mise.toml
  id: yq-version
  run: |
    YQ_VER=$(grep '^yq = ' mise.toml | sed 's/yq = "\(.*\)"/\1/')
    if [ -z "$YQ_VER" ]; then
      echo "::error::Failed to parse yq version from mise.toml"
      exit 1
    fi
    echo "version=$YQ_VER" >> $GITHUB_OUTPUT
```

`${{ steps.yq-version.outputs.version }}` (or `steps.lychee-version...` for lychee) feeds both the
cache key and the download URL everywhere the version used to be hardcoded — three yq sites plus
one lychee site in `ci.yml`, one yq site in `deploy-docs.yml`. Bumping `mise.toml`'s `yq`/`lychee`
pin is sufficient on its own; no other file needs a manual update, and no tool-version-parity
checker needs to know about either (there is nothing left in `ci.yml`/`deploy-docs.yml` for one to
compare against). `lychee` previously had no `mise.toml` entry at all — its CI-pinned version was
the only copy anywhere; it now has one, matching the parity `yq` already had before this section
was written.

### Caching

CI jobs use explicit `actions/cache` steps with `cache: false` on `setup-go` to control
cache keys precisely. Two Go caches are maintained.

**Module cache** — dependency-only, split restore + save per Go job:

```yaml
- name: Restore Go modules cache
  id: gomod
  uses: actions/cache/restore@v6
  with:
    path: ~/go/pkg/mod
    key: ${{ runner.os }}-gomod-${{ hashFiles('**/go.sum') }}
    restore-keys: |
      ${{ runner.os }}-gomod-
# ... end of job ...
- name: Save Go modules cache
  if: success() && steps.gomod.outputs.cache-hit != 'true' && github.ref == 'refs/heads/main'
  uses: actions/cache/save@v6
  with:
    path: ~/go/pkg/mod
    key: ${{ steps.gomod.outputs.cache-primary-key }}
```

**Go build cache** (`~/.cache/go-build`) — split `actions/cache/restore` + `actions/cache/save`
so the log shows exact vs fallback restore (`cache-matched-key`). The key is **source-aware**
(was `go.sum`-only, which froze the cache at an old snapshot and never refreshed as source
changed) and **split by job purpose** so `validate` (non-race) and `test` (race+coverage) never
overwrite each other's entry:

```yaml
- name: Restore Go build cache
  id: gocache
  uses: actions/cache/restore@v6
  with:
    path: ~/.cache/go-build
    key: ${{ runner.os }}-${{ runner.arch }}-go-<GOVER>-gocache-<purpose>-deps-<go.sum hash>-src-<source hash>
    restore-keys: |
      ${{ runner.os }}-${{ runner.arch }}-go-<GOVER>-gocache-<purpose>-deps-<go.sum hash>-src-
      ${{ runner.os }}-${{ runner.arch }}-go-<GOVER>-gocache-<purpose>-
# ... compile / test ...
- name: Save Go build cache
  if: success() && steps.gocache.outputs.cache-hit != 'true' && github.ref == 'refs/heads/main'
  uses: actions/cache/save@v6
  with:
    path: ~/.cache/go-build
    key: ${{ steps.gocache.outputs.cache-primary-key }}
```

Purpose prefixes: `gocache-validate-`, `gocache-test-race-cover-`, `gocache-security-`. `<GOVER>`
comes from a `Read Go version from go.mod` step. The source hash covers `**/*.go`, `go.mod`,
`go.sum`, `Makefile`, `**/testdata/**`. Save runs only on a non-exact (fallback/miss) restore,
only when the run succeeded (so a broken build never publishes a cache), and only on `main`.

**Cross-ref scoping.** GitHub caches are ref-scoped: a `merge_group` (queue) run cannot restore a
`pull_request` run's cache — only the default branch (`main`) is shared. So these caches are warmed
by push-to-main and restored by PR + queue via restore-key fallback. This lowers both runs'
absolute cost but does not deduplicate the PR↔queue build (inherent to the merge queue). Measured
in launcher: warmed cycles cut `test` ~50%, `build`/`lint` ~30%.

**Saves are default-branch only.** Nothing but the PR itself can read a PR-scoped entry, and the
`gh-readonly-queue/*` branch a queue run saves to is deleted right after the run, so every Go
cache save is gated on `github.ref == 'refs/heads/main'`. Saving from those refs only fills the
size-capped cache server, whose LRU eviction then pushes out the `main` entries runs restore. New
cache steps follow the same rule: split restore/save with a `main`-gated save, never the combined
`actions/cache` (which saves on a miss from any ref). The small tool-binary and Hugo-module caches
are the exception: they are small and rarely re-keyed, so the combined form writes little.

Tool binaries are also cached to avoid reinstalling on every run:
- `goimports` — keyed by `go.sum` hash (tied to `golang.org/x/tools` version)
- `yq`, `lychee` — keyed by the version a "Read `<tool>` version from mise.toml" step reads at run
  time (see [yq and lychee Versions](#yq-and-lychee-versions) below), never a hardcoded literal
- `govulncheck` — keyed by pinned version (`v1.8.0`)

Cache and artifact traffic is routed through an in-cluster falcondev cache server backed by
Garage S3. Two layers work together:

1. **Binary patch** (`containers/actions-runner/Dockerfile` in opsmaster): `Runner.Worker.dll`
   is patched to read `CUSTOM_ACTIONS_RESULTS_URL` instead of `ACTIONS_RESULTS_URL` for its own
   internal connection. `CUSTOM_ACTIONS_RESULTS_URL` is set as a pod env var in the runner's
   `HelmRelease`. This ensures the Worker process itself connects through the cache server.

2. **Workflow env** (`ACTIONS_RESULTS_URL`): The binary patch replaces **all** UTF-16LE
   occurrences of `ACTIONS_RESULTS_URL` in the DLL — including the name the Worker injects into
   step process environments (renamed to `ACTIONS_RESULTS_ORL` as a side effect). Setting
   `ACTIONS_RESULTS_URL` in the workflow `env:` block overrides this so step processes
   (`upload-artifact`, `download-artifact`, `actions/cache` v2) see the correct URL.

`ACTIONS_CACHE_URL` / `ACTIONS_CACHE_SERVICE_V2` are not needed — cache actions use the v2
Results API path through `ACTIONS_RESULTS_URL`.

### docs-build Caching

The `docs-build` job uses two separate caches:
- `gomod` — Go module cache
- `hugo` — Hugo module cache (`$HUGO_CACHEDIR` only, **not** `~/go/pkg/mod`)

### Path Filters

The `changes` job uses `dorny/paths-filter` to skip jobs when unrelated files change:

- `go` output — gates `lint`, `test` and `Security`. It is a **deny-list** (go-kure/kure#800): true
  unless every changed file is documentation, meaning under `site/**` or `docs/**`, or a Markdown
  file anywhere (`**/*.md`). Every other path, including one nobody thought to classify, runs the Go
  jobs. Docs files that `validate` reads are added back: `docs/compatibility.md`
  (`sync-versions.sh check`), this file (the golangci-lint and govulncheck version-parity
  checks), and `docs/api-tables.md` and `docs/api-tables.json` (written and compared by the
  builder generator, `gen-builders.sh check`). A new docs file that a Go-gated job reads must be
  added to `go_docs_inputs`. This replaced an allowlist whose silent failure mode was the problem: a path missing
  from it skipped `lint` and `test`, and a skipped required check still satisfies the ruleset, so
  the PR read green unexercised. Eight separate additions to that list were each made after such a
  miss. The cost of the inversion is a Go run on PRs that touch only an unusual non-Go, non-docs
  file (a license or an editor config). Implemented as two `dorny/paths-filter` steps: `nondocs`
  with `predicate-quantifier: every` over `**` and the three negated docs paths, and
  `go_docs_inputs` for the added-back files.
- `docs:` filter — triggers the `docs-build` job (`doc-gate` runs on every PR regardless). Includes
  `site/**`, `docs/**`, `**.md` (every Markdown file, `.claude/CLAUDE.md` included), `pkg/**`,
  `examples/**` (the builder-reference check reads the comments of `examples/` Go files),
  `scripts/**`, `cliff.toml` (the site self-link check reads it, since its header template writes
  the links at the top of `CHANGELOG.md`), and `.github/workflows/ci.yml` (only ci.yml, since
  other workflows don't affect the docs build).

### Branch Patterns

- Release branches: `release/*` (note: not `releases/*`)
- Development branches: `main`, `develop`

---

## Estimated CI Time

| Scenario | Before (4 workflows) | After (2 workflows) |
|----------|---------------------|---------------------|
| PR opened | ~8 min (duplicate work) | ~4 min |
| Push to main | ~5 min | ~4 min |
| PR merge | ~5 min (full re-run) | ~0 min (same SHA, skipped) |

---

## Self-Hosted Runner Requirements

The runner is described on [Shared Workflows](/contributing/shared-workflows/). Its image lacks
`make`, so every job that calls `make` includes an explicit install step:
`sudo apt-get install -y --no-install-recommends make`

---

## Maintenance Notes

- **When adding/modifying workflows:** Update this document with changes
- **Version updates:** Run `make sync-go-version` to update Go version in all files
- **Version check:** Run `make check-go-version` to verify consistency
- **Action versions:** Keep GitHub Actions up to date (currently using v3-v6)
- **New jobs using `make`:** Add the `Install build tools` step (see above) if the job runs on `autops-kube-kure`

---

## See Also

- [Makefile](https://github.com/go-kure/kure/blob/main/Makefile) - Local development commands
- [mise.toml](https://github.com/go-kure/kure/blob/main/mise.toml) - Local tool version management
- [gen-versions-toml.sh](https://github.com/go-kure/kure/blob/main/scripts/gen-versions-toml.sh) - Versioned docs config generator
