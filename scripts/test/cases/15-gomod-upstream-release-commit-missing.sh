#!/bin/bash
# Row 15: upstream-dep declares upstream_release but upstream_release_commit
# is deleted entirely -- validate_gomod must error rather than fall through
# to a digest comparison against an empty string.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
yq eval -i 'del(.infrastructure.upstream-dep.upstream_release_commit)' "$FIXTURE/versions.yaml"
run_check
assert_rc 1
assert_err_contains 'upstream_release is set but upstream_release_commit is missing in versions.yaml'
