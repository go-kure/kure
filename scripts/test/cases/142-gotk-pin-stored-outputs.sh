#!/bin/bash
# Row 142 (go-kure/kure#1365): after a re-vendor the stored bootstrap outputs
# are regenerated, and the result is kept only when every change is explained: a
# gotk-*.txt differs in its first line (the bundle summary) with the same object
# count, and a flux-operator-*.txt differs in its `    version:` line, from the
# old toolkit version to the new. Anything else (a line further down, a changed
# count, a new file) means the rendered bootstrap changed and a person has to
# read it: every stored output goes back as it was, the script fails and names
# the file, and the new archive stays so that `test` fails on the stale outputs.
# A `go test` that fails, matches no test, or skips one is refused the same way.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

golden_rel=pkg/stack/fluxcd/testdata/bootstrap-output
for mode in body count opbody added fail missing skipped subskip; do
    export STUB_GOTK_GO_MODE="$mode"
    new_gotk_fixture v0.2.0
    golden="$FIXTURE/$golden_rel"
    cp -R "$golden" "$FIXTURE/golden-before"
    run_gotk
    assert_rc 1
    assert_out_not_contains 'sync-gotk-pin: done'
    case "$mode" in
        body | count | opbody | added)
            assert_err_contains "beyond the digest line and the version line"
            case "$mode" in
                body | count) want=gotk-oci.txt ;;
                opbody) want=flux-operator-oci.txt ;;
                added) want=gotk-extra.txt ;;
            esac
            assert_err_contains "  $want"
            ;;
        fail) assert_err_contains 'regenerating' ;;
        missing | skipped) assert_err_contains 'did not run and pass' ;;
        # The parent reports PASS while a golden subtest skipped, so its stored
        # output is stale and would be accepted as unchanged.
        subskip) assert_err_contains 'subtests were skipped' ;;
    esac
    if ! diff -r "$FIXTURE/golden-before" "$golden" >/dev/null 2>&1; then
        echo "FAIL ($mode): stored outputs not restored:" >&2
        diff -r "$FIXTURE/golden-before" "$golden" >&2
        exit 1
    fi
    if ! cmp -s "$FIXTURE/new-manifests.tar.gz" "$FIXTURE/internal/gotk/manifests.tar.gz"; then
        echo "FAIL ($mode): the re-vendored archive was not kept" >&2
        exit 1
    fi
done

# The four test names run by this script and by sync-flux-operator-pin.sh are
# the same list: a rename in one is caught here, not at the next bump.
names_of() {
    sed -n '/^GOLDEN_TEST_NAMES=(/,/^)/p' "$1" | sed -E '1d;$d;s/^[[:space:]]+//'
}
gotk_names="$(names_of "$SYNC_GOTK_PIN")"
operator_names="$(names_of "$SYNC_FLUX_OPERATOR_PIN")"
if [[ -z "$operator_names" || "$gotk_names" != "$operator_names" ]]; then
    echo "FAIL: the golden test names differ between the two scripts" >&2
    printf 'gotk:\n%s\noperator:\n%s\n' "$gotk_names" "$operator_names" >&2
    exit 1
fi
