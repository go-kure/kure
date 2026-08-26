#!/bin/bash
# Row 11 (below): range-dep's pin sits below supported_range.
# Two case files, not one looped case: sync-versions.sh:576's range check is
# one combined (( ver_key < lo_key || ver_key > hi_key )) -- a single
# mutated-both-ways fixture entry can't tell you which comparison broke if
# the other is later disabled by a regression.
# Needs run_generate before run_check (see 12a's header comment): the pin
# mutation changes range-dep's build-version column in docs/compatibility.md,
# so validate_docs_drift independently fails too -- proven by reproduction
# (suppressing the range guard's error-counting here still left this case
# passing via docs-drift's unrelated rc=1, verified locally then reverted),
# same defect class as row 2. run_generate regenerates the docs against the
# mutation, so only the range guard can supply this case's rc=1.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
gomod_sub 's|github.com/example/range-dep v2.0.3|github.com/example/range-dep v1.9.0|'
run_generate
assert_rc 0
run_check
assert_rc 1
assert_err_contains 'range-dep 1.9 (go.mod github.com/example/range-dep v1.9.0) is outside supported_range "2.0"'
