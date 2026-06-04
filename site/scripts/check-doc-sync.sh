#!/usr/bin/env bash
# check-doc-sync.sh — Canonical structural validator for the documentation-sync
# standard (see meta/standards/documentation.md, "Documentation Sync").
#
# Validates a repo's docs-map.yaml against the filesystem:
#   1. Every public Go package (under code_roots) appears in the map exactly once.
#      Skipped when the map sets docs_only: true.
#   2. Every package path in the map exists on disk (no orphan entries).
#   3. Mounted packages have an existing README; unmounted ones carry a reason.
#   4. Mount targets are unique.
#   5. Every extra_mounts source file exists.
#   6. If a table generator is present (<map-dir>/scripts/gen-docs-tables.sh), the
#      generated tables are up to date.
#
# Usage: check-doc-sync.sh [REPO_ROOT] [--map PATH]
#   REPO_ROOT defaults to the current directory.
#   --map     path to docs-map.yaml; defaults to <root>/docs-map.yaml or
#             <root>/site/docs-map.yaml.
# Exits non-zero on any violation.

set -euo pipefail

ROOT="."
MAP=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --map) MAP="$2"; shift 2 ;;
    *) ROOT="$1"; shift ;;
  esac
done
ROOT="$(cd "$ROOT" && pwd)"

command -v yq >/dev/null 2>&1 || { echo "ERROR: yq (mikefarah v4) is required" >&2; exit 1; }

if [[ -z "$MAP" ]]; then
  if [[ -f "$ROOT/docs-map.yaml" ]]; then MAP="$ROOT/docs-map.yaml"
  elif [[ -f "$ROOT/site/docs-map.yaml" ]]; then MAP="$ROOT/site/docs-map.yaml"
  else echo "ERROR: docs-map.yaml not found under $ROOT (or $ROOT/site)" >&2; exit 1
  fi
fi
MAP_DIR="$(cd "$(dirname "$MAP")" && pwd)"

errors=0
fail() { echo "FAIL: $*" >&2; errors=$((errors + 1)); }

docs_only="$(yq '.docs_only // false' "$MAP")"

# Map package paths.
mapfile -t map_paths < <(yq '.packages[]?.path' "$MAP")

# 2. + 3. Validate each map entry.
for path in "${map_paths[@]}"; do
  [[ -z "$path" || "$path" == "null" ]] && continue
  [[ -d "$ROOT/$path" ]] || fail "docs-map package path does not exist: $path"
  # Every package entry MUST name an existing README (mounted or not).
  readme="$(yq ".packages[] | select(.path == \"$path\") | .readme" "$MAP")"
  if [[ -z "$readme" || "$readme" == "null" ]]; then
    fail "package missing readme: $path"
  elif [[ ! -f "$ROOT/$readme" ]]; then
    fail "package README not found: $readme (package $path)"
  fi
  mounted="$(yq ".packages[] | select(.path == \"$path\") | (.mount != null)" "$MAP")"
  if [[ "$mounted" != "true" ]]; then
    reason="$(yq ".packages[] | select(.path == \"$path\") | .reason" "$MAP")"
    [[ -n "$reason" && "$reason" != "null" ]] || fail "unmounted package needs a reason: $path"
  fi
done

# Duplicate package paths.
if [[ ${#map_paths[@]} -gt 0 ]]; then
  dupes="$(printf '%s\n' "${map_paths[@]}" | sort | uniq -d)"
  [[ -z "$dupes" ]] || fail "duplicate package paths in docs-map: $dupes"
fi

# 1. Every public package on disk is in the map (skipped for docs_only repos).
if [[ "$docs_only" != "true" ]]; then
  mapfile -t code_roots < <(yq '.code_roots[]?' "$MAP")
  for croot in "${code_roots[@]}"; do
    [[ -z "$croot" || "$croot" == "null" ]] && continue
    [[ -d "$ROOT/$croot" ]] || { fail "code_root does not exist: $croot"; continue; }
    while IFS= read -r dir; do
      rel="${dir#"$ROOT"/}"
      if ! printf '%s\n' "${map_paths[@]}" | grep -qxF "$rel"; then
        fail "public package not in docs-map.yaml: $rel (add a mount: or mounted:false entry)"
      fi
    done < <(find "$ROOT/$croot" -type f -name '*.go' ! -name '*_test.go' -printf '%h\n' | sort -u)
  done
fi

# 4. Unique mount targets across packages AND extra_mounts (collisions silently
#    overwrite generated content).
dup_targets="$( { yq '.packages[]? | select(.mount) | .mount.target' "$MAP"; yq '.extra_mounts[]?.target' "$MAP"; } | grep -vx 'null' | sort | uniq -d)"
[[ -z "$dup_targets" ]] || fail "duplicate mount targets (packages + extra_mounts): $dup_targets"

# 5. extra_mounts sources exist.
while IFS= read -r src; do
  [[ -z "$src" || "$src" == "null" ]] && continue
  [[ -f "$ROOT/$src" ]] || fail "extra_mounts source not found: $src"
done < <(yq '.extra_mounts[]?.source' "$MAP")

# 6. Generated tables are current (if a generator is present).
GEN="$MAP_DIR/scripts/gen-docs-tables.sh"
if [[ -x "$GEN" || -f "$GEN" ]]; then
  if ! bash "$GEN" --check >/dev/null 2>&1; then
    fail "generated tables are out of date — run: bash ${GEN#"$ROOT"/}"
  fi
fi

if [[ $errors -gt 0 ]]; then
  echo "check-doc-sync: $errors violation(s)." >&2
  exit 1
fi
echo "check-doc-sync: OK (${#map_paths[@]} packages mapped, docs_only=$docs_only)."
