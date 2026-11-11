#!/bin/bash
# Row 137 (go-kure/kure#1259): Renovate keeps only the files a postUpgradeTasks
# command wrote that match the rule's fileFilters, and drops the rest silently.
# A file the pin script rewrites but no glob names would be missing from the
# bot's commit, and CI would go red on a drift nothing names. So: run the script
# on a fixture, collect every file it changed, and require each to match a glob
# of the rule in renovate.json that runs the script. A copy of the globs with the
# golden one removed must fail the same check, or the check proves nothing.
set -uo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/../lib.sh"

renovate="$TEST_DIR/../../renovate.json"
# yq, not jq: the sync scripts already require yq, so every job that runs this
# matrix has it. The rule is found by its command, not by position or
# description: a command equal to the script's own path, never one that merely
# contains it (`contains` on strings is a substring test, and `echo <script>`
# would not run it).
pin_rule='select((.postUpgradeTasks.commands // []) | any_c(. == "./scripts/sync-flux-operator-pin.sh"))'

# pin_rule_count FILE -- how many packageRules in FILE run the pin script.
pin_rule_count() {
    yq eval -p json -o yaml "[.packageRules[] | ${pin_rule}] | length" "$1"
}

rule_count="$(pin_rule_count "$renovate")"
if [[ "$rule_count" != 1 ]]; then
    echo "FAIL: expected exactly one renovate rule running ./scripts/sync-flux-operator-pin.sh, found: '$rule_count'" >&2
    exit 1
fi
filter_globs="$(yq eval -p json -o yaml ".packageRules[] | ${pin_rule} | .postUpgradeTasks.fileFilters[]" "$renovate")"
if [[ -z "$filter_globs" ]]; then
    echo "FAIL: the renovate rule running the pin script has no fileFilters" >&2
    exit 1
fi

# glob_to_regex GLOB -- Renovate's minimatch subset used here: ** crosses
# directories, * stays inside one path segment.
glob_to_regex() {
    printf '^%s$' "$(printf '%s' "$1" | sed -e 's/[.]/\\./g' -e 's/\*\*/\x01/g' -e 's/\*/[^\/]*/g' -e 's/\x01/.*/g')"
}

# uncovered FILTERFILE FILE... -- print each FILE no glob in FILTERFILE matches.
uncovered() {
    local filterfile="$1" f g
    shift
    for f in "$@"; do
        local hit=0
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

new_pin_fixture v0.2.0
# Scratch files live in the fixture, which the helper's EXIT trap removes.
filters="$FIXTURE/file-filters.txt"
printf '%s\n' "$filter_globs" > "$filters"

# Negative control for the rule lookup: a rule whose command only contains the
# script's path (`echo <script>`) is not a rule that runs it.
nearmiss="$FIXTURE/renovate-nearmiss.json"
sed 's|"./scripts/sync-flux-operator-pin.sh"|"echo ./scripts/sync-flux-operator-pin.sh"|' "$renovate" > "$nearmiss"
if cmp -s "$renovate" "$nearmiss"; then
    echo "FAIL: control found no pin command in renovate.json to alter" >&2
    exit 1
fi
if [[ "$(pin_rule_count "$nearmiss")" != 0 ]]; then
    echo "FAIL: the rule lookup matched a command that only contains the pin script's path" >&2
    exit 1
fi
cp -R "$FIXTURE/pkg" "$FIXTURE/pkg-before"
run_pin
assert_rc 0

# Everything the script changed or added under the fixture's pkg/.
changed=()
while IFS= read -r -d '' f; do
    rel="${f#"$FIXTURE"/}"
    before="$FIXTURE/pkg-before/${rel#pkg/}"
    if [[ ! -e "$before" ]] || ! cmp -s "$before" "$f"; then
        changed+=("$rel")
    fi
done < <(find "$FIXTURE/pkg" -type f -print0 | sort -z)

# The run must have written the three kinds of file the rule has to carry; an
# empty or short list would make the coverage check below pass for nothing.
for want in pkg/stack/fluxcd/flux_operator_install.yaml \
            pkg/stack/fluxcd/flux_operator_install.go \
            pkg/stack/fluxcd/docs/bootstrap-generation.md \
            pkg/stack/fluxcd/testdata/bootstrap-output/flux-operator-oci.txt \
            pkg/stack/fluxcd/testdata/bootstrap-output/flux-operator-git.txt; do
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
    echo "FAIL: renovate.json fileFilters would drop files the pin script writes:" >&2
    printf '  %s\n' "${missing//$'\n'/$'\n'  }" >&2
    exit 1
fi

# Negative control: without the golden glob the same check must find the stored
# outputs uncovered.
without="$FIXTURE/file-filters-without-golden.txt"
grep -v 'bootstrap-output' "$filters" > "$without"
if cmp -s "$filters" "$without"; then
    echo "FAIL: control found no bootstrap-output glob to remove from fileFilters" >&2
    exit 1
fi
control="$(uncovered "$without" "${changed[@]}")"
if [[ "$control" != *bootstrap-output/flux-operator-oci.txt* ]]; then
    echo "FAIL: control: dropping the golden glob did not uncover the stored outputs; got: '$control'" >&2
    exit 1
fi
