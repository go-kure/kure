#!/bin/bash
# Row 16: upstream-dep's go.mod pin digest no longer prefixes
# upstream_release_commit -- the pin has drifted off the declared release
# even though it's still syntactically a pseudo-version. Needs run_generate
# before run_check (see 12a's header comment): the pin mutation changes
# upstream-dep's build-version column in docs/compatibility.md, so
# validate_docs_drift independently fails too -- proven by reproduction
# (suppressing the digest guard's error-counting here still left this case
# passing via docs-drift's unrelated rc=1, verified locally then reverted),
# same defect class as row 2. run_generate regenerates the docs against the
# mutation, so only the digest guard can supply this case's rc=1.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
gomod_sub 's|github.com/example/upstream-dep v0.0.0-20260101000000-eeeeeeeeeeee|github.com/example/upstream-dep v0.0.0-20260101000000-ffffffffffff|'
run_generate
assert_rc 0
run_check
assert_rc 1
assert_err_contains "go.mod pin digest 'ffffffffffff' does not match declared release v3.4.0"
