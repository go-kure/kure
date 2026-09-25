#!/bin/bash
# Fixture action.yml bodies below carry a literal $GITHUB_ACTION_PATH on purpose.
# shellcheck disable=SC2016
# Row 119 (go-kure/kure#888 review): every $GITHUB_ACTION_PATH in an
# action.yml must be one whole `$GITHUB_ACTION_PATH/<path>` word. A trailing
# slash with the path after a closing quote, a bare `$GITHUB_ACTION_PATH/`
# kept in a variable or cd'ed into, or a suffix after the closing quote each
# named a file no reference was taken from, and read as inert.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_compare ahead "tools/x.py" "scripts/check-a.sh.py"
yml() { printf 'name: x\nruns:\n  using: composite\n  steps:\n    - shell: bash\n      run: %s\n' "$1"; }
main='bash "$GITHUB_ACTION_PATH/../../../scripts/check-a.sh"'

while IFS= read -r run <&3; do
    pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(yml "$run")"
    pi_run
    pi_expect 1 "names the go-kure/.github checkout other than as \$GITHUB_ACTION_PATH/<path>"
done 3<<LINES
$main && python3 "\$GITHUB_ACTION_PATH/"'../../../tools/x.py'
x="\$GITHUB_ACTION_PATH/"; $main && python3 "\${x}../../../tools/x.py"
$main && python3 "\$GITHUB_ACTION_PATH/"../../../tools/x.py
$main && cd "\$GITHUB_ACTION_PATH/" && python3 ../../../tools/x.py
python3 "\$GITHUB_ACTION_PATH/../../../scripts/check-a.sh".py
bash "\$GITHUB_ACTION_PATH/../../../scripts/\$NAME.sh"
LINES

# Whole words, quoted or not, braced or not, before a redirect or separator.
pi_compare ahead "scripts/check-a.sh"
pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(yml 'bash ${GITHUB_ACTION_PATH}/../../../scripts/check-a.sh 2>&1; bash "$GITHUB_ACTION_PATH/../../../scripts/check-a.sh">/dev/null')"
pi_run
pi_expect 1 "1 consumed path(s) changed"
pi_expect 1 "  scripts/check-a.sh"
