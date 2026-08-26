#!/bin/bash
# Row 7: `go` is entirely unresolvable on PATH -- the offline invariant.
# Must degrade to a warning (rc 0), never a hard failure.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
with_stub_go absent
run_check
assert_rc 0
assert_out_contains "could not resolve github.com/example/floor-owner's own go.mod (no Go, cold module cache, or no network) -- skipping MVS-floor equality check"
