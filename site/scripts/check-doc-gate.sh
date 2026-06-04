#!/usr/bin/env bash
# check-doc-gate.sh — Layer 3 of the documentation-sync standard: when a mapped
# package's source changes, its mapped docs (README and/or guides) MUST change in
# the same PR.
#
# The trigger is intentionally coarse ("mapped package source changed", not a true
# public-API diff); the maintainer-restricted `docs-skip` escape hatch handles the
# false positives (routine private-helper edits).
#
# Usage: bash scripts/check-doc-gate.sh <base-ref> [KURE_ROOT] [--skip=true|false]
#   <base-ref>  ref to diff against (e.g. origin/main)
#   --skip      when true, bypass the gate (the CI job passes this from the
#               docs-skip label). Prints a notice and exits 0.

set -euo pipefail

SKIP=false
ARGS=()
for a in "$@"; do
  case "$a" in
    --skip=*) SKIP="${a#--skip=}" ;;
    *) ARGS+=("$a") ;;
  esac
done
BASE="${ARGS[0]:?usage: check-doc-gate.sh <base-ref> [KURE_ROOT] [--skip=...]}"
ROOT="${ARGS[1]:-$(git rev-parse --show-toplevel)}"
MAP="$ROOT/site/docs-map.yaml"

if [[ "$SKIP" == "true" ]]; then
  echo "doc-gate: bypassed via maintainer docs-skip label."
  exit 0
fi

command -v yq >/dev/null 2>&1 || { echo "ERROR: yq is required" >&2; exit 1; }
[[ -f "$MAP" ]] || { echo "ERROR: docs map not found: $MAP" >&2; exit 1; }

changed_set="$(git -C "$ROOT" diff --name-only "${BASE}...HEAD")"
doc_changed() { grep -qxF "$1" <<<"$changed_set"; }

violations=0
while IFS= read -r path; do
  [[ -n "$path" ]] || continue
  # Source files directly in this package dir (not nested subpackages, which are
  # their own map entries), excluding tests.
  src="$(grep -E "^${path}/[^/]+\.go$" <<<"$changed_set" | grep -v '_test\.go$' || true)"
  [[ -n "$src" ]] || continue

  readme="$(yq ".packages[] | select(.path==\"$path\") | .readme" "$MAP")"
  ok=0
  [[ -n "$readme" && "$readme" != "null" ]] && doc_changed "$readme" && ok=1
  while IFS= read -r g; do
    [[ -n "$g" && "$g" != "null" ]] || continue
    doc_changed "site/content/${g}.md" && ok=1
  done < <(yq ".packages[] | select(.path==\"$path\") | .guides[]?" "$MAP")

  if [[ $ok -ne 1 ]]; then
    echo "FAIL: $path source changed but its docs did not (expected a change to $readme or its mapped guides)"
    violations=$((violations + 1))
  fi
done < <(yq '.packages[].path' "$MAP")

if [[ $violations -gt 0 ]]; then
  echo "doc-gate: $violations package(s) changed without doc updates." >&2
  echo "Update the package README/guide in this PR, or apply the maintainer-restricted 'docs-skip' label." >&2
  exit 1
fi
echo "doc-gate: OK"
