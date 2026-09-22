#!/bin/bash
# Row 66 (go-kure/kure#766): a hand-edited, non-numeric supported_range
# ("1.x - 3.x") must fail check. Before the fix both comparisons resolved on
# the differing major components (2 vs 3, 2 vs 1) without ever reading the
# non-numeric minor, so check reported the pin "within" the range.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
yq eval -i '.infrastructure.range-dep.supported_range = "1.x - 3.x"' "$FIXTURE/versions.yaml"
run_check
assert_rc 1
assert_err_contains 'range-dep: supported_range "1.x - 3.x" is malformed'
assert_out_not_contains 'range-dep: v2.0.3 within supported_range'

# A repeated separator must fail too, not be read as its first and last bound.
rm -rf "$FIXTURE"  # the EXIT trap only covers the latest fixture
new_fixture
yq eval -i '.infrastructure.range-dep.supported_range = "1.0 - garbage - 3.0"' "$FIXTURE/versions.yaml"
run_check
assert_rc 1
assert_err_contains 'range-dep: supported_range "1.0 - garbage - 3.0" is malformed'
