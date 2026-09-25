#!/bin/bash
# Fixture workflow bodies below carry a literal ${{ }} expression on purpose.
# shellcheck disable=SC2016
# Row 107 (go-kure/kure#888): a checkout whose `repository:` is given as an
# expression is refused, since it may evaluate to go-kure/.github.
# `${{ github.repository_owner }}/.github` with a hex ref: used to be skipped,
# so that SHA never reached the consistency check.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
other="$PI_ROOT/repo/.github/workflows/other.yml"
checkout() { printf 'jobs:\n  k:\n    steps:\n      - uses: actions/checkout@v4\n        with:\n          repository: %s\n          ref: %s\n' "$1" "$PI_OTHER" >"$other"; }

checkout '${{ github.repository_owner }}/.github'
pi_run
pi_expect 1 "unrecognized go-kure/.github reference"

checkout "\"\${{ format('{0}/.github', github.repository_owner) }}\""
pi_run
pi_expect 1 "unrecognized go-kure/.github reference"

# Another literal repository at another ref is not go-kure/.github's pin.
checkout 'go-kure/go-kure.github.io'
pi_run
pi_expect 0 "pin refresh is inert"
