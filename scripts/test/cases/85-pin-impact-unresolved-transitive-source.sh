#!/bin/bash
# Fixture script bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 85 (go-kure/kure#731): a sourced sibling that cannot be fetched fails closed.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\nsource "$SCRIPT_DIR/missing.sh"\n')"
pi_run
pi_expect 1 "could not fetch scripts/missing.sh"
