#!/bin/bash
# Row 21: a 40-hex commit SHA embedded in notes-dep's notes: field. Needs
# run_generate before run_check (see 23's header comment, which already
# documents this for notes-dep mutations): generate_docs embeds notes
# verbatim into docs/compatibility.md's table, so a notes change without
# regenerating leaves the committed docs stale and validate_docs_drift
# independently fails too -- proven by reproduction (suppressing
# validate_no_sha_in_notes' own error-counting here still left this case
# passing via docs-drift's unrelated rc=1, verified locally then reverted),
# same defect class as row 2. run_generate regenerates the docs against the
# mutation, so only validate_no_sha_in_notes can supply this case's rc=1.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
yq_set '.infrastructure.notes-dep.notes' "pinned at $(printf 'b%.0s' $(seq 40))"
run_generate
assert_rc 0
run_check
assert_rc 1
assert_err_contains 'notes-dep: notes contains a raw commit SHA'
