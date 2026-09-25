#!/bin/bash
# Row 105 (go-kure/kure#888): a `uses:` or `repository:` value this line scan
# cannot read whole from its own line is refused, whatever it names: a value
# on the next line, a block scalar, a double-quoted value continued with a
# trailing backslash or carrying an escape sequence. Each used to be skipped,
# so the SHA it pins never reached the consistency check.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
other="$PI_ROOT/repo/.github/workflows/other.yml"
steps() { printf 'jobs:\n  k:\n    steps:\n%s\n' "$1" >"$other"; }
refused() { pi_run; pi_expect 1 "unrecognized go-kure/.github reference"; }

steps "      - uses:
          go-kure/.github/.github/actions/check-a@$PI_OTHER"
refused
steps "      - uses: >-
          go-kure/.github/.github/actions/check-a@$PI_OTHER"
refused
steps "      - uses: \"go-kure/\\
          .github/.github/actions/check-a@$PI_OTHER\""
refused
steps "      - uses: \"go-kure\\x2f.github/.github/actions/check-a@$PI_OTHER\""
refused
steps "      - uses: actions/checkout@v4
        with:
          repository:
            go-kure/.github
          ref: $PI_OTHER"
refused

# A block scalar under another key may name go-kure/.github on a line of its own.
steps "      - name: note
        run: |
          echo go-kure/.github"
pi_run
pi_expect 0 "pin refresh is inert"
