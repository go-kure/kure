#!/bin/bash
# Fixture bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 111 (go-kure/kure#888): the ways left to reach a go-kure/.github file
# without $SCRIPT_DIR are refused. A script naming $GITHUB_ACTION_PATH or the
# runner's `_actions` directory, and an action.yml naming `_actions` or using
# $GITHUB_ACTION_PATH other than as `$GITHUB_ACTION_PATH/<path>`, could start
# a non-.sh sibling (or one on PATH through them) that no scan follows, and
# read as inert.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_compare ahead "tools/x.py"

while IFS= read -r body <&3; do
    pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\n%s\n' "$body")"
    pi_run
    pi_expect 1 "names the go-kure/.github checkout other than through \$SCRIPT_DIR in scripts/check-a.sh"
done 3<<'LINES'
python3 "$GITHUB_ACTION_PATH/../../../tools/x.py"
PATH="${GITHUB_ACTION_PATH}/../../../tools:$PATH" x.py
python3 /home/runner/work/_actions/go-kure/.github/main/tools/x.py
LINES
pi_raw "$PI_NEW" scripts/check-a.sh $'#!/bin/bash\necho ok'

run_yml() { printf 'name: x\nruns:\n  using: composite\n  steps:\n    - shell: bash\n      run: %s\n' "$1"; }
while IFS= read -r run <&3; do
    pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(run_yml "$run")"
    pi_run
    pi_expect 1 "names the go-kure/.github checkout other than as \$GITHUB_ACTION_PATH/<path>"
done 3<<'RUNS'
bash "$GITHUB_ACTION_PATH/../../../scripts/check-a.sh" && python3 "$RUNNER_WORKSPACE/../../_actions/go-kure/.github/main/tools/x.py"
bash "$GITHUB_ACTION_PATH/../../../scripts/check-a.sh" && python3 "${GITHUB_ACTION_PATH%/*}/../../tools/x.py"
GITHUB_ACTION_PATH="$RUNNER_TEMP"; bash "$GITHUB_ACTION_PATH/../../../scripts/check-a.sh"
RUNS
