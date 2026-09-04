#!/bin/bash
# Row 41: widen refuses a "new" upper bound that is not actually above the
# current UPPER bound -- e.g. a typo'd version, or an accidental narrowing.
# range-dep's supported_range is a single value ("2.0", lo == hi), so this
# case can't tell a lo-comparison bug from a hi-comparison bug on its own --
# row 47 covers the real "1.5 - 1.7" narrowing scenario a single-value range
# can't exercise.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
run_widen range-dep 1.9 --note "irrelevant"
assert_rc 1
assert_err_contains 'widen: new upper bound "1.9" is not above the current upper bound "2.0"'
