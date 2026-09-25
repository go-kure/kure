#!/bin/bash
# Fixture workflow bodies below carry a literal ${{ }} expression on purpose.
# shellcheck disable=SC2016
# Row 96 (go-kure/kure#731): a `repository: go-kure/.github` checkout pins its
# SHA whatever the order of the keys in its with: mapping, so a reordered block
# still takes part in the pin-consistency check; a ref belonging to another
# step's checkout does not.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
checkout="$PI_ROOT/repo/.github/workflows/checkout.yml"

# path: between repository: and ref:
printf 'jobs:\n  k:\n    steps:\n      - name: guard\n        uses: actions/checkout@v4\n        with:\n          repository: go-kure/.github\n          path: .tmp/dot-github\n          ref: %s # main\n' "$PI_OTHER" >"$checkout"
pi_run
pi_expect 1 "inconsistent go-kure/.github pins"

# ref: before repository:, quoted
printf "jobs:\n  k:\n    steps:\n      - uses: actions/checkout@v4\n        with:\n          ref: '%s'\n          fetch-depth: 1\n          repository: \"go-kure/.github\" # canonical\n" "$PI_OTHER" >"$checkout"
pi_run
pi_expect 1 "inconsistent go-kure/.github pins"

# A reordered block pinning the same SHA as every uses: is consistent.
printf 'jobs:\n  k:\n    steps:\n      - with:\n          ref: %s\n          repository: go-kure/.github\n        uses: actions/checkout@v4\n' "$PI_NEW" >"$checkout"
pi_run
pi_expect 0 "pin refresh is inert"

# A ref: in a different step's checkout is not attributed to go-kure/.github.
printf 'jobs:\n  k:\n    steps:\n      - uses: actions/checkout@v4\n        with:\n          repository: go-kure/.github\n          ref: ${{ steps.x.outputs.sha }}\n      - uses: actions/checkout@v4\n        with:\n          repository: other/repo\n          ref: %s\n' "$PI_OTHER" >"$checkout"
pi_run
pi_expect 0 "pin refresh is inert"
