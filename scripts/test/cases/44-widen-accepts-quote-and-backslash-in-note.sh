#!/bin/bash
# Row 44 (go-kure/kure#802): the note reaches yq through strenv, never through
# the expression text, and generate_go_api never emits notes -- so a note
# containing a double quote or a backslash is written verbatim, and the
# following generate+check round still passes (neither drift guard breaks).
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
gomod_sub 's|github.com/example/range-dep v2.0.3|github.com/example/range-dep v2.1.0|'
run_widen range-dep 2.1 --note 'v2.1 renames "Spec" and C:\path stays'
assert_rc 0
got=$(yq '.infrastructure.range-dep.notes' "$FIXTURE/versions.yaml")
if [[ "$got" != *'v2.1 renames "Spec" and C:\path stays'* ]]; then
    echo "note not written verbatim; notes are now: $got" >&2
    exit 1
fi
run_generate
assert_rc 0
run_check
assert_rc 0

# The docs row must carry the note verbatim even where echo would expand
# escapes (bash with xpg_echo): \c would truncate the row, \n split it.
rm -rf "$FIXTURE"  # the EXIT trap only covers the latest fixture
new_fixture
run_widen range-dep 2.1 --note 'escapes \c and \n stay literal'
assert_rc 0
if ! env BASHOPTS=xpg_echo bash "$SYNC_VERSIONS" generate >/dev/null 2>&1; then
    echo "generate failed under xpg_echo" >&2
    exit 1
fi
if ! grep -qF 'escapes \c and \n stay literal |' "$FIXTURE/docs/compatibility.md"; then
    echo "docs row lost the literal backslashes under xpg_echo:" >&2
    grep -n 'range-dep' "$FIXTURE/docs/compatibility.md" >&2
    exit 1
fi
