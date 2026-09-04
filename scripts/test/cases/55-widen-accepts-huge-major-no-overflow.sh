#!/bin/bash
# Row 55 (Codex finding on go-kure/kure#765): mm_cmp converted major/minor
# components with bash's $(( )) arithmetic, a 64-bit signed integer -- an
# absurdly large major component (2^64+2 = 18446744073709551618) silently
# wraps to 2. Against range-dep's supported_range "2.0", widening to
# "18446744073709551618.0" is genuinely far above the current bound, but
# the wrapped major (2) compared equal to the current major (2), so widen
# incorrectly refused a legitimately-higher bound. Regression case against
# the arithmetic-based mm_cmp, not the new digit-string comparison --
# proves red before the fix, green after.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
run_widen range-dep 18446744073709551618.0 --note "irrelevant"
assert_rc 0
assert_out_contains 'supported_range widened to "2.0 - 18446744073709551618.0"'
