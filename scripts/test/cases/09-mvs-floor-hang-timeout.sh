#!/bin/bash
# Row 9: `go list` hangs; SYNC_VERSIONS_PROBE_TIMEOUT bounds it to ~1s instead
# of the production 10s default, and the result must still be a warning
# (rc 0), not a hang.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
with_stub_go hang
export SYNC_VERSIONS_PROBE_TIMEOUT=1

start=$(date +%s)
run_check
elapsed=$(( $(date +%s) - start ))

assert_rc 0
assert_out_contains "could not resolve github.com/example/floor-owner's own go.mod (no Go, cold module cache, or no network) -- skipping MVS-floor equality check"

if [[ $elapsed -ge 4 ]]; then
    echo "FAIL: expected the 1s override to bound the hang, took ${elapsed}s" >&2
    exit 1
fi
