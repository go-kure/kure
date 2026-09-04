#!/bin/bash
# Row 40: widen refuses a floor_module entry -- its range is never enforced
# by validate_gomod (see that guard's own skip), so there is nothing to widen.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
run_widen floor-dep 9.9 --note "irrelevant"
assert_rc 1
assert_err_contains "widen: floor-dep declares floor_module (github.com/example/floor-owner) -- its range is never range-checked and there is nothing to widen."
