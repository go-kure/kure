#!/bin/bash
# Row 29: net-dep's upstream_release tag resolves live to a commit different
# from the declared upstream_release_commit -- versions.yaml was hand-edited
# out of sync with reality, independent of go.mod.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
with_stub_net mismatch
run_check
assert_rc 1
assert_err_contains 'does not match the commit example/net-dep@v2.9.0 resolves to today'
