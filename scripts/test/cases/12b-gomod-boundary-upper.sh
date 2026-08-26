#!/bin/bash
# Row 12 (upper bound): boundary-dep's pin sits exactly at supported_range's
# high end. See 12a's header comment for why run_generate runs first.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
gomod_sub 's|github.com/example/boundary-dep v1.6.0|github.com/example/boundary-dep v1.7.9|'
run_generate
assert_rc 0
run_check
assert_rc 0
assert_out_contains 'boundary-dep: v1.7.9 within supported_range "1.5 - 1.7"'
