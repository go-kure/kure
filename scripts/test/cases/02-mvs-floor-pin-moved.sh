#!/bin/bash
# Row 2: floor-dep's pin moved to another same-major.minor pseudo-version --
# no longer exact-string-equal to what floor-owner requires. Needs
# run_generate before run_check (see 12a's header comment): without it,
# validate_docs_drift independently fails too (docs/compatibility.md still
# embeds the pre-mutation pin), so rc=1 alone would not prove
# validate_mvs_floors's own return path fired -- proven by reproduction: with
# validate_mvs_floors's error-counting deliberately suppressed, this case
# still passed via docs-drift's unrelated rc=1 (verified locally, then
# reverted). run_generate regenerates the docs against the moved pin, so
# only validate_mvs_floors can supply this case's rc=1.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
gomod_sub 's|github.com/example/floor-dep v0.5.1|github.com/example/floor-dep v0.5.1-0.20260101000000-000000000001|'
run_generate
assert_rc 0
run_check
assert_rc 1
assert_err_contains "pin v0.5.1-0.20260101000000-000000000001 does not match github.com/example/floor-owner's requirement v0.5.1"
