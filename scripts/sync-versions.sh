#!/bin/bash
# sync-versions.sh - Validate and manage version consistency
#
# Usage:
#   ./scripts/sync-versions.sh check      - Validate consistency
#   ./scripts/sync-versions.sh generate   - Generate docs and the pkg/versions Go API from versions.yaml
#
# This script ensures that:
# 1. each go.mod dependency version falls WITHIN versions.yaml "supported_range"
#    (the build version is read from go.mod; there is no "current" field to sync).
#    Skipped for an entry declaring floor_module: its version is Go's
#    minimum-version-selection floor, not a choice kure's supported_range can
#    gate -- see item 6 and validate_mvs_floors.
# 2. Documentation is generated from versions.yaml + go.mod
# 3. go.mod's "// Current pin: ..." comment matches its k8s.io/api replace directive
# 4. versions.yaml notes never carry a raw commit SHA (must reference a vendor-guard-checked
#    ref: pin instead, see scripts/vendor-guard.sh)
# 5. pkg/versions/versions_gen.go (the published Go API's generated data) matches
#    versions.yaml -- see generate_go_api / validate_go_api_drift
# 6. a versions.yaml entry declaring floor_module has its go.mod pin exactly equal
#    to what floor_module's own go.mod currently requires -- see validate_mvs_floors
#
# Test-only overrides (used by scripts/test/, never set in a real invocation):
#   SYNC_VERSIONS_REPO_ROOT     - point REPO_ROOT at a synthetic fixture tree instead of the
#                                 checkout this script lives in. Setting this in a real
#                                 invocation makes the script validate a different tree than
#                                 the one it lives in -- do not set it outside the test harness.
#   SYNC_VERSIONS_PROBE_TIMEOUT - override the 10s timeout on the validate_mvs_floors `go list`
#                                 probe, so the harness's timeout case runs in ~1s instead of 10s.
#   SYNC_VERSIONS_TIMEOUT_CMD   - override the resolved `timeout`/`gtimeout` binary name (even to
#                                 "" for none), so the harness can exercise the unbounded fallback
#                                 without making a real `timeout` unresolvable on PATH.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${SYNC_VERSIONS_REPO_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
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

# Resolved once, from the start of the `check)` branch in main() -- not from
# check_dependencies, and not for `generate` (see below). Empty means "no
# bounded-execution binary available" -- run_bounded then runs the command
# unbounded.
TIMEOUT_BIN=""

resolve_timeout_bin() {
    # Test-only override: set (even to "") it wins, so the harness can exercise
    # the unbounded fallback without making `timeout` unresolvable on PATH --
    # a PATH truncation would also drop yq and trip check_dependencies first.
    if [[ -n "${SYNC_VERSIONS_TIMEOUT_CMD+set}" ]]; then
        TIMEOUT_BIN="$SYNC_VERSIONS_TIMEOUT_CMD"
    else
        local candidate
        for candidate in timeout gtimeout; do
            if command -v "$candidate" >/dev/null 2>&1; then
                TIMEOUT_BIN="$candidate"
                break
            fi
        done
    fi
    # Empty either way it got here means run_bounded runs unbounded -- warn
    # once regardless of which branch produced it (a warning inside the
    # override branch only would never fire for case 35's SYNC_VERSIONS_TIMEOUT_CMD="").
    if [[ -z "$TIMEOUT_BIN" ]]; then
        warning "no 'timeout' (or 'gtimeout') on PATH -- the MVS-floor go.mod probe and the git ls-remote tag lookup will run unbounded, so a stalled module proxy or git remote can hang this script. Install GNU coreutils to restore the bound."
    fi
}

# run_bounded <seconds> <command...> -- bound the command when a timeout
# binary exists, otherwise run it as-is. Keeps the caller's exit status and
# stdout unchanged in both cases, so every rc test downstream is unaffected.
run_bounded() {
    local seconds="$1"
    shift
    if [[ -n "$TIMEOUT_BIN" ]]; then
        "$TIMEOUT_BIN" "$seconds" "$@"
    else
        "$@"
    fi
}

# make_temp <what> [mktemp-args...] -- echo a usable temp path or fail loudly.
# Callers must use `|| return 1`: an unchecked empty path turns into a
# redirect to an empty filename (bash: "line N: : No such file or directory",
# not an "ambiguous redirect" -- that specific message needs an unquoted
# multi-/zero-word expansion, which "$DOCS_FILE"/"$GO_API_FILE" never are)
# or a `rm -f ''` RETURN trap instead of an explanation.
make_temp() {
    local what="$1"
    shift
    local path
    # ${1+"$@"}, not a bare "$@": with zero mktemp-args (every call site here
    # except generate_go_api's), "$@" has zero positional params -- referencing
    # it under `set -u` is only safe on bash >= 4.4 (earlier bash raises
    # "$@: unbound variable"). ${1+"$@"} sidesteps that entirely: it expands to
    # nothing when there are no positional params, to "$@" unchanged otherwise.
    path=$(mktemp ${1+"$@"}) || {
        error "$what: mktemp failed -- either the mktemp binary is missing, or it could not create a file (check \$TMPDIR and free space)"
        return 1
    }
    # -f, not -e: every call site here (see above) has mktemp create a plain
    # file, never a directory -- a future caller passing mktemp's `-d` would
    # need its own review of the downstream cat/mv usage, not a silent pass
    # through this check.
    if [[ -z "$path" || ! -f "$path" ]]; then
        error "$what: mktemp produced an unusable path ('$path')"
        return 1
    fi
    printf '%s' "$path"
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
        ls_remote_out="$(run_bounded 5 git ls-remote "https://github.com/${repo}" "refs/tags/${tag}" "refs/tags/${tag}^{}" 2>/dev/null)"
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

# Strip leading zeros from a decimal digit string, keeping at least one
# digit ("007" -> "7", "0" -> "0").
strip_leading_zeros() {
    local s="$1"
    while [[ ${#s} -gt 1 && "${s:0:1}" == "0" ]]; do
        s="${s:1}"
    done
    printf '%s' "$s"
}

# Compare two non-negative decimal integer strings without bash arithmetic
# ($(( )) is a 64-bit signed integer -- a version component with enough
# digits, e.g. a 20-digit major, silently wraps). Strips leading zeros, then
# compares digit-string length (longer wins) and falls back to lexicographic
# order for equal lengths, which matches numeric order once leading zeros
# are gone. Echoes -1, 0, or 1.
numcmp() {
    local a b
    a=$(strip_leading_zeros "$1")
    b=$(strip_leading_zeros "$2")
    if [[ ${#a} -ne ${#b} ]]; then
        [[ ${#a} -lt ${#b} ]] && { echo -1; return; }
        echo 1
        return
    fi
    if [[ "$a" == "$b" ]]; then
        echo 0
    elif [[ "$a" < "$b" ]]; then
        echo -1
    else
        echo 1
    fi
}

# Compare two "major.minor" strings numerically, major first then minor, via
# numcmp (see above -- no fixed-multiplier key, no bash-arithmetic overflow).
# Echoes -1, 0, or 1.
mm_cmp() {
    local a="$1" b="$2"
    local a_major="${a%%.*}" a_minor="${a#*.}"
    a_minor="${a_minor%%.*}"
    local b_major="${b%%.*}" b_minor="${b#*.}"
    b_minor="${b_minor%%.*}"
    local major_cmp
    major_cmp=$(numcmp "$a_major" "$b_major")
    if [[ "$major_cmp" != 0 ]]; then
        echo "$major_cmp"
        return
    fi
    numcmp "$a_minor" "$b_minor"
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
    tmp=$(make_temp "sync_gomod_pin_comment") || return 1
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

# Validate that each versions.yaml entry declaring a floor_module has its
# go.mod pin exactly equal to what floor_module's own go.mod currently
# requires for go_module. Some dependencies (barman-cloud) are never chosen
# directly: Go's minimum-version selection floors the pin above any tag that
# exists, from a *different* module's requirement, so the range check in
# validate_gomod() cannot catch a pin that has drifted off that floor onto
# some other v0.5.x pseudo-version the floor module does not itself require.
#
# GOWORK=off: this repo is normally checked out inside a multi-module
# workspace (go.work listing sibling modules on a newer Go toolchain); every
# `go` invocation here must ignore that workspace or it fails outright.
# -mod=readonly: this guard only ever reads go.mod state, never rewrites it.
# -C "$REPO_ROOT": `go list -m` resolves against the module of the *current
# directory*, not any path baked into the command -- every other file this
# script touches goes through $REPO_ROOT, so this probe must too, or a caller
# that invokes the script by absolute path from outside the checkout has no
# enclosing go.mod, silently degrading to the warning branch below instead of
# actually running the check.
# run_bounded ... 10: this is the one external-resolution call in this
# function, and a stalled module-proxy connection would otherwise hang here
# indefinitely instead of reaching the warning branch promptly -- same
# discipline as resolve_tag_commit()'s `run_bounded 5 git ls-remote` /
# `curl --max-time 5`. When no `timeout`/`gtimeout` binary is on PATH,
# run_bounded degrades to running the command unbounded (resolve_timeout_bin
# warns once at startup); this probe can then hang on a stalled proxy.
#
# Three-way outcome, same discipline as resolve_tag_commit(): success,
# definitive error, or (only for the "can we even ask the question" step)
# a warning -- never a hard failure purely because Go or the module cache
# was unavailable. In CI, `make deps` runs before this check
# (.github/workflows/ci.yml) and always populates the module cache, so the
# warning path there can never mask a real mismatch.
validate_mvs_floors() {
    local errors=0
    info ""
    info "Validating MVS-floor dependencies..."

    local deps
    deps=$(yq '.infrastructure | keys | .[]' "$VERSIONS_FILE")

    while IFS= read -r dep; do
        local go_module floor_module
        go_module=$(yq ".infrastructure.${dep}.go_module" "$VERSIONS_FILE")
        floor_module=$(yq ".infrastructure.${dep}.floor_module // \"\"" "$VERSIONS_FILE")

        if [[ -z "$floor_module" || "$floor_module" == "null" ]]; then
            continue
        fi

        # Step 1: the pin itself. Do NOT strip the leading "v" -- this
        # comparison is exact-string against the floor module's own
        # requirement, unlike validate_gomod()'s major.minor range check.
        local actual
        actual=$(get_gomod_version "$go_module")
        if [[ -z "$actual" ]]; then
            error "$dep: floor_module is set but $go_module was not found in go.mod"
            errors=$((errors + 1))
            continue
        fi

        # Step 2: the floor module itself must be a go.mod dependency.
        local floor_pin
        floor_pin=$(get_gomod_version "$floor_module")
        if [[ -z "$floor_pin" ]]; then
            error "$dep: floor_module $floor_module (claimed to set the floor for $go_module) was not found in go.mod"
            errors=$((errors + 1))
            continue
        fi

        # Step 3: locate the floor module's own go.mod so we can read what
        # IT requires for $go_module. Unreachable (no go, cold cache, no
        # network) degrades to a warning -- this is the "can we even ask"
        # step, not the comparison itself.
        local gomod_path gomod_rc=0
        gomod_path=$(run_bounded "${SYNC_VERSIONS_PROBE_TIMEOUT:-10}" env GOWORK=off go list -C "$REPO_ROOT" -mod=readonly -m -f '{{.GoMod}}' "$floor_module" 2>/dev/null) || gomod_rc=$?
        if [[ $gomod_rc -ne 0 || -z "$gomod_path" || ! -f "$gomod_path" ]]; then
            warning "$dep: could not resolve $floor_module's own go.mod (no Go, cold module cache, or no network) -- skipping MVS-floor equality check"
            continue
        fi

        # Step 4: extract what the floor module's go.mod requires for
        # $go_module. Double-quoted filter: $go_module must be interpolated
        # by bash here, the same idiom used for ${dep} in
        # validate_no_sha_in_notes()/generate_docs() -- a single-quoted
        # filter would embed the literal text "$go_module" instead of its
        # value and never match, always falling into the empty/error case
        # below regardless of the true state.
        # Guard the assignment explicitly: under `set -euo pipefail` an
        # unguarded `x=$(...)` still aborts the whole script on a non-zero
        # pipeline exit (go mod edit or yq failing), instead of falling
        # into the empty/error branch below the way step 3's `go list`
        # does via `|| gomod_rc=$?`. Capture the pipe's own status too, so
        # the two ways `required` can end up empty -- the requirement
        # genuinely absent vs. `go mod edit`/`yq` itself failing to read
        # $gomod_path -- get distinct, accurate error text instead of the
        # tool-failure case being misreported as a confirmed absence.
        local required required_pipe_rc=0
        required=$(GOWORK=off go mod edit -json "$gomod_path" | yq -p json ".Require[]? | select(.Path == \"$go_module\") | .Version" 2>/dev/null) || required_pipe_rc=$?
        if [[ -z "$required" || "$required" == "null" ]]; then
            if [[ $required_pipe_rc -ne 0 ]]; then
                error "$dep: could not read $floor_module's go.mod requirements (go mod edit or yq failed against $gomod_path, rc=$required_pipe_rc) -- cannot confirm the floor_module claim in versions.yaml"
            else
                error "$dep: $floor_module's go.mod does not require $go_module at all -- the floor_module claim in versions.yaml does not hold. Point floor_module at the module that actually raises the pin, or drop it."
            fi
            errors=$((errors + 1))
            continue
        fi

        # Step 5: exact-string equality.
        if [[ "$required" == "$actual" ]]; then
            success "$dep: pin $actual matches the floor set by $floor_module"
        else
            error "$dep: pin $actual does not match $floor_module's requirement $required. Either $floor_module moved (re-derive with: go mod edit -droprequire=$go_module && go mod tidy), or some other dependency now raises the pin above the floor and the floor_module claim in versions.yaml is stale."
            errors=$((errors + 1))
        fi
    done <<< "$deps"

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
        local go_module supported basis floor_module
        go_module=$(yq ".infrastructure.${dep}.go_module" "$VERSIONS_FILE")
        supported=$(yq ".infrastructure.${dep}.supported_range" "$VERSIONS_FILE")
        basis=$(yq ".infrastructure.${dep}.version_basis // \"semver\"" "$VERSIONS_FILE")
        floor_module=$(yq ".infrastructure.${dep}.floor_module // \"\"" "$VERSIONS_FILE")

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

        # A floor_module entry's version is not kure's choice -- it is Go's
        # minimum-version-selection floor set by floor_module's own go.mod,
        # and validate_mvs_floors already asserts the pin equals that floor
        # exactly. Range-checking the same version against a hand-maintained
        # supported_range here can only ever demand a forced-yes edit:
        # refusing to widen means refusing the bump that raised the floor.
        # The range check was never a deliberate gate for these entries --
        # it applied only as a side effect of this loop iterating every
        # infrastructure key (go-kure/kure#703). See docs/dependency-updates.md's
        # "MVS-floor dependencies" section.
        if [[ -n "$floor_module" && "$floor_module" != "null" ]]; then
            success "$dep: range check not applicable (pin v$actual_version derived from $floor_module's own go.mod, see validate_mvs_floors)"
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

        local ver_mm
        ver_mm=$(version_mm "$actual_version" "$basis")

        if [[ "$(mm_cmp "$ver_mm" "$hi_mm")" == 1 ]]; then
            # Above the upper bound is exactly what `widen` raises -- the
            # command it prints will succeed once the note is filled in.
            error "$dep $ver_mm (go.mod $go_module v$actual_version) is outside supported_range \"$supported\". After confirming API compatibility: ./scripts/sync-versions.sh widen $dep $ver_mm --note \"<compatibility assessment>\""
            errors=$((errors + 1))
        elif [[ "$(mm_cmp "$ver_mm" "$lo_mm")" == -1 ]]; then
            # Below the lower bound: `widen` only ever raises the upper
            # bound and would refuse this value outright (it's not above
            # the current one) -- printing that command here would just
            # hand the human a command guaranteed to fail. This direction
            # means the pin moved backward for some other reason; there is
            # no mechanical remediation to suggest.
            error "$dep $ver_mm (go.mod $go_module v$actual_version) is below supported_range \"$supported\"'s lower bound -- this is not something 'widen' can fix (it only raises the upper bound). Confirm why the pin moved backward, then update supported_range and notes by hand."
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
        local floor_module
        floor_module=$(yq ".infrastructure.${dep}.floor_module // \"\"" "$VERSIONS_FILE")
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

        # A floor_module entry's version is not a hand-maintained range --
        # see validate_gomod's matching skip and docs/dependency-updates.md's
        # "MVS-floor dependencies" section. Render that instead of whatever
        # (or nothing) supported_range happens to hold.
        local compat_cell="$supported"
        if [[ -n "$floor_module" && "$floor_module" != "null" ]]; then
            compat_cell="derived from $floor_module"
        fi

        echo "| $dep | $build_version | $compat_cell | $notes |" >> "$DOCS_FILE"
    done <<< "$deps"

    cat >> "$DOCS_FILE" << 'EOF'

## Understanding the Matrix

### Build Version (go.mod)
The version Kure imports and builds against — read directly from `go.mod`, the single
source of truth for the pin. CI (`sync-versions.sh check`) asserts it falls within the
declared `supported_range` — except an entry whose Deployment Compatibility cell reads
"derived from `<module>`" (an MVS-floor dependency): its pin is not chosen by Kure, so
there is no independent range to assert, and CI instead only checks that the pin matches
what `<module>`'s own go.mod requires. See `docs/dependency-updates.md`'s "MVS-floor
dependencies" section.

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

    # go.current is kept in sync with mise.toml by scripts/sync-go-version.sh,
    # including on Renovate's own branches via postUpgradeTasks -- it is
    # bot-written text, not purely hand-maintained, so it is guarded the same
    # as the per-dependency fields below rather than trusted silently: it is
    # emitted by a raw %s outside the loop's own guard.
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
    tmp=$(make_temp "generate_go_api" "$(dirname "$out")/.versions_gen.XXXXXX") || return 1

    {
        printf '// Code generated by scripts/sync-versions.sh from versions.yaml. DO NOT EDIT.\n\n'
        printf 'package versions\n\n'
        printf '// GoVersion is the go.current field of versions.yaml.\n'
        printf 'const GoVersion string = "%s" // doc-gate:trivial\n\n' "$go_current"
        printf 'var infrastructure = []Dependency{\n'
    } > "$tmp"

    local deps
    deps=$(yq '.infrastructure | keys | .[]' "$VERSIONS_FILE")

    while IFS= read -r dep; do
        local go_module supported basis lo hi field floor_module
        go_module=$(yq ".infrastructure.${dep}.go_module // \"\"" "$VERSIONS_FILE")
        supported=$(yq ".infrastructure.${dep}.supported_range // \"\"" "$VERSIONS_FILE")
        basis=$(yq ".infrastructure.${dep}.version_basis // \"semver\"" "$VERSIONS_FILE")
        floor_module=$(yq ".infrastructure.${dep}.floor_module // \"\"" "$VERSIONS_FILE")

        # Mirror validate_gomod's skip and generate_docs' "derived from"
        # rendering: a floor_module entry's range is never enforced (see
        # validate_gomod), so the published API must not expose one either
        # -- a stray supported_range left on such an entry would otherwise
        # leak an unenforced-looking bound to every pkg/versions consumer.
        if [[ -n "$floor_module" && "$floor_module" != "null" ]]; then
            supported=""
        else
            # Normalize yq's "no key" result to "" so the unsafe-literal
            # check and the printf below don't need their own null case.
            floor_module=""
        fi

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
        for field in "$dep" "$go_module" "$supported" "$basis" "$floor_module"; do
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
            printf '\t\t%-15s "%s",\n' 'FloorModule:' "$floor_module"
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

    local expected cleanup_cmd
    expected=$(make_temp "validate_docs_drift") || return 1
    # %q-quote both paths before building the trap string: a literal single
    # quote anywhere in $expected (e.g. from a TMPDIR containing one) would
    # otherwise break out of the '...' quoting below and either error at trap
    # time or silently skip the rm -f, leaking both temp files -- reproduced
    # directly against TMPDIR="/tmp/quote'dir" before this fix.
    printf -v cleanup_cmd 'rm -f %q %q' "$expected" "$expected.diff"
    # shellcheck disable=SC2064  # expand $expected now (already %q-quoted), not at trap time
    trap "$cleanup_cmd" RETURN

    generate_docs "$expected" >/dev/null || return 1

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

    local expected cleanup_cmd
    expected=$(make_temp "validate_go_api_drift") || return 1
    # See validate_docs_drift's identical comment: %q-quote before building
    # the trap string so a literal single quote in $expected cannot break it.
    printf -v cleanup_cmd 'rm -f %q %q' "$expected" "$expected.diff"
    # shellcheck disable=SC2064  # expand $expected now (already %q-quoted), not at trap time
    trap "$cleanup_cmd" RETURN

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

# Widen a dependency's supported_range upper bound and replace its notes,
# then regenerate. The judgment (is the new version actually compatible?)
# stays entirely human -- this only automates the yq edit and the two-file
# regeneration that validate_gomod's range-check error otherwise leaves as a
# fully manual, three-artifact chore every time. See docs/dependency-updates.md's
# "Widening a supported_range" section.
# $1: dependency key (an .infrastructure entry). $2: new upper bound
# (major.minor). $3: replacement notes text.
widen_dependency() {
    local dep="$1" new_hi="$2" note="$3"

    local exists
    exists=$(yq ".infrastructure | has(\"${dep}\")" "$VERSIONS_FILE")
    if [[ "$exists" != "true" ]]; then
        error "widen: no such dependency '$dep' in versions.yaml's .infrastructure"
        return 1
    fi

    # A floor_module entry's range is never enforced (validate_gomod skips
    # it, see the guard above) -- nothing for widen to usefully change.
    local floor_module
    floor_module=$(yq ".infrastructure.${dep}.floor_module // \"\"" "$VERSIONS_FILE")
    if [[ -n "$floor_module" && "$floor_module" != "null" ]]; then
        error "widen: $dep declares floor_module ($floor_module) -- its range is never range-checked and there is nothing to widen. See docs/dependency-updates.md's \"MVS-floor dependencies\" section."
        return 1
    fi

    if go_string_literal_unsafe "$note"; then
        error "widen: --note text cannot be emitted as a YAML/Go string literal (contains a double quote, backslash, or newline): $note"
        return 1
    fi

    # notes is rendered raw into a Markdown table cell by generate_docs
    # (a '|' would open an extra column) -- go_string_literal_unsafe above
    # only guards Go-string-literal safety, a different downstream consumer,
    # so this needs its own check.
    if [[ "$note" == *'|'* ]]; then
        error "widen: --note text cannot contain '|' -- it is rendered as a Markdown table cell in docs/compatibility.md and a pipe would break the table: $note"
        return 1
    fi

    # validate_no_sha_in_notes (the check guard) rejects a bare commit SHA in
    # any notes: block -- mirror that here so widen refuses up front instead
    # of writing a note that only fails later, at the next `check`.
    local sha_hit
    sha_hit=$(printf '%s' "$note" | grep -oiE '\b[0-9a-f]{40}\b|\b[0-9a-f]{12}\b' | head -n1 || true)
    if [[ -n "$sha_hit" ]]; then
        error "widen: --note text contains a raw commit SHA ($sha_hit) -- reference the pseudo-version pinned in go.mod instead, not the literal commit: $note"
        return 1
    fi

    # $2 must be exactly major.minor: mm_cmp's parsing silently truncates
    # anything past the second component (so "2.1.0" and "2.1" compare
    # equal), which would otherwise let a malformed bound through the
    # comparison below and get written verbatim into supported_range.
    if [[ ! "$new_hi" =~ ^[0-9]+\.[0-9]+$ ]]; then
        error "widen: new upper bound \"$new_hi\" is not major.minor (e.g. \"1.26\") -- supported_range only ever records major.minor"
        return 1
    fi

    local supported
    supported=$(yq ".infrastructure.${dep}.supported_range // \"\"" "$VERSIONS_FILE")
    if [[ -z "$supported" || "$supported" == "null" ]]; then
        error "widen: $dep has no supported_range declared -- add one by hand first; widen only raises an existing bound"
        return 1
    fi

    local lo hi
    if [[ "$supported" == *" - "* ]]; then
        lo="${supported%% - *}"; hi="${supported##* - }"
    else
        lo="$supported"; hi="$supported"
    fi

    # Compare against the current UPPER bound, not the lower one: for an
    # existing multi-version range (e.g. "1.5 - 1.7"), a new_hi that is
    # above the lower bound but at or below the current upper bound (e.g.
    # "1.6") would otherwise silently narrow the range and drop support for
    # 1.7 while still being reported as "widened".
    if [[ "$(mm_cmp "$new_hi" "$hi")" != 1 ]]; then
        error "widen: new upper bound \"$new_hi\" is not above the current upper bound \"$hi\" -- widen only raises the upper bound, never narrows the range"
        return 1
    fi

    local new_range="$lo - $new_hi"

    # widen_dependency is always called as the left side of `||` in main()
    # (`widen_dependency ... || exit 1`), which disables `set -e` for this
    # entire function -- a failing yq write here would otherwise go
    # unnoticed and this function would still report success. Check both
    # explicitly rather than relying on errexit.
    if ! yq eval -i ".infrastructure.${dep}.supported_range = \"${new_range}\"" "$VERSIONS_FILE"; then
        error "widen: failed to write supported_range for $dep -- versions.yaml was not changed"
        return 1
    fi
    if ! yq eval -i ".infrastructure.${dep}.notes = \"${note}\"" "$VERSIONS_FILE"; then
        error "widen: failed to write notes for $dep -- supported_range was already widened to \"$new_range\" but notes was not; versions.yaml is now partially edited, fix notes by hand or revert"
        return 1
    fi

    success "$dep: supported_range widened to \"$new_range\"; notes replaced"
    info "yq may have reformatted the notes block onto one line -- reflow it to a '|' block manually if you want the usual multi-line prose style"
    info "Next: ./scripts/sync-versions.sh generate"
}

# Main command router
main() {
    local command="${1:-check}"

    check_dependencies

    case "$command" in
        check)
            resolve_timeout_bin
            info "=== Version Consistency Check ==="
            info ""
            local gomod_result=0
            validate_gomod || gomod_result=$?
            validate_gomod_pin_comment || gomod_result=1
            validate_no_sha_in_notes || gomod_result=1
            validate_mvs_floors || gomod_result=1
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
        widen)
            local dep="${2:-}" new_hi="${3:-}" note=""
            shift 3 2>/dev/null || true
            while [[ $# -gt 0 ]]; do
                case "$1" in
                    --note)
                        # A bare trailing --note leaves only 1 positional
                        # param; `shift 2` then fails outright under set -e
                        # (no usage message, just an abrupt exit) instead of
                        # falling through to the dep/new_hi/note emptiness
                        # check below.
                        if [[ $# -lt 2 ]]; then
                            echo "Usage: $0 widen <dep> <new-upper-bound> --note \"<compatibility assessment>\""
                            exit 1
                        fi
                        note="$2"
                        shift 2
                        ;;
                    *)
                        error "widen: unknown argument: $1"
                        echo "Usage: $0 widen <dep> <new-upper-bound> --note \"<compatibility assessment>\""
                        exit 1
                        ;;
                esac
            done
            if [[ -z "$dep" || -z "$new_hi" || -z "$note" ]]; then
                echo "Usage: $0 widen <dep> <new-upper-bound> --note \"<compatibility assessment>\""
                exit 1
            fi
            widen_dependency "$dep" "$new_hi" "$note" || exit 1
            exit 0
            ;;
        *)
            error "Unknown command: $command"
            echo "Usage: $0 {check|generate|widen}"
            exit 1
            ;;
    esac
}

main "$@"
