#!/bin/sh
# check-test-timeout.sh — enforce parity between the local test budget and CI's.
#
# The budget exists twice: `TEST_TIMEOUT ?= <d>` in the Makefile (what `make test`/`mise run test`
# enforce) and the `-timeout <d>` literal on the `go test` line of the test job in
# .github/workflows/ci.yml (what CI enforces). No execution path reads one and applies it to the
# other, so a change to either alone diverges silently — the defect class of go-kure/kure#781,
# where the local gate failed every changeset while CI stayed green (go-kure/kure#792).
#
# CI images do not have mise installed, so this is a plain POSIX shell script CI can run
# directly — it must NOT depend on mise. It needs make and yq on PATH (the validate job installs
# yq before running it).
#
# Exit non-zero on any mismatch, naming both locations.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

MK="Makefile"
CI=".github/workflows/ci.yml"

# timeout_of <where> <command> -- print the timeout a go test command runs with, or explain on
# stderr and return 1. Shell is too rich to parse from text, so the command is held to a canonical
# form that can be read exactly, and anything outside it fails rather than being guessed at:
#   - the command is cut at its first `;`, `&` or `|`, after descriptor redirections such as
#     `2>&1` (which do not end a command) are replaced by a placeholder word;
#   - every character is a letter, digit, whitespace or one of `. _ / = , : + @ % -`, so there is
#     no quote, escape, expansion, glob or other shell syntax and every argument is a plain word;
#   - every flag word is a known boolean flag or written -flag=value, except the timeout flag,
#     whose value may be the next word: any other flag could take the next word as its value
#     (`-skip -test.timeout=15m` hands the "timeout" to -skip);
#   - exactly one timeout flag (-timeout, --timeout, -test.timeout), with a value.
timeout_of() {
	_cmd="$(printf '%s\n' "$2" | sed -E 's/[0-9]*>&[0-9-]+/ _redirect_ /g; s/[;&|].*$//')"
	case "$_cmd" in
	*[!A-Za-z0-9._/=,:+@%[:space:]-]*)
		echo "✗ test budget: the go test command $1 uses shell syntax (a quote, escape, expansion, glob or similar); keep its arguments plain words so its timeout can be read" >&2
		return 1
		;;
	esac
	_val=""
	_flags=0
	_want=0
	_skip=2      # the program word (go, or make's marker) and `test`
	_positional=0 # a package pattern or other positional word has been seen
	for _word in $_cmd; do
		if [ "$_skip" -gt 0 ]; then
			_skip=$((_skip - 1))
			continue
		fi
		if [ "$_want" -eq 1 ]; then
			_val="$_word"
			_want=0
			continue
		fi
		case "$_word" in
		-timeout=* | --timeout=* | -test.timeout=* | --test.timeout=* | -timeout | --timeout | -test.timeout | --test.timeout)
			# After a positional word go test may stop reading flags, passing a later
			# -timeout through as an ignored argument; the flag must come first.
			if [ "$_positional" -eq 1 ]; then
				echo "✗ test budget: the timeout flag on the go test command $1 follows a positional argument, where go test may ignore it; put it before the package patterns" >&2
				return 1
			fi
			_flags=$((_flags + 1))
			case "$_word" in
			*=*) _val="${_word#*=}" ;;
			*) _want=1 ;;
			esac
			;;
		-v | -race | -short | -failfast | -cover | -json | -x | -n | -a | -benchmem | -trimpath | -msan | -asan | -work) ;;
		-*=*) ;;
		-*)
			echo "✗ test budget: '$_word' on the go test command $1 may take the next word as its value; write it as $_word=<value> so the timeout can be read unambiguously" >&2
			return 1
			;;
		*) _positional=1 ;;
		esac
	done
	if [ "$_flags" -ne 1 ] || [ "$_want" -eq 1 ] || [ -z "$_val" ]; then
		echo "✗ test budget: the go test command $1 must carry exactly one timeout flag with a value (found $_flags)" >&2
		return 1
	fi
	printf '%s\n' "$_val"
}

# Exactly one assignment of TEST_TIMEOUT in any form, and it must be the `?=` default: a second
# `:=`/`=`/`+=`/`override`/target-specific assignment would win or lose depending on how make
# resolves it, while this check compared the default. Tab-led recipe lines are not assignments
# (an `@echo TEST_TIMEOUT=...` diagnostic must not count); whatever make does resolve is then
# checked against `make -n test` below.
mk_count="$(grep -vE '^	' "$MK" | grep -cE '^([^#]*[^#A-Za-z0-9_])?TEST_TIMEOUT[[:space:]]*(\?|::?|\+|!)?=' || true)"
if [ "$mk_count" -ne 1 ]; then
	echo "✗ test budget: expected exactly 1 assignment of TEST_TIMEOUT in $MK (the '?=' default), found $mk_count"
	exit 1
fi
if ! grep -qE '^TEST_TIMEOUT[[:space:]]*\?=' "$MK"; then
	echo "✗ test budget: the one TEST_TIMEOUT assignment in $MK is not the top-level 'TEST_TIMEOUT ?=' default"
	exit 1
fi
MK_LINE="$(grep -nE '^TEST_TIMEOUT[[:space:]]*\?=' "$MK")"
MK_VAL="$(printf '%s\n' "$MK_LINE" | sed -E 's/^[0-9]+:TEST_TIMEOUT[[:space:]]*\?=[[:space:]]*//; s/[[:space:]]*#.*$//; s/[[:space:]]+$//')"

# The text of the assignment is not what `make test` uses: a `define` block, an included
# makefile, an `override` or a target-specific assignment can change the value without a second
# `TEST_TIMEOUT ... =` line for the count above to see. So read what `make test` would actually
# run: a dry run (-n executes no recipe) with GO replaced by a marker, and TEST_TIMEOUT, MAKEFLAGS
# and MAKELEVEL removed from the environment so the committed Makefile is what is measured.
# The timeout on that command must be the default this check located.
MK_DRY="$(env -u TEST_TIMEOUT -u MAKEFLAGS -u MAKELEVEL make -n --no-print-directory -f "$MK" test GO=__check_test_timeout_go 2>/dev/null || true)"
# As on the CI side, no `\` continuation: a continued line can make the go test text an
# argument of the command above it, or split the real invocation where the line reader
# below cannot see it.
if printf '%s\n' "$MK_DRY" | grep -qE '\\[[:space:]]*$'; then
	echo "✗ test budget: the recipe 'make -n test' runs in $MK uses a line continuation (\\); keep the go test command on one line so its timeout can be read"
	exit 1
fi
MK_CMD="$(printf '%s\n' "$MK_DRY" | grep -E '^__check_test_timeout_go test ' || true)"
# The recipe's go test line is the whole command: nothing may be chained, piped or backgrounded
# after it, since whatever follows (a quoted "$(GO)" test, another tool) is never read.
if printf '%s\n' "$MK_CMD" | grep -qE '[;&|]'; then
	echo "✗ test budget: the go test line 'make -n test' runs in $MK chains another command (; & |); keep it a single command"
	exit 1
fi
# Every other line the recipe runs must be a plain `echo "..."` (one closed double-quoted string
# whose backslashes never escape a quote,
# no expansion), `mkdir …` or `set -euxo pipefail`, as on the CI side: a heredoc, an open quote,
# a branch or an exit (under .ONESHELL the recipe is one script) could otherwise turn the go test
# line into text that never runs.
# The regex matches a literal $ and backtick on purpose.
# shellcheck disable=SC2016
mk_bad="$(printf '%s\n' "$MK_DRY" | grep -vE '^__check_test_timeout_go test ' | grep -vE '^[[:space:]]*$' |
	grep -vE '^[[:space:]]*(echo[[:space:]]+"([^"$`\\]|\\[^"$`])*"|mkdir([[:space:]]+[A-Za-z0-9._/=,:+@%-]+)+|set([[:space:]]+(-[eux]*o?|pipefail))+)[[:space:]]*$' | head -1 || true)"
if [ -n "$mk_bad" ]; then
	echo "✗ test budget: 'make -n test' in $MK may run only echo \"...\", mkdir and set -euxo pipefail lines besides go test; cannot tell whether go test runs past: $mk_bad"
	exit 1
fi
# Count invocations, not lines: a second `$(GO) test` (or a literal `go test`) chained on the same
# recipe line is cut off by timeout_of and would run with a budget this check never read.
if [ "$(printf '%s\n' "$MK_DRY" | grep -oE '(^|[^A-Za-z0-9_])(__check_test_timeout_go|go)[[:space:]]+test([[:space:]]|$)' | grep -c . || true)" -ne 1 ] ||
	[ "$(printf '%s' "$MK_CMD" | grep -c . || true)" -ne 1 ]; then
	echo "✗ test budget: 'make -n test' does not run exactly one \$(GO) test command in $MK"
	exit 1
fi
MK_RES="$(timeout_of "that 'make -n test' runs" "$MK_CMD")" || exit 1
if [ "$MK_RES" != "$MK_VAL" ]; then
	echo "✗ test budget: 'make test' runs with a timeout of '$MK_RES', not the '$MK_VAL' of the TEST_TIMEOUT ?= default -- a define, include, override or target-specific assignment changes it"
	exit 1
fi

# Work on ci.yml with shell comments removed (from a `#` at line start or after whitespace): a
# trailing `# previously -timeout 15m` must neither count nor be read as the value. The go test
# line carries no quoted `#`.
CI_CODE="$(grep -nE '' "$CI" | sed -E 's/(^[0-9]+:|[[:space:]])#.*$/\1/')"

# Every other mention of "timeout" is suspect, in any spelling or quoting: a continuation line
# (`\` then `-timeout 20m`), a `-test.timeout` alias, a quoted `"-test.timeout" 20m`, a tab
# separator or a GOFLAGS value would each override the budget while a parse of the go test line
# still read 15m -- and shell has too many ways to spell an argument to enumerate them. So allow
# exactly one timeout token in the whole comment-stripped file, the go test flag itself, and
# accept any other token containing "timeout" only if it is one of the known non-flag words below.
ALLOWED_TIMEOUT_WORDS=' timeout-minutes check-test-timeout check-test-timeout.sh '
timeout_tokens="$(printf '%s\n' "$CI_CODE" | sed -E 's/^[0-9]+://' | grep -oiE '[A-Za-z0-9_.-]*timeout[A-Za-z0-9_.-]*' || true)"
flag_count=0
for tok in $timeout_tokens; do
	case "$ALLOWED_TIMEOUT_WORDS" in
	*" $tok "*) ;;
	*) flag_count=$((flag_count + 1)) ;;
	esac
done
if [ "$flag_count" -ne 1 ]; then
	echo "✗ test budget: expected exactly 1 timeout token in $CI (the go test -timeout flag), found $flag_count"
	echo "  Any other 'timeout' mention outside comments, other than timeout-minutes, is treated as a possible override."
	exit 1
fi
# The budget that matters is the one the suite actually runs with: the `run:` of the step named
# "Run tests" in the `test` job, read with yq so a go test in another job, a smoke step or a
# heredoc's text cannot stand in for it. That step must hold exactly one line that starts with
# `go test` (so text that merely mentions a go test, or a second invocation, fails), and the
# flag must be on that line.
command -v yq >/dev/null 2>&1 || { echo "✗ test budget: yq is required to read $CI"; exit 1; }
if [ "$(yq '[.jobs.test.steps[] | select(.name == "Run tests")] | length' "$CI")" != 1 ]; then
	echo "✗ test budget: expected exactly one step named 'Run tests' in the test job of $CI"
	exit 1
fi
# Shell is too rich to parse from text, so this step is held to a canonical form that can be
# read exactly, and anything outside it fails rather than being guessed at:
#   - whole-line comments are ignored; nothing else is stripped;
#   - no `\` line continuation (a line that looks like `go test -timeout 15m` could be an
#     argument to the command above it);
#   - `go test` appears exactly once in the step, chained commands included;
#   - every other line is `mkdir …` or `set -euxo pipefail` (see below);
#   - that command is read by timeout_of (above), which holds it to the canonical form.
RUN_BLOCK="$(yq '.jobs.test.steps[] | select(.name == "Run tests") | .run' "$CI" | grep -vE '^[[:space:]]*#' || true)"
if printf '%s\n' "$RUN_BLOCK" | grep -qE '\\[[:space:]]*$'; then
	echo "✗ test budget: the test job's 'Run tests' step in $CI uses a line continuation (\\); keep the go test command on one line so its timeout can be read"
	exit 1
fi
if [ "$(printf '%s\n' "$RUN_BLOCK" | grep -oE '(^|[^A-Za-z0-9_])go[[:space:]]+test([[:space:]]|$)' | grep -c . || true)" -ne 1 ]; then
	echo "✗ test budget: the test job's 'Run tests' step in $CI must run exactly one go test command"
	exit 1
fi
# A descriptor redirection (`2>&1`, `>&2`) does not end the command, so it is replaced by a
# placeholder word before the line is cut at the first real `;`, `&` or `|`; everything the
# command receives up to that point is then checked.
GO_TEST_CMD="$(printf '%s\n' "$RUN_BLOCK" | grep -E '^[[:space:]]*go[[:space:]]+test[[:space:]]' || true)"
if [ -z "$GO_TEST_CMD" ]; then
	echo "✗ test budget: the go test command in the test job's 'Run tests' step must start its own line"
	exit 1
fi
# Every other line must be one of a few plain commands that cannot stop go test from running or
# change what it is: unquoted words from a narrow character set, first word `mkdir` or `set`
# (and `set` only with -e/-u/-x/-o pipefail). A heredoc, an open quote, a compound command
# (if/while/case, functions, braces, subshells), `exit`, `set -n` or a PATH change could
# otherwise turn the go test line into text, a branch that never runs, or another program.
OTHER_LINES="$(printf '%s\n' "$RUN_BLOCK" | grep -vE '^[[:space:]]*go[[:space:]]+test[[:space:]]' | grep -vE '^[[:space:]]*$' || true)"
bad_line="$(printf '%s\n' "$OTHER_LINES" | grep -vE '^[[:space:]]*$' | grep -vE '^[[:space:]]*(mkdir([[:space:]]+[A-Za-z0-9._/=,:+@%-]+)+|set([[:space:]]+(-[eux]*o?|pipefail))+)[[:space:]]*$' | head -1 || true)"
if [ -n "$bad_line" ]; then
	echo "✗ test budget: the test job's 'Run tests' step in $CI may hold only mkdir and set -euxo pipefail lines besides go test; cannot tell whether go test runs past: $bad_line"
	exit 1
fi
# The only thing allowed after the go test command is piping its output to a file
# (`2>&1 | tee <path>`): anything else chained there (a quoted "go" test, another tool) is
# never read.
if ! printf '%s\n' "$GO_TEST_CMD" | grep -qE '^[[:space:]]*go[[:space:]]+test[[:space:]][^;&|]*(2>&1[[:space:]]*\|[[:space:]]*tee[[:space:]]+[A-Za-z0-9._/-]+)?[[:space:]]*$'; then
	echo "✗ test budget: the go test command in the test job's 'Run tests' step of $CI may be followed only by '2>&1 | tee <file>'"
	exit 1
fi
CI_VAL="$(timeout_of "in the test job's 'Run tests' step of $CI" "$GO_TEST_CMD")" || exit 1
# For the message: the ci.yml line holding that command.
CI_LINE="$(grep -nE '^[[:space:]]*(run:[[:space:]]*)?go[[:space:]]+test[[:space:]].*timeout' "$CI" | head -1)"

if [ -z "$MK_VAL" ] || [ -z "$CI_VAL" ]; then
	echo "✗ test budget: could not read a value (Makefile: '$MK_VAL', ci.yml: '$CI_VAL')"
	exit 1
fi

MK_AT="$MK:${MK_LINE%%:*}"
CI_AT="$CI:${CI_LINE%%:*}"
if [ "$MK_VAL" != "$CI_VAL" ]; then
	echo "✗ test budget differs: $MK_AT has TEST_TIMEOUT ?= $MK_VAL, $CI_AT runs go test -timeout $CI_VAL"
	echo "  Set both to the same value; the local gate and CI must enforce one budget."
	exit 1
fi
echo "✓ test budget: $MK_VAL in both $MK_AT and $CI_AT"
