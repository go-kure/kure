#!/usr/bin/env bash
# sync-eso-pin.sh — re-pin an untagged-submodule dependency to the upstream
# release declared as `upstream_release` in versions.yaml.
#
# external-secrets moved its CRD types into an /apis submodule at v1.0 but
# never tagged that submodule (contents/apis/go.mod has no semver tags). Go
# can therefore only express the dependency as a pseudo-version, and its
# @latest resolves to upstream main HEAD — every upstream commit looks like a
# new version to Renovate. This script breaks that: it re-pins go.mod's
# pseudo-version to the commit behind a *named release* instead, so Renovate
# (via renovate.json's customManagers regex on versions.yaml's
# `upstream_release:` line, matchPackageNames-disabled for the raw gomod
# dependency) only proposes an update when upstream cuts a release.
#
# Steps:
#   1. Read `upstream_release` (e.g. "v2.9.0") for the given dep from
#      versions.yaml.
#   2. Resolve that tag to its commit via the GitHub API (falls back to
#      `git ls-remote` if the API is unreachable or rate-limited).
#   3. `go get <module>@<commit>` — NOT a hand-built pseudo-version string.
#      Go verifies a pseudo-version's embedded timestamp against the commit
#      it names, so rewriting only the hash of an existing pseudo-version
#      produces a version Go itself would reject; letting `go get` compute it
#      is the only correct way to move this pin.
#   4. `go mod tidy`.
#   5. Write the resolved commit back to versions.yaml's
#      `upstream_release_commit` field — the offline half of the drift guard
#      in scripts/sync-versions.sh, which asserts go.mod's pseudo-version
#      digest prefixes this field on every `check` run.
#   6. `./scripts/sync-versions.sh generate` to refresh docs/compatibility.md,
#      pkg/versions/versions_gen.go, and the go.mod pin comment.
#
# Idempotent: a re-run with `upstream_release` unchanged resolves the same
# commit, so `go get`/`go mod tidy` produce no further change, and step 5
# only writes versions.yaml when the resolved commit actually differs from
# what's already recorded.
#
# Invoked as a Renovate postUpgradeTasks command (renovate.json) whenever the
# upstream_release customManager dependency bumps; safe to run by hand too.
#
# Usage: ./scripts/sync-eso-pin.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
VERSIONS_FILE="$REPO_ROOT/versions.yaml"

# Only external-secrets declares upstream_release today, but nothing here is
# specific to it beyond DEP — a second untagged-submodule dependency can
# reuse this script by adding its own versions.yaml entry (go_module,
# upstream_repo, upstream_release, upstream_release_commit) and pointing a
# copy (or a parameterized call) at its DEP key. GO_MODULE and UPSTREAM_REPO
# are read from versions.yaml, not hardcoded here, so there is exactly one
# place declaring them — a second copy here could drift from versions.yaml
# the same way sync-versions.sh's drift guard exists to catch for the pin
# itself.
DEP="external-secrets"

cd "$REPO_ROOT"

GO_MODULE="$(yq ".infrastructure.${DEP}.go_module // \"\"" "$VERSIONS_FILE")"
UPSTREAM_REPO="$(yq ".infrastructure.${DEP}.upstream_repo // \"\"" "$VERSIONS_FILE")"
if [[ -z "$GO_MODULE" || "$GO_MODULE" == "null" ]]; then
    echo "sync-eso-pin: versions.yaml has no 'go_module' set for '$DEP'" >&2
    exit 1
fi
if [[ -z "$UPSTREAM_REPO" || "$UPSTREAM_REPO" == "null" ]]; then
    echo "sync-eso-pin: versions.yaml has no 'upstream_repo' set for '$DEP'" >&2
    exit 1
fi

release="$(yq ".infrastructure.${DEP}.upstream_release // \"\"" "$VERSIONS_FILE")"
if [[ -z "$release" || "$release" == "null" ]]; then
    echo "sync-eso-pin: versions.yaml has no 'upstream_release' set for '$DEP'" >&2
    exit 1
fi

echo "sync-eso-pin: resolving ${UPSTREAM_REPO}@${release} to a commit..."

# `refs/tags/<tag>` resolves to a commit SHA directly for a *lightweight* tag,
# but to a tag *object* SHA for an *annotated* tag — go get needs the commit,
# not the tag object, so an annotated tag must be peeled one level further
# via git/tags/<sha> before its .object.sha is usable. v2.9.0 today is
# lightweight (verified), but upstream's tagging convention is not a promise;
# resolve correctly for either shape rather than relying on today's shape.
# Bounded, not best-effort: unlike sync-versions.sh's resolve_tag_commit
# (which can gracefully warn and fall through on a network failure), this
# resolution is required for the script to do its job, so a longer timeout
# than that best-effort check's is appropriate. It must still be bounded —
# a black-holed/stalled connection with no deadline blocks curl (and this
# Renovate postUpgradeTasks command) indefinitely rather than failing fast
# into the script's own diagnostic below.
CURL_TIMEOUT_ARGS=(--connect-timeout 10 --max-time 30)
LS_REMOTE_TIMEOUT=30

commit=""
obj_type=""
api_response="$(curl -fsSL "${CURL_TIMEOUT_ARGS[@]}" "https://api.github.com/repos/${UPSTREAM_REPO}/git/ref/tags/${release}" 2>/dev/null || true)"
if [[ -n "$api_response" ]]; then
    commit="$(printf '%s' "$api_response" | yq -p json '.object.sha // ""' 2>/dev/null || true)"
    obj_type="$(printf '%s' "$api_response" | yq -p json '.object.type // ""' 2>/dev/null || true)"
fi
if [[ "$obj_type" == "tag" && -n "$commit" && "$commit" != "null" ]]; then
    echo "sync-eso-pin: ${release} is an annotated tag (object ${commit}), peeling to its commit" >&2
    tag_response="$(curl -fsSL "${CURL_TIMEOUT_ARGS[@]}" "https://api.github.com/repos/${UPSTREAM_REPO}/git/tags/${commit}" 2>/dev/null || true)"
    commit="$(printf '%s' "$tag_response" | yq -p json '.object.sha // ""' 2>/dev/null || true)"
fi

# Fallback: git ls-remote, no rate limit, no auth. Query both the plain ref
# and its peeled form (`^{}`, only present for annotated tags) and prefer the
# peeled line — same lightweight-vs-annotated distinction as the API path.
if [[ -z "$commit" || "$commit" == "null" ]]; then
    echo "sync-eso-pin: GitHub API lookup failed or rate-limited, falling back to git ls-remote" >&2
    ls_remote_out="$(timeout "$LS_REMOTE_TIMEOUT" git ls-remote "https://github.com/${UPSTREAM_REPO}" "refs/tags/${release}" "refs/tags/${release}^{}" 2>/dev/null || true)"
    commit="$(printf '%s\n' "$ls_remote_out" | awk -v r="refs/tags/${release}^{}" '$2 == r {print $1; found=1} END {exit !found}' || true)"
    if [[ -z "$commit" ]]; then
        commit="$(printf '%s\n' "$ls_remote_out" | awk -v r="refs/tags/${release}" '$2 == r {print $1}' | head -n1)"
    fi
fi

if [[ -z "$commit" || "$commit" == "null" ]]; then
    echo "sync-eso-pin: could not resolve tag '${release}' to a commit via API or ls-remote" >&2
    exit 1
fi

if ! [[ "$commit" =~ ^[0-9a-f]{40}$ ]]; then
    echo "sync-eso-pin: resolved value '$commit' does not look like a 40-char commit SHA" >&2
    exit 1
fi

echo "sync-eso-pin: ${release} -> ${commit}"

echo "sync-eso-pin: go get ${GO_MODULE}@${commit}"
go get "${GO_MODULE}@${commit}"

echo "sync-eso-pin: go mod tidy"
go mod tidy

# Idempotent by construction: only write when the resolved commit actually
# differs from what's already recorded, so a no-op re-run leaves
# versions.yaml untouched.
current_commit="$(yq ".infrastructure.${DEP}.upstream_release_commit // \"\"" "$VERSIONS_FILE")"
if [[ "$current_commit" != "$commit" ]]; then
    yq -i ".infrastructure.${DEP}.upstream_release_commit = \"${commit}\"" "$VERSIONS_FILE"
    echo "sync-eso-pin: versions.yaml upstream_release_commit -> ${commit}"
else
    echo "sync-eso-pin: versions.yaml upstream_release_commit already ${commit} -- no change"
fi

echo "sync-eso-pin: ./scripts/sync-versions.sh generate"
"$SCRIPT_DIR/sync-versions.sh" generate

echo "sync-eso-pin: done"
