#!/bin/bash
# Row 106 (go-kure/kure#888): a YAML alias in a `uses:` or `repository:`
# value, or a flow mapping naming either key, is refused whatever it names.
# An anchor defined under another key and used as `uses: *p` used to be
# skipped, so the SHA it pins never reached the consistency check.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
other="$PI_ROOT/repo/.github/workflows/other.yml"
refused() { pi_run; pi_expect 1 "unrecognized go-kure/.github reference"; }

cat >"$other" <<EOF
jobs:
  k:
    env:
      P: &p go-kure/.github/.github/actions/check-a@$PI_OTHER
    steps:
      - uses: *p
EOF
refused

cat >"$other" <<EOF
jobs:
  k:
    env:
      R: &r go-kure/.github
    steps:
      - uses: actions/checkout@v4
        with:
          repository: *r
          ref: $PI_OTHER
EOF
refused

cat >"$other" <<EOF
jobs:
  k:
    env:
      P: &p go-kure/.github/.github/actions/check-a@$PI_OTHER
    steps:
      - { uses: *p }
EOF
refused
