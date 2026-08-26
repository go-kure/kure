#!/bin/bash
# Row 16: upstream-dep's go.mod pin digest no longer prefixes
# upstream_release_commit -- the pin has drifted off the declared release
# even though it's still syntactically a pseudo-version.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
gomod_sub 's|github.com/example/upstream-dep v0.0.0-20260101000000-eeeeeeeeeeee|github.com/example/upstream-dep v0.0.0-20260101000000-ffffffffffff|'
run_check
assert_rc 1
assert_err_contains "go.mod pin digest 'ffffffffffff' does not match declared release v3.4.0"
