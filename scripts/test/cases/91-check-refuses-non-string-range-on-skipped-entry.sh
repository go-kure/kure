#!/bin/bash
# Row 91 (go-kure/kure#766): check validates every entry's supported_range, not
# only the ones validate_gomod compares against a pin. An entry with no
# go_module is skipped by that comparison; a YAML boolean range on it must
# still fail check.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
yq eval -i '.infrastructure.nomod-dep.supported_range = false' "$FIXTURE/versions.yaml"
run_check
assert_rc 1
assert_err_contains 'nomod-dep: supported_range is a !!bool'

# An explicit empty string is a declared, malformed range, not an absent one.
rm -rf "$FIXTURE"  # the EXIT trap only covers the latest fixture
new_fixture
yq eval -i '.infrastructure.nomod-dep.supported_range = ""' "$FIXTURE/versions.yaml"
run_check
assert_rc 1
assert_err_contains 'nomod-dep: supported_range "" is malformed'
