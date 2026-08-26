#!/bin/bash
# Row 8: `go list` prints a path that does not exist on disk.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
with_stub_go badpath
run_check
assert_rc 0
assert_out_contains "could not resolve github.com/example/floor-owner's own go.mod (no Go, cold module cache, or no network) -- skipping MVS-floor equality check"
