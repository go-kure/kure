#!/bin/bash
# Row 11 (above): range-dep's pin sits above supported_range. Its own case
# file, isolated from 11a -- see 11a's header comment for why. Also needs
# run_generate before run_check for the same docs-drift-masking reason as
# 11a -- see that file's header comment.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
gomod_sub 's|github.com/example/range-dep v2.0.3|github.com/example/range-dep v2.1.0|'
run_generate
assert_rc 0
run_check
assert_rc 1
assert_err_contains 'range-dep 2.1 (go.mod github.com/example/range-dep v2.1.0) is outside supported_range "2.0"'
