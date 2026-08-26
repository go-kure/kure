#!/bin/bash
# Row 13: version_basis: kubernetes normalizes k8s-basis-dep's v0.36.4 (major 0)
# to mm "1.36" before comparing against supported_range "1.36" -- without the
# normalization this would report outside "1.36" (its raw mm is "0.36").
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
run_check
assert_rc 0
assert_out_contains 'k8s-basis-dep: v0.36.4 within supported_range "1.36"'
