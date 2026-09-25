#!/bin/bash
# Row 103 (go-kure/kure#888): a quoted `"uses":`, `'repository':` or `"ref":`
# key, or one with a space before its colon, is the same key to YAML, so it is
# read like the plain one: its pin takes part in the consistency check and its
# action is audited. A quoted key with an escape sequence, which could spell
# any key, is refused. Each used to be skipped, so its SHA and its action went
# unseen.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"

pi_fixture check-a
other="$PI_ROOT/repo/.github/workflows/other.yml"
steps() { printf 'jobs:\n  k:\n    steps:\n%s\n' "$1" >"$other"; }

steps "      - \"uses\": go-kure/.github/.github/actions/check-a@$PI_OTHER"
pi_run
pi_expect 1 "inconsistent go-kure/.github pins"

steps "      - 'uses' : go-kure/.github/.github/actions/check-a@$PI_OTHER"
pi_run
pi_expect 1 "inconsistent go-kure/.github pins"

steps "      - uses: actions/checkout@v4
        with:
          \"repository\": go-kure/.github
          'ref': $PI_OTHER"
pi_run
pi_expect 1 "inconsistent go-kure/.github pins"

steps "      - \"u\\x73es\": go-kure/.github/.github/actions/check-a@$PI_OTHER"
pi_run
pi_expect 1 "unrecognized go-kure/.github reference"
steps "      - { \"u\\x73es\": go-kure/.github/.github/actions/check-a@$PI_OTHER }"
pi_run
pi_expect 1 "unrecognized go-kure/.github reference"

# An action named only through a quoted key is audited.
pi_raw "$PI_NEW" .github/actions/check-b/action.yml "$(pi_action_yml check-b)"
pi_raw "$PI_NEW" scripts/check-b.sh "echo b"
pi_compare ahead "scripts/check-b.sh"
steps "      - \"uses\": go-kure/.github/.github/actions/check-b@$PI_NEW"
pi_run
pi_expect 1 "1 consumed path(s) changed"
pi_expect 1 "  scripts/check-b.sh"

# A quoted ref: of the pinned SHA is a pin, not a checkout without one.
pi_compare ahead "docs/unrelated.md"
steps "      - uses: actions/checkout@v4
        with:
          repository: go-kure/.github
          \"ref\": $PI_NEW"
pi_run
pi_expect 0 "pin refresh is inert"
