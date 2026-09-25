#!/bin/bash
# Fixture action.yml bodies below carry a literal $GITHUB_ACTION_PATH on purpose.
# shellcheck disable=SC2016
# Row 104 (go-kure/kure#888): an action.yml's `using:` and `uses:` keys are
# read quoted or not, and in a flow mapping. A quoted `"using": node20` beside
# an unquoted `using: composite` used to leave only the composite one seen,
# and a quoted or flow-mapping nested `uses:` went unseen: both read as inert.
# A quoted key with an escape sequence, which could spell either, is refused.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_compare ahead "dist/index.js"
run='      run: bash "$GITHUB_ACTION_PATH/../../../scripts/check-a.sh"'

pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(printf 'name: x\nruns:\n  using: composite\n  "using": node20\n  main: ../../../dist/index.js\n  steps:\n    - shell: bash\n%s\n' "$run")"
pi_run
pi_expect 1 "is not a composite action (runs.using: composite, node20)"

pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(printf 'name: x\nruns:\n  using: composite\n  steps:\n    - shell: bash\n%s\n    - "uses": other/action@v1\n' "$run")"
pi_run
pi_expect 1 "contains a nested 'uses:' step"

pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(printf 'name: x\nruns:\n  using: composite\n  steps:\n    - shell: bash\n%s\n    - { uses: other/action@v1 }\n' "$run")"
pi_run
pi_expect 1 "contains a nested 'uses:' step"

pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(printf 'name: x\nruns:\n  using: composite\n  steps:\n    - shell: bash\n%s\n    - "u\\x73es": other/action@v1\n' "$run")"
pi_run
pi_expect 1 "has a quoted key with an escape sequence"

# A quoted composite declaration alone is still a composite action.
pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(printf "name: x\nruns:\n  'using' : composite\n  steps:\n    - shell: bash\n%s\n" "$run")"
pi_run
pi_expect 0 "pin refresh is inert"
