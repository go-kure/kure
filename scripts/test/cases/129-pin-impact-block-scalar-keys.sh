#!/bin/bash
# Row 129 (go-kure/kure#888 review): a line of a block scalar is text, held
# only to the key-shape checks, yet one starting with `uses:` or
# `repository:` still reached the pin readers: a `run: |` body line naming
# an action at a SHA was read as a pin of that action, and one naming the
# repository marked the step a checkout and aborted for want of a ref.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
pi_compare ahead "scripts/other.sh"
hex=cccccccccccccccccccccccccccccccccccccccc

while IFS= read -r body <&3; do
    printf 'jobs:\n  j:\n    steps:\n      - run: |\n          cat <<Y\n          %s\n          Y\n' "$body" \
        >"$PI_ROOT/repo/.github/workflows/gen.yml"
    pi_run
    pi_expect 0 "pin refresh is inert"
done 3<<BODIES
uses: go-kure/.github/.github/actions/check-b@$hex
repository: go-kure/.github
BODIES
