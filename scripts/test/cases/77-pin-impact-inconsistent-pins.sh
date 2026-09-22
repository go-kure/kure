#!/bin/bash
# Row 77 (go-kure/kure#731): two workflow files pinning different SHAs is refused.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
printf 'jobs:\n  k:\n    steps:\n      - uses: go-kure/.github/.github/actions/check-a@%s\n' "$PI_OTHER" >"$PI_ROOT/repo/.github/workflows/other.yml"
pi_run
pi_expect 1 "inconsistent go-kure/.github pins"
