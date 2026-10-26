#!/bin/bash
# Row 9: `go list` hangs; SYNC_VERSIONS_PROBE_TIMEOUT bounds it to ~2s instead
# of the production 10s default, and the result must still be a warning
# (rc 0), not a hang.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
with_stub_go hang
# 2s, not 1s: the stub must start and arm its TERM trap before the timeout
# fires, or a loaded host kills it unlogged.
export SYNC_VERSIONS_PROBE_TIMEOUT=2
export STUB_GO_HANG_LOG="$FIXTURE/go-hang.log"

run_check

assert_rc 0
assert_out_contains "could not resolve github.com/example/floor-owner's own go.mod (no Go, cold module cache, or no network) -- skipping MVS-floor equality check"

# Time the probe, not the run: the stub logs whether the timeout killed it and
# its own age at that moment, counted from when `timeout` launched it, so
# contention in check's other guards cannot fail this and a slow probe start
# cannot hide an ignored override. A probe that ran to completion had no
# timeout at all; one killed at 6s or later ignored the 2s override
# (production default is 10s, sync-versions.sh's
# `${SYNC_VERSIONS_PROBE_TIMEOUT:-10}`), while 6s leaves 4s for a loaded host
# to deliver the TERM late.
if [[ ! -f "$STUB_GO_HANG_LOG" ]]; then
    echo "FAIL: the hang stub never ran (no $STUB_GO_HANG_LOG)" >&2
    exit 1
fi
starts=$(grep -c '^start$' "$STUB_GO_HANG_LOG")
killed_age=$(awk '$1 == "killed" { print $2 }' "$STUB_GO_HANG_LOG")
if [[ $starts -ne 1 ]]; then
    echo "FAIL: expected the hang stub to run once, it ran $starts times:" >&2
    cat "$STUB_GO_HANG_LOG" >&2
    exit 1
fi
if ! grep -q '^killed' "$STUB_GO_HANG_LOG"; then
    echo "FAIL: expected the timeout to kill the hung probe, it was not killed:" >&2
    cat "$STUB_GO_HANG_LOG" >&2
    exit 1
fi
if [[ ! $killed_age =~ ^[0-9]+$ ]]; then
    echo "FAIL: the hang stub could not read its own age when killed:" >&2
    cat "$STUB_GO_HANG_LOG" >&2
    exit 1
fi
if [[ $killed_age -ge 6 ]]; then
    echo "FAIL: expected the 2s override to bound the hung probe, it was killed at ${killed_age}s" >&2
    exit 1
fi
