#!/bin/bash
# Row 34: the GitHub API is inconclusive (rate-limit-style, not a definitive
# 404), but `git ls-remote` reaches the server and resolves the tag -- the
# successful ls-remote fallback path. A broken match here would treat a
# reachable-but-empty ls-remote as definitive absence and misreport "does not
# exist" instead of confirming the release.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
with_stub_net ls-remote-match
run_check
assert_rc 0
assert_out_contains 'net-dep: upstream_release_commit confirmed against live example/net-dep@v2.9.0'
