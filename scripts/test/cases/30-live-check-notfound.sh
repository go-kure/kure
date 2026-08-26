#!/bin/bash
# Row 30: net-dep's upstream_release tag does not exist upstream (API 404) --
# a typo'd upstream_release, distinct from a network failure.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
with_stub_net notfound
run_check
assert_rc 1
assert_err_contains 'tag v2.9.0 does not exist in example/net-dep (confirmed reachable) — check upstream_release for a typo'
