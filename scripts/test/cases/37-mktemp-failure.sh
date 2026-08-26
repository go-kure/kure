#!/bin/bash
# Row 37: mktemp itself fails (a broken TMPDIR) -- make_temp must name the
# real defect instead of the caller dying on an ambiguous redirect or
# installing a `rm -f ''` RETURN trap over an empty path.
#
# Cannot go through run_check/_run: _run (scripts/test/lib.sh:106-120) itself
# allocates an `errfile` via a bare `mktemp` before ever invoking
# $SYNC_VERSIONS, so a broken TMPDIR set ahead of run_check fails the
# harness's own capture step first, never reaching sync-versions.sh. Invoke
# the binary directly instead, scoping the broken TMPDIR to that one process.
#
# This exercises validate_docs_drift and validate_go_api_drift only (both
# reached via `check`, both TMPDIR-honouring). generate_go_api's own mktemp
# site is co-located with $out via a template argument and ignores TMPDIR
# entirely; sync_gomod_pin_comment's site is generate-only. Neither is
# reachable from this check-only case -- noted here rather than contriving
# an unwritable-directory case for either.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
out=$(TMPDIR="$FIXTURE/no-such-dir" "$SYNC_VERSIONS" check 2>&1 1>/dev/null)
rc=$?
[[ $rc -eq 1 ]] || { echo "FAIL: expected rc 1, got $rc" >&2; exit 1; }
[[ "$out" == *"mktemp failed"* ]] || { echo "FAIL: stderr missing 'mktemp failed': $out" >&2; exit 1; }
