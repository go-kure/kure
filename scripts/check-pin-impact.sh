#!/usr/bin/env bash
# check-pin-impact.sh — render, and gate on, the real impact of a go-kure/.github
# pin bump before it merges.
#
# .github/workflows/*.yml pin go-kure/.github by commit SHA in two shapes: the
# `uses: go-kure/.github/.github/actions/<name>@<sha>` form (one per composite
# action consumed) and a `repository: go-kure/.github` / `ref: <sha>` checkout
# (the forbidden-terms job's checkout, which byte-compares the vendored guard,
# derives its ref from a `uses:` pin through a `${{ }}` expression instead).
# Renovate bumps every occurrence to the same new SHA in one
# PR (renovate.json's github-actions group), and its PR body offers nothing
# more than a compare link across the WHOLE dot-github repo — most of which
# (pr-review tooling, label taxonomy, docs) kure never executes. Deciding
# "does this bump actually change anything kure runs" required, by hand: list
# the actions kure references -> resolve each action.yml to the scripts/*.sh
# it runs -> resolve one level of `source` -> intersect that set against the
# compare's changed files (go-kure/kure#719, 2026-08-30). This script does
# that intersection and fails when it's non-empty, so a bump that touches a
# path kure actually executes cannot merge unreviewed.
#
# Two modes:
#   --base-ref REF   CI mode. NEW pin state is read from the working tree's
#                    own .github/workflows/*.yml (must be internally
#                    consistent — every occurrence pinning the same SHA).
#                    OLD pin state is read the same way from `git show
#                    REF:<file>`. Fails if either side is inconsistent.
#   --old SHA --new SHA
#                    Manual/verification mode: skip reading pin state from
#                    any git ref: fetch action.yml/script content directly at
#                    the given SHAs. The set of ACTIONS to inspect still comes
#                    from the working tree (which actions kure references
#                    doesn't depend on which commit go-kure/.github is at) —
#                    only which SHA their content is fetched at is overridden.
#                    Lets a bump be checked against arbitrary historical SHAs
#                    without checking out that exact repo state.
#
# CI images do not have mise installed (see check-tool-versions.sh) and this
# job runs bare like action-pins/forbidden-terms — no yq, no python. Plain
# bash + curl + grep + sed + awk only.
#
# Followed:
#   - pins: `uses: go-kure/.github/.github/actions/<subpath>@<40-hex>`, the
#     subpath nested or dotted (`group/check`, `check.v2`), and the `ref:
#     <40-hex>` of a block-style `repository: go-kure/.github` checkout step
#     in any key order; the repository name case-insensitively, the hex in
#     either case.
#   - per action: each `$GITHUB_ACTION_PATH/<rel>.sh` in its one `run:` step,
#     resolved against .github/actions/<subpath>/ with `..` hops counted.
#   - per script, transitively: a whole line `source|. "$SCRIPT_DIR/<name>.sh"`
#     or `[exec] [bash|sh] "$SCRIPT_DIR/<name>.sh" [args]` (args without
#     separators or substitutions; fd redirects, `&>file` and `&>>file`
#     allowed), resolved against the script's own directory, where SCRIPT_DIR
#     is defined as exactly `$(dirname "$0")`, `$(cd "$(dirname "$0")"
#     [&>/dev/null | >/dev/null 2>&1] && pwd)` or either with
#     "${BASH_SOURCE[0]}" for "$0", optionally behind `declare -r`,
#     `readonly` or `export`.
#
# Fails closed — aborts loudly rather than silently under-reporting the
# consumed set; a false "no impact" is the one failure mode this script
# exists to prevent — on:
#   - pins: inconsistent pins; a go-kure/.github reference in a `uses:` or
#     `repository:` context that yields no 40-hex pin (`@main`, a trailing
#     slash, `go-kure/.github@<sha>`, a subpath outside .github/actions, a
#     flow-mapping checkout, a checkout with a branch, another expression or
#     no `ref:`, a reusable-workflow call pinned to a SHA or inside a step) —
#     only a checkout whose `ref:` is exactly
#     `${{ steps.<id>.outputs.<name> }}`, and a job-level reusable-workflow
#     call at a non-SHA ref (below), may carry none.
#   - actions: `runs.using` other than composite (JavaScript and Docker
#     actions execute code no scan here can see); a nested `uses:`; more than
#     one `run:` step; a `github.action_path` expression; a `run:` step with
#     no `$GITHUB_ACTION_PATH` script; a non-.sh `$GITHUB_ACTION_PATH` target;
#     a .sh path mentioned that no `$GITHUB_ACTION_PATH` reference accounts
#     for; a '.'/'..' action subpath segment; a path that climbs above the
#     repository root or has an empty (`//`) segment.
#   - scripts: any line using `$SCRIPT_DIR` that is not exactly one of the two
#     followed forms; any `SCRIPT_DIR=` assignment (with any prefix) that is
#     not exactly one of the definition shapes above; any other line computing
#     the script's own directory (dirname "$0", ${0%/*}, BASH_SOURCE); any
#     other `source`/`.`, or `bash`/`sh`/path invocation of a .sh file, at a
#     command position; a '.'/'..' or empty segment in a sibling path; a
#     sibling that cannot be fetched.
#   - the compare: a status other than `ahead`; a file count near GitHub's
#     pagination cap.
#
# Not covered, knowingly:
#   - job-level reusable-workflow calls at a non-SHA ref, `uses:
#     go-kure/.github/.github/workflows/<file>.yml@<ref>`: they run at their
#     own ref (kure's call @main), not at the action pin, so no pin bump
#     changes them and this gate neither pins nor audits them.
#   - a sibling reached without naming $SCRIPT_DIR or the script's own
#     directory — a hard-coded absolute path, or a name found on PATH.
#   - YAML this line-based scan does not parse: quoted keys (`"uses":`,
#     `"repository":`, and `"using":` in an action.yml — alone the last fails
#     closed as `runs.using: <none>`, beside an unquoted `using: composite`
#     line it goes unseen), a `uses:` value continued on the next line,
#     anchors and aliases, and a repository given as an expression
#     (`${{ github.repository_owner }}/.github`).
#   - a sourced file's `$SCRIPT_DIR` is its caller's, but its siblings are
#     resolved against the sourced file's own directory; the two agree while
#     every consumed script sits in one directory, as today.
#   - false aborts, refused although harmless: a `[[ "${BASH_SOURCE[0]}" ==
#     "$0" ]]` guard, and a directory other than the script's own derived from
#     it, such as `ROOT=$(cd "$(dirname "$0")/.." && pwd)`.
#
# A genuine hit (a consumed path did change) is not necessarily wrong to
# merge — the pin bump may have been reviewed and found fine. There is no
# reviewer-facing way to express that short of a maintainer adding the
# `pin-impact-ack` label to the PR (same convention as check-doc-gate's
# `docs-skip`), which this script honours via PIN_IMPACT_ACK=true — a
# deliberate, audited override, never a silent pass.
#
# Usage: check-pin-impact.sh --base-ref origin/main
#        check-pin-impact.sh --old <40-hex> --new <40-hex>

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

DOTGITHUB_REPO="go-kure/.github"
WORKFLOW_FILES=(.github/workflows/*.yml .github/workflows/*.yaml)

BASE_REF=""
OLD_SHA=""
NEW_SHA=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --base-ref) BASE_REF="${2:-}"; [[ -n "$BASE_REF" ]] || { echo "check-pin-impact: --base-ref needs a REF" >&2; exit 2; }; shift 2 ;;
    --old) OLD_SHA="${2:-}"; [[ -n "$OLD_SHA" ]] || { echo "check-pin-impact: --old needs a SHA" >&2; exit 2; }; shift 2 ;;
    --new) NEW_SHA="${2:-}"; [[ -n "$NEW_SHA" ]] || { echo "check-pin-impact: --new needs a SHA" >&2; exit 2; }; shift 2 ;;
    -h|--help) sed -n '2,114p' "$0"; exit 0 ;;
    *) echo "check-pin-impact: unknown argument: $1" >&2; exit 2 ;;
  esac
done
if [[ -n "$BASE_REF" && ( -n "$OLD_SHA" || -n "$NEW_SHA" ) ]]; then
  echo "check-pin-impact: --base-ref and --old/--new are mutually exclusive" >&2
  exit 2
fi
if [[ -z "$BASE_REF" && ( -z "$OLD_SHA" || -z "$NEW_SHA" ) ]]; then
  echo "usage: $0 --base-ref REF | --old SHA --new SHA" >&2
  exit 2
fi

is_sha() { [[ "$1" =~ ^[0-9a-f]{40}$ ]]; }

# scan_workflow -- read one workflow file's content on stdin and account for
# every go-kure/.github reference in it. Prints one record per line:
#   pin <sha>      a 40-hex pin (uppercase hex is read, and printed lowercased)
#   action <sub>   the <subpath> of a pinned `uses:` action
#   bad <n>: <l>   line <n> references go-kure/.github in a `uses:` or
#                  `repository:` context but yields no pin
#
# `uses:` pins: the value must be exactly
# `go-kure/.github/.github/actions/<subpath>@<40-hex>`, the repository name
# matched case-insensitively (GitHub's are) and anchored at the value's start
# (so `xgo-kure/.github/...` is another repo, not this one). The subpath may be
# nested (`group/check`) or dotted (`check.v2`); a single-segment pattern used
# to skip both (go-kure/kure#731 round 9). A '.'/'..' segment is accepted here
# and refused where the action is resolved.
#
# `repository: go-kure/.github` checkouts: the `ref:` of the same step is the
# pin, whatever the order of the step's keys (it used to be only a `ref:` on
# the very next line, round 9). A step is a list item: it runs from its `-`
# line until the next line indented no deeper than that dash, so a nested list
# inside the step stays part of it and another step's `ref:` is never
# attributed to this checkout. A `ref:` that is exactly
# `${{ steps.<id>.outputs.<name> }}` (the forbidden-terms checkout derives its
# ref from a `uses:` pin that way) is no pin and no refusal; any other
# expression is refused, since it could name any commit.
#
# Completeness: anything else in a `uses:`/`repository:` context that names
# go-kure/.github — `@main`, a trailing slash, `go-kure/.github@<sha>`, a
# subpath outside .github/actions, a flow mapping `{ repository: ..., ref: ...
# }`, a checkout with a branch or no `ref:` — used to be skipped, silently
# dropping its SHA from the consistency check (go-kure/kure#731 review of
# go-kure/kure#886). It is reported as `bad` and the caller refuses the run.
# The one other exemption is a job-level (not in-step) reusable-workflow call,
# `uses: go-kure/.github/.github/workflows/<file>.yml@<ref>`, at a non-SHA ref.
# Comment lines and trailing ` #` comments are ignored; a mention of
# go-kure/.github in any other context (a `run:` string, a `name:`) is not a
# pin and not counted.
#
# Plain POSIX awk (no interval expressions, no gawk extensions): the pin-impact
# job runs bare.
scan_workflow() {
  awk -v repo="$DOTGITHUB_REPO" -v q="'" '
    function scalar(v) {
      sub(/[[:space:]]+$/, "", v)
      if (v ~ /^".*"$/ || v ~ ("^" q ".*" q "$")) v = substr(v, 2, length(v) - 2)
      return v
    }
    function is_hex40(v) { return v ~ /^[0-9a-f]+$/ && length(v) == 40 }
    function bad(n, l) { sub(/^[[:space:]]+/, "", l); print "bad " n ": " l }
    function flush() {
      if (is_repo) {
        if (sha != "") print "pin " sha
        else if (!ref_expr) bad(repo_nr, repo_text " (no 40-hex ref: in this checkout step)")
      }
      in_step = 0; is_repo = 0; sha = ""; ref_expr = 0
    }
    BEGIN { prefix = repo "/.github/actions/" }
    /^[[:space:]]*(#|$)/ { next }
    {
      code = $0
      sub(/[[:space:]]+#.*$/, "", code)
      lc = tolower(code)
      mentions = (lc ~ ("(^|[^a-z0-9_.-])go-kure/[.]github([^a-z0-9_.-]|$)"))

      match(code, /^ */); ind = RLENGTH
      if (code ~ /^ *-([[:space:]]|$)/) {
        if (!in_step || ind <= step_ind) { flush(); in_step = 1; step_ind = ind }
      } else if (in_step && ind <= step_ind) {
        flush()
      }

      line = code
      sub(/^ *(-[[:space:]]+)?/, "", line)
      if (line ~ /^uses:/) {
        sub(/^uses:[[:space:]]*/, "", line)
        v = scalar(line)
        lv = tolower(v)
        at = 0
        for (i = length(lv); i > 0; i--) if (substr(lv, i, 1) == "@") { at = i; break }
        sub_path = (at > 0) ? substr(v, length(prefix) + 1, at - length(prefix) - 1) : ""
        ref = (at > 0) ? substr(lv, at + 1) : ""
        if (at > 0 && substr(lv, 1, length(prefix)) == prefix \
            && sub_path ~ /^[A-Za-z0-9_.-]+(\/[A-Za-z0-9_.-]+)*$/ && is_hex40(ref)) {
          print "pin " ref
          print "action " sub_path
        } else if (!in_step && !is_hex40(ref) \
            && lv ~ /^go-kure\/[.]github\/[.]github\/workflows\/[a-z0-9_.-]+[.]ya?ml@[^[:space:]]+$/) {
          # A job-level reusable-workflow call at a non-SHA ref: runs at that
          # ref (kure calls these at @main), is not the action pin, and is
          # outside this gate. A SHA-pinned one would move with a bump this
          # gate does not audit, and one inside a step is no such call: both
          # fall through to `bad`.
        } else if (mentions) {
          bad(NR, $0)
        }
      } else if (line ~ /^repository:/) {
        sub(/^repository:[[:space:]]*/, "", line)
        if (tolower(scalar(line)) == repo && in_step) {
          is_repo = 1; repo_nr = NR; repo_text = $0
        } else if (mentions) {
          bad(NR, $0)
        }
      } else if (line ~ /^ref:/ && in_step) {
        sub(/^ref:[[:space:]]*/, "", line)
        v = scalar(line)
        if (is_hex40(tolower(v))) { sha = tolower(v); ref_expr = 0 }
        else if (v ~ /^\$\{\{ *steps\.[A-Za-z0-9_-]+\.outputs\.[A-Za-z0-9_-]+ *\}\}$/) { sha = ""; ref_expr = 1 }
        else { sha = ""; ref_expr = 0 }
      } else if (mentions && (lc ~ /(^|[^a-z0-9_-])(uses|repository)[[:space:]]*:/)) {
        bad(NR, $0)
      }
    }
    END { flush() }
  '
}

# Field <kind> of scan_workflow's output on stdin: `pin`, `action` or `bad`.
scan_field() { sed -n "s/^$1 //p"; }

# Resolve a single consistent SHA out of every workflow file's content, or
# fail loudly if the files disagree or none is found. $1: human label for
# error messages ("current working tree" / "REF").
resolve_pin() {
  local label="$1"; shift
  local shas
  shas="$(printf '%s\n' "$@" | sort -u)"
  local n
  n="$(printf '%s\n' "$shas" | grep -c . || true)"
  if [[ "$n" -eq 0 ]]; then
    echo "check-pin-impact: no go-kure/.github pin found in ${WORKFLOW_FILES[*]} (${label})" >&2
    exit 1
  fi
  if [[ "$n" -gt 1 ]]; then
    echo "check-pin-impact: inconsistent go-kure/.github pins across ${WORKFLOW_FILES[*]} (${label}):" >&2
    printf '%s\n' "$shas" | sed 's/^/  /' >&2
    exit 1
  fi
  printf '%s\n' "$shas"
}

# refuse_bad_refs <label> <scan-output-by-file>... -- abort if any scanned
# file has a go-kure/.github reference that yielded no pin (see scan_workflow).
# Arguments come in pairs: file label, that file's scan_workflow output.
refuse_bad_refs() {
  local where="$1"; shift
  local found="" file bads
  while [[ $# -gt 0 ]]; do
    file="$1"; bads="$(printf '%s\n' "$2" | scan_field bad)"; shift 2
    [[ -n "$bads" ]] && found="${found}$(printf '%s\n' "$bads" | sed "s#^#  ${file}:#")"$'\n'
  done
  if [[ -n "$found" ]]; then
    echo "check-pin-impact: unrecognized go-kure/.github reference(s) (${where}) — a reference that yields no 40-hex pin would be left out of the consistency check, refusing to guess:" >&2
    printf '%s' "$found" >&2
    exit 1
  fi
}

# --- Scan the working tree: action names (stable across old/new) and pins ---
declare -A scan_new=()
scan_new_args=()
for f in "${WORKFLOW_FILES[@]}"; do
  [[ -f "$f" ]] || continue
  scan_new["$f"]="$(scan_workflow <"$f")"
  scan_new_args+=("$f" "${scan_new[$f]}")
done
refuse_bad_refs "current working tree" "${scan_new_args[@]}"
action_names="$(for f in "${!scan_new[@]}"; do printf '%s\n' "${scan_new[$f]}" | scan_field action; done | sort -u)"
if [[ -z "$action_names" ]]; then
  echo "check-pin-impact: no go-kure/.github action references found in ${WORKFLOW_FILES[*]}" >&2
  exit 1
fi

# --- Resolve OLD/NEW SHAs ---
if [[ -n "$BASE_REF" ]]; then
  new_shas=()
  old_shas=()
  scan_old_args=()
  for f in "${WORKFLOW_FILES[@]}"; do
    [[ -f "$f" ]] || continue
    while IFS= read -r s; do [[ -n "$s" ]] && new_shas+=("$s"); done < <(printf '%s\n' "${scan_new[$f]}" | scan_field pin)
    base_content="$(git show "${BASE_REF}:${f}" 2>/dev/null || true)"
    [[ -n "$base_content" ]] || continue
    scan_old="$(printf '%s\n' "$base_content" | scan_workflow)"
    scan_old_args+=("$f" "$scan_old")
    while IFS= read -r s; do [[ -n "$s" ]] && old_shas+=("$s"); done < <(printf '%s\n' "$scan_old" | scan_field pin)
  done
  if [[ ${#scan_old_args[@]} -gt 0 ]]; then refuse_bad_refs "$BASE_REF" "${scan_old_args[@]}"; fi
  NEW_SHA="$(resolve_pin "current working tree" "${new_shas[@]}")"
  OLD_SHA="$(resolve_pin "$BASE_REF" "${old_shas[@]}")"
else
  is_sha "$OLD_SHA" || { echo "check-pin-impact: --old '$OLD_SHA' is not a 40-hex SHA" >&2; exit 2; }
  is_sha "$NEW_SHA" || { echo "check-pin-impact: --new '$NEW_SHA' is not a 40-hex SHA" >&2; exit 2; }
fi

echo "check-pin-impact: go-kure/.github ${OLD_SHA:0:8} -> ${NEW_SHA:0:8}"

if [[ "$OLD_SHA" == "$NEW_SHA" ]]; then
  echo "check-pin-impact: no pin change — OK"
  exit 0
fi

# --- Fetch helper: raw file content at a SHA, or empty+nonzero on failure ---
fetch() {
  local sha="$1" path="$2"
  local -a auth_args=()
  [[ -n "${GITHUB_TOKEN:-}" ]] && auth_args=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
  curl -fsSL --connect-timeout 10 --max-time 30 "${auth_args[@]}" \
    "https://raw.githubusercontent.com/${DOTGITHUB_REPO}/${sha}/${path}"
}

# --- Build the consumed-path set at NEW_SHA ---
declare -A consumed=()   # path -> 1
declare -A queued=()     # path -> 1 (scripts already fetched/expanded)
queue=()

add_consumed() { consumed["$1"]=1; }
enqueue() { [[ -n "${queued[$1]:-}" ]] && return 0; queued["$1"]=1; queue+=("$1"); }

# normalize_repo_path <dir> <rel> -- print the canonical repo path of <rel>
# taken from <dir>: `.` segments dropped, each `..` removing one directory.
# On a path that cannot be canonicalized — one that climbs above the repo
# root, or has an empty (`//`) segment — print why and return 1. GitHub's
# compare names canonical paths, so an uncanonical one would never match.
normalize_repo_path() {
  local combined="$1/$2" seg
  local -a segs=() out=()
  if [[ "$combined" == *//* ]]; then echo "has an empty ('//') segment"; return 1; fi
  IFS=/ read -ra segs <<<"$combined"
  for seg in "${segs[@]}"; do
    case "$seg" in
      .) ;;
      ..)
        if [[ ${#out[@]} -eq 0 ]]; then echo "escapes the repository root"; return 1; fi
        out=("${out[@]:0:${#out[@]}-1}") ;;
      *) out+=("$seg") ;;
    esac
  done
  (IFS=/; printf '%s\n' "${out[*]}")
}

while IFS= read -r name; do
  [[ -n "$name" ]] || continue
  # Same reason as the '.'/'..' check on sourced paths below: the compare
  # names canonical paths, so an uncanonicalized one would never match.
  if [[ "/${name}/" == *"/./"* || "/${name}/" == *"/../"* ]]; then
    echo "check-pin-impact: action path has a '.'/'..' segment — refusing to guess its normalized form: ${DOTGITHUB_REPO}/.github/actions/${name}" >&2
    exit 1
  fi
  action_path=".github/actions/${name}/action.yml"
  add_consumed "$action_path"
  content="$(fetch "$NEW_SHA" "$action_path")" || {
    echo "check-pin-impact: could not fetch ${action_path} at ${NEW_SHA:0:8} — refusing to under-report" >&2
    exit 1
  }

  # Everything below resolves what a composite action's `run:` steps execute.
  # A JavaScript (`node20`, `node24`) or Docker action runs an entrypoint no
  # scripts/*.sh scan can see — with no `run:` step it would pass every check
  # below and contribute only its action.yml, so a change to its real code
  # read as an inert bump (go-kure/kure#731 rounds 8-9). Require exactly one
  # `using:` and that it is `composite`; anything else, including none,
  # fails closed.
  using_values="$(printf '%s\n' "$content" \
    | { grep -E '^[[:space:]]+using:' || true; } \
    | sed -E "s/^[[:space:]]+using:[[:space:]]*//; s/[[:space:]]+#.*\$//; s/[[:space:]]+\$//; s/^[\"']//; s/[\"']\$//")"
  if [[ "$using_values" != "composite" ]]; then
    using_values="${using_values:-<none>}"
    echo "check-pin-impact: ${action_path} at ${NEW_SHA:0:8} is not a composite action (runs.using: ${using_values//$'\n'/, }) — this script cannot resolve what a JavaScript or Docker action executes, refusing to under-report" >&2
    exit 1
  fi

  # A YAML step's first key can sit right after the list dash (`- uses:
  # foo`, no separate `- name:` line) — a shape none of the three checks
  # below recognized (found by chatgpt-codex-connector review, 2026-08-30,
  # reproduced locally: `printf '  - uses: x\n' | grep -qE
  # '^[[:space:]]*uses:'` doesn't match). Every match below allows an
  # optional `- ` list-item marker before the keyword so a step written
  # either way is recognized the same.
  #
  # Fail closed if this action.yml isn't fully accounted for by the
  # scripts/*.sh pattern below (found by the kure-bot review on
  # go-kure/kure#729, 2026-08-30): a nested `uses:` step pulls in code this
  # script does not audit at all, and a `run:` step whose content contains
  # no scripts/*.sh reference (`${{ github.action_path }}/foo.sh`, a
  # non-.sh entrypoint) would otherwise silently contribute nothing to the
  # consumed set — exactly the false "no impact" this script exists to
  # prevent. Mirrors the unrecognized-`source`-expression check below.
  if printf '%s\n' "$content" | grep -qE '^[[:space:]]*(-[[:space:]]+)?uses:'; then
    echo "check-pin-impact: ${action_path} at ${NEW_SHA:0:8} contains a nested 'uses:' step — this script does not audit external actions transitively, refusing to under-report" >&2
    exit 1
  fi
  # A single scripts/*.sh match anywhere in the file would satisfy the
  # empty-check below even if a SECOND `run:` step invokes something this
  # scan doesn't recognize (a non-.sh entrypoint) — that step's real
  # dependency would then silently contribute nothing to the consumed set
  # (found by chatgpt-codex-connector review on go-kure/kure#729,
  # 2026-08-30). This script only audits at whole-action.yml granularity, so
  # more than one `run:` step is unauditable — refuse to guess which one a
  # given scripts/*.sh reference belongs to.
  run_step_count="$(printf '%s\n' "$content" | grep -cE '^[[:space:]]*(-[[:space:]]+)?run:' || true)"
  if [[ "$run_step_count" -gt 1 ]]; then
    echo "check-pin-impact: ${action_path} at ${NEW_SHA:0:8} has ${run_step_count} 'run:' steps — this script cannot confidently attribute scripts/*.sh references to individual steps, refusing to guess" >&2
    exit 1
  fi
  # What the run: step executes is every `$GITHUB_ACTION_PATH/<rel>`
  # reference, resolved against the action's own directory
  # (.github/actions/<name>) with its `..` hops counted. Taking the
  # `scripts/*.sh` the reference merely ends in used to consume the wrong
  # file whenever the hop count was not exactly three: a nested action's
  # `../../../scripts/x.sh` runs .github/scripts/x.sh, an action-local
  # `scripts/x.sh` runs .github/actions/<name>/scripts/x.sh (go-kure/kure#731
  # review of go-kure/kure#886). Comment lines are skipped.
  code_yml="$(printf '%s\n' "$content" | { grep -vE '^[[:space:]]*#' || true; })"
  # The same directory named through the expression context
  # (`${{ github.action_path }}`, case-insensitive like every expression) is
  # not resolved here, so whatever it starts would contribute nothing to the
  # consumed set (go-kure/kure#731 review of go-kure/kure#886).
  if printf '%s\n' "$code_yml" | grep -qiE "github(\\.|\\[.)action_path"; then
    echo "check-pin-impact: ${action_path} at ${NEW_SHA:0:8} uses a github.action_path expression — only \$GITHUB_ACTION_PATH references are resolved, refusing to guess what it invokes" >&2
    exit 1
  fi
  action_path_refs="$(printf '%s\n' "$code_yml" | { grep -oE '\$\{?GITHUB_ACTION_PATH\}?/[^"'\''[:space:];&|()]+' || true; } | sort -u)"
  if [[ -z "$action_path_refs" ]] && printf '%s\n' "$code_yml" | grep -qE '^[[:space:]]*(-[[:space:]]+)?run:'; then
    echo "check-pin-impact: ${action_path} at ${NEW_SHA:0:8} has a 'run:' step but no recognized \$GITHUB_ACTION_PATH/<path>.sh reference — refusing to under-report" >&2
    exit 1
  fi
  action_dir=".github/actions/${name}"
  while IFS= read -r ref; do
    [[ -n "$ref" ]] || continue
    rel="${ref#*GITHUB_ACTION_PATH}"; rel="${rel#\}}"; rel="${rel#/}"
    # Only a .sh script can be walked for what it sources or runs in turn;
    # anything else invoked from the action (a binary, `python x.py`) is
    # unauditable here.
    if [[ "$rel" != *.sh ]]; then
      echo "check-pin-impact: ${action_path} at ${NEW_SHA:0:8} invokes ${ref}, not a .sh script — refusing to guess what the rest invoke" >&2
      exit 1
    fi
    if ! target="$(normalize_repo_path "$action_dir" "$rel")"; then
      echo "check-pin-impact: ${action_path} at ${NEW_SHA:0:8} references ${ref}, which ${target} — refusing to guess its normalized form" >&2
      exit 1
    fi
    add_consumed "$target"
    enqueue "$target"
  done <<<"$action_path_refs"
  # A single 'run:' step can itself invoke more than one command (a
  # multiline `run: |` block), and the references above only cover what is
  # reached through $GITHUB_ACTION_PATH: a second script named some other way
  # (`${{ github.action_path }}/x.sh`, a cwd-relative `scripts/x.sh`, a bare
  # `other.sh`) would silently contribute nothing to the consumed set (found
  # by chatgpt-codex-connector review, 2026-08-30). So every .sh path the
  # action.yml mentions outside a comment must be the tail of one of those
  # references — the `See scripts/<name>.sh` in a real action's description
  # is, a second invocation is not.
  while IFS= read -r tok; do
    [[ -n "$tok" ]] || continue
    covered=0
    while IFS= read -r ref; do
      [[ -n "$ref" && "$ref" == *"$tok" ]] || continue
      pre="${ref%"$tok"}"
      if [[ -z "$pre" || "$pre" == */ || "$pre" == *"\$" || "$pre" == *"\${" || "$tok" == /* ]]; then covered=1; break; fi
    done <<<"$action_path_refs"
    if [[ $covered -eq 0 ]]; then
      echo "check-pin-impact: ${action_path} at ${NEW_SHA:0:8} mentions ${tok}, which no \$GITHUB_ACTION_PATH reference accounts for — refusing to guess what the rest invoke" >&2
      exit 1
    fi
  done < <(printf '%s\n' "$code_yml" | { grep -oE '[A-Za-z0-9_./-]*[A-Za-z0-9_-]\.sh\b' || true; } | sort -u)
done <<<"$action_names"

# The forbidden-terms job's second checkout byte-compares this file against
# kure's own vendored copy independently of everything above — included here
# too so it shows in the same report; defense-in-depth; not the only thing
# that catches a change here.
guard_script="scripts/check-forbidden-terms.sh"
add_consumed "$guard_script"
enqueue "$guard_script"

# Fixed-point expansion of `source $SCRIPT_DIR/x.sh` / `. $SCRIPT_DIR/x.sh`
# one directory-relative hop at a time. Only the `$SCRIPT_DIR`/`${SCRIPT_DIR}`
# form (the shape every dot-github script uses to source a sibling, e.g.
# check-doc-sync.sh -> exact-array-member.sh) is resolved automatically —
# resolving arbitrary `source` targets in general is not something a regex
# scanner should claim to do reliably. Anything else that looks like a source
# of another file aborts the run rather than silently skipping it. The same
# holds for a sibling run as a subprocess (see below the `source` scan).

# What may precede a command word for it to count as one: line start, a
# `;`/`&`/`|` separator, or a compound-command keyword.
SEP_RE='(^|[;&|]|\b(then|do|if|elif|while|until)\b)[[:space:]]*'
# A command that looks like it runs another script: `bash`/`sh` with a .sh
# file later on the line, or a command word that is a `$VAR/`, `${VAR}/`,
# `$(...)/`, `./` or `../` path to a .sh file, the quote around the directory
# part closed before the `/` or not; optionally behind `exec`. `$(.*)` is
# greedy so a nested substitution (`$(cd "$(dirname "$0")" && pwd)/x.sh`)
# is still seen.
EXEC_CANDIDATE_RE='(exec[[:space:]]+)?((bash|sh)[[:space:]][^;&|]*\.sh\b|"?(\$\{?[A-Za-z_][A-Za-z0-9_]*\}?|\$\(.*\)|\.{1,2})"?/[^[:space:];&|]*\.sh\b)'
# The one source shape resolved: `source|. "$SCRIPT_DIR/<name>.sh"`, the whole
# line. BASH_REMATCH[2] is <name>.sh.
SOURCE_RE='^[[:space:]]*(source|\.)[[:space:]]+"?\$\{?SCRIPT_DIR\}?/([A-Za-z0-9_./-]+\.sh)"?[[:space:]]*$'
# The one invocation shape resolved: `[exec] [bash|sh] "$SCRIPT_DIR/<name>.sh"
# [args]`, the whole line, args free of separators and substitutions except
# fd redirects (`2>&1`, `>&2`, `3>&-`) and `&>file`/`&>>file`.
# BASH_REMATCH[4] is <name>.sh.
# shellcheck disable=SC2016 # the backticks are a literal character class
EXEC_RE='^[[:space:]]*(exec[[:space:]]+)?((bash|sh)[[:space:]]+)?"?\$\{?SCRIPT_DIR\}?/([A-Za-z0-9_./-]+\.sh)"?([[:space:]]+([^;&|`()[:space:]]+|[0-9]*[<>]&[0-9-]+|&>>?[^;&|`()[:space:]]+))*[[:space:]]*$'
# A use of $SCRIPT_DIR / ${SCRIPT_DIR...}.
SCRIPT_DIR_RE='\$\{?SCRIPT_DIR([^A-Za-z0-9_]|$)'
# An expression for the script's own location: BASH_SOURCE, ${0...}, or
# dirname/realpath/readlink of $0.
SELF_DIR_RE='BASH_SOURCE|\$\{0([^0-9A-Za-z_]|$)|(dirname|realpath|readlink)[^;&|]*\$\{?0\}?([^0-9A-Za-z_]|$)'

# Any assignment to SCRIPT_DIR, whatever precedes it (`export`, `readonly`,
# `declare`, `local`, another assignment).
SCRIPT_DIR_ASSIGN_RE='(^|[^A-Za-z0-9_])SCRIPT_DIR='
# The only SCRIPT_DIR definitions trusted, the whole line: the script's own
# directory as `$(dirname "$0")` or `$(cd "$(dirname "$0")" [&>/dev/null |
# >/dev/null 2>&1] && pwd)`, or either with "${BASH_SOURCE[0]}" for "$0" —
# one dirname, of the script itself — the value quoted or not, optionally
# behind `declare -r`, `readonly` or `export`. Trusting any residue-free
# definition used to accept `$(dirname "$(dirname "$0")")`, and an assignment
# with no $0 in it was never looked at (go-kure/kure#731 review of
# go-kure/kure#886).
# shellcheck disable=SC2016 # a literal $( ), matched, not expanded
SCRIPT_DIR_DIRNAME_RE='\$\(dirname ("\$0"|"\$\{BASH_SOURCE\[0\]\}")\)'
SCRIPT_DIR_VALUE_RE='(\$\(cd "'"${SCRIPT_DIR_DIRNAME_RE}"'"( &>/dev/null| >/dev/null 2>&1)? && pwd\)|'"${SCRIPT_DIR_DIRNAME_RE}"')'
SCRIPT_DIR_DEF_RE='^[[:space:]]*((declare[[:space:]]+-r|readonly|export)[[:space:]]+)?SCRIPT_DIR=("'"${SCRIPT_DIR_VALUE_RE}"'"|'"${SCRIPT_DIR_VALUE_RE}"')[[:space:]]*$'

# consume_sibling <sourced|invoked> <from-script> <target> -- add a resolved
# sibling to the consumed set and the walk. A '.'/'..' segment in the matched
# suffix (e.g. `$SCRIPT_DIR/../x.sh`) would store this uncanonicalized path in
# `consumed`, but GitHub's compare response names the canonical repo path —
# the exact-string comparison below would then never match a later change to
# the file actually sourced (found by chatgpt-codex-connector review on
# go-kure/kure#729, 2026-08-30). Only single-hop sibling resolution
# (`$SCRIPT_DIR/x.sh`) is automatic by design (see the fixed-point comment
# above); a dot-segment target is exactly the kind of shape that resolution
# deliberately doesn't claim to handle, and neither is an empty (`//`)
# segment, which would be stored as `scripts//x.sh` (go-kure/kure#731 review
# of go-kure/kure#886).
consume_sibling() {
  local kind="$1" from="$2" target="$3"
  if [[ "$target" == *//* ]]; then
    echo "check-pin-impact: ${kind} path in ${from} has an empty ('//') segment — refusing to guess its normalized form: $target" >&2
    exit 1
  fi
  if [[ "$target" == *".."* || "$target" == *"/./"* ]]; then
    echo "check-pin-impact: ${kind} path in ${from} has a '.'/'..' segment — refusing to guess its normalized form: $target" >&2
    exit 1
  fi
  add_consumed "$target"
  enqueue "$target"
}

i=0
while [[ ${#queue[@]} -gt 0 ]]; do
  i=$((i + 1))
  if [[ $i -gt 200 ]]; then
    echo "check-pin-impact: source-resolution did not reach a fixed point after 200 steps — aborting" >&2
    exit 1
  fi
  script="${queue[0]}"
  queue=("${queue[@]:1}")
  content="$(fetch "$NEW_SHA" "$script")" || {
    echo "check-pin-impact: could not fetch ${script} at ${NEW_SHA:0:8} — refusing to under-report" >&2
    exit 1
  }
  script_dir="$(dirname "$script")"

  # Lines that look like they source another file at all — not just at the
  # very start of the line: a `source`/`.` inside a compound command (`if
  # cond; then source "$SCRIPT_DIR/x.sh"; fi`) was previously invisible to
  # this check entirely, never even reaching the "unrecognized expression"
  # abort below — a silent miss, not a fail-closed one (found by
  # chatgpt-codex-connector review on go-kure/kure#729, 2026-08-30).
  # Comment lines are excluded first: without that, the English word
  # "source" inside a comment or string (e.g. real content in
  # check-doc-sync.sh: `fail "extra_mounts source not found: $src"`) would
  # false-positive as a candidate and abort the run on nothing — confirmed
  # against that exact file. Restricting the "preceded by" side to a real
  # separator (line start, `;`/`&`/`|`, or `then`/`do`) rather than any
  # whitespace keeps that same false-positive class out of non-comment code
  # too, at the cost of not catching every conceivable compound shape — an
  # unrecognized one still aborts via the branch below, so under-reporting
  # isn't the failure mode this trades away.
  #
  # A sibling run as a subprocess rather than sourced (`bash
  # "$SCRIPT_DIR/helper.sh"`, `sh ...`, `exec ...`, or the path itself as the
  # command word) executes just the same, but the walk only followed
  # `source`/`.` — the helper never reached the consumed set, so a change to
  # it read as inert (go-kure/kure#731 round 8). And a scan keyed on the
  # command position still missed every call that does not sit right after a
  # separator: `if ! bash ...`, `command`/`env`/`nice`/`timeout` wrappers,
  # `FOO=1 bash ...`, `{ ...; }`, `( ... )`, `$( ... )`, `cat x | bash`, a
  # path stored in a variable first, `find -exec`, a non-.sh sibling
  # (go-kure/kure#731 review of go-kure/kure#886). So the rule is keyed on the
  # token instead: every line that uses $SCRIPT_DIR must be exactly the
  # whole-line source or subprocess form (SOURCE_RE / EXEC_RE, one $SCRIPT_DIR
  # on it), every `SCRIPT_DIR=` assignment must be exactly one of the trusted
  # definitions (SCRIPT_DIR_DEF_RE) — otherwise $SCRIPT_DIR could name any
  # directory — and every other line that computes the script's own directory
  # (dirname "$0", ${0%/*}, BASH_SOURCE) is refused — otherwise the directory
  # could be carried under another name.
  # The separator-anchored source and subprocess scans stay as a backstop for
  # `source "$OTHER/x.sh"`, `./x.sh`, `bash x.sh` and the like. Keying on a
  # bare `bash` word is deliberately avoided: real scripts run generated
  # content with `bash "$GEN"`.
  code_lines="$(printf '%s\n' "$content" | { grep -vE '^[[:space:]]*#' || true; })"
  candidate_lines="$(printf '%s\n' "$code_lines" | { grep -E "${SEP_RE}(source|\\.)[[:space:]]|${SEP_RE}${EXEC_CANDIDATE_RE}|${SCRIPT_DIR_RE}|${SCRIPT_DIR_ASSIGN_RE}|${SELF_DIR_RE}" || true; })"
  while IFS= read -r line; do
    [[ -n "$line" ]] || continue
    script_dir_uses="$(printf '%s\n' "$line" | { grep -oE "$SCRIPT_DIR_RE" || true; } | grep -c . || true)"
    self_dir_use=0
    printf '%s\n' "$line" | grep -qE "$SELF_DIR_RE" && self_dir_use=1
    assigns=0
    [[ "$line" =~ $SCRIPT_DIR_ASSIGN_RE ]] && assigns=1
    if [[ "$script_dir_uses" -eq 1 && $self_dir_use -eq 0 && $assigns -eq 0 && "$line" =~ $SOURCE_RE ]]; then
      consume_sibling sourced "$script" "${script_dir}/${BASH_REMATCH[2]}"
      continue
    fi
    if [[ "$script_dir_uses" -eq 1 && $self_dir_use -eq 0 && $assigns -eq 0 && "$line" =~ $EXEC_RE ]]; then
      consume_sibling invoked "$script" "${script_dir}/${BASH_REMATCH[4]}"
      continue
    fi
    if [[ $assigns -eq 1 && "$line" =~ $SCRIPT_DIR_DEF_RE ]]; then
      continue
    fi
    if printf '%s\n' "$line" | grep -qE "${SEP_RE}(source|\\.)[[:space:]]"; then
      what="source expression"
    elif printf '%s\n' "$line" | grep -qE "${SEP_RE}${EXEC_CANDIDATE_RE}"; then
      what="sibling-script invocation"
    elif [[ $assigns -eq 1 ]]; then
      what="SCRIPT_DIR definition"
    elif [[ "$script_dir_uses" -gt 0 ]]; then
      what="use of \$SCRIPT_DIR"
    else
      what="script-directory expression"
    fi
    echo "check-pin-impact: unrecognized ${what} in ${script} — refusing to guess whether it needs resolving:" >&2
    echo "  $line" >&2
    exit 1
  done <<<"$candidate_lines"
done

# --- Fetch the compare and intersect ---
auth_args=()
[[ -n "${GITHUB_TOKEN:-}" ]] && auth_args=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
compare_json="$(curl -fsSL --connect-timeout 10 --max-time 30 \
  -H "Accept: application/vnd.github+json" "${auth_args[@]}" \
  "https://api.github.com/repos/${DOTGITHUB_REPO}/compare/${OLD_SHA}...${NEW_SHA}")" || {
  echo "check-pin-impact: GitHub compare API request failed" >&2
  exit 1
}

# The three-dot compare above diffs from merge-base(OLD_SHA, NEW_SHA) to
# NEW_SHA, not literally OLD_SHA to NEW_SHA — GitHub's compare REST endpoint
# has no two-dot form to fall back to (confirmed: it 404s). If NEW_SHA is an
# ancestor of OLD_SHA (a hand-edited pin rollback — pins to go-kure/.github
# are maintained by hand, not just bumped forward by Renovate) the merge base
# is NEW_SHA itself, so `files` comes back empty and this would silently
# report "inert" regardless of what actually changed between the two — the
# exact false negative this script exists to prevent (kure-bot review
# finding, 2026-08-30; reproduced against real go-kure/.github history: a
# 5-commit rollback pair reports `files: 0`,
# `status: "behind"`). Only a strict fast-forward (NEW_SHA a descendant of
# OLD_SHA) makes the three-dot form correct, so require `status: "ahead"`.
compare_status="$(printf '%s' "$compare_json" | grep -oE '"status":[[:space:]]*"[^"]*"' | head -1 | sed -E 's/^"status":[[:space:]]*"//; s/"$//')"
if [[ "$compare_status" != "ahead" ]]; then
  echo "check-pin-impact: OLD_SHA...NEW_SHA compare status is '${compare_status:-unknown}', not 'ahead' — NEW_SHA is not a strict descendant of OLD_SHA (a pin rollback, or unrelated history), so this compare would under-report; refusing to guess" >&2
  exit 1
fi

changed_count="$(printf '%s' "$compare_json" | { grep -o '"filename"' || true; } | wc -l)"
if [[ "$changed_count" -ge 295 ]]; then
  echo "check-pin-impact: compare reports $changed_count changed files, near GitHub's ~300-file pagination cap — this script does not paginate and would under-report; verify by hand" >&2
  exit 1
fi

# `|| true` on the grep: OLD_SHA != NEW_SHA was already checked above, so a
# real diff is expected, but a zero-match grep here would otherwise make the
# whole pipeline exit 1 under `pipefail` (sed/sort downstream exit 0 on empty
# input) and abort this assignment under `set -e` — the same trap every
# `{ grep ... || true; }` above guards against.
changed_files="$(printf '%s' "$compare_json" \
  | { grep -oE '"filename":[[:space:]]*"[^"]*"' || true; } \
  | sed -E 's/^"filename":[[:space:]]*"//; s/"$//' \
  | sort -u)"

echo ""
echo "Consumed paths (${#consumed[@]}):"
for p in "${!consumed[@]}"; do echo "  $p"; done | sort
echo ""
echo "Changed in ${OLD_SHA:0:8}..${NEW_SHA:0:8} ($changed_count file(s)):"
printf '%s\n' "$changed_files" | sed 's/^/  /'

affected=()
for p in "${!consumed[@]}"; do
  if printf '%s\n' "$changed_files" | grep -qxF "$p"; then
    affected+=("$p")
  fi
done

echo ""
if [[ ${#affected[@]} -gt 0 ]]; then
  echo "check-pin-impact: $(( ${#affected[@]} )) consumed path(s) changed:" >&2
  printf '  %s\n' "${affected[@]}" >&2
  echo "This pin bump touches code kure actually executes — review the diff before merging." >&2
  # Maintainer override: add the 'pin-impact-ack' label to the PR once the
  # diff above has been reviewed and judged fine. Deliberately requires a
  # human action visible on the PR (a label), not a script flag anyone could
  # pass — this script's whole job is to make an unreviewed impact
  # unmergeable, not to make a reviewed one unmergeable too (P1 gap found by
  # chatgpt-codex-connector review on go-kure/kure#729, 2026-08-30: no
  # acknowledgement path existed at all).
  if [[ "${PIN_IMPACT_ACK:-false}" == "true" ]]; then
    echo "check-pin-impact: ACKNOWLEDGED via 'pin-impact-ack' label — merging despite the above; a maintainer reviewed this impact."
    exit 0
  fi
  echo "check-pin-impact: FAIL — add the 'pin-impact-ack' label after review to merge anyway." >&2
  exit 1
fi

echo "check-pin-impact: OK — no consumed path changed; pin refresh is inert."
