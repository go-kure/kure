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
    # Each case runs in its own subshell: lib.sh's FIXTURE/PATH/env state
    # (set via `export` and a bare global, never a subshell-scoped
    # assignment) must not leak between cases, and a case that dies mid-run
    # (an unguarded assert calling `exit 1`) must not take the runner down
    # with it.
    if out=$(bash "$case_file" 2>&1); then
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
