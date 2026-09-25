#!/bin/bash
# Fixture script bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 121 (go-kure/kure#888 review): the working-directory scan reads a walked
# script to its end. It used to stop at the first `cd`, and on a script of
# more than 64 KiB the writer then died of SIGPIPE and pipefail ended the whole
# run with rc 141 and no message.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pad=$(for i in $(seq 1 3000); do printf 'echo "padding line %05d xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"\n' "$i"; done)
pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\ncd "$RUNNER_TEMP"\n%s\n' "$pad")"
pi_run
pi_expect 0 "pin refresh is inert"

# With a relative SCRIPT_DIR the same cd is still reported, by name.
pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\nSCRIPT_DIR="$(dirname "$0")"\ncd "$RUNNER_TEMP"\n%s\n' "$pad")"
pi_run
pi_expect 1 "relative SCRIPT_DIR in scripts/check-a.sh and a working-directory change in scripts/check-a.sh"
