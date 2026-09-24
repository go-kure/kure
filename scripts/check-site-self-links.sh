#!/usr/bin/env bash
# check-site-self-links.sh — fail when a published page links to the docs site
# itself with an absolute URL outside the dev slot.
#
#   scripts/check-site-self-links.sh              # exit 1 and list every offending link
#   scripts/check-site-self-links.sh --self-test  # prove the check can still fail
#   scripts/check-site-self-links.sh --root DIR   # check another tree (used by --self-test)
#
# Why: the site is versioned. A push to main rebuilds only the dev slot
# (<base>/dev/); the release root (<base>) is rebuilt only when a release is
# marked latest (.github/workflows/manage-docs.yml -> deploy-docs.yml). An
# absolute link such as https://www.gokure.dev/kure/concepts/new-page/ therefore
# points at the last release build, and 404s for every page added since. The
# rendered-site link check (check-links, lychee --offline) cannot see this: it
# treats every http(s) URL as external and skips it.
#
# The rule for pages the site publishes:
#   - link to another site page with a version-relative path (/concepts/x/);
#     Hugo resolves it inside whichever version is being built;
#   - where the page is also read outside the site (CHANGELOG.md on GitHub, and
#     cliff.toml, which writes CHANGELOG.md), link to the dev slot:
#     https://www.gokure.dev/kure/dev/concepts/x/.
# Any other absolute URL under the site base fails, including the base itself
# and a pinned release slot (/kure/vX.Y/): no published page should depend on a
# build that newer content never reaches.
#
# The site base comes from site/hugo.toml's baseURL, so the check follows the
# site if it moves. Scheme and host are matched case-insensitively and with or
# without a leading "www.". A URL on the same host outside the base path (the
# org landing page, another project's site) is not this site and passes.
#
# Pages checked: every package README with a mount: block and every extra mount
# in site/docs-map.yaml (the authority for what is published), every file under
# site/content/, and cliff.toml. Needs yq. Pages outside that set (a `mounted:
# false` README, docs/history/, reviews) are not published and are not checked.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODE=check

while [ $# -gt 0 ]; do
	case "$1" in
	--self-test) MODE=self-test ;;
	--root)
		ROOT="${2:?--root needs a directory}"
		shift
		;;
	-h | --help)
		sed -n '2,8p' "$0"
		exit 0
		;;
	*)
		printf 'check-site-self-links: unknown argument %s\n' "$1" >&2
		exit 2
		;;
	esac
	shift
done

# scan_links BASE FILE... — print file:line: url for each offending link.
scan_links() {
	local base="$1"
	shift
	awk -v base="$base" '
	BEGIN {
		# base is scheme://host/path/ ; split it once.
		if (!match(base, /^[A-Za-z]+:\/\/[^\/]+/)) {
			print "check-site-self-links: cannot parse baseURL " base > "/dev/stderr"
			exit 3
		}
		host = tolower(substr(base, RSTART, RLENGTH))
		sub(/^[a-z]+:\/\//, "", host)
		sub(/^www\./, "", host)
		path = substr(base, RLENGTH + 1)          # e.g. /kure/
		sub(/\/+$/, "", path)                       # e.g. /kure
		# Any http(s) URL: stop at whitespace and the characters that end a
		# URL in Markdown, HTML or TOML.
		url_re = "[Hh][Tt][Tt][Pp][Ss]?://[^][[:space:]()<>\"'"'"'`]+"
	}
	{
		line = $0
		while (match(line, url_re)) {
			url = substr(line, RSTART, RLENGTH)
			line = substr(line, RSTART + RLENGTH)
			rest = url
			sub(/^[A-Za-z]+:\/\//, "", rest)
			h = rest; sub(/[\/?#].*$/, "", h)
			h = tolower(h); sub(/^www\./, "", h)
			if (h != host) continue
			p = rest; sub(/^[^\/?#]*/, "", p)          # path + query + fragment
			# Under the site base: exactly the base path, or base path followed
			# by /, ? or #. /kurel/ is another site and passes.
			if (p != path && index(p, path "/") != 1 && index(p, path "?") != 1 && index(p, path "#") != 1) continue
			tail = substr(p, length(path) + 1)
			# The dev slot: base/dev, base/dev/..., base/dev?..., base/dev#...
			if (tail ~ /^\/dev($|[\/?#])/) continue
			printf "%s:%d: %s\n", FILENAME, FNR, url
		}
	}' "$@"
}

# collect_pages — print every page this check covers, one per line, relative
# to ROOT. Fails rather than returning a short list.
collect_pages() {
	local map="site/docs-map.yaml" map_pages content_pages
	[ -f "$map" ] || {
		printf 'check-site-self-links: %s is missing\n' "$map" >&2
		return 1
	}
	command -v yq >/dev/null 2>&1 || {
		printf 'check-site-self-links: yq is required to read %s\n' "$map" >&2
		return 1
	}
	# Each failure is checked explicitly: this function runs on the left of
	# `||`, where set -e does not apply, so an unchecked yq or find failure
	# would shorten the page list instead of stopping the run.
	# Only packages with a mount: block are published; a `mounted: false`
	# README is read on GitHub alone and is out of this check's scope.
	map_pages=$(yq -r '(.packages[] | select(.mount) | .readme), .extra_mounts[].source' "$map") || {
		printf 'check-site-self-links: yq could not read %s\n' "$map" >&2
		return 1
	}
	map_pages=$(printf '%s\n' "$map_pages" | grep -v -x -e '' -e 'null' | sort -u || true)
	if [ -z "$map_pages" ]; then
		printf 'check-site-self-links: %s names no pages -- refusing to check a partial set\n' "$map" >&2
		return 1
	fi
	content_pages=$(find site/content -type f | sort) || {
		printf 'check-site-self-links: cannot list site/content\n' >&2
		return 1
	}
	if [ -z "$content_pages" ]; then
		printf 'check-site-self-links: site/content holds no pages -- refusing to check a partial set\n' >&2
		return 1
	fi
	local page missing=0
	while IFS= read -r page; do
		if [ ! -f "$page" ]; then
			printf 'check-site-self-links: %s names %s, which does not exist\n' "$map" "$page" >&2
			missing=1
		fi
	done <<<"$map_pages"
	[ "$missing" -eq 0 ] || return 1
	[ -f cliff.toml ] || {
		printf 'check-site-self-links: cliff.toml is missing\n' >&2
		return 1
	}
	printf '%s\n' "$map_pages" "$content_pages" cliff.toml
}

run_check() {
	cd "$ROOT"
	local base pages offenders
	base=$(sed -n "s/^baseURL[[:space:]]*=[[:space:]]*['\"]\([^'\"]*\)['\"].*/\1/p" site/hugo.toml 2>/dev/null | head -n 1)
	if [ -z "$base" ]; then
		printf 'check-site-self-links: no baseURL in site/hugo.toml\n' >&2
		return 1
	fi
	pages=$(collect_pages) || return 1
	local -a files
	mapfile -t files <<<"$pages"
	offenders=$(scan_links "$base" "${files[@]}") || {
		printf 'check-site-self-links: link scan failed\n' >&2
		return 1
	}
	if [ -n "$offenders" ]; then
		printf 'check-site-self-links: absolute links to %s outside the dev slot:\n\n' "$base" >&2
		printf '%s\n' "$offenders" | sed 's/^/  /' >&2
		cat >&2 <<EOF

The release root is rebuilt only when a release is marked latest, so a link
into it breaks for every page added since. Use a version-relative path
(/concepts/page/) in site pages, or the dev slot (${base%/}/dev/...) in text
also read outside the site (CHANGELOG.md, cliff.toml).
EOF
		return 1
	fi
	printf 'check-site-self-links: %s pages, no absolute links outside the dev slot.\n' "${#files[@]}"
}

self_test() {
	local tmp failures=0 out rc expected
	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' RETURN
	mkdir -p "$tmp/site/content/concepts" "$tmp/pkg/a" "$tmp/pkg/b" "$tmp/docs/history" "$tmp/stub"
	printf "baseURL = 'https://www.gokure.dev/kure/'\n" >"$tmp/site/hugo.toml"
	cat >"$tmp/site/docs-map.yaml" <<'EOF'
packages:
  - path: pkg/a
    readme: pkg/a/README.md
    mount: {target: api-reference/a.md, title: A}
  - path: pkg/b
    readme: pkg/b/README.md
    mounted: false
extra_mounts:
  - {source: CHANGELOG.md, target: changelog/releases.md}
EOF
	cat >"$tmp/pkg/a/README.md" <<'EOF'
[root page](https://www.gokure.dev/kure/concepts/a/)
[bare host](http://gokure.dev/kure/concepts/b/) and [upper](HTTPS://WWW.GoKure.DEV/kure/c/)
[base](https://www.gokure.dev/kure/) [base no slash](https://www.gokure.dev/kure)
[release slot](https://www.gokure.dev/kure/v0.1/x/) [not dev](https://www.gokure.dev/kure/devtools/)
[query](https://www.gokure.dev/kure?x=1)
[ok dev](https://www.gokure.dev/kure/dev/concepts/a/) [ok dev bare](https://www.gokure.dev/kure/dev)
[ok dev fragment](https://www.gokure.dev/kure/dev#top) [ok relative](/concepts/a/)
[ok other site](https://www.gokure.dev/kurel/) [ok org](https://www.gokure.dev/) [ok elsewhere](https://example.com/kure/x/)
EOF
	cat >"$tmp/CHANGELOG.md" <<'EOF'
> see <a href="https://www.gokure.dev/kure/concepts/c/">notes</a>
> see [dev](https://www.gokure.dev/kure/dev/concepts/c/)
EOF
	printf '[in content](https://www.gokure.dev/kure/api-reference/x/)\n' >"$tmp/site/content/concepts/_index.md"
	printf 'body = """\n> [x](https://www.gokure.dev/kure/concepts/d/)\n"""\n' >"$tmp/cliff.toml"
	printf '[unpublished](https://www.gokure.dev/kure/concepts/z/)\n' >"$tmp/docs/history/old.md"
	printf '[unpublished](https://www.gokure.dev/kure/concepts/y/)\n' >"$tmp/pkg/b/README.md"

	expected='CHANGELOG.md:1: https://www.gokure.dev/kure/concepts/c/
cliff.toml:2: https://www.gokure.dev/kure/concepts/d/
pkg/a/README.md:1: https://www.gokure.dev/kure/concepts/a/
pkg/a/README.md:2: HTTPS://WWW.GoKure.DEV/kure/c/
pkg/a/README.md:2: http://gokure.dev/kure/concepts/b/
pkg/a/README.md:3: https://www.gokure.dev/kure
pkg/a/README.md:3: https://www.gokure.dev/kure/
pkg/a/README.md:4: https://www.gokure.dev/kure/devtools/
pkg/a/README.md:4: https://www.gokure.dev/kure/v0.1/x/
pkg/a/README.md:5: https://www.gokure.dev/kure?x=1
site/content/concepts/_index.md:1: https://www.gokure.dev/kure/api-reference/x/'

	rc=0
	out=$(bash "$0" --root "$tmp" 2>&1) || rc=$?
	if [ "$rc" -ne 1 ]; then
		printf 'self-test: offending tree exited %s, want 1\n%s\n' "$rc" "$out" >&2
		failures=$((failures + 1))
	fi
	local got
	got=$(printf '%s\n' "$out" | sed -n 's/^  //p' | LC_ALL=C sort)
	if [ "$got" != "$(printf '%s\n' "$expected" | LC_ALL=C sort)" ]; then
		printf 'self-test: offender rows differ\n--- want\n%s\n--- got\n%s\n' "$expected" "$got" >&2
		failures=$((failures + 1))
	fi

	# Clean tree: only the allowed forms left.
	printf '[ok](https://www.gokure.dev/kure/dev/concepts/a/) [ok](/concepts/a/)\n' >"$tmp/pkg/a/README.md"
	printf '[ok](https://www.gokure.dev/kure/dev/concepts/c/)\n' >"$tmp/CHANGELOG.md"
	printf '[ok](/api-reference/x/)\n' >"$tmp/site/content/concepts/_index.md"
	printf 'body = "[x](https://www.gokure.dev/kure/dev/concepts/d/)"\n' >"$tmp/cliff.toml"
	rc=0
	out=$(bash "$0" --root "$tmp" 2>&1) || rc=$?
	if [ "$rc" -ne 0 ]; then
		printf 'self-test: clean tree exited %s, want 0\n%s\n' "$rc" "$out" >&2
		failures=$((failures + 1))
	fi

	# A page lister that prints part of its list and then fails must stop the
	# run, not check the part it printed.
	local tool
	for tool in yq find; do
		printf '#!/bin/sh\necho pkg/a/README.md\nexit 1\n' >"$tmp/stub/$tool"
		chmod +x "$tmp/stub/$tool"
		rc=0
		out=$(PATH="$tmp/stub:$PATH" bash "$0" --root "$tmp" 2>&1) || rc=$?
		if [ "$rc" -ne 1 ]; then
			printf 'self-test: %s failing after partial output exited %s, want 1\n%s\n' \
				"$tool" "$rc" "$out" >&2
			failures=$((failures + 1))
		fi
		rm "$tmp/stub/$tool"
	done

	# A mapped page that does not exist must fail, not shrink the page set.
	rm "$tmp/pkg/a/README.md"
	rc=0
	out=$(bash "$0" --root "$tmp" 2>&1) || rc=$?
	if [ "$rc" -ne 1 ] || ! printf '%s' "$out" | grep -q 'which does not exist'; then
		printf 'self-test: missing mapped page exited %s, want 1 naming it\n%s\n' "$rc" "$out" >&2
		failures=$((failures + 1))
	fi

	# An unreadable map must fail, not check an empty page set.
	printf 'packages: [\n' >"$tmp/site/docs-map.yaml"
	rc=0
	out=$(bash "$0" --root "$tmp" 2>&1) || rc=$?
	if [ "$rc" -ne 1 ]; then
		printf 'self-test: unreadable map exited %s, want 1\n%s\n' "$rc" "$out" >&2
		failures=$((failures + 1))
	fi

	if [ "$failures" -gt 0 ]; then
		printf 'check-site-self-links --self-test: %s failure(s)\n' "$failures" >&2
		return 1
	fi
	printf 'check-site-self-links --self-test: ok\n'
}

case "$MODE" in
check) run_check ;;
self-test) self_test ;;
esac
