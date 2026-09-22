#!/bin/bash
# Row 65 (go-kure/kure#802): when the existing notes is not a string widen can
# read, the default (append) path fails loudly and leaves versions.yaml
# untouched -- supported_range included -- instead of overwriting the notes.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
yq eval -i '.infrastructure.range-dep.notes = ["a", "b"]' "$FIXTURE/versions.yaml"
before=$(cat "$FIXTURE/versions.yaml")
run_widen range-dep 2.1 --note "v2.1 adds nothing kure imports."
assert_rc 1
assert_err_contains "notes is a !!seq, not a string -- cannot append to it"
if [[ "$(cat "$FIXTURE/versions.yaml")" != "$before" ]]; then
    echo "versions.yaml changed on a refused widen" >&2
    exit 1
fi
