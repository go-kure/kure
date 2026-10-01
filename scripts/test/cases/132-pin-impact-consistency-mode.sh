#!/bin/bash
# Row 132 (go-kure/kure#951): --consistency, the merge-queue mode, checks only
# that the working tree's go-kure/.github references pin one SHA -- whatever the
# base holds, and without fetching anything.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a check-b
# No fixture content: any fetch now fails like `curl -f`.
rm -rf "${PI_ROOT:?}/raw" "${PI_ROOT:?}/api"

# A consistent tree passes, although it differs from the base.
pi_run --consistency
pi_expect 0 "every go-kure/.github reference pins ${PI_NEW:0:8} — OK"

# A mixed base is not read: the tree alone decides. The committed `base` gets
# ci.yml at PI_NEW beside other.yml at PI_OTHER; the tree then agrees on PI_NEW.
printf 'jobs:\n  k:\n    steps:\n      - uses: go-kure/.github/.github/actions/check-a@%s\n' "$PI_OTHER" >"$PI_ROOT/repo/.github/workflows/other.yml"
pi_git add -A || { echo "git add failed" >&2; exit 1; }
pi_git commit -qm mixed-base || { echo "git commit failed" >&2; exit 1; }
printf 'jobs:\n  k:\n    steps:\n      - uses: go-kure/.github/.github/actions/check-a@%s\n' "$PI_NEW" >"$PI_ROOT/repo/.github/workflows/other.yml"
pi_run --consistency
pi_expect 0 "every go-kure/.github reference pins ${PI_NEW:0:8} — OK"

# A mixed tree -- the combination two separately-passing PRs can produce -- is refused.
printf 'jobs:\n  k:\n    steps:\n      - uses: go-kure/.github/.github/actions/check-a@%s\n' "$PI_OTHER" >"$PI_ROOT/repo/.github/workflows/other.yml"
pi_run --consistency
pi_expect 1 "inconsistent go-kure/.github pins"

# A reference that yields no pin is refused here as in CI mode.
printf 'jobs:\n  k:\n    steps:\n      - uses: go-kure/.github/.github/actions/check-a@main\n' >"$PI_ROOT/repo/.github/workflows/other.yml"
pi_run --consistency
pi_expect 1 "unrecognized go-kure/.github reference"

# The mode takes no base or SHAs.
pi_run --consistency --base-ref base
pi_expect 2 "--consistency takes no --base-ref or --old/--new"
pi_run --old "$PI_OLD" --new "$PI_NEW" --consistency
pi_expect 2 "--consistency takes no --base-ref or --old/--new"
