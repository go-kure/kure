#!/bin/bash
# Fixture action.yml bodies below carry literal $GITHUB_ACTION_PATH and ${{ }} on purpose.
# shellcheck disable=SC2016
# Row 102 (go-kure/kure#731): an action.yml that names the action directory
# through a `github.action_path` expression is refused. Only $GITHUB_ACTION_PATH
# references are resolved, so a second program started as
# `${{ github.action_path }}/tool` used to contribute nothing to the consumed set.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

run_yml() { printf 'name: x\nruns:\n  using: composite\n  steps:\n    - shell: bash\n      run: %s\n' "$1"; }

pi_fixture check-a
pi_compare ahead "tool"

while IFS= read -r run <&3; do
    pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(run_yml "$run")"
    pi_run
    pi_expect 1 "uses a github.action_path expression"
done 3<<'RUNS'
bash "$GITHUB_ACTION_PATH/../../../scripts/check-a.sh" && "${{ github.action_path }}/tool"
bash "${{ github.action_path }}/../../../scripts/check-a.sh"
bash "$GITHUB_ACTION_PATH/../../../scripts/check-a.sh" && "${{ GITHUB.ACTION_PATH }}/tool"
RUNS
