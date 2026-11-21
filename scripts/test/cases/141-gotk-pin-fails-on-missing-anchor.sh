#!/bin/bash
# Row 141 (go-kure/kure#1365): each line the script rewrites must exist exactly
# once, checked before any download. A count other than one means the file's
# shape changed and a rewrite would be a guess; failing after the archive was
# already replaced is how a lockstep pin drifts.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

# The constant line gone, then doubled, then the page mention gone, then doubled.
# The page mention twice on one line counts as two: a line count would pass the
# preflight and the rewrite would then update only the first.
for breakage in no-const two-const no-page two-page same-line-page; do
    new_gotk_fixture v0.2.0
    case "$breakage" in
        no-const) sed -i '/^const Version/d' "$FIXTURE/internal/gotk/gotk.go" ;;
        two-const) printf 'const Version = "v0.1.0"\n' >> "$FIXTURE/internal/gotk/gotk.go" ;;
        no-page) printf 'nothing here\n' > "$FIXTURE/pkg/stack/fluxcd/docs/bootstrap-generation.md" ;;
        two-page)
            # The backticks are literal markdown, not a command substitution.
            # shellcheck disable=SC2016
            printf 'One (`GotkVersion`, currently **v0.1.0**).\nTwo (`GotkVersion`, currently **v0.1.0**).\n' \
                > "$FIXTURE/pkg/stack/fluxcd/docs/bootstrap-generation.md"
            ;;
        same-line-page)
            # shellcheck disable=SC2016
            printf 'Both (`GotkVersion`, currently **v0.1.0**) and (`GotkVersion`, currently **v0.1.0**).\n' \
                > "$FIXTURE/pkg/stack/fluxcd/docs/bootstrap-generation.md"
            ;;
    esac
    gotk_snapshot
    run_gotk
    assert_rc 1
    assert_err_contains 'expected exactly one line matching'
    if [[ -e "$FIXTURE/curl.log" ]]; then
        echo "FAIL ($breakage): script downloaded something despite the missing anchor" >&2
        exit 1
    fi
    assert_gotk_tree_unchanged "$breakage: refused run modified the tree"
done
