#!/bin/bash
# Row 39 (widen subcommand, happy path + round-trip): range-dep's pin moves
# above its declared supported_range (same mutation as row 11b/above); widen
# raises the upper bound and replaces the notes text, and a subsequent
# generate+check round then passes. Proves widen's edit is what actually
# unblocks validate_gomod, not just that the subcommand itself exits 0.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
gomod_sub 's|github.com/example/range-dep v2.0.3|github.com/example/range-dep v2.1.0|'
run_widen range-dep 2.1 --note "v2.1 is a routine minor, no API changes affecting the imported types."
assert_rc 0
assert_out_contains 'range-dep: supported_range widened to "2.0 - 2.1"; notes replaced'
assert_out_contains 'Next: ./scripts/sync-versions.sh generate'

# widen's own "Next:" line is a separate, explicit step -- run it before
# check, exactly as a human following that message would.
run_generate
assert_rc 0
run_check
assert_rc 0
assert_out_contains 'range-dep: v2.1.0 within supported_range "2.0 - 2.1"'
