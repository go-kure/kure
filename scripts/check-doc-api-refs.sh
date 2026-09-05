#!/usr/bin/env bash
# check-doc-api-refs.sh — fail when a live documentation page names a builder
# function kure does not export.
#
#   scripts/check-doc-api-refs.sh          # exit 1 and list every stale reference
#   scripts/check-doc-api-refs.sh --list   # print the resolved symbol set and exit 0
#
# The builder-contract epic (ADR-038) removed several hundred Set*/Add*/Create*
# helpers. A page that still calls one reads as current API and is worse than no
# page at all, so this check is the machine-checkable half of "the docs describe
# the library that shipped".
#
# What it checks: every identifier shaped like a kure builder call
# (Create|Set|Add followed by an upper-case letter) that appears in a live page
# must be declared in pkg/ as a function or method. Names outside that shape are
# not builders and are not checked; upstream and third-party calls that happen to
# fit the shape are listed in EXTERNAL below.
#
# What it deliberately does not check: the pages in EXCLUDED_PAGES. A migration
# ledger must name the functions it removed, and a dated history or review record
# describes the tree as it stood on its date -- rewriting either to satisfy a
# grep would destroy the thing that makes it useful.
#
# A page that needs a placeholder name -- a recipe writing about a kind that does
# not exist yet -- fences the passage:
#
#     <!-- doc-api-refs:ignore-start --> reason
#     ... CreateNewKind, AddNewKindRule ...
#     <!-- doc-api-refs:ignore-end -->
#
# Keep the fence around the placeholders and nothing else: everything inside it
# is unchecked, so a real call that drifts in there rots unnoticed.

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

# Pages that may name a removed function, with the reason each is exempt.
EXCLUDED_PAGES=(
	docs/builder-contract-release-1.md # the release-1 deletion ledger: naming them is its job
	docs/history/                      # dated design records; true as of their date
	docs/reviews/                      # dated review records; ditto
	docs/ux-design.md                  # proposed UX, not shipped API (says so in its header)
	docs/plugin-architecture-design.md # proposed plugin API, not shipped (ditto)
)

# Identifiers matching the builder shape that belong to somebody else's API.
EXTERNAL=(
	AddCommand         # spf13/cobra
	AddFlags           # spf13/pflag
	AddToScheme        # k8s.io/apimachinery scheme builders
	AddKnownTypes      # k8s.io/apimachinery runtime.SchemeBuilder
	SetGroupVersionKind # k8s.io/apimachinery runtime.Object
	SetupWithManager   # sigs.k8s.io/controller-runtime
)

symbols=$(mktemp)
referenced=$(mktemp)
trap 'rm -f "$symbols" "$referenced"' EXIT

# Exported functions and methods declared in the public tree. Test files count:
# a symbol that only exists in a _test.go file is not importable, and no page may
# cite one, so they are excluded.
find pkg -name '*.go' ! -name '*_test.go' -print0 |
	xargs -0 grep -hoE '^func (\([^)]*\) )?[A-Z][A-Za-z0-9_]*' |
	sed -E 's/^func (\([^)]*\) )?//' |
	sort -u >"$symbols"

printf '%s\n' "${EXTERNAL[@]}" >>"$symbols"
sort -u -o "$symbols" "$symbols"

# Live pages: everything under the documentation roots that is not exempt.
pages=()
while IFS= read -r page; do
	skip=0
	for excluded in "${EXCLUDED_PAGES[@]}"; do
		case "$page" in "$excluded"* | "./$excluded"*) skip=1 ;; esac
	done
	[ "$skip" -eq 0 ] && pages+=("$page")
done < <(find README.md docs site/content -name '*.md' -type f | sort)

if [ "${1:-}" = "--list" ]; then
	printf 'symbols: %s\n' "$(wc -l <"$symbols")"
	printf 'pages:   %s\n' "${#pages[@]}"
	exit 0
fi

# One "file:line:identifier" row per builder-shaped reference in a live page,
# skipping any passage the page fenced off. An unclosed fence is an error: it
# would silently disable the check for the rest of the file.
# ENDFILE is a gawk extension and the CI runner's awk is mawk, so the unclosed
# check runs on the first line of the NEXT file and again at END.
awk '
	function unclosed() {
		if (skip) {
			print "unclosed doc-api-refs:ignore-start in " current > "/dev/stderr"
			rc = 1
		}
	}
	FNR == 1 { unclosed(); skip = 0; current = FILENAME }
	/<!-- doc-api-refs:ignore-start -->/ { skip = 1; next }
	/<!-- doc-api-refs:ignore-end -->/   { skip = 0; next }
	skip { next }
	{
		line = $0
		offset = 0
		while (match(line, /(Create|Set|Add)[A-Z][A-Za-z0-9]*/)) {
			# Reject a match that continues an identifier (foo.MySetName).
			if (RSTART + offset == 1 || substr($0, RSTART + offset - 1, 1) !~ /[A-Za-z0-9_]/) {
				print FILENAME ":" FNR ":" substr(line, RSTART, RLENGTH)
			}
			offset += RSTART + RLENGTH - 1
			line = substr(line, RSTART + RLENGTH)
		}
	}
	END { unclosed(); exit rc }
' "${pages[@]}" | sort -u >"$referenced"

status=0
while IFS= read -r row; do
	[ -n "$row" ] || continue
	name=${row##*:}
	grep -qxF "$name" "$symbols" && continue
	if [ "$status" -eq 0 ]; then
		printf 'Documentation names builder functions that pkg/ does not export:\n\n' >&2
		status=1
	fi
	printf '  %s\n' "$row" >&2
done <"$referenced"

if [ "$status" -ne 0 ]; then
	cat >&2 <<-'EOF'

		Each row is a page naming a function no longer in the public API. Fix the page
		-- the replacement expression for every function the builder-contract epic
		removed is in docs/builder-contract-release-1.md. If the reference is
		deliberate (a dated record, or a third-party API that happens to match the
		Create/Set/Add shape), add the page to EXCLUDED_PAGES or the name to EXTERNAL
		in scripts/check-doc-api-refs.sh, with the reason.
	EOF
	exit 1
fi

printf 'check-doc-api-refs: %s pages, %s builder references, all resolved.\n' \
	"${#pages[@]}" "$(wc -l <"$referenced")"
