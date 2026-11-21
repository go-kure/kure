#!/bin/bash
# Row 143 (go-kure/kure#1365): Renovate keeps only the files a postUpgradeTasks
# command wrote that match the rule's fileFilters, and drops the rest silently.
# A file the gotk script rewrites but no glob names would be missing from the
# bot's commit, and CI would go red on a drift nothing names. So: run the script
# on a fixture, collect every file it changed, and require each to match a glob
# of the rule in renovate.json that runs the script. A copy of the globs with
# the gotk golden one removed must fail the same check, or the check proves
# nothing. The script must run in the same rule as the flux-operator one.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

renovate="$TEST_DIR/../../renovate.json"
# The rule is found by its command, not by position or description: a command
# equal to the script's own path, never one that merely contains it.
cmd='./scripts/sync-gotk-pin.sh'
rule="select((.postUpgradeTasks.commands // []) | any_c(. == \"$cmd\"))"

rule_count="$(yq eval -p json -o yaml "[.packageRules[] | ${rule}] | length" "$renovate")"
if [[ "$rule_count" != 1 ]]; then
    echo "FAIL: expected exactly one renovate rule running $cmd, found: '$rule_count'" >&2
    exit 1
fi
filter_globs="$(yq eval -p json -o yaml ".packageRules[] | ${rule} | .postUpgradeTasks.fileFilters[]" "$renovate")"
if [[ -z "$filter_globs" ]]; then
    echo "FAIL: the renovate rule running $cmd has no fileFilters" >&2
    exit 1
fi

#
# The two pin scripts share one rule: postUpgradeTasks has no merge semantics,
# so a second rule matching the same bump would replace the command list. The
# order of the two does not matter when both succeed.
operator_rule_count="$(yq eval -p json -o yaml "[.packageRules[] | select((.postUpgradeTasks.commands // []) | any_c(. == \"./scripts/sync-flux-operator-pin.sh\")) | select((.postUpgradeTasks.commands // []) | any_c(. == \"$cmd\"))] | length" "$renovate")"
if [[ "$operator_rule_count" != 1 ]]; then
    echo "FAIL: the rule running $cmd must be the one running ./scripts/sync-flux-operator-pin.sh (found: '$operator_rule_count')" >&2
    exit 1
fi

glob_to_regex() {
    printf '^%s$' "$(printf '%s' "$1" | sed -e 's/[.]/\\./g' -e 's/\*\*/\x01/g' -e 's/\*/[^\/]*/g' -e 's/\x01/.*/g')"
}

# uncovered FILTERFILE FILE... -- print each FILE no glob in FILTERFILE matches.
uncovered() {
    local filterfile="$1" f g hit
    shift
    for f in "$@"; do
        hit=0
        while IFS= read -r g; do
            if [[ "$f" =~ $(glob_to_regex "$g") ]]; then
                hit=1
                break
            fi
        done < "$filterfile"
        if [[ "$hit" -eq 0 ]]; then
            printf '%s\n' "$f"
        fi
    done
}

new_gotk_fixture v0.2.0
filters="$FIXTURE/file-filters.txt"
printf '%s\n' "$filter_globs" > "$filters"
rm -rf "$FIXTURE/before"
mkdir "$FIXTURE/before"
cp -R "$FIXTURE/internal" "$FIXTURE/pkg" "$FIXTURE/before/"
run_gotk
assert_rc 0

changed=()
while IFS= read -r -d '' f; do
    rel="${f#"$FIXTURE"/}"
    if [[ ! -e "$FIXTURE/before/$rel" ]] || ! cmp -s "$FIXTURE/before/$rel" "$f"; then
        changed+=("$rel")
    fi
done < <(find "$FIXTURE/internal" "$FIXTURE/pkg" -type f -print0 | sort -z)

# The run must have written every kind of file the rule has to carry; a short
# list would make the coverage check below pass for nothing.
for want in internal/gotk/manifests.tar.gz \
            internal/gotk/gotk.go \
            pkg/stack/fluxcd/docs/bootstrap-generation.md \
            pkg/stack/fluxcd/testdata/bootstrap-output/gotk-oci.txt \
            pkg/stack/fluxcd/testdata/bootstrap-output/flux-operator-oci.txt; do
    found=0
    for c in "${changed[@]}"; do
        if [[ "$c" == "$want" ]]; then
            found=1
        fi
    done
    if [[ "$found" -eq 0 ]]; then
        echo "FAIL: fixture run did not change $want; changed: ${changed[*]:-none}" >&2
        exit 1
    fi
done

missing="$(uncovered "$filters" "${changed[@]}")"
if [[ -n "$missing" ]]; then
    echo "FAIL: renovate.json fileFilters would drop files the gotk script writes:" >&2
    printf '  %s\n' "${missing//$'\n'/$'\n'  }" >&2
    exit 1
fi

# Negative control: without the gotk golden glob the same check must find the
# gotk stored outputs uncovered.
without="$FIXTURE/file-filters-without-gotk-golden.txt"
grep -vF 'gotk-*.txt' "$filters" > "$without"
if cmp -s "$filters" "$without"; then
    echo "FAIL: control found no gotk-*.txt glob to remove from fileFilters" >&2
    exit 1
fi
control="$(uncovered "$without" "${changed[@]}")"
if [[ "$control" != *bootstrap-output/gotk-oci.txt* ]]; then
    echo "FAIL: control: dropping the gotk golden glob did not uncover the stored outputs; got: '$control'" >&2
    exit 1
fi
