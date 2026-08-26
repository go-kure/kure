#!/bin/bash
# Row 28: go.current containing a double-quote can't be emitted as a Go
# string literal by generate_go_api's separate go.current guard (distinct
# from the per-entry loop's guard that row 27 exercises) -- without this
# guard, generate could emit an uncompilable `const GoVersion` literal while
# every per-entry field still passes.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
yq eval -i '.go.current = "1.26.6\"bad"' "$FIXTURE/versions.yaml"
run_generate
assert_rc 1
assert_err_contains 'versions.yaml go.current cannot be emitted as a Go string literal: 1.26.6"bad'
