#!/bin/bash
# Row 115 (go-kure/kure#888 review): the pin of a `repository: go-kure/.github`
# checkout is the `ref:` of that step's `with:` mapping, and only that one. A
# `ref:` anywhere else in the step — under `env:`, in a block scalar body, a
# second `ref:` or `REF:`, a second `with:` — used to be read as the pin, the
# last one winning, so a decoy at the new SHA hid a checkout at another
# commit (or at the default branch) and read as inert. A checkout step with
# any of them is refused.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
other="$PI_ROOT/repo/.github/workflows/other.yml"
steps() { printf 'jobs:\n  k:\n    steps:\n%s\n' "$1" >"$other"; }
refused() { pi_run; pi_expect 1 "unrecognized go-kure/.github reference"; }
co="      - uses: actions/checkout@v4"

steps "$co
        with:
          repository: go-kure/.github
          ref: $PI_OTHER
        env:
          ref: $PI_NEW"
refused
steps "$co
        env:
          ref: $PI_NEW
        with:
          repository: go-kure/.github"
refused
steps "$co
        with:
          repository: go-kure/.github
        env:
          ref: $PI_NEW"
refused
steps "$co
        with:
          repository: go-kure/.github
          ref: $PI_OTHER
          sparse-checkout: |
            ref: $PI_NEW"
refused
steps "$co
        with:
          repository: go-kure/.github
          ref: $PI_OTHER
          ref: $PI_NEW"
refused
steps "$co
        with:
          repository: go-kure/.github
          ref: $PI_NEW
          REF: $PI_OTHER"
refused
steps "$co
        with:
          repository: go-kure/.github
          ref: $PI_OTHER
        with:
          ref: $PI_NEW"
refused
steps "$co
        with:
          repository: go-kure/.github
        with:
          ref: $PI_NEW"
refused
steps "$co
        with:
          repository: go-kure/.github
        ref: $PI_NEW"
refused
# No ref: at all was refused before and still is.
steps "$co
        with:
          repository: go-kure/.github"
refused

# The with: mapping's own ref:, other keys around it, reads as the pin.
steps "      - name: checkout
        uses: actions/checkout@v4
        env:
          FOO: bar
        with:
          path: dotgithub
          repository: go-kure/.github
          ref: $PI_NEW
          fetch-depth: 1"
pi_run
pi_expect 0 "pin refresh is inert"
