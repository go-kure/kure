#!/bin/bash
# Row 75 (go-kure/kure#731): a bump that changes a consumed script fails without the ack.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_compare ahead "scripts/check-a.sh"
pi_run
pi_expect 1 "FAIL — add the 'pin-impact-ack' label"
