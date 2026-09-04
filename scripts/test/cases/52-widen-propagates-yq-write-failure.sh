#!/bin/bash
# Row 52 (Codex finding on go-kure/kure#765's re-review): widen_dependency runs as
# the left side of `||` in main() (`widen_dependency ... || exit 1`), which
# disables this script's own `set -e` for the ENTIRE function -- a failing
# `yq eval -i` would otherwise go unnoticed and the function would still
# report success. Fakes the SECOND write (notes) failing via a local `yq`
# shim prepended to PATH ahead of the real one, so the first write
# (supported_range) genuinely succeeds first -- reproducing the exact
# partial-edit scenario the finding described, not just "some write fails".
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture

real_yq=$(command -v yq)
shim=$(mktemp -d) || {
    echo "52: mktemp -d failed -- refusing to continue" >&2
    exit 1
}
if [[ -z "$shim" || ! -d "$shim" ]]; then
    echo "52: mktemp -d produced an unusable path ('$shim') -- refusing to continue" >&2
    exit 1
fi
# new_fixture already registered an EXIT trap removing $FIXTURE (lib.sh) --
# a second `trap ... EXIT` here would replace it, not compose with it, and
# every run would leave the copied fixture tree behind. Remove both.
trap 'rm -rf "$shim" "$FIXTURE"' EXIT
cat > "$shim/yq" << EOF
#!/bin/bash
if [[ "\$1" == "eval" && "\$2" == "-i" && "\$3" == *".notes ="* ]]; then
    echo "yq-shim: simulated write failure" >&2
    exit 1
fi
exec "$real_yq" "\$@"
EOF
chmod +x "$shim/yq"

export PATH="$shim:$PATH"
run_widen range-dep 2.1 --note "irrelevant"
assert_rc 1
assert_err_contains "widen: failed to write notes for range-dep"

# The first write must have gone through before the second failed -- proves
# this is a genuine partial-edit scenario, not both writes failing together.
current_range=$("$real_yq" '.infrastructure.range-dep.supported_range' "$FIXTURE/versions.yaml")
if [[ "$current_range" != "2.0 - 2.1" ]]; then
    echo "FAIL: expected supported_range already widened to \"2.0 - 2.1\" when the notes write failed, got: $current_range" >&2
    exit 1
fi
