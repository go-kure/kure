#!/bin/bash
# Row 97 (go-kure/kure#731): nested (`actions/group/check`) and dotted
# (`actions/check.v2`) action subpaths are discovered, resolved and pinned like
# single-segment ones; a '.'/'..' segment in one is refused.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a group/check check.v2

pi_compare ahead "scripts/group/check.sh"
pi_run
pi_expect 1 "1 consumed path(s) changed"
pi_expect 1 "  scripts/group/check.sh"

pi_compare ahead "scripts/check.v2.sh"
pi_run
pi_expect 1 "1 consumed path(s) changed"
pi_expect 1 "  scripts/check.v2.sh"

pi_compare ahead ".github/actions/group/check/action.yml"
pi_run
pi_expect 1 "  .github/actions/group/check/action.yml"

# Their SHAs count toward pin consistency.
printf 'jobs:\n  k:\n    steps:\n      - uses: go-kure/.github/.github/actions/group/check@%s\n' "$PI_OTHER" >"$PI_ROOT/repo/.github/workflows/other.yml"
pi_run
pi_expect 1 "inconsistent go-kure/.github pins"
printf 'jobs:\n  k:\n    steps:\n      - uses: go-kure/.github/.github/actions/check.v2@%s\n' "$PI_OTHER" >"$PI_ROOT/repo/.github/workflows/other.yml"
pi_run
pi_expect 1 "inconsistent go-kure/.github pins"

printf 'jobs:\n  k:\n    steps:\n      - uses: go-kure/.github/.github/actions/group/../check-a@%s\n' "$PI_NEW" >"$PI_ROOT/repo/.github/workflows/other.yml"
pi_run
pi_expect 1 "action path has a '.'/'..' segment"
