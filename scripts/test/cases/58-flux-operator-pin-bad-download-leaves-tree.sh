#!/bin/bash
# Row 58: a download that fails (HTTP error), comes back empty, or is not an
# install bundle (an HTML error page) must abort with the tree untouched --
# nothing is written until the download validates, so a half-applied re-vendor
# (new constant, old bundle) cannot reach the bot's commit.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_pin_fixture v0.2.0
before=$(pin_tree_hash)

for mode in fail empty html; do
    export STUB_PIN_MODE="$mode"
    run_pin
    assert_rc 1
    case "$mode" in
        fail)  assert_err_contains 'could not download' ;;
        empty) assert_err_contains 'downloaded empty' ;;
        html)  assert_err_contains "has no 'CustomResourceDefinition' document" ;;
    esac
    if [[ "$(pin_tree_hash)" != "$before" ]]; then
        echo "FAIL: mode=$mode modified the tree" >&2
        exit 1
    fi
done
