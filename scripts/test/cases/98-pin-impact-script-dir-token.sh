#!/bin/bash
# Fixture script bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 98 (go-kure/kure#731): every line that names $SCRIPT_DIR must be the
# whole-line `source` or subprocess form the checker resolves, and every line
# that derives the script's own directory (dirname "$0", ${0%/*}, BASH_SOURCE)
# must be a plain SCRIPT_DIR= definition -- anything else is refused. A call at
# a position the separator scan never looks at (after `!`, `command`, `env`,
# an assignment, inside `{ }`/`( )`/`$( )`, behind a pipe or a variable), or a
# non-.sh sibling, used to read as inert.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_raw "$PI_NEW" scripts/helper.sh "echo helper"
pi_compare ahead "scripts/helper.sh"

# Read on fd 3 so nothing pi_run starts can drain the list.
while IFS= read -r body <&3; do
    pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\nSCRIPT_DIR="$(dirname "$0")"\n%s\n' "$body")"
    pi_run
    pi_expect 1 "refusing to guess whether it needs resolving"
done 3<<'LINES'
if ! bash "$SCRIPT_DIR/helper.sh"; then exit 1; fi
! "$SCRIPT_DIR/helper.sh"
command bash "$SCRIPT_DIR/helper.sh"
env bash "$SCRIPT_DIR/helper.sh"
/usr/bin/env bash "$SCRIPT_DIR/helper.sh"
nice bash "$SCRIPT_DIR/helper.sh"
time bash "$SCRIPT_DIR/helper.sh"
timeout 60 bash "$SCRIPT_DIR/helper.sh"
FOO=1 bash "$SCRIPT_DIR/helper.sh"
{ bash "$SCRIPT_DIR/helper.sh"; }
( "$SCRIPT_DIR/helper.sh" )
out=$(bash "$SCRIPT_DIR/helper.sh")
else bash "$SCRIPT_DIR/helper.sh"
x) "$SCRIPT_DIR/helper.sh" ;;
cat "$SCRIPT_DIR/helper.sh" | bash
helper="$SCRIPT_DIR/helper.sh"
find . -name x -exec bash "$SCRIPT_DIR/helper.sh" {} \;
xargs bash "$SCRIPT_DIR/helper.sh"
"$SCRIPT_DIR/helper"
grep -f "$SCRIPT_DIR/terms.txt" file
bash "${SCRIPT_DIR:-.}/helper.sh"
bash "$SCRIPT_DIR/helper.sh" "$SCRIPT_DIR/other.sh"
if ! source "$SCRIPT_DIR/helper.sh"; then exit 1; fi
{ source "$SCRIPT_DIR/helper.sh"; }
else source "$SCRIPT_DIR/helper.sh"
HERE="$(dirname "$0")"
bash "$(dirname "${BASH_SOURCE[0]}")/helper"
"${0%/*}/helper"
SCRIPT_DIR="$(dirname "$0")"; "$(dirname "$0")/helper"
LINES

# The real definition shape is accepted, and what it sources is followed.
pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\nSCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"\nsource "$SCRIPT_DIR/helper.sh"\n')"
pi_run
pi_expect 1 "1 consumed path(s) changed"
pi_expect 1 "  scripts/helper.sh"
