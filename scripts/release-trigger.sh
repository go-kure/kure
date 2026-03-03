#!/bin/sh
# Release trigger — dry-run by default, --do-it to execute via CI.
#
# Usage:
#   ./scripts/release-trigger.sh                    # Preview current release
#   ./scripts/release-trigger.sh beta               # Preview promotion to beta
#   ./scripts/release-trigger.sh stable              # Preview stable release
#   ./scripts/release-trigger.sh bump minor          # Preview minor version bump
#   ./scripts/release-trigger.sh --do-it             # Execute release via CI
#   ./scripts/release-trigger.sh beta --do-it        # Execute beta promotion via CI
#   ./scripts/release-trigger.sh bump minor --do-it  # Execute minor bump via CI
#
# The script shows a preview of what will happen, then exits. Pass --do-it
# to trigger the CI pipeline that performs the actual release.
#
# See: https://gitlab.com/autops/wharf/meta/-/blob/main/standards/release-process.md

set -eu

# ── Parse arguments ──────────────────────────────────────────────────────

DO_IT=0
TYPE=""
SCOPE=""
for arg in "$@"; do
    case "$arg" in
        --do-it) DO_IT=1 ;;
        *)
            if [ -z "$TYPE" ]; then
                TYPE="$arg"
            else
                SCOPE="$arg"
            fi
            ;;
    esac
done

# ── Auto-detect type from VERSION ────────────────────────────────────────

if [ -z "$TYPE" ]; then
    VERSION=$(cat VERSION 2>/dev/null) || { echo "ERROR: VERSION file not found"; exit 1; }
    case "$VERSION" in
        *-alpha.*) TYPE=alpha ;;
        *-beta.*)  TYPE=beta ;;
        *-rc.*)    TYPE=rc ;;
        *)
            echo "ERROR: VERSION $VERSION has no prerelease suffix."
            echo "Specify type: mise run release <alpha|beta|rc|stable|bump>"
            exit 1
            ;;
    esac
fi

# ── Show dry-run preview ────────────────────────────────────────────────

if [ "$TYPE" = "bump" ]; then
    echo "=== Version Bump Preview ==="
else
    echo "=== Release Preview ==="
fi
echo ""
DRY_RUN=1 ./scripts/release.sh "$TYPE" $SCOPE
echo ""

# ── If not --do-it, show how to proceed and exit ────────────────────────

if [ "$DO_IT" != "1" ]; then
    echo "---"
    if [ "$TYPE" = "bump" ]; then
        CMD="mise run release bump $SCOPE"
    else
        # Show type only if it differs from auto-detected
        AUTO_TYPE=$(sed -n 's/.*-\(alpha\|beta\|rc\).*/\1/p' VERSION 2>/dev/null || true)
        if [ "$TYPE" = "$AUTO_TYPE" ]; then
            CMD="mise run release"
        else
            CMD="mise run release $TYPE"
        fi
    fi
    echo "To execute, run:"
    echo "  $CMD --do-it"
    exit 0
fi

# ── Trigger CI ───────────────────────────────────────────────────────────

echo "=== Triggering CI ==="
echo ""

if git remote get-url origin 2>/dev/null | grep -q github.com; then
    ARGS="--field type=${TYPE}"
    [ -n "$SCOPE" ] && ARGS="$ARGS --field scope=${SCOPE}"
    echo "Dispatching GitHub workflow: release-create.yml (type=${TYPE})"
    gh workflow run release-create.yml $ARGS
    echo ""
    echo "Watch progress:"
    echo "  gh run list --workflow=release-create.yml"
else
    VARS="RELEASE_TYPE:${TYPE}"
    [ -n "$SCOPE" ] && VARS="${VARS},RELEASE_SCOPE:${SCOPE}"
    echo "Creating GitLab pipeline on main (RELEASE_TYPE=${TYPE})"
    glab ci run --branch main --variables-env "$VARS"
    echo ""
    echo "Watch progress:"
    echo "  glab ci status"
fi
