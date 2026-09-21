#!/bin/bash
# Row 60: the no-download fast path applies only when the constant AND the
# README both already name the go.mod version. With the constant current but
# the README stale (a half-applied earlier run, or a hand edit), the script
# must go on to re-vendor and fix the README rather than declare the lockstep
# intact.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_pin_fixture v0.2.0
printf 'package fluxcd\n\nconst FluxOperatorVersion = "v0.2.0"\n' \
    > "$FIXTURE/pkg/stack/fluxcd/flux_operator_install.go"
run_pin
assert_rc 0
assert_out_not_contains 'constant and README already'
if ! grep -qF 'currently **v0.2.0**' "$FIXTURE/pkg/stack/fluxcd/README.md"; then
    echo "FAIL: stale README not corrected:" >&2
    cat "$FIXTURE/pkg/stack/fluxcd/README.md" >&2
    exit 1
fi
if ! grep -qxF 'image: new' "$FIXTURE/pkg/stack/fluxcd/flux_operator_install.yaml"; then
    echo "FAIL: bundle not re-vendored on partial lockstep" >&2
    exit 1
fi
