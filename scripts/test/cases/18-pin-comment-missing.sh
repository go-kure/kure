#!/bin/bash
# Row 18: delete the "// Current pin: ..." comment line entirely.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
gomod_sub '/^\/\/ Current pin: /d'
run_check
assert_rc 1
assert_err_contains "go.mod is missing the '// Current pin: ' comment above the k8s.io replace block"
