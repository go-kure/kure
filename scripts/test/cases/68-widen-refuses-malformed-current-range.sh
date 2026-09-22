#!/bin/bash
# Row 68 (go-kure/kure#766): widen validates the CURRENT supported_range it
# reads from versions.yaml, not only its own new-upper-bound argument, and
# changes nothing when that range is malformed.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
yq eval -i '.infrastructure.range-dep.supported_range = "2.0 - 2.x"' "$FIXTURE/versions.yaml"
before=$(cat "$FIXTURE/versions.yaml")
run_widen range-dep 2.1 --note "v2.1 adds nothing kure imports."
assert_rc 1
assert_err_contains "current supported_range \"2.0 - 2.x\" is malformed"
if [[ "$(cat "$FIXTURE/versions.yaml")" != "$before" ]]; then
    echo "versions.yaml changed on a refused widen" >&2
    exit 1
fi

# A repeated separator in the current range must be refused, not rewritten.
rm -rf "$FIXTURE"  # the EXIT trap only covers the latest fixture
new_fixture
yq eval -i '.infrastructure.range-dep.supported_range = "1.0 - garbage - 2.0"' "$FIXTURE/versions.yaml"
before=$(cat "$FIXTURE/versions.yaml")
run_widen range-dep 2.1 --note "v2.1 adds nothing kure imports."
assert_rc 1
assert_err_contains "current supported_range \"1.0 - garbage - 2.0\" is malformed"
if [[ "$(cat "$FIXTURE/versions.yaml")" != "$before" ]]; then
    echo "versions.yaml changed on a refused widen" >&2
    exit 1
fi
