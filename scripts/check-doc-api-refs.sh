#!/usr/bin/env bash
# check-doc-api-refs.sh — fail when a live documentation page names a builder
# function kure does not export.
#
#   scripts/check-doc-api-refs.sh              # exit 1 and list every stale reference
#   scripts/check-doc-api-refs.sh --list       # print the input sizes and exit 0
#   scripts/check-doc-api-refs.sh --self-test  # prove the extractor can still fail
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
# Which pages count comes from site/docs-map.yaml plus the docs trees a reader
# browses on GitHub, so a page mounted from a new directory cannot escape the
# check by being somewhere this script never thought to look. It needs yq.
#
# What it deliberately does not check: the pages in EXCLUDED_PAGES. A migration
# ledger must name the functions it removed, and a dated history or review record
# describes the tree as it stood on its date -- rewriting either to satisfy a
# grep would destroy the thing that makes it useful.
#
# A live page sometimes names an unresolvable identifier on purpose: a recipe
# writing about a kind that does not exist yet, or a sentence saying a particular
# helper was removed and will not come back. Skip one line:
#
#     There is no `AddDeploymentContainer`. <!-- doc-api-refs:ignore removed -->
#
# or fence a passage:
#
#     <!-- doc-api-refs:ignore-start reason -->
#     ... CreateNewKind, AddNewKindRule ...
#     <!-- doc-api-refs:ignore-end -->
#
# The reason goes inside the comment, and is not optional: a form that suppresses
# anything needs a space and then a reason starting with an alphanumeric.
# `ignore-end` suppresses nothing and so needs none. Markdown passes the whole
# line of an HTML block through verbatim, so a reason written after the `-->`
# renders as visible text on the page. A `<!-- doc-api-refs:` comment that is
# none of the three forms -- a misspelled keyword, a missing reason -- is an
# error rather than a silent skip.
#
# Prefer the single-line form -- a comment that starts a line interrupts the
# surrounding markdown paragraph, and the fence is only worth that when the names
# genuinely span lines. Keep either tight around those names and nothing else:
# everything inside is unchecked, so a real call that drifts in there rots
# unnoticed.

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
	CHANGELOG.md                       # generated from commit subjects; a released change may name what it removed
)

# Identifiers matching the builder shape that belong to somebody else's API.
EXTERNAL=(
	AddCommand         # spf13/cobra
	AddFlags           # spf13/pflag
	AddToScheme        # k8s.io/apimachinery scheme builders
	AddKnownTypes      # k8s.io/apimachinery runtime.SchemeBuilder
	SetGroupVersionKind # k8s.io/apimachinery runtime.Object
	SetupWithManager   # sigs.k8s.io/controller-runtime
	CreateReplace      # helm-controller's CRDsPolicy constant, not a constructor
)

symbols=$(mktemp)
referenced=$(mktemp)
selftest_dir=
# shellcheck disable=SC2064 # expand now: the paths must survive the function that set them
trap 'rm -rf "$symbols" "$referenced" ${selftest_dir:+"$selftest_dir"}' EXIT

# One "file:line:identifier" row per builder-shaped reference in the pages named
# on stdin (NUL-separated), skipping any passage a page fenced off. An unclosed
# fence is an error: it would silently disable the check for the rest of the file.
# ENDFILE is a gawk extension and the CI runner's awk is mawk, so the unclosed
# check runs on the first line of the NEXT file and again at END.
extract_refs() {
	awk '
		function unclosed() {
			if (skip) {
				print "unclosed doc-api-refs:ignore-start in " current > "/dev/stderr"
				rc = 1
			}
		}
		FNR == 1 { unclosed(); skip = 0; current = FILENAME }
		# A marker line is never scanned for references, whichever form it takes.
		# The three forms are matched exhaustively and anything else that opens a
		# doc-api-refs comment is an error: a typo such as `ignore-strt` would
		# otherwise fall through to the bare-ignore rule and silently suppress the
		# line, which is the one failure mode a reader cannot see. The space after
		# each keyword also keeps `ignore-end` from ever being read as `ignore`.
		/<!-- doc-api-refs:/ {
			# A form that suppresses must carry a reason, and the reason must
			# start with an alphanumeric: a space alone is not enough, since the
			# one in `<!-- doc-api-refs:ignore -->` belongs to the closing `-->`
			# and would suppress a line with nothing said about why. `ignore-end`
			# suppresses nothing, so it needs no reason.
			opens = ($0 ~ /<!-- doc-api-refs:ignore-start [A-Za-z0-9]/)
			closes = ($0 ~ /<!-- doc-api-refs:ignore-end /)
			# Both on one line is a self-contained fence: it changes no state.
			if (opens && closes) next
			if (opens) { skip = 1; next }
			if (closes) { skip = 0; next }
			if ($0 ~ /<!-- doc-api-refs:ignore [A-Za-z0-9]/) next
			print "unrecognised doc-api-refs marker in " FILENAME " line " FNR > "/dev/stderr"
			rc = 1
			next
		}
		skip { next }
		{
			line = $0
			offset = 0
			# The underscore is in the class on purpose: the declaration extractor
			# accepts it, so without it CreateDeployment_Gone would be truncated to
			# the CreateDeployment that does exist and resolve.
			while (match(line, /(Create|Set|Add)[A-Z][A-Za-z0-9_]*/)) {
				# Reject a match that continues an identifier (foo.MySetName).
				if (RSTART + offset == 1 || substr($0, RSTART + offset - 1, 1) !~ /[A-Za-z0-9_]/) {
					print FILENAME ":" FNR ":" substr(line, RSTART, RLENGTH)
				}
				offset += RSTART + RLENGTH - 1
				line = substr(line, RSTART + RLENGTH)
			}
		}
		END { unclosed(); exit rc }
	' "$@"
}

# The check is only worth its CI minute if it can go red, and every way it goes
# quietly green -- a fence marker matching more than it should, an identifier
# boundary that stops matching -- is invisible from the repo's own output. These
# cases pin the extractor against a synthetic tree instead.
self_test() {
	selftest_dir=$(mktemp -d)
	local d=$selftest_dir failures=0

	cat >"$d/plain.md" <<-'EOF'
		A page calling `CreateGone` and `CreateReal`, and a method
		reference like foo.MyCreateGone that is not a call of ours.
		CreateReal xSetSkipped CreateGone AddReal trails a rejected match.
		CreateDeployment_Gone is not CreateDeployment.
	EOF
	cat >"$d/skipped.md" <<-'EOF'
		There is no `CreateGone`. <!-- doc-api-refs:ignore removed -->
		`CreateGone` here is not skipped by the line above.
		<!-- doc-api-refs:ignore-start reason --> `AddGone` <!-- doc-api-refs:ignore-end -->
		`CreateReal` is still checked after a self-contained fence.
	EOF
	cat >"$d/fenced.md" <<-'EOF'
		<!-- doc-api-refs:ignore-start reason -->
		`CreateGone`, `AddGoneThing`
		<!-- doc-api-refs:ignore-end -->
		`CreateReal` is checked again after the fence.
	EOF
	cat >"$d/unclosed.md" <<-'EOF'
		<!-- doc-api-refs:ignore-start reason -->
		`CreateGone`
	EOF
	cat >"$d/typo.md" <<-'EOF'
		<!-- doc-api-refs:ignore-strt reason --> `CreateGone`
	EOF
	cat >"$d/noreason.md" <<-'EOF'
		<!-- doc-api-refs:ignore --> `CreateGone`
	EOF
	cat >"$d/noreason-start.md" <<-'EOF'
		<!-- doc-api-refs:ignore-start --> `CreateGone`
		<!-- doc-api-refs:ignore-end -->
	EOF

	local got
	# Line 3 pins the offset arithmetic: a rejected match between two accepted
	# ones must not consume the rest of the line. Line 4 pins the underscore.
	local want='plain.md:1:CreateGone
plain.md:1:CreateReal
plain.md:3:CreateReal
plain.md:3:CreateGone
plain.md:3:AddReal
plain.md:4:CreateDeployment_Gone
plain.md:4:CreateDeployment
skipped.md:2:CreateGone
skipped.md:4:CreateReal
fenced.md:4:CreateReal'
	got=$(extract_refs "$d/plain.md" "$d/skipped.md" "$d/fenced.md" | sed "s#^$d/##")
	if [ "$got" != "$want" ]; then
		printf 'self-test: extraction mismatch\nwant:\n%s\ngot:\n%s\n' "$want" "$got" >&2
		failures=$((failures + 1))
	fi

	if extract_refs "$d/unclosed.md" >/dev/null 2>&1; then
		printf 'self-test: an unclosed ignore-start did not fail\n' >&2
		failures=$((failures + 1))
	fi

	if extract_refs "$d/typo.md" >/dev/null 2>&1; then
		printf 'self-test: a misspelled marker was accepted instead of reported\n' >&2
		failures=$((failures + 1))
	fi

	# The space before `-->` would satisfy a bare `ignore ` / `ignore-start `
	# pattern, so a reason-less marker must be rejected explicitly rather than
	# silently honoured.
	if extract_refs "$d/noreason.md" >/dev/null 2>&1; then
		printf 'self-test: a reason-less ignore was accepted instead of reported\n' >&2
		failures=$((failures + 1))
	fi

	if extract_refs "$d/noreason-start.md" >/dev/null 2>&1; then
		printf 'self-test: a reason-less ignore-start was accepted instead of reported\n' >&2
		failures=$((failures + 1))
	fi

	# A name absent from the symbol set must be reported; one present must not.
	# CreateDeployment is in the set and CreateDeployment_Gone is not, so the
	# suffix must survive extraction or the fourth line resolves wrongly.
	printf 'AddReal\nCreateDeployment\nCreateReal\n' >"$symbols"
	extract_refs "$d/plain.md" | sort -u >"$referenced"
	local unresolved
	local want_unresolved='plain.md:1:CreateGone
plain.md:3:CreateGone
plain.md:4:CreateDeployment_Gone'
	unresolved=$(report_unresolved | sed "s#^$d/##")
	if [ "$unresolved" != "$want_unresolved" ]; then
		printf 'self-test: resolution mismatch\nwant:\n%s\ngot:\n%s\n' "$want_unresolved" "$unresolved" >&2
		failures=$((failures + 1))
	fi

	if [ "$failures" -ne 0 ]; then
		printf 'check-doc-api-refs --self-test: %s failure(s)\n' "$failures" >&2
		return 1
	fi
	printf 'check-doc-api-refs --self-test: ok\n'
}

# Every row in $referenced whose identifier is absent from $symbols. Exits 0 when
# it printed at least one row, so it reads as `if unresolved=$(report_unresolved)`.
report_unresolved() {
	local row name found=1
	while IFS= read -r row; do
		[ -n "$row" ] || continue
		name=${row##*:}
		grep -qxF "$name" "$symbols" && continue
		printf '%s\n' "$row"
		found=0
	done <"$referenced"
	return "$found"
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
	exit
fi

# Exported functions and methods declared in the public tree. Test files count:
# a symbol that only exists in a _test.go file is not importable, and no page may
# cite one, so they are excluded.
find pkg -name '*.go' ! -name '*_test.go' -print0 |
	xargs -0 grep -hoE '^func (\([^)]*\) )?[A-Z][A-Za-z0-9_]*' |
	sed -E 's/^func (\([^)]*\) )?//' |
	sort -u >"$symbols"

printf '%s\n' "${EXTERNAL[@]}" >>"$symbols"
sort -u -o "$symbols" "$symbols"

# Live pages: everything the site publishes, plus the repository-root and docs/
# trees a reader browses on GitHub without the site.
#
# site/docs-map.yaml is the authority for what is published, so the page list is
# derived from it rather than from a hand-kept list of roots: DEVELOPMENT.md and
# examples/patches/README.md are mounted from outside docs/ and site/content, and
# a future mount from a new directory must not silently escape the check. The
# find roots stay as well -- a page under site/content is published without
# appearing in the map, and a page under docs/ is read on GitHub whether or not
# it is mounted.
#
# Each enumeration is a plain assignment, run and checked on its own, rather than
# three commands inside one process substitution: a process substitution's exit
# status is never examined, so a missing yq would leave the page list silently
# short of every mapped page while the run still reported "all resolved" -- the
# fail-open this script exists to prevent. An empty result is treated the same
# way, since a yq that succeeds against a restructured map yields nothing.
docs_pages=$(find README.md docs site/content -name '*.md' -type f)
# The package READMEs are the API-reference pages the site mounts, so they are
# exactly the pages a stale call hurts most.
pkg_pages=$(find pkg -name 'README.md' -type f)
map_pages=$(yq -r '.packages[].readme, .extra_mounts[].source' site/docs-map.yaml)

for _list in docs_pages pkg_pages map_pages; do
	if [ -z "${!_list}" ]; then
		printf 'check-doc-api-refs: %s enumerated no pages -- refusing to check a partial set\n' \
			"$_list" >&2
		exit 1
	fi
done
unset _list

pages=()
while IFS= read -r page; do
	[ -f "$page" ] || continue
	skip=0
	for excluded in "${EXCLUDED_PAGES[@]}"; do
		case "$page" in "$excluded"* | "./$excluded"*) skip=1 ;; esac
	done
	[ "$skip" -eq 0 ] && pages+=("$page")
done < <(printf '%s\n%s\n%s\n' "$docs_pages" "$pkg_pages" "$map_pages" | sed 's#^\./##' | sort -u)

if [ "${1:-}" = "--list" ]; then
	printf 'symbols: %s\n' "$(wc -l <"$symbols")"
	printf 'pages:   %s\n' "${#pages[@]}"
	exit 0
fi

extract_refs "${pages[@]}" | sort -u >"$referenced"

if unresolved=$(report_unresolved); then
	printf 'Documentation names builder functions that pkg/ does not export:\n\n' >&2
	while IFS= read -r row; do printf '  %s\n' "$row" >&2; done <<<"$unresolved"
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
