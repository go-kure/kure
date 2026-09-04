#!/bin/bash
# Row 44: widen refuses a --note value that cannot be safely embedded as a
# YAML/Go string literal -- the same go_string_literal_unsafe guard
# generate_go_api applies to every field it emits, reused here so a note
# widen writes can never itself break the go-api drift guard downstream.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
run_widen range-dep 2.1 --note 'contains a "quote"'
assert_rc 1
assert_err_contains "widen: --note text cannot be emitted as a YAML/Go string literal"
