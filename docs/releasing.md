# Releasing

How to release this repository: which button to press, which option to pick, and what to do
when a release fails. The same guide is published for every go-kure repository that releases
through the shared workflows, so it says "this repository" rather than naming one; its source is
[`standards/release-process.md`](https://github.com/go-kure/.github/blob/main/standards/release-process.md)
in `go-kure/.github`.

## Overview

A release takes two workflows. You run the first; the second runs by itself.

| Workflow | Started by | What it does |
|----------|------------|--------------|
| **Release** | you, from the Actions tab | Checks the branch, waits for CI, writes the `CHANGELOG.md` section, commits, tags, moves `VERSION` on, and pushes all of it in one atomic push |
| **Release / Publish** | the tag Release pushes | Tests, validates the tag, creates the GitHub release, starts the versioned docs deploy, refreshes the Go module proxy |

```text
you ──▶ Release (main or release/vX.Y)
          guard ─▶ wait for CI ─▶ commit + tag + VERSION bump ─▶ atomic push
                                                                   │ tag vX.Y.Z[-stage.N]
                                                                   ▼
                                                 Release / Publish (automatic)
                                                   test + validate ─▶ GitHub release
                                                                   ├─▶ versioned docs (stable only)
                                                                   └─▶ Go proxy refresh
```

Release waits for Publish and turns red if Publish fails, so one green Release run means the
version is out.

## Quick start

To release the version in `VERSION` on `main`:

1. Actions → **Release** → **Run workflow**, leave "Use workflow from" on `main`, leave the action
   on `release`, tick **Dry run**, and run it. The run's summary shows the tag it would make and the
   `VERSION` that would follow.
2. If that is what you want, run it again with **Dry run** unticked.
3. Wait for the run to go green: about a minute for the checks, up to 30 minutes for CI on the
   commit, then the time Release / Publish takes (up to 45 minutes).

That is the whole procedure for an ordinary prerelease. The rest of this guide is for moving
between stages, patch releases on a release branch, and failures.

## The Run-workflow panel

```text
Release                                     [Run workflow ▾]
┌─────────────────────────────────────────────────────────┐
│ Use workflow from   [ Branch: main ▾ ]                   │  ← main or release/vX.Y; others refused
│                                                          │
│ What to do                                               │
│ [ release ▾ ]                                            │  ← dropdown, exactly one option per run
│                                                          │
│ [ ] Dry run: preview only, change nothing                │  ← checkbox
│                                                          │
│                                 [ Run workflow ]         │
└─────────────────────────────────────────────────────────┘
```

- **Use workflow from** is the branch the release is made from: `main`, or a `release/vX.Y`
  branch for a patch release. A real run from any other branch, or from a tag, is refused within
  about a minute, before anything else runs.
- **What to do** picks one action. Rule of thumb: **an option starting with `release` makes a tag
  and a GitHub release; every other option only moves `VERSION`.**
- **Dry run** combines with every option. It changes nothing — no commit, no tag, no push — skips
  the CI and Publish waits, and prints the plan in the run's summary. It is allowed from any
  branch, so a preview works on a branch that has no CI run. It still refuses a tag or a next
  `VERSION` that already exists. A real run checks more: every option needs a clean tree and
  an unmoved branch tip, and the `release…` options also wait for green CI and refuse a local
  `replace` in `go.mod`. A clean preview does not guarantee the real run succeeds.

## Which option when

| Situation | On | Pick | Example: `VERSION` → tag / `VERSION` after |
|-----------|----|------|--------------------------------------------|
| Ship the next prerelease | `main` | `release` | `v0.2.0-beta.14` → `v0.2.0-beta.14` / `v0.2.0-beta.15` |
| The alphas are feature-complete; start betas | `main` | `release-as-beta` | `v0.2.0-alpha.7` → `v0.2.0-beta.0` / `v0.2.0-beta.1` |
| The betas have settled; cut a release candidate | `main` | `release-as-rc` | `v0.2.0-beta.14` → `v0.2.0-rc.0` / `v0.2.0-rc.1` |
| The release candidate is good; ship it | `main` | `release-as-stable` | `v0.2.0-rc.1` → `v0.2.0` / `v0.2.1-alpha.0` |
| Start work on the next minor version | `main` | `start-next-minor` | `v0.2.1-alpha.0` → no tag / `v0.3.0-alpha.0`, and `release/v0.2` is created |
| Start work on the next major version | `main` | `start-next-major` | `v0.2.1-alpha.0` → no tag / `v1.0.0-alpha.0`, and `release/v0.2` is created |
| Ship a fix for a line that has moved on | `release/vX.Y` | `release` | `v0.2.1` → `v0.2.1` / `v0.2.2` |
| A release refuses because the tag in `VERSION` already exists | `main` | `skip-prerelease-number` | `v0.2.0-beta.15` → no tag / `v0.2.0-beta.16` |

## What each option does

On `main`, `VERSION` is always a prerelease (`vX.Y.Z-alpha.N`, `-beta.N` or `-rc.N`). Starting
from `VERSION` `v0.2.0-beta.14`:

| What to do | Tag | `VERSION` after | Also |
|------------|-----|-----------------|------|
| `release` (default) | `v0.2.0-beta.14` | `v0.2.0-beta.15` | |
| `release-as-beta` | from alpha: `v0.2.0-beta.0`; already beta: as `release`; from rc: refused | `v0.2.0-beta.1` (from alpha) | |
| `release-as-rc` | `v0.2.0-rc.0` (already rc: as `release`) | `v0.2.0-rc.1` | |
| `release-as-stable` | `v0.2.0` | `v0.2.1-alpha.0` | |
| `start-next-minor` | none | `v0.3.0-alpha.0` | creates `release/v0.2` when `v0.2` has a stable tag and no branch yet |
| `start-next-major` | none | `v1.0.0-alpha.0` | creates `release/v0.2` on the same condition |
| `skip-prerelease-number` | none | `v0.2.0-beta.15` | recovery only |

Going back a stage is refused: `release-as-beta` on an rc says so and changes nothing. To restart
a line at an earlier stage, start the next minor version instead.

On `release/vX.Y`, `VERSION` is always the next stable patch, and **`release` is the only
option**: it tags `VERSION` as it stands (`v0.2.1`) and moves it to the next patch (`v0.2.2`).
Every other option is refused on a release branch, dry run included.

## Release branches

A release branch, `release/vX.Y`, carries patch releases for a line after `main` has moved on to
the next one.

- **Created for you.** `start-next-minor` and `start-next-major` create the branch for the line
  being left, when that line has a stable tag and no branch yet. It starts at the line's highest
  stable tag, plus one commit setting `VERSION` to the next patch, and is pushed in the same atomic
  push as `main`'s new `VERSION`: both land, or neither does. A line with no stable tag gets no
  branch (there is nothing to patch), and an existing branch is left as it is.
- **Protected.** The branch has `main`'s required checks but no merge queue, so a pull request must
  be up to date with the branch before it merges. Only the release bot pushes to it directly.
- **Fixes land on `main` first.** Merge the fix to `main` as usual, then open a pull request against
  `release/vX.Y` that cherry-picks it (`git cherry-pick -x <commit>`), and merge that. There is no
  automatic backporting.
- **Release from the branch** with **Use workflow from** set to `release/vX.Y` and the action left
  on `release`.
- **Each branch keeps its own `CHANGELOG.md`.** A patch's section lists only the commits since the
  previous tag on that branch. `main`'s `CHANGELOG.md` does not list patch releases made on release
  branches, and a patch tag never makes `main` list a section twice.
- **Latest follows the highest stable version.** A patch release becomes GitHub's Latest release,
  and moves the docs site's `latest`, only while it is the highest stable tag in the repository.
- **An older line without a branch** (one left before release branches existed) is set up by
  hand: create `release/vX.Y` from that line's highest stable tag, then open a pull request against
  it that sets `VERSION` to the next patch. Creating the branch needs write access only; the required
  checks apply to pull requests against it.

A worked timeline:

```text
main                                                release/v0.2
────                                                ────────────
VERSION v0.2.0-rc.1
Release: release-as-stable
  → tag v0.2.0, VERSION v0.2.1-alpha.0
(feature work)
Release: start-next-minor
  → VERSION v0.3.0-alpha.0 ───────── creates ──────▶ at v0.2.0, VERSION v0.2.1
Release: release
  → tag v0.3.0-alpha.0, VERSION v0.3.0-alpha.1
fix merged (PR to main) ─────────── cherry-pick PR ─▶ fix merged
                                                    Release (from release/v0.2): release
                                                      → tag v0.2.1, VERSION v0.2.2
                                                      Publish: v0.2.1 is Latest (highest stable),
                                                      docs slot v0.2 and docs latest
Release: release-as-stable (later)
  → tag v0.3.0: now the Latest release
                                                    Release: release
                                                      → tag v0.2.2: published, docs slot v0.2,
                                                        NOT Latest (v0.3.0 is higher)
```

## What happens after you click

**Release** (`release.yml`):

1. **Check branch and action** — about a minute. Refuses an unknown action, a tag instead of a
   branch, a branch other than `main` or `release/vX.Y` (unless it is a dry run), and any action
   other than `release` on a release branch.
2. **Wait for CI on this commit** — real runs of the `release…` options only. Waits up to 30
   minutes for CI to pass on the commit being released; a failed or unfinished CI run stops the
   release with nothing tagged, and so does a commit with no CI run at all after two minutes.
3. **Release** — runs the release script against this repository:
   - refuses if the branch moved on since the run started (start the release again);
   - refuses if the tag, or the `VERSION` that would follow it, already exists as a tag;
   - renders the new `CHANGELOG.md` section with git-cliff and commits `release: <tag>`;
   - creates the annotated tag, then commits the next `VERSION`;
   - checks the tree for downstream references, then pushes the branch and the tag (or `main` and
     the new release branch) in one atomic push.
4. **Wait for Release / Publish** — only when a tag was pushed. Up to 2 minutes for Publish to
   start and 45 minutes for it to finish; Release fails if Publish does.

Only one Release run per repository runs at a time. A second run waits for the first; if a third
is started meanwhile, GitHub cancels the waiting one.

**Release / Publish** (`release-publish.yml`, on the tag):

1. **Test** — the full test suite, with the race detector.
2. **Validate** — the tag format, a `CHANGELOG.md` section for the tag, whether this is the Latest
   release and the newest stable tag of its own line, and, on a tag push, that the tag is greater
   than every other tag of its own line (`vX.Y.*`), so a patch on a release branch passes beside a
   newer line's prereleases.
3. **GoReleaser** — refuses if the tag already has a GitHub release, then renders the release
   notes (the commits since the previous tag on the same branch) and creates the release. What it
   attaches is set by this repository's `.goreleaser.yml`; a library may ship none.
4. **Start the versioned docs deploy** — stable tags only: the `vX.Y` slot of the docs site, and
   the site's `latest` only when this is the Latest release. A tag that has a newer stable tag of
   its own line (an older patch published again) deploys nothing, so the slot keeps the newer
   patch's docs. Publish does not wait for that run; check it in the Actions tab.
5. **Refresh the Go module proxy** — requests the new version from `proxy.golang.org`.

## When a release fails

**Never move or delete a tag, and never delete a GitHub release.** The Go module proxy records the
commit a tag points at the first time anyone fetches it and never lets it change, and a deleted
release cannot be told apart from one that never existed. Every recovery below works with the tag
where it is.

**Never run a Publish you start by hand (a re-run or a dispatch) and a Release at the same time, in
either order.** Before starting either, check in the Actions tab that no Release and no Publish run
of this repository is still in progress. Release runs one at a time and waits for its own tag's
Publish, but a hand-started Publish runs beside it. Publish decides the docs `latest` and the
`vX.Y` slot from the tags it sees when it starts, so a newer tag pushed meanwhile can have its docs
replaced.

### Release failed before pushing

If Release failed in the checks, the CI wait, or the release job before its push, **nothing was
published**: the push is atomic, so no tag, no commit and no branch reached the repository. Fix
what the error names and run Release again.

| The error says | Do this |
|----------------|---------|
| A release runs only from main or a release/vX.Y branch | Pick `main` or the release branch in "Use workflow from", or tick Dry run to preview |
| CI did not pass on the commit | Fix CI on the branch, then run Release again |
| The branch moved after this run started | Run Release again; it releases the new tip |
| Tag … already exists | That version is tagged already. If its Publish failed, recover it as below. Then move `VERSION` past it: `skip-prerelease-number` on `main`, a pull request setting the next patch on a release branch |
| The tree contains downstream references | Fix the source, or the `cliff.toml` postprocessor that let a name into `CHANGELOG.md` |
| The push was refused | Nothing was published; read the push error (branch protection, a moved branch) and run Release again |

### The tag is pushed and Publish failed

Release then turns red at "Wait for Release / Publish". First establish what Publish actually did,
then pick the recovery.

#### Determining what a release actually did

When a tag's publish run goes wrong, the first question is always the same: did it publish,
partly publish, or never publish — and what is the safe recovery? Answering that by reading
the run page is unreliable, because six separate facts have to be held at once and each one
is a route to a confidently wrong conclusion.
**[`scripts/release-state.sh`](https://github.com/go-kure/.github/blob/main/scripts/release-state.sh)
answers it instead**. It lives in `go-kure/.github` alongside the shared publish workflow, and is
run from a checkout of that repository:

```bash
scripts/release-state.sh go-kure/<repo> v0.2.0-beta.11
scripts/release-state.sh --state-only go-kure/<repo> v0.1.0-alpha.21
```

It prints the evidence it used, a recommended action, and exactly one of:

| State | Meaning |
|-------|---------|
| `published` | the publishing job concluded success in **some** attempt, and the release object exists |
| `partial` | the release exists and the publishing job **ran**, but never concluded success in any attempt — it failed or was cancelled, so the run skipped the jobs that follow publication; the recovery is by hand, not a re-run, and leaves the state `partial` (below) |
| `never-published` | no release object and no successful publishing job |
| `contradictory` | the run record and the release object disagree, in **either** direction: a release exists that the job never ran to produce, or the job succeeded and the release is gone |
| `no-run-found` | no workflow run for this tag at all **and no release object** |

**`published` deliberately keys on "some attempt", not "the most recent attempt".** A re-run that
fails in `test` skips the publishing job while the earlier attempt's release still stands, so
keying on the latest attempt would report a shipped release as unpublished and invite a re-publish.
That is the same conflation the whole script exists to prevent, so `partial` means *never
succeeded*, not *most recently failed*.

Both directions of `contradictory` print their own recommended action, because the operator's next
move differs: a release that nothing produced must not be deleted, while a success whose release
object has vanished must not be re-run. The state word stays the same so a caller's `case` needs
only the five branches.

**An empty run record is `no-run-found` only when there is no release to contradict.** Runs age
out, so a tag published long enough ago reaches "a release exists and the run record holds no run
at all" with nothing wrong — and that is the first direction of `contradictory`, reached by a
shorter route, not an absence of information. Reporting `no-run-found` there would send the
operator to check whether the tag was pushed, which is the wrong question for a release that
demonstrably exists, and would drop the do-not-delete warning.

Exit status is `0` when a state was determined and `1` when it was not. **A failed API call
yields no state** — it reports `undetermined` and exits `1`, because "never published" and
"the API did not answer" are different claims, and a recovery path that collapses them
re-publishes a release that may already exist.

**A publish that is still running also yields no state**, and reports `undetermined` for the same
reason: every state in the table is a statement about a *finished* publish. This one is called out
separately because it is the case where acting on a wrong answer does the most damage —
`never-published` recommends a re-run and `partial` recommends running the publication's follow-up
jobs by hand, and the one moment neither may happen is while the job is still going. The evidence
block names the run and attempt that is in flight, and the advice says to wait rather than to
re-run.

This outranks an earlier success, and that is the one place where "a success in some attempt wins"
does not apply. A success settles what the *past* attempts did; a job running now is about the
future, and GoReleaser re-uploads to the same release object — so an attempt in flight can still
turn a complete release into an incomplete one. The evidence block records the earlier success
explicitly, so the answer is "wait", never "the release is missing".

**A hole in the run record yields no state either, but only when nothing else showed a success.**
A `404` on the *release object* is a fact about publishing; a `404` on a run, an attempt or an
attempt's job list is not — the record was deleted or aged out, and what it contained is exactly
what the negative states claim was never there. So a gap plus no observed success reports
`undetermined`; a gap alongside a success observed elsewhere still reports `published`, because a
missing record cannot un-see a success that was read. The evidence block names each part that
could not be read.

Three things the script does that reading the run page by hand does not:

- **It queries every attempt, not the latest.** `gh run view --json jobs` reports only the
  most recent attempt, which is frequently not the attempt that published. A re-run that
  fails early shows `goreleaser: skipped` while an earlier attempt's release stands.
- **It never uses the asset count as an oracle.** How many assets a complete release carries
  is decided by that tag's own `.goreleaser.yml`; for a library repo the correct count is
  zero. The count is reported as evidence and is not an input to the verdict.
- **It distinguishes a carried-forward job row from one that ran.** A re-run copies
  non-rerun jobs forward unchanged, so an attempt's job list mixes attempts, and the
  conclusions are identical either way — only the timestamps separate them.

Every one of those behaviours is pinned by a case in
[`scripts/test/release-state-test.sh`](https://github.com/go-kure/.github/blob/main/scripts/test/release-state-test.sh),
which stubs `gh` and needs no token and no network. A new fact about how GitHub reports
release runs belongs there as a failing test, not as a new paragraph in this guide.

#### Recovery when no release exists (`never-published`)

| Situation | Recovery |
|-----------|----------|
| A job concluded `failure` (`test`, `validate` or `goreleaser`), the cause was transient, and the shared workflow needs no change | `gh run rerun --failed <run-id> --repo go-kure/<repo>` |
| A job concluded `cancelled` or `timed_out` | `gh run rerun <run-id> --repo go-kure/<repo>` — a **full** re-run |
| The shared workflow needed a fix | `gh run rerun <run-id> --repo go-kure/<repo>` — a **full** re-run, not `--failed` |
| The run is over 30 days old, or a newer tag of the same line exists | `gh workflow run release-publish.yml --repo go-kure/<repo> --ref <tag>` |

Why the rows split this way:

- **`--failed` selects only jobs that concluded `failure`.** A job that was `cancelled` or
  `timed_out` is not selected, and neither is anything waiting on it, so `--failed` reports
  success having re-run nothing while the release stays absent.
- **Only a full re-run picks up a fix to the shared workflow.** Re-running all jobs resolves the
  called workflow from its reference (`@main`); re-running failed jobs or a single job uses the
  called workflow at the first attempt's commit.
- **Re-runs are available for 30 days** after the run started. After that, dispatch.
- **A `--failed` re-run can lose `validate`'s Latest decision.** If `goreleaser` then refuses with
  "validate did not decide whether … is the Latest release", use a full re-run.
- **A re-run of a tag-push run checks version progression again**, because it keeps the original
  `push` event; a dispatch skips that check, since the tag passed it when it was created. Once a
  newer tag of the same line exists, a full re-run fails `validate`, so dispatch instead. The
  dispatch `--ref` must be the tag itself.
- **The publisher refuses to publish over an existing release**, on every path — tag push,
  dispatch, full re-run, and `--failed` or single-job re-run — so a re-run is a recovery only while
  no release exists. An answer that is neither "exists" nor "not found" refuses as well, except on
  the first attempt of a tag push, where it warns and proceeds so an API error cannot block a first
  publication. The check sees published releases only: GoReleaser keeps a release a draft until its
  uploads finish, so a draft left by a failed upload is not refused, and a re-run creates a new
  release beside it. `release-state.sh` reads the same lookup, so that case reports
  `never-published`, provided the run record is fully readable and no attempt is still running
  (otherwise `undetermined`). Once the re-run has published, delete the stale draft by its release
  id or on the releases page, never by tag: `gh release delete <tag>` can resolve the tag to the
  published release.

`release-state.sh` gives the cautious form of this table: its advice for `never-published` is
always the full re-run, with a warning against `--failed`. On a stable tag it also lists every
stable release (`gh` lists 30 unless given `--limit`) and says to escalate before re-running when
a newer one exists ([go-kure/.github#239](https://github.com/go-kure/.github/issues/239)).

#### Recovery when the release exists and publishing succeeded

If `goreleaser` concluded `success` and only a later job failed (docs deploy or proxy refresh),
**do not publish again**:

- The later job concluded `failure` and the cause was transient:
  `gh run rerun --failed <run-id> --repo go-kure/<repo>`. This re-runs the failed job only; the
  release is not touched. If the docs deploy then refuses because validate did not decide the
  docs deploy, continue with the next bullet.
- The later job concluded `cancelled` or `timed_out`, or the shared workflow needs a fix: a full
  re-run would redo publication and is refused. Do the follow-up work directly:

  ```bash
  # Docs deploy. --ref is required: without it the docs of the default branch are deployed into
  # the version slot. Both listings below read local tags: run them in a checkout of
  # go-kure/<repo> right after `git fetch --tags`, or a missing newer tag makes them wrong.
  # set_latest=true only if <tag> is the highest stable tag, which this prints:
  #   git tag --list 'v*' | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -n 1
  # Skip the deploy if <tag> is not the highest stable tag of its own line, which this prints:
  #   git tag --list '<vX.Y>.*' | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -n 1
  gh workflow run deploy-docs.yml --repo go-kure/<repo> --ref <tag> \
    -f version_slot=<vX.Y> -f version_label=<tag> -f set_latest=<true|false>

  # Go proxy refresh: request the version so the proxy fetches it.
  curl -fsS https://proxy.golang.org/github.com/go-kure/<repo>/@v/<tag>.info
  ```

#### Recovery when the release exists but publishing never succeeded (`partial`)

GoReleaser publishes a release only once its uploads finish, so what failed came after
publication: the release object is not what is missing, the jobs that need a successful
publishing job are (the docs deploy and the proxy refresh), because the run skipped them. **Do not
re-run, in any form**: while the release exists every path into publication refuses. Instead:

1. Check the release's assets against the tag's own `.goreleaser.yml`. If anything is missing (a
   leftover draft someone published by hand, say), escalate.
2. Do the follow-up work by hand with the commands above. `release-state.sh` prints them for the
   tag, except that for a stable tag that is not the newest stable release it says to escalate
   instead of deploying the docs
   ([go-kure/.github#239](https://github.com/go-kure/.github/issues/239)).

The state stays `partial` afterwards: it describes the run record, which these steps do not
change, and they may already have been done. Repeating them is safe: a docs deploy rebuilds the
same slot from the tag, and the proxy refresh is a read.

#### Anything else

`contradictory` is **an escalation, not a self-service recovery**. Collect the `release-state.sh`
output and hand it to a maintainer. Do not delete the release object: whether it is this run's to
remove is exactly what cannot be established from the command line, and a release can be replaced
by hand once its provenance is settled.

#### Known limits

- **A broken docs deploy at the tag cannot be recovered by re-running it.** `--ref` selects both
  the workflow version and the content, so `--ref <tag>` re-runs a faulty `deploy-docs.yml` from
  the tag, and omitting `--ref` deploys the default branch's content into the version slot. The
  dispatch above covers transient failures only.
- **A Publish wrapper that is broken at the tag cannot be recovered by either path.** A dispatch
  takes the wrapper from the tag, and a full re-run re-resolves only the called shared workflow.
  Escalate.
- **If a `latest` pointer ends up on the wrong release anyway**, point both back at the highest
  stable tag. Wait for any docs deploy still running first: deploys of different slots do not wait
  for each other, and the one that pushes second can fail.

  ```bash
  gh workflow run deploy-docs.yml --repo go-kure/<repo> --ref <highest-stable-tag> \
    -f version_slot=<vX.Y> -f version_label=<highest-stable-tag> -f set_latest=true
  gh release edit <highest-stable-tag> --repo go-kure/<repo> --latest
  gh release view --repo go-kure/<repo> --json tagName --jq .tagName   # shows the Latest release
  ```

## Reference

- **`VERSION`** holds the next version to be released from that branch. On `main` it is always a
  prerelease; on `release/vX.Y` it is always the next stable patch. Release moves it on.
- **Versions** are `vX.Y.Z` (stable) and `vX.Y.Z-alpha.N`, `-beta.N`, `-rc.N` (prereleases), in
  the order alpha < beta < rc < stable. A line is the `vX.Y` part.
- **`CHANGELOG.md`** is written by git-cliff from Conventional Commit messages, configured by this
  repository's `cliff.toml`. A release renders only its own section and inserts it below the header
  (`git-cliff --unreleased --use-branch-tags --tag <tag> --prepend CHANGELOG.md`), so published
  sections are never rewritten. `--use-branch-tags` counts only tags on the current branch, which
  keeps a release branch's patches out of `main`'s changelog. The release notes are the same
  section, rendered by Publish with `git-cliff --latest --use-branch-tags --strip header`.
- **Identity.** Release commits and tags are pushed by the `kure-release-bot` GitHub App (secrets
  `KURE_BOT_APP_ID` and `KURE_BOT_APP_PRIVATE_KEY`), the one actor allowed to push past the
  protection of `main` and `release/*`. Automation reuses this identity rather than minting a new
  one.
- **Concurrency.** One Release run per repository at a time; a second waits, and a third started
  meanwhile replaces the waiting one. Publish runs one at a time per tag.
- **Scripts.** Release runs
  [`scripts/release/release.sh`](https://github.com/go-kure/.github/blob/main/scripts/release/release.sh)
  from `go-kure/.github`, at the commit of the shared workflow that runs it, so this repository
  carries no copy. Publish's progression and Latest decisions are
  [`scripts/release/publish-policy.sh`](https://github.com/go-kure/.github/blob/main/scripts/release/publish-policy.sh).
  Both are tested against real git repositories by
  [`scripts/test/release-test.sh`](https://github.com/go-kure/.github/blob/main/scripts/test/release-test.sh).
- **Workflows.** The shared
  [`release.yml`](https://github.com/go-kure/.github/blob/main/.github/workflows/release.yml) and
  [`release-publish.yml`](https://github.com/go-kure/.github/blob/main/.github/workflows/release-publish.yml)
  in `go-kure/.github`, called by this repository's own `release.yml` and `release-publish.yml`.
