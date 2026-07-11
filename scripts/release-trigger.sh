#!/bin/sh
# Release trigger — dry-run by default, --do-it to execute via CI.
#
# Usage:
#   ./scripts/release-trigger.sh                         # Preview release (auto-infer from VERSION)
#   ./scripts/release-trigger.sh promote <rc|beta|stable> # Preview type promotion
#   ./scripts/release-trigger.sh bump <minor|major|prerelease> # Preview version bump
#   ./scripts/release-trigger.sh --do-it                 # Execute release via CI
#   ./scripts/release-trigger.sh promote rc --do-it      # Execute promotion via CI
#   ./scripts/release-trigger.sh bump minor --do-it      # Execute bump via CI
#
# The script shows a dry-run preview, then exits. Pass --do-it to trigger CI.
#
# See: https://gitlab.com/autops/wharf/meta/-/blob/main/standards/release-process.md

set -eu

# ── Parse arguments ──────────────────────────────────────────────────────

DO_IT=0
SUBCOMMAND=""
ARG=""

for _arg in "$@"; do
    case "$_arg" in
        --do-it) DO_IT=1 ;;
        --) ;;  # tolerate a stray separator (e.g. `mise run release -- --do-it`)
        promote|bump)
            if [ -n "$SUBCOMMAND" ]; then
                echo "ERROR: unexpected argument: $_arg" >&2; exit 1
            fi
            SUBCOMMAND="$_arg"
            ;;
        *)
            if [ -n "$SUBCOMMAND" ] && [ -z "$ARG" ]; then
                ARG="$_arg"
            else
                echo "ERROR: unexpected argument: '$_arg'" >&2
                echo "       Usage: release-trigger.sh [promote <type>|bump <scope>] [--do-it]" >&2
                exit 1
            fi
            ;;
    esac
done

# ── Validate subcommand arguments ────────────────────────────────────────

if [ "$SUBCOMMAND" = "promote" ]; then
    case "$ARG" in
        beta|rc|stable) ;;
        "") echo "ERROR: 'promote' requires a target: beta, rc, or stable" >&2; exit 1 ;;
        *)  echo "ERROR: invalid promote target '$ARG' (use: beta, rc, stable)" >&2; exit 1 ;;
    esac
elif [ "$SUBCOMMAND" = "bump" ]; then
    case "$ARG" in
        minor|major|prerelease) ;;
        "") echo "ERROR: 'bump' requires a scope: minor, major, or prerelease" >&2; exit 1 ;;
        *)  echo "ERROR: invalid bump scope '$ARG' (use: minor, major, prerelease)" >&2; exit 1 ;;
    esac
fi

# ── Show dry-run preview ────────────────────────────────────────────────

if [ "$DO_IT" = "1" ]; then
    echo "NOTE: showing a dry-run preview first (no changes are made here);"
    echo "      the REAL release is then dispatched to CI below."
    echo ""
fi

if [ "$SUBCOMMAND" = "bump" ]; then
    echo "=== Version Bump Preview ==="
    echo ""
    DRY_RUN=1 RELEASE_TYPE=bump RELEASE_SCOPE="$ARG" ./scripts/release.sh
else
    echo "=== Release Preview ==="
    echo ""
    if [ "$SUBCOMMAND" = "promote" ]; then
        DRY_RUN=1 RELEASE_TYPE="$ARG" ./scripts/release.sh
    else
        DRY_RUN=1 ./scripts/release.sh
    fi
fi
echo ""

# ── If not --do-it, show how to proceed and exit ─────────────────────────

if [ "$DO_IT" != "1" ]; then
    echo "---"
    if [ "$SUBCOMMAND" = "bump" ]; then
        echo "To execute, run:"
        echo "  mise run release bump $ARG --do-it"
    elif [ "$SUBCOMMAND" = "promote" ]; then
        echo "To execute, run:"
        echo "  mise run release promote $ARG --do-it"
    else
        echo "To execute, run:"
        echo "  mise run release --do-it"
    fi
    exit 0
fi

# ── Trigger CI ───────────────────────────────────────────────────────────

echo "Preview above made NO changes. Dispatching the REAL release to CI now…"
echo ""
echo "=== Triggering CI ==="
echo ""

REMOTE=$(git remote get-url origin 2>/dev/null || true)
if echo "$REMOTE" | grep -q github.com; then
    if [ "$SUBCOMMAND" = "bump" ]; then
        echo "Dispatching GitHub workflow: release-bump.yml (scope=${ARG})"
        gh workflow run release-bump.yml --field "scope=${ARG}"
        echo ""
        echo "Watch progress:"
        echo "  gh run list --workflow=release-bump.yml"
    elif [ "$SUBCOMMAND" = "promote" ]; then
        echo "Dispatching GitHub workflow: release-promote.yml (to=${ARG})"
        gh workflow run release-promote.yml --field "to=${ARG}"
        echo ""
        echo "Watch progress:"
        echo "  gh run list --workflow=release-promote.yml"
    else
        echo "Dispatching GitHub workflow: release-create.yml"
        gh workflow run release-create.yml
        echo ""
        echo "Watch progress:"
        echo "  gh run list --workflow=release-create.yml"
    fi
elif echo "$REMOTE" | grep -q gitlab; then
    if [ "$SUBCOMMAND" = "bump" ]; then
        echo "Creating GitLab pipeline on main (BUMP_SCOPE=${ARG})"
        glab ci run --branch main --variables-env "BUMP_SCOPE:${ARG}"
    elif [ "$SUBCOMMAND" = "promote" ]; then
        echo "Creating GitLab pipeline on main (PROMOTE_TO=${ARG})"
        glab ci run --branch main --variables-env "PROMOTE_TO:${ARG}"
    else
        echo "Creating GitLab pipeline on main (RELEASE_CREATE=1)"
        glab ci run --branch main --variables-env "RELEASE_CREATE:1"
    fi
    echo ""
    echo "Watch progress:"
    echo "  glab ci status"
else
    echo "ERROR: unsupported remote: $REMOTE" >&2
    exit 1
fi
