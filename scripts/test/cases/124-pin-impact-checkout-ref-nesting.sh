#!/bin/bash
# Row 124 (go-kure/kure#888 review): a `ref:` that YAML places somewhere other
# than the checkout's `with.ref` used to be read as the pin. Under a key the
# line scan does not parse (`a b:`, `a/b:`) it is `with['a b'].ref`; inside a
# multi-line quoted value it is text; at a column other than the `with:`
# mapping's children it is no child of `with:` at all. In each case the checkout
# ran go-kure/.github's default branch while the scan read the new SHA, and
# reported inert. A go-kure/.github checkout step with a line that is no
# readable key, a value that does not end on its line, or a `ref:` off the
# column of `with:`'s first child is refused.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
other="$PI_ROOT/repo/.github/workflows/other.yml"
steps() { printf 'jobs:\n  k:\n    steps:\n%s\n' "$1" >"$other"; }
refused() { pi_run; pi_expect 1 "unrecognized go-kure/.github reference"; }
co="      - uses: actions/checkout@v4"

# Nested under a key the scan does not parse.
steps "$co
        with:
          a b:
            ref: $PI_NEW
          repository: go-kure/.github"
refused
steps "$co
        with:
          a/b:
            ref: $PI_NEW
          repository: go-kure/.github"
refused
# Inside a multi-line double- or single-quoted value.
steps "$co
        with:
          repository: go-kure/.github
          path: \"x
          ref: $PI_NEW
          \""
refused
steps "$co
        with:
          repository: go-kure/.github
          path: 'x
          ref: $PI_NEW
          '"
refused
# ... also when every line it swallows reads as a key.
steps "$co
        with:
          repository: go-kure/.github
          path: \"x
          ref: $PI_NEW
          a: \""
refused
# Off the column of the with: mapping's first child.
steps "$co
        with:
            repository: go-kure/.github
          ref: $PI_NEW"
refused

# A plain checkout, quoted values that end on their line, reads as the pin.
steps "      - name: \"check out go-kure/.github\"
        uses: actions/checkout@v4
        with:
          repository: 'go-kure/.github'
          ref: \"$PI_NEW\"
          path: \"dot github\""
pi_run
pi_expect 0 "pin refresh is inert"
