#!/usr/bin/env bash
# inject-frontmatter.sh — Prepend Hugo front matter to kure markdown files.
#
# Kure docs lack Hugo front matter. This script reads the mapping from the
# normative source (site/docs-map.yaml) and writes processed copies into
# .generated/ for Hugo to mount as content. The mapping is NOT duplicated here —
# edit docs-map.yaml to change what gets published.
#
# Usage: bash scripts/inject-frontmatter.sh [KURE_ROOT]
#   KURE_ROOT defaults to .. (parent of site/)

set -euo pipefail

SITE_DIR="$(cd "$(dirname "$0")/.." && pwd)"
KURE_ROOT="${1:-$(cd "$SITE_DIR/.." && pwd)}"
GEN_DIR="$SITE_DIR/.generated"
DOCS_MAP="$SITE_DIR/docs-map.yaml"

command -v yq >/dev/null 2>&1 || { echo "ERROR: yq (mikefarah v4) is required" >&2; exit 1; }
[[ -f "$DOCS_MAP" ]] || { echo "ERROR: docs map not found: $DOCS_MAP" >&2; exit 1; }

rm -rf "$GEN_DIR"
mkdir -p "$GEN_DIR"

# inject_fm SOURCE_FILE TARGET_PATH TITLE WEIGHT
#   Copies SOURCE_FILE into .generated/TARGET_PATH with Hugo front matter prepended.
#   A missing source is fatal: docs-map.yaml declared it, so it must exist.
inject_fm() {
  local src="$1" target="$2" title="$3" weight="$4"
  local dest="$GEN_DIR/$target"

  if [[ ! -f "$src" ]]; then
    echo "ERROR: mapped source file not found: $src (target: $target)" >&2
    exit 1
  fi

  mkdir -p "$(dirname "$dest")"
  {
    echo "---"
    echo "title: \"$title\""
    echo "weight: $weight"
    echo "---"
    echo ""
    cat "$src"
  } > "$dest"
}

# Mounted package READMEs, then non-package extra mounts — all from docs-map.yaml.
while IFS=$'\t' read -r src target title weight; do
  [[ -n "$src" ]] || continue
  inject_fm "$KURE_ROOT/$src" "$target" "$title" "$weight"
done < <(yq '.packages[] | select(.mount) | [.readme, .mount.target, .mount.title, .mount.weight] | @tsv' "$DOCS_MAP")

while IFS=$'\t' read -r src target title weight; do
  [[ -n "$src" ]] || continue
  inject_fm "$KURE_ROOT/$src" "$target" "$title" "$weight"
done < <(yq '.extra_mounts[] | [.source, .target, .title, .weight] | @tsv' "$DOCS_MAP")

echo "Front matter injection complete. Output in $GEN_DIR"
