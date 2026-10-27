#!/bin/bash
# Row 134 (go-kure/kure#1150): when regenerating the stored bootstrap outputs
# changes anything beyond the bundle summary line of a flux-operator file -- a
# line further down, the object count, a gotk file, a new file -- the rendered
# bootstrap changed and a person has to read it. The script must put every
# stored output back as it was and fail, naming the file. The re-vendored
# bundle stays: the bot's PR then fails `test` on the stale outputs.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

for mode in body count gotk added; do
    export STUB_PIN_GO_MODE="$mode"
    new_pin_fixture v0.2.0
    golden="$FIXTURE/pkg/stack/fluxcd/testdata/bootstrap-output"
    cp -R "$golden" "$FIXTURE/golden-before"
    run_pin
    assert_rc 1
    assert_err_contains 'beyond the bundle digest line'
    case "$mode" in
        body | count) want=flux-operator-oci.txt ;;
        gotk) want=gotk-oci.txt ;;
        added) want=flux-operator-extra.txt ;;
    esac
    assert_err_contains "  $want"
    assert_out_not_contains 'sync-flux-operator-pin: done'
    if ! diff -r "$FIXTURE/golden-before" "$golden" >/dev/null 2>&1; then
        echo "FAIL ($mode): stored outputs not restored:" >&2
        diff -r "$FIXTURE/golden-before" "$golden" >&2
        exit 1
    fi
    if [[ "$(cat "$FIXTURE/pkg/stack/fluxcd/flux_operator_install.yaml")" != "${PIN_NEW_BUNDLE_BODY%$'\n'}" ]]; then
        echo "FAIL ($mode): the re-vendored bundle was not kept" >&2
        exit 1
    fi
done
