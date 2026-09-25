#!/bin/bash
# Fixture action.yml bodies below carry a literal $GITHUB_ACTION_PATH on purpose.
# shellcheck disable=SC2016
# Row 99 (go-kure/kure#731): a `$GITHUB_ACTION_PATH/<rel>` script is resolved
# against the action's own directory, `..` hops counted, so the consumed path
# is the file that actually runs -- not whatever scripts/*.sh the reference
# ends in. A path that climbs out of the repository or has an empty segment is
# refused.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

run_yml() { printf 'name: x\nruns:\n  using: composite\n  steps:\n    - shell: bash\n      run: bash "$GITHUB_ACTION_PATH/%s"\n' "$1"; }

pi_fixture check-a group/check

# Three hops from a nested action land in .github/, not the repo root.
pi_raw "$PI_NEW" .github/actions/group/check/action.yml "$(run_yml ../../../scripts/group/check.sh)"
pi_raw "$PI_NEW" .github/scripts/group/check.sh "echo real"
pi_compare ahead ".github/scripts/group/check.sh"
pi_run
pi_expect 1 "1 consumed path(s) changed"
pi_expect 1 "  .github/scripts/group/check.sh"
pi_raw "$PI_NEW" .github/actions/group/check/action.yml "$(pi_action_yml group/check)"

# An action-local script, under a scripts/ directory or directly beside action.yml.
pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(run_yml scripts/check-a.sh)"
pi_raw "$PI_NEW" .github/actions/check-a/scripts/check-a.sh "echo local"
pi_compare ahead ".github/actions/check-a/scripts/check-a.sh"
pi_run
pi_expect 1 "1 consumed path(s) changed"
pi_expect 1 "  .github/actions/check-a/scripts/check-a.sh"

pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(run_yml run.sh)"
pi_raw "$PI_NEW" .github/actions/check-a/run.sh "echo local"
pi_compare ahead ".github/actions/check-a/run.sh"
pi_run
pi_expect 1 "1 consumed path(s) changed"
pi_expect 1 "  .github/actions/check-a/run.sh"

# A '.' segment is dropped; the root-level script is still the one consumed.
pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(run_yml ./../../../scripts/check-a.sh)"
pi_compare ahead "scripts/check-a.sh"
pi_run
pi_expect 1 "  scripts/check-a.sh"

pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(run_yml ../../../../scripts/check-a.sh)"
pi_run
pi_expect 1 "escapes the repository root"

pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(run_yml ..//../../scripts/check-a.sh)"
pi_run
pi_expect 1 "has an empty ('//') segment"
