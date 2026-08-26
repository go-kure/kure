#!/bin/bash
# Row 5: floor-owner's go.mod does not require floor-dep at all.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
with_stub_go norequire
run_check
assert_rc 1
assert_err_contains "github.com/example/floor-owner's go.mod does not require github.com/example/floor-dep at all"
