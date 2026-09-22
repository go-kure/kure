#!/bin/bash
# Fixture script bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 80 (go-kure/kure#731): an action.yml with more than one run: step is refused.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(pi_action_yml check-a; printf '    - run: bash "$GITHUB_ACTION_PATH/../../../scripts/check-a.sh"\n      shell: bash\n')"
pi_run
pi_expect 1 "has 2 'run:' steps"
