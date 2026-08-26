#!/bin/bash
# scripts/test/run-tests.sh - runs every scripts/test/cases/*.sh in isolation
# and reports pass/fail. Hermetic and network-independent: see
# scripts/test/lib.sh and fixtures/stub-go/{go,curl,git} for how.
set -uo pipefail

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

pass=0
fail=0
failed_names=()

for case_file in "$TEST_DIR"/cases/*.sh; do
    name="$(basename "$case_file" .sh)"
    # Each case runs as its own `bash` process: lib.sh's FIXTURE/PATH/env
    # state (set via `export` and a bare global, never a subshell-scoped
    # assignment) must not leak between cases, and a case that dies mid-run
    # (an unguarded assert calling `exit 1`) must not take the runner down
    # with it.
    #
    # `env -u` clears the four harness control variables from the CALLER's
    # environment before each case. lib.sh applies its documented defaults
    # with `${VAR:-default}`, which preserves an inherited value -- so an
    # ambient STUB_GO_MODE/STUB_NET_MODE (say, left exported by a debugging
    # session) would silently run every case in a mode it never asked for,
    # and an ambient SYNC_VERSIONS_REPO_ROOT/SYNC_VERSIONS_PROBE_TIMEOUT
    # would reach sync-versions.sh directly. Clearing them here rather than
    # hard-assigning in lib.sh keeps `with_stub_go`/`with_stub_net` and case
    # 9's own SYNC_VERSIONS_PROBE_TIMEOUT override working unchanged, since
    # those all run after new_fixture.
    if out=$(env -u STUB_GO_MODE -u STUB_NET_MODE \
                 -u SYNC_VERSIONS_REPO_ROOT -u SYNC_VERSIONS_PROBE_TIMEOUT \
                 bash "$case_file" 2>&1); then
        pass=$((pass + 1))
        printf 'PASS %s\n' "$name"
    else
        fail=$((fail + 1))
        failed_names+=("$name")
        printf 'FAIL %s\n' "$name"
        printf '%s\n' "$out" | sed 's/^/    /'
    fi
done

echo ""
echo "$pass passed, $fail failed"

if [[ $fail -gt 0 ]]; then
    echo "Failed: ${failed_names[*]}"
    exit 1
fi
exit 0
