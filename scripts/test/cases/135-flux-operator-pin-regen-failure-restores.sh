#!/bin/bash
# Row 135 (go-kure/kure#1150): a `go test -update` that fails part way (a
# compile error, a test that fails before or after writing) may have written
# some stored outputs already. The script must put them all back and fail.
# A tree with no stored flux-operator output at all is refused before any
# download, like a missing rewrite anchor (row 59).
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

export STUB_PIN_GO_MODE=fail
new_pin_fixture v0.2.0
golden="$FIXTURE/pkg/stack/fluxcd/testdata/bootstrap-output"
cp -R "$golden" "$FIXTURE/golden-before"
run_pin
assert_rc 1
assert_err_contains 'regenerating pkg/stack/fluxcd/testdata/bootstrap-output failed'
if ! diff -r "$FIXTURE/golden-before" "$golden" >/dev/null 2>&1; then
    echo "FAIL: stored outputs not restored after a failed regeneration:" >&2
    diff -r "$FIXTURE/golden-before" "$golden" >&2
    exit 1
fi

export STUB_PIN_GO_MODE=digest
new_pin_fixture v0.2.0
rm "$FIXTURE"/pkg/stack/fluxcd/testdata/bootstrap-output/flux-operator-*.txt
pin_snapshot
run_pin
assert_rc 1
assert_err_contains 'no stored bootstrap output found'
assert_pin_tree_unchanged "a tree without stored outputs was changed"
if [[ -e "$FIXTURE/curl.log" ]]; then
    echo "FAIL: downloaded although the stored outputs are missing" >&2
    exit 1
fi
