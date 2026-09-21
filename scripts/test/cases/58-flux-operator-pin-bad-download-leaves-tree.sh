#!/bin/bash
# Row 58: a download that fails (HTTP error), comes back empty, or is not an
# install bundle (an HTML error page) must abort with the tree untouched --
# nothing is written until the download validates, so a half-applied re-vendor
# (new constant, old bundle) cannot reach the bot's commit.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_pin_fixture v0.2.0
pin_snapshot

for mode in fail empty html; do
    export STUB_PIN_MODE="$mode"
    run_pin
    assert_rc 1
    case "$mode" in
        fail)  assert_err_contains 'could not download' ;;
        empty) assert_err_contains 'downloaded empty' ;;
        html)  assert_err_contains "has no 'CustomResourceDefinition' document" ;;
    esac
    assert_pin_tree_unchanged "mode=$mode modified the tree"
done
