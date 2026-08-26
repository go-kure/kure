#!/bin/bash
# Row 36: a real `timeout`/`gtimeout` is on PATH (the harness's own default
# environment) -- resolve_timeout_bin must resolve it and never print the
# unbounded-fallback warning. Cheap, and it is the oracle for case 35:
# without this case, a resolver stuck permanently in fallback mode (e.g. one
# that ignores SYNC_VERSIONS_TIMEOUT_CMD's override the other way, or never
# actually finds `timeout` on PATH) would leave case 35 green for the wrong
# reason.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
run_check
assert_rc 0
assert_out_not_contains "will run unbounded"
