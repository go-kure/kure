#!/usr/bin/env bash
# check-unmapped-docs.sh — List kure .md files not mapped to the docs site.
#
# Exits with 0 always (warning only). Prints unmapped files to stderr.
#
# Usage: bash scripts/check-unmapped-docs.sh [KURE_ROOT]

set -euo pipefail

SITE_DIR="$(cd "$(dirname "$0")/.." && pwd)"
KURE_ROOT="${1:-$(cd "$SITE_DIR/.." && pwd)}"

# Files intentionally excluded from the docs site.
# Internal design docs are not published — only package READMEs are mounted.
EXCLUDED_PATTERNS=(
  "AGENTS.md"
  "OAM-helm-alternative.md"
  "README.md"
  ".claude/"
  ".github/"
  "changelogs/"
  "docs/history/"
  "docs/reviews/"
  "docs/puzl-cloud-kubesdk-review.md"
  "docs/plugin-architecture-design.md"
  "docs/ux-design.md"
  "internal/"
  "cmd/"
  "testdata/"
  "DESIGN.md"
  "DESIGN-DETAILS.md"
  "CODE-DESIGN.md"
  "CODE-IMPLEMENTATION-PLAN.md"
  "ARCHITECTURE.md"
  "STATUS.md"
  "ERROR_HANDLING.md"
  "PATCH_ENGINE_DESIGN.md"
  "PATH_RESOLUTION.md"
)

# Files mapped in inject-frontmatter.sh (source paths relative to KURE_ROOT).
MAPPED_FILES=(
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

is_excluded() {
  local file="$1"
  for pattern in "${EXCLUDED_PATTERNS[@]}"; do
    if [[ "$file" == *"$pattern"* ]]; then
      return 0
    fi
  done
  return 1
}

is_mapped() {
  local file="$1"
  for mapped in "${MAPPED_FILES[@]}"; do
    if [[ "$file" == "$mapped" ]]; then
      return 0
    fi
  done
  return 1
}

unmapped_count=0

while IFS= read -r -d '' filepath; do
  relpath="${filepath#$KURE_ROOT/}"

  if is_excluded "$relpath"; then
    continue
  fi

  if is_mapped "$relpath"; then
    continue
  fi

  echo "UNMAPPED: $relpath" >&2
  ((unmapped_count++))
done < <(find "$KURE_ROOT" -name '*.md' -not -path '*/.git/*' -not -path '*/vendor/*' -not -path '*/site/*' -not -path '*_test.go' -print0 | sort -z)

if [[ $unmapped_count -gt 0 ]]; then
  echo "" >&2
  echo "Found $unmapped_count unmapped markdown file(s)." >&2
  echo "Add them to site/scripts/inject-frontmatter.sh or site/scripts/check-unmapped-docs.sh EXCLUDED_PATTERNS." >&2
fi
