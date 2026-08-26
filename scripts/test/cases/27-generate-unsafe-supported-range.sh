#!/bin/bash
# Row 27: supported_range containing a double-quote can't be emitted as a Go
# string literal by generate_go_api -- it must refuse rather than emit
# uncompilable output. Uses yq eval -i directly (not yq_set): the value
# itself contains the quote character yq_set's own wrapping can't carry.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
yq eval -i '.infrastructure.range-dep.supported_range = "2.0\"bad"' "$FIXTURE/versions.yaml"
run_generate
assert_rc 1
assert_err_contains 'versions.yaml value cannot be emitted as a Go string literal: 2.0"bad'
