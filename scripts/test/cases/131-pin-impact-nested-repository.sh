#!/bin/bash
# Row 131 (go-kure/kure#901): a step is the go-kure/.github checkout when its
# `with:` mapping holds `repository: go-kure/.github`, at the column of that
# mapping's keys, as its `ref:` must be. A `repository:` line anywhere else in
# the step (under `env:`, deeper under `with:`, or a key of the step itself)
# used to mark the step as that checkout, so an unrelated checkout's `ref:` was
# read as a go-kure/.github pin and the check aborted on "inconsistent pins".
# Such a line is now a mention this scan cannot read, refused by its line.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
other="$PI_ROOT/repo/.github/workflows/other.yml"
steps() { printf 'jobs:\n  k:\n    steps:\n%s\n' "$1" >"$other"; }
refused() {
    pi_run
    pi_expect 1 "unrecognized go-kure/.github reference"
    pi_expect 1 "$1"
    if [[ "$PI_OUT" == *"inconsistent"* ]]; then
        printf 'read an unrelated checkout ref as a pin:\n%s\n' "$PI_OUT" >&2
        exit 1
    fi
}
co="      - uses: actions/checkout@v4"

steps "$co
        env:
          repository: go-kure/.github
        with:
          repository: other/repo
          ref: $PI_OTHER"
refused "other.yml:6: repository: go-kure/.github"
steps "$co
        with:
          repository: other/repo
          ref: $PI_OTHER
          extra:
            repository: go-kure/.github"
refused "other.yml:9: repository: go-kure/.github"
steps "$co
        repository: go-kure/.github
        with:
          repository: other/repo
          ref: $PI_OTHER"
refused "other.yml:5: repository: go-kure/.github"

# The with: mapping's own repository: still marks the checkout.
steps "$co
        with:
          fetch-depth: 1
          repository: go-kure/.github
          ref: $PI_NEW"
pi_run
pi_expect 0 "pin refresh is inert"
