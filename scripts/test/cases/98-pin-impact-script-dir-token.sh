#!/bin/bash
# Fixture script bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 98 (go-kure/kure#731): every line that names $SCRIPT_DIR must be the
# whole-line `source` or subprocess form the checker resolves, every
# `SCRIPT_DIR=` assignment must be one of the exact definition shapes, and
# every other line that derives the script's own directory (dirname "$0",
# ${0%/*}, BASH_SOURCE) is refused. A call at a position the separator scan
# never looks at (after `!`, `command`, `env`, an assignment, inside
# `{ }`/`( )`/`$( )`, behind a pipe or a variable), a non-.sh sibling, or a
# SCRIPT_DIR pointed somewhere else, used to read as inert.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_raw "$PI_NEW" scripts/helper.sh "echo helper"
pi_compare ahead "scripts/helper.sh"

# Each line: the reason the checker must give, then the script line. Read on
# fd 3 so nothing pi_run starts can drain the list.
while read -r why body <&3; do
    case "$why" in
        source) reason="unrecognized source expression" ;;
        invocation) reason="unrecognized sibling-script invocation" ;;
        definition) reason="unrecognized SCRIPT_DIR definition" ;;
        use) reason='unrecognized use of $SCRIPT_DIR' ;;
        selfdir) reason="unrecognized script-directory expression" ;;
        *) echo "case 98: unknown reason tag '$why'" >&2; exit 1 ;;
    esac
    pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\nSCRIPT_DIR="$(dirname "$0")"\n%s\n' "$body")"
    pi_run
    pi_expect 1 "${reason} in scripts/check-a.sh — refusing to guess whether it needs resolving"
done 3<<'LINES'
use if ! bash "$SCRIPT_DIR/helper.sh"; then exit 1; fi
use ! "$SCRIPT_DIR/helper.sh"
use command bash "$SCRIPT_DIR/helper.sh"
use env bash "$SCRIPT_DIR/helper.sh"
use /usr/bin/env bash "$SCRIPT_DIR/helper.sh"
use nice bash "$SCRIPT_DIR/helper.sh"
use time bash "$SCRIPT_DIR/helper.sh"
use timeout 60 bash "$SCRIPT_DIR/helper.sh"
use FOO=1 bash "$SCRIPT_DIR/helper.sh"
use { bash "$SCRIPT_DIR/helper.sh"; }
use ( "$SCRIPT_DIR/helper.sh" )
use out=$(bash "$SCRIPT_DIR/helper.sh")
use else bash "$SCRIPT_DIR/helper.sh"
use x) "$SCRIPT_DIR/helper.sh" ;;
use cat "$SCRIPT_DIR/helper.sh" | bash
use helper="$SCRIPT_DIR/helper.sh"
use find . -name x -exec bash "$SCRIPT_DIR/helper.sh" {} \;
use xargs bash "$SCRIPT_DIR/helper.sh"
use "$SCRIPT_DIR/helper"
use grep -f "$SCRIPT_DIR/terms.txt" file
invocation bash "${SCRIPT_DIR:-.}/helper.sh"
invocation bash "$SCRIPT_DIR/helper.sh" "$SCRIPT_DIR/other.sh"
use if ! source "$SCRIPT_DIR/helper.sh"; then exit 1; fi
use { source "$SCRIPT_DIR/helper.sh"; }
use else source "$SCRIPT_DIR/helper.sh"
selfdir HERE="$(dirname "$0")"
selfdir bash "$(dirname "${BASH_SOURCE[0]}")/helper"
selfdir "${0%/*}/helper"
definition SCRIPT_DIR="$(dirname "$0")"; "$(dirname "$0")/helper"
definition SCRIPT_DIR="$GITHUB_ACTION_PATH/../../../scripts/lib"
definition SCRIPT_DIR="$(dirname "$(dirname "$0")")"
definition export SCRIPT_DIR=/opt/lib
definition local SCRIPT_DIR="$(dirname "$0")"
definition SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
LINES

# Each exact definition shape is accepted, and what it sources is followed.
while IFS= read -r def <&3; do
    pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\n%s\nsource "$SCRIPT_DIR/helper.sh"\n' "$def")"
    pi_run
    pi_expect 1 "1 consumed path(s) changed"
    pi_expect 1 "  scripts/helper.sh"
done 3<<'DEFS'
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
declare -r SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
readonly SCRIPT_DIR="$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)"
export SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
SCRIPT_DIR=$(dirname "$0")
DEFS
