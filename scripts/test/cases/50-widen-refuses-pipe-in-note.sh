#!/bin/bash
# Row 50 (Codex finding on go-kure/kure#765): widen refuses a --note containing '|'.
# go_string_literal_unsafe (row 44) only guards Go-string-literal safety;
# notes is also rendered raw into a Markdown table cell by generate_docs,
# where a pipe would open an extra column, so this needs its own check.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
run_widen range-dep 2.1 --note 'has a | pipe'
assert_rc 1
assert_err_contains "widen: --note text cannot contain '|'"
