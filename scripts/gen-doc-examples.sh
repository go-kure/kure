#!/usr/bin/env bash
# Generate the Go blocks of the documentation pages from compiled Example
# functions, or check that they are generated.
#
#   scripts/gen-doc-examples.sh              rewrite the generated blocks
#   scripts/gen-doc-examples.sh --check      fail on drift, an unvouched block
#                                            or a page missing from the list
#   scripts/gen-doc-examples.sh --self-test  run the generator's own tests
#
# A page marks a generated block with
#
#   <!-- doc-example: <package dir> <Example function> -->
#   ...
#   <!-- doc-example:end -->
#
# and a Go block that cannot be generated -- a type declaration, a sketch with
# placeholders, a migration ledger -- with `<!-- doc-example:excerpt <reason> -->`
# directly above its fence. On an enabled page every ```go block is one or the
# other, so every snippet a reader copies either compiles in `go test` or says
# why it is not meant to. scripts/docexamples has the marker rules and the
# body format.
#
# Every page is enabled: ENABLED_PAGES is every tracked Markdown file except
# EXEMPT_PAGES, and --check fails when the two drift apart, so a new page with
# a Go block cannot escape by not being listed. That is the Markdown subset of
# the pages scripts/check-doc-api-refs.sh checks, less its EXCLUDED_PAGES, and
# EXEMPT_PAGES repeats those exclusions for the same reason: a dated record or
# a proposal is true as of its date and is not rewritten to compile.
#
# To add an example: write an Example function in a _test.go file of the
# package it demonstrates (package <name>_test), put the two markers where the
# block goes, and run this script. A new page goes into ENABLED_PAGES, or into
# EXEMPT_PAGES with its reason.
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

# The pages whose every Go block must be generated or marked as an excerpt.
ENABLED_PAGES=(
	.claude/CLAUDE.md
	AGENTS.md
	DEVELOPMENT.md
	OAM-helm-alternative.md
	README.md
	docs/ARCHITECTURE.md
	docs/api-tables.md
	docs/builder-contract-release-1.md
	docs/builder-contract-release-2.md
	docs/compatibility.md
	docs/dependency-updates.md
	docs/github-workflows.md
	docs/oci-layout.md
	docs/puzl-cloud-kubesdk-review.md
	docs/quickstart.md
	examples/README.md
	examples/demo/README.md
	examples/demo/clusters/umbrella/README.md
	examples/getting-started/README.md
	examples/kurel/frigate/README.md
	examples/patches/README.md
	pkg/errors/README.md
	pkg/gvk/README.md
	pkg/io/README.md
	pkg/kubernetes/README.md
	pkg/kubernetes/certmanager/README.md
	pkg/kubernetes/cilium/README.md
	pkg/kubernetes/cnpg/README.md
	pkg/kubernetes/externalsecrets/README.md
	pkg/kubernetes/fluxcd/README.md
	pkg/kubernetes/metallb/README.md
	pkg/kubernetes/prometheus/README.md
	pkg/kubernetes/volsync/README.md
	pkg/logger/README.md
	pkg/manifest/README.md
	pkg/stack/DESIGN.md
	pkg/stack/README.md
	pkg/stack/STATUS.md
	pkg/stack/argocd/README.md
	pkg/stack/fluxcd/README.md
	pkg/stack/helm/README.md
	pkg/stack/layout/README.md
	pkg/versions/README.md
	site/content/_index.md
	site/content/api-reference/_index.md
	site/content/changelog/_index.md
	site/content/concepts/_index.md
	site/content/concepts/builder-contract.md
	site/content/concepts/design-philosophy.md
	site/content/concepts/domain-model.md
	site/content/contributing/_index.md
	site/content/examples/_index.md
	site/content/getting-started/_index.md
	site/content/guides/_index.md
	site/content/guides/flux-workflow.md
	site/content/guides/library-usage.md
	site/content/guides/patching.md
)

# Markdown that is not a page of this check; a trailing / exempts a tree.
EXEMPT_PAGES=(
	.github/                           # forge templates, not documentation
	docs/history/                      # dated design records; true as of their date
	docs/reviews/                      # dated review records; ditto
	docs/ux-design.md                  # proposed UX, not shipped API
	docs/plugin-architecture-design.md # proposed plugin API, not shipped
	CHANGELOG.md                       # generated from commit subjects
)

# unlisted_pages reads Markdown paths on stdin and prints, as "+ path" or
# "- path", each one that ENABLED_PAGES is missing and each enabled page that
# is not among them. It prints nothing when the sets agree.
unlisted_pages() {
	local page exempt skip
	local -A seen=()
	while IFS= read -r page; do
		skip=0
		for exempt in "${EXEMPT_PAGES[@]}"; do
			case "$page" in "$exempt"*) skip=1 ;; esac
		done
		[ "$skip" -eq 1 ] && continue
		seen[$page]=1
	done
	for page in "${ENABLED_PAGES[@]}"; do
		if [ -z "${seen[$page]:-}" ]; then
			printf -- '- %s\n' "$page"
		fi
		unset "seen[$page]"
	done
	if [ "${#seen[@]}" -gt 0 ]; then
		printf -- '+ %s\n' "${!seen[@]}" | sort
	fi
}

self_test_unlisted_pages() {
	local got want
	got=$(printf '%s\n' "${ENABLED_PAGES[@]}" docs/history/x.md .github/T.md CHANGELOG.md | unlisted_pages)
	if [ -n "$got" ]; then
		printf 'self-test: the listed set plus exempt pages reported %s\n' "$got" >&2
		return 1
	fi
	got=$(printf '%s\n' "${ENABLED_PAGES[@]:1}" docs/new.md docs/history-notes.md | unlisted_pages)
	want=$(printf -- '- %s\n+ docs/history-notes.md\n+ docs/new.md' "${ENABLED_PAGES[0]}")
	if [ "$got" != "$want" ]; then
		printf 'self-test: unlisted_pages\nwant:\n%s\ngot:\n%s\n' "$want" "$got" >&2
		return 1
	fi
	printf 'ok  unlisted_pages\n'
}

export GOWORK="${GOWORK:-off}"
case "${1:-}" in
"") exec go run ./scripts/docexamples generate "${ENABLED_PAGES[@]}" ;;
--check)
	# Tracked files only: an untracked page is local until it is committed,
	# and CI sees it then.
	tracked=$(git ls-files -- '*.md')
	if [ -z "$tracked" ]; then
		printf 'gen-doc-examples: git ls-files listed no Markdown -- refusing to check an empty set\n' >&2
		exit 1
	fi
	drift=$(printf '%s\n' "$tracked" | unlisted_pages)
	if [ -n "$drift" ]; then
		printf 'gen-doc-examples: ENABLED_PAGES differs from the tracked Markdown less EXEMPT_PAGES\n(+ missing from the list, - listed but not tracked):\n%s\n' "$drift" >&2
		exit 1
	fi
	exec go run ./scripts/docexamples check "${ENABLED_PAGES[@]}"
	;;
--self-test)
	self_test_unlisted_pages
	exec go test ./scripts/docexamples
	;;
*)
	printf 'usage: %s [--check|--self-test]\n' "$0" >&2
	exit 2
	;;
esac
