#!/bin/bash
# Fixture script bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 84 (go-kure/kure#731): a source inside a compound command is detected and refused, not skipped.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\nif true; then source "$SCRIPT_DIR/helper.sh"; fi\n')"
pi_run
pi_expect 1 "unrecognized source expression"
