#!/bin/bash
# Row 61 (Codex finding on go-kure/kure#829): the go.mod lookup matched the
# module on any line, so a version in an `exclude (...)` block -- or one that
# precedes the real requirement -- was taken as the active pin and the script
# vendored a release Kure does not use. Only `require` directives count.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

# Excluded versions (block and single-line forms) come first; the real pin is
# in the require block after them.
new_pin_fixture v0.2.0
cat > "$FIXTURE/go.mod" <<GOMOD
module example.com/fixture

go 1.26.0

exclude (
	$PIN_MODULE v0.9.9
)

exclude $PIN_MODULE v0.9.8

require (
	$PIN_MODULE v0.2.0
)
GOMOD
run_pin
assert_rc 0
want_url="https://github.com/example/flux-operator/releases/download/v0.2.0/install.yaml"
if ! grep -qxF "$want_url" "$FIXTURE/curl.log"; then
    echo "FAIL: pin was not read from the require block; curl log:" >&2
    cat "$FIXTURE/curl.log" >&2
    exit 1
fi

# The module appears only in exclude/retract/replace forms: not required, so
# refuse instead of adopting an excluded version.
new_pin_fixture v0.2.0
cat > "$FIXTURE/go.mod" <<GOMOD
module example.com/fixture

go 1.26.0

exclude $PIN_MODULE v0.9.9

replace $PIN_MODULE v0.9.7 => ../local

retract v0.9.6

require example.com/other v1.0.0
GOMOD
pin_snapshot
run_pin
assert_rc 1
assert_err_contains 'go.mod does not require'
if [[ -e "$FIXTURE/curl.log" ]]; then
    echo "FAIL: downloaded a release for a module that is not required" >&2
    exit 1
fi
assert_pin_tree_unchanged "refused run modified the tree"

# Single-line `require` form.
new_pin_fixture v0.2.0
printf 'module example.com/fixture\n\ngo 1.26.0\n\nrequire %s v0.2.0\n' "$PIN_MODULE" > "$FIXTURE/go.mod"
run_pin
assert_rc 0
assert_out_contains "${PIN_MODULE} v0.2.0 -- downloading"
