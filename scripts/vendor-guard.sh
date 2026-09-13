#!/usr/bin/env bash
# vendor-guard.sh — keep the vendored copy of go-kure/.github's forbidden-terms
# guard in sync with the check-forbidden-terms action pin in
# .github/workflows/ci.yml.
#
# .github/workflows/ci.yml's forbidden-terms job checks out go-kure/.github a
# second time (to byte-compare the vendored guard against its canonical
# source) at a ref derived, at CI-run time, from the check-forbidden-terms
# action's own `uses:@<sha>` pin (go-kure/kure#813) — the same single pin
# Renovate's github-actions manager already tracks, so there is nothing left
# for this script to independently extract from a second `ref:` literal.
# This script re-fetches the canonical guard script from go-kure/.github at
# that same pin and re-vendors it, so the vendored copy and the pin move
# together.
#
# Invoked as a Renovate postUpgradeTasks command (renovate.json) whenever the
# go-kure/.github dependency bumps; safe to run by hand too. Idempotent: a
# no-op re-run leaves the vendored file untouched.
#
# Usage: ./scripts/vendor-guard.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CI_WORKFLOW="$REPO_ROOT/.github/workflows/ci.yml"
VENDORED="$REPO_ROOT/site/scripts/check-forbidden-terms.sh"

# Pull the ref out of the check-forbidden-terms action's own uses:@<sha> pin
# — the same pin Renovate's github-actions manager tracks, and the exact
# expression the CI resolver step uses, so guard and CI can never disagree.
# Refuse rather than guess if the pin isn't found exactly once: an added or
# removed occurrence (this repo's or a second, unrelated one) is ambiguous,
# not a reason to pick the first match. `|| true` is required: grep -c exits
# 1 on zero matches even though it still prints "0", and under set -e an
# unguarded command substitution would abort the whole script right here —
# silently, before this check's own error message ever runs.
#
# Portable POSIX regex, not GNU-only `\s`/`\S`/`-P`: this script (unlike the
# CI resolver step) is documented above as safe to run by hand, and
# DEVELOPMENT.md's prerequisites don't require GNU grep -- a run on macOS's
# BSD grep would otherwise either silently misparse (`\s` in `-E` is literal
# "s" to BSD grep, not whitespace) or hard-fail (`-P` is a GNU extension BSD
# grep doesn't have at all), in both cases masked by the `|| true` above and
# reported as "pin not found" instead of the real cause.
occurrences="$(grep -cE '^[[:space:]]*uses:.*check-forbidden-terms@' "$CI_WORKFLOW" || true)"
if [[ "$occurrences" -ne 1 ]]; then
    echo "vendor-guard: expected exactly one uses:...check-forbidden-terms@<ref> pin in $CI_WORKFLOW, found $occurrences -- refusing to guess which one to track" >&2
    exit 1
fi

# Anchored to a `uses:` line, the same expression the CI resolver step uses,
# so guard and CI can never disagree. Capture the whole non-whitespace token
# after the @ first, then validate it as one anchored ^...{40}$ match below
# -- not `\K[0-9a-f]{40}` in a single grep, which would silently accept a
# 41-character hex run (matching only the first 40) or 40 hex characters
# followed by a stray non-hex character (matching the 40 and stopping) with
# no indication either happened. sed's `s///p`, not grep -oP: portable POSIX
# BRE (`\(...\)` capture group, `[[:space:]]` class) instead of PCRE, same
# reason as the occurrence count above. No `|| true` needed here -- sed exits
# 0 whether or not the pattern matches; a non-match just leaves $sha empty,
# caught by the check right below.
sha="$(sed -n 's/^[[:space:]]*uses:.*check-forbidden-terms@\([^[:space:]]*\).*/\1/p' "$CI_WORKFLOW")"

if [[ -z "$sha" ]]; then
    echo "vendor-guard: could not extract the check-forbidden-terms action pin from $CI_WORKFLOW" >&2
    exit 1
fi

if ! [[ "$sha" =~ ^[0-9a-f]{40}$ ]]; then
    echo "vendor-guard: '$sha' is not a bare 40-character commit SHA -- refusing to use a truncated or malformed value" >&2
    exit 1
fi

url="https://raw.githubusercontent.com/go-kure/.github/${sha}/scripts/check-forbidden-terms.sh"

fetched="$(mktemp)" || {
    echo "vendor-guard: mktemp failed -- either the mktemp binary is missing, or it could not create a file (check \$TMPDIR and free space)" >&2
    exit 1
}
# -f, not -e: this call takes no mktemp args, so it always creates a plain
# file, never a directory.
if [[ -z "$fetched" || ! -f "$fetched" ]]; then
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
