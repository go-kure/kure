#!/bin/bash
# Row 54 (Codex finding on go-kure/kure#765): mm_key encoded "major.minor" as
# major*1000+minor, so a minor component >= 1000 collides with the next
# major (1.1001 -> 2001, same bucket as 2.0 -> 2000). Against range-dep's
# supported_range "2.0", widen 1.1001 is semantically LOWER (major 1 < 2)
# but the fixed-multiplier key made it compare as higher, letting widen
# accept it and write the inverted range "2.0 - 1.1001". This is a
# regression case against the old encoding, not the new comparison --
# proves red before the mm_key fix, green after.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
run_widen range-dep 1.1001 --note "irrelevant"
assert_rc 1
assert_err_contains "widen: new upper bound \"1.1001\" is not above the current upper bound \"2.0\""
