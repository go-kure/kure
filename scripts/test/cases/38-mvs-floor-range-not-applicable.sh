#!/bin/bash
# Row 38 (validate_gomod's floor_module skip -- go-kure/kure#703's other half):
# an entry that declares BOTH floor_module and a supported_range must not be
# range-checked, even when the range would otherwise reject the actual pin.
# floor-dep's real go.mod pin is v0.5.1; "0.4" deliberately excludes it, so
# before this guard change validate_gomod would have errored "floor-dep 0.5
# ... is outside supported_range \"0.4\"". validate_mvs_floors' own
# equality check (exercised unmutated by row 1's baseline) is unaffected and
# still passes, so rc stays 0 -- only the new skip line proves the guard ran
# at all instead of the range check having coincidentally passed.
# No run_generate needed: docs/compatibility.md's Deployment Compatibility
# cell for a floor_module entry renders "derived from <floor_module>"
# unconditionally (see generate_docs), so adding supported_range here does
# not change generated output and cannot drift the fixture's own baseline.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
yq_set ".infrastructure.floor-dep.supported_range" "0.4"
run_check
assert_rc 0
assert_out_contains "floor-dep: range check not applicable (pin v0.5.1 derived from github.com/example/floor-owner's own go.mod, see validate_mvs_floors)"
assert_out_contains "floor-dep: pin v0.5.1 matches the floor set by github.com/example/floor-owner"
