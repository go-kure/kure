#!/bin/bash
# Row 93 (go-kure/kure#731): an action.yml that is not a composite action (a
# JavaScript or Docker action, or one with no runs.using at all) is refused:
# the checker cannot resolve what such an action executes, so a change to its
# entrypoint would otherwise read as an inert bump.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_compare ahead "dist/index.js"

pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(printf 'name: check-a\nruns:\n  using: node20\n  main: ../../../dist/index.js\n')"
pi_run
pi_expect 1 "is not a composite action (runs.using: node20)"

pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(printf "name: check-a\nruns:\n  using: 'node24' # js\n  main: index.js\n")"
pi_run
pi_expect 1 "is not a composite action (runs.using: node24)"

pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(printf 'name: check-a\nruns:\n  using: docker\n  image: Dockerfile\n')"
pi_run
pi_expect 1 "is not a composite action (runs.using: docker)"

pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(printf 'name: check-a\nruns:\n  main: index.js\n')"
pi_run
pi_expect 1 "is not a composite action (runs.using: <none>)"

# A quoted, commented composite declaration is still a composite action.
# shellcheck disable=SC2016 # $GITHUB_ACTION_PATH is written literally into the fixture.
pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(printf 'name: check-a\nruns:\n  using: "composite" # steps below\n  steps:\n    - shell: bash\n      run: bash "$GITHUB_ACTION_PATH/../../../scripts/check-a.sh"\n')"
pi_run
pi_expect 0 "pin refresh is inert"
