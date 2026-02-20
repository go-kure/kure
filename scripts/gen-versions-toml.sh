#!/usr/bin/env bash
# gen-versions-toml.sh — Generate a versions.toml Hugo config overlay for Relearn theme versioning.
#
# Reads git tags to discover stable releases and produces a TOML file that Hugo
# merges with hugo.toml via --config hugo.toml,versions.toml.
#
# Usage:
#   ./scripts/gen-versions-toml.sh --version <version> [--latest <tag>] [--base-url <url>] [--output <path>]
#
# Options:
#   --version   The version identifier for this build (e.g., "v0.1.0" or "dev"). Required.
#   --latest    The tag to mark as isLatest (e.g., "v0.1.0"). Defaults to the highest stable tag.
#   --base-url  Site base URL. Defaults to "https://www.gokure.dev".
#   --output    Output file path. Defaults to "site/versions.toml".
#
# Examples:
#   # Dev build (from main):
#   ./scripts/gen-versions-toml.sh --version dev
#
#   # Stable build for v0.1.0:
#   ./scripts/gen-versions-toml.sh --version v0.1.0 --latest v0.1.0
#
#   # Rebuild old version after new release:
#   ./scripts/gen-versions-toml.sh --version v0.1.0 --latest v0.2.0

set -euo pipefail

VERSION=""
LATEST=""
BASE_URL="https://www.gokure.dev"
OUTPUT="site/versions.toml"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --version)  VERSION="$2";  shift 2 ;;
        --latest)   LATEST="$2";   shift 2 ;;
        --base-url) BASE_URL="$2"; shift 2 ;;
        --output)   OUTPUT="$2";   shift 2 ;;
        *)
            echo "Unknown option: $1" >&2
            exit 1
            ;;
    esac
done

if [[ -z "$VERSION" ]]; then
    echo "Error: --version is required" >&2
    exit 1
fi

# Collect stable tags (vX.Y.Z without pre-release suffix), deduplicate to minor level.
# For each minor (vX.Y), keep only the highest patch version.
declare -A minor_map
minor_count=0
stable_tags="$(git tag --list 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' || true)"

if [[ -n "$stable_tags" ]]; then
    while IFS= read -r tag; do
        minor="${tag%.*}"
        existing="${minor_map[$minor]:-}"
        if [[ -z "$existing" ]] || [[ "$(printf '%s\n%s\n' "$existing" "$tag" | sort -V | tail -1)" == "$tag" ]]; then
            minor_map[$minor]="$tag"
        fi
    done <<< "$stable_tags"
    minor_count=${#minor_map[@]}
fi

# Sort minors by version (ascending)
sorted_minors=()
if [[ $minor_count -gt 0 ]]; then
    while IFS= read -r m; do
        sorted_minors+=("$m")
    done < <(printf '%s\n' "${!minor_map[@]}" | sort -V)
fi

# Determine latest: use --latest if provided, otherwise highest stable tag
if [[ -z "$LATEST" && ${#sorted_minors[@]} -gt 0 ]]; then
    last_minor="${sorted_minors[-1]}"
    LATEST="${minor_map[$last_minor]}"
fi

# --- Generate TOML ---

{
    echo "[params]"
    echo "  version = '${VERSION}'"
    echo ""

    # Stable version entries (newest first for display order)
    for ((i=${#sorted_minors[@]}-1; i>=0; i--)); do
        minor="${sorted_minors[$i]}"
        tag="${minor_map[$minor]}"

        echo "  [[params.versions]]"
        echo "    identifier = '${minor}'"

        if [[ "$tag" == "$LATEST" ]]; then
            echo "    title = '${minor} (Latest)'"
            echo "    baseURL = '${BASE_URL}/'"
            echo "    isLatest = true"
        else
            echo "    title = '${minor}'"
            echo "    baseURL = '${BASE_URL}/${minor}/'"
        fi
        echo ""
    done

    # Dev entry
    echo "  [[params.versions]]"
    echo "    identifier = 'dev'"
    echo "    title = 'Development'"
    echo "    baseURL = '${BASE_URL}/dev/'"

} > "$OUTPUT"

echo "Generated $OUTPUT (version=${VERSION}, latest=${LATEST:-none}, entries=$((${#sorted_minors[@]} + 1)))"
