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
# 2. Documentation is generated from versions.yaml + go.mod
# 3. go.mod's "// Current pin: ..." comment matches its k8s.io/api replace directive
# 4. versions.yaml notes never carry a raw commit SHA (must reference a vendor-guard-checked
#    ref: pin instead, see scripts/vendor-guard.sh)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
VERSIONS_FILE="$REPO_ROOT/versions.yaml"
GO_MOD_FILE="$REPO_ROOT/go.mod"
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

# Extract a module's version from go.mod's replace directive ONLY -- no
# require-section fallback. Unlike get_gomod_version (used generically for
# range checks, where a module may legitimately have no replace directive),
# the pin comment specifically asserts the k8s.io/api *replace* directive's
# version, per its own doc comment. If the require fallback were used here,
# deleting the replace line while require still lists the same version
# (as it always does when the replace target is unmodified) would make
# expected_gomod_pin_comment silently recompute the same "expected" value
# from require, and validate_gomod_pin_comment would report a match despite
# the replace directive itself being gone -- exactly the drift this check
# exists to catch. Returns empty if no replace directive matches.
get_gomod_replace_version() {
    local module="$1"
    grep -E "^\s*${module} =>" "$GO_MOD_FILE" | awk '{print $NF}' | head -n1
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

# Read go.mod's "// Current pin: vX.Y.Z (Kubernetes 1.N)" comment, if present.
get_gomod_pin_comment() {
    grep -E '^// Current pin: ' "$GO_MOD_FILE" | head -n1
}

# Compute the "// Current pin: ..." comment that go.mod's k8s.io/api replace
# directive implies — the single source of truth for the pin
# (docs/dependency-updates.md). Reuses version_mm's kubernetes basis, which
# maps v0.N.x -> "1.N".
expected_gomod_pin_comment() {
    local api_version
    api_version=$(get_gomod_replace_version "k8s.io/api")
    if [[ -z "$api_version" ]]; then
        error "k8s.io/api replace directive not found in go.mod"
        return 1
    fi
    local k8s_mm
    k8s_mm=$(version_mm "$api_version" "kubernetes")
    printf '// Current pin: %s (Kubernetes %s)\n' "$api_version" "$k8s_mm"
}

# Rewrite go.mod's "// Current pin: ..." comment to match the k8s.io/api
# replace directive. A no-op (byte-for-byte) when already in sync. If the
# comment line is missing entirely (e.g. hand-edited away), insert it
# immediately above the "replace (" block rather than silently leaving it
# absent — otherwise `generate` would report success while
# validate_gomod_pin_comment kept failing with no fix available. If no
# "replace (" block anchor exists at all (e.g. a single-line replace
# directive), fail loudly instead of silently leaving the comment absent —
# exactly the loop this function exists to prevent.
sync_gomod_pin_comment() {
    local expected
    expected=$(expected_gomod_pin_comment) || return 1

    local tmp
    tmp=$(mktemp)
    if grep -qE '^// Current pin: ' "$GO_MOD_FILE"; then
        awk -v repl="$expected" '
            /^\/\/ Current pin: / { print repl; next }
            { print }
        ' "$GO_MOD_FILE" > "$tmp"
    else
        if ! awk -v repl="$expected" '
            !inserted && /^replace \(/ { print repl; print "//"; inserted=1 }
            { print }
            END { exit !inserted }
        ' "$GO_MOD_FILE" > "$tmp"; then
            rm -f "$tmp"
            error "go.mod has no 'replace (' block to anchor the pin comment above — insert '// Current pin: ...' by hand"
            return 1
        fi
    fi
    # mktemp creates the file mode 0600; mv would install that mode over
    # go.mod, silently stripping its group/other read permission. Preserve
    # the original file's mode instead of replacing the inode.
    cat "$tmp" > "$GO_MOD_FILE" && rm -f "$tmp"
}

# Assert go.mod's "// Current pin: ..." comment matches the k8s.io/api
# replace directive it sits above. Without this, a version bump that moved
# the replace directive left the hand-written comment silently drifted from
# the real pin — only caught by AI reviewers, not CI.
validate_gomod_pin_comment() {
    local current expected
    # get_gomod_pin_comment's grep|head pipeline exits non-zero exactly when
    # the comment is absent -- the case this function exists to report. Under
    # this script's `set -e`/pipefail that would abort here instead of
    # reaching the explicit -z branch below; guard it the same way
    # expected_gomod_pin_comment's caller already does.
    current=$(get_gomod_pin_comment || true)
    expected=$(expected_gomod_pin_comment) || return 1

    if [[ -z "$current" ]]; then
        error "go.mod is missing the '// Current pin: ' comment above the k8s.io replace block"
        return 1
    fi

    if [[ "$current" != "$expected" ]]; then
        error "go.mod pin comment out of date: has '$current', expected '$expected'. Run: ./scripts/sync-versions.sh generate"
        return 1
    fi

    success "go.mod pin comment matches: $expected"
    return 0
}

# Reject a raw commit SHA (40- or 12-hex, the pseudo-version suffix length)
# inside any infrastructure notes: block in versions.yaml. This is the actual
# drift vector this check exists for: a hand-written note pins a commit that
# the go.mod pseudo-version already encodes, and the note silently goes stale
# when the pin moves. Deliberately NOT a ban on patch-level semver prose (e.g.
# metallb's "v0.16.0 is a minor release...") — only a bare hex commit SHA.
validate_no_sha_in_notes() {
    local errors=0
    info ""
    info "Validating no raw commit SHAs in versions.yaml notes..."

    local deps
    deps=$(yq '.infrastructure | keys | .[]' "$VERSIONS_FILE")

    while IFS= read -r dep; do
        local notes
        notes=$(yq ".infrastructure.${dep}.notes" "$VERSIONS_FILE")
        [[ "$notes" == "null" ]] && continue

        local hit
        hit=$(printf '%s' "$notes" | grep -oiE '\b[0-9a-f]{40}\b|\b[0-9a-f]{12}\b' | head -n1 || true)
        if [[ -n "$hit" ]]; then
            error "$dep: notes contains a raw commit SHA ($hit) — reference the pseudo-version pinned in go.mod instead, not the literal commit"
            errors=$((errors + 1))
        fi
    done <<< "$deps"

    if [[ $errors -eq 0 ]]; then
        success "No raw commit SHAs in versions.yaml notes"
    fi

    return $errors
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

# Generate compatibility documentation
# $1: output path. The drift check passes a temp file here so it can compare
# without touching the working tree; `generate` passes $DOCS_FILE itself.
generate_docs() {
    local DOCS_FILE="$1"

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

For example, Kure may build against a single cert-manager patch release while the
generated YAML stays valid across several older and newer minor releases within its
supported range.

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

# Assert docs/compatibility.md matches what generate_docs would produce right now.
#
# Without this, `check` validated ranges only, so any bump that changed a build
# version or a supported_range left the committed matrix silently stale until
# someone happened to re-run `generate`. That drift was reported as a review
# finding on three separate dependency PRs before this guard existed.
validate_docs_drift() {
    info ""
    info "Validating generated documentation is current..."

    local expected
    expected=$(mktemp)
    # shellcheck disable=SC2064  # expand $expected now, not at trap time
    trap "rm -f '$expected' '$expected.diff'" RETURN

    generate_docs "$expected" >/dev/null

    if diff -u "$DOCS_FILE" "$expected" > "$expected.diff" 2>&1; then
        success "$(basename "$DOCS_FILE") is up to date"
        rm -f "$expected.diff"
        return 0
    fi

    error "$(basename "$DOCS_FILE") is out of date — run: ./scripts/sync-versions.sh generate"
    # Label the diff from the reader's point of view: '-' is what is committed,
    # '+' is what versions.yaml + go.mod currently imply.
    sed -e "1s|.*|--- committed: ${DOCS_FILE#"$REPO_ROOT"/}|" \
        -e "2s|.*|+++ expected (regenerated)|" "$expected.diff" >&2
    rm -f "$expected.diff"
    return 1
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
            validate_gomod_pin_comment || gomod_result=1
            validate_no_sha_in_notes || gomod_result=1
            validate_docs_drift || gomod_result=1

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
            sync_gomod_pin_comment
            generate_docs "$DOCS_FILE"
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
