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
  "README.md"
  "CHANGELOG.md"
  "DEVELOPMENT.md"
  "docs/ARCHITECTURE.md"
  "docs/quickstart.md"
  "docs/compatibility.md"
  "docs/github-workflows.md"
  "docs/plugin-architecture-design.md"
  "docs/ux-design.md"
  "pkg/launcher/README.md"
  "pkg/launcher/DESIGN.md"
  "pkg/launcher/DESIGN-DETAILS.md"
  "pkg/launcher/CODE-DESIGN.md"
  "pkg/launcher/CODE-IMPLEMENTATION-PLAN.md"
  "pkg/launcher/ARCHITECTURE.md"
  "pkg/patch/DESIGN.md"
  "pkg/patch/ERROR_HANDLING.md"
  "pkg/patch/PATCH_ENGINE_DESIGN.md"
  "pkg/patch/PATH_RESOLUTION.md"
  "pkg/stack/DESIGN.md"
  "pkg/stack/STATUS.md"
  "pkg/stack/generators/DESIGN.md"
  "pkg/stack/generators/ARCHITECTURE.md"
  "pkg/stack/layout/README.md"
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
