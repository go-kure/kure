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
# What it checks: every identifier shaped like a kure builder call -- Create, Set
# or Add followed by an upper-case letter, or the generic constructor's
# `Create[T]` bracket -- that appears in a live page must be declared in pkg/ as a
# function or method. Names outside that shape are not builders and are not
# checked; upstream and third-party calls that happen to fit the shape are listed
# in EXTERNAL below.
#
# Resolution is package-aware where the page says which package it means. pkg/
# holds same-named declarations in unrelated packages -- CreateLayoutWithResources
# is a method on three receivers across pkg/stack/fluxcd and pkg/stack/argocd --
# so a name written `fluxcd.CreateX` is resolved in fluxcd and not answered by an
# X that only argocd declares. A name written without a package selector is still
# resolved against the whole tree; report_unresolved records why that limit is
# deliberate.
#
# Which pages count comes from site/docs-map.yaml plus the docs trees a reader
# browses on GitHub, so a page mounted from a new directory cannot escape the
# check by being somewhere this script never thought to look. It needs yq.
#
# The public Go files under pkg/ are pages too: pkg.go.dev publishes their doc
# comments, and a doc comment naming a removed helper is as stale as a Markdown
# page naming one. Only the comment lines of a Go file are read -- code that
# calls a removed function does not compile, so the compiler already covers it.
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
# Every form must be a complete HTML comment, terminator included: an
# `ignore-start` whose `-->` was forgotten is malformed markdown that hides the
# rest of the page, so accepting it would grant a suppression nothing on the
# rendered site can show. The reason goes inside the comment, and is not
# optional: a form that suppresses anything needs a space and then a reason
# starting with an alphanumeric.
# `ignore-end` suppresses nothing and so needs none -- and because it does not,
# it must actually close a fence: a stray one is an error rather than a marker
# line that quietly takes its own references with it. Markdown passes the whole
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
	docs/history/                      # dated design records; true as of their date
	docs/reviews/                      # dated review records; ditto
	docs/ux-design.md                  # proposed UX, not shipped API (says so in its header)
	docs/plugin-architecture-design.md # proposed plugin API, not shipped (ditto)
	CHANGELOG.md                       # generated from commit subjects; a released change may name what it removed
)

# Pages whose job is to name removed functions beside their live replacements.
# Excluding such a page wholesale exempts the replacement column too, and those
# are live API: a later rename leaves the migration guide recommending a call
# that no longer compiles, with nothing red. The two halves cannot be separated
# by line -- 58 rows of this ledger name a removed builder and a live one on the
# same line, and its three-column tables put the removed names in the middle
# cell, so an ignore-fence covers the wrong thing whichever way it is drawn.
#
# So the exemption is per name, and the names are written down in REMOVED_LIST
# rather than inferred from where they sit on the page. Inferring them does not
# work: this ledger's tables are not all removal tables. Some pair an old
# signature with a new one under the same name, some pair a builder that still
# ships with the field it no longer sets, and some list the helpers that stay
# and say why -- so "the non-final cells are the removed names" exempts about
# thirty live builders the page recommends, including the ones it recommends
# most.
#
# Being written down also makes the set a snapshot. A derived set grows by
# itself: delete a builder tomorrow and it becomes exempt on this page the same
# day, silently. A listed one does not, and an unlisted removed name fails the
# build with the name to add.
LEDGER_PAGES=(
	docs/builder-contract-release-1.md # release-1 ledger: removed names left, live replacements right
)

# The names LEDGER_PAGES may mention. Validated, not trusted: every name in it
# must be absent from pkg/, so a live builder cannot hide there.
REMOVED_LIST=scripts/doc-api-refs-removed.txt

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

symbols=$(mktemp)    # "<package dir> <name>" per exported declaration
pkgdirs=$(mktemp)    # every package directory under pkg/, exported symbols or not
external=$(mktemp)   # EXTERNAL, one name per line
referenced=$(mktemp) # "<page>:<line>:<reference>" per builder-shaped reference
removed=$(mktemp)    # names a LEDGER_PAGES table lists as removed, one per line
selftest_dir=
# shellcheck disable=SC2064 # expand now: the paths must survive the function that set them
trap 'rm -rf "$symbols" "$pkgdirs" "$external" "$referenced" "$removed" ${selftest_dir:+"$selftest_dir"}' EXIT

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
			# Each form must be a complete HTML comment. Without the closing
			# `-->` in the pattern, `<!-- doc-api-refs:ignore-start reason`
			# with the terminator forgotten still opens a fence -- and since
			# markdown renders an unterminated comment by swallowing the rest
			# of the page, the malformed line and the suppression it grants
			# are both invisible in the built site.
			ostart = match($0, /<!-- doc-api-refs:ignore-start [A-Za-z0-9][^>]*-->/)
			opens = (ostart > 0)
			cstart = match($0, /<!-- doc-api-refs:ignore-end *-->/)
			closes = (cstart > 0)
			# Both on one line is a self-contained fence: it changes no state.
			# Only in that order, though -- a close followed by an open is not a
			# fence, and treating it as one skips the line it is written on,
			# which is where the stale reference sits.
			if (opens && closes) {
				if (ostart < cstart) next
				print "reversed doc-api-refs fence in " FILENAME " line " FNR > "/dev/stderr"
				rc = 1
				next
			}
			if (opens) {
				# A second opener inside an open fence is not a wider fence:
				# the first `ignore-end` would close both, so two opens and
				# one close leave the run green with the rest of the passage
				# suppressed and nothing said about the fence still open.
				# Depth-counting would make that legal; there is no passage
				# this file needs it for, so it is an error instead.
				if (skip) {
					print "nested doc-api-refs:ignore-start in " FILENAME " line " FNR > "/dev/stderr"
					rc = 1
					next
				}
				skip = 1
				next
			}
			if (closes) {
				# A close with nothing open still swallows its own line, so
				# `CreateGone` <!-- doc-api-refs:ignore-end --> would suppress a
				# reference through a marker documented as suppressing nothing.
				if (!skip) {
					print "unmatched doc-api-refs:ignore-end in " FILENAME " line " FNR > "/dev/stderr"
					rc = 1
				}
				skip = 0
				next
			}
			if ($0 ~ /<!-- doc-api-refs:ignore [A-Za-z0-9][^>]*-->/) next
			print "unrecognised doc-api-refs marker in " FILENAME " line " FNR > "/dev/stderr"
			rc = 1
			next
		}
		skip { next }
		# A Go file contributes its comments and nothing else. Code that calls a
		# removed function does not compile, so the compiler already checks it;
		# a doc comment naming one is exactly as stale as a Markdown page naming
		# one, and pkg.go.dev publishes it just as widely.
		# Both comment forms: pkg/stack/layout/doc.go is a `/* ... */` block, and
		# a package that documents a builder that way is published exactly like
		# one that uses `//`. A block is recognised only when it opens the line,
		# which is what a doc comment does and what a `/*` inside a string
		# literal does not.
		FILENAME ~ /\.go$/ {
			if (FNR == 1) inblock = 0
			if (!inblock && $0 ~ /^[ \t]*\/\*/) inblock = 1
			if (inblock) {
				if ($0 ~ /\*\//) inblock = 0
			} else if ($0 !~ /^[ \t]*\/\//) next
		}
		{
			line = $0
			offset = 0
			# The underscore is in the class on purpose: the declaration extractor
			# accepts it, so without it CreateDeployment_Gone would be truncated to
			# the CreateDeployment that does exist and resolve. The `\[` alternative
			# catches the generic constructor, spelled `kubernetes.Create[T]` and so
			# invisible to a pattern that demands an upper-case letter after Create;
			# the bracket is dropped again below, leaving the declared name `Create`.
			while (match(line, /(Create|Set|Add)([A-Z][A-Za-z0-9_]*|\[)/)) {
				abs = RSTART + offset
				# Reject a match that continues an identifier (foo.MySetName).
				if (abs == 1 || substr($0, abs - 1, 1) !~ /[A-Za-z0-9_]/) {
					ref = substr(line, RSTART, RLENGTH)
					generic = sub(/\[$/, "", ref)
					# Keep the package selector when there is one: resolution is
					# package-aware, and `fluxcd.CreateX` must not be answered by
					# an X that only another package declares. A selector that is
					# a variable rather than a package (engine.CreateLayout…) is
					# carried too and simply names no package at resolution time.
					qual = ""
					if (abs > 1 && substr($0, abs - 1, 1) == ".") {
						j = abs - 2
						while (j >= 1 && substr($0, j, 1) ~ /[A-Za-z0-9_]/) j--
						qual = substr($0, j + 1, abs - 2 - j)
					}
					# The bracket form counts only when a package selector spells
					# it out, which is how kure documents it. Bare `Set[` and
					# `Add[` are type syntax in every other language a review page
					# might quote -- Python `ClassVar[Set[T]]` matched here before
					# this condition existed.
					if (!generic || qual != "") {
						if (qual != "") ref = qual "." ref
						print FILENAME ":" FNR ":" ref
					}
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
		Qualified `pkgname.CreateGone` keeps its selector; a bare .CreateReal does not.
		The generic `kube.Create[T]` reduces to the declared name Create.
		Python `ClassVar[Set[PatchType]]` is type syntax and names no builder.
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
	cat >"$d/orphan-end.md" <<-'EOF'
		`CreateGone` <!-- doc-api-refs:ignore-end -->
	EOF
	# The opener has a reason and a well-formed keyword and is still not a
	# comment: a pattern that stops at the reason opens a fence here, and the
	# properly-formed close on line 3 tidies up after it, so the whole passage
	# is suppressed and the run exits 0.
	# Both markers, wrong order: not a fence, and the line it is written on is
	# the one carrying the stale name.
	cat >"$d/reversed-fence.md" <<-'EOF'
		`CreateGone` <!-- doc-api-refs:ignore-end --> <!-- doc-api-refs:ignore-start reason -->
	EOF
	cat >"$d/unterminated-ignore.md" <<-'EOF'
		`CreateGone` <!-- doc-api-refs:ignore reason
	EOF
	cat >"$d/unterminated-start.md" <<-'EOF'
		<!-- doc-api-refs:ignore-start reason
		`CreateGone`
		<!-- doc-api-refs:ignore-end -->
	EOF

	# Two opens and one close. The close matches the inner opener, so the file
	# ends with a fence still open and no reference reported -- clean, green and
	# wrong. Only the nested opener makes it visible.
	cat >"$d/nested-fence.md" <<-'EOF'
		<!-- doc-api-refs:ignore-start outer -->
		<!-- doc-api-refs:ignore-start inner -->
		`CreateGone`
		<!-- doc-api-refs:ignore-end -->
	EOF

	# The package-scoping tree: CreateLayoutWithResources survives in argocd and
	# has been removed from fluxcd, which is the shape a name-only symbol set
	# cannot see. qualified.md names both packages explicitly and must split.
	# The two package READMEs name the method the way this repository really does
	# -- through a selector that is a variable, indistinguishable from an import
	# alias -- and must both resolve: they pin the decision recorded above
	# report_unresolved, so re-adding a page-package rule breaks the self-test
	# instead of quietly failing correct pages.
	mkdir -p "$d/docs" "$d/pkg/stack/fluxcd" "$d/pkg/stack/argocd" "$d/pkg/kubernetes/fluxcd"
	cat >"$d/docs/qualified.md" <<-'EOF'
		Both `argocd.CreateLayoutWithResources` and `fluxcd.CreateLayoutWithResources`.
	EOF
	# Two packages are called fluxcd. This page is one of them, so its own
	# `fluxcd.` selector means itself: CreateOwn is declared here and resolves,
	# CreateElsewhere is declared in the other fluxcd and must not.
	cat >"$d/pkg/kubernetes/fluxcd/README.md" <<-'EOF'
		`fluxcd.CreateOwn` and `fluxcd.CreateElsewhere`.
	EOF
	cat >"$d/pkg/stack/fluxcd/README.md" <<-'EOF'
		`engine.CreateLayoutWithResources(cluster, rules)` builds the layout.
	EOF
	cat >"$d/pkg/stack/argocd/README.md" <<-'EOF'
		`engine.CreateLayoutWithResources(cluster, rules)` builds the layout.
	EOF

	# A Go file: the doc comment is checked, the code below it is not. Both name
	# CreateGone, so a filter that stopped at the comment boundary in either
	# direction shows up as a count.
	cat >"$d/pkg/stack/fluxcd/doc.go" <<-'EOF'
		// Package fluxcd builds things.
		//
		// CreateGone is advertised here and does not exist.
		package fluxcd

		func caller() { CreateGone() }
	EOF

	# The block form, and the line after it: the closing */ must end the
	# comment, or every line of the file counts as documentation.
	cat >"$d/pkg/stack/fluxcd/block.go" <<-'EOF'
		/*
		Package fluxcd builds things.

		CreateBlockGone is advertised here and does not exist.
		*/
		package fluxcd

		func other() { CreateGone() }
	EOF

	# A ledger page: removed names on the left, live replacements on the right,
	# both on the same line, and a three-column row that puts the removed names
	# in the middle -- so no cell position separates the two halves. Row 5 is
	# the rot the rule still has to catch: a replacement that no longer exists,
	# named both in its cell and in the page's prose.
	cat >"$d/ledger.md" <<-'EOF'
		| Removed | Replacement |
		|---|---|
		| `SetGoneThing(obj, v)` | `AddReal(obj, v)` |
		| `pkg` | `AddGoneMiddle(obj)` | `CreateReal(obj)` |
		| `SetAlsoGone(obj, v)` | `CreateGone(obj, v)` |
		Prose naming `SetGoneThing` and `AddGoneMiddle` is exempt; prose naming
		`CreateGone` is not.
	EOF

	# The list the page is checked against. `CreateReal` is deliberately absent
	# even though a table row names it: it is the live replacement, and a cell
	# position must not be able to smuggle it in.
	cat >"$d/removed.txt" <<-'EOF'
		# removed by the fixture release
		SetGoneThing
		AddGoneMiddle

		SetAlsoGone # trailing comment
	EOF

	# The same removed name on a page that is not a ledger: the exemption is
	# scoped to the ledger pages, not granted to the whole tree.
	cat >"$d/notledger.md" <<-'EOF'
		Use `SetGoneThing` for this.
	EOF

	local got
	# Line 3 pins the offset arithmetic: a rejected match between two accepted
	# ones must not consume the rest of the line. Line 4 pins the underscore,
	# line 5 the selector capture (and that a dot with no identifier before it
	# yields none), line 6 the generic bracket form, line 7 that the bracket form
	# is ignored without one -- it emits nothing at all.
	local want='plain.md:1:CreateGone
plain.md:1:CreateReal
plain.md:3:CreateReal
plain.md:3:CreateGone
plain.md:3:AddReal
plain.md:4:CreateDeployment_Gone
plain.md:4:CreateDeployment
plain.md:5:pkgname.CreateGone
plain.md:5:CreateReal
plain.md:6:kube.Create
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

	# A close with nothing open swallows its own line, so accepting it silently
	# would let the marker documented as suppressing nothing suppress something.
	if extract_refs "$d/orphan-end.md" >/dev/null 2>&1; then
		printf 'self-test: an unmatched ignore-end was accepted instead of reported\n' >&2
		failures=$((failures + 1))
	fi

	if extract_refs "$d/reversed-fence.md" >/dev/null 2>&1; then
		printf 'self-test: a reversed one-line fence was accepted instead of reported\n' >&2
		failures=$((failures + 1))
	fi

	# The single-line form has the same obligation, and no fence to close it: a
	# terminator-less bare ignore still swallows its own line.
	if extract_refs "$d/unterminated-ignore.md" >/dev/null 2>&1; then
		printf 'self-test: an unterminated ignore was accepted instead of reported\n' >&2
		failures=$((failures + 1))
	fi

	# Nothing about the CreateGone between these two markers may reach the
	# caller, and the run must fail: the opener is not a complete comment.
	if extract_refs "$d/unterminated-start.md" >/dev/null 2>&1; then
		printf 'self-test: an unterminated ignore-start was accepted instead of reported\n' >&2
		failures=$((failures + 1))
	fi

	if extract_refs "$d/nested-fence.md" >/dev/null 2>&1; then
		printf 'self-test: a nested ignore-start was accepted instead of reported\n' >&2
		failures=$((failures + 1))
	fi

	# A name absent from the symbol set must be reported; one present must not.
	# CreateDeployment is in the set and CreateDeployment_Gone is not, so the
	# suffix must survive extraction or the fourth line resolves wrongly.
	cat >"$symbols" <<-'EOF'
		pkg/kubernetes Create
		pkg/kubernetes/fluxcd CreateOwn
		pkg/stack/argocd CreateLayoutWithResources
		pkg/stack/fluxcd AddReal
		pkg/stack/fluxcd CreateDeployment
		pkg/stack/fluxcd CreateElsewhere
		pkg/stack/fluxcd CreateReal
	EOF
	printf 'pkg/kubernetes\npkg/kubernetes/fluxcd\npkg/stack/argocd\npkg/stack/fluxcd\n' >"$pkgdirs"
	printf 'AddCommand\n' >"$external"

	extract_refs "$d/plain.md" | sort -u >"$referenced"
	local unresolved
	# `pkgname` and `kube` name no package, so both fall back to the whole tree:
	# CreateGone is nowhere in it, Create is.
	local want_unresolved='plain.md:1:CreateGone
plain.md:3:CreateGone
plain.md:4:CreateDeployment_Gone
plain.md:5:pkgname.CreateGone'
	unresolved=$(report_unresolved | sed "s#^$d/##")
	if [ "$unresolved" != "$want_unresolved" ]; then
		printf 'self-test: resolution mismatch\nwant:\n%s\ngot:\n%s\n' "$want_unresolved" "$unresolved" >&2
		failures=$((failures + 1))
	fi

	# The same name on three pages: the spelling that names the package it was
	# removed from is the only one that fails. A flat name set passes all three.
	# The fourth page pins the base-name collision -- a selector answered by the
	# page's own package rather than by whichever package shares its name.
	extract_refs "$d/docs/qualified.md" "$d/pkg/kubernetes/fluxcd/README.md" \
		"$d/pkg/stack/fluxcd/README.md" "$d/pkg/stack/argocd/README.md" |
		sort -u >"$referenced"
	local want_scoped='docs/qualified.md:1:fluxcd.CreateLayoutWithResources
pkg/kubernetes/fluxcd/README.md:1:fluxcd.CreateElsewhere'
	unresolved=$(report_unresolved | sed "s#^$d/##")
	if [ "$unresolved" != "$want_scoped" ]; then
		printf 'self-test: package-scope mismatch\nwant:\n%s\ngot:\n%s\n' "$want_scoped" "$unresolved" >&2
		failures=$((failures + 1))
	fi

	local want_go='pkg/stack/fluxcd/block.go:4:CreateBlockGone
pkg/stack/fluxcd/doc.go:3:CreateGone'
	got=$(extract_refs "$d/pkg/stack/fluxcd/block.go" "$d/pkg/stack/fluxcd/doc.go" |
		sed "s#^$d/##")
	if [ "$got" != "$want_go" ]; then
		printf 'self-test: go-comment extraction mismatch\nwant:\n%s\ngot:\n%s\n' \
			"$want_go" "$got" >&2
		failures=$((failures + 1))
	fi

	# The list is a plain name per line, with `#` comments and blank lines to
	# keep it readable, and it is sorted so a diff of it reads as a diff of the
	# removals. An inline comment is stripped, not taken as part of the name.
	local want_removed='AddGoneMiddle
SetAlsoGone
SetGoneThing'
	got=$(collect_removed "$d/removed.txt")
	if [ "$got" != "$want_removed" ]; then
		printf 'self-test: removed-list mismatch\nwant:\n%s\ngot:\n%s\n' \
			"$want_removed" "$got" >&2
		failures=$((failures + 1))
	fi

	# The ledger rule: a listed name is exempt anywhere on a ledger page, prose
	# included, and nowhere else. Every other name on the page resolves like any
	# other reference, in a table cell (line 5) and in prose (line 7) alike --
	# including `CreateReal`, which the page recommends and the list omits.
	LEDGER_LIST="$d/ledger.md"
	collect_removed "$d/removed.txt" >"$removed"
	extract_refs "$d/ledger.md" "$d/notledger.md" | sort -u >"$referenced"
	local want_ledger='ledger.md:5:CreateGone
ledger.md:7:CreateGone
notledger.md:1:SetGoneThing'
	unresolved=$(report_unresolved | sed "s#^$d/##")
	if [ "$unresolved" != "$want_ledger" ]; then
		printf 'self-test: ledger resolution mismatch\nwant:\n%s\ngot:\n%s\n' \
			"$want_ledger" "$unresolved" >&2
		failures=$((failures + 1))
	fi

	# A live name in the list is refused. It would silence the ledger's own
	# recommendation of that name, so it fails the run rather than being
	# dropped: only a human knows whether the name came back or never went.
	printf 'CreateReal\n' >>"$removed"
	if check_removed_are_gone 2>/dev/null; then
		printf 'self-test: a live name in the removed list was accepted\n' >&2
		failures=$((failures + 1))
	fi
	LEDGER_LIST=
	: >"$removed"

	if [ "$failures" -ne 0 ]; then
		printf 'check-doc-api-refs --self-test: %s failure(s)\n' "$failures" >&2
		return 1
	fi
	printf 'check-doc-api-refs --self-test: ok\n'
}

# The removed-name list: one name per line, with `#` comments and blank lines
# allowed so the file can say which release each block belongs to.
collect_removed() {
	sed -e 's/#.*//' -e 's/^[ \t]*//' -e 's/[ \t]*$//' -e '/^$/d' "$@" | sort -u
}

# Nothing in the list may be a name pkg/ still exports. A live name in there is
# not a harmless spare entry: it silences the ledger's own recommendation of
# that name, which is the one thing this check is here to catch. It fails the
# run rather than being dropped quietly, because the list and the tree
# disagreeing means one of them is wrong and only a human knows which.
check_removed_are_gone() {
	local live
	# Keyed on FILENAME rather than NR == FNR, for the same reason
	# report_unresolved is: the file a record came from is the thing being
	# tested, so say so. The two forms agree here -- an empty $symbols makes
	# NR == FNR span both files, but with no symbols there is nothing live to
	# report either way, and the run refuses an empty $symbols upstream anyway.
	live=$(awk -v symfile="$symbols" \
		'FILENAME == symfile { name[$2] = 1; next } $0 in name' \
		"$symbols" "$removed")
	[ -n "$live" ] || return 0
	printf 'check-doc-api-refs: %s lists names that pkg/ still exports:\n\n' "$REMOVED_LIST" >&2
	printf '%s\n' "$live" | sed 's/^/  /' >&2
	printf '\n' >&2
	printf 'Either the name came back, in which case drop it from the list, or it\n' >&2
	printf 'was never removed and the entry is wrong. While it is listed, the ledger\n' >&2
	printf 'may name it freely and a later deletion of it goes unnoticed.\n' >&2
	return 1
}

# Every row in $referenced that does not resolve, given where the row was written.
# Exits 0 when it printed at least one row, so it reads as
# `if unresolved=$(report_unresolved)`.
#
# Three rules:
#
#   qualified `pkg.Name` on a page that lives in a package called pkg
#       -> Name must be declared in that exact package
#   qualified `pkg.Name` anywhere else
#       -> Name must be declared in a package whose base name is pkg
#   anything else
#       -> Name must be declared somewhere under pkg/
#
# The qualified rules are what makes a removal in one package fail a page that
# names that package, even while a same-named declaration survives elsewhere.
# Base names are not unique -- pkg/kubernetes/fluxcd and pkg/stack/fluxcd are
# both `fluxcd`, and 60 references in this tree use that selector -- so the union
# of same-named packages answers a reference that only one of them should. The
# page's own location is the only disambiguation the text offers, and the first
# rule takes it. It is not complete: nothing distinguishes the two for a page
# under docs/ or site/content/, so a name removed from one while the other keeps
# it still resolves there. The two packages currently share no builder-shaped
# name at all (184 in pkg/kubernetes/fluxcd, 2 in pkg/stack/fluxcd, no overlap),
# so no reference in the tree is ambiguous today.
#
# It is deliberately not extended to unqualified names on a package's own README.
# That rule is not decidable from the text and was measured against this tree: a
# package README legitimately names a neighbour's builder both in prose
# (pkg/stack/fluxcd/README.md's SetGitRepositoryReference, declared in
# pkg/kubernetes/fluxcd) and inside its own Go examples through an import alias
# (the same file's pubfluxcd.CreateGitRepository), and an alias is spelled exactly
# like the variable receiver in engine.CreateLayoutWithResources two code blocks
# later. Failing on those would buy one more catchable removal and cost three
# suppressions on pages that are correct, which is a worse check.
report_unresolved() {
	awk -v symfile="$symbols" -v dirfile="$pkgdirs" -v extfile="$external" \
		-v remfile="$removed" -v ledgers="${LEDGER_LIST:-}" '
		BEGIN {
			nl = split(ledgers, l, " ")
			for (li = 1; li <= nl; li++) isledger[l[li]] = 1
		}
		FILENAME == remfile { gone[$0] = 1; next }
		FILENAME == symfile {
			names[$2] = 1
			decl[$1 " " $2] = 1
			base = $1
			sub(/.*\//, "", base)
			bybase[base " " $2] = 1
			next
		}
		FILENAME == dirfile {
			ispkg[$0] = 1
			base = $0
			sub(/.*\//, "", base)
			pkgbase[base] = 1
			next
		}
		FILENAME == extfile { ext[$0] = 1; next }
		{
			n = split($0, p, ":")
			if (n < 3) next
			ref = p[n]
			file = p[1]
			for (i = 2; i <= n - 2; i++) file = file ":" p[i]

			qual = ""
			name = ref
			i = index(ref, ".")
			if (i > 0) { qual = substr(ref, 1, i - 1); name = substr(ref, i + 1) }

			if (name in ext) next
			# A ledger page may name what it says it removed, anywhere on the
			# page: its own tables are the list of what that is. Everything else
			# it names, prose included, is a live recommendation and resolves.
			if ((file in isledger) && (name in gone)) next
			if (qual != "" && (qual in pkgbase)) {
				# Base names are not unique: pkg/kubernetes/fluxcd and
				# pkg/stack/fluxcd are both spelled fluxcd, so a bare base-name
				# match lets either answer. A page that lives in a package of
				# that name is the one case the text settles -- it means itself.
				pagedir = file
				sub(/\/[^\/]*$/, "", pagedir)
				sub(/^.*\/pkg\//, "pkg/", pagedir)
				pbase = pagedir
				sub(/.*\//, "", pbase)
				if (pbase == qual && (pagedir in ispkg)) {
					if ((pagedir " " name) in decl) next
					print
					found = 1
					next
				}
				if ((qual " " name) in bybase) next
				print
				found = 1
				next
			}
			if (name in names) next
			print
			found = 1
		}
		END { exit !found }
	' "$symbols" "$pkgdirs" "$external" "$removed" "$referenced"
}

if [ "${1:-}" = "--self-test" ]; then
	self_test
	exit
fi

# Exported functions and methods declared in the public tree, each paired with
# the package directory that declares it. Test files do not count: a symbol that
# only exists in a _test.go file is not importable, and no page may cite one.
# Neither does anything under an internal/ directory, for the same reason one
# step further out -- pkg/kubernetes/internal is closed to consumers, so a
# declaration there must never be what makes a public page resolve. It holds no
# builder-shaped name today; the exclusion is what keeps that from mattering.
#
# -H is not optional. Without it grep omits the file name whenever xargs hands it
# a single path, which happens for the last batch of a long list -- the rows would
# then lose the package half of the pair for an arbitrary tail of the tree.
go_files=$(find pkg -name '*.go' ! -name '*_test.go' ! -path '*/internal/*' -type f)
printf '%s\n' "$go_files" | sed 's#/[^/]*$##' | sort -u >"$pkgdirs"
printf '%s\n' "$go_files" | tr '\n' '\0' |
	xargs -0 grep -HoE '^func (\([^)]*\) )?[A-Z][A-Za-z0-9_]*' |
	sed -E 's#/[^/]*\.go:func (\([^)]*\) )?# #' |
	sort -u >"$symbols"

printf '%s\n' "${EXTERNAL[@]}" >"$external"

for _list in go_files symbols pkgdirs external; do
	case "$_list" in
	go_files) [ -n "$go_files" ] && continue ;;
	*) [ -s "${!_list}" ] && continue ;;
	esac
	printf 'check-doc-api-refs: %s is empty -- refusing to resolve against nothing\n' \
		"$_list" >&2
	exit 1
done
unset _list

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
#
# examples/ is a find root of its own rather than left to the map: the map mounts
# one example README, and the rest are advertised as runnable, so a removed
# symbol surviving in examples/getting-started is exactly as misleading as one on
# the site.
#
# The repository-root Markdown is enumerated as a directory rather than as
# README.md alone: AGENTS.md is read by every agent that touches this tree and
# carries worked examples, DEVELOPMENT.md the same for a human, and neither is
# less live than the site for being at the root.
docs_pages=$(find . -maxdepth 1 -name '*.md' -type f
	find docs examples site/content -name '*.md' -type f)
# The package READMEs are the API-reference pages the site mounts, so they are
# exactly the pages a stale call hurts most -- but the enumeration is every
# Markdown file under pkg/, not just those. pkg/stack/DESIGN.md describes the
# shipped design and pkg/stack/STATUS.md says in its first line that it reflects
# current implementation state; a builder named in either is as live a claim as
# one named in a README, and sits closer to the code than the site does. A page
# under pkg/ that stops being current belongs in EXCLUDED_PAGES by name, with
# its reason, like every other exemption here.
pkg_pages=$(find pkg -name '*.md' -type f)
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
done < <(printf '%s\n%s\n%s\n%s\n' "$docs_pages" "$pkg_pages" "$map_pages" "$go_files" |
	sed 's#^\./##' | sort -u)

# The ledger pages arrive through the same enumeration as any other page, so
# they are already in `pages`; what they get is the name-level exemption from
# REMOVED_LIST. A ledger page that stopped existing would drop out of the
# enumeration silently, so require each one.
for _ledger in "${LEDGER_PAGES[@]}"; do
	if [ ! -f "$_ledger" ]; then
		printf 'check-doc-api-refs: LEDGER_PAGES names %s, which does not exist\n' \
			"$_ledger" >&2
		exit 1
	fi
done
unset _ledger
LEDGER_LIST="${LEDGER_PAGES[*]}"

if [ ! -f "$REMOVED_LIST" ]; then
	printf 'check-doc-api-refs: %s is missing -- without it the ledgers cannot be checked\n' \
		"$REMOVED_LIST" >&2
	exit 1
fi
collect_removed "$REMOVED_LIST" >"$removed"
check_removed_are_gone || exit 1

if [ "${1:-}" = "--list" ]; then
	printf 'declarations: %s\n' "$(wc -l <"$symbols")"
	printf 'packages:     %s\n' "$(wc -l <"$pkgdirs")"
	printf 'pages:        %s (Markdown pages plus public Go files)\n' "${#pages[@]}"
	printf 'removed:      %s (names the ledgers may mention)\n' "$(wc -l <"$removed")"
	exit 0
fi

extract_refs "${pages[@]}" | sort -u >"$referenced"

if unresolved=$(report_unresolved); then
	printf 'Documentation names builder functions that pkg/ does not export:\n\n' >&2
	while IFS= read -r row; do printf '  %s\n' "$row" >&2; done <<<"$unresolved"
	cat >&2 <<-'EOF'

		Each row is a page naming a function that is not in the public API where the
		page says it is. A row written `pkg.Name` was resolved in the package that
		selector names, so it can appear because the name lives in another package
		rather than because it was deleted. Fix the page -- the replacement
		expression for every function the builder-contract epic removed is in
		docs/builder-contract-release-1.md. If the reference is deliberate (a dated
		record, or a third-party API that happens to match the Create/Set/Add shape),
		add the page to EXCLUDED_PAGES or the name to EXTERNAL in
		scripts/check-doc-api-refs.sh, with the reason.
	EOF
	exit 1
fi

printf 'check-doc-api-refs: %s pages and Go files, %s builder references, all resolved.\n' \
	"${#pages[@]}" "$(wc -l <"$referenced")"
