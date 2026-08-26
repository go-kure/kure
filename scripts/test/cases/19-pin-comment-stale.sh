#!/bin/bash
# Row 19: bump the k8s.io/api replace directive, leave the "// Current pin:"
# comment untouched -- the two must now disagree.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
gomod_sub 's|k8s.io/api => k8s.io/api v0.36.4|k8s.io/api => k8s.io/api v0.37.0|'
run_check
assert_rc 1
assert_err_contains "go.mod pin comment out of date: has '// Current pin: v0.36.4 (Kubernetes 1.36)', expected '// Current pin: v0.37.0 (Kubernetes 1.37)'"
