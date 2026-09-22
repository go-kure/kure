#!/bin/bash
# Row 92 (go-kure/kure#802): `--note --replace-notes` leaves the note out; widen
# must refuse it rather than record the literal option as the assessment, and
# change nothing.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
before=$(cat "$FIXTURE/versions.yaml")
run_widen range-dep 2.1 --note --replace-notes
assert_rc 1
assert_out_contains 'Usage:'
if [[ "$(cat "$FIXTURE/versions.yaml")" != "$before" ]]; then
    echo "versions.yaml changed on a refused widen" >&2
    exit 1
fi
