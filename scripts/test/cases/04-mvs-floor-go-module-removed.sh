#!/bin/bash
# Row 4: go_module (floor-dep) removed from go.mod entirely. Needs
# run_generate before run_check (see 12a's header comment): removing
# floor-dep's require line changes its build-version column in
# docs/compatibility.md from "v0.5.1" to "(transitive)", so
# validate_docs_drift independently fails too -- proven by reproduction
# (suppressing validate_mvs_floors's error-counting here still left this
# case passing via docs-drift's unrelated rc=1, verified locally then
# reverted), same defect class as row 2. run_generate regenerates the docs
# against the removal, so only validate_mvs_floors can supply this case's
# rc=1.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
gomod_sub '/github.com\/example\/floor-dep v0.5.1/d'
run_generate
assert_rc 0
run_check
assert_rc 1
assert_err_contains "floor_module is set but github.com/example/floor-dep was not found in go.mod"
