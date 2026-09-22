#!/bin/bash
# Row 89 (go-kure/kure#731): the harness does not depend on the developer's
# git configuration. With a global config that demands commit signing through
# a program that always fails, the fixture still builds and a case still runs.
set -uo pipefail

cfg=$(mktemp) || { echo "mktemp failed" >&2; exit 1; }
printf '[commit]\n\tgpgsign = true\n[gpg]\n\tformat = openpgp\n\tprogram = /bin/false\n' >"$cfg"
export GIT_CONFIG_GLOBAL="$cfg"
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
trap 'rm -rf "$PI_ROOT" "$cfg"' EXIT
pi_run
pi_expect 0 "pin refresh is inert"
