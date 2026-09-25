#!/bin/bash
# Fixture script bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 109 (go-kure/kure#888): a relative SCRIPT_DIR, `$(dirname "$0")`, is
# refused once any script the walk reaches changes the working directory
# (`cd`, `pushd`, `popd`), since `$SCRIPT_DIR/x.sh` then names a path relative
# to the new one. It used to be resolved as a sibling regardless. The
# absolute `$(cd ... && pwd)` form is unaffected by a cd.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_raw "$PI_NEW" scripts/helper.sh "echo helper"

pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\nSCRIPT_DIR="$(dirname "$0")"\ncd "$RUNNER_TEMP"\nsource "$SCRIPT_DIR/helper.sh"\n')"
pi_run
pi_expect 1 "relative SCRIPT_DIR in scripts/check-a.sh and a working-directory change in scripts/check-a.sh"

# The cd may sit in another script the walk reaches, here a sourced one.
pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\nSCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"\nsource "$SCRIPT_DIR/helper.sh"\nbash "$SCRIPT_DIR/y.sh"\n')"
pi_raw "$PI_NEW" scripts/helper.sh "$(printf 'pushd "$RUNNER_TEMP" >/dev/null\n')"
pi_raw "$PI_NEW" scripts/y.sh "echo y"
pi_run
pi_expect 1 "relative SCRIPT_DIR in scripts/check-a.sh and a working-directory change in scripts/helper.sh"

# An absolute SCRIPT_DIR survives the cd; what it sources is followed.
pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\nSCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"\ncd "$RUNNER_TEMP"\nsource "$SCRIPT_DIR/helper.sh"\n')"
pi_raw "$PI_NEW" scripts/helper.sh "echo helper"
pi_compare ahead "scripts/helper.sh"
pi_run
pi_expect 1 "1 consumed path(s) changed"
pi_expect 1 "  scripts/helper.sh"
