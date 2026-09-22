#!/bin/bash
# Row 86 (go-kure/kure#731): a compare near the 300-file cap is refused rather than under-read.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
files=()
for ((i = 1; i <= 295; i++)); do files+=("docs/f$i.md"); done
pi_compare ahead "${files[@]}"
pi_run
pi_expect 1 "near GitHub's ~300-file pagination cap"
