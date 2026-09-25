#!/bin/bash
# Fixture action.yml bodies below carry a literal $GITHUB_ACTION_PATH on purpose.
# shellcheck disable=SC2016
# Row 120 (go-kure/kure#888 review): a `? ` complex key could spell `uses`,
# `repository` or `using` without this line scan seeing which, so it is
# refused in a workflow and in an action.yml.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
other="$PI_ROOT/repo/.github/workflows/other.yml"
printf 'jobs:\n  k:\n    steps:\n      - ? uses\n        : go-kure/.github/.github/actions/check-a@%s\n' "$PI_OTHER" >"$other"
pi_run
pi_expect 1 "unrecognized go-kure/.github reference"
rm "$other"

pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(printf 'name: x\nruns:\n  using: composite\n  steps:\n    - shell: bash\n      run: bash "$GITHUB_ACTION_PATH/../../../scripts/check-a.sh"\n    - ? uses\n      : other/action@v1\n')"
pi_run
pi_expect 1 "has a quoted key with an escape sequence, or a '?' complex key"
