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
#     <40-hex>` in the `with:` mapping of a block-style `repository:
#     go-kure/.github` checkout step, in any key order; the repository name
#     case-insensitively and with or without a trailing `.git` (actions/
#     checkout clones https://github.com/<repository>), the hex in either
#     case. A key is read as YAML reads it: `"uses":`, `'ref':` and
#     `repository :` are the plain keys, in a workflow and an action.yml.
#   - per action: each `$GITHUB_ACTION_PATH/<rel>.sh` (or
#     `${GITHUB_ACTION_PATH}/<rel>.sh`) in its one `run:` step, one whole
#     word, resolved against .github/actions/<subpath>/ with `..` hops
#     counted.
#   - per script, transitively: a whole line `source|. "$SCRIPT_DIR/<name>.sh"`
#     or `[exec] [bash|sh] "$SCRIPT_DIR/<name>.sh" [args]` (args without
#     separators or substitutions; fd redirects, `&>file` and `&>>file`
#     allowed), resolved against the script's own directory, where SCRIPT_DIR
#     is defined as exactly `$(dirname "$0")`, `$(cd "$(dirname "$0")"
#     [&>/dev/null | >/dev/null [2>&1] | 2>/dev/null] && pwd [-P])` or either
#     with "${BASH_SOURCE[0]}" for "$0", optionally behind `declare -r`,
#     `readonly` or `export`; `cd --`, `dirname --`, `1>` for `>` and a space
#     after `&>` or `>` are the same definition.
#   - the run-when-executed guard `if [[ "${BASH_SOURCE[0]}" == "$0" ]]`
#     (`!=`, `"${0}"`, `; then`), alone on its line, names no directory.
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
#     call at a non-SHA ref (below), may carry none. A checkout step with a
#     `ref:` anywhere but its `with:` mapping (under `env:`, in a block
#     scalar body, at the step's own level), more than one `ref:` in any
#     letter case, or more than one `with:`.
#   - workflow YAML the line scan cannot read, whatever it names: a
#     `uses:`/`repository:` value not whole on its own line (empty, continued
#     on the next line, a block scalar, an alias, anchor, tag or flow
#     collection, a quoted value with an escape — `\` in double quotes, `''`
#     in single quotes — or with no closing quote); a `repository:` given as
#     an expression; a `uses`/`repository` key that does not start its line
#     (a flow mapping, a tagged or anchored key) or is not in lower case; a
#     quoted key with an escape sequence; a `? ` complex key. A `uses:`
#     expression is refused only when it names go-kure/.github: GitHub does
#     not evaluate expressions in `uses:`.
#   - actions: `runs.using` other than composite (JavaScript and Docker
#     actions execute code no scan here can see), a `using` in a flow mapping
#     or not in lower case included; a nested `uses:`, flow-mapping, quoted
#     or in any letter case; more than one `run:` step, `RUN:` and the like
#     counted; a `github.action_path` expression; $GITHUB_ACTION_PATH other
#     than as one whole `$GITHUB_ACTION_PATH/<path>` or
#     `${GITHUB_ACTION_PATH}/<path>` word — the path of `[A-Za-z0-9_./-]`,
#     then at most a closing quote, then whitespace, `;&|)<>` or the line end
#     (so not reassigned, `${GITHUB_ACTION_PATH%/*}`, a bare trailing `/`, a
#     path after a closing quote or a suffix after it); the runner's
#     `_actions` directory by path; a quoted key with an escape sequence or a
#     `? ` complex key; a `run:` step with no `$GITHUB_ACTION_PATH` script; a
#     non-.sh `$GITHUB_ACTION_PATH` target; a .sh path mentioned that no
#     `$GITHUB_ACTION_PATH` reference accounts for; a '.'/'..' action subpath
#     segment; a path that climbs above the repository root or has an empty
#     (`//`) segment.
#   - scripts: any line using `$SCRIPT_DIR` that is not exactly one of the two
#     followed forms; any `SCRIPT_DIR=` assignment (with any prefix) that is
#     not exactly one of the definition shapes above; the word SCRIPT_DIR in
#     any other form (`SCRIPT_DIR+=`, `SCRIPT_DIR[0]=`, `read SCRIPT_DIR`, `for
#     SCRIPT_DIR in`, `n=SCRIPT_DIR`); name indirection — `${!name}` (not the
#     array-keys `${!name[@]}`), a `declare -n`/`local -n`/`typeset -n`
#     nameref, `eval`; any other line computing the script's own directory
#     (dirname "$0", ${0%/*}, BASH_SOURCE, BASH_ARGV, a positional slice
#     `${@:...}`/`${*:...}`), or naming `$0` at all outside a message to
#     stderr (`echo "usage: $0 ..." >&2` or `1>&2`), a `sed -n '<lines>p'
#     "$0"` read of the script itself or an awk record (`f($0`, `, $0`, ` =
#     $0`, `$0 ~`, `$0 !~`) — `x=$0`, `a=($0)`, `printf -v`, `read <<<"$0"`,
#     a function argument or a message to another fd carry the directory
#     under another name; a line naming $GITHUB_ACTION_PATH or the runner's
#     `_actions` directory; any other `source`/`.`, or `bash`/`sh`/path
#     invocation of a .sh file, at a command position; a '.'/'..' or empty
#     segment in a sibling path; a sibling that cannot be fetched.
#   - where $SCRIPT_DIR is not the script's own directory: a file sourced
#     from a script in another directory that names SCRIPT_DIR at all (it
#     shares its caller's); a script run as its own process that uses
#     $SCRIPT_DIR before a trusted definition (it reads an inherited one); a
#     relative definition, `$(dirname "$0")`, in any walked script while any
#     walked script changes the working directory (`cd`, `pushd`, `popd`
#     outside a `$(cd` or `(cd` subshell).
#   - the compare: a status other than `ahead`; a file count near GitHub's
#     pagination cap.
#
# Not covered, knowingly:
#   - job-level reusable-workflow calls at a non-SHA ref, `uses:
#     go-kure/.github/.github/workflows/<file>.yml@<ref>`: they run at their
#     own ref (kure's call @main), not at the action pin, so no pin bump
#     changes them and this gate neither pins nor audits them.
#   - a sibling reached without naming $SCRIPT_DIR, $0 or the checkout — a
#     hard-coded absolute path, a name found on PATH, or a path assembled at
#     run time (/proc/self, a variable filled from a file).
#   - definition order is line order: a trusted SCRIPT_DIR definition inside
#     a function or a branch that never runs still counts as defining it for
#     the lines below.
#   - a `,$0` outside awk is taken for an awk record (`for p in {x,$0}`).
#   - a `$0` message to stderr read back: the stderr exemption assumes
#     stderr is not redirected into a file or a capture (`exec 2>f`, `$(f
#     2>&1)`) that the script then reads.
#   - the script's path from the call stack, `caller`: it names no `$0`.
#   - false aborts, refused although harmless: a directory other than the
#     script's own derived from it, such as `ROOT=$(cd "$(dirname "$0")/.."
#     && pwd)` — following it would mean tracking an arbitrary variable
#     through every script that sources or inherits it, and real scripts
#     reuse such names for argument-derived paths; `uses:` or `repository:`
#     text anywhere in a single-line workflow value (`run: grep -n
#     "repository:" ci.yml`, `with: { repository: foo/bar }`), and `ref:`
#     text anywhere in a checkout step of go-kure/.github; `${!prefix@}`,
#     `export SCRIPT_DIR` or `readonly SCRIPT_DIR` on a line of its own.
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
    -h|--help) sed -n '2,167p' "$0"; exit 0 ;;
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
# `repository: go-kure/.github` checkouts: the `ref:` key of the step's own
# `with:` mapping is the pin, whatever the order of the step's keys (it used
# to be only a `ref:` on the very next line, round 9). The repository value
# is matched case-insensitively and with or without a trailing `.git`:
# actions/checkout clones https://github.com/<value>, which names the same
# repository either way. A step is a list item: it runs from its `-` line
# until the next line indented no deeper than that dash, so a nested list
# inside the step stays part of it and another step's `ref:` is never
# attributed to this checkout. Within the step, a key stack tracks the mapping
# each key sits in; a `ref:` anywhere else (under `env:`, at the step's own
# level, in a block scalar body, in a flow mapping), a second `ref:` in any
# letter case (action inputs are case-insensitive) or a second `with:` makes
# the checkout refused — any of them used to be read as the pin, the last one
# winning, so a decoy at the new SHA hid the real checkout (go-kure/kure#888
# review). A `ref:` that is exactly
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
# Keys are read as YAML reads them (go-kure/kure#888): a quoted `"uses":` or
# `'repository':`, or one with a space before its colon, is the plain key. A
# quoted key with an escape sequence, which could spell any key, and a `? `
# complex key are refused. A `uses:`/`repository:` value must be readable
# whole from its own line — not continued on the next line, not a block
# scalar, alias, anchor, tag or flow collection, not a quoted value with an
# escape (`\` in double quotes, `''` in single quotes) or with no closing
# quote — and a `repository:` may not be an expression: any of them could
# name go-kure/.github without spelling it here, so each is refused whatever
# it names. So is a `uses`/`repository` key anywhere else on a line (a flow
# mapping, a tagged key) or in another letter case, mention or not; the
# `uses:`/`repository:` text inside a single-line value (`run: grep -n
# "repository:" ci.yml`) is refused the same way, a known false abort. A
# `uses:` expression is not refused as such (GitHub does not evaluate
# expressions in `uses:`); it is refused only when it names go-kure/.github
# and yields no pin. Lines inside a block scalar
# (a `run: |` body) are text, and are held only to the checks above this
# paragraph.
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
        if (with_n > 1) bad(repo_nr, repo_text " (more than one with: in this checkout step)")
        else if (stray_ref || ref_n > 1) bad(repo_nr, repo_text " (a ref: outside the step'"'"'s with: mapping, or more than one ref:)")
        else if (sha != "") print "pin " sha
        else if (!ref_expr) bad(repo_nr, repo_text " (no 40-hex ref: in this checkout step)")
      }
      in_step = 0; is_repo = 0; sha = ""; ref_expr = 0
      sp = 0; step_keycol = -1; ref_n = 0; stray_ref = 0; with_n = 0
    }
    # A value this scan can take whole from its own line.
    function readable(v) {
      if (v == "" || v ~ /^[|>*&!{]/ || v ~ /^\[/) return 0
      if (v ~ /^"/) return (v ~ /^"[^"\\]*"$/)
      if (v ~ ("^" q)) return (v ~ ("^" q "[^" q "]*" q "$"))
      return 1
    }
    BEGIN { prefix = repo "/.github/actions/"; blk = 0; sp = 0; step_keycol = -1 }
    /^[[:space:]]*(#|$)/ { next }
    {
      code = $0
      sub(/[[:space:]]+#.*$/, "", code)
      lc = tolower(code)
      mentions = (lc ~ ("(^|[^a-z0-9_.-])go-kure/[.]github([.]git)?([^a-z0-9_.-]|$)"))
      keytok = (lc ~ ("(^|[^a-z0-9_-])[\"" q "]?(uses|repository)[\"" q "]?[[:space:]]*:"))
      reftok = (lc ~ ("(^|[^a-z0-9_-])[\"" q "]?ref[\"" q "]?[[:space:]]*:"))

      match(code, /^ */); ind = RLENGTH
      # A block scalar body: every line indented deeper than the key (or the
      # list dash) that opened it.
      in_blk = (blk && ind > blk_ind)
      if (!in_blk) blk = 0
      if (code ~ /^ *-([[:space:]]|$)/) {
        if (!in_step || ind <= step_ind) { flush(); in_step = 1; step_ind = ind }
      } else if (in_step && ind <= step_ind) {
        flush()
      }

      line = code
      sub(/^ *(-[[:space:]]+)?/, "", line)
      if (!in_blk) {
        keycol = length(code) - length(line)
        if (line ~ /^\?([[:space:]]|$)/) { bad(NR, $0 " (a complex key this scan cannot read)"); next }
        if (line ~ /"[^"]*\\[^"]*"[[:space:]]*:([[:space:]]|$)/) { bad(NR, $0 " (a quoted key with an escape sequence)"); next }
        iskey = 0; key = ""
        if (match(line, /^"[^"]*"[[:space:]]*:([[:space:]]|$)/) \
            || match(line, ("^" q "[^" q "]*" q "[[:space:]]*:([[:space:]]|$)")) \
            || match(line, /^[A-Za-z0-9_][A-Za-z0-9_.-]*[[:space:]]*:([[:space:]]|$)/)) {
          iskey = 1
          key = substr(line, 1, RLENGTH)
          val = substr(line, RLENGTH + 1)
          sub(/[[:space:]]*:[[:space:]]*$/, "", key)
          sub(/^[[:space:]]+/, "", val); sub(/[[:space:]]+$/, "", val)
          if (key ~ /^"/ || key ~ ("^" q)) key = substr(key, 2, length(key) - 2)
          if (key == "uses" || key == "repository" || key == "ref") line = key ": " val
          if (in_step) {
            # Where this key sits in the step: the stack holds the keys it
            # is nested under, the step'"'"'s own keys at step_keycol.
            lk = tolower(key)
            if (step_keycol < 0) step_keycol = keycol
            while (sp > 0 && st_ind[sp] >= keycol) sp--
            if (sp == 0 && lk == "with") with_n++
            if (lk == "ref") {
              ref_n++
              if (key == "ref" && sp == 1 && st_key[1] == "with" && st_ind[1] == step_keycol) {
                v = scalar(val)
                if (is_hex40(tolower(v))) sha = tolower(v)
                else if (v ~ /^\$\{\{ *steps\.[A-Za-z0-9_-]+\.outputs\.[A-Za-z0-9_-]+ *\}\}$/) ref_expr = 1
              } else {
                stray_ref = 1
              }
            } else if (reftok) {
              ref_n++; stray_ref = 1
            }
            sp++; st_ind[sp] = keycol; st_key[sp] = lk
          }
          if ((key == "uses" || key == "repository") && !readable(val)) {
            bad(NR, $0 " (a value this scan cannot read whole from its line)"); next
          }
          if (key == "repository" && val ~ /\$\{\{/) { bad(NR, $0 " (a repository given as an expression)"); next }
          if (val ~ /^[|>][-+0-9]*$/) { blk = 1; blk_ind = keycol }
        } else if (line ~ /^[|>][-+0-9]*$/) {
          blk = 1; blk_ind = ind
        }
        if (in_step && !iskey && reftok) { ref_n++; stray_ref = 1 }
        if (keytok && !(key == "uses" || key == "repository")) { bad(NR, $0); next }
      } else if (in_step && reftok) {
        # A `ref:` in a block scalar body is text, but a checkout step that
        # carries one is refused rather than read.
        ref_n++; stray_ref = 1
      }
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
        # actions/checkout clones https://github.com/<value>, so a trailing
        # `.git` names the same repository.
        v = tolower(scalar(line))
        sub(/[.]git$/, "", v)
        if (v == repo && in_step) {
          is_repo = 1; repo_nr = NR; repo_text = $0
        } else if (mentions) {
          bad(NR, $0)
        }
      } else if (mentions && keytok) {
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

# How each script was reached, for the checks after the walk: run as its own
# process (from an action, the guard, or a subprocess call), or sourced from a
# script in another directory (path -> that script).
declare -A entry_of=()   # path -> 1
declare -A xsrc_of=()    # path -> sourcing script

add_consumed() { consumed["$1"]=1; }
enqueue() { [[ -n "${queued[$1]:-}" ]] && return 0; queued["$1"]=1; queue+=("$1"); }

# The runner's copy of every action repository, go-kure/.github's included:
# a path through it reaches the checkout without any name this script follows.
ACTIONS_DIR_RE='(^|[^A-Za-z0-9_])_actions([^A-Za-z0-9_]|$)'

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
  #
  # Keys are read as YAML reads them: `"using":` and `'uses' :` are the plain
  # keys, so a quoted `"using": node20` beside an unquoted `using: composite`,
  # or a quoted nested `"uses":`, is no longer missed (go-kure/kure#888).
  # key_yml is the non-comment lines with each line-leading key unquoted. A
  # quoted key with an escape sequence, or a `? ` complex key, could spell
  # either, and is refused. Whether the runner also reads `USES:`, `Using:`
  # or `RUN:` as its key is not established here, so a `uses`, `using` or
  # `run` key in any letter case is taken as that key: refused, unreadable or
  # counted.
  code_yml="$(printf '%s\n' "$content" | { grep -vE '^[[:space:]]*#' || true; })"
  #
  # Every multi-line test below reads its input with `<<<`, never through a
  # pipe: under pipefail, `printf | grep -q` fails once the input outgrows the
  # pipe buffer — grep exits on its first match, printf dies of SIGPIPE, and
  # the pipeline reads as "no match" (go-kure/kure#888 review).
  if grep -qE '"[^"]*\\[^"]*"[[:space:]]*:([[:space:]]|$)|^[[:space:]]*(-[[:space:]]+)?\?([[:space:]]|$)' <<<"$code_yml"; then
    echo "check-pin-impact: ${action_path} at ${NEW_SHA:0:8} has a quoted key with an escape sequence, or a '?' complex key — this scan cannot tell which key it is, refusing to guess" >&2
    exit 1
  fi
  key_yml="$(printf '%s\n' "$code_yml" | sed -E \
    -e "s/^([[:space:]]*(-[[:space:]]+)?)[\"']([A-Za-z0-9_-]+)[\"'][[:space:]]*:/\\1\\3:/" \
    -e "s/^([[:space:]]*(-[[:space:]]+)?)([A-Za-z0-9_-]+)[[:space:]]+:([[:space:]]|\$)/\\1\\3:\\4/")"
  # A `using` key that does not start its line (a flow mapping), or is not
  # spelled in lower case, has no value this scan can read.
  using_values="$(printf '%s\n' "$key_yml" \
    | { grep -iE "(^|[^A-Za-z0-9_-])[\"']?using[\"']?[[:space:]]*:" || true; } \
    | sed -E "/^[[:space:]]*(-[[:space:]]+)?using:/!s/.*/<unreadable>/; s/^[[:space:]]*(-[[:space:]]+)?using:[[:space:]]*//; s/[[:space:]]+#.*\$//; s/[[:space:]]+\$//; s/^[\"']//; s/[\"']\$//")"
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
  # prevent. Mirrors the unrecognized-`source`-expression check below. The
  # key is looked for anywhere on a line, so a flow-mapping step
  # (`- { uses: ... }`) is seen too (go-kure/kure#888), in any letter case.
  if grep -qiE "(^|[^A-Za-z0-9_-])[\"']?uses[\"']?[[:space:]]*:" <<<"$key_yml"; then
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
  run_step_count="$(grep -ciE '^[[:space:]]*(-[[:space:]]+)?run:' <<<"$key_yml" || true)"
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
  # review of go-kure/kure#886). Comment lines are skipped (code_yml, above).
  # The same directory named through the expression context
  # (`${{ github.action_path }}`, case-insensitive like every expression) is
  # not resolved here, so whatever it starts would contribute nothing to the
  # consumed set (go-kure/kure#731 review of go-kure/kure#886).
  if grep -qiE "github(\\.|\\[.)action_path" <<<"$code_yml"; then
    echo "check-pin-impact: ${action_path} at ${NEW_SHA:0:8} uses a github.action_path expression — only \$GITHUB_ACTION_PATH references are resolved, refusing to guess what it invokes" >&2
    exit 1
  fi
  # Nor is any other way of naming the checkout: $GITHUB_ACTION_PATH other
  # than as `$GITHUB_ACTION_PATH/` or `${GITHUB_ACTION_PATH}/` (reassigned, or
  # cut down with `${GITHUB_ACTION_PATH%/*}`), or the runner's `_actions`
  # directory by path (go-kure/kure#888). Every mention must be one complete
  # followed path: `$GITHUB_ACTION_PATH/` then path characters, ending the
  # word — at whitespace, `;&|)<>` or the line end, a closing quote allowed
  # first. `"$GITHUB_ACTION_PATH/"../x.py`, a bare trailing slash, or a
  # suffix after the quote (`".../x.sh".py`) names a file this scan would not
  # see (go-kure/kure#888 review).
  gap_all="$( { grep -oE 'GITHUB_ACTION_PATH' <<<"$code_yml" || true; } | wc -l)"
  # shellcheck disable=SC2016 # a literal $, matched, not expanded
  gap_words="$( { grep -oE '(\$GITHUB_ACTION_PATH|\$\{GITHUB_ACTION_PATH\})/[A-Za-z0-9_./-]+["'\'']?([[:space:];&|)<>]|$)' <<<"$code_yml" || true; } \
    | sed -E "s/[\"'[:space:];&|)<>]+\$//")"
  gap_ref="$(grep -c . <<<"$gap_words" || true)"
  if [[ "$gap_all" -ne "$gap_ref" ]] || grep -qE "$ACTIONS_DIR_RE" <<<"$code_yml"; then
    echo "check-pin-impact: ${action_path} at ${NEW_SHA:0:8} names the go-kure/.github checkout other than as \$GITHUB_ACTION_PATH/<path>, one whole word — refusing to guess what it invokes" >&2
    exit 1
  fi
  action_path_refs="$( { [[ -z "$gap_words" ]] || printf '%s\n' "$gap_words"; } | sort -u)"
  if [[ -z "$action_path_refs" ]] && grep -qiE '^[[:space:]]*(-[[:space:]]+)?run:' <<<"$key_yml"; then
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
    entry_of["$target"]=1
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
entry_of["$guard_script"]=1

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
# The word SCRIPT_DIR in any form. Outside a trusted definition it may appear
# only as a plain `$SCRIPT_DIR` or `${SCRIPT_DIR}` read: `SCRIPT_DIR+=/lib`,
# `SCRIPT_DIR[0]=`, `read SCRIPT_DIR` or `n=SCRIPT_DIR` repoint or pass on the
# name without an assignment this scan would see (go-kure/kure#888 review).
SCRIPT_DIR_WORD_RE='(^|[^A-Za-z0-9_])SCRIPT_DIR([^A-Za-z0-9_]|$)'
# Name indirection, which reads or writes a variable without naming it: an
# indirect expansion `${!name}` (the array-keys form `${!name[@]}` is none), a
# nameref declaration (`declare -n`, `local -n`, `typeset -n`), and `eval`.
# shellcheck disable=SC2016 # a literal ${!, matched, not expanded
INDIRECTION_RE='\$\{!|(^|[^A-Za-z0-9_])(declare|local|typeset)([[:space:]]+[-+][A-Za-z]+)*[[:space:]]+-[A-Za-z]*n|(^|[^A-Za-z0-9_.-])eval([[:space:]]|$)'
# shellcheck disable=SC2016 # a literal ${!, matched, not expanded
ARRAY_KEYS_RE='\$\{![A-Za-z_][A-Za-z0-9_]*\[[@*]\]\}'
# An expression for the script's own location: BASH_SOURCE, BASH_ARGV (and
# BASH_ARGV0), ${0...}, a positional slice `${@:...}`/`${*:...}` (an offset
# of 0, however computed, is $0), or dirname/realpath/readlink of $0.
SELF_DIR_RE='BASH_SOURCE|BASH_ARGV|\$\{0([^0-9A-Za-z_]|$)|\$\{[@*]:[^-+?=]|(dirname|realpath|readlink)[^;&|]*\$\{?0\}?([^0-9A-Za-z_]|$)'
# A bare $0: the script's own path, refused unless strip_self_path_uses below
# accounts for it. `x=$0` and then `"${x%/*}/tool.py"`, `printf -v x ... "$0"`,
# `read x <<<"$0"` or a function called with "$0" carry the directory under
# another name (go-kure/kure#888).
# shellcheck disable=SC2016 # a literal $0, matched, not expanded
BARE_SELF_RE='\$0([^0-9A-Za-z_]|$)'
# The run-when-executed guard, alone on its line: two strings compared, no
# directory derived.
# shellcheck disable=SC2016 # literal $0/${...}, matched, not expanded
GUARD_RE='^[[:space:]]*if[[:space:]]+\[\[[[:space:]]+"\$\{BASH_SOURCE\[0\]\}"[[:space:]]+(==|!=)[[:space:]]+"\$(0|\{0\})"[[:space:]]+\]\](;[[:space:]]*then)?[[:space:]]*$'
# Names of the go-kure/.github checkout other than $SCRIPT_DIR: the action's
# directory, or the runner's `_actions` copy by path (go-kure/kure#888).
CHECKOUT_NAME_RE="GITHUB_ACTION_PATH|${ACTIONS_DIR_RE}"
# A working-directory change: cd/pushd/popd as a word. strip_subshell_cd
# removes the `$(cd` and `(cd` forms, which change only a subshell's.
CWD_CHANGE_RE='(^|[^A-Za-z0-9_.$-])(cd|pushd|popd)([^A-Za-z0-9_.-]|$)'

# strip_self_path_uses -- on stdin, one line; print it without the $0
# occurrences that name no directory: a message to stderr (`echo "usage: $0
# ..." >&2` or `1>&2`, before `;`, `}` or the line end — a message to any
# other fd could be read back), a read of the script's own text (`sed -n
# '<lines>p' "$0"`), and an awk program's record (a function argument `f($0`
# or `, $0`, ` = $0`, `$0 ~`, `$0 !~`). A bash array `a=($0)` is no awk call:
# the `(` must follow a name.
strip_self_path_uses() {
  # shellcheck disable=SC2016 # literal $0 in the sed expressions
  sed -E \
    -e 's/echo([[:space:]]+-[neE]+)*[[:space:]]+"[^"]*\$0[^"]*"[[:space:]]*1?>&2[[:space:]]*(;|\}|$)/\2/g' \
    -e "s/sed[[:space:]]+-n[[:space:]]+'[0-9,]+p'[[:space:]]+\"\\\$0\"//g" \
    -e 's/([A-Za-z0-9_]\(|,)[[:space:]]*\$0([^0-9A-Za-z_]|$)/\2/g' \
    -e 's/[[:space:]]=[[:space:]]+\$0([^0-9A-Za-z_]|$)/\1/g' \
    -e 's/\$0[[:space:]]+!?~//g'
}

# Any assignment to SCRIPT_DIR, whatever precedes it (`export`, `readonly`,
# `declare`, `local`, another assignment).
SCRIPT_DIR_ASSIGN_RE='(^|[^A-Za-z0-9_])SCRIPT_DIR='
# The only SCRIPT_DIR definitions trusted, the whole line: the script's own
# directory as `$(dirname "$0")` or `$(cd "$(dirname "$0")" [&>/dev/null |
# >/dev/null 2>&1] && pwd)`, or either with "${BASH_SOURCE[0]}" for "$0" —
# one dirname, of the script itself — the value quoted or not, optionally
# behind `declare -r`, `readonly` or `export`. `cd --`, `dirname --` and a
# space after `&>`/`>` are the same definition (go-kure/kure#888), and so are
# `>/dev/null` alone, `2>/dev/null` and `pwd -P`: a redirect of cd's output
# or errors leaves pwd's output alone, and the physical path of a directory
# holds the same files as its logical one (no `..` is followed). Trusting
# any residue-free definition used to accept `$(dirname "$(dirname "$0")")`,
# and an assignment with no $0 in it was never looked at (go-kure/kure#731
# review of go-kure/kure#886).
# shellcheck disable=SC2016 # a literal $( ), matched, not expanded
SCRIPT_DIR_DIRNAME_RE='\$\(dirname( --)? ("\$0"|"\$\{BASH_SOURCE\[0\]\}")\)'
SCRIPT_DIR_VALUE_RE='(\$\(cd( --)? "'"${SCRIPT_DIR_DIRNAME_RE}"'"( &> ?/dev/null| 1?> ?/dev/null( 2>&1)?| 2> ?/dev/null)? && pwd( -P)?\)|'"${SCRIPT_DIR_DIRNAME_RE}"')'
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
  if [[ "$kind" != sourced ]]; then
    entry_of["$target"]=1
  elif [[ "$(dirname "$target")" != "$(dirname "$from")" && -z "${xsrc_of[$target]:-}" ]]; then
    xsrc_of["$target"]="$from"
  fi
}

# What the walk records per script for the checks after it (go-kure/kure#888).
declare -A sd_mention=()   # first line naming SCRIPT_DIR at all
declare -A sd_early=()     # first use of $SCRIPT_DIR before a trusted definition
declare -A rel_def=()      # a trusted definition without `cd ... && pwd`
declare -A cwd_change=()   # first line changing the working directory

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
  #
  # go-kure/kure#888 adds: a bare `$0` outside a message, a read of the
  # script's own text or an awk record is a self-path expression too; a line
  # naming $GITHUB_ACTION_PATH or the runner's `_actions` directory is
  # refused; and the walk records, for the checks after it, which scripts
  # name SCRIPT_DIR, use it before defining it, define it relatively, or
  # change the working directory.
  code_lines="$(printf '%s\n' "$content" | { grep -vE '^[[:space:]]*#' || true; })"
  candidate_lines="$( { grep -E "${SEP_RE}(source|\\.)[[:space:]]|${SEP_RE}${EXEC_CANDIDATE_RE}|${SCRIPT_DIR_RE}|${SCRIPT_DIR_ASSIGN_RE}|${SCRIPT_DIR_WORD_RE}|${INDIRECTION_RE}|${SELF_DIR_RE}|${BARE_SELF_RE}|${CHECKOUT_NAME_RE}" <<<"$code_lines" || true; })"
  # The first such line, read to the end: an early `exit` would kill the
  # writer with SIGPIPE and, under pipefail, the whole run with rc 141.
  cwd_line="$(awk -v re="$CWD_CHANGE_RE" '!found { l = $0; gsub(/\$?\([[:space:]]*(cd|pushd|popd)/, "(", l); if (l ~ re) { print; found = 1 } }' <<<"$code_lines")"
  [[ -n "$cwd_line" ]] && cwd_change["$script"]="$cwd_line"
  defined=0
  while IFS= read -r line; do
    [[ -n "$line" ]] || continue
    [[ "$line" =~ $GUARD_RE ]] && continue
    assigns=0
    [[ "$line" =~ $SCRIPT_DIR_ASSIGN_RE ]] && assigns=1
    # Before the source/subprocess forms: one could pass the name on as an
    # argument. A SCRIPT_DIR assignment naming it is no trusted definition,
    # and is refused as one below.
    if [[ $assigns -eq 0 ]] && grep -qE "$CHECKOUT_NAME_RE" <<<"$line"; then
      echo "check-pin-impact: a line names the go-kure/.github checkout other than through \$SCRIPT_DIR in ${script} — refusing to guess what it reaches:" >&2
      echo "  $line" >&2
      exit 1
    fi
    # A variable read or written without its name: whatever it holds (the
    # script's directory, SCRIPT_DIR itself) is out of sight (go-kure/kure#888
    # review).
    if grep -qE "$INDIRECTION_RE" <<<"$(sed -E "s/${ARRAY_KEYS_RE}//g" <<<"$line")"; then
      echo "check-pin-impact: name indirection (\${!name}, a nameref or eval) in ${script} — refusing to guess which variable it reaches:" >&2
      echo "  $line" >&2
      exit 1
    fi
    script_dir_uses="$( { grep -oE "$SCRIPT_DIR_RE" <<<"$line" || true; } | grep -c . || true)"
    # SCRIPT_DIR named other than by a plain read or a trusted definition.
    sd_word=0 sd_other=0
    if grep -qE "$SCRIPT_DIR_WORD_RE" <<<"$line"; then
      sd_word=1
      # shellcheck disable=SC2016 # literal $SCRIPT_DIR in the sed expression
      if [[ ! "$line" =~ $SCRIPT_DIR_DEF_RE ]] \
          && grep -qE "$SCRIPT_DIR_WORD_RE" <<<"$(sed -E 's/\$SCRIPT_DIR([^A-Za-z0-9_]|$)/\1/g; s/\$\{SCRIPT_DIR\}//g' <<<"$line")"; then
        sd_other=1
      fi
    fi
    self_dir_use=0
    grep -qE "$SELF_DIR_RE" <<<"$line" && self_dir_use=1
    if [[ $self_dir_use -eq 0 ]] && grep -qE "$BARE_SELF_RE" <<<"$(strip_self_path_uses <<<"$line")"; then
      self_dir_use=1
    fi
    if [[ $sd_word -eq 1 ]] && [[ -z "${sd_mention[$script]:-}" ]]; then
      sd_mention["$script"]="$line"
    fi
    if [[ "$script_dir_uses" -gt 0 && $assigns -eq 0 && $defined -eq 0 && -z "${sd_early[$script]:-}" ]]; then
      sd_early["$script"]="$line"
    fi
    if [[ "$script_dir_uses" -eq 1 && $sd_other -eq 0 && $self_dir_use -eq 0 && $assigns -eq 0 && "$line" =~ $SOURCE_RE ]]; then
      consume_sibling sourced "$script" "${script_dir}/${BASH_REMATCH[2]}"
      continue
    fi
    if [[ "$script_dir_uses" -eq 1 && $sd_other -eq 0 && $self_dir_use -eq 0 && $assigns -eq 0 && "$line" =~ $EXEC_RE ]]; then
      consume_sibling invoked "$script" "${script_dir}/${BASH_REMATCH[4]}"
      continue
    fi
    if [[ $assigns -eq 1 && "$line" =~ $SCRIPT_DIR_DEF_RE ]]; then
      defined=1
      if [[ "$line" != *"&& pwd)"* && "$line" != *"&& pwd -P)"* && -z "${rel_def[$script]:-}" ]]; then
        rel_def["$script"]="$line"
      fi
      continue
    fi
    # A line that is a candidate only for a $0 strip_self_path_uses accounts
    # for names no directory.
    if [[ "$script_dir_uses" -eq 0 && $sd_other -eq 0 && $assigns -eq 0 && $self_dir_use -eq 0 ]] \
        && ! grep -qE "${SEP_RE}(source|\\.)[[:space:]]|${SEP_RE}${EXEC_CANDIDATE_RE}" <<<"$line"; then
      continue
    fi
    if grep -qE "${SEP_RE}(source|\\.)[[:space:]]" <<<"$line"; then
      what="source expression"
    elif grep -qE "${SEP_RE}${EXEC_CANDIDATE_RE}" <<<"$line"; then
      what="sibling-script invocation"
    elif [[ $assigns -eq 1 ]]; then
      what="SCRIPT_DIR definition"
    elif [[ "$script_dir_uses" -gt 0 ]]; then
      what="use of \$SCRIPT_DIR"
    elif [[ $sd_other -eq 1 ]]; then
      what="mention of SCRIPT_DIR"
    else
      what="script-directory expression"
    fi
    echo "check-pin-impact: unrecognized ${what} in ${script} — refusing to guess whether it needs resolving:" >&2
    echo "  $line" >&2
    exit 1
  done <<<"$candidate_lines"
done

# Siblings above were resolved from each script's own directory. That holds
# only where $SCRIPT_DIR is that directory (go-kure/kure#888):
#   - a file sourced from a script in another directory shares its caller's
#     SCRIPT_DIR, and redefining it would change what the caller's later
#     lines name, so such a file may not name SCRIPT_DIR at all;
#   - a script run as its own process (from an action, the guard, or a
#     subprocess call) that uses $SCRIPT_DIR before defining it reads
#     whatever SCRIPT_DIR it inherited;
#   - a relative definition, `$(dirname "$0")`, names a path relative to the
#     working directory, so no script the walk reached may change it.
walked="$(printf '%s\n' "${!queued[@]}" | sort)"
rel_at="" cwd_at=""
while IFS= read -r p; do
  [[ -n "$p" ]] || continue
  if [[ -n "${xsrc_of[$p]:-}" && -n "${sd_mention[$p]:-}" ]]; then
    echo "check-pin-impact: ${p} is sourced from ${xsrc_of[$p]} in another directory, yet names SCRIPT_DIR — it shares its caller's, so its siblings cannot be resolved from its own directory; refusing to guess:" >&2
    echo "  ${sd_mention[$p]}" >&2
    exit 1
  fi
  if [[ -n "${entry_of[$p]:-}" && -n "${sd_early[$p]:-}" ]]; then
    echo "check-pin-impact: ${p} uses \$SCRIPT_DIR before defining it — run as its own process, it reads whatever SCRIPT_DIR it inherited; refusing to guess:" >&2
    echo "  ${sd_early[$p]}" >&2
    exit 1
  fi
  [[ -z "$rel_at" && -n "${rel_def[$p]:-}" ]] && rel_at="$p"
  [[ -z "$cwd_at" && -n "${cwd_change[$p]:-}" ]] && cwd_at="$p"
done <<<"$walked"
if [[ -n "$rel_at" && -n "$cwd_at" ]]; then
  echo "check-pin-impact: relative SCRIPT_DIR in ${rel_at} and a working-directory change in ${cwd_at} — \$SCRIPT_DIR/<name>.sh may then name another file; refusing to guess:" >&2
  echo "  ${rel_def[$rel_at]}" >&2
  echo "  ${cwd_change[$cwd_at]}" >&2
  exit 1
fi

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
# `sed -n 1…p` reads to the end: a `head -1` would let grep die of SIGPIPE
# under pipefail on a large compare.
compare_status="$( { grep -oE '"status":[[:space:]]*"[^"]*"' <<<"$compare_json" || true; } | sed -nE '1{s/^"status":[[:space:]]*"//; s/"$//; p;}')"
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
  if grep -qxF -- "$p" <<<"$changed_files"; then
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
