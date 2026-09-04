#!/bin/bash
# Row 42: widen refuses a dependency key that doesn't exist in
# versions.yaml's .infrastructure map.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
run_widen no-such-dep 9.9 --note "irrelevant"
assert_rc 1
assert_err_contains "widen: no such dependency 'no-such-dep' in versions.yaml's .infrastructure"
