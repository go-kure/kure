#!/bin/bash
# Row 31: net-dep's live check is entirely unreachable (both curl and git
# ls-remote fail) -- the offline invariant: this must warn, never hard-fail a
# `check` run just because the network (or GitHub) is unavailable. This is
# also the default STUB_NET_MODE for every other case, so with_stub_net is
# called here only for explicitness.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
with_stub_net unreachable
run_check
assert_rc 0
assert_out_contains 'could not verify upstream_release_commit against example/net-dep@v2.9.0 live (no network or rate-limited) — offline digest check above still holds'
