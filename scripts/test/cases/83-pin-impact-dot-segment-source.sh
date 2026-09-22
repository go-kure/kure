#!/bin/bash
# Fixture script bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 83 (go-kure/kure#731): a sourced path with a ../ segment is refused rather than normalised by guess.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\nsource "$SCRIPT_DIR/../lib/x.sh"\n')"
pi_run
pi_expect 1 "has a '.'/'..' segment"
