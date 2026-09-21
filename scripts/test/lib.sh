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
NETDEP_COMMIT=$(printf 'a%.0s' {1..40})
MISMATCH_COMMIT=$(printf 'c%.0s' {1..40})
TAGOBJ_SHA=$(printf 'd%.0s' {1..40})
UPSTREAM_DIGEST_COMMIT=$(printf 'e%.0s' {1..40})
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
    FIXTURE=$(mktemp -d) || {
        echo "new_fixture: mktemp -d failed -- refusing to continue (would otherwise copy into an empty/unset path)" >&2
        exit 1
    }
    if [[ -z "$FIXTURE" || ! -d "$FIXTURE" ]]; then
        echo "new_fixture: mktemp -d produced an unusable path ('$FIXTURE') -- refusing to continue" >&2
        exit 1
    fi
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
# Temp-file + mv, not `sed -i` -- BSD/macOS sed's `-i` takes the backup
# suffix as its next (mandatory) argument, so `sed -i "$1"` would consume
# $1 itself as that suffix and leave go.mod unedited on any non-GNU sed.
# Same pattern as scripts/sync-tool-versions.sh's own Makefile/ci.yml/docs
# edits, for the same reason.
gomod_sub() {
    sed "$1" "$FIXTURE/go.mod" > "$FIXTURE/go.mod.tmp" && mv "$FIXTURE/go.mod.tmp" "$FIXTURE/go.mod"
}

# run_check / run_generate / run_widen -- invoke sync-versions.sh against
# $FIXTURE, capturing stdout, stderr and exit code into $out / $err / $rc.
# Never lets a non-zero exit abort the case (the harness itself does not run
# under `set -e`; this is belt-and-braces since lib.sh does set -u/-o pipefail).
run_check() { _run check; }
run_generate() { _run generate; }
# run_widen <dep> <new-upper-bound> [--note "..."] -- forwards every argument
# after "widen" verbatim, so a case can also omit --note or pass an unknown
# flag to exercise the subcommand's own argument-parsing errors.
run_widen() { _run widen "$@"; }

_run() {
    local errfile
    errfile=$(mktemp) || {
        echo "_run: mktemp failed -- refusing to run '$SYNC_VERSIONS $*' without a place to capture stderr" >&2
        exit 1
    }
    if [[ -z "$errfile" || ! -f "$errfile" ]]; then
        echo "_run: mktemp produced an unusable path ('$errfile') -- refusing to continue" >&2
        exit 1
    fi
    rc=0
    out=$("$SYNC_VERSIONS" "$@" 2>"$errfile") || rc=$?
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

assert_err_not_contains() {
    local needle="$1"
    if [[ "$err" == *"$needle"* ]]; then
        echo "FAIL: stderr unexpectedly contains: $needle" >&2
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

# ---------------------------------------------------------------------------
# sync-flux-operator-pin.sh helpers. A separate fixture from new_fixture: that
# one builds a versions.yaml/docs tree for sync-versions.sh, this one builds
# the four files the pin script reads or rewrites, and stubs only `curl`
# (fixtures/stub-pin/curl) -- the script needs no go or git.
# ---------------------------------------------------------------------------
SYNC_FLUX_OPERATOR_PIN="$TEST_DIR/../sync-flux-operator-pin.sh"
PIN_MODULE="github.com/example/flux-operator"
PIN_FIXTURES=()

# A three-document stand-in for an upstream install.yaml. It must carry a
# CustomResourceDefinition and a Deployment (the script's shape check) and be
# distinguishable from the fixture's pre-existing "old" bundle.
PIN_NEW_BUNDLE_BODY=$'apiVersion: v1\nkind: Namespace\n---\nkind: CustomResourceDefinition\n---\nkind: Deployment\nimage: new\n'

# new_pin_fixture <go.mod-version> -- builds $FIXTURE with go.mod requiring
# $PIN_MODULE at that version (inside a `require (` block, next to a `replace`
# line naming a different module, so the awk that reads it is exercised on the
# shapes it must skip), an "old" vendored bundle pinned at v0.1.0, and the
# constant and README anchors the script rewrites. Sets STUB_PIN_LOG,
# STUB_PIN_BODY and STUB_PIN_MODE=ok. Call bare, like new_fixture.
new_pin_fixture() {
    local gomod_version="$1"
    FIXTURE=$(mktemp -d) || {
        echo "new_pin_fixture: mktemp -d failed -- refusing to continue" >&2
        exit 1
    }
    if [[ -z "$FIXTURE" || ! -d "$FIXTURE" ]]; then
        echo "new_pin_fixture: mktemp -d produced an unusable path ('$FIXTURE') -- refusing to continue" >&2
        exit 1
    fi
    # A case may call this more than once (case 59 does). The trap body is
    # evaluated at exit, so `rm -rf "$FIXTURE"` would remove only the last
    # fixture and leak the earlier ones; accumulate every path instead.
    PIN_FIXTURES+=("$FIXTURE")
    trap 'rm -rf "${PIN_FIXTURES[@]}"' EXIT

    mkdir -p "$FIXTURE/pkg/stack/fluxcd" "$FIXTURE/bin"
    cat > "$FIXTURE/versions.yaml" <<YAML
infrastructure:
  flux-operator:
    go_module: "$PIN_MODULE"
    supported_range: "0.23 - 0.58"
YAML
    cat > "$FIXTURE/go.mod" <<GOMOD
module example.com/fixture

go 1.26.0

require (
	github.com/example/other v9.9.9
	$PIN_MODULE $gomod_version
)

replace github.com/example/other => ../other
GOMOD
    printf 'apiVersion: v1\nkind: Namespace\n---\nkind: CustomResourceDefinition\n---\nkind: Deployment\nimage: old\n' \
        > "$FIXTURE/pkg/stack/fluxcd/flux_operator_install.yaml"
    cat > "$FIXTURE/pkg/stack/fluxcd/flux_operator_install.go" <<'GO'
package fluxcd

const FluxOperatorVersion = "v0.1.0"
GO
    cat > "$FIXTURE/pkg/stack/fluxcd/README.md" <<'MD'
The bundle is vendored (`FluxOperatorVersion`, currently **v0.1.0**). See the source.
MD
    printf '%s' "$PIN_NEW_BUNDLE_BODY" > "$FIXTURE/new-bundle.yaml"

    export SYNC_FLUX_OPERATOR_PIN_REPO_ROOT="$FIXTURE"
    export STUB_PIN_MODE="${STUB_PIN_MODE:-ok}"
    export STUB_PIN_BODY="$FIXTURE/new-bundle.yaml"
    export STUB_PIN_LOG="$FIXTURE/curl.log"
    export PATH="$FIXTURES_DIR/stub-pin:$PATH"
}

# run_pin -- run sync-flux-operator-pin.sh against $FIXTURE, capturing stdout,
# stderr and exit code into $out / $err / $rc like run_check does.
run_pin() {
    local errfile
    errfile=$(mktemp) || {
        echo "run_pin: mktemp failed -- refusing to run without a place to capture stderr" >&2
        exit 1
    }
    rc=0
    out=$("$SYNC_FLUX_OPERATOR_PIN" 2>"$errfile") || rc=$?
    err=$(cat "$errfile")
    rm -f "$errfile"
}

# pin_snapshot / assert_pin_tree_unchanged -- let a case assert "left
# untouched" over everything under the fixture's pkg/stack/fluxcd (a stray
# *.new scratch file counts) without naming each file. A copy compared with
# `diff -r`, not a checksum: `diff` is POSIX, and this harness runs without
# `set -e`, so a missing checksum tool would make both sides an empty string
# and every "unchanged" assertion pass falsely. Here a failed copy or a diff
# error (rc 2, as well as rc 1 for a difference) ends the case.
pin_snapshot() {
    rm -rf "$FIXTURE/snap"
    if ! cp -R "$FIXTURE/pkg/stack/fluxcd" "$FIXTURE/snap"; then
        echo "pin_snapshot: could not snapshot $FIXTURE/pkg/stack/fluxcd -- refusing to continue" >&2
        exit 1
    fi
}

assert_pin_tree_unchanged() {
    if ! diff -r "$FIXTURE/snap" "$FIXTURE/pkg/stack/fluxcd" >/dev/null 2>&1; then
        echo "FAIL: $1" >&2
        diff -r "$FIXTURE/snap" "$FIXTURE/pkg/stack/fluxcd" >&2
        exit 1
    fi
}
