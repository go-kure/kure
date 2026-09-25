#!/bin/bash
# Fixture script bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 122 (go-kure/kure#888 review): `>/dev/null` without `2>&1`,
# `2>/dev/null` and `pwd -P` in the SCRIPT_DIR idiom are the same definition —
# a redirect of cd's output or errors leaves pwd's output alone, and the
# physical path of the directory holds the same files — and what they source
# is followed. They used to abort as unrecognized. `pwd -P` counts as an
# absolute definition, so a later cd does not make it relative.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_raw "$PI_NEW" scripts/helper.sh "echo helper"
pi_compare ahead "scripts/helper.sh"

while IFS= read -r def <&3; do
    pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\n%s\ncd "$RUNNER_TEMP"\nsource "$SCRIPT_DIR/helper.sh"\n' "$def")"
    pi_run
    pi_expect 1 "1 consumed path(s) changed"
    pi_expect 1 "  scripts/helper.sh"
done 3<<'DEFS'
SCRIPT_DIR="$(cd "$(dirname "$0")" >/dev/null && pwd)"
SCRIPT_DIR="$(cd "$(dirname "$0")" 2>/dev/null && pwd)"
SCRIPT_DIR="$(cd "$(dirname "$0")" 1> /dev/null 2>&1 && pwd)"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
SCRIPT_DIR="$(cd -- "$(dirname -- "$0")" > /dev/null && pwd -P)"
DEFS

# Anything else after cd is still no trusted definition.
pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\n%s\n' 'SCRIPT_DIR="$(cd "$(dirname "$0")" 2>"$LOG" && pwd)"')"
pi_run
pi_expect 1 "unrecognized SCRIPT_DIR definition in scripts/check-a.sh"
