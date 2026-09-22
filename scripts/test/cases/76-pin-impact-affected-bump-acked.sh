#!/bin/bash
# Row 76 (go-kure/kure#731): the same affected bump passes, loudly, with PIN_IMPACT_ACK=true.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_compare ahead "scripts/check-a.sh"
export PIN_IMPACT_ACK=true
pi_run
pi_expect 0 "ACKNOWLEDGED via 'pin-impact-ack' label"
