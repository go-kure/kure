#!/bin/bash
# Row 139 (go-kure/kure#1365): a go.mod pin that is not a plain vX.Y.Z tag (a
# pseudo-version or a pre-release) has no release asset to vendor. The script
# must refuse before any download, rather than fetch another release and stamp
# the wrong version into the constant, and a go.mod that does not require the
# module at all is refused too.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

for version in v2.10.2-0.20260101000000-abcdef123456 v2.11.0-rc.1; do
    new_gotk_fixture "$version"
    gotk_snapshot
    run_gotk
    assert_rc 1
    assert_err_contains 'not a plain vX.Y.Z release tag'
    if [[ -e "$FIXTURE/curl.log" ]]; then
        echo "FAIL: script downloaded something despite refusing $version" >&2
        exit 1
    fi
    assert_gotk_tree_unchanged "refused run modified the tree ($version)"
done

new_gotk_fixture v0.2.0
sed -i '/example\/flux2/d' "$FIXTURE/go.mod"
gotk_snapshot
run_gotk
assert_rc 1
assert_err_contains 'go.mod does not require'
assert_gotk_tree_unchanged "run on a go.mod without the module modified the tree"
