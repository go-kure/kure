#!/usr/bin/env bash
# build.sh — Orchestrate Hugo site build.
#
# Usage: bash scripts/build.sh [KURE_ROOT]
#   Run from site/ directory.

set -euo pipefail

SITE_DIR="$(cd "$(dirname "$0")/.." && pwd)"
KURE_ROOT="${1:-$(cd "$SITE_DIR/.." && pwd)}"

echo "=== Validating docs map (code↔docs sync) ==="
# Canonical script lives in go-kure/.github; kure no longer vendors a copy.
# Read it from the sibling checkout's origin/main — same approach as the
# site:check mise task (mise.toml), which this duplicates rather than shares
# since mise.toml's inline run: string can't source a bash file.
git -C "$KURE_ROOT/../dot-github" fetch -q origin main
_docsync_tmp="$(mktemp -d)"
trap 'rm -rf "$_docsync_tmp"' EXIT
git -C "$KURE_ROOT/../dot-github" archive origin/main scripts | tar -x -C "$_docsync_tmp"
bash "$_docsync_tmp/scripts/check-doc-sync.sh" "$KURE_ROOT"

echo ""
echo "=== Injecting front matter ==="
bash "$SITE_DIR/scripts/inject-frontmatter.sh" "$KURE_ROOT"

echo ""
echo "=== Building Hugo site ==="
cd "$SITE_DIR"
hugo --minify

echo ""
echo "=== Build complete ==="
echo "Site output in $SITE_DIR/public/"
