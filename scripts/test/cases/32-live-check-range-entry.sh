#!/bin/bash
# Row 32: its own net-dep-range entry, never net-dep's -- see
# fixtures/base/versions.yaml's header comment on why this can't live in the
# base fixture (an earlier draft put it there and broke 7 of the other 13
# cases). Injected here instead, only for this one case: a go.mod require
# line and a versions.yaml entry sharing net-dep's upstream_release_commit
# (NETDEP_COMMIT -- safe to reuse, since the stub always returns the same
# fixed value regardless of which entry is being validated) but pinned to
# net-dep-range's OWN mm ("1.5") while declaring the release's mm ("2.9") --
# so the live check succeeds (STUB_NET_MODE=match) and the range check then
# runs against the SUBSTITUTED release version, not the pin's own version.
# If sync-versions.sh:552's substitution were ever deleted, actual_version
# would stay the pin's pseudo-version (mm "1.5"), which IS inside "1.5" --
# rc 0 instead of this case's expected rc 1 -- proving the substitution is
# load-bearing rather than a no-op.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture

sed -i '/^)/i\
	github.com/example/net-dep-range v1.5.0-20260101000000-aaaaaaaaaaaa' "$FIXTURE/go.mod"

yq eval -i '.infrastructure.net-dep-range = {
  "go_module": "github.com/example/net-dep-range",
  "upstream_repo": "example/net-dep-range",
  "upstream_release": "v2.9.0",
  "upstream_release_commit": env(NETDEP_COMMIT),
  "supported_range": "1.5",
  "notes": "Synthetic live-check dependency for scripts/test/ row 32 exclusively."
}' "$FIXTURE/versions.yaml"

with_stub_net match
run_generate
assert_rc 0
run_check
assert_rc 1
# sync-versions.sh:577 prints $actual_version AFTER the :552 substitution --
# the error names the SUBSTITUTED release version (v2.9.0), not the go.mod
# pin (v1.5.0-...) -- the same substitution being proven load-bearing above.
assert_err_contains 'net-dep-range 2.9 (go.mod github.com/example/net-dep-range v2.9.0) is outside supported_range "1.5"'
