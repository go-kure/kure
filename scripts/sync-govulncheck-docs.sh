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

# Trim trailing whitespace too — an untrimmed value would embed a literal space in every doc
# mention this script rewrites, via the ver substitution below.
CI_VAL="$(grep -E '^[[:space:]]*GOVULNCHECK_VERSION:' .github/workflows/ci.yml | head -1 | sed -E 's/^[^:]+:[[:space:]]*//' | tr -d "\"'" | sed -E 's/^v//; s/[[:space:]]+$//')"
if [ -z "$CI_VAL" ]; then
	echo "Error: GOVULNCHECK_VERSION not found in .github/workflows/ci.yml"
	exit 1
fi

DOC="docs/github-workflows.md"
if [ ! -f "$DOC" ]; then
	echo "Error: $DOC not found"
	exit 1
fi

# Rewrite only the version immediately following the FIRST "govulncheck" on each line — a
# version-shaped token before it (another tool's pin) must stay untouched, and a line can
# legitimately mention govulncheck twice (e.g. "govulncheck ... v1.7.0 ... govulncheck-gate
# action"), so a sed `.*` greedy match to the LAST occurrence would skip past the actual pin.
# awk's index()/match() give genuine first-occurrence, single-token splitting that sed's
# regex-only approach can't (POSIX ERE has no non-greedy quantifier).
awk -v ver="$CI_VAL" '
{
	line = $0
	idx = index(tolower(line), "govulncheck")
	if (idx > 0) {
		prefix = substr(line, 1, idx + length("govulncheck") - 1)
		rest = substr(line, idx + length("govulncheck"))
		if (match(rest, /v[0-9]+\.[0-9]+(\.[0-9]+)?/)) {
			rest = substr(rest, 1, RSTART - 1) "v" ver substr(rest, RSTART + RLENGTH)
		}
		line = prefix rest
	}
	print line
}' "$DOC" > "$DOC.tmp" && mv "$DOC.tmp" "$DOC"

echo "Synced govulncheck doc mentions to v$CI_VAL ($DOC)"

# sed exits 0 even when a pattern matched zero lines — self-verify by re-running the checker
# rather than trusting sed's silent-success-on-no-match behavior.
if ! sh "$ROOT/scripts/check-govulncheck-docs.sh"; then
	echo "Error: sync ran but a mention still does not match v$CI_VAL — the doc's format probably drifted from what this script's sed pattern expects. Fix scripts/sync-govulncheck-docs.sh and scripts/check-govulncheck-docs.sh together." >&2
	exit 1
fi
