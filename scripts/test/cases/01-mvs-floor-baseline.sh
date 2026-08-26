#!/bin/bash
# Row 1: baseline (no mutation) -- validate_mvs_floors' success path.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
run_check
assert_rc 0
assert_out_contains "floor-dep: pin v0.5.1 matches the floor set by github.com/example/floor-owner"
