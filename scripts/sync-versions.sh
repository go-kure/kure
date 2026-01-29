#!/bin/bash
# sync-versions.sh - Validate and manage version consistency
#
# Usage:
#   ./scripts/sync-versions.sh check      - Validate consistency
#   ./scripts/sync-versions.sh generate   - Generate docs from versions.yaml
#
# This script ensures that:
# 1. go.mod versions match versions.yaml "current" field EXACTLY
# 2. dependabot.yml ignore rules match versions.yaml "max_dependabot" field
# 3. Documentation is generated from versions.yaml

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

# Validate that go.mod versions match versions.yaml current field
validate_gomod() {
    local errors=0
    info "Validating go.mod versions..."

    # Check Go version
    local go_current
    go_current=$(yq '.go.current' "$VERSIONS_FILE")
    local gomod_go_version
    gomod_go_version=$(grep '^go ' "$GO_MOD_FILE" | awk '{print $2}')

    if [[ "$gomod_go_version" != "$go_current" ]]; then
        error "Go version mismatch: go.mod has '$gomod_go_version', versions.yaml expects '$go_current'"
        ((errors++))
    else
        success "Go version matches: $go_current"
    fi

    # Check infrastructure dependencies
    local deps
    deps=$(yq '.infrastructure | keys | .[]' "$VERSIONS_FILE")

    while IFS= read -r dep; do
        local go_module
        go_module=$(yq ".infrastructure.${dep}.go_module" "$VERSIONS_FILE")
        local expected_version
        expected_version=$(yq ".infrastructure.${dep}.current" "$VERSIONS_FILE")

        if [[ "$go_module" == "null" ]]; then
            continue
        fi

        local actual_version
        actual_version=$(get_gomod_version "$go_module")

        # Remove 'v' prefix if present for comparison
        actual_version="${actual_version#v}"

        if [[ -z "$actual_version" ]]; then
            warning "Module $go_module not found in go.mod (may be transitive)"
            continue
        fi

        if [[ "$actual_version" != "$expected_version" ]]; then
            error "Version mismatch for $go_module: go.mod has 'v$actual_version', versions.yaml expects '$expected_version'"
            ((errors++))
        else
            success "$dep: $actual_version"
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
    deps=$(yq '.infrastructure | to_entries | .[] | select(.value.max_dependabot != null) | .key' "$VERSIONS_FILE")

    while IFS= read -r dep; do
        local go_module
        go_module=$(yq ".infrastructure.${dep}.go_module" "$VERSIONS_FILE")
        local max_version
        max_version=$(yq ".infrastructure.${dep}.max_dependabot" "$VERSIONS_FILE")

        # Check if dependency appears in dependabot ignore section
        if ! grep -q "$go_module" "$DEPENDABOT_FILE" 2>/dev/null; then
            warning "Dependency $go_module (max: $max_version) not found in dependabot ignore rules"
            ((errors++))
        else
            success "$dep: ignore rule present"
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
# Kure Compatibility Matrix

This document describes the versions of infrastructure tools that Kure supports.

## Version Philosophy

Kure maintains two version concepts for each dependency:

1. **Build Version** (`current` in versions.yaml): The exact library version Kure imports in go.mod
2. **Deployment Compatibility** (`supported_range`): The range of deployed tool versions that Kure can generate YAML for

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
        local current
        current=$(yq ".infrastructure.${dep}.current" "$VERSIONS_FILE")
        local supported
        supported=$(yq ".infrastructure.${dep}.supported_range" "$VERSIONS_FILE")
        local notes
        notes=$(yq ".infrastructure.${dep}.notes" "$VERSIONS_FILE")

        if [[ "$notes" == "null" ]]; then
            notes=""
        fi

        echo "| $dep | $current | $supported | $notes |" >> "$DOCS_FILE"
    done <<< "$deps"

    cat >> "$DOCS_FILE" << 'EOF'

## Understanding the Matrix

### Build Version (go.mod)
The version Kure imports and builds against. This is validated by CI to match `versions.yaml`.

### Deployment Compatibility
The range of versions that Kure can generate valid YAML for. Kure may generate YAML compatible with older or newer versions than it builds against.

For example:
- Kure builds against cert-manager 1.16.2
- But generates YAML compatible with cert-manager 1.14.x, 1.15.x, and 1.16.x

## Upgrading Dependencies

When upgrading a dependency:

1. Update `versions.yaml` with new `current` and `supported_range`
2. Run `go get <module>@<version>` to update go.mod
3. Update code for any API changes
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
            validate_gomod
            local gomod_result=$?
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
