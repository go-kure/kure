#!/bin/bash
# Row 12 (lower bound): boundary-dep's pin sits exactly at supported_range's
# low end -- proves the range check is inclusive. Needs run_generate before
# run_check: generate_docs/generate_go_api embed the go.mod build version
# verbatim, so mutating the pin without regenerating first leaves the
# committed docs/go-api stale and validate_docs_drift/validate_go_api_drift
# report rc 1 for a reason unrelated to the boundary check itself.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
gomod_sub 's|github.com/example/boundary-dep v1.6.0|github.com/example/boundary-dep v1.5.0|'
run_generate
assert_rc 0
run_check
assert_rc 0
assert_out_contains 'boundary-dep: v1.5.0 within supported_range "1.5 - 1.7"'
