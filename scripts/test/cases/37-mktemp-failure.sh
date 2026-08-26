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
# Discriminator: if the `|| return 1` guarding make_temp's result were ever
# dropped, execution would continue with expected="" and reach a downstream
# empty-filename redirect inside generate_docs/generate_go_api (`cat >
# "$DOCS_FILE"` with $DOCS_FILE=""), whose failure is reported by the *shell*
# (not mktemp) with this exact phrasing -- "line N: : No such file or
# directory", NOT an "ambiguous redirect" (that specific bash error needs an
# unquoted multi-/zero-word expansion, which this quoted, merely-empty
# variable never produces). It never appears when the guard stops execution
# right after mktemp itself fails. Without this check, rc==1 and "mktemp
# failed" alone hold in both the guard-intact and guard-removed cases
# (make_temp's own error() message fires unconditionally on mktemp failure,
# regardless of whether the caller then checks the return), so this case
# would pass vacuously against that regression.
[[ "$out" != *"sync-versions.sh: line"* ]] || { echo "FAIL: guard did not stop execution -- downstream redirect failure leaked into stderr: $out" >&2; exit 1; }
