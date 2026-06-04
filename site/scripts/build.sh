#!/usr/bin/env bash
# build.sh — Orchestrate Hugo site build.
#
# Usage: bash scripts/build.sh [KURE_ROOT]
#   Run from site/ directory.

set -euo pipefail

SITE_DIR="$(cd "$(dirname "$0")/.." && pwd)"
KURE_ROOT="${1:-$(cd "$SITE_DIR/.." && pwd)}"

echo "=== Validating docs map (code↔docs sync) ==="
bash "$SITE_DIR/scripts/check-doc-sync.sh" "$KURE_ROOT"

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
