#!/bin/bash
# Row 133 (go-kure/kure#1150): a re-vendor changes the bundle summary line of
# every stored flux-operator bootstrap output, so the script regenerates them
# with the tests' -update flag and keeps the result when that line is all that
# changed. The gotk output, which does not read the bundle, stays as it was. A
# second run, and a run that leaves the bundle alone, must not regenerate.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_pin_fixture v0.2.0
golden="$FIXTURE/pkg/stack/fluxcd/testdata/bootstrap-output"
cp -R "$golden" "$FIXTURE/golden-before"
run_pin
assert_rc 0
assert_out_contains 'bootstrap-output/flux-operator-oci.txt -> bundle digest of v0.2.0'
assert_out_contains 'bootstrap-output/flux-operator-git.txt -> bundle digest of v0.2.0'

if ! grep -qF -- '-run ^(TestBootstrap_OutputUnchanged|TestBootstrapSource(Access|SemVer|Verify)_StoredOutput)$ -count=1 -update' "$FIXTURE/go.log"; then
    echo "FAIL: go was not asked to update the stored outputs; log:" >&2
    cat "$FIXTURE/go.log" >&2
    exit 1
fi

digest="$(sha256sum "$FIXTURE/pkg/stack/fluxcd/flux_operator_install.yaml" | cut -d' ' -f1)"
for name in flux-operator-oci flux-operator-git; do
    if [[ "$(head -n 1 "$golden/$name.txt")" != "=== install bundle: 3 objects, sha256 $digest ===" ]]; then
        echo "FAIL: $name.txt summary line not regenerated:" >&2
        head -n 1 "$golden/$name.txt" >&2
        exit 1
    fi
    if ! cmp -s <(tail -n +2 "$FIXTURE/golden-before/$name.txt") <(tail -n +2 "$golden/$name.txt"); then
        echo "FAIL: $name.txt changed below the summary line" >&2
        exit 1
    fi
done
if ! cmp -s "$FIXTURE/golden-before/gotk-oci.txt" "$golden/gotk-oci.txt"; then
    echo "FAIL: gotk-oci.txt changed" >&2
    exit 1
fi

# In sync: no download, no regeneration.
pin_snapshot
run_pin
assert_rc 0
assert_pin_tree_unchanged "second run changed the tree (not idempotent)"
if [[ "$(wc -l < "$FIXTURE/go.log")" -ne 1 ]]; then
    echo "FAIL: in-sync run still regenerated; log:" >&2
    cat "$FIXTURE/go.log" >&2
    exit 1
fi

# The bundle already matches and only the constant is stale: the stored
# outputs do not depend on the constant, so nothing is regenerated.
new_pin_fixture v0.2.0
cp "$FIXTURE/new-bundle.yaml" "$FIXTURE/pkg/stack/fluxcd/flux_operator_install.yaml"
run_pin
assert_rc 0
assert_out_not_contains 'regenerating'
if [[ -e "$FIXTURE/go.log" ]]; then
    echo "FAIL: regenerated although the bundle did not change; log:" >&2
    cat "$FIXTURE/go.log" >&2
    exit 1
fi
