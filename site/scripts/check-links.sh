#!/usr/bin/env bash
# check-links.sh — Layer 1 of the documentation-sync standard: verify that every
# internal link resolves in the *rendered* site.
#
# Builds the site with root-relative URLs (so links resolve offline against the
# output tree) and runs lychee with --root-dir. External links are not checked
# here (--offline); flaky external checks belong in a separate non-blocking job.
#
# Usage: bash scripts/check-links.sh [KURE_ROOT]
# Requires: hugo, yq, lychee.

set -euo pipefail

SITE_DIR="$(cd "$(dirname "$0")/.." && pwd)"
KURE_ROOT="${1:-$(cd "$SITE_DIR/.." && pwd)}"
LC_DIR="$SITE_DIR/.linkcheck"

for tool in hugo yq lychee; do
  command -v "$tool" >/dev/null 2>&1 || { echo "ERROR: $tool is required" >&2; exit 1; }
done

# Render with root-relative URLs into a throwaway dir. The production baseURL
# (and version prefix) is irrelevant to internal link integrity; a root-relative
# build lets lychee resolve every link offline against the output.
bash "$SITE_DIR/scripts/inject-frontmatter.sh" "$KURE_ROOT" >/dev/null
rm -rf "$LC_DIR"
( cd "$SITE_DIR" && hugo --baseURL "/" --destination "$LC_DIR" --quiet )

echo "=== lychee (internal links, offline) ==="
lychee --offline --root-dir "$LC_DIR" --no-progress "$LC_DIR/**/*.html"
echo "check-links: OK (internal links resolve)"
