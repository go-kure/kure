#!/bin/bash
# Fixture script bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 101 (go-kure/kure#731): a sibling path with an empty (`//`) segment is
# refused, sourced or run: it used to be stored as scripts//helper.sh, which
# never matches the compare's scripts/helper.sh.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_raw "$PI_NEW" scripts/helper.sh "echo helper"
pi_compare ahead "scripts/helper.sh"

pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\nbash "$SCRIPT_DIR//helper.sh"\n')"
pi_run
pi_expect 1 "has an empty ('//') segment"

pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\nsource "$SCRIPT_DIR//helper.sh"\n')"
pi_run
pi_expect 1 "has an empty ('//') segment"
