#!/bin/bash
# Row 3: floor_module points at a module that is not a go.mod dependency at all.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
yq_set '.infrastructure.floor-dep.floor_module' 'github.com/example/nowhere'
run_check
assert_rc 1
assert_err_contains "floor_module github.com/example/nowhere (claimed to set the floor for github.com/example/floor-dep) was not found in go.mod"
