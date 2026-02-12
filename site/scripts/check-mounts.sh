#!/usr/bin/env bash
# check-mounts.sh — Verify every file referenced by inject-frontmatter.sh exists.
#
# Exits non-zero if any mounted file is missing.
#
# Usage: bash scripts/check-mounts.sh [KURE_ROOT]

set -euo pipefail

SITE_DIR="$(cd "$(dirname "$0")/.." && pwd)"
KURE_ROOT="${1:-$(cd "$SITE_DIR/.." && pwd)}"

MOUNTED_FILES=(
  "CHANGELOG.md"
  "DEVELOPMENT.md"
  "docs/ARCHITECTURE.md"
  "docs/quickstart.md"
  "docs/compatibility.md"
  "docs/github-workflows.md"
  "pkg/stack/README.md"
  "pkg/stack/fluxcd/README.md"
  "pkg/stack/generators/README.md"
  "pkg/stack/layout/README.md"
  "pkg/launcher/README.md"
  "pkg/patch/README.md"
  "pkg/io/README.md"
  "pkg/errors/README.md"
  "pkg/cli/README.md"
  "pkg/kubernetes/fluxcd/README.md"
  "pkg/logger/README.md"
  "examples/patches/README.md"
  "examples/generators/README.md"
  "examples/kurel/frigate/README.md"
  "examples/validation/README.md"
)

missing=0

for file in "${MOUNTED_FILES[@]}"; do
  if [[ ! -f "$KURE_ROOT/$file" ]]; then
    echo "ERROR: mounted file not found: $file" >&2
    ((missing++))
  fi
done

if [[ $missing -gt 0 ]]; then
  echo "FATAL: $missing mounted file(s) missing. Fix mounts or source files." >&2
  exit 1
fi

echo "All $((${#MOUNTED_FILES[@]})) mounted files verified."
