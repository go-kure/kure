#!/bin/bash
# Row 10: an entry with no floor_module gets no success or error line from
# validate_mvs_floors at all -- it `continue`s before printing anything.
# range-dep (a validate_gomod entry, not an mvs-floor one) stands in for
# "any entry lacking floor_module"; its own validate_gomod output uses a
# different message shape ("range-dep: vX within supported_range"), so it
# cannot be confused with the mvs-floors "pin ... matches the floor" shape
# this case is proving absent.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
run_check
assert_rc 0
assert_out_not_contains "range-dep: pin"
