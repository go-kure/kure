#!/bin/bash
# Row 57: a go.mod pin that is not a plain vX.Y.Z tag (a pseudo-version, here)
# has no release asset to vendor. The script must refuse -- before any
# download -- rather than fetch some other release and stamp the wrong
# version into the constant.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_pin_fixture v0.2.1-0.20260101000000-abcdef123456
pin_snapshot
run_pin
assert_rc 1
assert_err_contains 'not a plain vX.Y.Z release tag'
if [[ -e "$FIXTURE/curl.log" ]]; then
    echo "FAIL: script downloaded something despite refusing the version" >&2
    exit 1
fi
assert_pin_tree_unchanged "refused run modified the tree"
