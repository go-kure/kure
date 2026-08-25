#!/bin/bash
# sync-versions.sh - Validate and manage version consistency
#
# Usage:
#   ./scripts/sync-versions.sh check      - Validate consistency
#   ./scripts/sync-versions.sh generate   - Generate docs and the pkg/versions Go API from versions.yaml
#
# This script ensures that:
# 1. each go.mod dependency version falls WITHIN versions.yaml "supported_range"
#    (the build version is read from go.mod; there is no "current" field to sync)
# 2. Documentation is generated from versions.yaml + go.mod
# 3. go.mod's "// Current pin: ..." comment matches its k8s.io/api replace directive
# 4. versions.yaml notes never carry a raw commit SHA (must reference a vendor-guard-checked
#    ref: pin instead, see scripts/vendor-guard.sh)
# 5. pkg/versions/versions_gen.go (the published Go API's generated data) matches
#    versions.yaml -- see generate_go_api / validate_go_api_drift

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
VERSIONS_FILE="$REPO_ROOT/versions.yaml"
GO_MOD_FILE="$REPO_ROOT/go.mod"
DOCS_FILE="$REPO_ROOT/docs/compatibility.md"
GO_API_FILE="$REPO_ROOT/pkg/versions/versions_gen.go"

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
#
# NOTE: this only matches pseudo-version form 1 (vX.0.0-<ts>-<hash>, the
# untagged-repo case). Forms 2 and 3 (vX.Y.Z-pre.0.<ts>-<hash> and
# vX.Y.(Z+1)-0.<ts>-<hash> -- the shapes you get when the module *does* have
# tags but the pin sits between two of them) never match, and version_mm()
# below parses their leading "X.Y" correctly regardless. That is deliberate
# for barman-cloud (versions.yaml): widening this regex to also match forms
# 2/3 would route it into the pseudo-version skip branch and silently
# disable its range check. Do not widen without re-auditing every
# infrastructure entry whose pin is a form-2/3 pseudo-version.
is_pseudo_version() {
    [[ "$1" =~ -[0-9]{14}-[0-9a-f]{12}$ ]]
}

# Resolve <repo>@<tag> to a commit SHA via the GitHub API, peeling an
# annotated tag's object SHA to the commit it points at (same logic as
# scripts/sync-eso-pin.sh — kept duplicated rather than shared, since the two
# scripts have different failure-handling needs: this one is best-effort and
# must never hard-fail an offline `check` run, sync-eso-pin.sh must). Falls
# back to `git ls-remote`.
#
# Prints the commit SHA and returns 0 on success. On failure, distinguishes
# two outcomes callers must NOT collapse into one:
#   - return 1: a reachable server definitively reported the tag does not
#     exist (API 404, or ls-remote succeeded with no matching ref). This is
#     a real configuration error — e.g. a typo'd upstream_release — and
#     must not be downgraded to "could not verify."
#   - return 2: neither path could reach the server at all (DNS/connect/TLS
#     failure, timeout, rate limit). This is the "could not verify" case.
resolve_tag_commit() {
    local repo="$1" tag="$2"
    local commit="" obj_type="" api_tag_missing=0

    local raw http_code body
    raw="$(curl -sS --max-time 5 -w $'\n%{http_code}' "https://api.github.com/repos/${repo}/git/ref/tags/${tag}" 2>/dev/null)"
    if [[ $? -eq 0 && -n "$raw" ]]; then
        http_code="${raw##*$'\n'}"
        body="${raw%$'\n'*}"
        if [[ "$http_code" == "200" ]]; then
            commit="$(printf '%s' "$body" | yq -p json '.object.sha // ""' 2>/dev/null || true)"
            obj_type="$(printf '%s' "$body" | yq -p json '.object.type // ""' 2>/dev/null || true)"
        elif [[ "$http_code" == "404" ]]; then
            api_tag_missing=1
        fi
        # any other http_code (403 rate-limit, 5xx, ...) is inconclusive,
        # falls through to the ls-remote fallback below.
    fi

    if [[ "$obj_type" == "tag" && -n "$commit" && "$commit" != "null" ]]; then
        local tag_response
        tag_response="$(curl -fsSL --max-time 5 "https://api.github.com/repos/${repo}/git/tags/${commit}" 2>/dev/null || true)"
        commit="$(printf '%s' "$tag_response" | yq -p json '.object.sha // ""' 2>/dev/null || true)"
    fi

    if [[ -z "$commit" || "$commit" == "null" ]]; then
        if [[ $api_tag_missing -eq 1 ]]; then
            return 1
        fi
        local ls_remote_out
        ls_remote_out="$(timeout 5 git ls-remote "https://github.com/${repo}" "refs/tags/${tag}" "refs/tags/${tag}^{}" 2>/dev/null)"
        local ls_remote_rc=$?
        commit="$(printf '%s\n' "$ls_remote_out" | awk -v r="refs/tags/${tag}^{}" '$2 == r {print $1; found=1} END {exit !found}' || true)"
        if [[ -z "$commit" ]]; then
            commit="$(printf '%s\n' "$ls_remote_out" | awk -v r="refs/tags/${tag}" '$2 == r {print $1}' | head -n1)"
        fi
        if [[ -z "$commit" ]]; then
            # ls-remote reaching the server (exit 0) with zero matching lines is
            # itself a definitive "tag does not exist," same as the API's 404 —
            # unless the API already proved reachability via a *different*
            # inconclusive status (api_reachable tracks 200/404 only), a clean
            # ls-remote exit is the strongest signal available.
            if [[ $ls_remote_rc -eq 0 ]]; then
                return 1
            fi
            return 2
        fi
    fi

    if [[ "$commit" =~ ^[0-9a-f]{40}$ ]]; then
        printf '%s' "$commit"
        return 0
    fi
    return 2
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
            local upstream_release
            upstream_release=$(yq ".infrastructure.${dep}.upstream_release // \"\"" "$VERSIONS_FILE")

            if [[ -z "$upstream_release" || "$upstream_release" == "null" ]]; then
                info "$dep: $actual_version (pseudo-version, no upstream_release declared — range check skipped)"
                continue
            fi

            # A dep declaring upstream_release is pinned to a named release, not
            # tracking main HEAD (see scripts/sync-eso-pin.sh). Assert the pin
            # hasn't drifted off that release before substituting it for the
            # range check below. The digest comparison right below is offline
            # (no network needed: upstream_release_commit is a structured field,
            # not re-resolved from the tag here) and is the check that always
            # runs. It proves go.mod matches upstream_release_commit, but not
            # that upstream_release_commit is actually the commit upstream's
            # <upstream_release> tag points to — a hand-edit that bumps
            # upstream_release without re-running sync-eso-pin.sh would pass it
            # even though upstream_release_commit is now stale. The second,
            # best-effort online check further down closes that gap.
            local upstream_release_commit
            upstream_release_commit=$(yq ".infrastructure.${dep}.upstream_release_commit // \"\"" "$VERSIONS_FILE")
            if [[ -z "$upstream_release_commit" || "$upstream_release_commit" == "null" ]]; then
                error "$dep: upstream_release is set but upstream_release_commit is missing in versions.yaml"
                errors=$((errors + 1))
                continue
            fi

            # go.mod's pseudo-version suffix is a 12-char abbreviated commit
            # digest; it must prefix the full commit declared for the release.
            local pin_digest="${actual_version: -12}"
            if [[ "$upstream_release_commit" != "$pin_digest"* ]]; then
                error "$dep: go.mod pin digest '$pin_digest' does not match declared release $upstream_release (upstream_release_commit '$upstream_release_commit') — the pin has drifted off the declared release. Run: ./scripts/sync-eso-pin.sh"
                errors=$((errors + 1))
                continue
            fi
            success "$dep: pin digest '$pin_digest' matches declared release $upstream_release"

            # Best-effort online check: confirm upstream_release_commit is
            # actually the commit <upstream_repo>'s <upstream_release> tag
            # resolves to today. resolve_tag_commit distinguishes "tag
            # definitively doesn't exist" (rc=1 — a real error, e.g. a
            # typo'd upstream_release) from "couldn't reach the server at
            # all" (rc=2 — a warning; this must never break an otherwise-
            # valid offline `check` run). A confirmed live mismatch is also
            # an error: the field pair is stale independent of go.mod.
            local upstream_repo
            upstream_repo=$(yq ".infrastructure.${dep}.upstream_repo // \"\"" "$VERSIONS_FILE")
            if [[ -n "$upstream_repo" && "$upstream_repo" != "null" ]]; then
                local resolved_commit resolve_rc=0
                resolved_commit=$(resolve_tag_commit "$upstream_repo" "$upstream_release") || resolve_rc=$?
                if [[ $resolve_rc -eq 0 ]]; then
                    if [[ "$resolved_commit" != "$upstream_release_commit" ]]; then
                        error "$dep: upstream_release_commit '$upstream_release_commit' does not match the commit $upstream_repo@$upstream_release resolves to today ('$resolved_commit') — versions.yaml was hand-edited out of sync. Run: ./scripts/sync-eso-pin.sh"
                        errors=$((errors + 1))
                        continue
                    fi
                    success "$dep: upstream_release_commit confirmed against live $upstream_repo@$upstream_release"
                elif [[ $resolve_rc -eq 1 ]]; then
                    error "$dep: tag $upstream_release does not exist in $upstream_repo (confirmed reachable) — check upstream_release for a typo"
                    errors=$((errors + 1))
                    continue
                else
                    warning "$dep: could not verify upstream_release_commit against $upstream_repo@$upstream_release live (no network or rate-limited) — offline digest check above still holds"
                fi
            fi

            # Substitute the release version and fall through to the ordinary
            # range-parsing logic below, so supported_range is enforced for
            # this dep for the first time instead of always being skipped.
            actual_version="${upstream_release#v}"
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

Consumers that need the deployment-compatibility metadata (2) or the Go toolchain
version programmatically should import `github.com/go-kure/kure/pkg/versions`
rather than parsing `versions.yaml`. Per-dependency build versions (1) are not
exported -- they change on every routine bump, so keeping them out keeps this
API's content stable; read them from `go.mod` instead.

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

# True if $1 cannot be safely embedded in a Go interpreted string literal
# ("...") by plain printf '%s' substitution: an unescaped quote or backslash
# breaks the literal, and an embedded newline splits it across source lines,
# which Go rejects outright (interpreted string literals cannot contain a
# literal newline; only backtick raw literals can, and generate_go_api never
# emits those). Used to guard every versions.yaml scalar generate_go_api emits.
go_string_literal_unsafe() {
    [[ "$1" == *'"'* || "$1" == *'\'* || "$1" == *$'\n'* ]]
}

# Generate the pkg/versions Go API from versions.yaml.
# $1: output path (drift check passes a temp file; `generate` passes $GO_API_FILE).
# Output must be gofmt-canonical -- the drift check is a byte comparison, and
# a later `gofmt -w` on the committed file would make it fail permanently.
generate_go_api() {
    local out="$1"
    info "Generating Go version metadata API..."

    local go_current
    go_current=$(yq '.go.current' "$VERSIONS_FILE")

    # go.current is hand-maintained (never Renovate-edited) and always a
    # plain Go version string in practice, but guard it the same as the
    # per-dependency fields below rather than trust that invariant silently:
    # it is emitted by a raw %s outside the loop's own guard.
    if go_string_literal_unsafe "$go_current"; then
        error "versions.yaml go.current cannot be emitted as a Go string literal: $go_current"
        return 1
    fi

    # Build into a temp buffer first, never the real $out directly: the
    # per-dependency guard below can fail mid-loop, and $out may be the
    # committed pkg/versions/versions_gen.go. Writing straight to $out would
    # leave a truncated, non-compiling file (missing closing brace and any
    # remaining entries) sitting at the committed path on that failure path.
    #
    # No RETURN trap here: this function is called from validate_go_api_drift,
    # which sets its own RETURN trap for its own local $expected. A RETURN
    # trap set here would clobber that one for the rest of the caller's
    # lifetime -- traps aren't scoped per call frame in bash -- and the
    # caller's later return would then fire OUR trap against a $tmp that has
    # already gone out of scope, aborting under set -u ("tmp: unbound
    # variable"), even on a clean, successful validate_go_api_drift run.
    # Clean up explicitly on the one failure path instead.
    #
    # The buffer must live in $out's own directory, not the bare `mktemp`
    # default (/tmp or $TMPDIR): on a layout where that's a different
    # filesystem from $out (e.g. tmpfs /tmp vs an on-disk checkout), `mv`
    # across devices falls back to copy+unlink -- not atomic -- and an
    # interruption mid-copy can leave $out truncated, exactly what buffering
    # into a temp file was meant to prevent. Co-locating keeps `mv` a same-
    # filesystem rename(2), which is atomic.
    local tmp
    tmp=$(mktemp "$(dirname "$out")/.versions_gen.XXXXXX")

    {
        printf '// Code generated by scripts/sync-versions.sh from versions.yaml. DO NOT EDIT.\n\n'
        printf 'package versions\n\n'
        printf '// GoVersion is the go.current field of versions.yaml.\n'
        printf 'const GoVersion string = "%s"\n\n' "$go_current"
        printf 'var infrastructure = []Dependency{\n'
    } > "$tmp"

    local deps
    deps=$(yq '.infrastructure | keys | .[]' "$VERSIONS_FILE")

    while IFS= read -r dep; do
        local go_module supported basis lo hi field
        go_module=$(yq ".infrastructure.${dep}.go_module // \"\"" "$VERSIONS_FILE")
        supported=$(yq ".infrastructure.${dep}.supported_range // \"\"" "$VERSIONS_FILE")
        basis=$(yq ".infrastructure.${dep}.version_basis // \"semver\"" "$VERSIONS_FILE")

        # Same split as the range guard in validate_gomod: keep the two in
        # step, or the exported bounds and the CI range check could disagree.
        if [[ "$supported" == *" - "* ]]; then
            lo="${supported%% - *}"; hi="${supported##* - }"
        else
            lo="$supported"; hi="$supported"
        fi

        # Refuse to emit a file that will not compile. $out is untouched at
        # this point -- only $tmp exists; remove it explicitly before
        # returning (see the no-RETURN-trap note above).
        for field in "$dep" "$go_module" "$supported" "$basis"; do
            if go_string_literal_unsafe "$field"; then
                error "versions.yaml value cannot be emitted as a Go string literal: $field"
                rm -f "$tmp"
                return 1
            fi
        done

        {
            printf '\t{\n'
            printf '\t\t%-15s "%s",\n' 'Name:' "$dep"
            printf '\t\t%-15s "%s",\n' 'GoModule:' "$go_module"
            printf '\t\t%-15s "%s",\n' 'SupportedRange:' "$supported"
            printf '\t\t%-15s "%s",\n' 'Min:' "$lo"
            printf '\t\t%-15s "%s",\n' 'Max:' "$hi"
            printf '\t\t%-15s "%s",\n' 'VersionBasis:' "$basis"
            printf '\t},\n'
        } >> "$tmp"
    done <<< "$deps"

    printf '}\n' >> "$tmp"

    # mktemp creates $tmp at mode 0600 (owner-only); mv preserves that mode
    # onto $out. Without this, a regenerated versions_gen.go would go from
    # the committed file's normal 0644 to owner-read-only on disk -- git
    # itself is unaffected (it only tracks the executable bit, and `generate`
    # is never run in CI), but a local `generate` would leave the working
    # copy unreadable to anything not running as the same user.
    chmod 644 "$tmp"

    # Only now does $out change -- atomically, and only on full success.
    mv "$tmp" "$out"
    success "Generated $out"
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

# Assert pkg/versions/versions_gen.go matches what generate_go_api would
# produce right now. Same reasoning as validate_docs_drift: without this, a
# bump that changed a supported_range leaves the committed Go API silently
# stale, and a consumer pinning kure by module version reads the old range.
validate_go_api_drift() {
    info ""
    info "Validating generated Go version API is current..."

    local expected
    expected=$(mktemp)
    # shellcheck disable=SC2064  # expand $expected now, not at trap time
    trap "rm -f '$expected' '$expected.diff'" RETURN

    generate_go_api "$expected" >/dev/null || return 1

    if diff -u "$GO_API_FILE" "$expected" > "$expected.diff" 2>&1; then
        success "$(basename "$GO_API_FILE") is up to date"
        rm -f "$expected.diff"
        return 0
    fi

    error "$(basename "$GO_API_FILE") is out of date — run: ./scripts/sync-versions.sh generate"
    sed -e "1s|.*|--- committed: ${GO_API_FILE#"$REPO_ROOT"/}|" \
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
            validate_go_api_drift || gomod_result=1

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
            generate_go_api "$GO_API_FILE"
            success "Documentation and Go API generated successfully"
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
