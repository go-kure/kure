#!/bin/bash
# Row 49 (Codex finding on kure#765): widen requires an exact major.minor
# upper bound. mm_key truncates anything past the second component, so
# "2.1.0" would otherwise compare equal to "2.1" and pass through, then get
# written verbatim into supported_range -- leaving Min/Max outside their
# documented major.minor format.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
run_widen range-dep 2.1.0 --note "irrelevant"
assert_rc 1
assert_err_contains 'widen: new upper bound "2.1.0" is not major.minor'
