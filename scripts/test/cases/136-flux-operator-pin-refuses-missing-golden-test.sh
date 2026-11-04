#!/bin/bash
# Row 136 (go-kure/kure#1259): `go test -run` exits 0 when its regexp matches no
# test, and writes nothing. A golden test renamed or removed would then leave
# its stored outputs on the old bundle digest while the script reported
# success, so the script checks that every test it expects printed a
# "=== RUN" line, and otherwise restores the outputs and fails naming the test.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

export STUB_PIN_GO_MODE=missing
new_pin_fixture v0.2.0
golden="$FIXTURE/pkg/stack/fluxcd/testdata/bootstrap-output"
cp -R "$golden" "$FIXTURE/golden-before"
run_pin
assert_rc 1
assert_err_contains 'go did not run these tests'
assert_err_contains '  TestBootstrap_OutputUnchanged'
# Only the missing test is listed, not the three that ran (the go output above
# the list holds their "=== RUN" lines, which do not start with two spaces).
assert_err_not_contains $'\n  TestBootstrapSource'
if ! diff -r "$FIXTURE/golden-before" "$golden" >/dev/null 2>&1; then
    echo "FAIL: stored outputs not restored after a test did not run:" >&2
    diff -r "$FIXTURE/golden-before" "$golden" >&2
    exit 1
fi

# Control: the same fixture with every test running succeeds, so the refusal
# above is the missing test and nothing else.
export STUB_PIN_GO_MODE=digest
new_pin_fixture v0.2.0
run_pin
assert_rc 0
