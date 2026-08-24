#!/bin/sh
# sync-tool-versions.sh — rewrite the golangci-lint version pinned by hand in
# Makefile, .github/workflows/ci.yml and docs/github-workflows.md from the
# mise.toml [tools] source of truth.
#
# The inverse of scripts/check-tool-versions.sh — keep every copy equal, or
# the checker will fail on a file this cannot fix, or Renovate's
# postUpgradeTasks will sync a file the checker does not police.
#
# Run scripts/check-tool-versions.sh afterwards (or 'mise run check-tool-versions').
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

MISE="mise.toml"
MISE_VAL="$(grep -E '^golangci-lint = "' "$MISE" | head -1 | cut -d'"' -f2)"
# See the matching comment in check-tool-versions.sh: strip a possible "v" prefix so this
# doesn't write "v$MISE_VAL" as "vv2.13.1" into every target.
MISE_VAL="${MISE_VAL#v}"
if [ -z "$MISE_VAL" ]; then
	echo "Error: golangci-lint not found in $MISE [tools]"
	exit 1
fi

# Both patterns below match anything scripts/check-tool-versions.sh's own count/extraction
# regexes accept (arbitrary spacing around ":=", arbitrary indentation and quote style in
# ci.yml) — a narrower syncer than checker means a validly-formatted variant the checker
# tolerates still leaves the syncer rewriting nothing, and its own self-verify below aborting.
sed -i -E "s/^(GOLANGCI_LINT_VERSION[[:space:]]*:=[[:space:]]*)v[0-9.]+/\\1v$MISE_VAL/" Makefile
sed -i -E "s/^([[:space:]]*GOLANGCI_LINT_VERSION:[[:space:]]*)[\"']?v[0-9.]+[\"']?([[:space:]]*)\$/\\1'v$MISE_VAL'\\2/" .github/workflows/ci.yml
# shellcheck disable=SC2016 # literal markdown backticks, not unexpanded vars
sed -i -E "s/^- Golangci-lint Version: \`v[0-9.]+\`\$/- Golangci-lint Version: \`v$MISE_VAL\`/" docs/github-workflows.md

echo "Synced golangci-lint pins to $MISE_VAL (Makefile, ci.yml, docs/github-workflows.md)"

# sed exits 0 even when a pattern matched zero lines (format drift, a
# renamed line) — self-verify by re-running the checker rather than
# trusting sed's silent-success-on-no-match behavior.
if ! sh "$ROOT/scripts/check-tool-versions.sh"; then
	echo "Error: sync ran but a pin still does not match $MISE ($MISE_VAL) — a target file's format probably drifted from what this script's sed patterns expect. Fix scripts/sync-tool-versions.sh and scripts/check-tool-versions.sh together." >&2
	exit 1
fi
