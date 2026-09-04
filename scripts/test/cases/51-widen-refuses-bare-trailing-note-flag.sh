#!/bin/bash
# Row 51 (kure-bot finding on kure#765's Codex re-review): `--note` passed
# with no value left only 1 positional param when the case handler tried
# `shift 2`. Under this script's own `set -e`, that failing shift aborted
# the whole process with no usage message instead of falling through to
# the usual "Usage: ..." handling.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
run_widen range-dep 2.1 --note
assert_rc 1
assert_out_contains 'Usage: '
