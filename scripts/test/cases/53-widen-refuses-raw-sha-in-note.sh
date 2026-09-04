#!/bin/bash
# Row 53 (Codex finding on go-kure/kure#765): widen's note validation covered
# unsafe string literals and Markdown pipes, but not a raw commit SHA -- the
# check guard (validate_no_sha_in_notes) rejects one, so widen writing an
# unchecked note created a versions.yaml that only failed at the next check,
# not up front. Mirrors validate_no_sha_in_notes's own regex.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
run_widen range-dep 2.1 --note "pinned at abcdef012345 for now"
assert_rc 1
assert_err_contains "widen: --note text contains a raw commit SHA"
