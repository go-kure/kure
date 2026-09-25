#!/bin/bash
# Fixture script bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 94 (go-kure/kure#731): a sibling script run as a subprocess via
# $SCRIPT_DIR -- through bash, sh, exec or directly -- is consumed like a
# sourced one, so a change to it is affected.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_raw "$PI_NEW" scripts/helper.sh "echo helper"
pi_compare ahead "scripts/helper.sh"

for call in \
    'bash "$SCRIPT_DIR/helper.sh"' \
    'sh "${SCRIPT_DIR}/helper.sh" --flag "$@"' \
    '"$SCRIPT_DIR/helper.sh" "$@"' \
    'exec "$SCRIPT_DIR/helper.sh" >"$out"'; do
    pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\nSCRIPT_DIR="$(dirname "$0")"\n%s\n' "$call")"
    pi_run
    # The impact verdict itself, not merely a mention of the path: a failure to
    # fetch helper.sh would also name it.
    pi_expect 1 "1 consumed path(s) changed"
    pi_expect 1 "  scripts/helper.sh"
    pi_expect 1 "This pin bump touches code kure actually executes"
done

# A subprocess call inside a comment is not an invocation.
pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\n# bash "$SCRIPT_DIR/helper.sh"\necho ok\n')"
pi_run
pi_expect 0 "pin refresh is inert"
