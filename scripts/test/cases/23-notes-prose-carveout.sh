#!/bin/bash
# Row 23: the deliberate carve-out -- ordinary semver prose in notes: (no bare
# hex SHA) must pass. run_generate first: generate_docs embeds notes verbatim
# into docs/compatibility.md's table, so a notes change without regenerating
# leaves the committed docs stale and validate_docs_drift fails for a reason
# unrelated to what this case is proving.
#
# Two things make this case able to FAIL, which an earlier version of it could
# not do (measured: with the yq_set line below removed it still exited 0,
# because the assertion is validate_no_sha_in_notes' validator-wide success
# line, which the untouched -- also SHA-free -- baseline fixture emits too):
#
#   1. The mutation is read back. `yq eval -i '.a.b = "x"'` CREATES a missing
#      path instead of erroring, so renaming the notes-dep fixture entry would
#      otherwise silently turn this case vacuous rather than failing loudly.
#   2. The prose is boundary-shaped, not merely SHA-free: an 11-hex and a
#      13-hex word sit either side of the 12-hex pattern at
#      sync-versions.sh:309. Loosening `{12}` to `{11,}` makes the 11-hex word
#      match; dropping the `\b` anchors makes the 13-hex word match. Either
#      regression flips this case to rc 1.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture

CARVEOUT_NOTES='v0.16.0 is a minor release; refs abcdefabcde and abcdefabcdefa are not commit SHAs.'
yq_set '.infrastructure.notes-dep.notes' "$CARVEOUT_NOTES"

landed=$(yq '.infrastructure.notes-dep.notes' "$FIXTURE/versions.yaml")
if [[ "$landed" != "$CARVEOUT_NOTES" ]]; then
    echo "FAIL: the notes mutation did not land -- versions.yaml has: $landed" >&2
    echo "      (expected: $CARVEOUT_NOTES)" >&2
    exit 1
fi

run_generate
assert_rc 0
run_check
assert_rc 0
assert_out_contains 'No raw commit SHAs in versions.yaml notes'
