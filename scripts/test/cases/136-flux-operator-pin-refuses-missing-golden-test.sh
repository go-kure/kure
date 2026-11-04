#!/bin/bash
# Row 136 (go-kure/kure#1259): `go test -run` exits 0 when its regexp matches no
# test, or when a test skips itself, and writes nothing either way. A golden test
# renamed, removed or skipped would then leave its stored outputs on the old
# bundle digest while the script reported success, so the script checks that
# every test it expects printed both its "=== RUN" line and its "--- PASS:" line
# (a skip prints "=== RUN" but "--- SKIP"), and otherwise restores the outputs
# and fails naming the test.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

# Mode missing: no RUN line for the first test (renamed or removed). Mode
# skipped: RUN and SKIP, no PASS (the test calls t.Skip).
for mode in missing skipped; do
    export STUB_PIN_GO_MODE=$mode
    new_pin_fixture v0.2.0
    golden="$FIXTURE/pkg/stack/fluxcd/testdata/bootstrap-output"
    cp -R "$golden" "$FIXTURE/golden-before"
    run_pin
    assert_rc 1
    assert_err_contains 'did not run and pass'
    assert_err_contains '  TestBootstrap_OutputUnchanged'
    # Only the affected test is listed, not the three that passed (the go output
    # above the list holds their "=== RUN" lines, which do not start with two
    # spaces).
    assert_err_not_contains $'\n  TestBootstrapSource'
    if ! diff -r "$FIXTURE/golden-before" "$golden" >/dev/null 2>&1; then
        echo "FAIL: ($mode) stored outputs not restored after a test did not pass:" >&2
        diff -r "$FIXTURE/golden-before" "$golden" >&2
        exit 1
    fi
done

# Control: the same fixture with every test passing succeeds, so the refusals
# above are the missing or skipped test and nothing else.
export STUB_PIN_GO_MODE=digest
new_pin_fixture v0.2.0
run_pin
assert_rc 0
