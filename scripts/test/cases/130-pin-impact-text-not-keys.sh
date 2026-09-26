#!/bin/bash
# Fixture action.yml bodies below carry a literal $GITHUB_ACTION_PATH on purpose.
# shellcheck disable=SC2016
# Row 130 (go-kure/kure#888 review): two false aborts. The action.yml key
# checks, which look for `uses`, `run` and `using` anywhere on a line, also
# read the text of a `run: |` body, so `echo "Dry run: ..."` counted as a
# second run step. And a step whose list dash stands alone on its line was
# read as a line with no key, refusing a well-formed checkout step.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_compare ahead "scripts/other.sh"

while IFS= read -r text <&3; do
    pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(printf 'name: x\nruns:\n  using: composite\n  steps:\n    - shell: bash\n      run: |\n        echo "%s"\n        bash "$GITHUB_ACTION_PATH/../../../scripts/check-a.sh"\n' "$text")"
    pi_run
    pi_expect 0 "pin refresh is inert"
done 3<<'TEXTS'
Dry run: skipping
uses: nothing
using: composite
TEXTS

# The dash alone on its line.
printf 'jobs:\n  j:\n    steps:\n      -\n        uses: actions/checkout@v4\n        with:\n          repository: go-kure/.github\n          ref: %s\n' \
    "$PI_NEW" >"$PI_ROOT/repo/.github/workflows/co.yml"
pi_run
pi_expect 0 "pin refresh is inert"
