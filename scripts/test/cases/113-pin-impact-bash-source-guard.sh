#!/bin/bash
# Fixture script bodies below carry literal $VARs on purpose.
# shellcheck disable=SC2016
# Row 113 (go-kure/kure#888): the run-when-executed guard,
# `if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then` on a line of its own, compares
# two strings and derives no directory, so it is accepted. It used to abort
# as a script-directory expression. Anything else on the line is still read.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a

while IFS= read -r guard <&3; do
    pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\nmain() { echo ok; }\n%s\n  main "$@"\nfi\n' "$guard")"
    pi_run
    pi_expect 0 "pin refresh is inert"
done 3<<'GUARDS'
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
if [[ "${BASH_SOURCE[0]}" != "$0" ]]; then
GUARDS

while IFS= read -r line <&3; do
    pi_raw "$PI_NEW" scripts/check-a.sh "$(printf '#!/bin/bash\n%s\n' "$line")"
    pi_run
    pi_expect 1 "in scripts/check-a.sh — refusing to guess whether it needs resolving"
done 3<<'LINES'
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then source "$(dirname "$0")/x.sh"; fi
[[ "${BASH_SOURCE[0]}" == "$0" ]] && python3 "$(dirname "${BASH_SOURCE[0]}")/tool.py"
LINES
