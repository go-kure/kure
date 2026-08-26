#!/bin/bash
# Row 11 (below): range-dep's pin sits below supported_range.
# Two case files, not one looped case: sync-versions.sh:576's range check is
# one combined (( ver_key < lo_key || ver_key > hi_key )) -- a single
# mutated-both-ways fixture entry can't tell you which comparison broke if
# the other is later disabled by a regression.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
gomod_sub 's|github.com/example/range-dep v2.0.3|github.com/example/range-dep v1.9.0|'
run_check
assert_rc 1
assert_err_contains 'range-dep 1.9 (go.mod github.com/example/range-dep v1.9.0) is outside supported_range "2.0"'
