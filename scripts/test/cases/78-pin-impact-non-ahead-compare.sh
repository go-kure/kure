#!/bin/bash
# Row 78 (go-kure/kure#731): a rollback (compare not ahead) is refused rather than read as inert.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_compare behind
pi_run
pi_expect 1 "compare status is 'behind', not 'ahead'"
