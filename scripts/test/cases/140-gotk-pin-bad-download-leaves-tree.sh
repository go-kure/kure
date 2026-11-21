#!/bin/bash
# Row 140 (go-kure/kure#1365): a download that fails, comes back empty, is not a
# gzip (an HTML error page), or is a gzip tar without the toolkit manifests must
# abort with the tree untouched. Nothing is written until the download
# validates, so a half-applied re-vendor (new constant, old archive) cannot
# reach the bot's commit.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_gotk_fixture v0.2.0
gotk_snapshot

for mode in fail empty html; do
    export STUB_PIN_MODE="$mode"
    run_gotk
    assert_rc 1
    case "$mode" in
        fail)  assert_err_contains 'could not download' ;;
        empty) assert_err_contains 'downloaded empty' ;;
        html)  assert_err_contains 'is not a gzip archive' ;;
    esac
    assert_gotk_tree_unchanged "mode=$mode modified the tree"
done

# A valid gzip tar that holds no controller manifest is not the bundle either.
export STUB_PIN_MODE=ok
gotk_archive "$FIXTURE/new-manifests.tar.gz" other unrelated.yaml
run_gotk
assert_rc 1
assert_err_contains 'has no source-controller.yaml'
assert_gotk_tree_unchanged "an archive without the toolkit manifests modified the tree"
