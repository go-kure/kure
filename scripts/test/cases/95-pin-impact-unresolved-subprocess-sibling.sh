#!/bin/bash
# Fixture script bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 95 (go-kure/kure#731): a sibling-script subprocess the checker cannot
# resolve fails closed under the same rules as a transitive source -- a
# directory expression other than $SCRIPT_DIR, a compound line, a '..'
# segment, a target that cannot be fetched.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_raw "$PI_NEW" scripts/helper.sh "echo helper"

pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\n"$(dirname "$0")/helper.sh"\n')"
pi_run
pi_expect 1 "unrecognized sibling-script invocation"

pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\n"$(cd "$(dirname "$0")" && pwd)/helper.sh"\n')"
pi_run
pi_expect 1 "unrecognized sibling-script invocation"

pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\n"$SCRIPT_DIR"/helper.sh\n')"
pi_run
pi_expect 1 "unrecognized sibling-script invocation"

pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\nif true; then bash "$SCRIPT_DIR/helper.sh"; fi\n')"
pi_run
pi_expect 1 "unrecognized sibling-script invocation"

pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\nbash "$SCRIPT_DIR/helper.sh" || exit 1\n')"
pi_run
pi_expect 1 "unrecognized sibling-script invocation"

pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\nsh "$SCRIPT_DIR/../lib/x.sh"\n')"
pi_run
pi_expect 1 "has a '.'/'..' segment"

pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\nbash "$SCRIPT_DIR/missing.sh"\n')"
pi_run
pi_expect 1 "could not fetch scripts/missing.sh"
