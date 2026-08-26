#!/bin/bash
# Row 33: net-dep's upstream_release tag is an annotated tag -- the first API
# response is the tag *object*, which must be peeled to the commit it points
# at (a second API call) before comparing against upstream_release_commit. A
# broken peel would compare the tag object's own SHA instead and misreport a
# mismatch even though the release is correctly pinned.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
with_stub_net tag-object
run_check
assert_rc 0
assert_out_contains 'net-dep: upstream_release_commit confirmed against live example/net-dep@v2.9.0'
