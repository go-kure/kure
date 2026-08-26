#!/bin/bash
# Row 6: `go mod edit` fails outright -- distinct from row 5's clean "not
# required" result (guards #707's R2-02 fix: the two must not be conflated).
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
with_stub_go editfail
run_check
assert_rc 1
assert_err_contains "could not read github.com/example/floor-owner's go.mod requirements"
assert_err_contains "rc=3"
