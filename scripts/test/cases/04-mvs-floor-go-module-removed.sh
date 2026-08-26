#!/bin/bash
# Row 4: go_module (floor-dep) removed from go.mod entirely.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
gomod_sub '/github.com\/example\/floor-dep v0.5.1/d'
run_check
assert_rc 1
assert_err_contains "floor_module is set but github.com/example/floor-dep was not found in go.mod"
