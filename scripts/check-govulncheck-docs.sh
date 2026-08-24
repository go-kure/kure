#!/bin/sh
# check-govulncheck-docs.sh — enforce parity between the govulncheck version pinned in
# .github/workflows/ci.yml's env: block and every mention of it in docs/github-workflows.md.
#
# Unlike golangci-lint (source of truth: mise.toml, a file none of its own targets are),
# govulncheck's source of truth is ci.yml itself — the same file Renovate's customManagers
# regex entry in renovate.json bumps directly. Before that customManager existed, govulncheck
# was invisible to Renovate and any manual bump landed in the same PR as its doc updates; now
# that it can update automatically, an automated bump leaves the docs stale unless something
# enforces parity.
#
# CI images do not have mise installed, so this is a plain POSIX shell script CI can run
# directly — it must NOT depend on mise.
#
# Exit non-zero on any mismatch. Run scripts/sync-govulncheck-docs.sh to fix.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

CI_VAL="$(grep -E '^[[:space:]]*GOVULNCHECK_VERSION:' .github/workflows/ci.yml | head -1 | sed -E 's/^[^:]+:[[:space:]]*//' | tr -d "\"'" | sed -E 's/^v//')"
if [ -z "$CI_VAL" ]; then
	echo "✗ govulncheck: GOVULNCHECK_VERSION not found in .github/workflows/ci.yml"
	exit 1
fi

DOC="docs/github-workflows.md"
if [ ! -f "$DOC" ]; then
	echo "✗ govulncheck: $DOC not found"
	exit 1
fi

# Every line pairing "govulncheck" (case-insensitive) with a vX.Y.Z-shaped version is a target —
# matches the doc's three current mentions (a table cell, a version-list bullet, a cache-key
# bullet) without hand-anchoring three different line shapes. Zero matches is itself a failure:
# a renamed/deleted mention must not silently pass.
LINES="$(grep -inE 'govulncheck.*v[0-9]+\.[0-9]+' "$DOC" || true)"
if [ -z "$LINES" ]; then
	echo "✗ $DOC: no govulncheck version mention found (expected at least one)"
	exit 1
fi

ERRORS=0
LINE_COUNT=0
# Heredoc, not a pipe, so the loop runs in this shell (not a subshell) and ERRORS/LINE_COUNT
# survive past the loop — a `LINES | while read` here would silently drop every increment.
while IFS= read -r line; do
	lineno="${line%%:*}"
	val="$(printf '%s\n' "$line" | grep -oE 'v[0-9]+\.[0-9]+(\.[0-9]+)?' | head -1 | sed -E 's/^v//')"
	LINE_COUNT=$((LINE_COUNT + 1))
	if [ "$val" != "$CI_VAL" ]; then
		echo "✗ $DOC:$lineno has v$val (expected v$CI_VAL from .github/workflows/ci.yml)"
		ERRORS=$((ERRORS + 1))
	fi
done <<EOF
$LINES
EOF

if [ "$ERRORS" -gt 0 ]; then
	echo "Found $ERRORS mismatch(es). Run 'scripts/sync-govulncheck-docs.sh' (or 'mise run sync-govulncheck-docs') to fix."
	exit 1
fi
echo "✓ govulncheck docs (v$CI_VAL, $LINE_COUNT mention(s) in $DOC)"
