#!/bin/bash
# Row 144 (go-kure/kure#1365): like row 135 for the other pin script. A
# `go test -update` that fails part way, a run cancelled by SIGTERM while it
# runs, and a go that dies on its own while its test binary carries on writing
# must all end with the stored outputs as they were: the script ends the whole
# process group of the regeneration before it restores, so no late write lands
# on the restored files.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

golden_rel=pkg/stack/fluxcd/testdata/bootstrap-output

# A run cancelled while `go test` runs (a Renovate job cancelled): the outputs
# it wrote are unvalidated, so the script puts them back on its way out even
# though they are a change it would otherwise accept.
export STUB_GOTK_GO_MODE=term
new_gotk_fixture v0.2.0
golden="$FIXTURE/$golden_rel"
cp -R "$golden" "$FIXTURE/golden-before"
run_gotk
assert_rc 143
if ! diff -r "$FIXTURE/golden-before" "$golden" >/dev/null 2>&1; then
    echo "FAIL: stored outputs not restored after an interrupted regeneration:" >&2
    diff -r "$FIXTURE/golden-before" "$golden" >&2
    exit 1
fi
# Wait past the child's 8 s of writing: a leak would land here, after the check
# above has already passed.
sleep 9
if ! diff -r "$FIXTURE/golden-before" "$golden" >/dev/null 2>&1; then
    echo "FAIL: a write after the interrupted run landed on the restored outputs:" >&2
    diff -r "$FIXTURE/golden-before" "$golden" >&2
    exit 1
fi

# go killed on its own while its test binary carries on writing: the script
# must end what is left of the regeneration before it restores.
export STUB_GOTK_GO_MODE=orphan
new_gotk_fixture v0.2.0
golden="$FIXTURE/$golden_rel"
cp -R "$golden" "$FIXTURE/golden-before"
run_gotk
assert_rc 1
assert_err_contains "regenerating $golden_rel failed"
if ! diff -r "$FIXTURE/golden-before" "$golden" >/dev/null 2>&1; then
    echo "FAIL: a write after go exited landed on the restored outputs:" >&2
    diff -r "$FIXTURE/golden-before" "$golden" >&2
    exit 1
fi
sleep 9
if ! diff -r "$FIXTURE/golden-before" "$golden" >/dev/null 2>&1; then
    echo "FAIL: a write well after go exited landed on the restored outputs:" >&2
    diff -r "$FIXTURE/golden-before" "$golden" >&2
    exit 1
fi
