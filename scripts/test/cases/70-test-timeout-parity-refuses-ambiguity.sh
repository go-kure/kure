#!/bin/bash
# Row 70 (go-kure/kure#792): check-test-timeout refuses rather than guesses
# whenever the budget either side runs with could differ from the one it read.
set -uo pipefail

SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/check-test-timeout.sh"
root=$(mktemp -d) || { echo "mktemp -d failed" >&2; exit 1; }
trap 'rm -rf "$root"' EXIT
mkdir -p "$root/scripts" "$root/.github/workflows"
cp "$SRC" "$root/scripts/"

# The recipe is make syntax, written literally.
# shellcheck disable=SC2016
RECIPE=$'\ntest:\n\t$(GO) test -timeout $(TEST_TIMEOUT) ./...\n'
mk() { printf '%s%s' "$1" "$RECIPE" >"$root/Makefile"; }
# ci <run block> [<extra yaml appended after the test job>]
ci() {
    {
        printf 'jobs:\n  test:\n    steps:\n    - name: Run tests\n      run: |\n'
        printf '%s\n' "$1" | sed 's/^/        /'
        printf '%s' "${2:-}"
    } >"$root/.github/workflows/ci.yml"
}
run() { out=$(sh "$root/scripts/check-test-timeout.sh" 2>&1); rc=$?; }
fail() { echo "FAIL: $*"; echo "$out"; exit 1; }
good_ci='go test -timeout 15m ./...'

# Makefile side: more than one assignment, or none.
ci "$good_ci"
mk $'TEST_TIMEOUT ?= 15m\nTEST_TIMEOUT ?= 20m\n'; run
[ "$rc" -ne 0 ] || fail "two Makefile assignments should fail"
for override in 'TEST_TIMEOUT := 20m' 'TEST_TIMEOUT = 20m' 'override TEST_TIMEOUT = 20m' 'test: TEST_TIMEOUT := 20m'; do
    mk "TEST_TIMEOUT ?= 15m"$'\n'"$override"$'\n'; run
    [ "$rc" -ne 0 ] || fail "a second assignment ($override) beside the ?= default should fail"
done
mk $'GO ?= go\n'; run
[ "$rc" -ne 0 ] || fail "no Makefile assignment should fail"

# Makefile side: make resolves TEST_TIMEOUT differently from the ?= line.
mk $'TEST_TIMEOUT ?= 15m\ndefine TEST_TIMEOUT\n20m\nendef\n'; run
[ "$rc" -ne 0 ] || fail "a define overriding TEST_TIMEOUT should fail"
mk $'TEST_TIMEOUT ?= 15m\ninclude override.mk\n'
printf 'override TEST_TIMEOUT := 20m\n' >"$root/override.mk"; run
[ "$rc" -ne 0 ] || fail "an included override of TEST_TIMEOUT should fail"
printf 'test: TEST_TIMEOUT := 20m\n' >"$root/override.mk"; run
[ "$rc" -ne 0 ] || fail "an included target-specific TEST_TIMEOUT for test should fail"
# The Makefile recipe is read with the same rules: a -timeout that is another flag's value.
# The recipe is make syntax, written literally.
# shellcheck disable=SC2016
printf 'TEST_TIMEOUT ?= 15m\n\ntest:\n\t$(GO) test -skip -timeout $(TEST_TIMEOUT) ./...\n' >"$root/Makefile"; run
[ "$rc" -ne 0 ] || fail "a recipe -timeout that is -skip's value should fail"
mk $'TEST_TIMEOUT ?= 15m\n'

# CI side: a second timeout in any spelling, anywhere in the file.
ci 'go test -v -timeout 15m -test.timeout 20m ./...'; run
[ "$rc" -ne 0 ] || fail "a -test.timeout alias beside -timeout should fail"
ci $'go test -v -timeout 15m \\\n  -timeout 20m ./...'; run
[ "$rc" -ne 0 ] || fail "a -timeout on a continuation line should fail"
ci 'go test -v -timeout 15m "-test.timeout" 20m ./...'; run
[ "$rc" -ne 0 ] || fail "a quoted -test.timeout alias should fail"
ci $'go test -v -timeout 15m -test.timeout\t20m ./...'; run
[ "$rc" -ne 0 ] || fail "a tab-separated -test.timeout alias should fail"
ci "$good_ci" $'    env:\n      GOFLAGS: "-timeout=20m"\n'; run
[ "$rc" -ne 0 ] || fail "a GOFLAGS timeout should fail"
ci "$good_ci" $'  other:\n    timeout-minutes: 20\n    steps: []\n'; run
[ "$rc" -eq 0 ] || fail "timeout-minutes beside one -timeout flag should pass"

# CI side: the budget must be on the suite's own command.
ci 'go test ./...'; run
[ "$rc" -ne 0 ] || fail "no -timeout on the suite command should fail"
ci $'echo "expected: go test -timeout 15m"\ngo test ./...'; run
[ "$rc" -ne 0 ] || fail "a -timeout only inside an echo should fail"
# A continuation lets another command consume a timeout-bearing line, or splits the real
# go test across lines; the step must not use one.
ci $'printf \'%s\\n\' \\\ngo test -timeout 15m ./...\ngo \\\ntest -v ./...'; run
[ "$rc" -ne 0 ] || fail "a continuation making go test -timeout an argument should fail"
# Quotes, comments and expansions are outside the canonical form: quoted text
# can pose as a flag (-skip '-timeout 15m') or hide the real one behind a '#'.
ci "go test -v -skip '-timeout 15m' ./..."; run
[ "$rc" -ne 0 ] || fail "a -timeout inside a quoted argument should fail"
ci "go test -v -timeout 15m -skip ' # unrelated' -test.timeout 20m ./..."; run
[ "$rc" -ne 0 ] || fail "a quoted # hiding a second timeout should fail"
# '$T' is a literal expansion written into the fixture on purpose.
# shellcheck disable=SC2016
for spelling in '-timeout "15m"' "-timeout '15m'" '"-timeout" 15m' '-timeout $T'; do
    ci "go test -v $spelling ./..."; run
    [ "$rc" -ne 0 ] || fail "a quoted or expanded timeout ($spelling) should fail"
done
# A redirection does not end the command: whatever follows 2>&1 is still its arguments.
ci "go test -timeout 15m ./... 2>&1 -skip ' # unrelated' -test.timeout 20m"; run
[ "$rc" -ne 0 ] || fail "a quoted override after 2>&1 should fail"
ci 'go test -timeout 15m ./... 2>&1 -test.timeout 20m'; run
[ "$rc" -ne 0 ] || fail "a second timeout after 2>&1 should fail"
# A value-taking flag written without = takes the next word as its argument, so a
# "timeout" there is that flag's value: go test -skip -test.timeout=15m runs with 10m.
ci 'go test -v -skip -test.timeout=15m ./...'; run
[ "$rc" -ne 0 ] || fail "a timeout that is another flag's value should fail"
ci 'go test -v -run -timeout 15m ./...'; run
[ "$rc" -ne 0 ] || fail "a -timeout that is -run's value should fail"
# An escaped space joins two apparent words into one argument; globs and braces expand.
ci 'go test -v -skip=foo\ -timeout=15m ./...'; run
[ "$rc" -ne 0 ] || fail "an escaped space hiding the timeout inside -skip should fail"
ci 'go test -v -run=Test* -timeout 15m ./...'; run
[ "$rc" -ne 0 ] || fail "a glob in the go test command should fail"
# The Makefile recipe is held to the same form.
# shellcheck disable=SC2016
printf 'TEST_TIMEOUT ?= 15m\n\ntest:\n\t$(GO) test -v -skip=foo\\ -timeout=$(TEST_TIMEOUT) ./...\n' >"$root/Makefile"; ci "$good_ci"; run
[ "$rc" -ne 0 ] || fail "an escaped space in the recipe should fail"
mk $'TEST_TIMEOUT ?= 15m\n'
# After a positional argument go test may stop parsing flags and ignore a later
# -timeout, so the flag must precede the package patterns -- on both sides.
ci 'go test ./... -v positional -timeout 15m'; run
[ "$rc" -ne 0 ] || fail "a -timeout after a positional argument should fail"
# shellcheck disable=SC2016
printf 'TEST_TIMEOUT ?= 15m\n\ntest:\n\t$(GO) test ./... -v positional -timeout $(TEST_TIMEOUT)\n' >"$root/Makefile"; ci "$good_ci"; run
[ "$rc" -ne 0 ] || fail "a recipe -timeout after a positional argument should fail"
# A continuation lets the checked line be another command's argument and hides the real one.
# shellcheck disable=SC2016
printf 'GO := go\nTEST_TIMEOUT ?= 15m\ntest:\n\t@printf "%%s\\n" \\\n\t$(GO) test -timeout $(TEST_TIMEOUT) ./...\n\t@$(GO) \\\n\ttest -timeout 20m ./...\n' >"$root/Makefile"; run
[ "$rc" -ne 0 ] || fail "a recipe line continuation should fail"
# Under .ONESHELL a heredoc turns the go test line into text that never runs; so does an exit.
# shellcheck disable=SC2016
printf 'GO := go\nTEST_TIMEOUT ?= 15m\n.ONESHELL:\ntest:\n\t@cat <<EOF\n\t$(GO) test -timeout $(TEST_TIMEOUT) ./...\n\tEOF\n' >"$root/Makefile"; run
[ "$rc" -ne 0 ] || fail "a .ONESHELL heredoc-only recipe should fail"
# shellcheck disable=SC2016
printf 'TEST_TIMEOUT ?= 15m\n.ONESHELL:\ntest:\n\texit 0\n\t$(GO) test -timeout $(TEST_TIMEOUT) ./...\n' >"$root/Makefile"; run
[ "$rc" -ne 0 ] || fail "an exit before go test should fail"
# An escaped closing quote leaves the string open across the go test line.
# shellcheck disable=SC2016
printf 'TEST_TIMEOUT ?= 15m\n.ONESHELL:\ntest:\n\t@echo "before\\"\n\t$(GO) test -timeout $(TEST_TIMEOUT) ./...\n\t@echo "after\\"\n' >"$root/Makefile"; run
[ "$rc" -ne 0 ] || fail "an echo whose closing quote is escaped should fail"
# An escaped backslash before the quote closes it.
# shellcheck disable=SC2016
printf 'TEST_TIMEOUT ?= 15m\ntest:\n\t@echo "dir\\\\"\n\t$(GO) test -timeout $(TEST_TIMEOUT) ./...\n' >"$root/Makefile"; run
[ "$rc" -eq 0 ] || fail "an echo ending in an escaped backslash should pass"
# The real recipe's echo lines are fine.
# shellcheck disable=SC2016
printf 'TEST_TIMEOUT ?= 15m\ntest:\n\t@echo "\\033[33mRunning tests...\\033[0m"\n\t$(GO) test -timeout $(TEST_TIMEOUT) ./...\n\t@echo "done"\n' >"$root/Makefile"; run
[ "$rc" -eq 0 ] || fail "echo lines around go test should pass"
# Anything chained after the recipe's go test is unread, so it is refused whatever it is.
# shellcheck disable=SC2016
printf 'TEST_TIMEOUT ?= 15m\ntest:\n\t$(GO) test -timeout $(TEST_TIMEOUT) ./pkg/errors && "$(GO)" test -timeout 20m ./...\n' >"$root/Makefile"; run
[ "$rc" -ne 0 ] || fail "a quoted \$(GO) test chained on the recipe line should fail"
# A second test command chained on the recipe line runs with its own budget.
# shellcheck disable=SC2016
printf 'TEST_TIMEOUT ?= 15m\n\ntest:\n\t$(GO) test -timeout $(TEST_TIMEOUT) ./pkg/errors && $(GO) test -timeout 20m ./...\n' >"$root/Makefile"; run
[ "$rc" -ne 0 ] || fail "a second \$(GO) test chained on the recipe line should fail"
# shellcheck disable=SC2016
printf 'TEST_TIMEOUT ?= 15m\n\ntest:\n\t$(GO) test -timeout $(TEST_TIMEOUT) ./pkg/errors && go test -timeout 20m ./...\n' >"$root/Makefile"; run
[ "$rc" -ne 0 ] || fail "a literal go test chained on the recipe line should fail"
mk $'TEST_TIMEOUT ?= 15m\n'
# A second go test chained on the same line is a second command.
ci 'go test -timeout 15m ./pkg/errors && go test -v ./...'; run
[ "$rc" -ne 0 ] || fail "a second go test chained on the line should fail"
ci 'go test -timeout 15m ./pkg/errors && "go" test -v ./...'; run
[ "$rc" -ne 0 ] || fail "a quoted \"go\" test chained on the CI line should fail"
ci 'go test -timeout 15m ./... 2>&1 | tee /tmp/gotest.log && echo done'; run
[ "$rc" -ne 0 ] || fail "anything after the tee should fail"
ci 'go test ./... ; echo -timeout 15m'; run
[ "$rc" -ne 0 ] || fail "a -timeout on a later command on the same line should fail"
ci 'go test ./... && echo -timeout 15m'; run
[ "$rc" -ne 0 ] || fail "a -timeout after && on the same line should fail"
ci $'cat <<EOF\ngo test -timeout 15m ./...\nEOF\ngo test ./...'; run
[ "$rc" -ne 0 ] || fail "a -timeout only inside heredoc text should fail"
# Anything else in the step could keep the one go test line from running as a command:
# heredoc text, a multi-line quote, a branch, an early exit, noexec, or another go on PATH.
for block in $'cat <<EOF\ngo test -timeout 15m ./...\nEOF' \
    $'echo "\ngo test -timeout 15m ./...\n"' \
    $'if false; then\ngo test -timeout 15m ./...\nfi' \
    $'exit 0\ngo test -timeout 15m ./...' \
    $'set -n\ngo test -timeout 15m ./...' \
    $'set -o noexec\ngo test -timeout 15m ./...' \
    $'export PATH=/opt/fake\ngo test -timeout 15m ./...'; do
    ci "$block"; run
    [ "$rc" -ne 0 ] || fail "a step where go test may not run should fail: $block"
done
ci $'set -euo pipefail\nmkdir -p coverage\ngo test -timeout 15m ./...'; run
[ "$rc" -eq 0 ] || fail "set -euo pipefail and mkdir beside go test should pass"
ci 'go test ./...' $'  smoke:\n    steps:\n    - name: Run tests\n      run: go test -timeout 15m ./smoke/...\n'; run
[ "$rc" -ne 0 ] || fail "a -timeout only on another job's go test should fail"
ci "$good_ci" $'    - name: Run tests\n      run: go test ./...\n'; run
[ "$rc" -ne 0 ] || fail "two 'Run tests' steps in the test job should fail"
exit 0
