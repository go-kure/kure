#!/bin/bash
# scripts/test/lib.sh - helpers for scripts/sync-versions.sh's failure-path harness.
# Sourced by every scripts/test/cases/*.sh file. Never executed directly.

set -uo pipefail

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FIXTURES_DIR="$TEST_DIR/fixtures"
SYNC_VERSIONS="$TEST_DIR/../sync-versions.sh"

# Three 40-hex-character placeholder commit SHAs shared by every rows-29-34
# case. resolve_tag_commit's own format gate (sync-versions.sh:163-164) is
# `^[0-9a-f]{40}$` -- anything else, even one character short, fails silently
# into the warning/rc=2 branch instead of exercising the intended one.
# Generated, never hand-typed: a hand-typed 38-character placeholder in an
# earlier draft of this harness is exactly the bug this generate-and-assert
# pair exists to prevent.
NETDEP_COMMIT=$(printf 'a%.0s' $(seq 40))
MISMATCH_COMMIT=$(printf 'c%.0s' $(seq 40))
TAGOBJ_SHA=$(printf 'd%.0s' $(seq 40))
UPSTREAM_DIGEST_COMMIT=$(printf 'e%.0s' $(seq 40))
for _v in NETDEP_COMMIT MISMATCH_COMMIT TAGOBJ_SHA UPSTREAM_DIGEST_COMMIT; do
    _val="${!_v}"
    _len=${#_val}
    if [[ "$_len" -ne 40 ]]; then
        echo "lib.sh: $_v is $_len characters, not 40 -- fix the generator, do not hand-edit the value" >&2
        exit 1
    fi
done
unset _v _val _len
export NETDEP_COMMIT MISMATCH_COMMIT TAGOBJ_SHA UPSTREAM_DIGEST_COMMIT

# Fixed constants describing fixtures/base/versions.yaml's floor-dep entry --
# shared between lib.sh's own documentation and fixtures/stub-go/go, which
# hardcodes the same two values as its canned `go mod edit -json` answer.
FLOOR_REQUIRE_PATH="github.com/example/floor-dep"
FLOOR_REQUIRE_VERSION="v0.5.1"
export FLOOR_REQUIRE_PATH FLOOR_REQUIRE_VERSION

# Set by new_fixture. Read by every helper below. Never assign via
# `fx=$(new_fixture)` -- a command-substitution subshell fires new_fixture's
# own EXIT trap the instant the subshell exits (right after copying),
# deleting the fixture before the calling case body ever runs. Call it bare.
FIXTURE=""

# Copies fixtures/base/ into a fresh mktemp -d, creates docs/ and
# pkg/versions/, runs `generate` once so the two drift guards start green,
# and registers cleanup. Also arranges PATH so fixtures/stub-go/'s go, curl
# and git stubs are found ahead of the real ones, defaulting
# STUB_GO_MODE=ok and STUB_NET_MODE=unreachable -- both are the default for
# every case, not an opt-in.
new_fixture() {
    FIXTURE=$(mktemp -d)
    trap 'rm -rf "$FIXTURE"' EXIT

    cp -r "$FIXTURES_DIR/base/." "$FIXTURE/"
    mkdir -p "$FIXTURE/docs" "$FIXTURE/pkg/versions"

    export SYNC_VERSIONS_REPO_ROOT="$FIXTURE"
    export STUB_GO_MODE="${STUB_GO_MODE:-ok}"
    export STUB_NET_MODE="${STUB_NET_MODE:-unreachable}"
    export PATH="$FIXTURES_DIR/stub-go:$PATH"

    local out rc=0
    out=$("$SYNC_VERSIONS" generate 2>&1) || rc=$?
    if [[ $rc -ne 0 ]]; then
        echo "new_fixture: initial 'generate' failed (rc=$rc) -- fixture is broken, not the case under test:" >&2
        echo "$out" >&2
        exit 1
    fi
}

# yq_set <yaml-path> <value> -- set a scalar in $FIXTURE/versions.yaml.
# For anything beyond a scalar set (deleting a key, adding a whole new
# entry), call `yq eval -i '...' "$FIXTURE/versions.yaml"` directly in the
# case file -- this helper only covers the common case.
yq_set() {
    local path="$1" value="$2"
    yq eval -i "${path} = \"${value}\"" "$FIXTURE/versions.yaml"
}

# gomod_sub <sed-expr> -- apply a sed substitution to $FIXTURE/go.mod.
gomod_sub() {
    sed -i "$1" "$FIXTURE/go.mod"
}

# run_check / run_generate -- invoke sync-versions.sh against $FIXTURE,
# capturing stdout, stderr and exit code into $out / $err / $rc. Never lets
# a non-zero exit abort the case (the harness itself does not run under
# `set -e`; this is belt-and-braces since lib.sh does set -u/-o pipefail).
run_check() { _run check; }
run_generate() { _run generate; }

_run() {
    local errfile
    errfile=$(mktemp)
    rc=0
    out=$("$SYNC_VERSIONS" "$1" 2>"$errfile") || rc=$?
    err=$(cat "$errfile")
    rm -f "$errfile"
}

assert_rc() {
    local expected="$1"
    if [[ "$rc" -ne "$expected" ]]; then
        echo "FAIL: expected rc=$expected, got rc=$rc" >&2
        echo "--- stdout ---" >&2; echo "$out" >&2
        echo "--- stderr ---" >&2; echo "$err" >&2
        exit 1
    fi
}

assert_err_contains() {
    local needle="$1"
    if [[ "$err" != *"$needle"* ]]; then
        echo "FAIL: stderr does not contain: $needle" >&2
        echo "--- stdout ---" >&2; echo "$out" >&2
        echo "--- stderr ---" >&2; echo "$err" >&2
        exit 1
    fi
}

assert_out_contains() {
    local needle="$1"
    if [[ "$out" != *"$needle"* ]]; then
        echo "FAIL: stdout does not contain: $needle" >&2
        echo "--- stdout ---" >&2; echo "$out" >&2
        echo "--- stderr ---" >&2; echo "$err" >&2
        exit 1
    fi
}

assert_out_not_contains() {
    local needle="$1"
    if [[ "$out" == *"$needle"* ]]; then
        echo "FAIL: stdout unexpectedly contains: $needle" >&2
        echo "--- stdout ---" >&2; echo "$out" >&2
        exit 1
    fi
}

# with_stub_go <mode> -- override STUB_GO_MODE (default "ok", set by
# new_fixture) for a case needing missing/badpath/norequire/editfail/hang
# instead, or "absent" for the one-file always-fail shim (see below).
# Must be called AFTER new_fixture.
with_stub_go() {
    local mode="$1"
    if [[ "$mode" == "absent" ]]; then
        # Don't exclude go's real bindir from PATH -- its location varies by
        # install method, and a flat PATH truncation also drops yq, tripping
        # check_dependencies before the guard under test even runs. Instead
        # prepend a one-file always-fail `go` shim ahead of PATH, including
        # ahead of fixtures/stub-go -- this makes `go` unresolvable to any
        # real go binary while leaving fixtures/stub-go's own curl/git stubs
        # reachable, so the net-dep live-check path stays hermetic even
        # though go is gone.
        export PATH="$FIXTURES_DIR/stub-go-absent:$PATH"
    else
        export STUB_GO_MODE="$mode"
    fi
}

# with_stub_net <mode> -- override STUB_NET_MODE (default "unreachable", set
# by new_fixture) for a case needing mismatch, match, notfound, tag-object
# or ls-remote-match instead (rows 29-34).
with_stub_net() {
    export STUB_NET_MODE="$1"
}
