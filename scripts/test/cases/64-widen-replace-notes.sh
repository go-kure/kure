#!/bin/bash
# Row 64 (go-kure/kure#802): replacing the notes stays reachable, but only
# behind the explicit --replace-notes flag, and says so in its output.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
run_widen range-dep 2.1 --note "Rewritten from scratch." --replace-notes
assert_rc 0
assert_out_contains 'notes replaced (--replace-notes)'
got=$(yq '.infrastructure.range-dep.notes' "$FIXTURE/versions.yaml")
if [[ "$got" != "Rewritten from scratch." ]]; then
    echo "expected the notes to be replaced, got: $got" >&2
    exit 1
fi
