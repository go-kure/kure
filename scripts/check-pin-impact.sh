#!/usr/bin/env bash
# check-pin-impact.sh — render, and gate on, the real impact of a go-kure/.github
# pin bump before it merges.
#
# .github/workflows/*.yml pin go-kure/.github by commit SHA in two shapes: the
# `uses: go-kure/.github/.github/actions/<name>@<sha>` form (one per composite
# action consumed) and the `repository: go-kure/.github` / `ref: <sha>` checkout
# in the forbidden-terms job (byte-compares the vendored guard, see
# vendor-guard.sh). Renovate bumps every occurrence to the same new SHA in one
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
# Resolved automatically: `uses:` action paths with nested or dotted subpaths
# (`actions/group/check@`, `actions/check.v2@`); a `repository:`/`ref:`
# checkout whatever the order of its step's keys; sibling scripts reached from
# a consumed script through `source`/`.` or run as a subprocess (`bash`/`sh`/
# `exec`/direct), each only in the `$SCRIPT_DIR/<name>.sh` form.
#
# Fails closed: any construct this script cannot confidently resolve (an
# ambiguous/inconsistent pin, an action that is not `runs.using: composite`
# (JavaScript and Docker actions execute code no scripts/*.sh scan can see),
# a '.'/'..' segment in an action or sibling path, a `source` or
# sibling-script invocation it doesn't recognize the shape of, a compare
# response near GitHub's pagination cap) aborts loudly rather than silently
# under-reporting the consumed set. A false "no impact" is the one failure
# mode this script exists to prevent.
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
    -h|--help) sed -n '2,53p' "$0"; exit 0 ;;
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

# `uses: go-kure/.github/.github/actions/<subpath>@` prefix, as an ERE. The
# subpath may be nested (`group/check`) or dotted (`check.v2`): a
# single-segment `[A-Za-z0-9_-]+` pattern silently skipped both, dropping
# their SHA from the consistency check and their scripts from the consumed
# set (go-kure/kure#731 round 9). A '.'/'..' segment is matched here and
# refused where the action is resolved.
USES_PREFIX_RE="${DOTGITHUB_REPO//./\\.}/\\.github/actions/[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)*@"

# Every `go-kure/.github/.github/actions/<subpath>@<sha>` and every `ref: <sha>`
# of a step that checks out `repository: go-kure/.github`, across the given
# file's content (read from stdin). One SHA per line; the action-name form's
# name is not needed here, only its SHA.
extract_shas() {
  # Read stdin into a variable first: the `uses:@sha` pipeline below would
  # otherwise drain stdin, leaving the `ref:` awk pass with nothing to read —
  # a function's stdin is one stream, not re-readable per command.
  local content
  content="$(cat)"

  # `|| true` on each: under `pipefail`, a pipeline with zero matches upstream
  # returns the upstream grep's exit-1 even when the downstream command
  # succeeds on empty input — expected whenever a file uses only one of the
  # two pin forms (or neither), not an error; must not abort the caller's
  # `set -e` context (this function's output commonly feeds a `$(...)`
  # assignment, not just a process substitution).
  printf '%s' "$content" \
    | { grep -oE "${USES_PREFIX_RE}[0-9a-f]{40}" || true; } \
    | { grep -oE '[0-9a-f]{40}$' || true; }
  # `repository: go-kure/.github` checkouts: the `ref:` of the same step is
  # the pin, whatever the order of the step's keys. This used to take only a
  # `ref:` on the line right after `repository:`, so a `path:` between them or
  # a `ref:` written first silently dropped the SHA from the consistency check
  # (go-kure/kure#731 round 9). A step is a list item: it runs from its `-`
  # line until the next line indented no deeper than that dash, so a nested
  # list inside the step stays part of it and another step's `ref:` is never
  # attributed to this checkout. A non-SHA `ref:` (an expression, like the
  # forbidden-terms checkout's) is not a pin and yields nothing. Still refuses
  # via the consistency check below rather than guessing if this ever matches
  # more than intended.
  printf '%s\n' "$content" | awk -v repo="$DOTGITHUB_REPO" -v q="'" '
    function flush() {
      if (is_repo && sha != "") print sha
      in_step = 0; is_repo = 0; sha = ""
    }
    function scalar(v) {
      sub(/[[:space:]]+#.*$/, "", v); sub(/[[:space:]]+$/, "", v)
      if (v ~ /^".*"$/ || v ~ ("^" q ".*" q "$")) v = substr(v, 2, length(v) - 2)
      return v
    }
    /^[[:space:]]*(#|$)/ { next }
    {
      match($0, /^ */); ind = RLENGTH
      if ($0 ~ /^ *-([[:space:]]|$)/) {
        if (!in_step || ind <= step_ind) { flush(); in_step = 1; step_ind = ind }
      } else if (in_step && ind <= step_ind) {
        flush()
      }
      if (!in_step) next
      line = $0
      sub(/^ *(-[[:space:]]+)?/, "", line)
      if (line ~ /^repository:/) {
        sub(/^repository:[[:space:]]*/, "", line)
        if (scalar(line) == repo) is_repo = 1
      } else if (line ~ /^ref:/) {
        sub(/^ref:[[:space:]]*/, "", line)
        line = scalar(line)
        if (line ~ /^[0-9a-f]+$/ && length(line) == 40) sha = line
      }
    }
    END { flush() }
  '
  return 0
}

# All distinct action subpaths referenced, across the given file's content
# (stdin). Action names, not SHAs — stable across a pin bump. The strip is
# anchored to the repo prefix grep -o emits, so a nested subpath that itself
# contains an `actions/` segment keeps it.
extract_action_names() {
  grep -oE "$USES_PREFIX_RE" | sed -E "s#^${DOTGITHUB_REPO//./\\.}/\\.github/actions/##; s/@\$//"
}

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

# --- Discover action names from the working tree (stable across old/new) ---
all_action_names=""
for f in "${WORKFLOW_FILES[@]}"; do
  [[ -f "$f" ]] || continue
  # `|| true`: extract_action_names ends in `sed`, but under `pipefail` a
  # zero-match `grep -oE` upstream still makes the pipeline's status grep's
  # 1 even though sed itself exits 0 -- a workflow file with no go-kure/.github
  # action reference (release-*.yml, manage-docs.yml, pr-review.yml) is
  # expected, not an error, and must not abort this `$(...)` assignment
  # under `set -e`.
  all_action_names="$all_action_names
$(extract_action_names <"$f" || true)"
done
action_names="$(printf '%s\n' "$all_action_names" | grep -v '^$' | sort -u)"
if [[ -z "$action_names" ]]; then
  echo "check-pin-impact: no go-kure/.github action references found in ${WORKFLOW_FILES[*]}" >&2
  exit 1
fi

# --- Resolve OLD/NEW SHAs ---
if [[ -n "$BASE_REF" ]]; then
  new_shas=()
  old_shas=()
  for f in "${WORKFLOW_FILES[@]}"; do
    [[ -f "$f" ]] || continue
    while IFS= read -r s; do [[ -n "$s" ]] && new_shas+=("$s"); done < <(extract_shas <"$f")
    base_content="$(git show "${BASE_REF}:${f}" 2>/dev/null || true)"
    [[ -n "$base_content" ]] || continue
    while IFS= read -r s; do [[ -n "$s" ]] && old_shas+=("$s"); done < <(printf '%s\n' "$base_content" | extract_shas)
  done
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
  scripts_found="$(printf '%s\n' "$content" | { grep -oE 'scripts/[A-Za-z0-9_./-]+\.sh' || true; } | sort -u)"
  if [[ -z "$scripts_found" ]] && printf '%s\n' "$content" | grep -qE '^[[:space:]]*(-[[:space:]]+)?run:'; then
    echo "check-pin-impact: ${action_path} at ${NEW_SHA:0:8} has a 'run:' step but no recognized scripts/*.sh reference — refusing to under-report" >&2
    exit 1
  fi
  # A single 'run:' step can itself invoke more than one command (a
  # multiline `run: |` block) — the check above only asks "does this file
  # mention scripts/*.sh at all", so a second, unrecognized executable
  # invoked in the same block (`${{ github.action_path }}/tool`, a bare
  # `python other.py`) would silently contribute nothing to the consumed set
  # (found by chatgpt-codex-connector review, 2026-08-30). Every dot-github
  # action.yml invokes its script(s) the same
  # way — `$GITHUB_ACTION_PATH/../../../scripts/<name>.sh` — confirmed
  # against every action.yml this script currently resolves, so a mismatch
  # between "how many $GITHUB_ACTION_PATH-relative references exist" and
  # "how many of them look like a recognized scripts/*.sh path" means
  # something this scan doesn't recognize is also being invoked from the
  # same action-relative root.
  action_path_refs="$(printf '%s\n' "$content" | { grep -oE '\$\{?GITHUB_ACTION_PATH\}?/[^"[:space:]]+' || true; } | sort -u)"
  action_path_ref_count="$(printf '%s\n' "$action_path_refs" | grep -c . || true)"
  scripts_found_count="$(printf '%s\n' "$scripts_found" | grep -c . || true)"
  if [[ "$action_path_ref_count" -ne "$scripts_found_count" ]]; then
    echo "check-pin-impact: ${action_path} at ${NEW_SHA:0:8} has ${action_path_ref_count} \$GITHUB_ACTION_PATH reference(s) but only ${scripts_found_count} recognized scripts/*.sh match(es) — refusing to guess what the rest invoke" >&2
    exit 1
  fi
  while IFS= read -r script; do
    [[ -n "$script" ]] || continue
    add_consumed "$script"
    enqueue "$script"
  done <<<"$scripts_found"
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
# The one invocation shape resolved: `[exec] [bash|sh] "$SCRIPT_DIR/<name>.sh"
# [args]`, the whole line, args free of separators and substitutions.
# BASH_REMATCH[4] is <name>.sh.
EXEC_RE='^[[:space:]]*(exec[[:space:]]+)?((bash|sh)[[:space:]]+)?"?\$\{?SCRIPT_DIR\}?/([A-Za-z0-9_./-]+\.sh)"?([[:space:]]+[^;&|`()]*)?$'

# consume_sibling <sourced|invoked> <from-script> <target> -- add a resolved
# sibling to the consumed set and the walk. A '.'/'..' segment in the matched
# suffix (e.g. `$SCRIPT_DIR/../x.sh`) would store this uncanonicalized path in
# `consumed`, but GitHub's compare response names the canonical repo path —
# the exact-string comparison below would then never match a later change to
# the file actually sourced (found by chatgpt-codex-connector review on
# go-kure/kure#729, 2026-08-30). Only single-hop sibling resolution
# (`$SCRIPT_DIR/x.sh`) is automatic by design (see the fixed-point comment
# above); a dot-segment target is exactly the kind of shape that resolution
# deliberately doesn't claim to handle.
consume_sibling() {
  local kind="$1" from="$2" target="$3"
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
  code_lines="$(printf '%s\n' "$content" | { grep -vE '^[[:space:]]*#' || true; })"
  candidate_lines="$(printf '%s\n' "$code_lines" | { grep -E "${SEP_RE}(source|\\.)[[:space:]]" || true; })"
  while IFS= read -r line; do
    [[ -n "$line" ]] || continue
    if [[ "$line" =~ ^[[:space:]]*(source|\.)[[:space:]]+\"?\$\{?SCRIPT_DIR\}?/([A-Za-z0-9_./-]+\.sh)\"?[[:space:]]*$ ]]; then
      consume_sibling sourced "$script" "${script_dir}/${BASH_REMATCH[2]}"
    else
      echo "check-pin-impact: unrecognized source expression in ${script} — refusing to guess whether it needs resolving:" >&2
      echo "  $line" >&2
      exit 1
    fi
  done <<<"$candidate_lines"

  # A sibling run as a subprocess rather than sourced (`bash
  # "$SCRIPT_DIR/helper.sh"`, `sh ...`, `exec ...`, or the path itself as the
  # command word) executes just the same, but the walk above only followed
  # `source`/`.` — the helper never reached the consumed set, so a change to
  # it read as inert (go-kure/kure#731 round 8). Detected broadly (after the
  # same separators, a `bash`/`sh` line naming a .sh file, or a command word
  # that is a `$VAR/`, `$(...)/`, `./` or `../` path to a .sh file) and
  # resolved narrowly, under the same rules as `source`: only the whole-line
  # `$SCRIPT_DIR/<name>.sh` form, with arguments free of command separators
  # and substitutions, is followed. Anything else detected aborts —
  # `"$(dirname "$0")/x.sh"` included, since a sourced file's `$0` is its
  # caller's.
  candidate_lines="$(printf '%s\n' "$code_lines" | { grep -E "${SEP_RE}${EXEC_CANDIDATE_RE}" || true; })"
  while IFS= read -r line; do
    [[ -n "$line" ]] || continue
    if [[ "$line" =~ $EXEC_RE ]]; then
      consume_sibling invoked "$script" "${script_dir}/${BASH_REMATCH[4]}"
    else
      echo "check-pin-impact: unrecognized sibling-script invocation in ${script} — refusing to guess whether it needs resolving:" >&2
      echo "  $line" >&2
      exit 1
    fi
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
# input) and abort this assignment under `set -e` — same trap as
# extract_action_names above.
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
