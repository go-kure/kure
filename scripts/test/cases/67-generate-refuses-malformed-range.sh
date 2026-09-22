#!/bin/bash
# Row 67 (go-kure/kure#766): generate must refuse a malformed supported_range
# rather than publish its halves as pkg/versions Min/Max metadata, and must
# leave the committed Go API file untouched.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

new_fixture
before=$(cat "$FIXTURE/pkg/versions/versions_gen.go")
before_docs=$(cat "$FIXTURE/docs/compatibility.md")
before_gomod=$(cat "$FIXTURE/go.mod")
yq eval -i '.infrastructure.range-dep.supported_range = "2.0 - 3.x"' "$FIXTURE/versions.yaml"
run_generate
assert_rc 1
assert_err_contains 'range-dep: supported_range "2.0 - 3.x" is malformed'
# Nothing is written: not the Go API, and not the docs or go.mod, which
# generate writes before the Go API.
if [[ "$(cat "$FIXTURE/pkg/versions/versions_gen.go")" != "$before" ]]; then
    echo "generate rewrote versions_gen.go despite refusing the range" >&2
    exit 1
fi
if [[ "$(cat "$FIXTURE/docs/compatibility.md")" != "$before_docs" ]]; then
    echo "generate rewrote docs/compatibility.md despite refusing the range" >&2
    exit 1
fi
if [[ "$(cat "$FIXTURE/go.mod")" != "$before_gomod" ]]; then
    echo "generate rewrote go.mod despite refusing the range" >&2
    exit 1
fi

# A repeated separator must be refused too, not published as 1.0..3.0.
rm -rf "$FIXTURE"  # the EXIT trap only covers the latest fixture
new_fixture
before=$(cat "$FIXTURE/pkg/versions/versions_gen.go")
yq eval -i '.infrastructure.range-dep.supported_range = "1.0 - garbage - 3.0"' "$FIXTURE/versions.yaml"
run_generate
assert_rc 1
assert_err_contains 'range-dep: supported_range "1.0 - garbage - 3.0" is malformed'
if [[ "$(cat "$FIXTURE/pkg/versions/versions_gen.go")" != "$before" ]]; then
    echo "generate rewrote versions_gen.go despite refusing the range" >&2
    exit 1
fi

# A quoted "null" is a string value, not an absent range: it must be refused
# before anything is written too -- here with a stale go.mod pin comment that
# a premature sync_gomod_pin_comment would rewrite.
rm -rf "$FIXTURE"  # the EXIT trap only covers the latest fixture
new_fixture
gomod_sub 's|k8s.io/api => k8s.io/api v0.36.4|k8s.io/api => k8s.io/api v0.37.0|'
before=$(cat "$FIXTURE/pkg/versions/versions_gen.go")
before_docs=$(cat "$FIXTURE/docs/compatibility.md")
before_gomod=$(cat "$FIXTURE/go.mod")
yq eval -i '.infrastructure.range-dep.supported_range = "null"' "$FIXTURE/versions.yaml"
run_generate
assert_rc 1
assert_err_contains 'range-dep: supported_range "null" is malformed'
for pair in "pkg/versions/versions_gen.go:$before" "docs/compatibility.md:$before_docs" "go.mod:$before_gomod"; do
    f="${pair%%:*}"
    if [[ "$(cat "$FIXTURE/$f")" != "${pair#*:}" ]]; then
        echo "generate rewrote $f despite refusing a quoted null range" >&2
        exit 1
    fi
done

# A YAML boolean is not an absent range: yq's `//` would read `false` as
# null, so it must still be refused before anything is written.
rm -rf "$FIXTURE"  # the EXIT trap only covers the latest fixture
new_fixture
before_docs=$(cat "$FIXTURE/docs/compatibility.md")
yq eval -i '.infrastructure.range-dep.supported_range = false' "$FIXTURE/versions.yaml"
run_generate
assert_rc 1
assert_err_contains 'range-dep: supported_range is a !!bool'
if [[ "$(cat "$FIXTURE/docs/compatibility.md")" != "$before_docs" ]]; then
    echo "generate rewrote docs/compatibility.md despite refusing a boolean range" >&2
    exit 1
fi
