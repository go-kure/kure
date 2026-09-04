#!/bin/bash
# Row 48 (Codex finding on go-kure/kure#765): a pin below supported_range's LOWER
# bound must not recommend `widen` -- that command only ever raises the
# upper bound and would refuse this exact value (see row 41/47), so
# printing it here would hand the human a command guaranteed to fail.
# Same mutation as row 11a; that case asserts the unchanged prefix, this one
# asserts the new below-range wording and the absence of any widen command.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
gomod_sub 's|github.com/example/range-dep v2.0.3|github.com/example/range-dep v1.9.0|'
run_generate
assert_rc 0
run_check
assert_rc 1
assert_err_contains 'range-dep 1.9 (go.mod github.com/example/range-dep v1.9.0) is below supported_range "2.0"'
assert_err_contains "this is not something 'widen' can fix"
assert_err_not_contains './scripts/sync-versions.sh widen'
