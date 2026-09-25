#!/bin/bash
# Fixture workflow bodies below carry a literal ${{ }} expression on purpose.
# shellcheck disable=SC2016
# Row 100 (go-kure/kure#731): every go-kure/.github reference in a `uses:` or
# `repository:` context must yield a 40-hex pin, or the run is refused -- a
# reference the parser skipped used to drop its SHA from the consistency check
# without a word. The repository name matches case-insensitively and an
# uppercase SHA is read as a pin; only a checkout whose ref: is a ${{ }}
# expression may carry no pin.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
other="$PI_ROOT/repo/.github/workflows/other.yml"
steps() { printf 'jobs:\n  k:\n    steps:\n%s\n' "$1" >"$other"; }
upper_other=$(printf '%s' "$PI_OTHER" | tr 'a-f' 'A-F')
upper_new=$(printf '%s' "$PI_NEW" | tr 'a-f' 'A-F')

refused() { pi_run; pi_expect 1 "unrecognized go-kure/.github reference"; }
inconsistent() { pi_run; pi_expect 1 "inconsistent go-kure/.github pins"; }

steps "      - uses: actions/checkout@v4
        with: { repository: go-kure/.github, ref: $PI_OTHER }"
refused
steps "      - uses: go-kure/.github/.github/actions/check-a@main"
refused
steps "      - uses: go-kure/.github/.github/actions/check-a/@$PI_OTHER"
refused
steps "      - uses: go-kure/.github@$PI_OTHER"
refused
steps "      - uses: go-kure/.github/actions/check-a@$PI_OTHER"
refused
steps "      - uses: actions/checkout@v4
        with:
          repository: go-kure/.github
          ref: main"
refused
steps "      - uses: actions/checkout@v4
        with:
          repository: go-kure/.github
          path: .tmp/x"
refused

# Mixed case and uppercase hex are pins, so a different SHA is inconsistent ...
steps "      - uses: actions/checkout@v4
        with:
          repository: Go-Kure/.github
          ref: $PI_OTHER"
inconsistent
steps "      - uses: Go-Kure/.GitHub/.github/actions/check-a@$PI_OTHER"
inconsistent
steps "      - uses: go-kure/.github/.github/actions/check-a@$upper_other"
inconsistent
# ... and the same SHA in uppercase is consistent.
steps "      - uses: go-kure/.github/.github/actions/check-a@$upper_new"
pi_run
pi_expect 0 "pin refresh is inert"

# A ${{ }} ref is no pin and no refusal; neither is a reusable-workflow call,
# which runs at its own ref; another owner's repo is not go-kure's.
steps "      - uses: go-kure/.github/.github/workflows/release-create.yml@main
      - uses: actions/checkout@v4
        with:
          repository: go-kure/.github
          ref: \${{ steps.x.outputs.sha }}
      - uses: xgo-kure/.github/.github/actions/check-a@$PI_OTHER"
pi_run
pi_expect 0 "pin refresh is inert"
