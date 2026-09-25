#!/bin/bash
# Fixture action.yml bodies below carry a literal $GITHUB_ACTION_PATH on purpose.
# shellcheck disable=SC2016
# Row 123 (go-kure/kure#888 review): whether the runner reads `USES:`,
# `Using:` or `RUN:` as its lower-case key is not established here, so an
# action.yml key in another letter case is taken as that key: a nested
# `USES:` step is refused, a `Using: node20` makes the action unreadable, and
# a `RUN:` step counts as a second run step. In a workflow, `USES:`,
# `Repository:` and `REF:` were already refused, or are refused by row 115.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_compare ahead "tools/x.py"
run='      run: bash "$GITHUB_ACTION_PATH/../../../scripts/check-a.sh"'

pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(printf 'name: x\nruns:\n  using: composite\n  steps:\n    - shell: bash\n%s\n    - USES: other/action@v1\n' "$run")"
pi_run
pi_expect 1 "contains a nested 'uses:' step"

pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(printf 'name: x\nruns:\n  using: composite\n  Using: node20\n  steps:\n    - shell: bash\n%s\n' "$run")"
pi_run
pi_expect 1 "is not a composite action (runs.using: composite, <unreadable>)"

pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(printf 'name: x\nruns:\n  using: composite\n  steps:\n    - shell: bash\n%s\n    - shell: bash\n      RUN: python3 tools/x.py\n' "$run")"
pi_run
pi_expect 1 "has 2 'run:' steps"

other="$PI_ROOT/repo/.github/workflows/other.yml"
printf 'jobs:\n  k:\n    steps:\n      - USES: go-kure/.github/.github/actions/check-a@%s\n' "$PI_OTHER" >"$other"
pi_run
pi_expect 1 "unrecognized go-kure/.github reference"
