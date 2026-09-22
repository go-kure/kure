#!/bin/bash
# Fixture script bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 88 (go-kure/kure#731): an impact found through a transitively sourced
# sibling is acknowledged by PIN_IMPACT_ACK like any direct one.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\nSCRIPT_DIR="$(dirname "$0")"\nsource "$SCRIPT_DIR/helper.sh"\n')"
pi_raw "$PI_NEW" scripts/helper.sh "echo helper"
pi_compare ahead "scripts/helper.sh"
export PIN_IMPACT_ACK=true
pi_run
pi_expect 0 "ACKNOWLEDGED via 'pin-impact-ack' label"
pi_expect 0 "  scripts/helper.sh"
