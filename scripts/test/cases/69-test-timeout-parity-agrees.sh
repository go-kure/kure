#!/bin/bash
# Row 69 (go-kure/kure#792): check-test-timeout passes when the timeout `make test`
# runs with and the one on the test job's "Run tests" go test command agree,
# in any spelling go and the shell accept, and fails naming both locations when
# either side alone changes.
set -uo pipefail

SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/check-test-timeout.sh"
root=$(mktemp -d) || { echo "mktemp -d failed" >&2; exit 1; }
trap 'rm -rf "$root"' EXIT
mkdir -p "$root/scripts" "$root/.github/workflows"
cp "$SRC" "$root/scripts/"

# mk <TEST_TIMEOUT> -- a Makefile with the ?= default and a test recipe using it.
mk() {
    # The recipe is make syntax, written literally.
    # shellcheck disable=SC2016
    printf 'GO ?= go\nTEST_TIMEOUT ?= %s\n\ntest:\n\t$(GO) test -timeout $(TEST_TIMEOUT) ./...\n' "$1" >"$root/Makefile"
}
# ci <go test line> -- ci.yml whose test job's "Run tests" step runs that line.
ci() {
    printf 'jobs:\n  test:\n    steps:\n    - name: Run tests\n      run: |\n        # go test -timeout 1s is only a comment and must not count\n        %s 2>&1 | tee /tmp/gotest.log\n' "$1" >"$root/.github/workflows/ci.yml"
}
run() { out=$(sh "$root/scripts/check-test-timeout.sh" 2>&1); rc=$?; }
fail() { echo "FAIL: $*"; echo "$out"; exit 1; }

mk 15m; ci 'go test -v -race -timeout 15m ./...'; run
[ "$rc" -eq 0 ] || fail "agreeing values should pass"

mk 20m; run
[ "$rc" -ne 0 ] || fail "Makefile-only change should fail"
case "$out" in *"Makefile:2"*".github/workflows/ci.yml:"*) ;; *) fail "error must name both locations";; esac

mk 15m; ci 'go test -v -timeout 30m ./...'; run
[ "$rc" -ne 0 ] || fail "ci.yml-only change should fail"
case "$out" in *"TEST_TIMEOUT ?= 15m"*"-timeout 30m"*) ;; *) fail "error must show both values";; esac

# An inline comment is outside the canonical form, so it can neither hide a
# divergent value nor stand in for a removed flag.
ci 'go test -v -timeout 20m ./... # previously -timeout 15m'; run
[ "$rc" -ne 0 ] || fail "a trailing comment must not mask a divergent -timeout"
ci 'go test -v ./... # previously -timeout 15m'; run
[ "$rc" -ne 0 ] || fail "a comment must not stand in for a removed -timeout"

# Every canonical spelling of an equal value agrees: the = form, the
# -test.timeout alias, extra spaces, a tab.
for spelling in '-test.timeout=15m' '-timeout=15m' '-timeout   15m' $'-timeout\t15m'; do
    ci "go test -v $spelling ./..."; run
    [ "$rc" -eq 0 ] || fail "go test $spelling agrees with 15m and should pass"
done
ci 'go test -v --timeout=30m ./...'; run
[ "$rc" -ne 0 ] || fail "--timeout=30m diverging from 15m should fail"

# Another variable whose name merely ends in TEST_TIMEOUT is not a second assignment.
mk 15m; printf 'INTEGRATION_TEST_TIMEOUT ?= 5m\n' >>"$root/Makefile"; ci 'go test -v -timeout 15m ./...'; run
[ "$rc" -eq 0 ] || fail "INTEGRATION_TEST_TIMEOUT beside TEST_TIMEOUT should pass"
# Recipe text that prints the variable is not an assignment either.
mk 15m
# shellcheck disable=SC2016
printf '\ninfo:\n\t@echo "TEST_TIMEOUT=$(TEST_TIMEOUT)"\n' >>"$root/Makefile"; run
[ "$rc" -eq 0 ] || fail "a recipe echoing TEST_TIMEOUT= should pass"
mk 15m

# Extra whitespace between go and test still names the ci.yml line.
ci $'go  \ttest -v -timeout 30m ./...'; run
[ "$rc" -ne 0 ] || fail "a divergent value with spaced command words should fail"
case "$out" in *".github/workflows/ci.yml:"[0-9]*) ;; *) fail "error must carry the ci.yml line number";; esac
exit 0
