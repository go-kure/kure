#!/usr/bin/env bash
# vendor-guard.sh — keep the vendored copy of go-kure/.github's forbidden-terms
# guard in sync with the SHA pinned in .github/workflows/ci.yml's `ref:` field.
#
# .github/workflows/ci.yml's forbidden-terms job pins go-kure/.github twice:
# once via the check-forbidden-terms action's `uses:` digest (tracked by
# Renovate's github-actions manager) and once via a second checkout step's
# `ref:` (invisible to that manager — see renovate.json's customManagers
# entry). Left to drift, `forbidden-terms` byte-compares the vendored copy
# against a stale revision. This script closes that gap: given the `ref:`
# SHA, it re-fetches the canonical guard script from go-kure/.github at that
# SHA and re-vendors it, so the two pins move together.
#
# Invoked as a Renovate postUpgradeTasks command (renovate.json) whenever the
# go-kure/.github customManager dependency bumps; safe to run by hand too.
# Idempotent: a no-op re-run leaves the vendored file untouched.
#
# Usage: ./scripts/vendor-guard.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CI_WORKFLOW="$REPO_ROOT/.github/workflows/ci.yml"
VENDORED="$REPO_ROOT/site/scripts/check-forbidden-terms.sh"

# Pull the `ref:` SHA out of the "Checkout canonical guard" step — the
# unmanaged pin the customManager in renovate.json tracks. If a second
# `repository: go-kure/.github` checkout is ever added, `grep -A3 | grep -m1`
# would silently pick a `ref:` from whichever block happened to match first
# in the concatenated output, possibly the wrong one -- refuse instead of
# guessing.
repo_count="$(grep -c 'repository: go-kure/\.github' "$CI_WORKFLOW" || true)"
if [[ "$repo_count" -gt 1 ]]; then
    echo "vendor-guard: $repo_count 'repository: go-kure/.github' checkout blocks in $CI_WORKFLOW -- ambiguous, refusing to guess which ref: to track" >&2
    exit 1
fi

# `grep -A3` after the `repository:` line lands on the very next `ref:`
# line; anchoring on both means a reordered or removed YAML block fails
# loudly here instead of silently reading a stale or wrong SHA. `|| true` on
# the whole pipeline is required under `pipefail`: if either grep finds no
# match, that grep's non-zero status becomes the pipeline's status even
# though the final awk still exits 0, which would otherwise abort here
# under `set -e` before the empty-$sha check below ever runs — verified by
# direct probe.
sha="$(grep -A3 'repository: go-kure/\.github' "$CI_WORKFLOW" | grep -m1 'ref:' | awk '{print $2}' || true)"

if [[ -z "$sha" ]]; then
    echo "vendor-guard: could not find 'ref:' after 'repository: go-kure/.github' in $CI_WORKFLOW" >&2
    exit 1
fi

if ! [[ "$sha" =~ ^[0-9a-f]{40}$ ]]; then
    echo "vendor-guard: '$sha' does not look like a 40-char commit SHA" >&2
    exit 1
fi

url="https://raw.githubusercontent.com/go-kure/.github/${sha}/scripts/check-forbidden-terms.sh"

fetched="$(mktemp)" || {
    echo "vendor-guard: mktemp failed -- no writable temporary directory (check \$TMPDIR and free space)" >&2
    exit 1
}
if [[ -z "$fetched" || ! -e "$fetched" ]]; then
    echo "vendor-guard: mktemp produced an unusable path ('$fetched')" >&2
    exit 1
fi
trap 'rm -f "$fetched"' EXIT

if ! curl -fsSL "$url" -o "$fetched"; then
    echo "vendor-guard: failed to fetch $url" >&2
    exit 1
fi

if [[ ! -s "$fetched" ]]; then
    echo "vendor-guard: fetched file from $url is empty" >&2
    exit 1
fi

# Idempotent by construction: only write (and re-chmod) when the fetched
# content actually differs from what's vendored, so a second run against an
# already-synced tree makes no further change to $VENDORED.
if [[ -f "$VENDORED" ]] && cmp -s "$fetched" "$VENDORED"; then
    echo "vendor-guard: $VENDORED already matches go-kure/.github@${sha} — no change"
    exit 0
fi

mkdir -p "$(dirname "$VENDORED")"
cp "$fetched" "$VENDORED"
chmod +x "$VENDORED"
echo "vendor-guard: re-vendored $VENDORED from go-kure/.github@${sha}"
