#!/bin/bash
# Row 21: a 40-hex commit SHA embedded in notes-dep's notes: field.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
yq_set '.infrastructure.notes-dep.notes' "pinned at $(printf 'b%.0s' $(seq 40))"
run_check
assert_rc 1
assert_err_contains 'notes-dep: notes contains a raw commit SHA'
