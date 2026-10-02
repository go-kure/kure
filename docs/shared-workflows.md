# Shared Workflows

How the parts of this repository's GitHub automation that come from `go-kure/.github` behave:
the AI pull-request review, the `@claude` assistant, the merge queue, auto-rebase, the shared CI
checks and the docs deploy step. The same page is published for every go-kure repository that
uses them, so it says "the calling repository" rather than naming one; its source is
[`standards/github-workflows.md`](https://github.com/go-kure/.github/blob/main/standards/github-workflows.md)
in `go-kure/.github`. What the calling repository's own CI and Deploy Docs workflows run, job by
job, is on its GitHub Workflows page.

## Overview

`go-kure/.github` provides two kinds of shared building block, and they update differently.

| Kind | Referenced as | When a change reaches the calling repository |
|------|---------------|----------------------------------------------|
| Reusable workflow (`pr-review.yml`, `claude.yml`, `release.yml`, `release-publish.yml`, `release-state.yml`, `auto-rebase.yml`) | `go-kure/.github/.github/workflows/<name>@main` | On the next run, as soon as the change merges in `go-kure/.github` |
| Composite action (the CI checks and `deploy-docs-push`) | `go-kure/.github/.github/actions/<name>@<commit SHA>` | Only when the calling repository bumps the pinned SHA |

The calling repository's CI and Deploy Docs workflows are its own files, not calls to a reusable
workflow. They run the shared composite actions as steps, next to steps of their own. The
release workflows, `release-state.yml` included, are described in the Releasing guide.

Renovate bumps the composite-action pins as one `go-kure/.github` update.

## PR Review

An AI review of same-repository pull requests. Findings become review threads that must be
resolved before the pull request can merge. A diff too large for the GitHub API (HTTP 406) is
not reviewed: the run notes it in its job summary and passes.

### Triggers

The calling repository's `pr-review.yml` runs on `pull_request` (`opened`, `synchronize`,
`reopened`) and `merge_group`, with `contents: read` and `pull-requests: write`, and calls
[`pr-review.yml`](https://github.com/go-kure/.github/blob/main/.github/workflows/pr-review.yml)
with `secrets: inherit` and a `pr_review_context` string describing the project.

The job is skipped, and a skipped run counts as passing, for:

- pull requests from forks;
- `merge_group` runs on the queue's temporary ref;
- any run while `PR_REVIEW_THREADS_MODE` is `off`.

Draft pull requests are reviewed. Draft status blocks merge, not review.

### How it works

1. The job, named `AI Code Review` (check context `pr-review / AI Code Review`), runs on the
   self-hosted runner with a 20-minute timeout. A run already in progress for the same pull
   request is never cancelled; a newer run waits for it, and replaces any older run still waiting.
2. It fetches the pull request's diff and splits a large diff into chunks.
3. A first pass reviews each chunk. The prompt carries the project context, the calling
   repository's `AGENTS.md` and `.claude/CLAUDE.md`, and a standards document: by default the
   org standards from `go-kure/.github` (see "Configuration" below).
4. A second pass assesses the findings and drops false positives.
5. The remaining findings are reconciled with the threads already on the pull request.

The model is reached through an in-cluster, OpenAI-compatible proxy backed by Claude Max
credentials, so no API key secret is needed.

### Findings and threads

The default mode, `enforce`, turns each finding into a resolvable review thread:

- a new finding opens a thread;
- a thread the assessment calls a false positive gets a reply and is resolved;
- a thread whose issue is absent from two consecutive reviews, at two different head commits,
  gets a reply and is resolved;
- a thread with a reply from a person is never auto-resolved by either rule; a person resolves it;
- a thread the review resolved is reopened if its issue comes back;
- a thread a person resolved is never reopened.

New review threads are capped at 5 across the pull request. Threads already open, or about to be
reopened, count against the cap first and are never held back by it; new findings, most severe
first, get whatever places remain. So more than 5 threads can be open when earlier ones return.
New findings past the cap go into one overflow comment.

The check is required on `main` and on `release/*` branches, so a failed review run blocks the
merge. Separately, the rulesets require every review thread to be resolved before merge.

Threads are written with the `KURE_BOT_PAT` organization secret, as the `kure-bot` account.
Where that secret is not set, the job falls back to `github.token`, which can open threads but
cannot resolve them. A caller can supply its own identity instead: a `BOT_PAT` secret together
with the `bot-login` input naming the account that PAT posts as. `BOT_PAT` takes precedence over
both. The two come as a pair: a run that receives one without the other fails before the review
starts, so a repository that has a `BOT_PAT` secret reaching the call through `secrets: inherit`
must also pass `bot-login`.

### Configuration

The defaults are set in `pr-review.yml`. The mode can be changed with a variable, and the
standards file with the `standards-file` and `standards-source` inputs; changing any other value
means editing `pr-review.yml` in `go-kure/.github`.

| Setting | Default | Notes |
|---------|---------|-------|
| `PR_REVIEW_THREADS_MODE` | `enforce` | Overridable through the `PR_REVIEW_THREADS_MODE` variable |
| `PR_REVIEW_MAX_FINDINGS_TOTAL` | `5` | Cap on new threads, after open and reopened ones are counted |
| `PR_REVIEW_MAX_DIFF_CHARS` | `50000` | Soft size of one diff chunk: a hunk is never split, so a chunk can exceed it; a single hunk over 4 times this value is truncated and the review marked incomplete |
| `PR_REVIEW_MAX_TOKENS` | `1500` | Review pass |
| `PR_REVIEW_ASSESS_MAX_TOKENS` | `4096` | Assessment pass |
| `PR_REVIEW_AGENTS_FILE` | `AGENTS.md` | Read from the calling repository |
| `PR_REVIEW_STANDARDS_FILE` | `docs/standards.md` | Set from the `standards-file` input (an empty value disables it). Read from the checkout `standards-source` names |
| `PR_REVIEW_STANDARDS_SOURCE` | `action` | Set from the `standards-source` input. `action` reads the standards file from `go-kure/.github` at the commit `pr-review.yml` pins its `pr-review-threads` action to, so a standards change reaches reviews only when that pin moves; `caller` reads it from the calling repository's checkout. Any other value fails the review |

The model names in `pr-review.yml` are labels only; the proxy decides which model answers.

The three modes:

| Mode | Behaviour |
|------|-----------|
| `enforce` | Findings become resolvable review threads, as above |
| `advisory` | One plain comment per run with the findings table; no threads, and no comment when the diff is empty |
| `off` | The job is skipped |

An unrecognised mode is treated as `advisory`, never as `enforce`.

### When the review is broken

If the review fails on every pull request (proxy down, rate limits, a bug in the review script),
set the variable `PR_REVIEW_THREADS_MODE` to `off`. From the next run on, the job is skipped
before checkout and the skipped check counts as passing. A pull request whose check already
failed needs a new run (push, or re-run the check), and threads already open still have to be
resolved. Unset the variable, or set it back to `enforce`, to resume.

Inside the reusable workflow, the variable resolves against the calling repository, not
`go-kure/.github`. Set it as an organization variable to stop the review everywhere, or on the
affected repository alone. Setting it on `go-kure/.github` has no effect on other repositories.

The full design, failure handling and reconciliation rules are in
[`docs/pr-review-threads.md`](https://github.com/go-kure/.github/blob/main/docs/pr-review-threads.md).

## Claude

The calling repository's `claude.yml` calls
[`claude.yml`](https://github.com/go-kure/.github/blob/main/.github/workflows/claude.yml) in
`go-kure/.github` on new issue comments, pull-request review comments, submitted pull-request
reviews, and opened or assigned issues. The job runs only when the comment, review body, issue
body or issue title contains `@claude`.

It runs [`anthropics/claude-code-action`](https://github.com/anthropics/claude-code-action) on
the self-hosted runner, authenticated with the `CLAUDE_CODE_OAUTH_TOKEN` secret, with read
access to contents, pull requests, issues and actions, plus `id-token: write`, its one write
permission, which lets the job request an OIDC token from GitHub. A reusable workflow cannot
raise the permissions its caller grants, so the calling workflow must grant all five.

## Merge Queue

Pull requests to `main` land through GitHub's merge queue. The rules come from
[`governance/repository-settings-policy.yaml`](https://github.com/go-kure/.github/blob/main/governance/repository-settings-policy.yaml)
in `go-kure/.github` and are applied by its `settings.yml` workflow.

### How to merge

1. Wait for the required checks to pass on the pull request and resolve every review thread.
2. Click **Merge when ready**, or enable auto-merge, which adds the pull request to the queue
   once its checks pass.
3. The queue rebases the pull request onto `main`, runs CI on the result (`merge_group` event),
   and lands it if the checks pass.

The queue tests the real merged result, so the pull request does not need to be up to date with
`main` before it is queued.

### Queue settings

| Setting | Value |
|---------|-------|
| Merge method | Rebase (linear history) |
| Entries built and merged at once | 1 |
| Wait before merging | 0 minutes |
| Grouping | All checks must pass; a failing entry is removed |
| Check response timeout | 60 minutes |

### Required checks

| Check | Source |
|-------|--------|
| `lint` | The calling repository's CI |
| `test` | The calling repository's CI |
| `build` | The calling repository's CI |
| `pr-review / AI Code Review` | PR Review, above |

Up-to-date enforcement (`strict`) is off on `main`, since the queue tests the merged result.

### Release branches

`release/*` branches, created by the Release workflow, have no merge queue: GitHub does not allow
one on a wildcard branch rule. They require the same four checks with up-to-date enforcement on,
so a backport pull request must be rebased onto the branch before it merges. A maintainer can
still create such a branch by hand from a stable tag.

## Auto-Rebase

[`auto-rebase.yml`](https://github.com/go-kure/.github/blob/main/.github/workflows/auto-rebase.yml)
is opt-in: it runs only in a repository whose own workflow calls it on `push` to `main`, with
`secrets: inherit`. Each run rebases the open pull requests that target `main` onto it, drafts
included, and pushes with the `AUTO_REBASE_PAT` secret. It skips pull requests labelled
`dependencies`, those targeting any other branch (release or stacked branches), and those from a
fork that does not allow maintainer edits. A newer run cancels
one still in progress. The merge queue does not need it; it keeps open pull requests current for
review.

## Draft PRs

Open a pull request as a draft while it is still changing. CI and PR Review both run on drafts,
so feedback arrives before the pull request is marked ready. A draft cannot be merged or queued.

## Shared CI Checks

The calling repository's CI runs these composite actions from `go-kure/.github`. Each one runs a
script of the same name from `go-kure/.github` at the pinned commit.

| Action | What it checks |
|--------|----------------|
| `check-action-pins` | Every action reference, `go-kure` composite actions included, is a full 40-character commit SHA. Exempt: local `./` and `docker://` references, and job-level calls to `go-kure` reusable workflows, which stay on `@main` |
| `check-forbidden-terms` | No tracked file in scope references the downstream platform (the No Downstream References standard); always scans the whole tree, on every event |
| `govulncheck-gate` | A govulncheck JSON report has no reachable advisory outside the allowlist |
| `check-doc-sync` | `docs-map.yaml` matches the tree: every public package mapped, every mapped path present, mount targets unique, generated tables current |
| `check-doc-gate` | When a mapped package's own non-test `.go` files change, other than a trivial change to a generated file (`// Code generated … DO NOT EDIT.`) in which every changed line replaces exactly one line and either ends in `// doc-gate:trivial` in both versions with the same text before its first `=`, or is a table row whose only change is its `ModuleVersion` value, its mapped docs change in the same pull request. Separately, when a changed file matches a `review_mappings` entry's `change` glob, at least one of that entry's `docs` changes too; this covers non-Go files such as workflows and configuration |
| `check-links` | Every internal link in the built site points at an existing page (external links and `#fragment` anchors are not checked) |

### govulncheck gate

The gate reads a report written by `govulncheck -format json` and exits:

| Exit | Meaning |
|------|---------|
| 0 | No reachable advisory outside the allowlist |
| 1 | A reachable advisory outside the allowlist |
| 2 | The report could not be read; the gate fails closed |

The `allowlist` input is a space-separated list of OSV IDs accepted as known risk. Every entry
needs a written justification in the calling repository.

### Doc gate bypass

A maintainer can apply the `docs-skip` label to a pull request to bypass `check-doc-gate` for a
change that needs no doc update.

### Forbidden-terms guard and vendored copies

`check-forbidden-terms` must run on every CI event, with no path filter, so a pull request and
the merge queue see the same result. A term that is legitimate for an unrelated reason carries an
`allow-term:<word>` pragma on the same or an adjacent line. The full rule is
[No Downstream References](https://github.com/go-kure/.github/blob/main/docs/standards.md) in
`docs/standards.md`.

Three files from `go-kure/.github` are vendored into the calling repository: the guard script,
for local tooling, and the Releasing guide and this page, for its docs site.
The calling repository's `scripts/vendor-guard.sh` fetches each one at the commit its
`check-forbidden-terms` pin names, and its CI checks out `go-kure/.github` at that commit and
byte-compares each vendored copy. When Renovate bumps the pin, it runs
`scripts/vendor-guard.sh` on the same branch, so the copies move with the pin. Do not edit a
vendored copy; change it in `go-kure/.github` and let the next pin bump carry it.

## Docs Deploy

The calling repository's Deploy Docs workflow builds its site and checks out the pages
repository, `go-kure/go-kure.github.io`, with the `DEPLOY_TOKEN` secret, a token with write
access to that repository. The
[`deploy-docs-push`](https://github.com/go-kure/.github/tree/main/.github/actions/deploy-docs-push)
action then writes the build and pushes it.

### Slots and the site root

Each deploy writes one version slot under the repository's directory in the pages repository:
`dev` for `main`, or `v<major>.<minor>` for a release. The slot is replaced as a whole. A slot
name is one path segment, either `dev` or starting with `v`; anything else fails the step.

A release deploy asks for the site root only when Publish decided no higher stable tag exists
(`set_latest=true`). Prereleases do not count, so a patch on an older line asks for the root while
the newer line has only prereleases; otherwise it deploys its slot alone. When asked, the root is
written only if, at write time:

- the label is a stable `vX.Y.Z` version and, after the tags are fetched again from the calling
  repository, no stable tag is higher (the same rule Publish uses); otherwise the slot is deployed
  alone, the root is left untouched and the run logs a notice. An error in that check fails the
  step;
- the label is an existing tag at the commit being deployed; otherwise the step fails. To deploy
  an older tag's docs, dispatch Deploy Docs with that tag as the ref.

A root write replaces everything in the repository's directory except `dev` and the `v*` slots.

### Concurrent deploys

Deploys of the same slot run one at a time (concurrency group `deploy-docs-<slot>`): a deploy in
progress is never cancelled, a newer one waits for it, and a newer one replaces any older deploy
of that slot still waiting. Deploys of different slots can race. A push rejected because another
deploy landed first is written again on the new tip and retried, up to 5 attempts with 5 seconds
between them. Any other push failure fails the step at once.

Deploys to the same slot are serialized, not re-checked, so an older patch release deployed after
a newer one still replaces that slot's content.

### Removing a slot

With `remove: "true"` the action deletes one slot instead of deploying (the calling repository's
Manage Docs workflow, where it has one). It takes only `target-path`, `site-subdir` and `slot`,
and the caller runs it in that slot's deploy group (`deploy-docs-<slot>`), so it never overlaps a
deploy of the same slot. Each attempt starts from the pages branch's current tip and
removes the slot there; a push rejected because another slot's deploy landed first is retried the
same way as a deploy. A slot that does not exist on the pages branch fails the step. A removal
writes no build, root or `CNAME`.

### Files at the pages root

Every deploy attempt writes `CNAME` (the custom domain, default `www.gokure.dev`) and
`.nojekyll` at the root of the pages repository.

### Older tags

A deploy runs the Deploy Docs workflow of the ref it was dispatched at, with the action pin that
ref carries. A tag cut before a part of this behaviour existed deploys without it: no root
re-check, no tag check, or no retry. Sequence deploys of such tags yourself, as the recovery
procedure in the Releasing guide does.

## Self-Hosted Runner

PR Review, Claude, the calling repository's CI and its Deploy Docs job run on
`autops-kube-kure`, a self-hosted Actions Runner Controller scale set with access to in-cluster
services such as the PR review proxy. The runner image does not include `make` or `envsubst`;
workflows install what they need or avoid it.

## Reference

- [`go-kure/.github`](https://github.com/go-kure/.github): shared workflows, composite actions
  and org standards
- [`docs/standards.md`](https://github.com/go-kure/.github/blob/main/docs/standards.md): org
  standards
- [`docs/pr-review-threads.md`](https://github.com/go-kure/.github/blob/main/docs/pr-review-threads.md):
  PR Review design
- [`standards/release-process.md`](https://github.com/go-kure/.github/blob/main/standards/release-process.md):
  the Releasing guide
- [GitHub merge queue documentation](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-a-merge-queue)
