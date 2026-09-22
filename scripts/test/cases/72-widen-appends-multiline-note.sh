#!/bin/bash
# Row 72 (go-kure/kure#802): a multi-line --note appended to multi-line
# history keeps every existing line and every new line, in order, as one '|'
# literal block -- and the following generate+check round still passes.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
NOTES=$'First paragraph of history.\nSecond line of history.\n' yq eval -i '.infrastructure.range-dep.notes = strenv(NOTES) | .infrastructure.range-dep.notes style="literal"' "$FIXTURE/versions.yaml"
gomod_sub 's|github.com/example/range-dep v2.0.3|github.com/example/range-dep v2.1.0|'
run_widen range-dep 2.1 --note $'v2.1 adds one field.\nNo type kure imports changed.'
assert_rc 0
want=$'First paragraph of history.\nSecond line of history.\nv2.1 adds one field.\nNo type kure imports changed.'
got=$(yq '.infrastructure.range-dep.notes' "$FIXTURE/versions.yaml")
if [[ "$got" != "$want" ]]; then
    printf 'notes mismatch:\n--- want\n%s\n--- got\n%s\n' "$want" "$got" >&2
    exit 1
fi
if ! grep -qE '^    notes: \|$' "$FIXTURE/versions.yaml"; then
    echo "notes is not written as a '|' literal block" >&2
    exit 1
fi
run_generate
assert_rc 0
run_check
assert_rc 0
