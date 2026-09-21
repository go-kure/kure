#!/usr/bin/env bash
# sync-flux-operator-pin.sh — re-vendor the flux-operator install bundle so it
# matches the flux-operator version pinned in go.mod.
#
# pkg/stack/fluxcd embeds the upstream install manifest
# (flux_operator_install.yaml) and names its release in FluxOperatorVersion,
# so the FluxInstance type kure compiles against and the operator it installs
# stay in lockstep. Renovate bumps go.mod but knows nothing about the vendored
# bundle, so before this script every flux-operator bump arrived with the
# bundle and constant still on the old release — a P1 review finding on the
# bump PR each time. This is the mechanical half of that fix; the
# `supported_range` widen stays manual on purpose (it records a compatibility
# assessment, see docs/dependency-updates.md).
#
# Steps:
#   1. Read the flux-operator module path from versions.yaml
#      (`infrastructure.flux-operator.go_module`) and its version from go.mod.
#   2. Download that release's `install.yaml` asset (bounded timeouts) and
#      sanity-check it. Nothing is written until the download validates and
#      every line to be rewritten has been found, so a failed run leaves the
#      tree untouched.
#   3. Replace pkg/stack/fluxcd/flux_operator_install.yaml with it.
#   4. Rewrite the FluxOperatorVersion constant in
#      pkg/stack/fluxcd/flux_operator_install.go.
#   5. Rewrite the "currently **vX.Y.Z**" mention in
#      pkg/stack/fluxcd/README.md.
#
# Each rewrite target must be found exactly once, checked up front — silently
# rewriting nothing is how a lockstep pin drifts.
#
# Idempotent: a re-run at the same go.mod version rewrites nothing. When the
# constant and README already name the go.mod version the script exits before
# the download, so the many Renovate bumps unrelated to flux-operator (this runs
# after every gomod/mise bump) never touch the network.
#
# Invoked as a Renovate postUpgradeTasks command (renovate.json) alongside the
# other sync scripts; safe to run by hand too. Every path written here must
# also be listed in that rule's fileFilters — Renovate drops an unlisted path
# from its commit silently.
#
# Usage: ./scripts/sync-flux-operator-pin.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Overridable so scripts/test/ can run against a fixture tree.
REPO_ROOT="${SYNC_FLUX_OPERATOR_PIN_REPO_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
VERSIONS_FILE="$REPO_ROOT/versions.yaml"
GO_MOD="$REPO_ROOT/go.mod"

DEP="flux-operator"
BUNDLE_DIR="$REPO_ROOT/pkg/stack/fluxcd"
BUNDLE_FILE="$BUNDLE_DIR/flux_operator_install.yaml"
CONST_FILE="$BUNDLE_DIR/flux_operator_install.go"
README_FILE="$BUNDLE_DIR/README.md"

# Required for the script to do its job, so a fail-fast bound rather than a
# best-effort one — but it must be bounded: a stalled connection with no
# deadline would block this Renovate postUpgradeTasks command indefinitely.
CURL_TIMEOUT_ARGS=(--connect-timeout 10 --max-time 60)

for f in "$VERSIONS_FILE" "$GO_MOD" "$BUNDLE_FILE" "$CONST_FILE" "$README_FILE"; do
    if [[ ! -f "$f" ]]; then
        echo "sync-flux-operator-pin: expected file not found: $f" >&2
        exit 1
    fi
done

GO_MODULE="$(yq ".infrastructure.${DEP}.go_module // \"\"" "$VERSIONS_FILE")"
if [[ -z "$GO_MODULE" || "$GO_MODULE" == "null" ]]; then
    echo "sync-flux-operator-pin: versions.yaml has no 'go_module' set for '$DEP'" >&2
    exit 1
fi
# The release asset lives under the GitHub repo, which for this module is its
# path minus the host. Derived rather than declared a second time, so it cannot
# drift from versions.yaml; a module hosted elsewhere fails here, not later as
# a confusing 404.
if [[ "$GO_MODULE" != github.com/* ]]; then
    echo "sync-flux-operator-pin: go_module '${GO_MODULE}' is not a github.com module path" >&2
    exit 1
fi
UPSTREAM_REPO="${GO_MODULE#github.com/}"

# Handles both `require <mod> <ver>` and a `require ( ... )` block line, and
# skips `replace` directives (whose second field is `=>`, not a version).
version="$(awk -v m="$GO_MODULE" '
    $1 == "replace" { next }
    $1 == "require" { $1 = ""; $0 = $0 }
    $1 == m { print $2; exit }
' "$GO_MOD")"
if [[ -z "$version" ]]; then
    echo "sync-flux-operator-pin: go.mod does not require ${GO_MODULE}" >&2
    exit 1
fi
# A release asset exists only for a plain release tag. A pseudo-version or
# pre-release here means someone pinned an untagged commit; refuse rather than
# guess which release the bundle should match.
if ! [[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "sync-flux-operator-pin: go.mod pins ${GO_MODULE} at '${version}', which is not a plain vX.Y.Z release tag" >&2
    exit 1
fi

# Preflight, before any download or write: each line this script rewrites must
# exist exactly once. A count other than one means the file's shape changed and
# a rewrite would be a guess -- and checking here, not per-rewrite, is what
# keeps a missing anchor from failing after the bundle was already replaced.
require_one() {
    local file="$1" pattern="$2" count
    count="$(grep -cE -- "$pattern" "$file" || true)"
    if [[ "$count" != "1" ]]; then
        echo "sync-flux-operator-pin: expected exactly one line matching /${pattern}/ in ${file#"$REPO_ROOT"/}, found ${count}" >&2
        exit 1
    fi
}
require_one "$CONST_FILE" '^const FluxOperatorVersion = "[^"]*"'
require_one "$README_FILE" "\`FluxOperatorVersion\`, currently \\*\\*v[^*]*\\*\\*"

# Renovate runs this after every gomod/mise bump, nearly all of which have
# nothing to do with flux-operator. When the constant and the README already
# name the go.mod version the lockstep is intact, so skip the download: an
# unrelated bump must not depend on github.com being reachable. (Deliberately
# not a content check of the bundle -- a hand-edited bundle at the right
# version is not something a bot run should silently overwrite.)
current_const="$(sed -nE 's/^const FluxOperatorVersion = "([^"]*)".*/\1/p' "$CONST_FILE")"
# The backticks are literal markdown in the README, not a command substitution.
# shellcheck disable=SC2016
current_readme="$(sed -nE 's/.*`FluxOperatorVersion`, currently \*\*(v[^*]*)\*\*.*/\1/p' "$README_FILE")"
if [[ "$current_const" == "$version" && "$current_readme" == "$version" ]]; then
    echo "sync-flux-operator-pin: constant and README already ${version} -- no change"
    echo "sync-flux-operator-pin: done"
    exit 0
fi

url="https://github.com/${UPSTREAM_REPO}/releases/download/${version}/install.yaml"
echo "sync-flux-operator-pin: ${GO_MODULE} ${version} -- downloading ${url}"

tmp="$(mktemp)"
# The .new files are rewrite()'s scratch copies; a failed sed must not leave one
# behind in the source tree.
trap 'rm -f "$tmp" "$CONST_FILE.new" "$README_FILE.new"' EXIT

if ! curl -fsSL "${CURL_TIMEOUT_ARGS[@]}" -o "$tmp" "$url"; then
    echo "sync-flux-operator-pin: could not download ${url}" >&2
    exit 1
fi
if [[ ! -s "$tmp" ]]; then
    echo "sync-flux-operator-pin: ${url} downloaded empty" >&2
    exit 1
fi
# Cheap shape check: an HTML error page or a truncated body must not replace
# the bundle. TestFluxOperatorInstallObjects checks the full inventory.
for kind in CustomResourceDefinition Deployment; do
    if ! grep -q "^kind: ${kind}\$" "$tmp"; then
        echo "sync-flux-operator-pin: ${url} has no '${kind}' document -- not an install bundle" >&2
        exit 1
    fi
done

# rewrite FILE SED_EXPR LABEL -- applies SED_EXPR and writes only when the
# result differs. The anchors were already proven present exactly once by the
# preflight above, so this cannot half-apply.
rewrite() {
    local file="$1" expr="$2" label="$3"
    sed -E "$expr" "$file" > "$file.new"
    if cmp -s "$file" "$file.new"; then
        rm -f "$file.new"
        echo "sync-flux-operator-pin: ${label} already ${version} -- no change"
    else
        mv "$file.new" "$file"
        echo "sync-flux-operator-pin: ${label} -> ${version}"
    fi
}

if cmp -s "$tmp" "$BUNDLE_FILE"; then
    echo "sync-flux-operator-pin: ${BUNDLE_FILE#"$REPO_ROOT"/} already matches ${version} -- no change"
else
    cp "$tmp" "$BUNDLE_FILE"
    echo "sync-flux-operator-pin: ${BUNDLE_FILE#"$REPO_ROOT"/} -> ${version}"
fi

rewrite "$CONST_FILE" \
    "s/^(const FluxOperatorVersion = )\"[^\"]*\"/\\1\"${version}\"/" \
    "FluxOperatorVersion"

rewrite "$README_FILE" \
    "s/(\`FluxOperatorVersion\`, currently \\*\\*)v[^*]*(\\*\\*)/\\1${version}\\2/" \
    "pkg/stack/fluxcd/README.md mention"

echo "sync-flux-operator-pin: done"
