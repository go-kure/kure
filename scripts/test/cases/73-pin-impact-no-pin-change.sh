#!/bin/bash
# Row 73 (go-kure/kure#731): OLD == NEW pin exits 0 before any fetch.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_workflow "$PI_OLD" check-a
pi_run
pi_expect 0 "no pin change — OK"
