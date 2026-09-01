#!/bin/sh
# sync-go-version.sh — propagate the mise.toml [tools].go version into go.mod,
# every .github/workflows/*.yml(.yaml) GO_VERSION/go-version mirror, and
# docs/github-workflows.md. Ported verbatim from the Makefile's sync-go-version
# recipe (same sed patterns, same order) so Renovate's postUpgradeTasks can
# call it directly — postUpgradeTasks runs plain commands, not make targets,
# matching the convention already used for sync-tool-versions.sh,
# sync-govulncheck-docs.sh and sync-eso-pin.sh in this repo.
#
# Run scripts/../Makefile's check-go-version afterwards (or 'make check-go-version').
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

MISE="mise.toml"
VERSIONS="versions.yaml"

if [ ! -f "$MISE" ]; then
	echo "Error: $MISE not found"
	exit 1
fi

GO_VER="$(grep '^go = ' "$MISE" | cut -d'"' -f2)"
if [ -z "$GO_VER" ]; then
	echo "Error: could not extract Go version from $MISE"
	exit 1
fi
echo "Syncing to Go version: $GO_VER"

# versions.yaml's go.current mirrors mise.toml the same way go.mod does --
# scripts/sync-versions.sh's validate_gomod() compares go.mod's directive
# against THIS field, not against mise.toml directly, and generate_go_api()
# / the compatibility.md generator both emit THIS field's value verbatim
# (pkg/versions/versions_gen.go's GoVersion const, docs/compatibility.md's
# "Current: Go ..." line) -- never go.mod's. A version bump that updated
# go.mod but not this field would trade one CI failure (check-go-version)
# for another (sync-versions.sh check's validate_gomod), and would leave
# both generated artifacts one step stale if generate ran first. Written
# with yq -i, the same tool sync-eso-pin.sh already uses for its own
# versions.yaml writes.
if [ ! -f "$VERSIONS" ]; then
	echo "Error: $VERSIONS not found"
	exit 1
fi
yq -i ".go.current = \"$GO_VER\"" "$VERSIONS"

# A glob with no match expands to its own literal pattern string under
# POSIX sh (no nullglob) -- passing that straight to sed would try to open a
# file named literally ".github/workflows/*.yaml" and, under set -eu, abort
# the whole script right there, before the go.mod sed below ever runs. The
# Makefile recipe this was ported from had the same unguarded glob but never
# hit this: make's default recipe shell isn't -e, so a failing sed there just
# warned on stderr and fell through to the next command. Guard explicitly so
# this script's own set -eu can't silently skip go.mod.
for f in .github/workflows/*.yml .github/workflows/*.yaml; do
	[ -e "$f" ] || continue
	sed -i -E "s/^([[:space:]]*)GO_VERSION: '[^']*'/\1GO_VERSION: '$GO_VER'/" "$f"
	sed -i "s/go-version: '[^']*'/go-version: '$GO_VER'/" "$f"
done
sed -i -E "s/^go [^[:space:]]+/go $GO_VER/" go.mod
GOMOD_VER="$(grep -E '^go ' go.mod | head -1 | awk '{print $2}')"
if [ "$GOMOD_VER" != "$GO_VER" ]; then
	echo "Error: go.mod's go directive reads '$GOMOD_VER' after sync, expected $GO_VER"
	exit 1
fi
if [ -f docs/github-workflows.md ]; then
	sed -i "s/Go Version: \`[0-9][^']*\`/Go Version: \`$GO_VER\`/g" docs/github-workflows.md
fi

echo "Go version synced to $GO_VER"
