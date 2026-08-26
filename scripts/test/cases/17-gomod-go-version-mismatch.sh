#!/bin/bash
# Row 17: go.mod's go line no longer matches versions.yaml's go.current.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
gomod_sub 's|^go 1.26.6|go 1.26.7|'
run_check
assert_rc 1
assert_err_contains "Go version mismatch: go.mod has '1.26.7', versions.yaml expects '1.26.6'"
