#!/bin/bash
# Row 22: a 12-hex (pseudo-version-length) commit SHA embedded in notes-dep's
# notes: field -- the regex must catch both lengths, not just 40. Also needs
# run_generate before run_check for the same docs-drift-masking reason as
# row 21 -- see that file's header comment (and 23's, which documents it
# first for this same notes-dep field).
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
yq_set '.infrastructure.notes-dep.notes' "pinned at $(printf 'b%.0s' {1..12})"
run_generate
assert_rc 0
run_check
assert_rc 1
assert_err_contains 'notes-dep: notes contains a raw commit SHA'
