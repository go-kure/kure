#!/bin/bash
# Fixture script bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 112 (go-kure/kure#888): the common `cd -- "$(dirname -- ...)"` form of
# the SCRIPT_DIR idiom, and a space after `&>` or `>` in its redirect, are
# trusted definitions like the forms without them, and what they source is
# followed. They used to abort as unrecognized. A directory other than the
# script's own is still refused, `--` or not.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_raw "$PI_NEW" scripts/helper.sh "echo helper"
pi_compare ahead "scripts/helper.sh"

while IFS= read -r def <&3; do
    pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\n%s\nsource "$SCRIPT_DIR/helper.sh"\n' "$def")"
    pi_run
    pi_expect 1 "1 consumed path(s) changed"
    pi_expect 1 "  scripts/helper.sh"
done 3<<'DEFS'
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
SCRIPT_DIR="$(cd -- "$(dirname -- "$0")" && pwd)"
SCRIPT_DIR="$(cd "$(dirname "$0")" &> /dev/null && pwd)"
readonly SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" > /dev/null 2>&1 && pwd)"
SCRIPT_DIR=$(dirname -- "$0")
DEFS

while IFS= read -r def <&3; do
    pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\n%s\n' "$def")"
    pi_run
    pi_expect 1 "in scripts/check-a.sh — refusing to guess whether it needs resolving"
done 3<<'DEFS'
SCRIPT_DIR="$(cd -- "$(dirname -- "$0")/.." && pwd)"
ROOT=$(cd "$(dirname "$0")/.." && pwd)
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." &> /dev/null && pwd)"
DEFS
