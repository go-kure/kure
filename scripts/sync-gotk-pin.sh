#!/usr/bin/env bash
# sync-gotk-pin.sh — re-vendor the flux2 toolkit install bundle so it matches
# the flux2 version pinned in go.mod.
#
# internal/gotk embeds the toolkit's install manifests (manifests.tar.gz, the
# asset of one flux2 release) and names its release in Version, so the
# controllers `gotk` bootstrap mode installs and the API types kure compiles
# against stay in lockstep. Renovate bumps go.mod but knows nothing about the
# vendored bundle, so before this script every flux2 bump arrived with the
# archive and constant on the old release and `test` red on
# TestVendoredPinsMatchGoMod (go-kure/kure#1365). The `supported_range` widen
# stays manual on purpose: it records a compatibility assessment.
#
# Steps:
#   1. Read the flux2 module path from versions.yaml
#      (`infrastructure.fluxcd.go_module`) and its version from go.mod.
#   2. Download that release's `manifests.tar.gz` asset (bounded timeouts) and
#      check its shape. Nothing is written until the download validates and
#      every line to be rewritten has been found, so a failed run leaves the
#      tree untouched.
#   3. Replace internal/gotk/manifests.tar.gz with it.
#   4. Rewrite the Version constant in internal/gotk/gotk.go.
#   5. Rewrite the "currently **vX.Y.Z**" mention in
#      pkg/stack/fluxcd/docs/bootstrap-generation.md.
#   6. Regenerate the stored bootstrap outputs under
#      pkg/stack/fluxcd/testdata/bootstrap-output with the tests' -update flag.
#      The ten gotk-*.txt files summarise the bundle in their first line (its
#      sha256), and the flux-operator-*.txt files record the toolkit version in
#      their `    version:` line (the tests pass GotkVersion as the FluxVersion).
#      The result is kept only when those are all that changed, with the object
#      count of each gotk file unchanged. Anything wider is a rendered-bootstrap
#      change for a person: the stored files are put back, the script fails and
#      the PR stays red on `test`.
#
# Each rewrite target must be found exactly once, checked up front. The old
# version is read from the constant before anything is staged, and no step
# assumes the new version is the higher one.
#
# Idempotent: a re-run at the same go.mod version rewrites nothing. When the
# constant and the bootstrap page already name the go.mod version the script
# exits before the download, so the many Renovate bumps unrelated to flux2 (this
# runs after every gomod/mise bump) never touch the network.
#
# Invoked as a Renovate postUpgradeTasks command (renovate.json), before
# sync-flux-operator-pin.sh; safe to run by hand too. Every path written here
# must also be listed in that rule's fileFilters: Renovate drops an unlisted
# path from its commit silently.
#
# Usage: ./scripts/sync-gotk-pin.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Overridable so scripts/test/ can run against a fixture tree.
REPO_ROOT="${SYNC_GOTK_PIN_REPO_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
VERSIONS_FILE="$REPO_ROOT/versions.yaml"
GO_MOD="$REPO_ROOT/go.mod"

DEP="fluxcd"
ARCHIVE="$REPO_ROOT/internal/gotk/manifests.tar.gz"
CONST_FILE="$REPO_ROOT/internal/gotk/gotk.go"
README_FILE="$REPO_ROOT/pkg/stack/fluxcd/docs/bootstrap-generation.md"
GOLDEN_DIR="$REPO_ROOT/pkg/stack/fluxcd/testdata/bootstrap-output"
# The tests that write the files in GOLDEN_DIR (compareWithStored with -update),
# the same list as sync-flux-operator-pin.sh (a test compares the two). The names
# build the -run regexp, so step 6 can check that each one ran and passed: a
# regexp that matches no test, or a test that skips, makes go exit 0 and write
# nothing.
GOLDEN_TEST_NAMES=(
    TestBootstrap_OutputUnchanged
    TestBootstrapSourceAccess_StoredOutput
    TestBootstrapSourceSemVer_StoredOutput
    TestBootstrapSourceVerify_StoredOutput
)
GOLDEN_TESTS="^($(IFS='|'; echo "${GOLDEN_TEST_NAMES[*]}"))\$"

# Required for the script to do its job, so a fail-fast bound rather than a
# best-effort one — but it must be bounded: a stalled connection with no
# deadline would block this Renovate postUpgradeTasks command indefinitely.
CURL_TIMEOUT_ARGS=(--connect-timeout 10 --max-time 60)

for f in "$VERSIONS_FILE" "$GO_MOD" "$ARCHIVE" "$CONST_FILE" "$README_FILE"; do
    if [[ ! -f "$f" ]]; then
        echo "sync-gotk-pin: expected file not found: $f" >&2
        exit 1
    fi
done
for kind in gotk flux-operator; do
    if ! compgen -G "$GOLDEN_DIR/$kind-*.txt" >/dev/null; then
        echo "sync-gotk-pin: no stored bootstrap output found: $GOLDEN_DIR/$kind-*.txt" >&2
        exit 1
    fi
done

GO_MODULE="$(yq ".infrastructure.${DEP}.go_module // \"\"" "$VERSIONS_FILE")"
if [[ -z "$GO_MODULE" || "$GO_MODULE" == "null" ]]; then
    echo "sync-gotk-pin: versions.yaml has no 'go_module' set for '$DEP'" >&2
    exit 1
fi
# The release asset lives under the GitHub repo, which for this module is its
# path minus the host and minus the major-version suffix (github.com/fluxcd/
# flux2/v2 is the repository fluxcd/flux2). Derived rather than declared a
# second time, so it cannot drift from versions.yaml; a module hosted elsewhere
# fails here, not later as a confusing 404.
if [[ "$GO_MODULE" != github.com/* ]]; then
    echo "sync-gotk-pin: go_module '${GO_MODULE}' is not a github.com module path" >&2
    exit 1
fi
UPSTREAM_REPO="${GO_MODULE#github.com/}"
UPSTREAM_REPO="${UPSTREAM_REPO%/v[0-9]*}"

# Only `require` directives count: the single-line form and lines inside a
# `require ( ... )` block. A line naming the module inside an `exclude`,
# `replace` or `retract` block is not the pin and must not be mistaken for it.
version="$(awk -v m="$GO_MODULE" '
    $1 == "require" && $2 == "(" { inreq = 1; next }
    inreq && $1 == ")"           { inreq = 0; next }
    $1 == "require"              { if ($2 == m) { print $3; exit } next }
    inreq && $1 == m             { print $2; exit }
' "$GO_MOD")"
if [[ -z "$version" ]]; then
    echo "sync-gotk-pin: go.mod does not require ${GO_MODULE}" >&2
    exit 1
fi
# A release asset exists only for a plain release tag. A pseudo-version or
# pre-release here means someone pinned an untagged commit; refuse rather than
# guess which release the bundle should match.
if ! [[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "sync-gotk-pin: go.mod pins ${GO_MODULE} at '${version}', which is not a plain vX.Y.Z release tag" >&2
    exit 1
fi

# Preflight, before any download or write: each line this script rewrites must
# exist exactly once. A count other than one means the file's shape changed and
# a rewrite would be a guess.
require_one() {
    local file="$1" pattern="$2" count
    # Occurrences, not lines: two mentions on one line would count as one and
    # the substitution would then update only the first.
    count="$({ grep -oE -- "$pattern" "$file" || true; } | wc -l | tr -d ' ')"
    if [[ "$count" != "1" ]]; then
        echo "sync-gotk-pin: expected exactly one line matching /${pattern}/ in ${file#"$REPO_ROOT"/}, found ${count}" >&2
        exit 1
    fi
}
require_one "$CONST_FILE" '^const Version = "[^"]*"'
require_one "$README_FILE" "\`GotkVersion\`, currently \\*\\*v[^*]*\\*\\*"

# Renovate runs this after every gomod/mise bump, nearly all of which have
# nothing to do with flux2. When the constant and the bootstrap page already name
# the go.mod version the lockstep is intact, so skip the download: an unrelated
# bump must not depend on github.com being reachable. (Deliberately not a content
# check of the archive: a hand-edited archive at the right version is not
# something a bot run should silently overwrite.)
old_version="$(sed -nE 's/^const Version = "([^"]*)".*/\1/p' "$CONST_FILE")"
# The backticks are literal markdown in the bootstrap page, not a command substitution.
# shellcheck disable=SC2016
current_readme="$(sed -nE 's/.*`GotkVersion`, currently \*\*(v[^*]*)\*\*.*/\1/p' "$README_FILE")"
if [[ "$old_version" == "$version" && "$current_readme" == "$version" ]]; then
    echo "sync-gotk-pin: constant and bootstrap page already ${version} -- no change"
    echo "sync-gotk-pin: done"
    exit 0
fi

url="https://github.com/${UPSTREAM_REPO}/releases/download/${version}/manifests.tar.gz"
echo "sync-gotk-pin: ${GO_MODULE} ${version} -- downloading ${url}"

tmp="$(mktemp)"
# regen_log receives the output of step 6's `go test -v`.
regen_log="$(mktemp)"
# golden_snap holds the stored bootstrap outputs as they were, so step 6 can
# put them back.
golden_snap="$(mktemp -d)"
golden_rel="${GOLDEN_DIR#"$REPO_ROOT"/}"

# restore_goldens -- put the stored outputs back exactly as they were, removing
# any file the update added.
restore_goldens() {
    rm -rf "$GOLDEN_DIR"
    cp -Rp "$golden_snap/stored" "$GOLDEN_DIR"
    goldens_unvalidated=false
}

# regen_pid is the process group running step 6's `go test -update` while it
# runs. stop_regen ends that whole group -- go and the test binary it started
# -- and waits until none of it is left, so nothing writes a stored output
# after it has been restored.
regen_pid=""
stop_regen() {
    local i
    [[ -n "$regen_pid" ]] || return 0
    kill -TERM -- "-$regen_pid" 2>/dev/null || true
    for ((i = 0; i < 50; i++)); do
        kill -0 -- "-$regen_pid" 2>/dev/null || break
        sleep 0.1
    done
    kill -KILL -- "-$regen_pid" 2>/dev/null || true
    wait "$regen_pid" 2>/dev/null || true
    regen_pid=""
}

# goldens_unvalidated is true from the moment step 6 may write the stored
# outputs until they are validated or restored, so any other way out -- an
# unexpected error, or a SIGTERM while `go test` runs (bash runs the EXIT trap
# on it too) -- stops the regeneration and puts them back before the snapshot
# is deleted. The .new files are stage()'s scratch copies; a failure while
# staging must not leave one behind in the source tree.
goldens_unvalidated=false
cleanup() {
    stop_regen
    if [[ "$goldens_unvalidated" == true ]]; then
        restore_goldens || echo "sync-gotk-pin: could not restore ${golden_rel}" >&2
    fi
    rm -f "$tmp" "$regen_log" "$ARCHIVE.new" "$CONST_FILE.new" "$README_FILE.new"
    rm -rf "$golden_snap"
}
trap cleanup EXIT

if ! curl -fsSL "${CURL_TIMEOUT_ARGS[@]}" -o "$tmp" "$url"; then
    echo "sync-gotk-pin: could not download ${url}" >&2
    exit 1
fi
if [[ ! -s "$tmp" ]]; then
    echo "sync-gotk-pin: ${url} downloaded empty" >&2
    exit 1
fi
# Cheap shape check: an HTML error page or a truncated body must not replace
# the archive. Deliberately not an inventory check: the tests own the expected
# controllers, and the bot's PR goes red there, visibly, if a release changes
# them. The listing is read into a variable first, never piped into `grep -q`:
# under pipefail a reader that exits early can fail the writer on a closed pipe.
if ! gzip -t "$tmp" 2>/dev/null; then
    echo "sync-gotk-pin: ${url} is not a gzip archive" >&2
    exit 1
fi
listing="$(tar -tzf "$tmp" 2>/dev/null)" || {
    echo "sync-gotk-pin: ${url} is not a tar archive" >&2
    exit 1
}
for entry in source-controller.yaml rbac.yaml; do
    if ! grep -qxE -- "(\./)?${entry//./\\.}" <<<"$listing"; then
        echo "sync-gotk-pin: ${url} has no ${entry} -- not the toolkit install bundle" >&2
        exit 1
    fi
done

# Two phases, so a failure cannot leave the three artifacts disagreeing:
#   1. stage: build every changed file as FILE.new next to its target. Anything
#      that can realistically fail here (a full disk, an unwritable directory, a
#      sed error) fails before a single target has been touched, and the EXIT
#      trap removes the scratch copies.
#   2. swap: rename each staged file into place. A rename within one directory
#      needs no free space and no new permission, so this phase has nothing left
#      to run out of; only an interrupt between two renames could split them, and
#      re-running the script (idempotent) repairs that.
staged=()

# stage FILE [SED_EXPR] -- stage FILE's new content: the downloaded archive when
# no SED_EXPR is given, otherwise FILE with SED_EXPR applied. Records FILE for
# the swap phase only when the content differs. The anchors were proven present
# exactly once by the preflight above.
stage() {
    local file="$1" expr="${2:-}"
    if [[ -n "$expr" ]]; then
        sed -E "$expr" "$file" > "$file.new"
    else
        cp "$tmp" "$file.new"
    fi
    if cmp -s "$file" "$file.new"; then
        rm -f "$file.new"
        echo "sync-gotk-pin: ${file#"$REPO_ROOT"/} already ${version} -- no change"
    else
        staged+=("$file")
    fi
}

stage "$ARCHIVE"
stage "$CONST_FILE" \
    "s/^(const Version = )\"[^\"]*\"/\\1\"${version}\"/"
stage "$README_FILE" \
    "s/(\`GotkVersion\`, currently \\*\\*)v[^*]*(\\*\\*)/\\1${version}\\2/"

# ${staged[@]+...}: an empty array is an unbound variable under `set -u` on bash
# older than 4.4.
regenerate=false
for file in ${staged[@]+"${staged[@]}"}; do
    mv "$file.new" "$file"
    echo "sync-gotk-pin: ${file#"$REPO_ROOT"/} -> ${version}"
    # Either the archive or the version changes what the tests write.
    if [[ "$file" == "$ARCHIVE" || "$file" == "$CONST_FILE" ]]; then
        regenerate=true
    fi
done

if [[ "$regenerate" != true ]]; then
    echo "sync-gotk-pin: done"
    exit 0
fi

# Step 6. The archive is already in place by now and stays there: on a failure
# below, the bot's commit carries the re-vendor with the stored outputs as they
# were, and `test` names the files that differ.
cp -Rp "$GOLDEN_DIR" "$golden_snap/stored"
goldens_unvalidated=true

echo "sync-gotk-pin: regenerating ${golden_rel}"
# -count=1: a cached pass would run nothing, so write nothing. GOWORK=off: the
# bootstrap tests validate against the module graph go.mod pins, which a parent
# go.work could replace (DEVELOPMENT.md). -v: its output names each test that
# ran, which the check below needs. Started in the background under job control
# (set -m) so it gets its own process group for stop_regen: killing go alone
# would leave the test binary it started writing.
set -m
(cd "$REPO_ROOT" && GOWORK=off exec go test ./pkg/stack/fluxcd/ -v -run "$GOLDEN_TESTS" -count=1 -update >"$regen_log" 2>&1) &
regen_pid=$!
set +m
regen_rc=0
wait "$regen_pid" || regen_rc=$?
# go has exited, but if it was killed (OOM, a signal of its own) the test binary
# may still be writing: end what is left of the group either way.
stop_regen
if [[ "$regen_rc" -ne 0 ]]; then
    cat "$regen_log" >&2
    restore_goldens
    echo "sync-gotk-pin: regenerating ${golden_rel} failed; stored outputs left as they were" >&2
    exit 1
fi
# go exits 0 for a -run regexp that matches no test, writing nothing, and for a
# test that skips itself. A test renamed, removed or skipped would then leave its
# stored outputs on the old digest, and the script would report success: so every
# test must have run (its "=== RUN" line, which go also prints before a skip) and
# passed (its top-level "--- PASS:" line, which a skip does not print).
missing=()
for name in "${GOLDEN_TEST_NAMES[@]}"; do
    if ! grep -qxF -- "=== RUN   ${name}" "$regen_log" \
        || ! grep -qE -- "^--- PASS: ${name} \(" "$regen_log"; then
        missing+=("$name")
    fi
done
if [[ ${#missing[@]} -gt 0 ]]; then
    cat "$regen_log" >&2
    restore_goldens
    echo "sync-gotk-pin: these tests did not run and pass, so ${golden_rel} was not regenerated by them:" >&2
    printf '  %s\n' "${missing[@]}" >&2
    echo "sync-gotk-pin: stored outputs left as they were; a test was renamed, removed or skipped, so check the go output above and GOLDEN_TEST_NAMES in this script" >&2
    exit 1
fi
# A golden test writes its stored outputs in subtests, and a subtest that skips
# leaves its output stale while the parent still reports PASS. The unchanged-file
# check below would accept that file, so any skip in these tests is a refusal.
skipped_subtests="$(grep -E -- '^[[:space:]]*--- SKIP: ' "$regen_log" || true)"
if [[ -n "$skipped_subtests" ]]; then
    cat "$regen_log" >&2
    restore_goldens
    echo "sync-gotk-pin: these subtests were skipped, so their stored outputs in ${golden_rel} were not regenerated:" >&2
    printf '%s\n' "$skipped_subtests" >&2
    echo "sync-gotk-pin: stored outputs left as they were" >&2
    exit 1
fi
tail -n 1 "$regen_log"

# The changes a re-vendor may make. A gotk-*.txt file: its first line, the
# bundle summary, with the same object count; everything below it the same. A
# flux-operator-*.txt file: its `    version:` line, from the old toolkit version
# to the new; everything else the same. Each kind's count follows on its own line
# in a gotk file, so a bundle that gained or lost an object changes more than the
# first line anyway.
header_re='^=== install bundle: ([0-9]+) objects, sha256 [0-9a-f]{64} ===$'
digest_only() {
    local old="$1" new="$2" old_header new_header old_count
    old_header="$(head -n 1 "$old")"
    new_header="$(head -n 1 "$new")"
    [[ "$old_header" =~ $header_re ]] || return 1
    old_count="${BASH_REMATCH[1]}"
    [[ "$new_header" =~ $header_re ]] || return 1
    [[ "${BASH_REMATCH[1]}" == "$old_count" ]] || return 1
    cmp -s <(tail -n +2 "$old") <(tail -n +2 "$new")
}
version_only() {
    local old="$1" new="$2"
    cmp -s <(sed -E "s/^    version: ${old_version//./\\.}\$/    version: ${version}/" "$old") "$new"
}

regenerated=()
wider=()
while IFS= read -r name; do
    old="$golden_snap/stored/$name"
    new="$GOLDEN_DIR/$name"
    if [[ -f "$old" && -f "$new" ]] && cmp -s "$old" "$new"; then
        continue
    fi
    explained=false
    if [[ -f "$old" && -f "$new" ]]; then
        if [[ "$name" == gotk-*.txt ]]; then
            digest_only "$old" "$new" && explained=true
        elif [[ "$name" == flux-operator-*.txt ]]; then
            version_only "$old" "$new" && explained=true
        fi
    fi
    if [[ "$explained" == true ]]; then
        regenerated+=("$name")
    else
        wider+=("$name")
    fi
done < <({ ls -A "$golden_snap/stored"; ls -A "$GOLDEN_DIR"; } | sort -u)

if [[ ${#wider[@]} -gt 0 ]]; then
    echo "sync-gotk-pin: the re-vendor changed ${golden_rel} beyond the digest line and the version line:" >&2
    for name in "${wider[@]}"; do
        echo "  ${name}" >&2
        diff -u --label "stored/$name" --label "regenerated/$name" \
            "$golden_snap/stored/$name" "$GOLDEN_DIR/$name" 2>&1 | head -n 40 >&2 || true
    done
    restore_goldens
    echo "sync-gotk-pin: stored outputs left as they were; the rendered bootstrap changed, so review it and regenerate by hand: GOWORK=off go test ./pkg/stack/fluxcd/ -update" >&2
    exit 1
fi
goldens_unvalidated=false

for name in ${regenerated[@]+"${regenerated[@]}"}; do
    echo "sync-gotk-pin: ${golden_rel}/${name} -> ${version}"
done
echo "sync-gotk-pin: done"
