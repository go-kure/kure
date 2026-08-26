#!/bin/bash
# Row 24: running generate twice with no mutation between must be a no-op --
# proves generate_docs/generate_go_api are idempotent, not just correct once.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
run_generate
assert_rc 0
run_check
assert_rc 0
assert_out_contains 'compatibility.md is up to date'
assert_out_contains 'versions_gen.go is up to date'
