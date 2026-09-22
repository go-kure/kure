#!/bin/bash
# Fixture script bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 81 (go-kure/kure#731): a run: block invoking a second, non-.sh action-relative path is refused.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(printf 'runs:\n  using: composite\n  steps:\n    - shell: bash\n      run: |\n        bash "$GITHUB_ACTION_PATH/../../../scripts/check-a.sh"\n        "$GITHUB_ACTION_PATH/tool"\n')"
pi_run
pi_expect 1 "refusing to guess what the rest invoke"
