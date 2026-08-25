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
#   6. `./scripts/sync-versions.sh generate` to refresh docs/compatibility.md
#      and the go.mod pin comment.
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
# specific to it beyond these three constants — a second untagged-submodule
# dependency can reuse this script by adding its own entry and pointing a
# copy (or a parameterized call) at these three values.
DEP="external-secrets"
GO_MODULE="github.com/external-secrets/external-secrets/apis"
UPSTREAM_REPO="external-secrets/external-secrets"

cd "$REPO_ROOT"

release="$(yq ".infrastructure.${DEP}.upstream_release // \"\"" "$VERSIONS_FILE")"
if [[ -z "$release" || "$release" == "null" ]]; then
    echo "sync-eso-pin: versions.yaml has no 'upstream_release' set for '$DEP'" >&2
    exit 1
fi

echo "sync-eso-pin: resolving ${UPSTREAM_REPO}@${release} to a commit..."

# Primary: GitHub API tag ref lookup (unauthenticated, works for public repos,
# subject to the low unauthenticated rate limit).
commit=""
api_response="$(curl -fsSL "https://api.github.com/repos/${UPSTREAM_REPO}/git/ref/tags/${release}" 2>/dev/null || true)"
if [[ -n "$api_response" ]]; then
    commit="$(printf '%s' "$api_response" | yq -p json '.object.sha // ""' 2>/dev/null || true)"
fi

# Fallback: git ls-remote, no rate limit, no auth.
if [[ -z "$commit" || "$commit" == "null" ]]; then
    echo "sync-eso-pin: GitHub API lookup failed or rate-limited, falling back to git ls-remote" >&2
    commit="$(git ls-remote "https://github.com/${UPSTREAM_REPO}" "refs/tags/${release}" | awk '{print $1}' | head -n1)"
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
