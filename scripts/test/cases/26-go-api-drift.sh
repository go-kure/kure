#!/bin/bash
# Row 26: hand-edit the committed pkg/versions/versions_gen.go so it no longer
# matches what generate_go_api would produce right now.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
printf '\n// hand-edited, not regenerated\n' >> "$FIXTURE/pkg/versions/versions_gen.go"
run_check
assert_rc 1
assert_err_contains 'versions_gen.go is out of date'
