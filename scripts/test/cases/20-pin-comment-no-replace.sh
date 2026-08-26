#!/bin/bash
# Row 20: delete the k8s.io/api replace directive entirely. The pin comment
# check can't even compute an expected value without it.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
gomod_sub '/k8s.io\/api => k8s.io\/api v0.36.4/d'
run_check
assert_rc 1
assert_err_contains "replace directive not found in go.mod"
