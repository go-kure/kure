#!/bin/bash
# Row 14: pseudo-dep is a pseudo-version with no upstream_release declared --
# the range check must be skipped entirely, not evaluated against its
# deliberately-unsatisfiable supported_range "9.9".
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
run_check
assert_rc 0
assert_out_contains 'pseudo-dep: 0.0.0-20260101000000-000000000099 (pseudo-version, no upstream_release declared — range check skipped)'
