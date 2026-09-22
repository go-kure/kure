#!/bin/bash
# Fixture script bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 82 (go-kure/kure#731): a script sourced via $SCRIPT_DIR is consumed, so a change to it is affected.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\nSCRIPT_DIR="$(dirname "$0")"\nsource "$SCRIPT_DIR/helper.sh"\n')"
pi_raw "$PI_NEW" scripts/helper.sh "echo helper"
pi_compare ahead "scripts/helper.sh"
pi_run
# The impact verdict itself, not merely a mention of the path: a failure to
# fetch helper.sh would also name it.
pi_expect 1 "1 consumed path(s) changed"
pi_expect 1 "  scripts/helper.sh"
pi_expect 1 "This pin bump touches code kure actually executes"
