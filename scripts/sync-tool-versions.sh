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
# Restrict to the [tools] table (see the matching comment in check-tool-versions.sh, including
# its whitespace- and quoted-key-tolerant header match) and, same discipline as the checker,
# require exactly one match -- head -1 previously let a duplicate pin resolve silently to
# whichever occurrence sorted first, then propagate that guess into every target file instead of
# refusing to sync an invalid mise.toml.
MISE_TOOLS="$(awk '/^[[:space:]]*\[[[:space:]]*["'\'']?tools["'\'']?[[:space:]]*\]/{f=1;next} /^[[:space:]]*\[/{f=0} f' "$MISE")"
# Same pattern check-tool-versions.sh's MISE_PATTERN accepts (any "=" spacing, either TOML
# quote style) -- a narrower extraction here than the checker's count/match means a validly
# formatted variant the checker accepts still leaves this script finding nothing to sync.
MISE_PATTERN='^golangci-lint[[:space:]]*=[[:space:]]*["'\'']'
mise_count="$(printf '%s\n' "$MISE_TOOLS" | grep -cE "$MISE_PATTERN" || true)"
if [ "$mise_count" -ne 1 ]; then
	echo "Error: expected exactly 1 golangci-lint [tools] entry in $MISE, found $mise_count"
	exit 1
fi
MISE_VAL="$(printf '%s\n' "$MISE_TOOLS" | grep -E "$MISE_PATTERN" | sed -E "s/^golangci-lint[[:space:]]*=[[:space:]]*[\"']//; s/[\"'].*//")"
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
#
# Temp-file + mv, not `sed -i -E` — this script declares itself #!/bin/sh (portable POSIX),
# but BSD/macOS sed's `-i` takes the backup suffix as its next argument, so `-i -E` parses
# `-E` as that suffix: the substitution then runs as a literal BRE (no groups), nothing
# matches, and a stray `Makefile-E`/`ci.yml-E` is left behind. The self-verify below would
# then fail with a misleading "a target file's format probably drifted" — sync-govulncheck-docs.sh
# already avoids this the same way.
sed -E "s/^(GOLANGCI_LINT_VERSION[[:space:]]*:=[[:space:]]*)v[0-9.]+/\\1v$MISE_VAL/" Makefile > Makefile.tmp && mv Makefile.tmp Makefile
sed -E "s/^([[:space:]]*GOLANGCI_LINT_VERSION:[[:space:]]*)[\"']?v[0-9.]+[\"']?([[:space:]]*)\$/\\1'v$MISE_VAL'\\2/" .github/workflows/ci.yml > .github/workflows/ci.yml.tmp && mv .github/workflows/ci.yml.tmp .github/workflows/ci.yml
# shellcheck disable=SC2016 # literal markdown backticks, not unexpanded vars
sed -E "s/^- Golangci-lint Version: \`v[0-9.]+\`\$/- Golangci-lint Version: \`v$MISE_VAL\`/" docs/github-workflows.md > docs/github-workflows.md.tmp && mv docs/github-workflows.md.tmp docs/github-workflows.md

echo "Synced golangci-lint pins to $MISE_VAL (Makefile, ci.yml, docs/github-workflows.md)"

# sed exits 0 even when a pattern matched zero lines (format drift, a
# renamed line) — self-verify by re-running the checker rather than
# trusting sed's silent-success-on-no-match behavior.
if ! sh "$ROOT/scripts/check-tool-versions.sh"; then
	echo "Error: sync ran but a pin still does not match $MISE ($MISE_VAL) — a target file's format probably drifted from what this script's sed patterns expect. Fix scripts/sync-tool-versions.sh and scripts/check-tool-versions.sh together." >&2
	exit 1
fi
