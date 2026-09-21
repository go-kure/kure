#!/bin/bash
# Row 56: sync-flux-operator-pin.sh moves all three lockstep artifacts -- the
# vendored bundle, the FluxOperatorVersion constant, and the README mention --
# to the go.mod version, downloading that exact release's install.yaml. A
# second run must then change nothing (the idempotence contract Renovate's
# postUpgradeTasks relies on, so a no-op bot re-run leaves no diff).
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_pin_fixture v0.2.0
run_pin
assert_rc 0

want_url="https://github.com/example/flux-operator/releases/download/v0.2.0/install.yaml"
if ! grep -qxF "$want_url" "$FIXTURE/curl.log"; then
    echo "FAIL: curl was not asked for $want_url; log:" >&2
    cat "$FIXTURE/curl.log" >&2
    exit 1
fi

if [[ "$(cat "$FIXTURE/pkg/stack/fluxcd/flux_operator_install.yaml")" != "${PIN_NEW_BUNDLE_BODY%$'\n'}" ]]; then
    echo "FAIL: vendored bundle was not replaced with the downloaded one" >&2
    exit 1
fi
if ! grep -qxF 'const FluxOperatorVersion = "v0.2.0"' "$FIXTURE/pkg/stack/fluxcd/flux_operator_install.go"; then
    echo "FAIL: FluxOperatorVersion constant not rewritten:" >&2
    cat "$FIXTURE/pkg/stack/fluxcd/flux_operator_install.go" >&2
    exit 1
fi
# The backticks are literal markdown in the README, not a command substitution.
# shellcheck disable=SC2016
if ! grep -qF '`FluxOperatorVersion`, currently **v0.2.0**' "$FIXTURE/pkg/stack/fluxcd/README.md"; then
    echo "FAIL: README mention not rewritten:" >&2
    cat "$FIXTURE/pkg/stack/fluxcd/README.md" >&2
    exit 1
fi
# The rest of the README line must survive the rewrite.
if ! grep -qF 'The bundle is vendored (' "$FIXTURE/pkg/stack/fluxcd/README.md" \
   || ! grep -qF '). See the source.' "$FIXTURE/pkg/stack/fluxcd/README.md"; then
    echo "FAIL: README rewrite clobbered surrounding text:" >&2
    cat "$FIXTURE/pkg/stack/fluxcd/README.md" >&2
    exit 1
fi

pin_snapshot
run_pin
assert_rc 0
assert_out_contains 'constant and README already v0.2.0 -- no change'
assert_pin_tree_unchanged "second run changed the tree (not idempotent)"
# In sync, so the second run must not have gone to the network: the log still
# holds only the first run's single request.
if [[ "$(wc -l < "$FIXTURE/curl.log")" -ne 1 ]]; then
    echo "FAIL: in-sync run still downloaded; log:" >&2
    cat "$FIXTURE/curl.log" >&2
    exit 1
fi
