#!/bin/sh
# Release automation script for kure
# Creates release commits, tags, and (in CI) pushes to main.
#
# Usage:
#   ./scripts/release.sh alpha          # Release alpha (use DRY_RUN=1 for preview)
#   ./scripts/release.sh bump minor     # Bump minor version
#   RELEASE_TYPE=alpha ./scripts/release.sh  # Via env var (CI)
#
# Environment:
#   RELEASE_TYPE   Release type: alpha|beta|rc|stable|bump (or positional arg)
#   RELEASE_SCOPE  Bump scope: minor|major (or positional arg after "bump")
#   DRY_RUN        1 = preview without changes (default: 0)
#   CI             Set by CI runners; enables push and git identity setup
#
# Requirements:
#   - git-cliff installed (via mise)
#   - VERSION file at repository root
#   - cliff.toml configuration
#
# See: https://gitlab.com/autops/wharf/meta/-/blob/main/standards/release-process.md

set -eu

# ── Configuration ─────────────────────────────────────────────────────────

VERSION_FILE="VERSION"
CHANGELOG_FILE="CHANGELOG.md"
DRY_RUN="${DRY_RUN:-0}"

# Accept release type from positional args or env vars
if [ $# -ge 1 ]; then
    RELEASE_TYPE="$1"
    shift
    if [ $# -ge 1 ]; then
        RELEASE_SCOPE="$1"
        shift
    else
        RELEASE_SCOPE="${RELEASE_SCOPE:-}"
    fi
else
    RELEASE_TYPE="${RELEASE_TYPE:-}"
    RELEASE_SCOPE="${RELEASE_SCOPE:-}"
fi

# ── Colors (disabled if not a terminal) ───────────────────────────────────

if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[0;33m'
    BLUE='\033[0;34m'
    NC='\033[0m'
else
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    NC=''
fi

# ── Logging ───────────────────────────────────────────────────────────────

log_info()    { printf "${BLUE}INFO:${NC} %s\n" "$1"; }
log_success() { printf "${GREEN}OK:${NC} %s\n" "$1"; }
log_warn()    { printf "${YELLOW}WARN:${NC} %s\n" "$1"; }
log_error()   { printf "${RED}ERROR:${NC} %s\n" "$1" >&2; }
die()         { log_error "$1"; exit 1; }

# ── Version parsing ───────────────────────────────────────────────────────

read_version() {
    [ -f "$VERSION_FILE" ] || die "VERSION file not found"
    tr -d '\n' < "$VERSION_FILE"
}

is_prerelease() {
    case "$1" in
        *-alpha.*|*-beta.*|*-rc.*) return 0 ;;
        *) return 1 ;;
    esac
}

prerelease_type() {
    case "$1" in
        *-alpha.*) echo "alpha" ;;
        *-beta.*)  echo "beta" ;;
        *-rc.*)    echo "rc" ;;
        *)         echo "" ;;
    esac
}

base_part() {
    echo "$1" | sed 's/-\(alpha\|beta\|rc\)\.[0-9]*$//'
}

prerelease_number() {
    echo "$1" | sed -n 's/.*-\(alpha\|beta\|rc\)\.\([0-9]*\)$/\2/p'
}

major_part() {
    echo "$1" | sed 's/^v//' | cut -d. -f1
}

minor_part() {
    echo "$1" | sed 's/^v//' | cut -d. -f2
}

patch_part() {
    base=$(base_part "$1")
    echo "$base" | sed 's/^v//' | cut -d. -f3
}

# ── Version manipulation ──────────────────────────────────────────────────

bump_patch() {
    base=$(base_part "$1")
    major=$(major_part "$base")
    minor=$(minor_part "$base")
    patch=$(patch_part "$base")
    echo "v${major}.${minor}.$((patch + 1))"
}

bump_minor() {
    base=$(base_part "$1")
    major=$(major_part "$base")
    minor=$(minor_part "$base")
    echo "v${major}.$((minor + 1)).0"
}

bump_major() {
    base=$(base_part "$1")
    major=$(major_part "$base")
    echo "v$((major + 1)).0.0"
}

start_prerelease() {
    version="$1"
    type="$2"
    base=$(base_part "$version")
    echo "${base}-${type}.0"
}

bump_prerelease() {
    version="$1"
    type=$(prerelease_type "$version")
    base=$(base_part "$version")
    num=$(prerelease_number "$version")
    echo "${base}-${type}.$((num + 1))"
}

transition_prerelease() {
    version="$1"
    target="$2"
    base=$(base_part "$version")
    echo "${base}-${target}.0"
}

# ── Validation ────────────────────────────────────────────────────────────

validate_version() {
    echo "$1" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-alpha\.[0-9]+|-beta\.[0-9]+|-rc\.[0-9]+)?$' \
        || die "Invalid version format: $1"
}

check_local_replaces() {
    if [ -f "go.mod" ] && grep -q 'replace.*=>.*\.\./' go.mod; then
        die "go.mod contains local replace directives — remove them before releasing"
    fi
}

validate_git_state() {
    # Uncommitted changes are blocking
    if [ -n "$(git diff --cached --name-only)" ] || [ -n "$(git diff --name-only)" ]; then
        die "Working directory has uncommitted changes — commit or stash first"
    fi

    # Untracked files are a warning
    untracked=$(git ls-files --others --exclude-standard)
    if [ -n "$untracked" ]; then
        log_warn "Untracked files present (will not be included in release)"
    fi
}

check_tag_exists() {
    if git rev-parse "refs/tags/$1" >/dev/null 2>&1; then
        die "Tag $1 already exists"
    fi
}

# ── CI setup ──────────────────────────────────────────────────────────────

setup_ci() {
    log_info "CI mode: configuring git identity"
    git config user.name "wharf-release-bot"
    git config user.email "wharf-release-bot@noreply"

    log_info "Validating HEAD matches origin/main..."
    git fetch origin main
    local_head=$(git rev-parse HEAD)
    remote_main=$(git rev-parse origin/main)
    if [ "$local_head" != "$remote_main" ]; then
        die "HEAD ($local_head) != origin/main ($remote_main) — release must run on latest main"
    fi
    log_success "HEAD matches origin/main"
}

# ── Git operations ────────────────────────────────────────────────────────

write_version() {
    if [ "$DRY_RUN" = "1" ]; then
        log_info "[DRY_RUN] Would write $1 to $VERSION_FILE"
    else
        printf '%s\n' "$1" > "$VERSION_FILE"
        log_success "Updated $VERSION_FILE to $1"
    fi
}

generate_changelog() {
    if [ "$DRY_RUN" = "1" ]; then
        log_info "[DRY_RUN] Would generate changelog for $1"
    else
        log_info "Generating changelog for $1..."
        git-cliff --tag "$1" -o "$CHANGELOG_FILE"
        log_success "Updated $CHANGELOG_FILE"
    fi
}

git_commit() {
    if [ "$DRY_RUN" = "1" ]; then
        log_info "[DRY_RUN] Would commit: $1"
    else
        git add "$VERSION_FILE" "$CHANGELOG_FILE"
        git commit -m "$1"
        log_success "Created commit: $1"
    fi
}

git_tag() {
    if [ "$DRY_RUN" = "1" ]; then
        log_info "[DRY_RUN] Would create tag: $1"
    else
        git tag -a "$1" -m "Release $1"
        log_success "Created tag: $1"
    fi
}

git_push() {
    tag="$1"
    if [ "$DRY_RUN" = "1" ]; then
        log_info "[DRY_RUN] Would push: main + $tag"
        return
    fi
    if [ "${CI:-}" = "true" ]; then
        log_info "Pushing release..."
        git push --atomic origin HEAD:main "$tag"
        log_success "Pushed main + $tag"
    fi
}

# ── Release workflows ─────────────────────────────────────────────────────

release_prerelease() {
    target_type="$1"
    current=$(read_version)
    validate_version "$current"

    current_type=$(prerelease_type "$current")

    # Determine release version
    if [ -z "$current_type" ]; then
        # No prerelease suffix — start new prerelease cycle
        release_version=$(start_prerelease "$current" "$target_type")
    elif [ "$current_type" = "$target_type" ]; then
        # Same type — current version IS the release version
        release_version="$current"
    else
        # Different type — transition (e.g., alpha → beta)
        release_version=$(transition_prerelease "$current" "$target_type")
    fi

    validate_version "$release_version"

    # Compute next dev version
    next_version=$(bump_prerelease "$release_version")

    log_info "Current version:  $current"
    log_info "Release version:  $release_version"
    log_info "Next dev version: $next_version"
    echo ""

    if [ "$DRY_RUN" != "1" ]; then
        validate_git_state
        check_local_replaces
        check_tag_exists "$release_version"
    fi

    # Write release version if different from current
    if [ "$release_version" != "$current" ]; then
        write_version "$release_version"
    fi

    # Generate changelog and create release commit + tag
    generate_changelog "$release_version"
    git_commit "release: $release_version"
    git_tag "$release_version"

    # Bump to next prerelease for development
    write_version "$next_version"
    if [ "$DRY_RUN" = "1" ]; then
        log_info "[DRY_RUN] Would commit: chore: bump version: $release_version -> $next_version"
    else
        git add "$VERSION_FILE"
        git commit -m "chore: bump version: $release_version -> $next_version"
        log_success "Bumped version to $next_version"
    fi

    git_push "$release_version"

    echo ""
    log_success "Release $release_version complete!"
}

release_stable() {
    current=$(read_version)
    validate_version "$current"

    if ! is_prerelease "$current"; then
        die "Current version $current is not a prerelease — use 'bump' to start a new cycle first"
    fi

    # Strip prerelease suffix for stable release
    release_version=$(base_part "$current")
    validate_version "$release_version"

    # Next development cycle: bump patch and start alpha
    next_base=$(bump_patch "$release_version")
    next_version=$(start_prerelease "$next_base" "alpha")

    log_info "Current version:  $current"
    log_info "Release version:  $release_version"
    log_info "Next dev version: $next_version"
    echo ""

    if [ "$DRY_RUN" != "1" ]; then
        validate_git_state
        check_local_replaces
        check_tag_exists "$release_version"
    fi

    write_version "$release_version"
    generate_changelog "$release_version"
    git_commit "release: $release_version"
    git_tag "$release_version"

    # Start next patch cycle
    write_version "$next_version"
    if [ "$DRY_RUN" = "1" ]; then
        log_info "[DRY_RUN] Would commit: chore: start next cycle: $next_version"
    else
        git add "$VERSION_FILE"
        git commit -m "chore: start next cycle: $next_version"
        log_success "Started next cycle at $next_version"
    fi

    git_push "$release_version"

    echo ""
    log_success "Release $release_version complete!"
}

release_bump() {
    scope="$1"
    [ -n "$scope" ] || die "bump requires scope: minor or major"

    current=$(read_version)
    validate_version "$current"

    base=$(base_part "$current")

    case "$scope" in
        minor) new_base=$(bump_minor "$base") ;;
        major) new_base=$(bump_major "$base") ;;
        *)     die "Invalid bump scope: $scope (use 'minor' or 'major')" ;;
    esac

    next_version=$(start_prerelease "$new_base" "alpha")

    log_info "Current version:  $current"
    log_info "Next dev version: $next_version"
    echo ""

    if [ "$DRY_RUN" != "1" ]; then
        validate_git_state
    fi

    write_version "$next_version"
    if [ "$DRY_RUN" = "1" ]; then
        log_info "[DRY_RUN] Would commit: chore: start next cycle: $next_version"
    else
        git add "$VERSION_FILE"
        git commit -m "chore: start next cycle: $next_version"
        log_success "Started $scope version cycle at $next_version"
    fi

    # Bump doesn't create a tag — just push the version commit
    if [ "$DRY_RUN" != "1" ] && [ "${CI:-}" = "true" ]; then
        log_info "Pushing version bump..."
        git push origin HEAD:main
        log_success "Pushed main"
    fi

    echo ""
    log_success "Version bump complete!"
}

# ── Main ──────────────────────────────────────────────────────────────────

usage() {
    cat <<EOF
Usage: $0 <type> [scope]

Types:
  alpha       Create an alpha prerelease
  beta        Create a beta prerelease
  rc          Create a release candidate
  stable      Create a stable release
  bump        Bump minor or major version (requires scope)

Scope (for bump only):
  minor       Bump minor version and start alpha cycle
  major       Bump major version and start alpha cycle

Environment:
  DRY_RUN=1         Preview changes without making them
  RELEASE_TYPE      Alternative to positional type argument
  RELEASE_SCOPE     Alternative to positional scope argument
  CI=true           CI mode: configure git identity and push on success

Examples:
  $0 alpha                       # Create alpha release
  DRY_RUN=1 $0 stable            # Preview stable release
  $0 bump minor                  # Start next minor version cycle
  RELEASE_TYPE=alpha $0          # CI-style invocation
EOF
    exit 1
}

main() {
    [ -n "$RELEASE_TYPE" ] || usage

    if [ "$DRY_RUN" = "1" ]; then
        log_warn "DRY_RUN mode — no changes will be made"
        echo ""
    fi

    # CI setup (only when actually executing)
    if [ "${CI:-}" = "true" ] && [ "$DRY_RUN" != "1" ]; then
        setup_ci
    fi

    case "$RELEASE_TYPE" in
        alpha|beta|rc) release_prerelease "$RELEASE_TYPE" ;;
        stable)        release_stable ;;
        bump)          release_bump "${RELEASE_SCOPE}" ;;
        *)             die "Unknown release type: $RELEASE_TYPE" ;;
    esac
}

main
