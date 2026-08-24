#!/bin/sh
# sync-govulncheck-docs.sh — rewrite every govulncheck version mention in
# docs/github-workflows.md from .github/workflows/ci.yml's GOVULNCHECK_VERSION pin.
#
# The inverse of scripts/check-govulncheck-docs.sh. ci.yml is the source of truth here (not
# mise.toml) — it's the file Renovate's govulncheck customManager in renovate.json bumps
# directly, so this script has nothing to read except the file it's also rewriting a copy of.
#
# Run scripts/check-govulncheck-docs.sh afterwards (or 'mise run check-govulncheck-docs').
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

CI_VAL="$(grep -E '^[[:space:]]*GOVULNCHECK_VERSION:' .github/workflows/ci.yml | head -1 | sed -E 's/^[^:]+:[[:space:]]*//' | tr -d "\"'" | sed -E 's/^v//')"
if [ -z "$CI_VAL" ]; then
	echo "Error: GOVULNCHECK_VERSION not found in .github/workflows/ci.yml"
	exit 1
fi

DOC="docs/github-workflows.md"
if [ ! -f "$DOC" ]; then
	echo "Error: $DOC not found"
	exit 1
fi

# Every line pairing "govulncheck" (case-insensitive) with a version gets that version
# rewritten — same selection rule as the checker, so a line the checker would flag is a line
# this can actually fix.
sed -i -E "/govulncheck/I s/v[0-9]+\.[0-9]+(\.[0-9]+)?/v$CI_VAL/" "$DOC"

echo "Synced govulncheck doc mentions to v$CI_VAL ($DOC)"

# sed exits 0 even when a pattern matched zero lines — self-verify by re-running the checker
# rather than trusting sed's silent-success-on-no-match behavior.
if ! sh "$ROOT/scripts/check-govulncheck-docs.sh"; then
	echo "Error: sync ran but a mention still does not match v$CI_VAL — the doc's format probably drifted from what this script's sed pattern expects. Fix scripts/sync-govulncheck-docs.sh and scripts/check-govulncheck-docs.sh together." >&2
	exit 1
fi
