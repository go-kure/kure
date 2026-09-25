#!/bin/bash
# Fixture action.yml bodies below carry a literal $GITHUB_ACTION_PATH on purpose.
# shellcheck disable=SC2016
# Row 125 (go-kure/kure#888 review): a whole `$GITHUB_ACTION_PATH/<x>.sh`
# word could still be taken apart — kept in a variable and cut to its
# directory, or handed to dirname — to reach a file no reference names, and
# read as inert. An action that mentions $GITHUB_ACTION_PATH may not use
# dirname/realpath/readlink or a parameter trim, and each such word must be the
# command run, not assigned or passed as an argument.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_compare ahead "tools/x.py"
yml() { printf 'name: x\nruns:\n  using: composite\n  steps:\n    - shell: bash\n      run: %s\n' "$1"; }
main='bash "$GITHUB_ACTION_PATH/../../../scripts/check-a.sh"'

# Derived: dirname/realpath/readlink or a trim, with or without the word in it.
while IFS= read -r run <&3; do
    pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(yml "$run")"
    pi_run
    pi_expect 1 "derives a path (dirname, realpath, readlink or a \${name%...}-style trim)"
done 3<<LINES
$main && python3 "\$(dirname "\$GITHUB_ACTION_PATH/../../../scripts/check-a.sh")/../tools/x.py"
$main; python3 "\${HOME%/*}/x.py"
$main; d="\$(realpath "\$RUNNER_TEMP")"
$main; python3 "\${1#*/}"
$main; python3 "\${f/check-a.sh/x.py}"
$main; python3 "\${f:0:9}x.py"
LINES

# Not the command: assigned, or passed as an argument.
while IFS= read -r run <&3; do
    pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(yml "$run")"
    pi_run
    pi_expect 1 "uses a \$GITHUB_ACTION_PATH/<path> word other than as the command it runs"
done 3<<LINES
x=\$GITHUB_ACTION_PATH/../../../scripts/check-a.sh; bash "\$x"
x="\$GITHUB_ACTION_PATH/../../../scripts/check-a.sh"; bash "\$x"
$main "\$GITHUB_ACTION_PATH/../../../scripts/check-a.sh"
LINES

# The review's reproduction: assigned and trimmed — refused either way.
pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(yml 'x=$GITHUB_ACTION_PATH/../../../scripts/check-a.sh; bash "$x" && python3 "${x%/*}/../tools/x.py"')"
pi_run
pi_expect 1 "refusing to guess what it reaches"

# The command word, after a separator or a keyword, with a default expansion
# beside it, is followed.
pi_compare ahead "scripts/check-a.sh"
pi_raw "$PI_NEW" .github/actions/check-a/action.yml "$(printf 'name: x\nruns:\n  using: composite\n  steps:\n    - shell: bash\n      run: |\n        set -e; bash -e "$GITHUB_ACTION_PATH/../../../scripts/check-a.sh" \\\n          "${INPUT_ROOT:-.}"\n        if ! bash "${GITHUB_ACTION_PATH}/../../../scripts/check-a.sh"; then exit 1; fi\n')"
pi_run
pi_expect 1 "1 consumed path(s) changed"
pi_expect 1 "  scripts/check-a.sh"
