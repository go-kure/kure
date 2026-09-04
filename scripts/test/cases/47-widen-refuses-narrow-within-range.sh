#!/bin/bash
# Row 47 (Codex finding on kure#765): widen must compare against the current
# UPPER bound, not the lower one. boundary-dep's supported_range is
# "1.5 - 1.7" -- "1.6" is above the lower bound (1.5) but at/below the
# current upper bound (1.7), so accepting it would silently narrow the
# range and drop declared support for 1.7 while still reporting "widened".
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
run_widen boundary-dep 1.6 --note "irrelevant"
assert_rc 1
assert_err_contains 'widen: new upper bound "1.6" is not above the current upper bound "1.7"'
