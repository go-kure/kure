#!/bin/bash
# Row 79 (go-kure/kure#731): a nested uses: step in an action.yml is refused, in both YAML spellings.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(printf 'runs:\n  using: composite\n  steps:\n    - name: x\n      uses: actions/checkout@v4\n')"
pi_run
pi_expect 1 "contains a nested 'uses:' step"
pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(printf 'runs:\n  using: composite\n  steps:\n    - uses: actions/checkout@v4\n')"
pi_run
pi_expect 1 "contains a nested 'uses:' step"
