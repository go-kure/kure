#!/bin/bash
# Row 46: generate_go_api actually threads FloorModule through to the
# generated Dependency literal -- not just that generate_docs' "derived
# from" rendering and generate_go_api's null-out happen to agree (row 38
# only proves validate_gomod's skip, and never inspects versions_gen.go).
# floor-dep must carry FloorModule + an empty range; range-dep (an ordinary
# hand-maintained-range entry) must carry an empty FloorModule and keep its
# real range.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
run_generate
assert_rc 0

gen="$FIXTURE/pkg/versions/versions_gen.go"
floor_block=$(awk '/Name:.*"floor-dep"/,/\t\},/' "$gen")
range_block=$(awk '/Name:.*"range-dep"/,/\t\},/' "$gen")

if [[ "$floor_block" != *'FloorModule:'*'"github.com/example/floor-owner"'* ]]; then
    echo "FAIL: floor-dep's generated entry is missing FloorModule=github.com/example/floor-owner" >&2
    echo "--- floor-dep block ---" >&2
    echo "$floor_block" >&2
    exit 1
fi
if [[ "$floor_block" != *'SupportedRange: ""'* ]]; then
    echo "FAIL: floor-dep's generated entry should have an empty SupportedRange" >&2
    echo "--- floor-dep block ---" >&2
    echo "$floor_block" >&2
    exit 1
fi
if [[ "$range_block" != *'FloorModule:    ""'* ]]; then
    echo "FAIL: range-dep's generated entry should have an empty FloorModule" >&2
    echo "--- range-dep block ---" >&2
    echo "$range_block" >&2
    exit 1
fi
if [[ "$range_block" == *'SupportedRange: ""'* ]]; then
    echo "FAIL: range-dep's generated entry lost its real SupportedRange" >&2
    echo "--- range-dep block ---" >&2
    echo "$range_block" >&2
    exit 1
fi
