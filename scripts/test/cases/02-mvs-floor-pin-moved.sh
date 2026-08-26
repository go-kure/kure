#!/bin/bash
# Row 2: floor-dep's pin moved to another same-major.minor pseudo-version --
# no longer exact-string-equal to what floor-owner requires.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
gomod_sub 's|github.com/example/floor-dep v0.5.1|github.com/example/floor-dep v0.5.1-0.20260101000000-000000000001|'
run_check
assert_rc 1
assert_err_contains "pin v0.5.1-0.20260101000000-000000000001 does not match github.com/example/floor-owner's requirement v0.5.1"
