#!/bin/bash
# Fixture script bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 108 (go-kure/kure#888): a $SCRIPT_DIR that is not the script's own
# directory is refused rather than resolved against it. A file sourced from
# another directory shares its caller's SCRIPT_DIR, and one that redefines it
# changes what the caller's later lines mean; a script run as a subprocess
# that uses SCRIPT_DIR before defining it reads whatever it inherited. Each
# used to resolve its siblings from its own directory, and read as inert.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
def='SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"'
pi_raw "$PI_NEW" scripts/y.sh "echo y"
pi_raw "$PI_NEW" scripts/lib/y.sh "echo lib-y"

# lib/common.sh's $SCRIPT_DIR is check-a.sh's: it runs scripts/y.sh.
pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\n%s\nsource "$SCRIPT_DIR/lib/common.sh"\n' "$def")"
pi_raw "$PI_NEW" scripts/lib/common.sh "$(printf 'source "$SCRIPT_DIR/y.sh"\n')"
pi_compare ahead "scripts/y.sh"
pi_run
pi_expect 1 "scripts/lib/common.sh is sourced from scripts/check-a.sh in another directory"

# lib/common.sh repoints the caller's SCRIPT_DIR: check-a.sh then runs scripts/lib/y.sh.
pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\n%s\nsource "$SCRIPT_DIR/lib/common.sh"\nsource "$SCRIPT_DIR/y.sh"\n' "$def")"
pi_raw "$PI_NEW" scripts/lib/common.sh 'SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"'
pi_compare ahead "scripts/lib/y.sh"
pi_run
pi_expect 1 "scripts/lib/common.sh is sourced from scripts/check-a.sh in another directory"

# lib/run.sh inherits the exported SCRIPT_DIR: it runs scripts/y.sh.
pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\nexport %s\nbash "$SCRIPT_DIR/lib/run.sh"\n' "$def")"
pi_raw "$PI_NEW" scripts/lib/run.sh "$(printf '#!/bin/bash\nbash "$SCRIPT_DIR/y.sh"\n')"
pi_compare ahead "scripts/y.sh"
pi_run
pi_expect 1 "scripts/lib/run.sh uses \$SCRIPT_DIR before defining it"

# A file sourced from another directory that never names SCRIPT_DIR is
# followed, and so is a same-directory one that uses its caller's.
pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\n%s\nsource "$SCRIPT_DIR/lib/common.sh"\nsource "$SCRIPT_DIR/helper.sh"\n' "$def")"
pi_raw "$PI_NEW" scripts/lib/common.sh "echo common"
pi_raw "$PI_NEW" scripts/helper.sh "$(printf 'source "$SCRIPT_DIR/y.sh"\n')"
pi_compare ahead "scripts/lib/common.sh" "scripts/y.sh"
pi_run
pi_expect 1 "2 consumed path(s) changed"
pi_expect 1 "  scripts/lib/common.sh"
pi_expect 1 "  scripts/y.sh"
