#!/usr/bin/env bash
# check-forbidden-terms.sh — Guards upstream (open-source go-kure) repos against
# references to downstream / closed-source Wharf consumers.
#
# Canonical copy lives in go-kure/.github at scripts/check-forbidden-terms.sh.
# Consuming repos (kure, launcher) may vendor a byte-synced copy; do not edit the
# vendored copy directly — change it here and re-sync.
#
# See docs/standards.md, "No Downstream References (MUST)".
#
# Modes:
#   --full-tree        Scan every tracked in-scope file. The blessed CI gate: run on
#                      EVERY event (pull_request/push/schedule/merge_group) so PR and
#                      merge-queue results are identical.
#   --diff BASE        Scan only lines added versus BASE (e.g. origin/main). Local/dev
#                      convenience only — MUST NOT gate CI (it diverges PR vs. queue).
#
# Scope (both modes): docs/  site/content/  pkg/**  cmd/**  scripts/**  **/*.md
#   .github/workflows/**  and .github/actions/**. The guard script itself is
#   excluded from its own scan.
#
# Forbidden terms (case-insensitive, whole word):
#   wharf  crane  barge  harbor  rudder
#
# Escape hatch: place `allow-term:<word>` on the same line as a legitimate hit, or
# on the line immediately above or below it (e.g. go-containerregistry's `crane`).
#
# Usage: check-forbidden-terms.sh --full-tree
#        check-forbidden-terms.sh --diff origin/main

set -euo pipefail

TERMS_RE='wharf|crane|barge|harbor|rudder'

MODE=""
BASE=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --full-tree) MODE="full"; shift ;;
    --diff) MODE="diff"; BASE="${2:-}"; [[ -n "$BASE" ]] || { echo "ERROR: --diff needs a BASE ref" >&2; exit 2; }; shift 2 ;;
    -h|--help) sed -n '2,29p' "$0"; exit 0 ;;
    *) echo "ERROR: unknown argument: $1" >&2; exit 2 ;;
  esac
done
[[ -n "$MODE" ]] || { echo "usage: $0 --full-tree | --diff BASE" >&2; exit 2; }

# In-scope path? (excludes the guard script itself, wherever it is vendored)
in_scope() {
  local f="$1"
  case "$f" in
    */check-forbidden-terms.sh|check-forbidden-terms.sh) return 1 ;;
  esac
  case "$f" in
    docs/*|site/content/*|pkg/*|cmd/*|scripts/*|.github/workflows/*|.github/actions/*) return 0 ;;
    *.md) return 0 ;;
  esac
  return 1
}

# Is this hit fully covered by `allow-term:<word>` pragmas? Checks the hit line and
# the lines immediately above and below it. EVERY matched word on the line must be
# covered, so a pragma for one term cannot silently excuse another on the same line.
allowed() {
  local file="$1" lineno="$2" words="$3" w
  local ctx
  ctx="$(sed -n "$((lineno > 1 ? lineno - 1 : 1)),$((lineno + 1))p" "$file" 2>/dev/null || true)"
  for w in $words; do
    printf '%s' "$ctx" | grep -qiE "allow-term:${w}\b" || return 1
  done
  return 0
}

violations=0
report() {
  echo "FORBIDDEN: $1:$2: $3" >&2
  violations=$((violations + 1))
}

# Emits "<file>\t<lineno>" candidate hits, one per line, depending on the mode.
candidates() {
  if [[ "$MODE" == "full" ]]; then
    local f
    while IFS= read -r f; do
      in_scope "$f" || continue
      [[ -f "$f" ]] || continue
      # grep exits 1 when a file has no match; tolerate it under set -e/pipefail.
      { grep -niE "\b(${TERMS_RE})\b" "$f" 2>/dev/null || true; } | while IFS=: read -r ln _; do
        printf '%s\t%s\n' "$f" "$ln"
      done
    done < <(git ls-files)
  else
    # Added lines only. --unified=0 suppresses context so the running line number
    # advances solely on added lines within each hunk.
    git diff --unified=0 --no-color "$BASE" -- . | awk '
      /^\+\+\+ / { f=$2; sub(/^b\//,"",f); next }
      # Hunk header: @@ -a,b +c,d @@ [context]. Take the first "+<digits>" as the
      # new-file start line; a greedy strip would break on a "+" in the context tail.
      /^@@ /     { if (match($0, /\+[0-9]+/)) ln = substr($0, RSTART+1, RLENGTH-1)+0; next }
      /^\+/      { print f "\t" ln; ln++ }
    '
  fi
}

while IFS=$'\t' read -r file lineno; do
  [[ -n "$file" && -n "$lineno" ]] || continue
  in_scope "$file" || continue
  [[ -f "$file" ]] || continue
  text="$(sed -n "${lineno}p" "$file")"
  # Matched words on this line (deduped, lowercase). grep exits 1 with no match —
  # expected for diff-mode candidate lines that carry no term; tolerate under set -e.
  words="$(printf '%s' "$text" | grep -oiE "\b(${TERMS_RE})\b" | tr '[:upper:]' '[:lower:]' | sort -u | tr '\n' ' ' || true)"
  [[ -n "${words// /}" ]] || continue
  allowed "$file" "$lineno" "$words" && continue
  report "$file" "$lineno" "$text"
done < <(candidates | sort -u)

if [[ $violations -gt 0 ]]; then
  echo "check-forbidden-terms: $violations forbidden downstream reference(s) ($MODE mode)." >&2
  echo "Reword to a generic role, or add an adjacent 'allow-term:<word>' pragma if legitimate." >&2
  exit 1
fi
echo "check-forbidden-terms: OK ($MODE mode)."
