#!/bin/bash
# Row 87 (go-kure/kure#731): the English word source in a comment is not read as a source line.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\n# the source of truth is elsewhere\necho "extra_mounts source not found"\n')"
pi_run
pi_expect 0 "pin refresh is inert"
