#!/bin/bash
# Row 74 (go-kure/kure#731): a bump whose compare touches nothing kure consumes is inert.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_compare ahead "docs/unrelated.md" ".github/actions/other/action.yml"
pi_run
pi_expect 0 "no consumed path changed; pin refresh is inert"
