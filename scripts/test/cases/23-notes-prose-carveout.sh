#!/bin/bash
# Row 23: the deliberate carve-out -- ordinary semver prose in notes: (no bare
# hex SHA) must pass. run_generate first: generate_docs embeds notes verbatim
# into docs/compatibility.md's table, so a notes change without regenerating
# leaves the committed docs stale and validate_docs_drift fails for a reason
# unrelated to what this case is proving.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
yq_set '.infrastructure.notes-dep.notes' 'v0.16.0 is a minor release; no API changes.'
run_generate
assert_rc 0
run_check
assert_rc 0
assert_out_contains 'No raw commit SHAs in versions.yaml notes'
