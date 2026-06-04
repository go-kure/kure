#!/usr/bin/env bash
# check-doc-sync.sh — Validate kure's code↔docs mapping against docs-map.yaml.
#
# Blocking structural check (Layer 2 of the documentation-sync standard). Asserts:
#   1. Every public Go package (under code_roots) appears in docs-map.yaml exactly
#      once — a new package with no map entry fails here.
#   2. Every package path in the map exists on disk (no orphan entries).
#   3. Mounted packages have an existing README; unmounted ones carry a reason.
#   4. Mount targets are unique.
#   5. Every extra_mounts source file exists.
#   6. The generated tables (AGENTS.md, _index.md) are up to date.
#
# Usage: bash scripts/check-doc-sync.sh
# Exits non-zero on any violation.

set -euo pipefail

SITE_DIR="$(cd "$(dirname "$0")/.." && pwd)"
KURE_ROOT="$(cd "$SITE_DIR/.." && pwd)"
DOCS_MAP="$SITE_DIR/docs-map.yaml"

command -v yq >/dev/null 2>&1 || { echo "ERROR: yq (mikefarah v4) is required" >&2; exit 1; }
[[ -f "$DOCS_MAP" ]] || { echo "ERROR: docs map not found: $DOCS_MAP" >&2; exit 1; }

errors=0
fail() { echo "FAIL: $*" >&2; errors=$((errors + 1)); }

# Map package paths (sorted, deduped check).
mapfile -t map_paths < <(yq '.packages[].path' "$DOCS_MAP")

# 2. + 3. Validate each map entry.
for path in "${map_paths[@]}"; do
  [[ -d "$KURE_ROOT/$path" ]] || fail "docs-map package path does not exist: $path"
  mounted="$(yq ".packages[] | select(.path == \"$path\") | (.mount != null)" "$DOCS_MAP")"
  if [[ "$mounted" == "true" ]]; then
    readme="$(yq ".packages[] | select(.path == \"$path\") | .readme" "$DOCS_MAP")"
    [[ -n "$readme" && "$readme" != "null" ]] || { fail "mounted package missing readme: $path"; continue; }
    [[ -f "$KURE_ROOT/$readme" ]] || fail "mounted README not found: $readme (package $path)"
  else
    reason="$(yq ".packages[] | select(.path == \"$path\") | .reason" "$DOCS_MAP")"
    [[ -n "$reason" && "$reason" != "null" ]] || fail "unmounted package needs a reason: $path"
  fi
done

# Duplicate package paths.
dupes="$(printf '%s\n' "${map_paths[@]}" | sort | uniq -d)"
[[ -z "$dupes" ]] || fail "duplicate package paths in docs-map: $dupes"

# 1. Every public package on disk is in the map.
mapfile -t code_roots < <(yq '.code_roots[]' "$DOCS_MAP")
for root in "${code_roots[@]}"; do
  while IFS= read -r dir; do
    rel="${dir#"$KURE_ROOT"/}"
    if ! printf '%s\n' "${map_paths[@]}" | grep -qxF "$rel"; then
      fail "public package not in docs-map.yaml: $rel (add a mount: or mounted:false entry)"
    fi
  done < <(find "$KURE_ROOT/$root" -type f -name '*.go' ! -name '*_test.go' -printf '%h\n' | sort -u)
done

# 4. Unique mount targets.
dup_targets="$(yq '.packages[] | select(.mount) | .mount.target' "$DOCS_MAP" | sort | uniq -d)"
[[ -z "$dup_targets" ]] || fail "duplicate mount targets: $dup_targets"

# 5. extra_mounts sources exist.
while IFS= read -r src; do
  [[ -n "$src" ]] || continue
  [[ -f "$KURE_ROOT/$src" ]] || fail "extra_mounts source not found: $src"
done < <(yq '.extra_mounts[].source' "$DOCS_MAP")

# 6. Generated tables are current.
if ! bash "$SITE_DIR/scripts/gen-docs-tables.sh" --check >/dev/null 2>&1; then
  fail "generated tables are out of date — run: bash site/scripts/gen-docs-tables.sh"
fi

if [[ $errors -gt 0 ]]; then
  echo "check-doc-sync: $errors violation(s)." >&2
  exit 1
fi
echo "check-doc-sync: OK (${#map_paths[@]} packages mapped)."
