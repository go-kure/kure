#!/bin/bash
# Row 59: if the constant or README line the script rewrites is gone (reworded,
# reformatted) or ambiguous, the run must fail loudly with the tree untouched --
# instead of matching nothing and reporting success (silently rewriting nothing
# is how a lockstep pin drifts), or failing only after the bundle was already
# replaced (a half-applied re-vendor: new bundle, old constant).
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

# README anchor reworded.
new_pin_fixture v0.2.0
printf 'The bundle is vendored at some version.\n' > "$FIXTURE/pkg/stack/fluxcd/README.md"
pin_snapshot
run_pin
assert_rc 1
assert_err_contains 'expected exactly one line matching'
assert_err_contains 'README.md'
assert_pin_tree_unchanged "missing README anchor still modified the tree"

# Constant declared twice: ambiguous, so refuse rather than rewrite both.
new_pin_fixture v0.2.0
printf 'package fluxcd\n\nconst FluxOperatorVersion = "v0.1.0"\nconst FluxOperatorVersion = "v0.1.0"\n' \
    > "$FIXTURE/pkg/stack/fluxcd/flux_operator_install.go"
pin_snapshot
run_pin
assert_rc 1
assert_err_contains 'expected exactly one line matching'
assert_err_contains 'flux_operator_install.go'
assert_pin_tree_unchanged "ambiguous constant still modified the tree"
