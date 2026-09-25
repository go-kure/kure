#!/bin/bash
# Row 116 (go-kure/kure#888 review): actions/checkout clones
# https://github.com/<repository>, and GitHub names are case-insensitive, so
# `go-kure/.github.git` and `Go-Kure/.GitHub.GIT` are the same repository.
# `.github.git` used to be neither a checkout nor a mention, so its ref was
# never read and a checkout at another commit read as inert.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
other="$PI_ROOT/repo/.github/workflows/other.yml"
checkout() { printf 'jobs:\n  k:\n    steps:\n      - uses: actions/checkout@v4\n        with:\n          repository: %s\n          ref: %s\n' "$1" "$2" >"$other"; }

for name in go-kure/.github.git Go-Kure/.GitHub.GIT '"go-kure/.github.git"'; do
    checkout "$name" "$PI_OTHER"
    pi_run
    pi_expect 1 "inconsistent go-kure/.github pins"
done

checkout go-kure/.github.git "$PI_NEW"
pi_run
pi_expect 0 "pin refresh is inert"

# Another repository whose name only starts the same stays out of it.
checkout go-kure/.github.gitx "$PI_OTHER"
pi_run
pi_expect 0 "pin refresh is inert"
