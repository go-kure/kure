#!/bin/bash
# Row 45: validate_gomod's own range-check error now prints the exact widen
# command to run, with the dependency and detected version already filled
# in -- not just "update supported_range by hand". Same mutation as row
# 11b/above; this case asserts the new error tail, that one still asserts
# the unchanged prefix.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
gomod_sub 's|github.com/example/range-dep v2.0.3|github.com/example/range-dep v2.1.0|'
run_generate
assert_rc 0
run_check
assert_rc 1
assert_err_contains 'After confirming API compatibility: ./scripts/sync-versions.sh widen range-dep 2.1 --note "<compatibility assessment>"'
