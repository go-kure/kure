#!/bin/bash
# Row 138 (go-kure/kure#1365): sync-gotk-pin.sh moves every artifact of the
# vendored flux2 toolkit bundle to the go.mod version: the archive, the Version
# constant, the bootstrap page mention and the stored bootstrap outputs. A second
# run must change nothing and use no network (what Renovate's postUpgradeTasks
# relies on, so an unrelated bump leaves no diff). The version may also go down:
# the script reads the old version before it stages anything.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

check_moved_to() {
    local version="$1" golden="$FIXTURE/pkg/stack/fluxcd/testdata/bootstrap-output" want_digest f
    want_url="https://github.com/example/flux2/releases/download/$version/manifests.tar.gz"
    if ! grep -qxF "$want_url" "$FIXTURE/curl.log"; then
        echo "FAIL: curl was not asked for $want_url; log:" >&2
        cat "$FIXTURE/curl.log" >&2
        exit 1
    fi
    if ! cmp -s "$FIXTURE/new-manifests.tar.gz" "$FIXTURE/internal/gotk/manifests.tar.gz"; then
        echo "FAIL: the archive was not replaced with the downloaded one" >&2
        exit 1
    fi
    if ! grep -qxF "const Version = \"$version\"" "$FIXTURE/internal/gotk/gotk.go"; then
        echo "FAIL: the Version constant was not rewritten:" >&2
        cat "$FIXTURE/internal/gotk/gotk.go" >&2
        exit 1
    fi
    # The backticks are literal markdown in the bootstrap page, not a command substitution.
    # shellcheck disable=SC2016
    if ! grep -qF "(\`GotkVersion\`, currently **$version**)" "$FIXTURE/pkg/stack/fluxcd/docs/bootstrap-generation.md"; then
        echo "FAIL: the bootstrap page mention was not rewritten:" >&2
        cat "$FIXTURE/pkg/stack/fluxcd/docs/bootstrap-generation.md" >&2
        exit 1
    fi
    # The rest of the page line must survive the rewrite.
    if ! grep -qF 'The gotk bundle is vendored (' "$FIXTURE/pkg/stack/fluxcd/docs/bootstrap-generation.md" \
       || ! grep -qF '), so generation needs no network.' "$FIXTURE/pkg/stack/fluxcd/docs/bootstrap-generation.md"; then
        echo "FAIL: the page rewrite clobbered surrounding text:" >&2
        cat "$FIXTURE/pkg/stack/fluxcd/docs/bootstrap-generation.md" >&2
        exit 1
    fi
    want_digest="$(sha256sum "$FIXTURE/internal/gotk/manifests.tar.gz" | cut -d' ' -f1)"
    for f in "$golden"/gotk-*.txt; do
        if [[ "$(head -n 1 "$f")" != "=== install bundle: 3 objects, sha256 $want_digest ===" ]]; then
            echo "FAIL: ${f#"$FIXTURE"/} does not carry the digest of the new archive; line 1: $(head -n 1 "$f")" >&2
            exit 1
        fi
    done
    for f in "$golden"/flux-operator-*.txt; do
        if ! grep -qxF "    version: $version" "$f"; then
            echo "FAIL: ${f#"$FIXTURE"/} does not name $version:" >&2
            cat "$f" >&2
            exit 1
        fi
    done
}

# Up: go.mod names a newer release than the vendored v0.1.0. Down: an older one.
for gomod in v0.2.0 v0.0.9; do
    new_gotk_fixture "$gomod"
    run_gotk
    assert_rc 0
    check_moved_to "$gomod"

    gotk_snapshot
    log_before="$(cat "$FIXTURE/curl.log")"
    run_gotk
    assert_rc 0
    assert_out_contains "constant and bootstrap page already $gomod -- no change"
    assert_gotk_tree_unchanged "second run changed the tree (not idempotent), go.mod $gomod"
    # In sync, so the second run must not have gone to the network: the log is as it was.
    if [[ "$(cat "$FIXTURE/curl.log")" != "$log_before" ]]; then
        echo "FAIL: second run, already in sync, still called curl (go.mod $gomod)" >&2
        exit 1
    fi
done
