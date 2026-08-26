#!/bin/bash
# Row 22: a 12-hex (pseudo-version-length) commit SHA embedded in notes-dep's
# notes: field -- the regex must catch both lengths, not just 40.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
yq_set '.infrastructure.notes-dep.notes' "pinned at $(printf 'b%.0s' $(seq 12))"
run_check
assert_rc 1
assert_err_contains 'notes-dep: notes contains a raw commit SHA'
