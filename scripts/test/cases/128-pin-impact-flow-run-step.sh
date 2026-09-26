#!/bin/bash
# Fixture action.yml bodies below carry a literal $GITHUB_ACTION_PATH on purpose.
# shellcheck disable=SC2016
# Row 128 (go-kure/kure#888 review): the `run:` key was counted, and looked
# for by the no-reference guard, only at the start of a line, while `uses:`
# and `using:` are seen anywhere on it. A step written as a flow mapping
# (`- { shell: bash, run: ... }`) was then not counted: a second such step
# passed the one-`run:`-step refusal, and a sole one that names nothing the
# scan resolves passed as inert.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_compare ahead "tools/x.py"
head='name: x\nruns:\n  using: composite\n  steps:\n'
block='    - shell: bash\n      run: bash "$GITHUB_ACTION_PATH/../../../scripts/check-a.sh"\n'

# A second step, written as a flow mapping.
while IFS= read -r step <&3; do
    pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(printf "${head}${block}%s\n" "$step")"
    pi_run
    pi_expect 1 "'run:' steps"
done 3<<'STEPS'
    - { shell: bash, run: python3 tools/x.py }
    - {shell: bash, "run": python3 tools/x.py}
    - { shell: bash, RUN: python3 tools/x.py }
STEPS

# The only step, a flow mapping that names no $GITHUB_ACTION_PATH script.
pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(printf "${head}%s\n" '    - { shell: bash, run: python3 tools/x.py }')"
pi_run
pi_expect 1 "no recognized"
