#!/bin/bash
# Row 90 (go-kure/kure#731): the fixture must not touch the caller's git state.
# A git hook exports GIT_DIR/GIT_INDEX_FILE; with those pointing at another
# repository, the fixture still builds in its own directory and the caller's
# index and HEAD are unchanged.
set -uo pipefail

caller=$(mktemp -d) || { echo "mktemp failed" >&2; exit 1; }
# This case's own setup must not inherit repository selectors either: run
# under a git hook, a bare `git -C` here would commit into the real repository.
cgit() {
    env -u GIT_DIR -u GIT_WORK_TREE -u GIT_INDEX_FILE -u GIT_OBJECT_DIRECTORY \
        -u GIT_ALTERNATE_OBJECT_DIRECTORIES -u GIT_COMMON_DIR -u GIT_NAMESPACE -u GIT_PREFIX \
        GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 \
        git -C "$caller" -c user.name=t -c user.email=t@example.invalid -c commit.gpgsign=false \
        -c core.hooksPath=/dev/null "$@"
}
cgit init -q || { echo "setup: git init failed" >&2; exit 1; }
cgit commit -q --allow-empty -m caller || { echo "setup: git commit failed" >&2; exit 1; }
before_head=$(cgit rev-parse HEAD)
before_index=$(cksum <"$caller/.git/index" 2>/dev/null || echo none)
export GIT_DIR="$caller/.git" GIT_INDEX_FILE="$caller/.git/index" GIT_WORK_TREE="$caller"

source "$(dirname "${BASH_SOURCE[0]}")/../pin-impact-lib.sh"
pi_fixture check-a
trap 'rm -rf "$PI_ROOT" "$caller"' EXIT
pi_run
pi_expect 0 "pin refresh is inert"

unset GIT_DIR GIT_INDEX_FILE GIT_WORK_TREE
if [[ "$(cgit rev-parse HEAD)" != "$before_head" ]]; then
    echo "the fixture committed into the caller's repository" >&2
    exit 1
fi
if [[ "$(cksum <"$caller/.git/index" 2>/dev/null || echo none)" != "$before_index" ]]; then
    echo "the fixture rewrote the caller's index" >&2
    exit 1
fi
