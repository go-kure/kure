#!/bin/bash
# Row 25: hand-edit the committed docs/compatibility.md so it no longer
# matches what generate_docs would produce right now.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
printf '\nhand-edited, not regenerated\n' >> "$FIXTURE/docs/compatibility.md"
run_check
assert_rc 1
assert_err_contains 'compatibility.md is out of date'
