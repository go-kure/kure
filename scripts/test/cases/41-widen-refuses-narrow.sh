#!/bin/bash
# Row 41: widen refuses a "new" upper bound that is not actually above the
# current lower bound -- e.g. a typo'd version, or an accidental narrowing.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
run_widen range-dep 1.9 --note "irrelevant"
assert_rc 1
assert_err_contains 'widen: new upper bound "1.9" is not above the current lower bound "2.0"'
