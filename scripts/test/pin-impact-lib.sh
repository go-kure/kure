#!/bin/bash
# scripts/test/pin-impact-lib.sh - hermetic harness for scripts/check-pin-impact.sh.
# Sourced by the scripts/test/cases/*-pin-impact-*.sh files. Never executed directly.
#
# Each case gets a throwaway git repository holding a copy of check-pin-impact.sh
# and a synthetic .github/workflows/ci.yml, committed once on branch `base` with
# the OLD pin and then rewritten in the working tree to the NEW pin -- the shape
# CI mode (`--base-ref`) reads. No network: a `curl` stub on PATH serves
# go-kure/.github file content from $PI_ROOT/raw/<sha>/<path> and compare
# responses from $PI_ROOT/api/<old>...<new>.json, and fails like `curl -f` (exit
# 22) for anything it has no fixture for, so a missing fixture reads as a fetch
# failure rather than as an empty file.

set -uo pipefail

PI_SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/check-pin-impact.sh"
PI_OLD=$(printf 'a%.0s' {1..40})
PI_NEW=$(printf 'b%.0s' {1..40})
PI_OTHER=$(printf 'c%.0s' {1..40})
export PI_OLD PI_NEW PI_OTHER  # read by the case files that source this lib
PI_ROOT=""
PI_OUT=""
PI_RC=0

# pi_fixture <action-name>... -- a repo whose ci.yml references each named
# go-kure/.github action, pinned to PI_OLD on `base` and PI_NEW in the working
# tree. Every action gets a default composite action.yml running
# scripts/<name>.sh, and every script (plus the always-consumed
# scripts/check-forbidden-terms.sh) a plain body at PI_NEW. An inert compare
# (status ahead, one unrelated file) is installed; cases override what they test.
pi_fixture() {
    PI_ROOT=$(mktemp -d) || { echo "pi_fixture: mktemp -d failed" >&2; exit 1; }
    trap 'rm -rf "$PI_ROOT"' EXIT
    mkdir -p "$PI_ROOT/repo/scripts" "$PI_ROOT/repo/.github/workflows" "$PI_ROOT/stub" "$PI_ROOT/raw" "$PI_ROOT/api"
    cp "$PI_SRC" "$PI_ROOT/repo/scripts/"

    local name
    for name in "$@"; do
        pi_raw "$PI_NEW" ".github/actions/$name/action.yml" "$(pi_action_yml "$name")"
        pi_raw "$PI_NEW" "scripts/$name.sh" $'#!/bin/bash\necho ok'
    done
    pi_raw "$PI_NEW" "scripts/check-forbidden-terms.sh" $'#!/bin/bash\necho ok'
    pi_compare ahead "docs/unrelated.md"

    pi_workflow "$PI_OLD" "$@"
    pi_git init -q -b base || { echo "pi_fixture: git init failed" >&2; exit 1; }
    pi_git add -A || { echo "pi_fixture: git add failed" >&2; exit 1; }
    pi_git commit -qm base || { echo "pi_fixture: git commit failed" >&2; exit 1; }
    pi_workflow "$PI_NEW" "$@"

    cat >"$PI_ROOT/stub/curl" <<'STUB'
#!/bin/bash
url="${*: -1}"
case "$url" in
https://raw.githubusercontent.com/go-kure/.github/*)
    rest="${url#https://raw.githubusercontent.com/go-kure/.github/}"
    f="$PI_ROOT/raw/$rest" ;;
https://api.github.com/repos/go-kure/.github/compare/*)
    f="$PI_ROOT/api/${url##*/}.json" ;;
*) echo "curl stub: unexpected URL $url" >&2; exit 2 ;;
esac
[[ -f "$f" ]] || exit 22
cat "$f"
STUB
    chmod +x "$PI_ROOT/stub/curl"
    export PI_ROOT
    export PATH="$PI_ROOT/stub:$PATH"
    unset PIN_IMPACT_ACK GITHUB_TOKEN
}

# pi_git <args>... -- git in the fixture repo, isolated from the caller's
# configuration: no global or system config (so no commit signing, templates or
# hooks from the developer's setup), a fixed identity, and hooks disabled.
# Repository-selection variables a caller may have exported (a git hook
# running `make precommit` sets GIT_DIR and GIT_INDEX_FILE, for one). `git -C`
# does not override them, so left in place they would point the fixture's
# add/commit -- and the checker's `git show` -- at the caller's real repository.
PI_GIT_UNSET=(-u GIT_DIR -u GIT_WORK_TREE -u GIT_INDEX_FILE -u GIT_OBJECT_DIRECTORY
    -u GIT_ALTERNATE_OBJECT_DIRECTORIES -u GIT_COMMON_DIR -u GIT_NAMESPACE -u GIT_PREFIX)

pi_git() {
    env "${PI_GIT_UNSET[@]}" GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 \
        git -C "$PI_ROOT/repo" -c user.name=pin-impact-test -c user.email=test@example.invalid \
        -c commit.gpgsign=false -c tag.gpgsign=false -c core.hooksPath=/dev/null "$@"
}

# pi_action_yml <name> -- the default composite action.yml: one run step that
# invokes scripts/<name>.sh the way every go-kure/.github action does, climbing
# from $GITHUB_ACTION_PATH (.github/actions/<name>) to the repo root: three
# `..` hops plus one per extra segment of a nested <name>.
pi_action_yml() {
    local up="../../.." slashes="${1//[!\/]/}" i
    for ((i = 0; i < ${#slashes}; i++)); do up="$up/.."; done
    # $GITHUB_ACTION_PATH is written literally into the fixture action.yml.
    # shellcheck disable=SC2016
    printf 'name: %s\nruns:\n  using: composite\n  steps:\n    - name: run\n      shell: bash\n      run: bash "$GITHUB_ACTION_PATH/%s/scripts/%s.sh"\n' "$1" "$up" "$1"
}

# pi_workflow <sha> <action-name>... -- (re)write ci.yml pinning every action to <sha>.
pi_workflow() {
    local sha="$1"; shift
    {
        printf 'jobs:\n  j:\n    steps:\n'
        local name
        for name in "$@"; do
            printf '      - uses: go-kure/.github/.github/actions/%s@%s # main\n' "$name" "$sha"
        done
    } >"$PI_ROOT/repo/.github/workflows/ci.yml"
}

# pi_raw <sha> <path> <content> -- serve <content> as <path> at <sha>.
pi_raw() {
    mkdir -p "$(dirname "$PI_ROOT/raw/$1/$2")"
    printf '%s\n' "$3" >"$PI_ROOT/raw/$1/$2"
}

# pi_compare <status> <file>... -- the OLD...NEW compare response.
pi_compare() {
    local status="$1"; shift
    {
        printf '{"status": "%s", "files": [' "$status"
        local first=1 f
        for f in "$@"; do
            [[ $first -eq 1 ]] || printf ','
            first=0
            printf '{"filename": "%s"}' "$f"
        done
        printf ']}\n'
    } >"$PI_ROOT/api/${PI_OLD}...${PI_NEW}.json"
}

# pi_run -- run the checker in CI mode against the `base` branch.
pi_run() {
    PI_OUT=$(cd "$PI_ROOT/repo" && env "${PI_GIT_UNSET[@]}" bash scripts/check-pin-impact.sh --base-ref base 2>&1)
    PI_RC=$?
}

pi_expect() { # <rc> <substring>
    if [[ "$PI_RC" -ne "$1" ]]; then
        printf 'expected rc=%s, got rc=%s\n%s\n' "$1" "$PI_RC" "$PI_OUT" >&2
        exit 1
    fi
    if [[ "$PI_OUT" != *"$2"* ]]; then
        printf 'expected output containing %q\n%s\n' "$2" "$PI_OUT" >&2
        exit 1
    fi
}
