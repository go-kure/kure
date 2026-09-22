#!/bin/bash
# Row 71 (go-kure/kure#766): mm_cmp's own operand guard is reachable from
# check -- version_mm passes a go.mod pin's major.minor straight to it -- so a
# malformed pin fails loudly with the comparison diagnostic instead of being
# compared digit-by-digit or read as an empty result inside [[ ]].
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
gomod_sub 's|github.com/example/range-dep v2.0.3|github.com/example/range-dep v2.x.3|'
run_check
assert_rc 1
assert_err_contains 'mm_cmp: operand "2.x" is not major.minor'
assert_err_contains 'range-dep: cannot compare go.mod version v2.x.3'
assert_out_not_contains 'range-dep: v2.x.3 within supported_range'
