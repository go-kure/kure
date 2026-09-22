#!/bin/bash
# Row 63 (go-kure/kure#802): widen appends the new note as its own line after
# the existing notes instead of replacing them, and writes a '|' literal block
# (not a flattened '|-' line). The notes are the audit trail for the whole
# range; replacing them deleted the reasons for the part of the range that
# still stands.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
run_widen range-dep 2.1 --note "v2.1 adds nothing kure imports."
assert_rc 0
assert_out_contains 'note appended to the existing notes'
want=$'Synthetic range-check dependency for scripts/test/ row 11.\nv2.1 adds nothing kure imports.'
got=$(yq '.infrastructure.range-dep.notes' "$FIXTURE/versions.yaml")
if [[ "$got" != "$want" ]]; then
    printf 'notes mismatch:\n--- want\n%s\n--- got\n%s\n' "$want" "$got" >&2
    exit 1
fi
if ! grep -qE '^    notes: \|$' "$FIXTURE/versions.yaml"; then
    echo "notes is not written as a '|' literal block:" >&2
    grep -n -A3 'range-dep:' "$FIXTURE/versions.yaml" >&2
    exit 1
fi
