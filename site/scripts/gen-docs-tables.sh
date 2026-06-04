#!/usr/bin/env bash
# gen-docs-tables.sh — Regenerate doc tables from the normative docs-map.yaml.
#
# Rewrites the content between BEGIN/END markers in:
#   - AGENTS.md                              "Reverse Mapping: Code to Docs" table
#   - site/content/api-reference/_index.md   package navigation tables
#
# Usage:
#   bash scripts/gen-docs-tables.sh          # rewrite the files in place
#   bash scripts/gen-docs-tables.sh --check  # fail if the files are out of date
#
# Never hand-edit the generated regions; edit docs-map.yaml and run this.

set -euo pipefail

SITE_DIR="$(cd "$(dirname "$0")/.." && pwd)"
KURE_ROOT="$(cd "$SITE_DIR/.." && pwd)"
DOCS_MAP="$SITE_DIR/docs-map.yaml"
AGENTS="$KURE_ROOT/AGENTS.md"
INDEX="$SITE_DIR/content/api-reference/_index.md"

BT='`'
CHECK=0
[[ "${1:-}" == "--check" ]] && CHECK=1

command -v yq >/dev/null 2>&1 || { echo "ERROR: yq (mikefarah v4) is required" >&2; exit 1; }

RM_BEGIN="<!-- BEGIN GENERATED: reverse-mapping (source: site/docs-map.yaml) -->"
RM_END="<!-- END GENERATED: reverse-mapping -->"
NAV_BEGIN="<!-- BEGIN GENERATED: api-reference-nav (source: site/docs-map.yaml) -->"
NAV_END="<!-- END GENERATED: api-reference-nav -->"

# Reverse-mapping table (AGENTS.md).
build_reverse_map() {
  echo "| Package Changed | Auto-Synced (README) | Guides to Review |"
  echo "|-----------------|---------------------|------------------|"
  yq '.packages[] | select(.mount) | [.path, .mount.target, (.guides // [] | join("||"))] | @tsv' "$DOCS_MAP" \
  | while IFS=$'\t' read -r path target guides; do
      local ref="${BT}${target%.md}${BT}"
      local g="—"
      if [[ -n "$guides" ]]; then
        g="${BT}${guides//||/${BT}, ${BT}}${BT}"
      fi
      echo "| ${BT}${path}/${BT} | ${ref} | ${g} |"
    done
  yq '.review_mappings[] | [.change, .reference, .guides] | @tsv' "$DOCS_MAP" \
  | while IFS=$'\t' read -r change ref guides; do
      echo "| ${change} | ${ref} | ${guides} |"
    done
}

# Grouped navigation tables (_index.md).
build_nav() {
  local first=1
  local group
  for group in "Core Domain" "Resource Operations" "Utilities"; do
    [[ $first -eq 1 ]] || echo ""
    first=0
    echo "## ${group}"
    echo ""
    echo "| Package | Description | Reference |"
    echo "|---------|-------------|-----------|"
    yq ".packages[] | select(.mount and .mount.group == \"${group}\") | [.path, .mount.target, .mount.title, .mount.weight, .mount.desc] | @tsv" "$DOCS_MAP" \
    | sort -t"$(printf '\t')" -k4 -n \
    | while IFS=$'\t' read -r path target title weight desc; do
        local base="${target#api-reference/}"; base="${base%.md}"
        echo "| [${title}](${base}) | ${desc} | [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure/${path}) |"
      done
  done
}

# replace_between FILE BEGIN END CONTENT_FILE — replace lines between markers.
replace_between() {
  local file="$1" begin="$2" end="$3" content_file="$4"
  [[ -f "$file" ]] || { echo "ERROR: file not found: $file" >&2; exit 1; }
  grep -qF "$begin" "$file" || { echo "ERROR: begin marker missing in $file" >&2; exit 1; }
  grep -qF "$end" "$file"   || { echo "ERROR: end marker missing in $file" >&2; exit 1; }
  awk -v b="$begin" -v e="$end" -v cf="$content_file" '
    index($0,b){print; while((getline line < cf)>0) print line; close(cf); skip=1; next}
    index($0,e){skip=0}
    skip{next}
    {print}
  ' "$file"
}

tmp_rm="$(mktemp)"; tmp_nav="$(mktemp)"
trap 'rm -f "$tmp_rm" "$tmp_nav"' EXIT
build_reverse_map > "$tmp_rm"
build_nav > "$tmp_nav"

status=0
apply() {
  local file="$1" begin="$2" end="$3" content_file="$4"
  local out; out="$(replace_between "$file" "$begin" "$end" "$content_file")"
  if [[ $CHECK -eq 1 ]]; then
    if ! diff -u <(cat "$file") <(printf '%s\n' "$out") >/dev/null; then
      echo "OUT OF DATE: $file (run: bash site/scripts/gen-docs-tables.sh)" >&2
      diff -u "$file" <(printf '%s\n' "$out") >&2 || true
      status=1
    fi
  else
    printf '%s\n' "$out" > "$file"
    echo "regenerated: $file"
  fi
}

apply "$AGENTS" "$RM_BEGIN" "$RM_END" "$tmp_rm"
apply "$INDEX"  "$NAV_BEGIN" "$NAV_END" "$tmp_nav"

exit $status
