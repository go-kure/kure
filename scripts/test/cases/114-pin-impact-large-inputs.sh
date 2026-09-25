#!/bin/bash
# Fixture action.yml bodies below carry a literal $GITHUB_ACTION_PATH on purpose.
# shellcheck disable=SC2016
# Row 114 (go-kure/kure#888 review): an input larger than the pipe buffer
# (64 KiB) is read whole. Piped into `grep -q` under pipefail, the writer died
# of SIGPIPE once grep had its match and the test read as "no match": a
# nested `uses:` step in a large action.yml, and a consumed path early in a
# large changed-file list, both read as an inert bump.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pad=$(for i in $(seq 1 6000); do printf '    padding line %05d xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n' "$i"; done)
pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(printf 'name: x\nruns:\n  using: composite\n  steps:\n    - uses: other/action@v1\n    - shell: bash\n      run: bash "$GITHUB_ACTION_PATH/../../../scripts/check-a.sh"\ndescription: |\n%s\n' "$pad")"
pi_run
pi_expect 1 "contains a nested 'uses:' step"

# A consumed path first in a changed-file list of more than 64 KiB.
pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(pi_action_yml check-a)"
long=$(printf 'd%.0s' {1..400})
files=(".github/actions/check-a/action.yml")
for i in $(seq 1 250); do files+=("zz/$long/f$i.md"); done
pi_compare ahead "${files[@]}"
pi_run
pi_expect 1 "1 consumed path(s) changed"
pi_expect 1 "  .github/actions/check-a/action.yml"
