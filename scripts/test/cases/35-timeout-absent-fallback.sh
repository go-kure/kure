#!/bin/bash
# Row 35: no `timeout`/`gtimeout` binary available -- resolve_timeout_bin
# falls back to running the two bounded probes unbounded, and warns once at
# startup. Load-bearing assertion: the MVS-floor guard must still guard (the
# same floor-match success line as case 01), proving the guard doesn't
# silently stop guarding just because there's no timeout binary.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
export SYNC_VERSIONS_TIMEOUT_CMD=""
run_check
assert_rc 0
assert_out_contains "floor-dep: pin v0.5.1 matches the floor set by github.com/example/floor-owner"
assert_out_contains "will run unbounded"
