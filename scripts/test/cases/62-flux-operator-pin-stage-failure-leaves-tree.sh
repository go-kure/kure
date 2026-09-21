#!/bin/bash
# Row 62 (Codex finding on go-kure/kure#829): the script replaced the bundle
# first and rewrote the constant and README afterwards, so a failure in the
# later writes left a new bundle beside an old constant -- the exact split the
# lockstep exists to prevent. It now stages every output before touching any
# target. Make the LAST staged file (the README) unwritable by putting a
# directory where its scratch copy goes: the bundle and constant scratch copies
# are already staged by then, so a regression to write-as-you-go replaces the
# bundle and this case sees it.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_pin_fixture v0.2.0
mkdir "$FIXTURE/pkg/stack/fluxcd/README.md.new"
pin_snapshot
run_pin
assert_rc 1
# The bundle and constant scratch copies must be gone, and no target touched.
assert_pin_tree_unchanged "a failure while staging left the tree changed"
