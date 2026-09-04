#!/bin/bash
# Row 43: widen without --note is a usage error, not a silent no-op -- the
# note is the one place human judgment about compatibility lives, and
# main()'s own argument parsing rejects the call before widen_dependency
# ever runs (the usage line goes to stdout, matching the existing
# "Unknown command" fallback's own plain `echo`, never `error`/stderr).
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
run_widen range-dep 2.1
assert_rc 1
assert_out_contains 'Usage: '
