#!/bin/bash
# Fixture script bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 117 (go-kure/kure#888 review): SCRIPT_DIR repointed or read without a
# `SCRIPT_DIR=` assignment or a `$SCRIPT_DIR` read — `+=`, an array index,
# `read`, `for`, the name passed on as a string — and any name indirection
# (`${!name}`, a nameref, `eval`) are refused. Each let `$SCRIPT_DIR/<name>`
# name a file the walk resolved elsewhere, or not at all, and read as inert.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_raw "$PI_NEW" scripts/y.sh "echo y"
pi_raw "$PI_NEW" scripts/lib/y.sh "echo lib-y"
pi_compare ahead "scripts/lib/y.sh" "scripts/tool.py"
def='SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"'
body() { pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\n%s\n%s\nsource "$SCRIPT_DIR/y.sh"\n' "$def" "$1")"; }

while IFS= read -r line <&3; do
    body "$line"
    pi_run
    pi_expect 1 "unrecognized mention of SCRIPT_DIR in scripts/check-a.sh"
done 3<<'LINES'
SCRIPT_DIR+=/lib
SCRIPT_DIR[0]+=/lib
read -r SCRIPT_DIR <<<"$PWD/lib"
for SCRIPT_DIR in "$PWD/lib"; do :; done
n=SCRIPT_DIR
unset SCRIPT_DIR
LINES

while IFS= read -r line <&3; do
    body "$line"
    pi_run
    pi_expect 1 "name indirection"
done 3<<'LINES'
declare -n d=SCRIPT_DIR
d=x; declare -n d
local -n d=x
typeset -n d=x
declare -rn d=x
n=0; x="${!n}"
python3 "${!n}/tool.py"
eval "d=\$$n"
LINES

# The array-keys form names no variable; the sibling is still followed.
body 'for i in "${!arr[@]}"; do :; done'
pi_compare ahead "scripts/y.sh"
pi_run
pi_expect 1 "1 consumed path(s) changed"
pi_expect 1 "  scripts/y.sh"
