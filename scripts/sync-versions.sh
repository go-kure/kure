#!/bin/bash
# sync-versions.sh - Validate and manage version consistency
#
# Usage:
#   ./scripts/sync-versions.sh check      - Validate consistency
#   ./scripts/sync-versions.sh generate   - Generate docs from versions.yaml
#
# This script ensures that:
# 1. each go.mod dependency version falls WITHIN versions.yaml "supported_range"
#    (the build version is read from go.mod; there is no "current" field to sync)
# 2. dependabot.yml ignore rules match versions.yaml "max_dependabot" field
# 3. Documentation is generated from versions.yaml + go.mod

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
VERSIONS_FILE="$REPO_ROOT/versions.yaml"
GO_MOD_FILE="$REPO_ROOT/go.mod"
DEPENDABOT_FILE="$REPO_ROOT/.github/dependabot.yml"
DOCS_FILE="$REPO_ROOT/docs/compatibility.md"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Logging functions
error() { echo -e "${RED}ERROR: $1${NC}" >&2; }
success() { echo -e "${GREEN}✓ $1${NC}"; }
warning() { echo -e "${YELLOW}⚠ $1${NC}"; }
info() { echo "$1"; }

# Check if yq is installed
check_dependencies() {
    if ! command -v yq &> /dev/null; then
        error "yq is required but not installed. Install with: brew install yq"
        exit 1
    fi
}

# Extract version from go.mod for a given module
get_gomod_version() {
    local module="$1"
    # Extract version from go.mod (handles both direct and replace directives)
    local version

    # First check replace directives (format: "module => module version")
    version=$(grep -E "^\s*${module} =>" "$GO_MOD_FILE" | awk '{print $NF}' | head -n1)

    # If not found in replace, check require section
    if [[ -z "$version" ]]; then
        version=$(grep -E "^\s*${module} " "$GO_MOD_FILE" | grep -v "=>" | awk '{print $2}' | head -n1)
    fi

    echo "$version"
}

# True (exit 0) if the version string is a Go pseudo-version (untagged module
# pinned to a commit, e.g. v0.0.0-20260213133823-31b0c7c37342). Such versions
# carry no meaningful semver and are skipped by the range guard.
is_pseudo_version() {
    [[ "$1" =~ -[0-9]{14}-[0-9a-f]{12}$ ]]
}

# Turn a "major.minor" string into a comparable integer key (major*1000+minor).
mm_key() {
    local mm="$1"
    local major="${mm%%.*}"
    local minor="${mm#*.}"
    minor="${minor%%.*}"
    echo $((10#$major * 1000 + 10#$minor))
}

# Extract "major.minor" from a full version, applying version_basis normalization.
# For version_basis == kubernetes, k8s.io/* modules are v0.N.x but the range is
# expressed in cluster terms (1.N), so 0.N is normalized to 1.N.
version_mm() {
    local version="$1" basis="$2"
    local ver="${version#v}"
    ver="${ver%%-*}"   # drop any prerelease/build suffix
    local major="${ver%%.*}"
    local rest="${ver#*.}"
    local minor="${rest%%.*}"
    if [[ "$basis" == "kubernetes" && "$major" == "0" ]]; then
        major=1
    fi
    echo "${major}.${minor}"
}

# Validate that each go.mod dependency version falls within supported_range
validate_gomod() {
    local errors=0
    info "Validating go.mod versions..."

    # Check Go version (mise.toml is authoritative; versions.yaml mirrors it)
    local go_current
    go_current=$(yq '.go.current' "$VERSIONS_FILE")
    local gomod_go_version
    gomod_go_version=$(grep '^go ' "$GO_MOD_FILE" | awk '{print $2}')

    if [[ "$gomod_go_version" != "$go_current" ]]; then
        error "Go version mismatch: go.mod has '$gomod_go_version', versions.yaml expects '$go_current'"
        errors=$((errors + 1))
    else
        success "Go version matches: $go_current"
    fi

    # Check infrastructure dependencies against their supported_range
    local deps
    deps=$(yq '.infrastructure | keys | .[]' "$VERSIONS_FILE")

    while IFS= read -r dep; do
        local go_module supported basis
        go_module=$(yq ".infrastructure.${dep}.go_module" "$VERSIONS_FILE")
        supported=$(yq ".infrastructure.${dep}.supported_range" "$VERSIONS_FILE")
        basis=$(yq ".infrastructure.${dep}.version_basis // \"semver\"" "$VERSIONS_FILE")

        if [[ "$go_module" == "null" ]]; then
            continue
        fi

        local actual_version
        actual_version=$(get_gomod_version "$go_module")
        actual_version="${actual_version#v}"

        if [[ -z "$actual_version" ]]; then
            warning "Module $go_module not found in go.mod (may be transitive)"
            continue
        fi

        if is_pseudo_version "$actual_version"; then
            info "$dep: $actual_version (pseudo-version — range check skipped)"
            continue
        fi

        if [[ "$supported" == "null" || -z "$supported" ]]; then
            warning "$dep: no supported_range declared — skipping range check"
            continue
        fi

        # Parse supported_range: "A.B - C.D" (range) or "A.B" (single major.minor)
        local lo_mm hi_mm
        if [[ "$supported" == *" - "* ]]; then
            lo_mm="${supported%% - *}"
            hi_mm="${supported##* - }"
        else
            lo_mm="$supported"
            hi_mm="$supported"
        fi

        local ver_mm ver_key lo_key hi_key
        ver_mm=$(version_mm "$actual_version" "$basis")
        ver_key=$(mm_key "$ver_mm")
        lo_key=$(mm_key "$lo_mm")
        hi_key=$(mm_key "$hi_mm")

        if (( ver_key < lo_key || ver_key > hi_key )); then
            error "$dep $ver_mm (go.mod $go_module v$actual_version) is outside supported_range \"$supported\". Update supported_range + notes in versions.yaml after confirming API compatibility."
            errors=$((errors + 1))
        else
            success "$dep: v$actual_version within supported_range \"$supported\""
        fi
    done <<< "$deps"

    return $errors
}

# Validate that dependabot.yml ignore rules match versions.yaml
validate_dependabot() {
    local errors=0
    info ""
    info "Validating dependabot.yml ignore rules..."

    # This is a basic check - full validation would parse YAML
    # For now, just check that key dependencies are present in ignore list

    local deps
    deps=$(yq '.infrastructure | to_entries | .[] | select((.value.max_dependabot == null) | not) | .key' "$VERSIONS_FILE") || true

    if [[ -z "$deps" ]]; then
        success "No max_dependabot constraints to validate"
        return 0
    fi

    while IFS= read -r dep; do
        [[ -z "$dep" ]] && continue
        local go_module
        go_module=$(yq ".infrastructure.${dep}.go_module" "$VERSIONS_FILE")
        local max_version
        max_version=$(yq ".infrastructure.${dep}.max_dependabot" "$VERSIONS_FILE")

        # Check if dependency appears in dependabot ignore section
        # Match both exact names and wildcard patterns (e.g., github.com/fluxcd/*)
        # We check multiple wildcard levels to catch patterns like github.com/org/*
        local matched=false

        # Check exact match (look for dependency-name: "module")
        if grep -qE "dependency-name:.*\"$go_module\"" "$DEPENDABOT_FILE" 2>/dev/null; then
            matched=true
        else
            # Check wildcard patterns by iteratively removing path components
            local module_path="$go_module"
            while [[ "$module_path" == */* ]]; do
                # Remove last component and add wildcard (escape * for grep)
                local parent_pattern
                parent_pattern=$(echo "$module_path" | sed 's|/[^/]*$|/\\*|')
                if grep -qE "dependency-name:.*\"$parent_pattern\"" "$DEPENDABOT_FILE" 2>/dev/null; then
                    matched=true
                    break
                fi
                # Move up one level
                module_path=$(echo "$module_path" | sed 's|/[^/]*$||')
            done
        fi

        if [[ "$matched" == "true" ]]; then
            success "$dep: ignore rule present"
        else
            warning "Dependency $go_module (max: $max_version) not found in dependabot ignore rules"
            errors=$((errors + 1))
        fi
    done <<< "$deps"

    if [[ $errors -eq 0 ]]; then
        success "Dependabot ignore rules look consistent"
    fi

    return 0  # Don't fail on dependabot warnings for now
}

# Generate compatibility documentation
generate_docs() {
    info "Generating compatibility documentation..."

    cat > "$DOCS_FILE" << 'EOF'
<!-- Generated by scripts/sync-versions.sh from versions.yaml + go.mod. Do not edit by hand. -->
# Kure Compatibility Matrix

This document describes the versions of infrastructure tools that Kure supports.
It is generated from `versions.yaml` (deployment compatibility metadata) plus
`go.mod` (the build versions).

## Version Philosophy

Kure maintains two version concepts for each dependency:

1. **Build Version** (read from go.mod): The exact library version Kure imports and builds against
2. **Deployment Compatibility** (`supported_range` in versions.yaml): The range of deployed tool versions that Kure can generate YAML for

## Go Version

EOF

    local go_version
    go_version=$(yq '.go.current' "$VERSIONS_FILE")
    echo "**Current:** Go $go_version" >> "$DOCS_FILE"
    echo "" >> "$DOCS_FILE"
    echo "## Infrastructure Dependencies" >> "$DOCS_FILE"
    echo "" >> "$DOCS_FILE"
    echo "| Tool | Build Version | Deployment Compatibility | Notes |" >> "$DOCS_FILE"
    echo "|------|---------------|-------------------------|-------|" >> "$DOCS_FILE"

    local deps
    deps=$(yq '.infrastructure | keys | .[]' "$VERSIONS_FILE")

    while IFS= read -r dep; do
        local go_module
        go_module=$(yq ".infrastructure.${dep}.go_module" "$VERSIONS_FILE")
        local supported
        supported=$(yq ".infrastructure.${dep}.supported_range" "$VERSIONS_FILE")
        local notes
        notes=$(yq ".infrastructure.${dep}.notes" "$VERSIONS_FILE")

        if [[ "$notes" == "null" ]]; then
            notes=""
        fi
        # Collapse multi-line notes into a single Markdown table cell
        notes="${notes//$'\n'/ }"

        # Build version comes from go.mod (the pin), not versions.yaml
        local build_version
        build_version=$(get_gomod_version "$go_module")
        build_version="${build_version#v}"
        if [[ -z "$build_version" ]]; then
            build_version="(transitive)"
        fi

        echo "| $dep | $build_version | $supported | $notes |" >> "$DOCS_FILE"
    done <<< "$deps"

    cat >> "$DOCS_FILE" << 'EOF'

## Understanding the Matrix

### Build Version (go.mod)
The version Kure imports and builds against — read directly from `go.mod`, the single
source of truth for the pin. CI (`sync-versions.sh check`) asserts it falls within the
declared `supported_range`.

### Deployment Compatibility
The range of versions that Kure can generate valid YAML for. Kure may generate YAML compatible with older or newer versions than it builds against.

For example:
- Kure builds against cert-manager 1.16.2
- But generates YAML compatible with cert-manager 1.14.x, 1.15.x, and 1.16.x

## Upgrading Dependencies

When upgrading a dependency:

1. Run `go get <module>@<version>` to update go.mod
2. Update code for any API changes
3. If the new version lands **outside** `supported_range`, widen the range and update
   `notes` in `versions.yaml` (only after confirming API compatibility). In-range patch
   bumps need no `versions.yaml` change.
4. Run `./scripts/sync-versions.sh generate` to update docs
5. Run `./scripts/sync-versions.sh check` to validate consistency

## Related Issues

- [#133](https://github.com/go-kure/kure/issues/133) - Go 1.25 upgrade tracking
- [#128](https://github.com/go-kure/kure/issues/128) - FluxCD ecosystem upgrade (blocked by Go 1.25)

EOF

    success "Generated $DOCS_FILE"
}

# Main command router
main() {
    local command="${1:-check}"

    check_dependencies

    case "$command" in
        check)
            info "=== Version Consistency Check ==="
            info ""
            local gomod_result=0
            validate_gomod || gomod_result=$?
            validate_dependabot

            if [[ $gomod_result -eq 0 ]]; then
                info ""
                success "All version checks passed!"
                exit 0
            else
                info ""
                error "Version validation failed"
                exit 1
            fi
            ;;
        generate)
            generate_docs
            success "Documentation generated successfully"
            exit 0
            ;;
        *)
            error "Unknown command: $command"
            echo "Usage: $0 {check|generate}"
            exit 1
            ;;
    esac
}

main "$@"
